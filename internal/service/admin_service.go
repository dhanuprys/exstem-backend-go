package service

import (
	"context"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// AdminService handles admin business logic.
type AdminService struct {
	adminRepo *repository.AdminRepository
	roleRepo  *repository.RoleRepository
}

// NewAdminService creates a new AdminService.
func NewAdminService(adminRepo *repository.AdminRepository, roleRepo *repository.RoleRepository) *AdminService {
	return &AdminService{adminRepo: adminRepo, roleRepo: roleRepo}
}

// GetByEmail retrieves an admin by email.
func (s *AdminService) GetByEmail(ctx context.Context, email string) (*model.Admin, error) {
	return s.adminRepo.GetByEmail(ctx, email)
}

// GetByID retrieves an admin by ID.
func (s *AdminService) GetByID(ctx context.Context, id int) (*model.Admin, error) {
	return s.adminRepo.GetByID(ctx, id)
}

// GetPermissions retrieves permission codes for an admin's role.
func (s *AdminService) GetPermissions(ctx context.Context, roleID int) ([]string, error) {
	return s.roleRepo.GetPermissionsByRoleID(ctx, roleID)
}

// Create creates a new admin.
func (s *AdminService) Create(ctx context.Context, admin *model.Admin) error {
	return s.adminRepo.Create(ctx, admin)
}
