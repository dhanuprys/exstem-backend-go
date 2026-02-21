package service

import (
	"context"
	"errors"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// AdminRoleService handles business logic for admin roles.
type AdminRoleService struct {
	roleRepo *repository.RoleRepository
}

// NewAdminRoleService creates a new AdminRoleService.
func NewAdminRoleService(roleRepo *repository.RoleRepository) *AdminRoleService {
	return &AdminRoleService{roleRepo: roleRepo}
}

// ListRoles retrieves all roles with their permissions.
func (s *AdminRoleService) ListRoles(ctx context.Context) ([]model.RoleWithPermissions, error) {
	return s.roleRepo.ListRolesWithPermissions(ctx)
}

// GetRoleByID retrieves a specific role and its permissions.
func (s *AdminRoleService) GetRoleByID(ctx context.Context, id int) (*model.RoleWithPermissions, error) {
	return s.roleRepo.GetRoleByID(ctx, id)
}

// CreateRole creates a new role and assigns its permissions.
func (s *AdminRoleService) CreateRole(ctx context.Context, name string, permissions []string) (*model.RoleWithPermissions, error) {
	if name == "" {
		return nil, errors.New("role name cannot be empty")
	}

	// 1. Create Role
	id, err := s.roleRepo.CreateRole(ctx, name)
	if err != nil {
		return nil, err
	}

	// 2. Assign Permissions
	if len(permissions) > 0 {
		err = s.roleRepo.AssignPermissionsToRole(ctx, id, permissions)
		if err != nil {
			// If assignment fails, we should ideally rollback the role creation.
			// Since we don't have tx injected here trivially, we could attempt to delete it
			// or have repository handle transactional creation.
			// For simplicity and to match current pattern, we attempt cleanup or just return err
			_ = s.roleRepo.DeleteRole(ctx, id)
			return nil, err
		}
	}

	return s.GetRoleByID(ctx, id)
}

// UpdateRole updates a role's name and permissions.
func (s *AdminRoleService) UpdateRole(ctx context.Context, id int, name string, permissions []string) (*model.RoleWithPermissions, error) {
	if id == 1 {
		return nil, errors.New("cannot update system Superadmin role")
	}
	if name == "" {
		return nil, errors.New("role name cannot be empty")
	}

	// 1. Update Role Name
	err := s.roleRepo.UpdateRole(ctx, id, name)
	if err != nil {
		return nil, err
	}

	// 2. Update Permissions (Replace all)
	// Delete existing
	err = s.roleRepo.DeleteAllPermissionsFromRole(ctx, id)
	if err != nil {
		return nil, err
	}

	// Assign new
	if len(permissions) > 0 {
		err = s.roleRepo.AssignPermissionsToRole(ctx, id, permissions)
		if err != nil {
			return nil, err
		}
	}

	return s.GetRoleByID(ctx, id)
}

// DeleteRole deletes a role.
func (s *AdminRoleService) DeleteRole(ctx context.Context, id int) error {
	if id == 1 {
		return errors.New("cannot delete system Superadmin role")
	}
	// Note: Role usage check is handled by DB foreign key constraints.
	// If a user has this role, deletion will fail at DB level.
	return s.roleRepo.DeleteRole(ctx, id)
}

// GetAllPermissions retrieves all available system permission codes.
func (s *AdminRoleService) GetAllPermissions() []string {
	perms := make([]string, len(model.AllPermissions))
	for i, p := range model.AllPermissions {
		perms[i] = string(p)
	}
	return perms
}
