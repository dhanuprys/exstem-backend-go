package model

import (
	"time"

	"github.com/google/uuid"
)

// QuestionBank represents a collection of questions.
type QuestionBank struct {
	ID          uuid.UUID `json:"id"`
	AuthorID    *int      `json:"author_id,omitempty"`
	SubjectID   *int      `json:"subject_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateQuestionBankRequest struct {
	Name        string `json:"name" binding:"required,min=3,max=255"`
	Description string `json:"description" binding:"omitempty"`
	SubjectID   *int   `json:"subject_id" binding:"omitempty"`
}
