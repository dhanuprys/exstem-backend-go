package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/response"
)

// Domain Errors
var (
	ErrNotExamAuthor    = errors.New("not the author of this exam")
	ErrNoQuestions      = errors.New("exam has no questions, cannot publish/start")
	ErrExamNotDraft     = errors.New("exam status is not DRAFT")
	ErrExamNotPublished = errors.New("exam status is not PUBLISHED")
)

// ExamService handles exam business logic and Redis caching.
type ExamService struct {
	examRepo     *repository.ExamRepository
	questionRepo *repository.QuestionRepository
	targetRepo   *repository.ExamTargetRuleRepository
	rdb          *redis.Client
	log          zerolog.Logger
}

// NewExamService creates a new ExamService.
func NewExamService(
	examRepo *repository.ExamRepository,
	questionRepo *repository.QuestionRepository,
	targetRepo *repository.ExamTargetRuleRepository,
	rdb *redis.Client,
	log zerolog.Logger,
) *ExamService {
	return &ExamService{
		examRepo:     examRepo,
		questionRepo: questionRepo,
		targetRepo:   targetRepo,
		rdb:          rdb,
		log:          log.With().Str("component", "exam_service").Logger(),
	}
}

// GetByID retrieves an exam by its UUID.
func (s *ExamService) GetByID(ctx context.Context, id uuid.UUID) (*model.Exam, error) {
	return s.examRepo.GetByID(ctx, id)
}

// ListByAuthor retrieves exams, filtered by author if not superadmin.
func (s *ExamService) ListByAuthor(ctx context.Context, authorID, page, perPage int) ([]model.Exam, *response.Pagination, error) {
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

	exams, total, err := s.examRepo.ListByAuthorPaginated(ctx, authorID, limit, offset)
	if err != nil {
		return nil, nil, err
	}

	if exams == nil {
		exams = []model.Exam{}
	}

	totalPages := (total + perPage - 1) / perPage

	pagination := &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: totalPages,
	}

	return exams, pagination, nil
}

// Create inserts a new exam as DRAFT.
func (s *ExamService) Create(ctx context.Context, exam *model.Exam) error {
	exam.Status = model.ExamStatusDraft
	return s.examRepo.Create(ctx, exam)
}

// Publish changes exam status to PUBLISHED and caches the payload + answer key in Redis.
// This is the critical path that populates the "Fast Lane".
func (s *ExamService) Publish(ctx context.Context, examID uuid.UUID, authorID int) error {
	exam, err := s.examRepo.GetByID(ctx, examID)
	if err != nil {
		return fmt.Errorf("get exam: %w", err)
	}

	if exam.AuthorID != authorID {
		return errors.New("not the author of this exam")
	}
	if exam.Status != model.ExamStatusDraft {
		return fmt.Errorf("exam status is %s, expected DRAFT", exam.Status)
	}

	// Prewarm cache for this exam.
	if err := s.WarmExamCache(ctx, exam); err != nil {
		return err
	}

	// Update status in PostgreSQL.
	if err := s.examRepo.UpdateStatus(ctx, examID, model.ExamStatusPublished); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	s.log.Info().Str("exam_id", examID.String()).Msg("Exam published")
	return nil
}

// RefreshCache re-caches the payload + answer key for a published exam.
// Called when questions are updated after publish.
func (s *ExamService) RefreshCache(ctx context.Context, examID uuid.UUID, authorID int) error {
	exam, err := s.examRepo.GetByID(ctx, examID)
	if err != nil {
		return fmt.Errorf("get exam: %w", err)
	}

	if authorID != 0 && exam.AuthorID != authorID {
		return ErrNotExamAuthor
	}
	if exam.Status != model.ExamStatusPublished {
		return ErrExamNotPublished
	}

	if err := s.WarmExamCache(ctx, exam); err != nil {
		return err
	}

	s.log.Info().Str("exam_id", examID.String()).Msg("Cache refreshed")
	return nil
}

