package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// QuestionHandler handles question management endpoints.
type QuestionHandler struct {
	questionService *service.QuestionService
}

// NewQuestionHandler creates a new QuestionHandler.
func NewQuestionHandler(questionService *service.QuestionService) *QuestionHandler {
	return &QuestionHandler{questionService: questionService}
}

// ListQBanks godoc
// GET /api/v1/admin/qbanks
// Lists question banks. Users with `qbanks:write_all` see all, others see only their own.
func (h *QuestionHandler) ListQBanks(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	search := c.Query("search")

	// Determine scope: write_all sees everything, everyone else sees only their own.
	var authorID *int
	hasWriteAll := false
	for _, p := range claims.Permissions {
		if p == string(model.PermissionQBanksWriteAll) {
			hasWriteAll = true
			break
		}
	}
	if !hasWriteAll {
		authorID = &claims.UserID
	}

	qbanks, pagination, err := h.questionService.ListQBanks(c.Request.Context(), authorID, page, perPage, search)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.SuccessWithPagination(c, http.StatusOK, qbanks, pagination)
}

// GetQBanks godoc
// GET /api/v1/admin/qbanks/:id
// Gets a specific question bank.
func (h *QuestionHandler) GetQBanks(c *gin.Context) {
	qbankID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	qbank, err := h.questionService.GetQBanks(c.Request.Context(), qbankID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	if qbank == nil {
		qbank = &model.QuestionBank{}
	}

	response.Success(c, http.StatusOK, qbank)
}

// CreateQBanks godoc
// POST /api/v1/admin/qbanks
// Creates a new question bank.
func (h *QuestionHandler) CreateQBanks(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	var req model.CreateQuestionBankRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	qbank := &model.QuestionBank{
		AuthorID:    &claims.UserID,
		Name:        req.Name,
		Description: req.Description,
		SubjectID:   req.SubjectID,
	}

	if err := h.questionService.CreateQBanks(c.Request.Context(), qbank); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, qbank)
}

// UpdateQBanks godoc
// PUT /api/v1/admin/qbanks/:id
// Updates a specific question bank.
func (h *QuestionHandler) UpdateQBanks(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	qbankID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.CreateQuestionBankRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	qbank := &model.QuestionBank{
		ID:          qbankID,
		AuthorID:    &claims.UserID,
		Name:        req.Name,
		Description: req.Description,
		SubjectID:   req.SubjectID,
	}

	if err := h.questionService.UpdateQBanks(c.Request.Context(), qbank); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, qbank)
}

// DeleteQBanks godoc
// DELETE /api/v1/admin/qbanks/:id
// Deletes a specific question bank.
func (h *QuestionHandler) DeleteQBanks(c *gin.Context) {
	qbankID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.questionService.DeleteQBanks(c.Request.Context(), qbankID); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "question bank deleted successfully"})
}

// ListQuestions godoc
// GET /api/v1/admin/qbanks/:qbank_id/questions
// Lists all questions for a qbank.
func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	qbankID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	questions, err := h.questionService.ListByQBank(c.Request.Context(), qbankID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	if questions == nil {
		questions = []model.Question{}
	}

	response.Success(c, http.StatusOK, questions)
}

// AddQuestion godoc
// POST /api/v1/admin/qbanks/:qbank_id/questions
// Adds a question to a qbank.
func (h *QuestionHandler) AddQuestion(c *gin.Context) {
	qbankID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.AddQuestionRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	question := &model.Question{
		QBankID:       qbankID,
		QuestionText:  req.QuestionText,
		QuestionType:  model.QuestionType(req.QuestionType),
		Options:       req.Options,
		CorrectOption: req.CorrectOption,
		OrderNum:      req.OrderNum,
	}

	if err := h.questionService.Create(c.Request.Context(), question); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, question)
}

// ReplaceQuestions godoc
// PUT /api/v1/admin/qbanks/:qbank_id/questions
// Bulk replaces all questions for a qbank.
func (h *QuestionHandler) ReplaceQuestions(c *gin.Context) {
	qbankID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.ReplaceQuestionsRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	questions := make([]model.Question, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = model.Question{
			QBankID:       qbankID,
			QuestionText:  q.QuestionText,
			QuestionType:  model.QuestionType(q.QuestionType),
			Options:       q.Options,
			CorrectOption: q.CorrectOption,
			OrderNum:      q.OrderNum,
		}
	}

	if err := h.questionService.ReplaceAll(c.Request.Context(), qbankID, questions); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "questions replaced successfully"})
}
