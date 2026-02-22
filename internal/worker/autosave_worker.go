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
	AutosaveBatchSize    = 50
	AutosaveBatchTimeout = 2 * time.Second
	AutosavePollTimeout  = 1 * time.Second
)

type AutosaveWorker struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  zerolog.Logger
}

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

func (w *AutosaveWorker) Start(ctx context.Context) {
	w.log.Info().Msg("AutosaveWorker started")

	buffer := make([]*answerPayload, 0, AutosaveBatchSize)
	lastFlush := time.Now()

	for {
		// 1. Should flush?
		if len(buffer) > 0 &&
			(time.Since(lastFlush) >= AutosaveBatchTimeout || len(buffer) >= AutosaveBatchSize) {

			w.flushSafe(ctx, buffer)
			buffer = buffer[:0]
			lastFlush = time.Now()
		}

		// 2. Shutdown?
		select {
		case <-ctx.Done():
			w.shutdown(buffer)
			return
		default:
		}

		// 3. Block & pop from Redis
		result, err := w.rdb.BLPop(ctx, AutosavePollTimeout, config.WorkerKey.PersistAnswersQueue).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			w.log.Error().Err(err).Msg("Redis error, backing off")
			time.Sleep(time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		var p answerPayload
		if err := json.Unmarshal([]byte(result[1]), &p); err != nil {
			w.log.Error().Err(err).Msg("Skipping malformed JSON")
			continue
		}

		buffer = append(buffer, &p)
	}
}

func (w *AutosaveWorker) flushSafe(ctx context.Context, batch []*answerPayload) {
	toUpsert := make([]*answerPayload, 0, len(batch))
	toDelete := make([]*answerPayload, 0, len(batch))

	for _, p := range batch {
		if p.Answer == "" {
			toDelete = append(toDelete, p)
		} else {
			toUpsert = append(toUpsert, p)
		}
	}

	// Upserts
	if len(toUpsert) > 0 {
		if err := w.bulkUpsert(ctx, toUpsert); err != nil {
			w.log.Warn().Err(err).Msg("Bulk upsert failed, using fallback")
			w.fallbackProcess(ctx, toUpsert)
		}
	}

	// Deletes
	if len(toDelete) > 0 {
		if err := w.bulkDelete(ctx, toDelete); err != nil {
			w.log.Warn().Err(err).Msg("Bulk delete failed, using fallback")
			w.fallbackProcess(ctx, toDelete)
		}
	}
}

///////////////////////////////////////////////////////////////////////////
// BULK UPSERT (optimized UNNEST + column aliases)
///////////////////////////////////////////////////////////////////////////

func (w *AutosaveWorker) bulkUpsert(ctx context.Context, batch []*answerPayload) error {
	n := len(batch)
	examIDs := make([]uuid.UUID, 0, n)
	students := make([]int, 0, n)
	questionIDs := make([]uuid.UUID, 0, n)
	answers := make([]string, 0, n)
	timestamps := make([]time.Time, n)

	now := time.Now()
	for i, p := range batch {
		eID, err1 := uuid.Parse(p.ExamID)
		qID, err2 := uuid.Parse(p.QID)
		if err1 != nil || err2 != nil {
			return err1
		}
		examIDs = append(examIDs, eID)
		students = append(students, p.StudentID)
		questionIDs = append(questionIDs, qID)
		answers = append(answers, p.Answer)
		timestamps[i] = now
	}

	query := `
		INSERT INTO student_answers (
			exam_id, student_id, question_id, answer, updated_at
		)
		SELECT 
			u.exam_id,
			u.student_id,
			u.question_id,
			u.answer,
			u.updated_at
		FROM UNNEST(
			$1::uuid[],
			$2::int[],
			$3::uuid[],
			$4::text[],
			$5::timestamptz[]
		) AS u (exam_id, student_id, question_id, answer, updated_at)
		ON CONFLICT (exam_id, student_id, question_id)
		DO UPDATE SET 
			answer = EXCLUDED.answer,
			updated_at = EXCLUDED.updated_at
	`

	_, err := w.pool.Exec(ctx, query, examIDs, students, questionIDs, answers, timestamps)
	return err
}

///////////////////////////////////////////////////////////////////////////
// BULK DELETE (optimized USING + aliases)
///////////////////////////////////////////////////////////////////////////

func (w *AutosaveWorker) bulkDelete(ctx context.Context, batch []*answerPayload) error {
	n := len(batch)
	examIDs := make([]uuid.UUID, 0, n)
	students := make([]int, 0, n)
	questionIDs := make([]uuid.UUID, 0, n)

	for _, p := range batch {
		eID, err1 := uuid.Parse(p.ExamID)
		qID, err2 := uuid.Parse(p.QID)
		if err1 != nil || err2 != nil {
			return err1
		}
		examIDs = append(examIDs, eID)
		students = append(students, p.StudentID)
		questionIDs = append(questionIDs, qID)
	}

	query := `
		DELETE FROM student_answers AS s
		USING (
			SELECT 
				u.exam_id,
				u.student_id,
				u.question_id
			FROM UNNEST(
				$1::uuid[],
				$2::int[],
				$3::uuid[]
			) AS u (exam_id, student_id, question_id)
		) AS u
		WHERE s.exam_id = u.exam_id
		  AND s.student_id = u.student_id
		  AND s.question_id = u.question_id
	`

	_, err := w.pool.Exec(ctx, query, examIDs, students, questionIDs)
	return err
}

///////////////////////////////////////////////////////////////////////////
// FALLBACK (single row)
///////////////////////////////////////////////////////////////////////////

func (w *AutosaveWorker) fallbackProcess(ctx context.Context, batch []*answerPayload) {
	requeue := make([]*answerPayload, 0)

	for _, p := range batch {
		if err := w.persistSingle(ctx, p); err != nil {
			w.log.Error().Err(err).
				Int("student_id", p.StudentID).
				Msg("Single persist failed, requeueing")
			requeue = append(requeue, p)
		}
	}

	if len(requeue) > 0 {
		w.requeue(ctx, requeue)
	}
}

func (w *AutosaveWorker) persistSingle(ctx context.Context, p *answerPayload) error {
	eID, err := uuid.Parse(p.ExamID)
	if err != nil {
		return nil
	}
	qID, err := uuid.Parse(p.QID)
	if err != nil {
		return nil
	}

	if p.Answer == "" {
		_, err = w.pool.Exec(ctx,
			`DELETE FROM student_answers 
			 WHERE exam_id=$1 AND student_id=$2 AND question_id=$3`,
			eID, p.StudentID, qID,
		)
		return err
	}

	_, err = w.pool.Exec(ctx,
		`INSERT INTO student_answers (exam_id, student_id, question_id, answer, updated_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (exam_id, student_id, question_id)
		 DO UPDATE SET 
			answer = EXCLUDED.answer,
			updated_at = NOW()`,
		eID, p.StudentID, qID, p.Answer,
	)
	return err
}

///////////////////////////////////////////////////////////////////////////
// REQUEUE
///////////////////////////////////////////////////////////////////////////

func (w *AutosaveWorker) requeue(ctx context.Context, items []*answerPayload) {
	pipe := w.rdb.Pipeline()
	for _, p := range items {
		data, _ := json.Marshal(p)
		pipe.RPush(ctx, config.WorkerKey.PersistAnswersQueue, data)
	}
	_, _ = pipe.Exec(ctx)
	time.Sleep(time.Second)
}

///////////////////////////////////////////////////////////////////////////
// SHUTDOWN
///////////////////////////////////////////////////////////////////////////

func (w *AutosaveWorker) shutdown(batch []*answerPayload) {
	w.log.Info().Msg("Worker stopping, flushing remaining buffer")
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w.flushSafe(ctx, batch)
}
