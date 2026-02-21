package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

type SubjectRepository struct {
	pool *pgxpool.Pool
}

func NewSubjectRepository(pool *pgxpool.Pool) *SubjectRepository {
	return &SubjectRepository{pool: pool}
}

func (r *SubjectRepository) Create(ctx context.Context, s *model.Subject) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO subjects (name) VALUES ($1) RETURNING id, created_at, updated_at`,
		s.Name).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *SubjectRepository) GetAll(ctx context.Context) ([]model.Subject, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, created_at, updated_at FROM subjects ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subjects []model.Subject
	for rows.Next() {
		var s model.Subject
		if err := rows.Scan(&s.ID, &s.Name, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		subjects = append(subjects, s)
	}
	return subjects, rows.Err()
}

func (r *SubjectRepository) Update(ctx context.Context, s *model.Subject) error {
	_, err := r.pool.Exec(ctx, `UPDATE subjects SET name = $1, updated_at = NOW() WHERE id = $2`, s.Name, s.ID)
	return err
}

func (r *SubjectRepository) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM subjects WHERE id = $1`, id)
	return err
}
