package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

type MajorRepository interface {
	GetAll(ctx context.Context) ([]*model.Major, error)
	GetByID(ctx context.Context, id int) (*model.Major, error)
	GetByCode(ctx context.Context, code string) (*model.Major, error)
	Create(ctx context.Context, major *model.Major) error
	Update(ctx context.Context, major *model.Major) error
	Delete(ctx context.Context, id int) error
}

type majorRepository struct {
	db *pgxpool.Pool
}

func NewMajorRepository(db *pgxpool.Pool) MajorRepository {
	return &majorRepository{db: db}
}

func (r *majorRepository) GetAll(ctx context.Context) ([]*model.Major, error) {
	query := `SELECT id, code, long_name, created_at, updated_at FROM majors ORDER BY long_name ASC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var majors []*model.Major
	for rows.Next() {
		m := &model.Major{}
		if err := rows.Scan(&m.ID, &m.Code, &m.LongName, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		majors = append(majors, m)
	}
	return majors, nil
}

func (r *majorRepository) GetByID(ctx context.Context, id int) (*model.Major, error) {
	query := `SELECT id, code, long_name, created_at, updated_at FROM majors WHERE id = $1`
	m := &model.Major{}
	err := r.db.QueryRow(ctx, query, id).Scan(&m.ID, &m.Code, &m.LongName, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *majorRepository) GetByCode(ctx context.Context, code string) (*model.Major, error) {
	query := `SELECT id, code, long_name, created_at, updated_at FROM majors WHERE code = $1`
	m := &model.Major{}
	err := r.db.QueryRow(ctx, query, code).Scan(&m.ID, &m.Code, &m.LongName, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *majorRepository) Create(ctx context.Context, major *model.Major) error {
	query := `
		INSERT INTO majors (code, long_name)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query, major.Code, major.LongName).Scan(&major.ID, &major.CreatedAt, &major.UpdatedAt)
}

func (r *majorRepository) Update(ctx context.Context, major *model.Major) error {
	query := `
		UPDATE majors
		SET code = $1, long_name = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query, major.Code, major.LongName, major.ID).Scan(&major.UpdatedAt)
}

func (r *majorRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM majors WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
