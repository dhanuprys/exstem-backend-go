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

// ScoringWorker consumes persist_scores_queue and updates exam_sessions in PostgreSQL.
type ScoringWorker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  zerolog.Logger
}

// NewScoringWorker creates a new ScoringWorker.
func NewScoringWorker(pool *pgxpool.Pool, rdb *redis.Client, log zerolog.Logger) *ScoringWorker {
	return &ScoringWorker{
		pool: pool,
		rdb:  rdb,
		log:  log.With().Str("component", "scoring_worker").Logger(),
	}
}

type scorePayload struct {
	StudentID int     `json:"student_id"`
	ExamID    string  `json:"exam_id"`
	Score     float64 `json:"score"`
}

// Start begins the infinite worker loop. Call in a goroutine.
func (w *ScoringWorker) Start(ctx context.Context) {
	w.log.Info().Msg("Worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("Worker stopping...")
			w.drain(context.Background())
			w.log.Info().Msg("Worker stopped")
			return
		default:
			w.processNext(ctx)
		}
	}
}

func (w *ScoringWorker) processNext(ctx context.Context) {
	result, err := w.rdb.BLPop(ctx, time.Second, "persist_scores_queue").Result()
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

	var payload scorePayload
	if err := json.Unmarshal([]byte(result[1]), &payload); err != nil {
		w.log.Error().Err(err).Msg("Unmarshal error")
		return
	}

	if err := w.persistScore(ctx, &payload); err != nil {
		w.log.Error().Err(err).
			Int("student_id", payload.StudentID).
			Str("exam_id", payload.ExamID).
			Msg("Persist error, retrying in 5s")
		w.rdb.RPush(ctx, "persist_scores_queue", result[1])
		time.Sleep(5 * time.Second)
	}
}

func (w *ScoringWorker) persistScore(ctx context.Context, p *scorePayload) error {
	examID, err := uuid.Parse(p.ExamID)
	if err != nil {
		return err
	}

	now := time.Now()
	_, err = w.pool.Exec(ctx,
		`UPDATE exam_sessions
		 SET status = 'COMPLETED', final_score = $1, finished_at = $2
		 WHERE exam_id = $3 AND student_id = $4`,
		p.Score, now, examID, p.StudentID,
	)
	return err
}

// drain processes all remaining items before shutdown.
func (w *ScoringWorker) drain(ctx context.Context) {
	drained := 0
	for {
		result, err := w.rdb.LPop(ctx, "persist_scores_queue").Result()
		if err != nil {
			break
		}

		var payload scorePayload
		if err := json.Unmarshal([]byte(result), &payload); err != nil {
			w.log.Error().Err(err).Msg("Drain unmarshal error")
			continue
		}

		if err := w.persistScore(ctx, &payload); err != nil {
			w.log.Error().Err(err).Msg("Drain persist error")
			w.rdb.RPush(ctx, "persist_scores_queue", result)
			break
		}
		drained++
	}

	if drained > 0 {
		w.log.Info().Int("count", drained).Msg("Drained remaining items")
	}
}
