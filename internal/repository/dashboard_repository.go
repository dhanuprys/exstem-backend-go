package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// DashboardRepository handles admin dashboard data access.
type DashboardRepository struct {
	pool *pgxpool.Pool
}

// NewDashboardRepository creates a new DashboardRepository.
func NewDashboardRepository(pool *pgxpool.Pool) *DashboardRepository {
	return &DashboardRepository{pool: pool}
}

// GetSummaryCounts retrieves the high-level metrics for the dashboard.
func (r *DashboardRepository) GetSummaryCounts(ctx context.Context) (totalStudents, totalExams, totalQBanks, totalQuestions int, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT 
			(SELECT COUNT(*) FROM students),
			(SELECT COUNT(*) FROM exams),
			(SELECT COUNT(*) FROM question_banks),
			(SELECT COUNT(*) FROM questions)`,
	).Scan(&totalStudents, &totalExams, &totalQBanks, &totalQuestions)
	return
}

// GetExamStatusCounts retrieves the distribution of exams by status.
func (r *DashboardRepository) GetExamStatusCounts(ctx context.Context) (map[model.ExamStatus]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT status, COUNT(*) FROM exams GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[model.ExamStatus]int)
	for rows.Next() {
		var status model.ExamStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// DashboardUpcomingExam represents minimal data for upcoming scheduled exams.
type DashboardUpcomingExam struct {
	ID             uuid.UUID  `json:"id"`
	Title          string     `json:"title"`
	ScheduledStart *time.Time `json:"scheduled_start"`
	Duration       int        `json:"duration_minutes"`
}

// GetUpcomingExams retrieves the next N scheduled exams that are PUBLISHED.
func (r *DashboardRepository) GetUpcomingExams(ctx context.Context, limit int) ([]DashboardUpcomingExam, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, scheduled_start, duration_minutes 
		 FROM exams 
		 WHERE status = $1 AND scheduled_start > NOW() 
		 ORDER BY scheduled_start ASC LIMIT $2`,
		model.ExamStatusPublished, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exams []DashboardUpcomingExam
	for rows.Next() {
		var e DashboardUpcomingExam
		if err := rows.Scan(&e.ID, &e.Title, &e.ScheduledStart, &e.Duration); err != nil {
			return nil, err
		}
		exams = append(exams, e)
	}
	if exams == nil {
		exams = []DashboardUpcomingExam{}
	}
	return exams, rows.Err()
}

// DashboardRecentExamResult represents minimal data for recently completed exams, averaging session results.
type DashboardRecentExamResult struct {
	ID               uuid.UUID  `json:"id"`
	Title            string     `json:"title"`
	EndDateTime      *time.Time `json:"end_date_time"` // scheduled_end or created_at fallback
	ParticipantCount int        `json:"participant_count"`
	AverageScore     *float64   `json:"average_score"`
}

// GetRecentExamResults retrieves the last N completed or archived exams with session completion stats.
func (r *DashboardRepository) GetRecentExamResults(ctx context.Context, limit int) ([]DashboardRecentExamResult, error) {
	query := `
		SELECT 
			e.id, 
			e.title, 
			COALESCE(e.scheduled_end, e.updated_at) as end_time,
			COUNT(s.id) as participant_count,
			AVG(s.final_score) as average_score
		FROM exams e
		LEFT JOIN exam_sessions s ON e.id = s.exam_id
		WHERE e.status IN ($1, $2)
		GROUP BY e.id, e.title, end_time
		ORDER BY end_time DESC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, query, model.ExamStatusCompleted, model.ExamStatusArchived, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DashboardRecentExamResult
	for rows.Next() {
		var r DashboardRecentExamResult
		if err := rows.Scan(&r.ID, &r.Title, &r.EndDateTime, &r.ParticipantCount, &r.AverageScore); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if results == nil {
		results = []DashboardRecentExamResult{}
	}
	return results, rows.Err()
}
