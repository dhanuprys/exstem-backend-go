package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

type AdminRoleHandler struct {
	service *service.AdminRoleService
}

func NewAdminRoleHandler(service *service.AdminRoleService) *AdminRoleHandler {
	return &AdminRoleHandler{service: service}
}

// ListRoles gets all roles with their associated permissions.
func (h *AdminRoleHandler) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, roles)
}

// GetRole gets a role and its permissions by ID.
func (h *AdminRoleHandler) GetRole(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	role, err := h.service.GetRoleByID(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "no rows in result set" {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, role)
}

// CreateUpdateRoleRequest payload for role operations.
type CreateUpdateRoleRequest struct {
	Name        string   `json:"name" binding:"required,min=2"`
	Permissions []string `json:"permissions"`
}

// CreateRole creates a new role with given permissions.
func (h *AdminRoleHandler) CreateRole(c *gin.Context) {
	var req CreateUpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidPayload)
		return
	}

	role, err := h.service.CreateRole(c.Request.Context(), req.Name, req.Permissions)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value") {
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, role)
}

// UpdateRole updates an existing role.
func (h *AdminRoleHandler) UpdateRole(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req CreateUpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidPayload)
		return
	}

	role, err := h.service.UpdateRole(c.Request.Context(), id, req.Name, req.Permissions)
	if err != nil {
		if strings.Contains(err.Error(), "cannot update") {
			response.Fail(c, http.StatusForbidden, response.ErrActionForbidden)
			return
		}
		if err.Error() == "no rows in result set" {
			response.Fail(c, http.StatusNotFound, response.ErrNotFound)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, role)
}

// DeleteRole deletes an existing role.
func (h *AdminRoleHandler) DeleteRole(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	err = h.service.DeleteRole(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "cannot delete") {
			response.Fail(c, http.StatusForbidden, response.ErrActionForbidden)
			return
		}
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			response.Fail(c, http.StatusConflict, response.ErrDependencyExists)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "Role deleted successfully"})
}

// GetPermissions lists all available permissions.
func (h *AdminRoleHandler) GetPermissions(c *gin.Context) {
	perms := h.service.GetAllPermissions()
	response.Success(c, http.StatusOK, perms)
}
