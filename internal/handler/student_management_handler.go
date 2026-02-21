package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// StudentManagementHandler handles admin-facing student management (CRUD, session reset).
type StudentManagementHandler struct {
	studentService *service.StudentService
	authService    *service.AuthService
}

// NewStudentManagementHandler creates a new StudentManagementHandler.
func NewStudentManagementHandler(
	studentService *service.StudentService,
	authService *service.AuthService,
) *StudentManagementHandler {
	return &StudentManagementHandler{
		studentService: studentService,
		authService:    authService,
	}
}

// ListStudents godoc
// GET /api/v1/admin/students
// Lists students with pagination, optionally filtered by class_id.
func (h *StudentManagementHandler) ListStudents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	var classID *int
	if cidStr := c.Query("class_id"); cidStr != "" {
		cid, err := strconv.Atoi(cidStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
			return
		}
		classID = &cid
	}

	students, pagination, err := h.studentService.ListStudents(c.Request.Context(), classID, page, perPage)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, gin.H{"students": students}, pagination)
}

// ResetStudentSession godoc
// POST /api/v1/admin/students/:id/reset-session
// Clears a student's active Redis session, allowing them to log in on a new device.
func (h *StudentManagementHandler) ResetStudentSession(c *gin.Context) {
	studentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.authService.ResetStudentSession(c.Request.Context(), studentID); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "student session reset successfully"})
}

// CreateStudent godoc
// POST /api/v1/admin/students
// Creates a new student.
func (h *StudentManagementHandler) CreateStudent(c *gin.Context) {
	var req model.CreateStudentRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	student := &model.Student{
		NIS:          req.NIS,
		NISN:         req.NISN,
		Name:         req.Name,
		Gender:       req.Gender,
		Religion:     req.Religion,
		PasswordHash: req.Password,
		ClassID:      req.ClassID,
	}

	// Service will hash the password.
	if err := h.studentService.Create(c.Request.Context(), student); err != nil {
		if errors.Is(err, repository.ErrDuplicateNISN) {
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"student": student})
}

// UpdateStudent godoc
// PUT /api/v1/admin/students/:id
// Updates an existing student's details, and optionally their password.
func (h *StudentManagementHandler) UpdateStudent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.UpdateStudentRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	student := &model.Student{
		ID:           id,
		NIS:          req.NIS,
		NISN:         req.NISN,
		Name:         req.Name,
		Gender:       req.Gender,
		Religion:     req.Religion,
		PasswordHash: req.Password,
		ClassID:      req.ClassID,
	}

	updatePassword := req.Password != ""

	if err := h.studentService.Update(c.Request.Context(), student, updatePassword); err != nil {
		if errors.Is(err, repository.ErrDuplicateNISN) {
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	// Fetch updated
	updatedStudent, _ := h.studentService.GetByID(c.Request.Context(), id)

	response.Success(c, http.StatusOK, gin.H{"student": updatedStudent})
}

// DeleteStudent godoc
// DELETE /api/v1/admin/students/:id
// Deletes a student by ID.
func (h *StudentManagementHandler) DeleteStudent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.studentService.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "student deleted successfully"})
}
