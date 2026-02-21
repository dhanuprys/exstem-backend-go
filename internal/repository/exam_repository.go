package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// ExamRepository handles exam data access.
type ExamRepository struct {
	pool *pgxpool.Pool
}

// NewExamRepository creates a new ExamRepository.
func NewExamRepository(pool *pgxpool.Pool) *ExamRepository {
	return &ExamRepository{pool: pool}
}

// GetByID retrieves an exam by its UUID.
func (r *ExamRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Exam, error) {
	e := &model.Exam{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, author_id, scheduled_start, scheduled_end,
		        duration_minutes, entry_token, status, created_at, updated_at
		 FROM exams WHERE id = $1`, id,
	).Scan(&e.ID, &e.Title, &e.AuthorID, &e.ScheduledStart, &e.ScheduledEnd,
		&e.DurationMinutes, &e.EntryToken, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// ListByAuthorPaginated retrieves exams filtered by author with pagination.
// Pass authorID=0 to list all exams (superadmin).
func (r *ExamRepository) ListByAuthorPaginated(ctx context.Context, authorID, limit, offset int) ([]model.Exam, int, error) {
	// 1. Get total count
	countQuery := `SELECT COUNT(*) FROM exams`
	var countArgs []interface{}
	if authorID > 0 {
		countQuery += ` WHERE author_id = $1`
		countArgs = append(countArgs, authorID)
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 2. Get paginated data
	query := `SELECT id, title, author_id, scheduled_start, scheduled_end,
	                  duration_minutes, entry_token, status, created_at, updated_at
	           FROM exams`
	var args []interface{}
	argIdx := 1

	if authorID > 0 {
		query += ` WHERE author_id = $1`
		args = append(args, authorID)
		argIdx++
	}

	query += ` ORDER BY created_at DESC LIMIT $` + formatInt(argIdx) + ` OFFSET $` + formatInt(argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var exams []model.Exam
	for rows.Next() {
		var e model.Exam
		if err := rows.Scan(&e.ID, &e.Title, &e.AuthorID, &e.ScheduledStart, &e.ScheduledEnd,
			&e.DurationMinutes, &e.EntryToken, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, err
		}
		exams = append(exams, e)
	}
	return exams, total, rows.Err()
}

func formatInt(n int) string {
	// simple helper safe for low numbers
	if n == 1 {
		return "1"
	}
	if n == 2 {
		return "2"
	}
	if n == 3 {
		return "3"
	}
	return "4"
}

// Create inserts a new exam.
func (r *ExamRepository) Create(ctx context.Context, e *model.Exam) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO exams (title, author_id, scheduled_start, scheduled_end, duration_minutes, entry_token, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		e.Title, e.AuthorID, e.ScheduledStart, e.ScheduledEnd,
		e.DurationMinutes, e.EntryToken, e.Status,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

// UpdateStatus updates an exam's status.
func (r *ExamRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.ExamStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exams SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

// ListPublished returns all exams with PUBLISHED status.
// Used for cache prewarming on application startup.
func (r *ExamRepository) ListPublished(ctx context.Context) ([]model.Exam, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, author_id, scheduled_start, scheduled_end,
		        duration_minutes, entry_token, status, created_at, updated_at
		 FROM exams WHERE status = $1
		 ORDER BY created_at DESC`, model.ExamStatusPublished)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exams []model.Exam
	for rows.Next() {
		var e model.Exam
		if err := rows.Scan(&e.ID, &e.Title, &e.AuthorID, &e.ScheduledStart, &e.ScheduledEnd,
			&e.DurationMinutes, &e.EntryToken, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		exams = append(exams, e)
	}
	return exams, rows.Err()
}
