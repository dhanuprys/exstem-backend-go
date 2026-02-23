package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/service"
	ws "github.com/stemsi/exstem-backend/internal/websocket"
)

// buildUpgrader creates a WebSocket upgrader with origin validation.
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

type WSHandler struct {
	rdb            *redis.Client
	examService    *service.ExamService
	sessionService *service.ExamSessionService
	log            zerolog.Logger
	upgrader       websocket.Upgrader
}

func NewWSHandler(rdb *redis.Client, examService *service.ExamService, sessionService *service.ExamSessionService, log zerolog.Logger, allowedOrigins []string) *WSHandler {
	return &WSHandler{
		rdb:            rdb,
		examService:    examService,
		sessionService: sessionService,
		log:            log.With().Str("component", "ws_handler").Logger(),
		upgrader:       buildUpgrader(allowedOrigins),
	}
}

// ExamWebSocketStream godoc
// WS /ws/v1/student/exams/:exam_id/stream
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

	// SECURITY: Validate session exists
	if err := h.sessionService.VerifyActiveSession(c.Request.Context(), examID, studentID); err != nil {
		ws.WriteError(conn, "no active session for this exam")
		return
	}
	answersKey := config.CacheKey.StudentAnswersKey(examID.String(), studentID)

	wsLog := h.log.With().
		Int("student_id", studentID).
		Str("exam_id", examID.String()).
		Logger()

	wsLog.Info().Msg("Student connected")

	for {
		// 1. READ RAW BYTES (Critical Step)
		// We do not unmarshal into a specific struct yet.
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				wsLog.Warn().Err(err).Msg("Unexpected close")
			}
			break
		}

		// 2. PEEK AT THE ACTION
		var envelope ws.RequestEnvelope
		if err := json.Unmarshal(messageBytes, &envelope); err != nil {
			wsLog.Warn().Err(err).Msg("Invalid JSON format")
			continue
		}

		// Logging
		wsLog.Info().Interface("messageaction", envelope.Action).Msg("Received message")

		// 3. ROUTE TO SPECIFIC HANDLER based on Action
		switch envelope.Action {
		case ws.ActionAutosave:
			var req ws.AutosaveRequest
			if err := json.Unmarshal(messageBytes, &req); err != nil {
				ws.WriteError(conn, "invalid autosave format")
				continue
			}
			h.handleAutosave(conn, answersKey, studentID, examID, &req)

		case ws.ActionCheat:
			var req ws.CheatRequest
			// This works because CheatRequest has the 'Payload' field
			if err := json.Unmarshal(messageBytes, &req); err != nil {
				wsLog.Error().Err(err).Msg("Cheat payload unmarshal failed")
				continue
			}
			h.handleCheat(wsLog, studentID, examID, &req)

		case ws.ActionSubmit:
			h.handleSubmit(conn, wsLog, answersKey, studentID, examID)

		case ws.ActionPing:
			ws.WriteTyped(conn, ws.PongResponse{Event: ws.EventPong})

		default:
			wsLog.Warn().Str("action", string(envelope.Action)).Msg("Unknown action")
			ws.WriteError(conn, "unknown action: "+string(envelope.Action))
		}
	}
}

// handleCheat queues the cheat event for persistence.
func (h *WSHandler) handleCheat(wsLog zerolog.Logger, studentID int, examID uuid.UUID, msg *ws.CheatRequest) {
	ctx := context.Background()

	// We store the payload as json.RawMessage (bytes) so that when it goes
	// into Postgres JSONB, it is treated as a nested object, not a string.
	// However, since msg.Payload is a string (double encoded), we just pass it through.
	cheatEvent := map[string]interface{}{
		"student_id": studentID,
		"exam_id":    examID.String(),
		"timestamp":  time.Now().Unix(),
		"payload":    msg.Payload, // The raw string from client
	}

	data, _ := json.Marshal(cheatEvent)

	if err := h.rdb.RPush(ctx, config.WorkerKey.PersistCheatsQueue, data).Err(); err != nil {
		wsLog.Error().Err(err).Msg("Failed to queue cheat report")
	}

	// Security Best Practice: Do NOT acknowledge cheat events to the client.
	// Silent logging prevents hackers from probing the detection system.
}

