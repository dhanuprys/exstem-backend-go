package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5" // useful if you need specific error checking
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
)

const (
	BatchSize    = 50
	BatchTimeout = 2 * time.Second
	PollTimeout  = 1 * time.Second // Must be >= 1s to satisfy Redis
)

type CheatWorker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  zerolog.Logger
}

func NewCheatWorker(pool *pgxpool.Pool, rdb *redis.Client, log zerolog.Logger) *CheatWorker {
	return &CheatWorker{
		pool: pool,
		rdb:  rdb,
		log:  log.With().Str("component", "cheat_worker").Logger(),
	}
}

type cheatPayload struct {
	StudentID int    `json:"student_id"`
	ExamID    string `json:"exam_id"`
	Timestamp int64  `json:"timestamp"`
	Payload   string `json:"payload"`
}

func (w *CheatWorker) Start(ctx context.Context) {
	w.log.Info().Msg("CheatWorker started (Production Mode)")

	buffer := make([]*cheatPayload, 0, BatchSize)
	lastFlushTime := time.Now()

	for {
		// 1. Check Flush Conditions (Time or Size)
		if len(buffer) > 0 {
			if len(buffer) >= BatchSize || time.Since(lastFlushTime) >= BatchTimeout {
				w.flushSafe(ctx, buffer)
				buffer = buffer[:0] // Clear buffer, keep capacity
				lastFlushTime = time.Now()
			}
		}

		// 2. Check Context (Graceful Shutdown)
		select {
		case <-ctx.Done():
			w.shutdown(buffer)
			return
		default:
			// Continue
		}

		// 3. Fetch from Redis
		// BLPop blocks for 1 second. Returns immediately if data exists.
		result, err := w.rdb.BLPop(ctx, PollTimeout, config.WorkerKey.PersistCheatsQueue).Result()

		if err != nil {
			if err == redis.Nil {
				continue // Timeout (Queue empty), loop back to check flush timer
			}
			if ctx.Err() != nil {
				return // Context cancelled
			}
			// Real Redis error (e.g., connection lost)
			w.log.Error().Err(err).Msg("Redis connection error, sleeping 3s")
			time.Sleep(3 * time.Second)
			continue
		}

		// 4. Process Data
		if len(result) < 2 {
			continue
		}

		var payload cheatPayload
		if err := json.Unmarshal([]byte(result[1]), &payload); err != nil {
			// If JSON is malformed, we CANNOT retry it. Log and discard.
			w.log.Error().Err(err).Str("data", result[1]).Msg("Discarding malformed JSON")
			continue
		}

		buffer = append(buffer, &payload)
	}
}

// flushSafe attempts bulk insert, then fallback insert, then requeue
func (w *CheatWorker) flushSafe(ctx context.Context, batch []*cheatPayload) {
	// Try Fast Path: Bulk Insert
	if err := w.bulkInsert(ctx, batch); err != nil {
		w.log.Warn().Err(err).Int("count", len(batch)).Msg("Bulk insert failed, attempting row-by-row recovery")

		// Fallback Path: Insert one by one
		w.fallbackInsert(ctx, batch)
	}
}

func (w *CheatWorker) bulkInsert(ctx context.Context, batch []*cheatPayload) error {
	rows := make([][]interface{}, 0, len(batch))
	for _, p := range batch {
		examID, err := uuid.Parse(p.ExamID)
		if err != nil {
			// Return error to trigger fallback, which will handle the bad UUID individually
			return err
		}
		rows = append(rows, []interface{}{
			examID, p.StudentID, p.Payload, time.Unix(p.Timestamp, 0),
		})
	}

	_, err := w.pool.CopyFrom(
		ctx,
		pgx.Identifier{"exam_cheats"},
		[]string{"exam_id", "student_id", "event_data", "recorded_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func (w *CheatWorker) fallbackInsert(ctx context.Context, batch []*cheatPayload) {
	requeueList := make([]*cheatPayload, 0)

	for _, p := range batch {
		examID, err := uuid.Parse(p.ExamID)
		if err != nil {
			w.log.Error().Str("exam_id", p.ExamID).Msg("Dropping cheat event with invalid UUID")
			continue
		}

		_, err = w.pool.Exec(ctx,
			`INSERT INTO exam_cheats (exam_id, student_id, event_data, recorded_at)
             VALUES ($1, $2, $3::jsonb, $4)`,
			examID, p.StudentID, p.Payload, time.Unix(p.Timestamp, 0),
		)

		if err != nil {
			// Identify if this is a data error or a connection error.
			// Ideally, we only requeue on connection errors.
			// But for safety, we requeue everything that fails SQL insert
			// (except obvious constraint violations if you want to be specific).

			w.log.Error().Err(err).Int("student_id", p.StudentID).Msg("Insert failed, requeueing")
			requeueList = append(requeueList, p)
		}
	}

	// If we have items to requeue (DB was down), push them back to Redis
	if len(requeueList) > 0 {
		w.requeue(ctx, requeueList)
	}
}

func (w *CheatWorker) requeue(ctx context.Context, items []*cheatPayload) {
	// Use a pipeline to push everything back quickly
	pipe := w.rdb.Pipeline()
	for _, p := range items {
		data, _ := json.Marshal(p)
		pipe.RPush(ctx, config.WorkerKey.PersistCheatsQueue, data)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		w.log.Error().Err(err).Msg("CRITICAL: Failed to requeue items to Redis. Data loss occurred.")
	} else {
		w.log.Info().Int("count", len(items)).Msg("Requeued failed items back to Redis")
		// Sleep a bit to avoid thrashing if the DB is down hard
		time.Sleep(2 * time.Second)
	}
}

func (w *CheatWorker) shutdown(buffer []*cheatPayload) {
	w.log.Info().Msg("Worker stopping, flushing remaining buffer...")

	// Give it 5 seconds to flush to DB
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if len(buffer) > 0 {
		w.flushSafe(shutdownCtx, buffer)
	}
}
