package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// RoleRepository handles role and permission data access.
type RoleRepository struct {
	pool *pgxpool.Pool
}

// NewRoleRepository creates a new RoleRepository.
func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{pool: pool}
}

// GetPermissionsByRoleID retrieves all permission codes for a given role.
func (r *RoleRepository) GetPermissionsByRoleID(ctx context.Context, roleID int) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.code
		 FROM permissions p
		 JOIN role_permissions rp ON p.id = rp.permission_id
		 WHERE rp.role_id = $1
		 ORDER BY p.code`, roleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		permissions = append(permissions, code)
	}
	return permissions, rows.Err()
}

// GetRoleByID retrieves a role and its permissions by ID.
func (r *RoleRepository) GetRoleByID(ctx context.Context, id int) (*model.RoleWithPermissions, error) {
	role := &model.Role{ID: id}
	err := r.pool.QueryRow(ctx, "SELECT name, created_at FROM roles WHERE id = $1", id).Scan(&role.Name, &role.CreatedAt)
	if err != nil {
		return nil, err
	}

	permissions, err := r.GetPermissionsByRoleID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.RoleWithPermissions{
		Role:        role,
		Permissions: permissions,
	}, nil
}

// ListRolesWithPermissions retrieves all roles with their associated permissions.
func (r *RoleRepository) ListRolesWithPermissions(ctx context.Context) ([]model.RoleWithPermissions, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, name, created_at FROM roles ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []model.RoleWithPermissions
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.CreatedAt); err != nil {
			return nil, err
		}

		// Note: Making separate queries for permissions per role could be optimized with a JOIN,
		// but since the number of roles is small, this is acceptable for now.
		permissions, err := r.GetPermissionsByRoleID(ctx, role.ID)
		if err != nil {
			return nil, err
		}

		roles = append(roles, model.RoleWithPermissions{
			Role:        &role,
			Permissions: permissions,
		})
	}

	return roles, rows.Err()
}

// CreateRole inserts a new role and returns its ID.
func (r *RoleRepository) CreateRole(ctx context.Context, name string) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, "INSERT INTO roles (name) VALUES ($1) RETURNING id", name).Scan(&id)
	return id, err
}

// UpdateRole updates an existing role's name.
func (r *RoleRepository) UpdateRole(ctx context.Context, id int, name string) error {
	_, err := r.pool.Exec(ctx, "UPDATE roles SET name = $1 WHERE id = $2", name, id)
	return err
}

// DeleteRole removes a role from the database.
func (r *RoleRepository) DeleteRole(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM roles WHERE id = $1", id)
	return err
}

// DeleteAllPermissionsFromRole removes all permissions associated with a role.
func (r *RoleRepository) DeleteAllPermissionsFromRole(ctx context.Context, roleID int) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM role_permissions WHERE role_id = $1", roleID)
	return err
}

// AssignPermissionsToRole assigns a list of permission codes to a role.
func (r *RoleRepository) AssignPermissionsToRole(ctx context.Context, roleID int, permissionCodes []string) error {
	if len(permissionCodes) == 0 {
		return nil
	}

	// First, fetch the integer IDs for the given permission codes
	// Building a dynamic query for IN clause
	query := "SELECT id FROM permissions WHERE code = ANY($1)"
	rows, err := r.pool.Query(ctx, query, permissionCodes)
	if err != nil {
		return err
	}
	defer rows.Close()

	var permissionIDs []int
	for rows.Next() {
		var pid int
		if err := rows.Scan(&pid); err != nil {
			return err
		}
		permissionIDs = append(permissionIDs, pid)
	}

	if len(permissionIDs) == 0 {
		return nil // No valid permissions found
	}

	// Now insert into role_permissions
	// In pgx/v5, easiest way to insert multiple rows is using CopyFrom or batch.
	// We'll use a simple loop within a transaction if possible, or multiple single inserts for simplicity.
	// Since we are likely already in a context (maybe tx), Exec in a loop is fine for few perms.

	// Better approach for PG: INSERT INTO ... VALUES ($1, $2), ($3, $4)... but dynamic is hard.
	// pgx.CopyFrom is best.

	_, err = r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"role_permissions"},
		[]string{"role_id", "permission_id"},
		pgx.CopyFromSlice(len(permissionIDs), func(i int) ([]interface{}, error) {
			return []interface{}{roleID, permissionIDs[i]}, nil
		}),
	)

	return err
}
