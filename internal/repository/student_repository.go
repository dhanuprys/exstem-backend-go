package repository

import (
	"context"
	"strconv"

	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

var ErrDuplicateNISN = errors.New("student with this NISN already exists")

// StudentRepository handles student data access.
type StudentRepository struct {
	pool *pgxpool.Pool
}

// NewStudentRepository creates a new StudentRepository.
func NewStudentRepository(pool *pgxpool.Pool) *StudentRepository {
	return &StudentRepository{pool: pool}
}

// GetByID retrieves a student by ID.
func (r *StudentRepository) GetByID(ctx context.Context, id int) (*model.Student, error) {
	s := &model.Student{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, nis, nisn, name, gender, religion, password_hash, class_id, created_at, updated_at
		 FROM students WHERE id = $1`, id,
	).Scan(&s.ID, &s.NIS, &s.NISN, &s.Name, &s.Gender, &s.Religion, &s.PasswordHash, &s.ClassID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetByNISN retrieves a student by their unique NISN.
func (r *StudentRepository) GetByNISN(ctx context.Context, nisn string) (*model.Student, error) {
	s := &model.Student{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, nis, nisn, name, gender, religion, password_hash, class_id, created_at, updated_at
		 FROM students WHERE nisn = $1`, nisn,
	).Scan(&s.ID, &s.NIS, &s.NISN, &s.Name, &s.Gender, &s.Religion, &s.PasswordHash, &s.ClassID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// ListPaginated retrieves students with pagination and optional class filter.
func (r *StudentRepository) ListPaginated(ctx context.Context, classID *int, limit, offset int) ([]model.Student, int, error) {
	// 1. Get total count
	countQuery := `SELECT COUNT(*) FROM students`
	var countArgs []interface{}
	if classID != nil {
		countQuery += ` WHERE class_id = $1`
		countArgs = append(countArgs, *classID)
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 2. Get paginated data
	query := `SELECT id, nis, nisn, name, gender, religion, password_hash, class_id, created_at, updated_at FROM students`
	var args []interface{}
	argIdx := 1

	if classID != nil {
		query += ` WHERE class_id = $1`
		args = append(args, *classID)
		argIdx++
	}

	query += ` ORDER BY name LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var students []model.Student
	for rows.Next() {
		var s model.Student
		if err := rows.Scan(&s.ID, &s.NIS, &s.NISN, &s.Name, &s.Gender, &s.Religion, &s.PasswordHash, &s.ClassID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, err
		}
		students = append(students, s)
	}
	return students, total, rows.Err()
}

// Create inserts a new student.
func (r *StudentRepository) Create(ctx context.Context, s *model.Student) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO students (nis, nisn, name, gender, religion, password_hash, class_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		s.NIS, s.NISN, s.Name, s.Gender, s.Religion, s.PasswordHash, s.ClassID,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateNISN
		}
		return err
	}
	return nil
}

// Update modifies a student's basic info (excluding password).
func (r *StudentRepository) Update(ctx context.Context, s *model.Student) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE students SET nis = $1, nisn = $2, name = $3, gender = $4, religion = $5, class_id = $6, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $7`,
		s.NIS, s.NISN, s.Name, s.Gender, s.Religion, s.ClassID, s.ID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateNISN
		}
		return err
	}
	return nil
}

// UpdatePassword updates a student's password hash.
func (r *StudentRepository) UpdatePassword(ctx context.Context, id int, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE students SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		passwordHash, id,
	)
	return err
}

// Delete removes a student by ID.
func (r *StudentRepository) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM students WHERE id = $1`, id)
	return err
}
