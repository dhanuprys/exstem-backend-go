package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService    *service.AuthService
	studentService *service.StudentService
	adminService   *service.AdminService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	authService *service.AuthService,
	studentService *service.StudentService,
	adminService *service.AdminService,
) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		studentService: studentService,
		adminService:   adminService,
	}
}

// GetStudentProfile godoc
// GET /api/v1/auth/student/me
// Returns the profile of the currently authenticated student.
func (h *AuthHandler) GetStudentProfile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	student, err := h.studentService.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrNotFound)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"student": gin.H{
			"id":       student.ID,
			"nisn":     student.NISN,
			"name":     student.Name,
			"class_id": student.ClassID,
		},
	})
}

// StudentLogout godoc
// POST /api/v1/auth/student/logout
// Logs out the currently authenticated student.
func (h *AuthHandler) StudentLogout(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	err := h.authService.ResetStudentSession(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{})
}

// GetAdminProfile godoc
// GET /api/v1/auth/admin/me
// Returns the profile of the currently authenticated admin.
func (h *AuthHandler) GetAdminProfile(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	admin, err := h.adminService.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrNotFound)
		return
	}

	permissions, err := h.adminService.GetPermissions(c.Request.Context(), admin.RoleID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"admin": gin.H{
			"id":        admin.ID,
			"email":     admin.Email,
			"name":      admin.Name,
			"role_id":   admin.RoleID,
			"role_name": admin.RoleName,
		},
		"permissions": permissions,
	})
}

// StudentLogin godoc
// POST /api/v1/auth/student/login
// Validates NISN + password, checks for existing session (rejects if active), returns JWT.
func (h *AuthHandler) StudentLogin(c *gin.Context) {
	var req model.StudentLoginRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	student, err := h.studentService.GetByNISN(c.Request.Context(), req.NISN)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrInvalidCredentials)
		return
	}

	if err := h.authService.CheckPassword(student.PasswordHash, req.Password); err != nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrInvalidCredentials)
		return
	}

	token, err := h.authService.GenerateStudentToken(c.Request.Context(), student.ID, student.ClassID)
	if err != nil {
		if errors.Is(err, service.ErrSessionAlreadyActive) {
			response.Fail(c, http.StatusConflict, response.ErrSessionActive)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"token": token,
		"student": gin.H{
			"id":       student.ID,
			"nisn":     student.NISN,
			"name":     student.Name,
			"class_id": student.ClassID,
		},
	})
}

// AdminLogin godoc
// POST /api/v1/auth/admin/login
// Validates email + password, returns JWT with permissions.
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req model.AdminLoginRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	admin, err := h.adminService.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrInvalidCredentials)
		return
	}

	if err := h.authService.CheckPassword(admin.PasswordHash, req.Password); err != nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrInvalidCredentials)
		return
	}

	permissions, err := h.adminService.GetPermissions(c.Request.Context(), admin.RoleID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	token, err := h.authService.GenerateAdminToken(admin.ID, admin.RoleID, permissions)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"token": token,
		"admin": gin.H{
			"id":        admin.ID,
			"email":     admin.Email,
			"name":      admin.Name,
			"role_id":   admin.RoleID,
			"role_name": admin.RoleName,
		},
		"permissions": permissions,
	})
}
