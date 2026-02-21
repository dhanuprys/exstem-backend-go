package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

type SubjectHandler struct {
	subjectService *service.SubjectService
}

func NewSubjectHandler(subjectService *service.SubjectService) *SubjectHandler {
	return &SubjectHandler{subjectService: subjectService}
}

// GetAll godoc
// GET /api/v1/admin/subjects
func (h *SubjectHandler) GetAll(c *gin.Context) {
	subjects, err := h.subjectService.GetAll(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	if subjects == nil {
		subjects = []model.Subject{}
	}

	response.Success(c, http.StatusOK, gin.H{"subjects": subjects})
}

// Create godoc
// POST /api/v1/admin/subjects
func (h *SubjectHandler) Create(c *gin.Context) {
	var req model.CreateSubjectRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	sub := &model.Subject{Name: req.Name}
	if err := h.subjectService.Create(c.Request.Context(), sub); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusCreated, gin.H{"subject": sub})
}

// Update godoc
// PUT /api/v1/admin/subjects/:id
func (h *SubjectHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.UpdateSubjectRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	sub := &model.Subject{ID: id, Name: req.Name}
	if err := h.subjectService.Update(c.Request.Context(), sub); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"message": "subject updated successfully"})
}

// Delete godoc
// DELETE /api/v1/admin/subjects/:id
func (h *SubjectHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.subjectService.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"message": "subject deleted successfully"})
}
