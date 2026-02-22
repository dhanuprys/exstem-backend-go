package service

import (
	"context"
	"errors"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

type MajorService interface {
	GetAllMajors(ctx context.Context) ([]*model.Major, error)
	CreateMajor(ctx context.Context, code, longName string) (*model.Major, error)
	UpdateMajor(ctx context.Context, id int, code, longName string) (*model.Major, error)
	DeleteMajor(ctx context.Context, id int) error
}

type majorService struct {
	majorRepo repository.MajorRepository
}

func NewMajorService(majorRepo repository.MajorRepository) MajorService {
	return &majorService{majorRepo: majorRepo}
}

func (s *majorService) GetAllMajors(ctx context.Context) ([]*model.Major, error) {
	return s.majorRepo.GetAll(ctx)
}

func (s *majorService) CreateMajor(ctx context.Context, code, longName string) (*model.Major, error) {
	if code == "" || longName == "" {
		return nil, errors.New("code and long_name are required")
	}

	existing, _ := s.majorRepo.GetByCode(ctx, code)
	if existing != nil {
		return nil, errors.New("major code already exists")
	}

	major := &model.Major{
		Code:     code,
		LongName: longName,
	}

	if err := s.majorRepo.Create(ctx, major); err != nil {
		return nil, err
	}
	return major, nil
}

func (s *majorService) UpdateMajor(ctx context.Context, id int, code, longName string) (*model.Major, error) {
	major, err := s.majorRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("major not found")
	}

	if code != "" {
		// Check uniqueness if changing code
		if code != major.Code {
			existing, _ := s.majorRepo.GetByCode(ctx, code)
			if existing != nil && existing.ID != id {
				return nil, errors.New("major code already exists")
			}
		}
		major.Code = code
	}
	if longName != "" {
		major.LongName = longName
	}

	if err := s.majorRepo.Update(ctx, major); err != nil {
		return nil, err
	}
	return major, nil
}

func (s *majorService) DeleteMajor(ctx context.Context, id int) error {
	_, err := s.majorRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("major not found")
	}
	// Depending on requirements, we do not enforce cascading FK drop to Students or Classes.
	// We just delete it from the dictionary.
	return s.majorRepo.Delete(ctx, id)
}
