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
	QuestionOrderBatchSize    = 50
	QuestionOrderBatchTimeout = 2 * time.Second
	QuestionOrderPollTimeout  = 1 * time.Second
)

type QuestionOrderWorker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  zerolog.Logger
}

func NewQuestionOrderWorker(pool *pgxpool.Pool, rdb *redis.Client, log zerolog.Logger) *QuestionOrderWorker {
	return &QuestionOrderWorker{
		pool: pool,
		rdb:  rdb,
		log:  log.With().Str("component", "question_order_worker").Logger(),
	}
}

type questionOrderPayload struct {
	ExamID    string   `json:"exam_id"`
	StudentID int      `json:"student_id"`
	Order     []string `json:"order"`
}

func (w *QuestionOrderWorker) Start(ctx context.Context) {
	w.log.Info().Msg("QuestionOrderWorker started")

	batch := make([]*questionOrderPayload, 0, QuestionOrderBatchSize)
	lastFlush := time.Now()

	for {
		if len(batch) > 0 &&
			(len(batch) >= QuestionOrderBatchSize || time.Since(lastFlush) >= QuestionOrderBatchTimeout) {

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
			item, err := w.rdb.BLPop(ctx, QuestionOrderPollTimeout, config.WorkerKey.PersistQuestionOrderQueue).Result()
			if err != nil {
				if err != redis.Nil && ctx.Err() == nil {
					w.log.Error().Err(err).Msg("BLPop error")
				}
				continue
			}

			if len(item) < 2 {
				continue
			}

			var p questionOrderPayload
			if err := json.Unmarshal([]byte(item[1]), &p); err != nil {
				w.log.Error().Err(err).Msg("Invalid JSON payload")
				continue
			}

			batch = append(batch, &p)
		}
	}
}

func (w *QuestionOrderWorker) flushSafe(ctx context.Context, batch []*questionOrderPayload) {
	if len(batch) == 0 {
		return
	}

	if err := w.bulkUpdate(ctx, batch); err != nil {
		w.log.Warn().Err(err).Msg("bulk question order update failed, using fallback")

		for _, p := range batch {
			if err := w.persistSingle(ctx, p); err != nil {
				w.log.Error().Err(err).Msg("persistSingle failed — requeueing")
				raw, _ := json.Marshal(p)
				w.rdb.RPush(ctx, config.WorkerKey.PersistQuestionOrderQueue, raw)
			}
		}
	}
}

func (w *QuestionOrderWorker) bulkUpdate(ctx context.Context, batch []*questionOrderPayload) error {
	n := len(batch)

	examIDs := make([]uuid.UUID, 0, n)
	students := make([]int, 0, n)
	ordersBytes := make([][]byte, 0, n)

	for _, p := range batch {
		eID, err := uuid.Parse(p.ExamID)
		if err != nil {
			return err
		}

		ob, _ := json.Marshal(p.Order)

		examIDs = append(examIDs, eID)
		students = append(students, p.StudentID)
		ordersBytes = append(ordersBytes, ob)
	}

	query := `
		UPDATE exam_sessions AS s
		SET question_order = t.qo
		FROM (
			SELECT 
				u.exam_id,
				u.student_id,
				u.qo
			FROM UNNEST(
				$1::uuid[],
				$2::int[],
				$3::jsonb[]
			) AS u (exam_id, student_id, qo)
		) AS t
		WHERE s.exam_id = t.exam_id
		  AND s.student_id = t.student_id
	`

	_, err := w.pool.Exec(ctx, query, examIDs, students, ordersBytes)
	return err
}

func (w *QuestionOrderWorker) persistSingle(ctx context.Context, p *questionOrderPayload) error {
	eID, err := uuid.Parse(p.ExamID)
	if err != nil {
		return err
	}

	ob, _ := json.Marshal(p.Order)

	_, err = w.pool.Exec(ctx,
		`UPDATE exam_sessions
		 SET question_order = $1
		 WHERE exam_id = $2 AND student_id = $3`,
		ob, eID, p.StudentID,
	)

	return err
}
