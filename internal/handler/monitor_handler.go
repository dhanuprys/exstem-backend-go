package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/middleware"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

const (
	refreshInterval   = 15 * time.Second
	keepAliveInterval = 30 * time.Second
	refreshTimeout    = 5 * time.Second // prevent slow queries from blocking the SSE loop
)

type MonitorHandler struct {
	rdb            *redis.Client
	examService    *service.ExamService
	sessionService *service.ExamSessionService
	monitorService *service.MonitorService
	log            zerolog.Logger
}

func NewMonitorHandler(
	rdb *redis.Client,
	examService *service.ExamService,
	sessionService *service.ExamSessionService,
	monitorService *service.MonitorService,
	log zerolog.Logger,
) *MonitorHandler {
	return &MonitorHandler{
		rdb:            rdb,
		examService:    examService,
		sessionService: sessionService,
		monitorService: monitorService,
		log:            log.With().Str("component", "monitor_handler").Logger(),
	}
}

// MonitorExamSSE godoc
// GET /api/v1/admin/exams/:id/monitor
func (h *MonitorHandler) MonitorExamSSE(c *gin.Context) {
	// 1. Auth check
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Fail(c, http.StatusUnauthorized, response.ErrTokenRequired)
		return
	}

	hasPerm := false
	for _, p := range claims.Permissions {
		if p == "exams:write" {
			hasPerm = true
			break
		}
	}
	if !hasPerm {
		response.Fail(c, http.StatusForbidden, response.ErrForbidden)
		return
	}

	examID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	exam, err := h.examService.GetByID(c.Request.Context(), examID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.ErrNotFound)
		return
	}

	reqCtx := c.Request.Context()

	// 2. SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	totalQuestions := exam.QuestionCount

	// 3. Build & send initial snapshot
	h.sendInitialSnapshot(c, reqCtx, examID, exam, totalQuestions)

	// 4. Subscribe to Redis Pub/Sub
	channelName := config.CacheKey.ExamMonitorChannel(examID.String())
	pubsub := h.rdb.Subscribe(reqCtx, channelName)
	defer pubsub.Close()

	ch := pubsub.Channel()

	keepAliveTicker := time.NewTicker(keepAliveInterval)
	defer keepAliveTicker.Stop()

	refreshTicker := time.NewTicker(refreshInterval)
	defer refreshTicker.Stop()

	// Track whether any student has joined so we can skip empty refreshes
	hasStudents := false

	h.log.Info().Str("exam_id", examID.String()).Msg("Admin attached to live monitor SSE")

	// Pre-allocate a reusable ping payload (never changes)
	pingPayload, _ := json.Marshal(map[string]string{"type": "ping"})

	for {
		select {
		case <-reqCtx.Done():
			h.log.Info().Str("exam_id", examID.String()).Msg("Admin disconnected from live monitor SSE")
			return

		case msg := <-ch:
			// Forward raw JSON directly — no deserialization needed
			c.Writer.Write([]byte("data: "))
			c.Writer.Write([]byte(msg.Payload))
			c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()

			// Mark that we have students (a join/submit/cheat event proves it)
			hasStudents = true

		case <-refreshTicker.C:
			if !hasStudents {
				continue // no point querying if nobody has joined
			}
			h.sendRefresh(c, reqCtx, examID, totalQuestions)

		case <-keepAliveTicker.C:
			c.Writer.Write([]byte("data: "))
			c.Writer.Write(pingPayload)
			c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		}
	}
}

// sendInitialSnapshot gathers data and writes the first SSE event.
func (h *MonitorHandler) sendInitialSnapshot(
	c *gin.Context,
	ctx context.Context,
	examID uuid.UUID,
	exam *model.Exam,
	totalQuestions int,
) {
	results, _, _ := h.sessionService.GetExamResults(ctx, examID, 1, 1000, nil, nil, nil, nil, nil)

	totalJoined := len(results)
	totalInProgress := 0
	totalCompleted := 0

	studentsSnapshot := make([]map[string]interface{}, 0, len(results))
	for _, res := range results {
		if res.Status == "IN_PROGRESS" {
			totalInProgress++
		} else if res.Status == "COMPLETED" {
			totalCompleted++
		}

		var score float64
		if res.FinalScore != nil {
			score = *res.FinalScore
		}

		studentsSnapshot = append(studentsSnapshot, map[string]interface{}{
			"student_id":      res.StudentID,
			"name":            res.Name,
			"class_name":      res.ClassName,
			"status":          res.Status,
			"score":           score,
			"started_at":      res.StartedAt,
			"answered_count":  int64(0),
			"cheat_count":     int64(0),
			"total_questions": totalQuestions,
		})
	}

	// Fetch counts with a timeout so a slow query doesn't block the connection
	var initialTotalCheats int64
	fetchCtx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()

	if progress, err := h.monitorService.GetStudentProgress(fetchCtx, examID); err == nil {
		initialTotalCheats = progress.TotalCheats
		for i, s := range studentsSnapshot {
			sid, ok := s["student_id"].(int)
			if !ok {
				continue
			}
			if count, found := progress.AnsweredCounts[sid]; found {
				studentsSnapshot[i]["answered_count"] = count
			}
			if count, found := progress.CheatCounts[sid]; found {
				studentsSnapshot[i]["cheat_count"] = count
			}
		}
	}

	c.SSEvent("message", map[string]interface{}{
		"type": "snapshot",
		"data": map[string]interface{}{
			"exam": map[string]interface{}{
				"id":              examID.String(),
				"title":           exam.Title,
				"duration":        exam.DurationMinutes,
				"total_questions": totalQuestions,
			},
			"stats": map[string]interface{}{
				"total_joined":      totalJoined,
				"total_in_progress": totalInProgress,
				"total_completed":   totalCompleted,
				"total_cheats":      initialTotalCheats,
			},
			"students": studentsSnapshot,
		},
	})
	c.Writer.Flush()
}

// sendRefresh polls DB+Redis for current progress and sends a compact refresh event.
func (h *MonitorHandler) sendRefresh(c *gin.Context, parentCtx context.Context, examID uuid.UUID, totalQuestions int) {
	// Scoped timeout prevents a slow query from stalling the SSE loop
	ctx, cancel := context.WithTimeout(parentCtx, refreshTimeout)
	defer cancel()

	progress, err := h.monitorService.GetStudentProgress(ctx, examID)
	if err != nil {
		h.log.Warn().Err(err).Msg("Failed to fetch student progress for refresh")
		return
	}

	// Single-pass merge: iterate answered counts, decorate with cheat counts
	progressData := make([]map[string]interface{}, 0, len(progress.AnsweredCounts)+len(progress.CheatCounts))

	for sid, answered := range progress.AnsweredCounts {
		progressData = append(progressData, map[string]interface{}{
			"student_id":     sid,
			"answered_count": answered,
			"cheat_count":    progress.CheatCounts[sid], // 0 if missing
		})
		delete(progress.CheatCounts, sid) // mark as handled
	}

	// Remaining cheat-only students (already submitted, not in-progress)
	for sid, cheats := range progress.CheatCounts {
		progressData = append(progressData, map[string]interface{}{
			"student_id":     sid,
			"answered_count": int64(0),
			"cheat_count":    cheats,
		})
	}

	c.SSEvent("message", map[string]interface{}{
		"type":            "refresh",
		"total_questions": totalQuestions,
		"total_cheats":    progress.TotalCheats,
		"students":        progressData,
	})
	c.Writer.Flush()
}
