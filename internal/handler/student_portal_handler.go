package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// StudentPortalHandler handles student-facing endpoints (exam taking, lobby).
type StudentPortalHandler struct {
	sessionService *service.ExamSessionService
	examService    *service.ExamService
}

// NewStudentPortalHandler creates a new StudentPortalHandler.
func NewStudentPortalHandler(
	sessionService *service.ExamSessionService,
	examService *service.ExamService,
) *StudentPortalHandler {
	return &StudentPortalHandler{
		sessionService: sessionService,
		examService:    examService,
	}
}

// GetLobby godoc
// GET /api/v1/student/lobby
// Returns exams available to the student based on class targeting rules.
func (h *StudentPortalHandler) GetLobby(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	lobby, err := h.sessionService.GetLobby(c.Request.Context(), claims.UserID, claims.ClassID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	if lobby == nil {
		lobby = []service.LobbyExam{}
	}

	response.Success(c, http.StatusOK, gin.H{"exams": lobby})
}

// JoinExam godoc
// POST /api/v1/student/exams/:exam_id/join
// Validates entry token and creates a session (idempotent).
func (h *StudentPortalHandler) JoinExam(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	examID, err := uuid.Parse(c.Param("exam_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.JoinExamRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	session, err := h.sessionService.JoinExam(c.Request.Context(), examID, claims.UserID, claims.ClassID, req.EntryToken)
	if err != nil {
		// Distinguish error types for specific codes.
		errMsg := err.Error()
		switch errMsg {
		case "invalid entry token":
			response.Fail(c, http.StatusBadRequest, response.ErrInvalidEntryToken)
		case "exam is not available for joining":
			response.Fail(c, http.StatusBadRequest, response.ErrExamNotAvailable)
		default:
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"session": session})
}

// GetExamPaper godoc
// GET /api/v1/student/exams/:exam_id/paper
// Returns the exam payload from Redis (bypasses PostgreSQL).
// SECURITY: Requires an active session for this exam — prevents IDOR.
func (h *StudentPortalHandler) GetExamPaper(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	examID, err := uuid.Parse(c.Param("exam_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	// SECURITY: Verify the student has an active session for this exam.
	// This prevents students from downloading exam papers they have not joined.
	if err := h.sessionService.VerifyActiveSession(c.Request.Context(), examID, claims.UserID); err != nil {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden)
		return
	}

	payload, err := h.examService.GetExamPayload(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrExamNotPublished)
		return
	}

	response.Success(c, http.StatusOK, payload)
}

// GetExamState godoc
// GET /api/v1/student/exams/:exam_id/state
// Returns the current state of the exam for the student.
// This endpoint will cover the page reload, so the frontend can get the answered questions and the remaining time.
func (h *StudentPortalHandler) GetExamState(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	examID, err := uuid.Parse(c.Param("exam_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	// SECURITY: Verify the student has an active session for this exam.
	if err := h.sessionService.VerifyActiveSession(c.Request.Context(), examID, claims.UserID); err != nil {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden)
		return
	}

	state, err := h.sessionService.GetExamState(c.Request.Context(), examID, claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, state)
}
