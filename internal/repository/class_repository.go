package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// ClassRepository handles class data access.
type ClassRepository struct {
	pool *pgxpool.Pool
}

// NewClassRepository creates a new ClassRepository.
func NewClassRepository(pool *pgxpool.Pool) *ClassRepository {
	return &ClassRepository{pool: pool}
}

// GetByID retrieves a class by its ID.
func (r *ClassRepository) GetByID(ctx context.Context, id int) (*model.Class, error) {
	c := &model.Class{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, grade_level, major_code, group_number, created_at, updated_at
		 FROM classes WHERE id = $1`, id,
	).Scan(&c.ID, &c.GradeLevel, &c.MajorCode, &c.GroupNumber, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// List retrieves all classes.
func (r *ClassRepository) List(ctx context.Context) ([]model.Class, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, grade_level, major_code, group_number, created_at, updated_at
		 FROM classes ORDER BY grade_level, major_code, group_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var classes []model.Class
	for rows.Next() {
		var c model.Class
		if err := rows.Scan(&c.ID, &c.GradeLevel, &c.MajorCode, &c.GroupNumber, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		classes = append(classes, c)
	}
	return classes, rows.Err()
}

// Create inserts a new class.
func (r *ClassRepository) Create(ctx context.Context, c *model.Class) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO classes (grade_level, major_code, group_number)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		c.GradeLevel, c.MajorCode, c.GroupNumber,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

// Update modifies an existing class.
func (r *ClassRepository) Update(ctx context.Context, c *model.Class) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE classes SET grade_level = $1, major_code = $2, group_number = $3, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $4`,
		c.GradeLevel, c.MajorCode, c.GroupNumber, c.ID,
	)
	return err
}

// Delete removes a class by its ID.
func (r *ClassRepository) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM classes WHERE id = $1`, id)
	return err
}
