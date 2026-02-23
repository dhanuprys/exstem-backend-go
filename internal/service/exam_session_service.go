package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// ExamSessionService handles exam session business logic.
type ExamSessionService struct {
	sessionRepo *repository.ExamSessionRepository
	examRepo    *repository.ExamRepository
	targetRepo  *repository.ExamTargetRuleRepository
	rdb         *redis.Client
}

// NewExamSessionService creates a new ExamSessionService.
func NewExamSessionService(
	sessionRepo *repository.ExamSessionRepository,
	examRepo *repository.ExamRepository,
	targetRepo *repository.ExamTargetRuleRepository,
	rdb *redis.Client,
) *ExamSessionService {
	return &ExamSessionService{
		sessionRepo: sessionRepo,
		examRepo:    examRepo,
		targetRepo:  targetRepo,
		rdb:         rdb,
	}
}

// LobbyStatus represents the concrete state of an exam in the lobby.
type LobbyStatus string

const (
	LobbyStatusUpcoming   LobbyStatus = "UPCOMING"
	LobbyStatusAvailable  LobbyStatus = "AVAILABLE"
	LobbyStatusInProgress LobbyStatus = "IN_PROGRESS"
	LobbyStatusCompleted  LobbyStatus = "COMPLETED"
	LobbyStatusClosed     LobbyStatus = "CLOSED"
)

