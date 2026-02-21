package service

import (
	"context"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/response"
	"golang.org/x/crypto/bcrypt"
)

// StudentService handles student business logic.
type StudentService struct {
	studentRepo *repository.StudentRepository
}

// NewStudentService creates a new StudentService.
func NewStudentService(studentRepo *repository.StudentRepository) *StudentService {
	return &StudentService{studentRepo: studentRepo}
}

// GetByNISN retrieves a student by their NISN.
func (s *StudentService) GetByNISN(ctx context.Context, nisn string) (*model.Student, error) {
	return s.studentRepo.GetByNISN(ctx, nisn)
}

// GetByID retrieves a student by ID.
func (s *StudentService) GetByID(ctx context.Context, id int) (*model.Student, error) {
	return s.studentRepo.GetByID(ctx, id)
}

// ListStudents retrieves all students with pagination and optional class filter.
func (s *StudentService) ListStudents(ctx context.Context, classID *int, page, perPage int) ([]model.Student, *response.Pagination, error) {
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

	students, total, err := s.studentRepo.ListPaginated(ctx, classID, limit, offset)
	if err != nil {
		return nil, nil, err
	}

	if students == nil {
		students = []model.Student{}
	}

	totalPages := (total + perPage - 1) / perPage

	pagination := &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: totalPages,
	}

	return students, pagination, nil
}

// Create inserts a new student with a hashed password.
func (s *StudentService) Create(ctx context.Context, student *model.Student) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(student.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	student.PasswordHash = string(hashed)
	return s.studentRepo.Create(ctx, student)
}

// Update modifies a student's details. Updates password if provided.
func (s *StudentService) Update(ctx context.Context, student *model.Student, updatePassword bool) error {
	// 1. Update basic info
	if err := s.studentRepo.Update(ctx, student); err != nil {
		return err
	}

	// 2. Update password if requested
	if updatePassword && student.PasswordHash != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(student.PasswordHash), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		return s.studentRepo.UpdatePassword(ctx, student.ID, string(hashed))
	}

	return nil
}

// Delete removes a student by ID.
func (s *StudentService) Delete(ctx context.Context, id int) error {
	return s.studentRepo.Delete(ctx, id)
}
