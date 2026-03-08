package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// RoomAssignmentRepository handles DB ops for room_sessions and student_room_assignments.
type RoomAssignmentRepository struct {
	pool *pgxpool.Pool
}

// NewRoomAssignmentRepository creates a new RoomAssignmentRepository.
func NewRoomAssignmentRepository(pool *pgxpool.Pool) *RoomAssignmentRepository {
	return &RoomAssignmentRepository{pool: pool}
}

// ListSessions retrieves all room sessions with room details.
func (r *RoomAssignmentRepository) ListSessions(ctx context.Context) ([]model.RoomSession, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.session_number, s.room_id, s.start_time::text, s.end_time::text, s.created_at,
		        rm.name, rm.capacity
		 FROM room_sessions s
		 JOIN rooms rm ON s.room_id = rm.id
		 ORDER BY s.session_number ASC, rm.name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []model.RoomSession
	for rows.Next() {
		var s model.RoomSession
		if err := rows.Scan(
			&s.ID, &s.SessionNumber, &s.RoomID, &s.StartTime, &s.EndTime, &s.CreatedAt,
			&s.RoomName, &s.RoomCapacity,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// ListAssignments retrieves all student room assignments with student details.
func (r *RoomAssignmentRepository) ListAssignments(ctx context.Context) ([]model.StudentRoomAssignment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.room_session_id, a.student_id, a.seat_number, a.created_at,
		        st.nis, st.name, cl.grade_level || ' ' || cl.major_code || ' ' || cl.group_number
		 FROM student_room_assignments a
		 JOIN room_sessions s ON a.room_session_id = s.id
		 JOIN students st ON a.student_id = st.id
		 JOIN classes cl ON st.class_id = cl.id
		 ORDER BY s.session_number ASC, s.room_id ASC, a.seat_number ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []model.StudentRoomAssignment
	for rows.Next() {
		var a model.StudentRoomAssignment
		if err := rows.Scan(
			&a.ID, &a.RoomSessionID, &a.StudentID, &a.SeatNumber, &a.CreatedAt,
			&a.StudentNIS, &a.StudentName, &a.ClassName,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}

	return assignments, rows.Err()
}

// ClearAll removes all room sessions (CASCADE deletes assignments).
func (r *RoomAssignmentRepository) ClearAll(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM room_sessions`)
	return err
}

// BulkCreate inserts sessions and assignments transactionally.
func (r *RoomAssignmentRepository) BulkCreate(ctx context.Context, sessions []model.RoomSession, assignments []model.StudentRoomAssignment) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Bulk insert sessions.
	if len(sessions) > 0 {
		var query strings.Builder
		query.WriteString(`INSERT INTO room_sessions (id, session_number, room_id) VALUES `)
		args := make([]interface{}, 0, len(sessions)*3)

		for i, s := range sessions {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(fmt.Sprintf("($%d, $%d, $%d)", i*3+1, i*3+2, i*3+3))
			args = append(args, s.ID, s.SessionNumber, s.RoomID)
		}

		if _, err := tx.Exec(ctx, query.String(), args...); err != nil {
			return err
		}
	}

	// 2. Bulk insert assignments (batched to avoid Postgres arg limits).
	if len(assignments) > 0 {
		batchSize := 1000
		for i := 0; i < len(assignments); i += batchSize {
			end := i + batchSize
			if end > len(assignments) {
				end = len(assignments)
			}
			batch := assignments[i:end]

			var query strings.Builder
			query.WriteString(`INSERT INTO student_room_assignments (id, room_session_id, student_id, seat_number) VALUES `)
			args := make([]interface{}, 0, len(batch)*4)

			for j, a := range batch {
				if j > 0 {
					query.WriteString(", ")
				}
				query.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d)", j*4+1, j*4+2, j*4+3, j*4+4))
				args = append(args, a.ID, a.RoomSessionID, a.StudentID, a.SeatNumber)
			}

			if _, err := tx.Exec(ctx, query.String(), args...); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// GetAllStudents returns all students ordered by class for distribution.
func (r *RoomAssignmentRepository) GetAllStudents(ctx context.Context) ([]model.Student, error) {
	query := `
		SELECT s.id, s.nis, s.nisn, s.name, s.gender, s.religion, s.password, s.class_id, s.created_at, s.updated_at
		FROM students s
		JOIN classes c ON s.class_id = c.id
		ORDER BY c.grade_level ASC, RANDOM()
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Student])
}

// GetStudentsByFilter gets students by class IDs and/or student IDs.
func (r *RoomAssignmentRepository) GetStudentsByFilter(ctx context.Context, classIDs []int, studentIDs []int) ([]model.Student, error) {
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
		ORDER BY c.grade_level ASC, RANDOM()
	`
	rows, err := r.pool.Query(ctx, query, classIDs, studentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[model.Student])
}

// UpdateSessionTimes updates start_time and end_time for sessions by session_number.
func (r *RoomAssignmentRepository) UpdateSessionTimes(ctx context.Context, sessions []model.SessionTimePayload) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, s := range sessions {
		_, err := tx.Exec(ctx,
			`UPDATE room_sessions SET start_time = $1::time, end_time = $2::time WHERE session_number = $3`,
			s.StartTime, s.EndTime, s.SessionNumber,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
