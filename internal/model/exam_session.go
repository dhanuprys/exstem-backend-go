package model

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus enumerates exam session states.
type SessionStatus string

const (
	SessionStatusInProgress SessionStatus = "IN_PROGRESS"
	SessionStatusCompleted  SessionStatus = "COMPLETED"
)

// ExamSession represents a student's exam attempt.
type ExamSession struct {
	ID         uuid.UUID     `json:"id"`
	ExamID     uuid.UUID     `json:"exam_id"`
	StudentID  int           `json:"student_id"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Status     SessionStatus `json:"status"`
	FinalScore *float64      `json:"final_score,omitempty"`
}

// JoinExamRequest is the payload for a student joining an exam.
type JoinExamRequest struct {
	EntryToken string `json:"entry_token" binding:"required,min=4,max=20"`
}
