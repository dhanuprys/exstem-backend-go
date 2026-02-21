package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// QuestionService handles question business logic.
type QuestionService struct {
	questionRepo *repository.QuestionRepository
}

// NewQuestionService creates a new QuestionService.
func NewQuestionService(questionRepo *repository.QuestionRepository) *QuestionService {
	return &QuestionService{questionRepo: questionRepo}
}

// ListByExam retrieves all questions for an exam.
func (s *QuestionService) ListByExam(ctx context.Context, examID uuid.UUID) ([]model.Question, error) {
	return s.questionRepo.ListByExam(ctx, examID)
}

// Create adds a question to an exam.
func (s *QuestionService) Create(ctx context.Context, question *model.Question) error {
	return s.questionRepo.Create(ctx, question)
}
