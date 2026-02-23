package model

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Question represents a single exam question.
type Question struct {
	ID            uuid.UUID       `json:"id"`
	QBankID       uuid.UUID       `json:"qbank_id"`
	QuestionText  string          `json:"question_text"`
	QuestionType  QuestionType    `json:"question_type"`
	Options       json.RawMessage `json:"options"`
	CorrectOption string          `json:"correct_option"`
	OrderNum      int             `json:"order_num"`
}

type QuestionType string

const (
	QuestionTypeMultipleChoice QuestionType = "MULTIPLE_CHOICE"
	QuestionTypeEssay          QuestionType = "ESSAY"
)

// AddQuestionRequest is the payload for adding a question to an exam.
type AddQuestionRequest struct {
	QuestionText  string          `json:"question_text" binding:"required,min=1,max=2000"`
	QuestionType  string          `json:"question_type" binding:"required,oneof=MULTIPLE_CHOICE ESSAY"`
	Options       json.RawMessage `json:"options" binding:"required"`
	CorrectOption string          `json:"correct_option" binding:"required,max=10"`
	OrderNum      int             `json:"order_num" binding:"min=0"`
}

// ReplaceQuestionsRequest is the payload for bulk replacing questions.
type ReplaceQuestionsRequest struct {
	Questions []AddQuestionRequest `json:"questions" binding:"dive"`
}
