package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ExamStatus enumerates the possible states of an exam.
type ExamStatus string

const (
	ExamStatusDraft      ExamStatus = "DRAFT"
	ExamStatusPublished  ExamStatus = "PUBLISHED"
	ExamStatusInProgress ExamStatus = "IN_PROGRESS"
	ExamStatusCompleted  ExamStatus = "COMPLETED"
	ExamStatusArchived   ExamStatus = "ARCHIVED"
)

// Exam represents an exam entity.
type Exam struct {
	ID                 uuid.UUID       `json:"id"`
	Title              string          `json:"title"`
	AuthorID           int             `json:"author_id"`
	ScheduledStart     *time.Time      `json:"scheduled_start,omitempty"`
	ScheduledEnd       *time.Time      `json:"scheduled_end,omitempty"`
	DurationMinutes    int             `json:"duration_minutes"`
	EntryToken         string          `json:"entry_token,omitempty"`
	CheatRules         json.RawMessage `json:"cheat_rules"`
	QuestionCount      int             `json:"question_count"`
	RandomizeQuestions bool            `json:"randomize_questions"`
	QBankID            *uuid.UUID      `json:"qbank_id,omitempty"`
	Status             ExamStatus      `json:"status"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// CreateExamRequest is the payload for creating a new exam.
type CreateExamRequest struct {
	Title           string     `json:"title" binding:"required,min=3,max=255"`
	ScheduledStart  *time.Time `json:"scheduled_start" binding:"omitempty"`
	ScheduledEnd    *time.Time `json:"scheduled_end" binding:"omitempty,gtfield=ScheduledStart"`
	DurationMinutes int        `json:"duration_minutes" binding:"required,min=1,max=480"`
	EntryToken      string     `json:"entry_token" binding:"omitempty,min=4,max=20"`
}

// ExamPayload is the Redis-cached payload sent to students (no correct answers).
type ExamPayload struct {
	ExamID    uuid.UUID            `json:"exam_id"`
	Title     string               `json:"title"`
	Duration  int                  `json:"duration_minutes"`
	Questions []QuestionForStudent `json:"questions"`
}

// QuestionForStudent is a question without the correct answer, sent to students.
type QuestionForStudent struct {
	ID           uuid.UUID       `json:"id"`
	QuestionText string          `json:"question_text"`
	Options      json.RawMessage `json:"options"`
	OrderNum     int             `json:"order_num"`
}

// UpdateExamRequest is the payload for updating an existing exam.
type UpdateExamRequest struct {
	Title              string          `json:"title" binding:"omitempty,min=3,max=255"`
	ScheduledStart     *time.Time      `json:"scheduled_start" binding:"omitempty"`
	ScheduledEnd       *time.Time      `json:"scheduled_end" binding:"omitempty,gtfield=ScheduledStart"`
	DurationMinutes    int             `json:"duration_minutes" binding:"omitempty,min=1,max=480"`
	CheatRules         json.RawMessage `json:"cheat_rules" binding:"omitempty"`
	RandomizeQuestions *bool           `json:"randomize_questions" binding:"omitempty"`
	QuestionCount      *int            `json:"question_count" binding:"omitempty"`
	EntryToken         string          `json:"entry_token" binding:"omitempty,min=4,max=20"`
	QBankID            *uuid.UUID      `json:"qbank_id" binding:"omitempty"`
}
