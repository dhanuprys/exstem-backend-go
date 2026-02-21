package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// ExamResult combines student data with their exam session details.
type ExamResult struct {
	StudentID  int                 `json:"student_id"`
	Name       string              `json:"name"`
	NISN       string              `json:"nisn"`
	ClassName  string              `json:"class_name"`
	FinalScore *float64            `json:"score"`
	Status     model.SessionStatus `json:"status"`
	StartedAt  *time.Time          `json:"started_at"`
	FinishedAt *time.Time          `json:"finished_at"`
}

// ExamSessionRepository handles exam session data access.
type ExamSessionRepository struct {
	pool *pgxpool.Pool
}

// NewExamSessionRepository creates a new ExamSessionRepository.
func NewExamSessionRepository(pool *pgxpool.Pool) *ExamSessionRepository {
	return &ExamSessionRepository{pool: pool}
}

// GetByExamAndStudent retrieves a session for a specific exam-student combination.
func (r *ExamSessionRepository) GetByExamAndStudent(ctx context.Context, examID uuid.UUID, studentID int) (*model.ExamSession, error) {
	s := &model.ExamSession{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, exam_id, student_id, started_at, finished_at, status, final_score
		 FROM exam_sessions
		 WHERE exam_id = $1 AND student_id = $2`, examID, studentID,
	).Scan(&s.ID, &s.ExamID, &s.StudentID, &s.StartedAt, &s.FinishedAt, &s.Status, &s.FinalScore)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Create inserts a new exam session (student joins the exam).
func (r *ExamSessionRepository) Create(ctx context.Context, s *model.ExamSession) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO exam_sessions (exam_id, student_id, status)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (exam_id, student_id) DO NOTHING
		 RETURNING id, started_at`,
		s.ExamID, s.StudentID, model.SessionStatusInProgress,
	).Scan(&s.ID, &s.StartedAt)
}

// Complete marks a session as completed with a final score.
func (r *ExamSessionRepository) Complete(ctx context.Context, examID uuid.UUID, studentID int, score float64) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE exam_sessions
		 SET status = $1, final_score = $2, finished_at = $3
		 WHERE exam_id = $4 AND student_id = $5`,
		model.SessionStatusCompleted, score, now, examID, studentID)
	return err
}

// ListByStudent retrieves all sessions for a given student.
func (r *ExamSessionRepository) ListByStudent(ctx context.Context, studentID int) ([]model.ExamSession, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, exam_id, student_id, started_at, finished_at, status, final_score
		 FROM exam_sessions
		 WHERE student_id = $1
		 ORDER BY started_at DESC`, studentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []model.ExamSession
	for rows.Next() {
		var s model.ExamSession
		if err := rows.Scan(&s.ID, &s.ExamID, &s.StudentID, &s.StartedAt, &s.FinishedAt, &s.Status, &s.FinalScore); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// ListByExam retrieves all student results for a specific exam, with optional filters and pagination.
func (r *ExamSessionRepository) ListByExam(ctx context.Context, examID uuid.UUID, page, perPage int, classID *int, gradeLevel *string, majorCode *string, groupNumber *int, religion *string) ([]ExamResult, int64, error) {
	offset := (page - 1) * perPage

	// Base query parts
	baseQuery := `
		FROM exam_sessions es
		JOIN students s ON es.student_id = s.id
		JOIN classes c ON s.class_id = c.id
		WHERE es.exam_id = $1
	`
	args := []any{examID}

	// Apply class filter if provided
	if classID != nil {
		args = append(args, *classID)
		baseQuery += fmt.Sprintf(" AND s.class_id = $%d", len(args))
	}

	// Apply optional filters
	if gradeLevel != nil && *gradeLevel != "" {
		args = append(args, *gradeLevel)
		baseQuery += fmt.Sprintf(" AND c.grade_level = $%d", len(args))
	}
	if majorCode != nil && *majorCode != "" {
		args = append(args, *majorCode)
		baseQuery += fmt.Sprintf(" AND c.major_code = $%d", len(args))
	}
	if groupNumber != nil {
		args = append(args, *groupNumber)
		baseQuery += fmt.Sprintf(" AND c.group_number = $%d", len(args))
	}
	if religion != nil && *religion != "" {
		args = append(args, *religion)
		baseQuery += fmt.Sprintf(" AND s.religion = $%d", len(args))
	}

	// Count total rows
	var total int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch paginated rows
	query := `
		SELECT 
			s.id, s.name, s.nisn, CONCAT(c.grade_level, ' ', c.major_code, ' ', c.group_number) as class_name,
			es.final_score, es.status, es.started_at, es.finished_at
		` + baseQuery + `
		ORDER BY class_name ASC, s.name ASC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(args)+2) + `
	`
	args = append(args, perPage, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []ExamResult
	for rows.Next() {
		var r ExamResult
		if err := rows.Scan(
			&r.StudentID, &r.Name, &r.NISN, &r.ClassName,
			&r.FinalScore, &r.Status, &r.StartedAt, &r.FinishedAt,
		); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}

	return results, total, nil
}
