package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// QuestionRepository handles question data access.
type QuestionRepository struct {
	pool *pgxpool.Pool
}

// NewQuestionRepository creates a new QuestionRepository.
func NewQuestionRepository(pool *pgxpool.Pool) *QuestionRepository {
	return &QuestionRepository{pool: pool}
}

// ListByExam retrieves all questions for a given exam, ordered by order_num.
func (r *QuestionRepository) ListByExam(ctx context.Context, examID uuid.UUID) ([]model.Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, exam_id, question_text, question_type, options, correct_option, order_num, score_value
		 FROM questions WHERE exam_id = $1
		 ORDER BY order_num`, examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []model.Question
	for rows.Next() {
		var q model.Question
		if err := rows.Scan(&q.ID, &q.ExamID, &q.QuestionText, &q.QuestionType, &q.Options, &q.CorrectOption, &q.OrderNum, &q.ScoreValue); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// Create inserts a new question.
func (r *QuestionRepository) Create(ctx context.Context, q *model.Question) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO questions (exam_id, question_text, question_type, options, correct_option, order_num, score_value)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		q.ExamID, q.QuestionText, q.QuestionType, q.Options, q.CorrectOption, q.OrderNum, q.ScoreValue,
	).Scan(&q.ID)
}
