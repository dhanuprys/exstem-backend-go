package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// ListQuestions godoc
// GET /api/v1/admin/exams/:exam_id/questions
// Lists all questions for an exam.
func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	questions, err := h.questionService.ListByExam(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	if questions == nil {
		questions = []model.Question{}
	}

	response.Success(c, http.StatusOK, gin.H{"questions": questions})
}

// AddQuestion godoc
// POST /api/v1/admin/exams/:exam_id/questions
// Adds a question to an exam.
func (h *QuestionHandler) AddQuestion(c *gin.Context) {
	examID, err := uuid.Parse(c.Param("id"))
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
		ExamID:        examID,
		QuestionText:  req.QuestionText,
		QuestionType:  model.QuestionType(req.QuestionType),
		Options:       req.Options,
		CorrectOption: req.CorrectOption,
		OrderNum:      req.OrderNum,
		ScoreValue:    req.ScoreValue,
	}

	if err := h.questionService.Create(c.Request.Context(), question); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"question": question})
}

// ReplaceQuestions godoc
// PUT /api/v1/admin/exams/:exam_id/questions
// Bulk replaces all questions for an exam.
func (h *QuestionHandler) ReplaceQuestions(c *gin.Context) {
	examID, err := uuid.Parse(c.Param("id"))
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
			ExamID:        examID,
			QuestionText:  q.QuestionText,
			QuestionType:  model.QuestionType(q.QuestionType),
			Options:       q.Options,
			CorrectOption: q.CorrectOption,
			OrderNum:      q.OrderNum,
			ScoreValue:    q.ScoreValue,
		}
	}

	if err := h.questionService.ReplaceAll(c.Request.Context(), examID, questions); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "questions replaced successfully"})
}
