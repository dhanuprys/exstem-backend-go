package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// AutosaveWorker consumes persist_answers_queue and UPSERTs answers to PostgreSQL.
type AutosaveWorker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  zerolog.Logger
}

// NewAutosaveWorker creates a new AutosaveWorker.
func NewAutosaveWorker(pool *pgxpool.Pool, rdb *redis.Client, log zerolog.Logger) *AutosaveWorker {
	return &AutosaveWorker{
		pool: pool,
		rdb:  rdb,
		log:  log.With().Str("component", "autosave_worker").Logger(),
	}
}

type answerPayload struct {
	StudentID int    `json:"student_id"`
	ExamID    string `json:"exam_id"`
	QID       string `json:"q_id"`
	Answer    string `json:"answer"`
}

// Start begins the infinite worker loop. Call in a goroutine.
func (w *AutosaveWorker) Start(ctx context.Context) {
	w.log.Info().Msg("Worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("Worker stopping...")
			// Drain remaining items before exit.
			w.drain(context.Background())
			w.log.Info().Msg("Worker stopped")
			return
		default:
			w.processNext(ctx)
		}
	}
}

func (w *AutosaveWorker) processNext(ctx context.Context) {
	// BLPop blocks until an item is available or timeout (1 second).
	result, err := w.rdb.BLPop(ctx, time.Second, "persist_answers_queue").Result()
	if err != nil {
		if err != redis.Nil && ctx.Err() == nil {
			if err.Error() != "redis: nil" {
				w.log.Error().Err(err).Msg("BLPop error")
			}
		}
		return
	}

	if len(result) < 2 {
		return
	}

	var payload answerPayload
	if err := json.Unmarshal([]byte(result[1]), &payload); err != nil {
		w.log.Error().Err(err).Msg("Unmarshal error")
		return
	}

	if err := w.persistAnswer(ctx, &payload); err != nil {
		w.log.Error().Err(err).
			Int("student_id", payload.StudentID).
			Str("exam_id", payload.ExamID).
			Msg("Persist error, retrying in 5s")
		// Push back to queue for retry.
		w.rdb.RPush(ctx, "persist_answers_queue", result[1])
		time.Sleep(5 * time.Second)
	}
}

func (w *AutosaveWorker) persistAnswer(ctx context.Context, p *answerPayload) error {
	examID, err := uuid.Parse(p.ExamID)
	if err != nil {
		return err
	}

	questionID, err := uuid.Parse(p.QID)
	if err != nil {
		return err
	}

	// UPSERT the answer — creates or updates without locking.
	_, err = w.pool.Exec(ctx,
		`INSERT INTO student_answers (exam_id, student_id, question_id, answer)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (exam_id, student_id, question_id) DO UPDATE
		 SET answer = EXCLUDED.answer, updated_at = NOW()`,
		examID, p.StudentID, questionID, p.Answer,
	)
	return err
}

// drain processes all remaining items in the queue before shutdown.
func (w *AutosaveWorker) drain(ctx context.Context) {
	drained := 0
	for {
		result, err := w.rdb.LPop(ctx, "persist_answers_queue").Result()
		if err != nil {
			break
		}

		var payload answerPayload
		if err := json.Unmarshal([]byte(result), &payload); err != nil {
			w.log.Error().Err(err).Msg("Drain unmarshal error")
			continue
		}

		if err := w.persistAnswer(ctx, &payload); err != nil {
			w.log.Error().Err(err).Msg("Drain persist error")
			w.rdb.RPush(ctx, "persist_answers_queue", result)
			break
		}
		drained++
	}

	if drained > 0 {
		w.log.Info().Int("count", drained).Msg("Drained remaining items")
	}
}
