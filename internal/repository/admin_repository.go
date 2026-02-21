package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// AdminRepository handles admin data access.
type AdminRepository struct {
	pool *pgxpool.Pool
}

// NewAdminRepository creates a new AdminRepository.
func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

// GetByID retrieves an admin by ID.
func (r *AdminRepository) GetByID(ctx context.Context, id int) (*model.Admin, error) {
	a := &model.Admin{}
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.email, a.name, a.password_hash, a.role_id, r.name, a.created_at, a.updated_at
		 FROM admins a JOIN roles r ON a.role_id = r.id
		 WHERE a.id = $1`, id,
	).Scan(&a.ID, &a.Email, &a.Name, &a.PasswordHash, &a.RoleID, &a.RoleName, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// GetByEmail retrieves an admin by their unique email.
func (r *AdminRepository) GetByEmail(ctx context.Context, email string) (*model.Admin, error) {
	a := &model.Admin{}
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.email, a.name, a.password_hash, a.role_id, r.name, a.created_at, a.updated_at
		 FROM admins a JOIN roles r ON a.role_id = r.id
		 WHERE a.email = $1`, email,
	).Scan(&a.ID, &a.Email, &a.Name, &a.PasswordHash, &a.RoleID, &a.RoleName, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Create inserts a new admin.
func (r *AdminRepository) Create(ctx context.Context, a *model.Admin) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO admins (email, name, password_hash, role_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		a.Email, a.Name, a.PasswordHash, a.RoleID,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}
