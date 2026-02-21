package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/service"
	ws "github.com/stemsi/exstem-backend/internal/websocket"
)

// buildUpgrader creates a WebSocket upgrader with origin validation.
// allowedOrigins comes from config.Config.AllowedOrigins.
// An empty slice permits all origins (development mode).
func buildUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if len(allowedOrigins) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			for _, allowed := range allowedOrigins {
				if strings.EqualFold(allowed, origin) {
					return true
				}
			}
			return false
		},
	}
}

// WSHandler handles WebSocket exam streaming.
type WSHandler struct {
	rdb            *redis.Client
	examService    *service.ExamService
	sessionService *service.ExamSessionService
	log            zerolog.Logger
	upgrader       websocket.Upgrader
}

// NewWSHandler creates a new WSHandler.
func NewWSHandler(rdb *redis.Client, examService *service.ExamService, sessionService *service.ExamSessionService, log zerolog.Logger, allowedOrigins []string) *WSHandler {
	return &WSHandler{
		rdb:            rdb,
		examService:    examService,
		sessionService: sessionService,
		log:            log.With().Str("component", "ws_handler").Logger(),
		upgrader:       buildUpgrader(allowedOrigins),
	}
}

// ws.RequestPayload and ws.ResponsePayload are used from the shared package.

// ExamWebSocketStream godoc
// WS /ws/v1/student/exams/:exam_id/stream
// Upgrades to WebSocket for real-time autosave and instant grading.
func (h *WSHandler) ExamWebSocketStream(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	examID, err := uuid.Parse(c.Param("exam_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid exam ID"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	studentID := claims.UserID

	// SECURITY: Validate the student has an active session before streaming.
	if err := h.sessionService.VerifyActiveSession(c.Request.Context(), examID, studentID); err != nil {
		ws.WriteError(conn, "no active session for this exam")
		return
	}
	answersKey := fmt.Sprintf("student:%d:exam:%s:answers", studentID, examID)

	wsLog := h.log.With().
		Int("student_id", studentID).
		Str("exam_id", examID.String()).
		Logger()

	wsLog.Info().Msg("Student connected")

	for {
		// Use helper to read message with deadline handling.
		var msg ws.RequestPayload
		err := ws.ReadJSON(conn, &msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				wsLog.Warn().Err(err).Msg("Unexpected close")
			} else {
				wsLog.Debug().Msg("Connection closed")
			}
			break
		}

		switch msg.Action {
		case ws.ActionAutosave:
			h.handleAutosave(conn, answersKey, studentID, examID, &msg)
		case ws.ActionSubmit:
			h.handleSubmit(conn, wsLog, answersKey, studentID, examID)
		default:
			wsLog.Warn().Str("action", string(msg.Action)).Msg("Unknown action")
			ws.WriteError(conn, "unknown action: "+string(msg.Action))
		}
	}
}

// handleAutosave saves a single answer to Redis and queues it for persistence.
func (h *WSHandler) handleAutosave(conn *websocket.Conn, answersKey string, studentID int, examID uuid.UUID, msg *ws.RequestPayload) {
	ctx := context.Background()

	if msg.QID == "" || msg.Answer == "" {
		ws.WriteError(conn, "q_id and ans are required")
		return
	}

	// SECURITY: Validate QID is a well-formed UUID to prevent Redis key injection.
	if _, err := uuid.Parse(msg.QID); err != nil {
		ws.WriteError(conn, "invalid q_id format")
		return
	}

	if err := h.rdb.HSet(ctx, answersKey, msg.QID, msg.Answer).Err(); err != nil {
		h.log.Error().Err(err).Int("student_id", studentID).Msg("Autosave Redis error")
		ws.WriteError(conn, "save failed")
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"student_id": studentID,
		"exam_id":    examID.String(),
		"q_id":       msg.QID,
		"answer":     msg.Answer,
	})
	h.rdb.RPush(ctx, "persist_answers_queue", payload)

	ws.WriteJSON(conn, ws.EventSuccess, map[string]string{"status": "saved"})
}

// handleSubmit grades the exam in RAM and queues the score for persistence.
func (h *WSHandler) handleSubmit(conn *websocket.Conn, wsLog zerolog.Logger, answersKey string, studentID int, examID uuid.UUID) {
	ctx := context.Background()

	answerKey, err := h.examService.GetAnswerKey(ctx, examID)
	if err != nil {
		wsLog.Error().Err(err).Msg("Get answer key error")
		ws.WriteError(conn, "grading failed")
		return
	}

	studentAnswers, err := h.rdb.HGetAll(ctx, answersKey).Result()
	if err != nil {
		wsLog.Error().Err(err).Msg("Get student answers error")
		ws.WriteError(conn, "failed to get answers")
		return
	}

	correct := 0
	total := len(answerKey)
	for qID, correctAns := range answerKey {
		if studentAns, ok := studentAnswers[qID]; ok && studentAns == correctAns {
			correct++
		}
	}

	var score float64
	if total > 0 {
		score = (float64(correct) / float64(total)) * 100
	}

	scorePayload, _ := json.Marshal(map[string]interface{}{
		"student_id": studentID,
		"exam_id":    examID.String(),
		"score":      score,
	})
	h.rdb.RPush(ctx, "persist_scores_queue", scorePayload)

	wsLog.Info().
		Float64("score", score).
		Int("correct", correct).
		Int("total", total).
		Msg("Exam submitted and graded")

	// Send scalable response: nested data
	responseData := map[string]interface{}{
		"status": "completed",
		"score":  score,
	}
	ws.WriteJSON(conn, ws.EventGraded, responseData)
}