// LobbyExam represents an exam as displayed in the student lobby.
type LobbyExam struct {
	ID              uuid.UUID            `json:"id"`
	Title           string               `json:"title"`
	ScheduledStart  *time.Time           `json:"scheduled_start,omitempty"`
	ScheduledEnd    *time.Time           `json:"scheduled_end,omitempty"`
	DurationMinutes int                  `json:"duration_minutes"`
	Status          model.ExamStatus     `json:"status"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	LobbyStatus     LobbyStatus          `json:"lobby_status"`
	SessionStatus   *model.SessionStatus `json:"session_status,omitempty"`
	FinalScore      *float64             `json:"final_score,omitempty"`
}

// GetLobby returns the list of exams available to a student based on their class.
func (s *ExamSessionService) GetLobby(ctx context.Context, studentID, classID int) ([]LobbyExam, error) {
	// Find all exam IDs targeting this student's class/grade/major.
	examIDs, err := s.targetRepo.FindExamsForStudent(ctx, classID)
	if err != nil {
		return nil, fmt.Errorf("find exams for student: %w", err)
	}

	// Get the student's existing sessions for status overlay.
	sessions, err := s.sessionRepo.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	sessionMap := make(map[uuid.UUID]*model.ExamSession, len(sessions))
	for i := range sessions {
		sessionMap[sessions[i].ExamID] = &sessions[i]
	}

	var lobby []LobbyExam
	now := time.Now()

	for _, eid := range examIDs {
		exam, err := s.examRepo.GetByID(ctx, eid)
		if err != nil {
			continue // Skip if exam was deleted
		}

		// Only show PUBLISHED or IN_PROGRESS exams in the lobby.
		if exam.Status != model.ExamStatusPublished && exam.Status != model.ExamStatusInProgress {
			continue
		}

		entry := LobbyExam{
			ID:              exam.ID,
			Title:           exam.Title,
			ScheduledStart:  exam.ScheduledStart,
			ScheduledEnd:    exam.ScheduledEnd,
			DurationMinutes: exam.DurationMinutes,
			Status:          exam.Status,
			CreatedAt:       exam.CreatedAt,
			UpdatedAt:       exam.UpdatedAt,
		}

		// Determine LobbyStatus
		if sess, ok := sessionMap[eid]; ok {
			entry.SessionStatus = &sess.Status
			entry.FinalScore = sess.FinalScore
			if sess.Status == model.SessionStatusCompleted {
				entry.LobbyStatus = LobbyStatusCompleted
			} else {
				entry.LobbyStatus = LobbyStatusInProgress
			}
		} else {
			// No session yet. Check schedule.
			if exam.ScheduledEnd != nil && now.After(*exam.ScheduledEnd) {
				entry.LobbyStatus = LobbyStatusClosed // Time's up
			} else if exam.ScheduledStart != nil && exam.ScheduledStart.After(now) {
				// Only show upcoming if it's scheduled for today
				y1, m1, d1 := exam.ScheduledStart.Date()
				y2, m2, d2 := now.Date()
				if y1 == y2 && m1 == m2 && d1 == d2 {
					entry.LobbyStatus = LobbyStatusUpcoming
				} else {
					// Don't show exams for future days
					continue
				}
			} else {
				entry.LobbyStatus = LobbyStatusAvailable
			}
		}

		lobby = append(lobby, entry)
	}

	return lobby, nil
}

// GetActiveExam returns the exam ID of the student's currently active session.
// It checks Redis first, falls back to PostgreSQL, and self-heals the cache.
func (s *ExamSessionService) GetActiveExam(ctx context.Context, studentID int) (*uuid.UUID, error) {
	key := config.CacheKey.StudentActiveExamKey(studentID)
	val, err := s.rdb.Get(ctx, key).Result()

	if err == redis.Nil {
		// Cache miss — check DB
		examID, dbErr := s.sessionRepo.GetActiveExamID(ctx, studentID)
		if dbErr != nil {
			if errors.Is(dbErr, pgx.ErrNoRows) {
				return nil, nil // No active session
			}
			return nil, fmt.Errorf("db fallback for active exam: %w", dbErr)
		}
		// Self-heal: cache in Redis
		_ = s.rdb.Set(ctx, key, examID.String(), 0)
		return examID, nil
	} else if err != nil {
		return nil, fmt.Errorf("redis error: %w", err)
	}

	// Cache hit
	parsed, err := uuid.Parse(val)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID in active_exam cache: %w", err)
	}
	return &parsed, nil
}

// JoinExam validates the entry token and creates a session for the student.
// classID is required to verify the student's class is eligible for this exam.
func (s *ExamSessionService) JoinExam(ctx context.Context, examID uuid.UUID, studentID, classID int, entryToken string) (*model.ExamSession, error) {
	exam, err := s.examRepo.GetByID(ctx, examID)
	if err != nil {
		return nil, fmt.Errorf("get exam: %w", err)
	}

	if exam.Status != model.ExamStatusPublished && exam.Status != model.ExamStatusInProgress {
		return nil, errors.New("exam is not available for joining")
	}

	now := time.Now()
	if exam.ScheduledStart != nil && now.Before(*exam.ScheduledStart) {
		return nil, errors.New("exam is not available for joining")
	}
	if exam.ScheduledEnd != nil && now.After(*exam.ScheduledEnd) {
		return nil, errors.New("exam is not available for joining")
	}

	if exam.EntryToken != entryToken {
		return nil, errors.New("invalid entry token")
	}

	// SECURITY: Verify the student's class is an eligible target for this exam.
	// This prevents a student from joining an exam that was not targeted at
	// their class/grade/major, even if they somehow obtained the entry token.
	allowedExamIDs, err := s.targetRepo.FindExamsForStudent(ctx, classID)
	if err != nil {
		return nil, fmt.Errorf("check eligibility: %w", err)
	}
	eligible := false
	for _, eid := range allowedExamIDs {
		if eid == examID {
			eligible = true
			break
		}
	}
	if !eligible {
		return nil, errors.New("exam is not available for joining")
	}

	// Check if student already has a session.
	existing, err := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing session: %w", err)
	}

	// IDEMPOTENCY CHECK: If they already joined, ensure Redis has the start time
	// This handles cases where they joined on a different device or refreshed immediately.
	if existing != nil {
		_ = s.rdb.Set(ctx, config.CacheKey.StudentExamSessionStartKey(examID.String(), studentID), existing.StartedAt.Unix(), 0)
		// Ensure active_exam key is set (idempotent)
		_ = s.rdb.Set(ctx, config.CacheKey.StudentActiveExamKey(studentID), examID.String(), 0)

		// Ensure Shuffled Questions are in Redis
		key := config.CacheKey.StudentShuffledQuestionKey(examID.String(), studentID)
		if s.rdb.Exists(ctx, key).Val() == 0 && len(existing.QuestionOrder) > 0 {
			orderJSON, _ := json.Marshal(existing.QuestionOrder)
			s.rdb.Set(ctx, key, orderJSON, 0)
		}

		return existing, nil
	}

	session := &model.ExamSession{
		ExamID:    examID,
		StudentID: studentID,
		// StartedAt will be set by the DB default NOW(), but we need it for Redis
		StartedAt: time.Now(),
	}

	// Try to create the session.
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Concurrent join detected
			existing, fetchErr := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
			if fetchErr != nil {
				return nil, fmt.Errorf("concurrent join detected, but fetch failed: %w", fetchErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("create session: %w", err)
	}

	// REDIS OPTIMIZATION: Store the Unix timestamp
	// Use session.StartedAt.Unix() to ensure DB and Redis are perfectly synced
	startKey := config.CacheKey.StudentExamSessionStartKey(session.ExamID.String(), session.StudentID)
	if err := s.rdb.Set(ctx, startKey, session.StartedAt.Unix(), 0).Err(); err != nil {
		// Log this error but don't fail the request. The Fallback in GetExamState will handle it.
		fmt.Printf("Warning: Failed to cache start time: %v\n", err)
	}

	// REDIS: Mark the active exam for this student
	_ = s.rdb.Set(ctx, config.CacheKey.StudentActiveExamKey(studentID), examID.String(), 0)

	// Initialize Shuffled Questions
	if err := s.initShuffledQuestions(ctx, exam, studentID); err != nil {
		fmt.Printf("Warning: Failed to init shuffled questions: %v\n", err)
	}

	return session, nil
}

// VerifyActiveSession checks that a student has an active (IN_PROGRESS) session
// for the given exam. Uses Redis first, falls back to PostgreSQL.
func (s *ExamSessionService) VerifyActiveSession(ctx context.Context, examID uuid.UUID, studentID int) error {
	// Fast path: check Redis active_exam key
	key := config.CacheKey.StudentActiveExamKey(studentID)
	val, err := s.rdb.Get(ctx, key).Result()
	if err == nil {
		// Cache hit — verify it matches the requested exam
		if val == examID.String() {
			return nil // Active session confirmed via Redis
		}
		// Different exam is active — not valid for this one
		return errors.New("no active session for this exam")
	}

	// Cache miss or error — fall back to DB
	sess, dbErr := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
	if dbErr != nil {
		return fmt.Errorf("no active session: %w", dbErr)
	}
	if sess.Status == model.SessionStatusCompleted {
		return errors.New("exam session is already completed")
	}

	// Self-heal: write back to Redis
	_ = s.rdb.Set(ctx, key, examID.String(), 0)
	return nil
}

// initShuffledQuestions generates, caches, and queues the persistence of shuffled questions
func (s *ExamSessionService) initShuffledQuestions(ctx context.Context, exam *model.Exam, studentID int) error {
	payloadKey := config.CacheKey.ExamPayloadKey(exam.ID.String())
	data, err := s.rdb.Get(ctx, payloadKey).Bytes()
	if err != nil {
		return fmt.Errorf("failed to get exam payload from redis: %w", err)
	}

	var payload model.ExamPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to parse exam payload: %w", err)
	}

	var qIDs []string
	for _, q := range payload.Questions {
		qIDs = append(qIDs, q.ID.String())
	}

	if exam.RandomizeQuestions {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(qIDs), func(i, j int) {
			qIDs[i], qIDs[j] = qIDs[j], qIDs[i]
		})
	}

	if exam.QuestionCount > 0 && exam.QuestionCount < len(qIDs) {
		qIDs = qIDs[:exam.QuestionCount]
	}

	shuffledKey := config.CacheKey.StudentShuffledQuestionKey(exam.ID.String(), studentID)
	orderJSON, err := json.Marshal(qIDs)
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, shuffledKey, orderJSON, 0)

	// Queue for async persistence
	workerPayload := map[string]interface{}{
		"exam_id":    exam.ID.String(),
		"student_id": studentID,
		"order":      qIDs,
	}
	workerPayloadJSON, _ := json.Marshal(workerPayload)
	pipe.RPush(ctx, config.WorkerKey.PersistQuestionOrderQueue, workerPayloadJSON)

	_, err = pipe.Exec(ctx)
	return err
}

// GetShuffledQuestionIDs retrieves the ordered question IDs for a student's exam session
func (s *ExamSessionService) GetShuffledQuestionIDs(ctx context.Context, examID uuid.UUID, studentID int) ([]string, error) {
	key := config.CacheKey.StudentShuffledQuestionKey(examID.String(), studentID)
	val, err := s.rdb.Get(ctx, key).Bytes()

	var qIDs []string
	if err == redis.Nil {
		// Cache miss, fallback to DB
		sess, dbErr := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
		if dbErr != nil {
			return nil, fmt.Errorf("failed to fetch session from DB: %w", dbErr)
		}
		qIDs = sess.QuestionOrder
		if len(qIDs) > 0 {
			orderJSON, _ := json.Marshal(qIDs)
			s.rdb.Set(ctx, key, orderJSON, 0)
		}
	} else if err != nil {
		return nil, fmt.Errorf("redis error: %w", err)
	} else {
		if err := json.Unmarshal(val, &qIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal shuffled keys: %w", err)
		}
	}

	return qIDs, nil
}

// GetExamState retrieves the current state of the exam for the student.
func (s *ExamSessionService) GetExamState(ctx context.Context, examID uuid.UUID, studentID int) (*model.ExamSessionState, error) {
	// 1. Get all related student answered question-answer from redis
	sessionKey := config.CacheKey.StudentAnswersKey(examID.String(), studentID)
	questionAnswers, err := s.rdb.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		return nil, fmt.Errorf("get question answers: %w", err)
	}

	// 2. Get Exam Duration
	durationStr, err := s.rdb.Get(ctx, config.CacheKey.ExamDurationKey(examID.String())).Result()
	if err != nil {
		return nil, fmt.Errorf("get exam duration: %w", err)
	}
	durationMinutes, err := strconv.Atoi(durationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format in redis: %w", err)
	}

	// 3. Get Session Start Time (With Failover Strategy)
	var startTimeUnix int64
	startKey := config.CacheKey.StudentExamSessionStartKey(examID.String(), studentID)

	val, err := s.rdb.Get(ctx, startKey).Result()

	if err == redis.Nil {
		// [CACHE MISS SCENARIO]
		// Redis doesn't have it (maybe evicted, or legacy session).
		// Fallback to PostgreSQL to get the source of truth.
		sess, dbErr := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
		if dbErr != nil {
			return nil, fmt.Errorf("session not found in cache or db: %w", dbErr)
		}

		startTimeUnix = sess.StartedAt.Unix()

		// Self-Heal: Put it back in Redis so the next request is fast
		_ = s.rdb.Set(ctx, startKey, startTimeUnix, 0)

	} else if err != nil {
		// Real Redis error (connection died, etc)
		return nil, fmt.Errorf("redis error getting start time: %w", err)
	} else {
		// [CACHE HIT SCENARIO]
		// Parse the string "169837482" into int64
		startTimeUnix, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid start time format in cache: %w", err)
		}
	}

	// 4. Calculate Remaining Time
	// Convert Unix Timestamp (int64) back to Time object
	startTime := time.Unix(startTimeUnix, 0)

	endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)
	remaining := time.Until(endTime)

	if remaining < 0 {
		remaining = 0
	}

	// 5. Get Cheat Rules
	var cheatRules map[string]bool
	res, err := s.rdb.Get(ctx, config.CacheKey.ExamCheatRulesKey(examID.String())).Bytes()
	if err != nil {
		return nil, fmt.Errorf("get cheat rules: %w", err)
	}
	if err := json.Unmarshal(res, &cheatRules); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cheat rules: %w", err)
	}

	// 6. Get Random Order Status
	isRandom, err := s.rdb.Get(ctx, config.CacheKey.ExamRandomOrderKey(examID.String())).Bool()
	if err != nil {
		isRandom = true
	}

	return &model.ExamSessionState{
		ExamID:           examID,
		StudentID:        studentID,
		IsRandomOrder:    isRandom,
		CheatRules:       cheatRules,
		AutosavedAnswers: questionAnswers,
		RemainingTime:    remaining.Seconds(),
	}, nil
}

// GetExamResults retrieves paginated exam results with optional filters.
func (s *ExamSessionService) GetExamResults(ctx context.Context, examID uuid.UUID, page, perPage int, classID *int, gradeLevel *string, majorCode *string, groupNumber *int, religion *string) ([]repository.ExamResult, int64, error) {
	return s.sessionRepo.ListByExam(ctx, examID, page, perPage, classID, gradeLevel, majorCode, groupNumber, religion)
}
