package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// ExamHandler handles exam management endpoints.
type ExamHandler struct {
	examService    *service.ExamService
	sessionService *service.ExamSessionService
}

// NewExamHandler creates a new ExamHandler.
func NewExamHandler(examService *service.ExamService, sessionService *service.ExamSessionService) *ExamHandler {
	return &ExamHandler{
		examService:    examService,
		sessionService: sessionService,
	}
}

func generateToken() string {
	bytes := make([]byte, 3) // 6 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "EXAM00"
	}
	return strings.ToUpper(hex.EncodeToString(bytes))
}

// ListExams godoc
// GET /api/v1/admin/exams
// Lists exams with pagination. Superadmins see all; teachers see only their own.
func (h *ExamHandler) ListExams(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	// Check if the admin has write_all permission (superadmin).
	authorFilter := claims.UserID
	for _, p := range claims.Permissions {
		if p == "exams:write_all" {
			authorFilter = 0 // Show all exams
			break
		}
	}

	exams, pagination, err := h.examService.ListByAuthor(c.Request.Context(), authorFilter, page, perPage)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, gin.H{"exams": exams}, pagination)
}

// CreateExam godoc
// POST /api/v1/admin/exams
// Creates a new draft exam.
func (h *ExamHandler) CreateExam(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	var req model.CreateExamRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	exam := &model.Exam{
		Title:           req.Title,
		AuthorID:        claims.UserID,
		ScheduledStart:  req.ScheduledStart,
		ScheduledEnd:    req.ScheduledEnd,
		DurationMinutes: req.DurationMinutes,
		EntryToken:      generateToken(),
	}

	if err := h.examService.Create(c.Request.Context(), exam); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"exam": exam})
}

// PublishExam godoc
// POST /api/v1/admin/exams/:exam_id/publish
// Publishes an exam: caches payload + answer key to Redis, changes status.
func (h *ExamHandler) PublishExam(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	authorFilter := claims.UserID
	for _, p := range claims.Permissions {
		if p == "exams:write_all" {
			authorFilter = 0
			break
		}
	}

	if err := h.examService.Publish(c.Request.Context(), examID, authorFilter); err != nil {
		switch {
		case errors.Is(err, service.ErrNotExamAuthor):
			response.Fail(c, http.StatusForbidden, response.ErrNotExamAuthor)
		case errors.Is(err, service.ErrNoQuestions):
			response.Fail(c, http.StatusBadRequest, response.ErrNoQuestions)
		case errors.Is(err, service.ErrExamNotDraft):
			response.Fail(c, http.StatusBadRequest, response.ErrExamNotAvailable)
		default:
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "exam published successfully"})
}

// AddTargetRule godoc
// POST /api/v1/admin/exams/:exam_id/target-rules
// Adds a target rule determining which students can see the exam.
func (h *ExamHandler) AddTargetRule(c *gin.Context) {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.AddTargetRuleRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	rule := &model.ExamTargetRule{
		ExamID:     examID,
		ClassID:    req.ClassID,
		GradeLevel: req.GradeLevel,
		MajorCode:  req.MajorCode,
		Religion:   req.Religion,
	}

	if err := h.examService.AddTargetRule(c.Request.Context(), rule); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"target_rule": rule})
}

