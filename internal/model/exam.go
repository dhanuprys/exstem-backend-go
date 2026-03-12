package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LocalTime isolates Postgres and UI clock formats securely dropping arbitrary offsets natively across JSON parsing bounds.
type LocalTime time.Time

// TimeLayout statically defines standard HTML datetime-local formats organically matching DB inputs.
const TimeLayout = "2006-01-02T15:04:05"
const TimeLayoutShort = "2006-01-02T15:04"

func (lt *LocalTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		return nil
	}
	// Try parsing standard seconds first within local physical location
	t, err := time.ParseInLocation(TimeLayout, s, time.Local)
	if err != nil {
		// Try parsing without seconds conditionally matching frontend outputs
		t, err = time.ParseInLocation(TimeLayoutShort, s, time.Local)
		if err != nil {
			return fmt.Errorf("invalid LocalTime format: %s", s)
		}
	}
	*lt = LocalTime(t)
	return nil
}

func (lt LocalTime) MarshalJSON() ([]byte, error) {
	t := time.Time(lt)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, t.Format(TimeLayout))), nil
}

// Time returns the underlying time.Time element directly.
func (lt LocalTime) Time() time.Time {
	return time.Time(lt)
}

// Value implements the driver.Valuer interface allowing pgx compatibility safely capturing nil pointers natively.
func (lt *LocalTime) Value() (driver.Value, error) {
	if lt == nil {
		return nil, nil
	}
	t := time.Time(*lt)
	if t.IsZero() {
		return nil, nil
	}
	return t, nil
}

// Scan implements the sql.Scanner interface decoding native Postgres times into the aliased type.
func (lt *LocalTime) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		// PGX captures TIMESTAMP WITHOUT TIMEZONE physically inside +0000 UTC locations.
		// We securely discard this default metadata wrapping the digits onto the actual Local machine timezone footprint natively bypassing offset jumps.
		localT := time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), time.Local)
		*lt = LocalTime(localT)
		return nil
	default:
		return fmt.Errorf("cannot scan type %T into LocalTime", value)
	}
}

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
	ScheduledStart     *LocalTime      `json:"scheduled_start,omitempty"`
	ScheduledEnd       *LocalTime      `json:"scheduled_end,omitempty"`
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
	ScheduledStart  *LocalTime `json:"scheduled_start" binding:"omitempty"`
	ScheduledEnd    *LocalTime `json:"scheduled_end" binding:"omitempty"` // gtfield handled in handler manually due to custom type
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
	ScheduledStart     *LocalTime      `json:"scheduled_start" binding:"omitempty"`
	ScheduledEnd       *LocalTime      `json:"scheduled_end" binding:"omitempty"` // gtfield handled in handler natively
	DurationMinutes    int             `json:"duration_minutes" binding:"omitempty,min=1,max=480"`
	CheatRules         json.RawMessage `json:"cheat_rules" binding:"omitempty"`
	RandomizeQuestions *bool           `json:"randomize_questions" binding:"omitempty"`
	QuestionCount      *int            `json:"question_count" binding:"omitempty"`
	EntryToken         string          `json:"entry_token" binding:"omitempty,min=4,max=20"`
	QBankID            *uuid.UUID      `json:"qbank_id" binding:"omitempty"`
}
