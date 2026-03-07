package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// ExamScheduleRepository handles DB ops for exam_schedules and exam_room_assignments.
type ExamScheduleRepository struct {
	pool *pgxpool.Pool
}

// NewExamScheduleRepository creates a new ExamScheduleRepository.
func NewExamScheduleRepository(pool *pgxpool.Pool) *ExamScheduleRepository {
	return &ExamScheduleRepository{pool: pool}
}

// ListByExam retrieves all schedules for an exam, including room details.
func (r *ExamScheduleRepository) ListByExam(ctx context.Context, examID uuid.UUID) ([]model.ExamSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.exam_id, s.session_number, s.room_id, s.start_time, s.end_time, s.created_at,
		        rm.name, rm.capacity
		 FROM exam_schedules s
		 JOIN rooms rm ON s.room_id = rm.id
		 WHERE s.exam_id = $1
		 ORDER BY s.session_number ASC, rm.name ASC`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []model.ExamSchedule
	for rows.Next() {
		var s model.ExamSchedule
		if err := rows.Scan(
			&s.ID, &s.ExamID, &s.SessionNumber, &s.RoomID, &s.StartTime, &s.EndTime, &s.CreatedAt,
			&s.RoomName, &s.RoomCapacity,
		); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}

	return schedules, rows.Err()
}

// ListAssignments retrieves all assignments for an exam, including student details.
func (r *ExamScheduleRepository) ListAssignments(ctx context.Context, examID uuid.UUID) ([]model.ExamRoomAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.exam_schedule_id, a.student_id, a.seat_number, a.created_at,
		        st.nis, st.name, cl.grade_level || ' ' || cl.major_code || ' ' || cl.group_number
		 FROM exam_room_assignments a
		 JOIN exam_schedules s ON a.exam_schedule_id = s.id
		 JOIN students st ON a.student_id = st.id
		 JOIN classes cl ON st.class_id = cl.id
		 WHERE s.exam_id = $1
		 ORDER BY s.session_number ASC, cl.grade_level ASC, cl.major_code ASC, cl.group_number ASC, st.name ASC`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []model.ExamRoomAssignment
	for rows.Next() {
		var a model.ExamRoomAssignment
		if err := rows.Scan(
			&a.ID, &a.ExamScheduleID, &a.StudentID, &a.SeatNumber, &a.CreatedAt,
			&a.StudentNIS, &a.StudentName, &a.ClassName,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}

	return assignments, rows.Err()
}

// DeleteByExam removes all existing schedules (and CASCADE handles assignments).
func (r *ExamScheduleRepository) DeleteByExam(ctx context.Context, examID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM exam_schedules WHERE exam_id = $1`, examID)
	return err
}

// UpdateTime updates the start and end times of a schedule.
func (r *ExamScheduleRepository) UpdateTime(ctx context.Context, scheduleID uuid.UUID, start, end *time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exam_schedules SET start_time = $1, end_time = $2 WHERE id = $3`,
		start, end, scheduleID,
	)
	return err
}

// BulkCreate inserts schedules and assignments transactionally.
func (r *ExamScheduleRepository) BulkCreate(ctx context.Context, schedules []model.ExamSchedule, assignments []model.ExamRoomAssignment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Bulk insert schedules
	if len(schedules) > 0 {
		var query strings.Builder
		query.WriteString(`INSERT INTO exam_schedules (id, exam_id, session_number, room_id, start_time, end_time) VALUES `)
		args := make([]interface{}, 0, len(schedules)*6)

		for i, s := range schedules {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", i*6+1, i*6+2, i*6+3, i*6+4, i*6+5, i*6+6))
			args = append(args, s.ID, s.ExamID, s.SessionNumber, s.RoomID, s.StartTime, s.EndTime)
		}

		if _, err := tx.Exec(ctx, query.String(), args...); err != nil {
			return err
		}
	}

	// 2. Bulk insert assignments
	if len(assignments) > 0 {
		// Use manual batching to avoid postgres arg limits
		batchSize := 1000
		for i := 0; i < len(assignments); i += batchSize {
			end := i + batchSize
			if end > len(assignments) {
				end = len(assignments)
			}
			batch := assignments[i:end]

			var query strings.Builder
			query.WriteString(`INSERT INTO exam_room_assignments (id, exam_schedule_id, student_id, seat_number) VALUES `)
			args := make([]interface{}, 0, len(batch)*4)

			for j, a := range batch {
				if j > 0 {
					query.WriteString(", ")
				}
				query.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d)", j*4+1, j*4+2, j*4+3, j*4+4))
				args = append(args, a.ID, a.ExamScheduleID, a.StudentID, a.SeatNumber)
			}

			if _, err := tx.Exec(ctx, query.String(), args...); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// GetStudentsByTargetRules gets eligible students for an exam based on target rules.
func (r *ExamScheduleRepository) GetStudentsByTargetRules(ctx context.Context, examID uuid.UUID) ([]model.Student, error) {
	query := `
		WITH distinct_students AS (
			SELECT DISTINCT s.id, s.nis, s.nisn, s.name, s.gender, s.religion, s.password, s.class_id, s.created_at, s.updated_at
			FROM students s
			JOIN classes c ON s.class_id = c.id
			JOIN exam_target_rules r ON r.exam_id = $1
			WHERE (r.class_id IS NULL OR s.class_id = r.class_id)
			  AND (r.grade_level IS NULL OR c.grade_level = r.grade_level)
			  AND (r.major_code IS NULL OR c.major_code = r.major_code)
			  AND (r.religion IS NULL OR s.religion = r.religion)
		)
		SELECT ds.* 
		FROM distinct_students ds
		JOIN classes c ON ds.class_id = c.id
		ORDER BY c.grade_level ASC, c.major_code ASC, c.group_number ASC, ds.name ASC
	`
	rows, err := r.pool.Query(ctx, query, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Student])
}

// GetStudentsByIDs gets students explicitly specified by ID or ClassID.
func (r *ExamScheduleRepository) GetStudentsByIDs(ctx context.Context, classIDs []int, studentIDs []int) ([]model.Student, error) {
	if len(classIDs) == 0 && len(studentIDs) == 0 {
		return []model.Student{}, nil
	}

	query := `
		WITH distinct_students AS (
			SELECT DISTINCT s.id, s.nis, s.nisn, s.name, s.gender, s.religion, s.password, s.class_id, s.created_at, s.updated_at
			FROM students s
			WHERE s.class_id = ANY($1) OR s.id = ANY($2)
		)
		SELECT ds.*
		FROM distinct_students ds
		JOIN classes c ON ds.class_id = c.id
		ORDER BY c.grade_level ASC, c.major_code ASC, c.group_number ASC, ds.name ASC
	`
	rows, err := r.pool.Query(ctx, query, classIDs, studentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Student])
}
