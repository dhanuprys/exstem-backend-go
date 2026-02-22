package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/config"
)

const (
	ScoreBatchSize    = 50
	ScoreBatchTimeout = 2 * time.Second
	ScorePollTimeout  = 1 * time.Second
)

type ScoringWorker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  zerolog.Logger
}

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

// ----------------------------------------------------------------
// Worker loop with batching
// ----------------------------------------------------------------

func (w *ScoringWorker) Start(ctx context.Context) {
	w.log.Info().Msg("ScoringWorker started")

	batch := make([]*scorePayload, 0, ScoreBatchSize)
	lastFlush := time.Now()

	for {
		// Should flush?
		if len(batch) > 0 &&
			(len(batch) >= ScoreBatchSize || time.Since(lastFlush) >= ScoreBatchTimeout) {

			w.flushSafe(ctx, batch)
			batch = batch[:0]
			lastFlush = time.Now()
		}

		select {
		case <-ctx.Done():
			w.log.Info().Msg("Shutdown requested. Flushing remaining batch...")
			w.flushSafe(context.Background(), batch)
			return

		default:
			item, err := w.rdb.BLPop(ctx, ScorePollTimeout, config.WorkerKey.PersistScoresQueue).Result()
			if err != nil {
				if err != redis.Nil && ctx.Err() == nil {
					w.log.Error().Err(err).Msg("BLPop error")
				}
				continue
			}

			if len(item) < 2 {
				continue
			}

			var p scorePayload
			if err := json.Unmarshal([]byte(item[1]), &p); err != nil {
				w.log.Error().Err(err).Msg("Invalid JSON payload")
				continue
			}

			batch = append(batch, &p)
		}
	}
}

// ----------------------------------------------------------------
// Batch Upsert/Update Wrapper
// ----------------------------------------------------------------

func (w *ScoringWorker) flushSafe(ctx context.Context, batch []*scorePayload) {
	if len(batch) == 0 {
		return
	}

	if err := w.bulkUpdateScores(ctx, batch); err != nil {
		w.log.Warn().Err(err).Msg("bulk score update failed, using fallback")

		for _, p := range batch {
			if err := w.persistSingle(ctx, p); err != nil {
				w.log.Error().Err(err).Msg("persistSingle failed — requeueing")
				raw, _ := json.Marshal(p)
				w.rdb.RPush(ctx, config.WorkerKey.PersistScoresQueue, raw)
			}
		}
		return
	}

	// After successful score updates → delete autosave buffers in Redis
	w.bulkClearAutosavedAnswers(ctx, batch)
}

// ----------------------------------------------------------------
// BULK PostgreSQL UPDATE using UNNEST + alias
// ----------------------------------------------------------------

func (w *ScoringWorker) bulkUpdateScores(ctx context.Context, batch []*scorePayload) error {
	n := len(batch)

	examIDs := make([]uuid.UUID, 0, n)
	students := make([]int, 0, n)
	scores := make([]float64, 0, n)
	finishedAts := make([]time.Time, n)

	now := time.Now()
	for i, p := range batch {
		eID, err := uuid.Parse(p.ExamID)
		if err != nil {
			return err
		}
		examIDs = append(examIDs, eID)
		students = append(students, p.StudentID)
		scores = append(scores, p.Score)
		finishedAts[i] = now
	}

	query := `
		UPDATE exam_sessions AS s
		SET status = 'COMPLETED',
		    final_score = t.score,
		    finished_at = t.finished_at
		FROM (
			SELECT 
				u.exam_id,
				u.student_id,
				u.score,
				u.finished_at
			FROM UNNEST(
				$1::uuid[],
				$2::int[],
				$3::float8[],
				$4::timestamptz[]
			) AS u (exam_id, student_id, score, finished_at)
		) AS t
		WHERE s.exam_id = t.exam_id
		  AND s.student_id = t.student_id
	`

	_, err := w.pool.Exec(ctx, query, examIDs, students, scores, finishedAts)
	return err
}

// ----------------------------------------------------------------
// BULK Redis DEL for clearing autosaved answers
// ----------------------------------------------------------------

func (w *ScoringWorker) bulkClearAutosavedAnswers(ctx context.Context, batch []*scorePayload) {
	pipe := w.rdb.Pipeline()

	for _, p := range batch {
		key := config.CacheKey.StudentAnswersKey(p.ExamID, p.StudentID)
		pipe.Del(ctx, key)
	}

	_, _ = pipe.Exec(ctx)
}

// ----------------------------------------------------------------
// FALLBACK single update
// ----------------------------------------------------------------

func (w *ScoringWorker) persistSingle(ctx context.Context, p *scorePayload) error {
	eID, err := uuid.Parse(p.ExamID)
	if err != nil {
		return err
	}

	_, err = w.pool.Exec(ctx,
		`UPDATE exam_sessions
		 SET status = 'COMPLETED',
		     final_score = $1,
		     finished_at = NOW()
		 WHERE exam_id = $2 AND student_id = $3`,
		p.Score, eID, p.StudentID,
	)

	return err
}