// WarmExamCache loads an exam's payload and answer key from PostgreSQL into Redis.
// This is the core cache-warming logic used by Publish, RefreshCache, and PrewarmAllCaches.
func (s *ExamService) WarmExamCache(ctx context.Context, exam *model.Exam) error {
	questions, err := s.questionRepo.ListByExam(ctx, exam.ID)
	if err != nil {
		return fmt.Errorf("list questions: %w", err)
	}
	if len(questions) == 0 {
		return ErrNoQuestions
	}

	// Build student-facing payload (without correct answers).
	studentQuestions := make([]model.QuestionForStudent, len(questions))
	for i, q := range questions {
		studentQuestions[i] = model.QuestionForStudent{
			ID:           q.ID,
			QuestionText: q.QuestionText,
			Options:      q.Options,
			OrderNum:     q.OrderNum,
		}
	}

	payload := model.ExamPayload{
		ExamID:    exam.ID,
		Title:     exam.Title,
		Duration:  exam.DurationMinutes,
		Questions: studentQuestions,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Build answer key map for RAM grading.
	answerKey := make(map[string]interface{}, len(questions))
	for _, q := range questions {
		answerKey[q.ID.String()] = q.CorrectOption
	}

	// Cache both atomically via pipeline.
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, fmt.Sprintf("exam:%s:payload", exam.ID), payloadJSON, 0)
	pipe.Del(ctx, fmt.Sprintf("exam:%s:key", exam.ID))
	pipe.HSet(ctx, fmt.Sprintf("exam:%s:key", exam.ID), answerKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache to redis: %w", err)
	}

	s.log.Debug().
		Str("exam_id", exam.ID.String()).
		Int("questions", len(questions)).
		Msg("Cache warmed")
	return nil
}

// PrewarmAllCaches loads all published exams into Redis on application startup.
// This prevents any lazy-loading race conditions under thundering herd traffic.
func (s *ExamService) PrewarmAllCaches(ctx context.Context) error {
	exams, err := s.examRepo.ListPublished(ctx)
	if err != nil {
		return fmt.Errorf("list published exams: %w", err)
	}

	if len(exams) == 0 {
		s.log.Info().Msg("No published exams to prewarm")
		return nil
	}

	s.log.Info().Int("count", len(exams)).Msg("Prewarming published exams...")

	warmed := 0
	for i := range exams {
		if err := s.WarmExamCache(ctx, &exams[i]); err != nil {
			s.log.Warn().
				Err(err).
				Str("exam_id", exams[i].ID.String()).
				Msg("Failed to warm exam, skipping")
			continue
		}
		warmed++
	}

	s.log.Info().
		Int("warmed", warmed).
		Int("total", len(exams)).
		Msg("Prewarming complete")
	return nil
}

// GetExamPayload retrieves the cached student payload from Redis.
func (s *ExamService) GetExamPayload(ctx context.Context, examID uuid.UUID) (*model.ExamPayload, error) {
	key := fmt.Sprintf("exam:%s:payload", examID)
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("exam not published or payload not cached")
		}
		return nil, fmt.Errorf("get payload: %w", err)
	}

	var payload model.ExamPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &payload, nil
}

// GetAnswerKey retrieves the answer key from Redis for instant grading.
func (s *ExamService) GetAnswerKey(ctx context.Context, examID uuid.UUID) (map[string]string, error) {
	key := fmt.Sprintf("exam:%s:key", examID)
	result, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("get answer key: %w", err)
	}
	if len(result) == 0 {
		return nil, errors.New("answer key not found in cache")
	}
	return result, nil
}

// AddTargetRule adds a target rule to an exam.
func (s *ExamService) AddTargetRule(ctx context.Context, rule *model.ExamTargetRule) error {
	return s.targetRepo.Create(ctx, rule)
}

// GetTargetRules retrieves target rules for an exam.
func (s *ExamService) GetTargetRules(ctx context.Context, examID uuid.UUID) ([]model.ExamTargetRule, error) {
	return s.targetRepo.ListByExam(ctx, examID)
}

// Update modifies an existing draft exam.
func (s *ExamService) Update(ctx context.Context, authorID int, exam *model.Exam) error {
	existing, err := s.examRepo.GetByID(ctx, exam.ID)
	if err != nil {
		return err
	}
	if authorID != 0 && existing.AuthorID != authorID {
		return ErrNotExamAuthor
	}
	if existing.Status != model.ExamStatusDraft {
		return ErrExamNotDraft
	}
	return s.examRepo.Update(ctx, exam)
}

// Delete removes a draft exam.
func (s *ExamService) Delete(ctx context.Context, id uuid.UUID, authorID int) error {
	existing, err := s.examRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if authorID != 0 && existing.AuthorID != authorID {
		return ErrNotExamAuthor
	}
	if existing.Status != model.ExamStatusDraft {
		return ErrExamNotDraft
	}
	return s.examRepo.Delete(ctx, id)
}
