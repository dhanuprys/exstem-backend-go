package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// MonitorRepository provides data access for the live exam monitoring feature.
// It combines PostgreSQL (session state) and Redis (live answer counts).
type MonitorRepository struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewMonitorRepository creates a new MonitorRepository.
func NewMonitorRepository(pool *pgxpool.Pool, rdb *redis.Client) *MonitorRepository {
	return &MonitorRepository{pool: pool, rdb: rdb}
}

// GetInProgressStudentIDs returns all student IDs with an active session for the given exam.
func (r *MonitorRepository) GetInProgressStudentIDs(ctx context.Context, examID uuid.UUID) ([]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT student_id FROM exam_sessions WHERE exam_id = $1 AND status = 'IN_PROGRESS'`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAnsweredCounts returns the count of answered questions for every student
// who has at least one answer recorded in the given exam.
func (r *MonitorRepository) GetAnsweredCounts(ctx context.Context, examID uuid.UUID) (map[int]int64, error) {
	result := make(map[int]int64)

	rows, err := r.pool.Query(ctx,
		`SELECT student_id, COUNT(*)
		 FROM student_answers
		 WHERE exam_id = $1
		 GROUP BY student_id`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sid int
		var count int64
		if err := rows.Scan(&sid, &count); err != nil {
			return nil, err
		}
		result[sid] = count
	}

	return result, rows.Err()
}

// GetCheatCounts returns the number of cheat events recorded for each student in the given exam.
func (r *MonitorRepository) GetCheatCounts(ctx context.Context, examID uuid.UUID) (map[int]int64, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT student_id, COUNT(*) 
		 FROM exam_cheats 
		 WHERE exam_id = $1 
		 GROUP BY student_id`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int]int64)
	for rows.Next() {
		var sid int
		var count int64
		if err := rows.Scan(&sid, &count); err != nil {
			return nil, err
		}
		counts[sid] = count
	}

	return counts, rows.Err()
}
