package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// ExamSessionService handles exam session business logic.
type ExamSessionService struct {
	sessionRepo *repository.ExamSessionRepository
	examRepo    *repository.ExamRepository
	targetRepo  *repository.ExamTargetRuleRepository
}

// NewExamSessionService creates a new ExamSessionService.
func NewExamSessionService(
	sessionRepo *repository.ExamSessionRepository,
	examRepo *repository.ExamRepository,
	targetRepo *repository.ExamTargetRuleRepository,
) *ExamSessionService {
	return &ExamSessionService{
		sessionRepo: sessionRepo,
		examRepo:    examRepo,
		targetRepo:  targetRepo,
	}
}

// LobbyExam represents an exam as displayed in the student lobby.
type LobbyExam struct {
	model.Exam
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
		if sess, ok := sessionMap[eid]; ok {
			entry.SessionStatus = &sess.Status
			entry.FinalScore = sess.FinalScore
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
	if existing != nil {
		return existing, nil // Return existing session (idempotent join)
	}

	session := &model.ExamSession{
		ExamID:    examID,
		StudentID: studentID,
	}
	// Try to create the session.
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		// If Create returns ErrNoRows, it means ON CONFLICT DO NOTHING was triggered.
		// This implies a concurrent request created the session.
		if errors.Is(err, pgx.ErrNoRows) {
			existing, fetchErr := s.sessionRepo.GetByExamAndStudent(ctx, examID, studentID)
			if fetchErr != nil {
				return nil, fmt.Errorf("concurrent join detected, but fetch failed: %w", fetchErr)
			}
			return existing, nil
		}
		return nil, fmt.Errorf("create session: %w", err)
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

// GetExamResults retrieves paginated exam results with optional filters.
func (s *ExamSessionService) GetExamResults(ctx context.Context, examID uuid.UUID, page, perPage int, classID *int, gradeLevel *string, majorCode *string, groupNumber *int, religion *string) ([]repository.ExamResult, int64, error) {
	return s.sessionRepo.ListByExam(ctx, examID, page, perPage, classID, gradeLevel, majorCode, groupNumber, religion)
}
