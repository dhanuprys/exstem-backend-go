package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type AdminUserService struct {
	pool *pgxpool.Pool
}

func NewAdminUserService(pool *pgxpool.Pool) *AdminUserService {
	return &AdminUserService{pool: pool}
}

// ListAdmins retrieves a paginated list of admins.
func (s *AdminUserService) ListAdmins(ctx context.Context, roleID, page, perPage int) ([]model.Admin, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	offset := (page - 1) * perPage

	// Count query
	countQuery := "SELECT COUNT(*) FROM admins WHERE 1=1"
	args := []interface{}{}
	if roleID > 0 {
		countQuery += " AND role_id = $1"
		args = append(args, roleID)
	}

	var total int
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Select query
	query := `
		SELECT a.id, a.email, a.name, a.role_id, a.created_at, a.updated_at, r.name as role_name
		FROM admins a
		JOIN roles r ON a.role_id = r.id
		WHERE 1=1
	`
	args = []interface{}{} // Reset args for main query to handle parameter indices correctly
	argCount := 1

	if roleID > 0 {
		query += " AND a.role_id = $1"
		args = append(args, roleID)
		argCount++
	}

	query += " ORDER BY a.created_at DESC LIMIT $" + string(rune('0'+argCount)) + " OFFSET $" + string(rune('0'+argCount+1))
	// Actually handling dynamic dollar signs in Go strings is annoying, let's just use simple logic
	// or create the query string carefully.

	// Re-building args correctly
	query = `
		SELECT a.id, a.email, a.name, a.role_id, a.created_at, a.updated_at, r.name as role_name
		FROM admins a
		JOIN roles r ON a.role_id = r.id
		WHERE 1=1
	`
	queryArgs := []interface{}{}

	if roleID > 0 {
		query += " AND a.role_id = $1"
		queryArgs = append(queryArgs, roleID)
		query += " ORDER BY a.created_at DESC LIMIT $2 OFFSET $3"
	} else {
		query += " ORDER BY a.created_at DESC LIMIT $1 OFFSET $2"
	}
	queryArgs = append(queryArgs, perPage, offset)

	rows, err := s.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	admins := []model.Admin{}
	for rows.Next() {
		var a model.Admin
		err := rows.Scan(
			&a.ID, &a.Email, &a.Name, &a.RoleID, &a.CreatedAt, &a.UpdatedAt, &a.RoleName,
		)
		if err != nil {
			return nil, 0, err
		}
		admins = append(admins, a)
	}

	return admins, total, nil
}

// CreateAdmin creates a new admin user.
func (s *AdminUserService) CreateAdmin(ctx context.Context, email, name, password string, roleID int) (*model.Admin, error) {
	// Check if email exists
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admins WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("email already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var id int
	var createdAt, updatedAt time.Time

	err = s.pool.QueryRow(ctx,
		"INSERT INTO admins (email, name, password_hash, role_id) VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at",
		email, name, string(hashedPassword), roleID,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	// Fetch created admin with role name
	var admin model.Admin
	err = s.pool.QueryRow(ctx, `
		SELECT a.id, a.email, a.name, a.role_id, a.created_at, a.updated_at, r.name
		FROM admins a
		JOIN roles r ON a.role_id = r.id
		WHERE a.id = $1
	`, id).Scan(&admin.ID, &admin.Email, &admin.Name, &admin.RoleID, &admin.CreatedAt, &admin.UpdatedAt, &admin.RoleName)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

// UpdateAdmin updates an existing admin user.
func (s *AdminUserService) UpdateAdmin(ctx context.Context, id int, email, name, password string, roleID int) (*model.Admin, error) {
	// Check if admin exists
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admins WHERE id = $1)", id).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("admin not found")
	}

	// Check email uniqueness if changed
	var emailExists bool
	err = s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admins WHERE email = $1 AND id != $2)", email, id).Scan(&emailExists)
	if err != nil {
		return nil, err
	}
	if emailExists {
		return nil, errors.New("email already registered")
	}

	var errUpdate error
	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		_, errUpdate = s.pool.Exec(ctx,
			"UPDATE admins SET email = $1, name = $2, password_hash = $3, role_id = $4, updated_at = NOW() WHERE id = $5",
			email, name, string(hashedPassword), roleID, id,
		)
	} else {
		_, errUpdate = s.pool.Exec(ctx,
			"UPDATE admins SET email = $1, name = $2, role_id = $3, updated_at = NOW() WHERE id = $4",
			email, name, roleID, id,
		)
	}
	if errUpdate != nil {
		return nil, errUpdate
	}

	// Return updated admin
	var admin model.Admin
	err = s.pool.QueryRow(ctx, `
		SELECT a.id, a.email, a.name, a.role_id, a.created_at, a.updated_at, r.name
		FROM admins a
		JOIN roles r ON a.role_id = r.id
		WHERE a.id = $1
	`, id).Scan(&admin.ID, &admin.Email, &admin.Name, &admin.RoleID, &admin.CreatedAt, &admin.UpdatedAt, &admin.RoleName)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

// DeleteAdmin deletes an admin user.
func (s *AdminUserService) DeleteAdmin(ctx context.Context, id int) error {
	res, err := s.pool.Exec(ctx, "DELETE FROM admins WHERE id = $1", id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("admin not found")
	}
	return nil
}

// GetRoles gets all available roles for selection
func (s *AdminUserService) GetRoles(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := s.pool.Query(ctx, "SELECT id, name FROM roles ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []map[string]interface{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		roles = append(roles, map[string]interface{}{
			"id":   id,
			"name": name,
		})
	}
	return roles, nil
}