// GetTargetRules godoc
// GET /api/v1/admin/exams/:id/target-rules
// Retrieves target rules for an exam.
func (h *ExamHandler) GetTargetRules(c *gin.Context) {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	rules, err := h.examService.GetTargetRules(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	if rules == nil {
		rules = []model.ExamTargetRule{}
	}

	response.Success(c, http.StatusOK, gin.H{"target_rules": rules})
}

// RefreshExamCache godoc
// POST /api/v1/admin/exams/:exam_id/refresh-cache
// Re-caches the exam payload + answer key to Redis after question changes.
func (h *ExamHandler) RefreshExamCache(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	authorFilter := claims.UserID
	for _, p := range claims.Permissions {
		if p == "exams:write_all" {
			authorFilter = 0
			break
		}
	}

	if err := h.examService.RefreshCache(c.Request.Context(), examID, authorFilter); err != nil {
		switch {
		case errors.Is(err, service.ErrNotExamAuthor):
			response.Fail(c, http.StatusForbidden, response.ErrNotExamAuthor)
		case errors.Is(err, service.ErrExamNotPublished):
			response.Fail(c, http.StatusBadRequest, response.ErrExamNotPublished)
		case errors.Is(err, service.ErrNoQuestions):
			response.Fail(c, http.StatusBadRequest, response.ErrNoQuestions)
		default:
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "exam cache refreshed successfully"})
}

// GetExamResults godoc
// GET /api/v1/admin/exams/:exam_id/results
// Returns paginated student results for an exam, optionally filtered by class_id.
func (h *ExamHandler) GetExamResults(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

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

	var gradeLevel *string
	if glStr := c.Query("grade_level"); glStr != "" {
		gradeLevel = &glStr
	}

	var majorCode *string
	if mcStr := c.Query("major_code"); mcStr != "" {
		majorCode = &mcStr
	}

	var groupNumber *int
	if gnStr := c.Query("group_number"); gnStr != "" {
		gn, err := strconv.Atoi(gnStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.ErrValidation)
			return
		}
		groupNumber = &gn
	}

	var religion *string
	if relStr := c.Query("religion"); relStr != "" {
		religion = &relStr
	}

	results, total, err := h.sessionService.GetExamResults(c.Request.Context(), examID, page, perPage, classID, gradeLevel, majorCode, groupNumber, religion)
	if err != nil {
		response.FailWithFields(c, http.StatusInternalServerError, response.ErrInternal, map[string]string{"error": err.Error()})
		return
	}

	pagination := &response.Pagination{
		Page:       page,
		PerPage:    perPage,
		TotalItems: int(total),
		TotalPages: int(math.Ceil(float64(total) / float64(perPage))),
	}

	response.SuccessWithPagination(c, http.StatusOK, gin.H{"results": results}, pagination)
}

// GetExam godoc
// GET /api/v1/admin/exams/:id
// Retrieves a single exam by ID.
func (h *ExamHandler) GetExam(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	exam, err := h.examService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrInvalidID) // Or a specific NotFound error
		return
	}

	response.Success(c, http.StatusOK, gin.H{"exam": exam})
}

// UpdateExam godoc
// PUT /api/v1/admin/exams/:id
// Updates an existing draft exam.
func (h *ExamHandler) UpdateExam(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.UpdateExamRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	// Fetch existing to overlay changes (or service can handle it, but handler doing it is fine for partial updates)
	existing, err := h.examService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrInvalidID)
		return
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.SubjectID != nil {
		if *req.SubjectID == 0 {
			existing.SubjectID = nil
		} else {
			existing.SubjectID = req.SubjectID
		}
	}
	if req.ScheduledStart != nil {
		existing.ScheduledStart = req.ScheduledStart
	}
	if req.ScheduledEnd != nil {
		existing.ScheduledEnd = req.ScheduledEnd
	}
	if req.DurationMinutes > 0 {
		existing.DurationMinutes = req.DurationMinutes
	}

	authorFilter := claims.UserID
	for _, p := range claims.Permissions {
		if p == "exams:write_all" {
			authorFilter = 0
			break
		}
	}

	if err := h.examService.Update(c.Request.Context(), authorFilter, existing); err != nil {
		switch {
		case errors.Is(err, service.ErrNotExamAuthor):
			response.Fail(c, http.StatusForbidden, response.ErrNotExamAuthor)
		case errors.Is(err, service.ErrExamNotDraft):
			response.Fail(c, http.StatusBadRequest, response.ErrExamNotDraft)
		default:
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"exam": existing})
}

// DeleteExam godoc
// DELETE /api/v1/admin/exams/:id
// Deletes a draft exam.
func (h *ExamHandler) DeleteExam(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	authorFilter := claims.UserID
	for _, p := range claims.Permissions {
		if p == "exams:write_all" {
			authorFilter = 0
			break
		}
	}

	if err := h.examService.Delete(c.Request.Context(), id, authorFilter); err != nil {
		switch {
		case errors.Is(err, service.ErrNotExamAuthor):
			response.Fail(c, http.StatusForbidden, response.ErrNotExamAuthor)
		case errors.Is(err, service.ErrExamNotDraft):
			response.Fail(c, http.StatusBadRequest, response.ErrExamNotDraft)
		default:
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "exam deleted"})
}
