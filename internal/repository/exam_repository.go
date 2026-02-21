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
		`SELECT e.id, e.title, e.author_id, e.subject_id, COALESCE(s.name, ''), e.scheduled_start, e.scheduled_end,
		        e.duration_minutes, e.entry_token, e.status, e.created_at, e.updated_at
		 FROM exams e
		 LEFT JOIN subjects s ON e.subject_id = s.id
		 WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.Title, &e.AuthorID, &e.SubjectID, &e.SubjectName, &e.ScheduledStart, &e.ScheduledEnd,
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
	query := `SELECT e.id, e.title, e.author_id, e.subject_id, COALESCE(s.name, ''), e.scheduled_start, e.scheduled_end,
	                  e.duration_minutes, e.entry_token, e.status, e.created_at, e.updated_at
	           FROM exams e
	           LEFT JOIN subjects s ON e.subject_id = s.id`
	var args []interface{}
	argIdx := 1

	if authorID > 0 {
		query += ` WHERE e.author_id = $1`
		args = append(args, authorID)
		argIdx++
	}

	query += ` ORDER BY e.created_at DESC LIMIT $` + formatInt(argIdx) + ` OFFSET $` + formatInt(argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var exams []model.Exam
	for rows.Next() {
		var e model.Exam
		if err := rows.Scan(&e.ID, &e.Title, &e.AuthorID, &e.SubjectID, &e.SubjectName, &e.ScheduledStart, &e.ScheduledEnd,
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
		`INSERT INTO exams (title, author_id, subject_id, scheduled_start, scheduled_end, duration_minutes, entry_token, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at, updated_at`,
		e.Title, e.AuthorID, e.SubjectID, e.ScheduledStart, e.ScheduledEnd,
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
		`SELECT e.id, e.title, e.author_id, e.subject_id, COALESCE(s.name, ''), e.scheduled_start, e.scheduled_end,
		        e.duration_minutes, e.entry_token, e.status, e.created_at, e.updated_at
		 FROM exams e
		 LEFT JOIN subjects s ON e.subject_id = s.id
		 WHERE e.status = $1
		 ORDER BY e.created_at DESC`, model.ExamStatusPublished)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exams []model.Exam
	for rows.Next() {
		var e model.Exam
		if err := rows.Scan(&e.ID, &e.Title, &e.AuthorID, &e.SubjectID, &e.SubjectName, &e.ScheduledStart, &e.ScheduledEnd,
			&e.DurationMinutes, &e.EntryToken, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		exams = append(exams, e)
	}
	return exams, rows.Err()
}

// Update modifies an existing exam's metadata.
func (r *ExamRepository) Update(ctx context.Context, e *model.Exam) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exams SET title = $1, subject_id = $2, scheduled_start = $3, scheduled_end = $4,
        duration_minutes = $5, entry_token = $6, updated_at = NOW()
 WHERE id = $7`,
		e.Title, e.SubjectID, e.ScheduledStart, e.ScheduledEnd, e.DurationMinutes, e.EntryToken, e.ID)
	return err
}

// Delete removes an exam.
func (r *ExamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM exams WHERE id = $1`, id)
	return err
}
