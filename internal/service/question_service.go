package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/response"
)

// QuestionService handles question business logic.
type QuestionService struct {
	questionRepo *repository.QuestionRepository
}

// NewQuestionService creates a new QuestionService.
func NewQuestionService(questionRepo *repository.QuestionRepository) *QuestionService {
	return &QuestionService{questionRepo: questionRepo}
}

// ListQBanks retrieves question banks with pagination.
// If authorID is non-nil, only banks owned by that author are returned.
func (s *QuestionService) ListQBanks(ctx context.Context, authorID *int, page, perPage int, search string) ([]model.QuestionBank, *response.Pagination, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}

	limit := perPage
	offset := (page - 1) * perPage

	var qbanks []model.QuestionBank
	var total int
	var err error

	if authorID != nil {
		qbanks, total, err = s.questionRepo.ListQBanksByAuthor(ctx, *authorID, limit, offset, search)
	} else {
		qbanks, total, err = s.questionRepo.ListQBanks(ctx, limit, offset, search)
	}
	if err != nil {
		return nil, nil, err
	}

	if qbanks == nil {
		qbanks = []model.QuestionBank{}
	}

	totalPages := (total + perPage - 1) / perPage

	pagination := &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: totalPages,
	}

	return qbanks, pagination, nil
}

// GetQBanks retrieves a specific question bank.
func (s *QuestionService) GetQBanks(ctx context.Context, qbankID uuid.UUID) (*model.QuestionBank, error) {
	return s.questionRepo.GetQBanks(ctx, qbankID)
}

// CreateQBanks creates a new question bank.
func (s *QuestionService) CreateQBanks(ctx context.Context, qbank *model.QuestionBank) error {
	return s.questionRepo.CreateQBanks(ctx, qbank)
}

// UpdateQBanks updates a specific question bank.
func (s *QuestionService) UpdateQBanks(ctx context.Context, qbank *model.QuestionBank) error {
	return s.questionRepo.UpdateQBanks(ctx, qbank)
}

// DeleteQBanks deletes a specific question bank.
func (s *QuestionService) DeleteQBanks(ctx context.Context, qbankID uuid.UUID) error {
	return s.questionRepo.DeleteQBanks(ctx, qbankID)
}

// ListByQBank retrieves all questions for an qbank.
func (s *QuestionService) ListByQBank(ctx context.Context, qbankID uuid.UUID) ([]model.Question, error) {
	return s.questionRepo.ListByQBank(ctx, qbankID)
}

// Create adds a question to an qbank.
func (s *QuestionService) Create(ctx context.Context, question *model.Question) error {
	return s.questionRepo.Create(ctx, question)
}

// ReplaceAll replaces all questions for an qbank
func (s *QuestionService) ReplaceAll(ctx context.Context, qBankID uuid.UUID, questions []model.Question) error {
	for i := range questions {
		questions[i].QBankID = qBankID
	}
	return s.questionRepo.ReplaceAll(ctx, qBankID, questions)
}
