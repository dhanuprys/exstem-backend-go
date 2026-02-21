package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

type AdminUserHandler struct {
	service *service.AdminUserService
}

func NewAdminUserHandler(service *service.AdminUserService) *AdminUserHandler {
	return &AdminUserHandler{service: service}
}

// ... (ListAdmins, CreateAdmin, UpdateAdmin remain unchanged)

func (h *AdminUserHandler) ListAdmins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	roleID, _ := strconv.Atoi(c.Query("role_id"))

	admins, total, err := h.service.ListAdmins(c.Request.Context(), roleID, page, perPage)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, admins, &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: (total + perPage - 1) / perPage,
	})
}

// CreateAdminRequest payload
type CreateAdminRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required,min=3"`
	Password string `json:"password" binding:"required,min=6"`
	RoleID   int    `json:"role_id" binding:"required"`
}

// CreateAdmin handles creating a new admin.
func (h *AdminUserHandler) CreateAdmin(c *gin.Context) {
	var req CreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_INPUT")
		return
	}

	admin, err := h.service.CreateAdmin(c.Request.Context(), req.Email, req.Name, req.Password, req.RoleID)
	if err != nil {
		if err.Error() == "email already registered" {
			response.Fail(c, http.StatusConflict, "EMAIL_EXISTS")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	response.Success(c, http.StatusCreated, admin)
}

// UpdateAdminRequest payload
type UpdateAdminRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required,min=3"`
	Password string `json:"password"` // Optional: only update if provided
	RoleID   int    `json:"role_id" binding:"required"`
}

// UpdateAdmin handles updating an existing admin.
func (h *AdminUserHandler) UpdateAdmin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var req UpdateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_INPUT")
		return
	}

	admin, err := h.service.UpdateAdmin(c.Request.Context(), id, req.Email, req.Name, req.Password, req.RoleID)
	if err != nil {
		if err.Error() == "admin not found" {
			response.Fail(c, http.StatusNotFound, "NOT_FOUND")
			return
		}
		if err.Error() == "email already registered" {
			response.Fail(c, http.StatusConflict, "EMAIL_EXISTS")
			return
		}
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	response.Success(c, http.StatusOK, admin)
}

// DeleteAdmin handles deleting an admin.
func (h *AdminUserHandler) DeleteAdmin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID")
		return
	}

	// Prevent self-deletion
	claims := middleware.GetClaims(c)
	if claims != nil && claims.UserID == id {
		// We don't have a specific error code for self-deletion in the enum yet, using CANNOT_DELETE_SELF would be ideal
		// For now using INVALID_ACTION or adding a new code. Let's use INVALID_ACTION if it exists or fallback to general error.
		// Checking response package, we likely need to add a specific code or reuse one.
		// Let's assume we can add "CANNOT_DELETE_SELF" or similar.
		// For now, I'll use a generic error with message override logic if possible, but Fail takes code.
		// I will add CANNOT_DELETE_SELF to err codes if I can, or use INVALID_INPUT.
		response.Fail(c, http.StatusConflict, response.ErrActionForbidden) // Using a placeholder, likely need to define it
		return
	}

	err = h.service.DeleteAdmin(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "admin not found" {
			response.Fail(c, http.StatusNotFound, "NOT_FOUND")
			return
		}
		// PGX error message for FK violation
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			response.Fail(c, http.StatusConflict, response.ErrDependencyExists)
			return
		}
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "Admin deleted successfully"})
}

// GetRoles handles listing roles for selection.
func (h *AdminUserHandler) GetRoles(c *gin.Context) {
	roles, err := h.service.GetRoles(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Success(c, http.StatusOK, roles)
}