// handleAutosave saves a single answer to Redis.
func (h *WSHandler) handleAutosave(conn *websocket.Conn, answersKey string, studentID int, examID uuid.UUID, msg *ws.AutosaveRequest) {
	ctx := context.Background()

	if msg.QID == "" {
		ws.WriteError(conn, "q_id is required")
		return
	}

	// Verify QID is valid UUID to prevent injection
	if _, err := uuid.Parse(msg.QID); err != nil {
		ws.WriteError(conn, "invalid q_id format")
		return
	}

	// Prepare persistence payload
	payload, _ := json.Marshal(map[string]interface{}{
		"student_id": studentID,
		"exam_id":    examID.String(),
		"q_id":       msg.QID,
		"answer":     msg.Answer,
	})

	// Handle Unanswer (Empty string)
	if msg.Answer == "" {
		if err := h.rdb.HDel(ctx, answersKey, msg.QID).Err(); err != nil {
			h.log.Error().Err(err).Int("student_id", studentID).Msg("Autosave Redis error")
			ws.WriteError(conn, "save failed")
			return
		}
		h.rdb.RPush(ctx, config.WorkerKey.PersistAnswersQueue, payload)
		ws.WriteTyped(conn, ws.AutosaveResponse{
			Event:  ws.EventSuccess,
			Status: "removed",
		})
		return
	}

	// Handle Save
	if err := h.rdb.HSet(ctx, answersKey, msg.QID, msg.Answer).Err(); err != nil {
		h.log.Error().Err(err).Int("student_id", studentID).Msg("Autosave Redis error")
		ws.WriteError(conn, "save failed")
		return
	}

	h.rdb.RPush(ctx, config.WorkerKey.PersistAnswersQueue, payload)

	ws.WriteTyped(conn, ws.AutosaveResponse{
		Event:  ws.EventSuccess,
		Status: "saved",
	})
}

// handleSubmit grades the exam in RAM.
func (h *WSHandler) handleSubmit(conn *websocket.Conn, wsLog zerolog.Logger, answersKey string, studentID int, examID uuid.UUID) {
	ctx := context.Background()

	// 1. Get correct answers (Cached in service layer usually)
	answerKey, err := h.examService.GetAnswerKey(ctx, examID)
	if err != nil {
		wsLog.Error().Err(err).Msg("Get answer key error")
		ws.WriteError(conn, "grading failed")
		return
	}

	// 2. Get student answers from Redis
	studentAnswers, err := h.rdb.HGetAll(ctx, answersKey).Result()
	if err != nil {
		wsLog.Error().Err(err).Msg("Get student answers error")
		ws.WriteError(conn, "failed to get answers")
		return
	}

	// 3. Get Student's Shuffled Questions
	orderedIDs, err := h.sessionService.GetShuffledQuestionIDs(ctx, examID, studentID)
	if err != nil {
		wsLog.Error().Err(err).Msg("Get student shuffled questions error")
		ws.WriteError(conn, "failed to get question subset")
		return
	}

	// 4. Grade it against their specific subset
	correct := 0
	total := len(orderedIDs)
	for _, qID := range orderedIDs {
		// Verify this question actually exists in the global answer key
		if correctAns, exists := answerKey[qID]; exists {
			if studentAns, answered := studentAnswers[qID]; answered && studentAns == correctAns {
				correct++
			}
		}
	}

	var score float64
	if total > 0 {
		score = (float64(correct) / float64(total)) * 100
	}

	// 4. Queue Score for Persistence
	scorePayload, _ := json.Marshal(map[string]interface{}{
		"student_id": studentID,
		"exam_id":    examID.String(),
		"score":      score,
	})
	h.rdb.RPush(ctx, config.WorkerKey.PersistScoresQueue, scorePayload)

	wsLog.Info().Float64("score", score).Msg("Exam submitted")

	ws.WriteTyped(conn, ws.GradedResponse{
		Event:  ws.EventGraded,
		Status: "completed",
		Score:  score,
	})
}
