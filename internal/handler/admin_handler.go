package handler

import (
	"github.com/stemsi/exstem-backend/internal/service"
)

// AdminHandler handles admin-specific endpoints (non-exam).
type AdminHandler struct {
	authService *service.AuthService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(authService *service.AuthService) *AdminHandler {
	return &AdminHandler{authService: authService}
}
