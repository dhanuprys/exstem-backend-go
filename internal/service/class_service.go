package service

import (
	"context"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// ClassService handles class business logic.
type ClassService struct {
	classRepo *repository.ClassRepository
}

// NewClassService creates a new ClassService.
func NewClassService(classRepo *repository.ClassRepository) *ClassService {
	return &ClassService{classRepo: classRepo}
}

// GetByID retrieves a class by its ID.
func (s *ClassService) GetByID(ctx context.Context, id int) (*model.Class, error) {
	return s.classRepo.GetByID(ctx, id)
}

// List retrieves all classes.
func (s *ClassService) List(ctx context.Context) ([]model.Class, error) {
	return s.classRepo.List(ctx)
}

// Create creates a new class.
func (s *ClassService) Create(ctx context.Context, class *model.Class) error {
	// Add potential uniqueness validations here if required (e.g. combination of grade, major, group)
	return s.classRepo.Create(ctx, class)
}

// Update modifies an existing class.
func (s *ClassService) Update(ctx context.Context, class *model.Class) error {
	return s.classRepo.Update(ctx, class)
}

// Delete removes a class.
func (s *ClassService) Delete(ctx context.Context, id int) error {
	// Note: Foreign key constraints on the `students` table will correctly prevent
	// deletion if students are assigned to this class. The handler uses this error structure.
	return s.classRepo.Delete(ctx, id)
}
