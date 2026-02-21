package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// ClassHandler handles admin-facing class management (CRUD).
type ClassHandler struct {
	classService *service.ClassService
}

// NewClassHandler creates a new ClassHandler.
func NewClassHandler(classService *service.ClassService) *ClassHandler {
	return &ClassHandler{classService: classService}
}

// ListClasses godoc
// GET /api/v1/admin/classes
// Lists all classes without pagination.
func (h *ClassHandler) ListClasses(c *gin.Context) {
	classes, err := h.classService.List(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"classes": classes})
}

// CreateClassRequest is the payload for creating or updating a class.
type CreateClassRequest struct {
	GradeLevel  string `json:"grade_level" binding:"required,min=1,max=10"`
	MajorCode   string `json:"major_code" binding:"required,min=1,max=10"`
	GroupNumber int    `json:"group_number" binding:"required,min=1"`
}

// CreateClass godoc
// POST /api/v1/admin/classes
// Creates a new class.
func (h *ClassHandler) CreateClass(c *gin.Context) {
	var req CreateClassRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	class := &model.Class{
		GradeLevel:  req.GradeLevel,
		MajorCode:   req.MajorCode,
		GroupNumber: req.GroupNumber,
	}

	if err := h.classService.Create(c.Request.Context(), class); err != nil {
		// Detect unique constraint violations (e.g., if a constraint exists on grade+major+group)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"class": class})
}

// UpdateClass godoc
// PUT /api/v1/admin/classes/:id
// Updates an existing class.
func (h *ClassHandler) UpdateClass(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req CreateClassRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	class := &model.Class{
		ID:          id,
		GradeLevel:  req.GradeLevel,
		MajorCode:   req.MajorCode,
		GroupNumber: req.GroupNumber,
	}

	if err := h.classService.Update(c.Request.Context(), class); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	// Fetch updated to get current updated_at timestamp
	updatedClass, _ := h.classService.GetByID(c.Request.Context(), id)

	response.Success(c, http.StatusOK, gin.H{"class": updatedClass})
}

// DeleteClass godoc
// DELETE /api/v1/admin/classes/:id
// Deletes a class by ID. Will fail if students are attached.
func (h *ClassHandler) DeleteClass(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.classService.Delete(c.Request.Context(), id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // Foreign key constraint violation
			response.Fail(c, http.StatusConflict, response.ErrDependencyExists)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "class deleted successfully"})
}
