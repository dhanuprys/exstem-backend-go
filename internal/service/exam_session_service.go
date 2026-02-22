package service

import (
	"context"
	"errors"
	"fmt"
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
)

// LobbyExam represents an exam as displayed in the student lobby.
type LobbyExam struct {
	model.Exam
	LobbyStatus   LobbyStatus          `json:"lobby_status"`
	SessionStatus *model.SessionStatus `json:"session_status,omitempty"`
	FinalScore    *float64             `json:"final_score,omitempty"`
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

		entry := LobbyExam{Exam: *exam}

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
			if exam.ScheduledStart != nil && exam.ScheduledStart.After(now) {
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

	return session, nil
}

// VerifyActiveSession checks that a student has an active (IN_PROGRESS) session
// for the given exam. Returns an error if no session exists or it is COMPLETED.
func (s *ExamSessionService) VerifyActiveSession(ctx context.Context, examID uuid.UUID, studentID int) error {
	sess, err := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}
	if sess.Status == model.SessionStatusCompleted {
		return errors.New("exam session is already completed")
	}
	return nil
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

	return &model.ExamSessionState{
		ExamID:           examID,
		StudentID:        studentID,
		AutosavedAnswers: questionAnswers,
		RemainingTime:    remaining.Seconds(),
	}, nil
}

// GetExamResults retrieves paginated exam results with optional filters.
func (s *ExamSessionService) GetExamResults(ctx context.Context, examID uuid.UUID, page, perPage int, classID *int, gradeLevel *string, majorCode *string, groupNumber *int, religion *string) ([]repository.ExamResult, int64, error) {
	return s.sessionRepo.ListByExam(ctx, examID, page, perPage, classID, gradeLevel, majorCode, groupNumber, religion)
}
