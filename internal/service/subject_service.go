package service

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

type SubjectService struct {
	subjectRepo *repository.SubjectRepository
	log         zerolog.Logger
}

func NewSubjectService(subjectRepo *repository.SubjectRepository, log zerolog.Logger) *SubjectService {
	return &SubjectService{
		subjectRepo: subjectRepo,
		log:         log.With().Str("component", "subject_service").Logger(),
	}
}

func (s *SubjectService) GetAll(ctx context.Context) ([]model.Subject, error) {
	return s.subjectRepo.GetAll(ctx)
}

func (s *SubjectService) Create(ctx context.Context, sub *model.Subject) error {
	return s.subjectRepo.Create(ctx, sub)
}

func (s *SubjectService) Update(ctx context.Context, sub *model.Subject) error {
	return s.subjectRepo.Update(ctx, sub)
}

func (s *SubjectService) Delete(ctx context.Context, id int) error {
	return s.subjectRepo.Delete(ctx, id)
}
