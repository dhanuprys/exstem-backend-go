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

// ListQBanks retrieves question banks with pagination and search.
func (r *QuestionRepository) ListQBanks(ctx context.Context, limit, offset int, search string) ([]model.QuestionBank, int, error) {
	// 1. Get total count
	countQuery := `SELECT COUNT(*) FROM question_banks WHERE name ILIKE $1 OR description ILIKE $1`
	searchParam := "%" + search + "%"

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, searchParam).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 2. Get paginated data
	query := `SELECT id, author_id, subject_id, name, description
		 FROM question_banks
		 WHERE name ILIKE $1 OR description ILIKE $1
		 ORDER BY id DESC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, searchParam, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var qbanks []model.QuestionBank
	for rows.Next() {
		var q model.QuestionBank
		if err := rows.Scan(&q.ID, &q.AuthorID, &q.SubjectID, &q.Name, &q.Description); err != nil {
			return nil, 0, err
		}
		qbanks = append(qbanks, q)
	}
	return qbanks, total, rows.Err()
}

// ListQBanksByAuthor retrieves question banks filtered by author with pagination and search.
func (r *QuestionRepository) ListQBanksByAuthor(ctx context.Context, authorID, limit, offset int, search string) ([]model.QuestionBank, int, error) {
	searchParam := "%" + search + "%"

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM question_banks
		 WHERE author_id = $1 AND (name ILIKE $2 OR description ILIKE $2)`,
		authorID, searchParam,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, author_id, subject_id, name, description
		 FROM question_banks
		 WHERE author_id = $1 AND (name ILIKE $2 OR description ILIKE $2)
		 ORDER BY id DESC LIMIT $3 OFFSET $4`,
		authorID, searchParam, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var qbanks []model.QuestionBank
	for rows.Next() {
		var q model.QuestionBank
		if err := rows.Scan(&q.ID, &q.AuthorID, &q.SubjectID, &q.Name, &q.Description); err != nil {
			return nil, 0, err
		}
		qbanks = append(qbanks, q)
	}
	return qbanks, total, rows.Err()
}

// GetQBanks retrieves a specific question bank.
func (r *QuestionRepository) GetQBanks(ctx context.Context, qbankID uuid.UUID) (*model.QuestionBank, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, author_id, subject_id, name, description
		 FROM question_banks WHERE id = $1`, qbankID,
	)
	var q model.QuestionBank
	if err := row.Scan(&q.ID, &q.AuthorID, &q.SubjectID, &q.Name, &q.Description); err != nil {
		return nil, err
	}
	return &q, nil
}

// CreateQBanks creates a new question bank.
func (r *QuestionRepository) CreateQBanks(ctx context.Context, qbank *model.QuestionBank) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO question_banks (author_id, subject_id, name, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`, qbank.AuthorID, qbank.SubjectID, qbank.Name, qbank.Description,
	).Scan(&qbank.ID)
}

// UpdateQBanks updates a specific question bank.
func (r *QuestionRepository) UpdateQBanks(ctx context.Context, qbank *model.QuestionBank) error {
	return r.pool.QueryRow(ctx,
		`UPDATE question_banks SET author_id = $2, subject_id = $3, name = $4, description = $5
		 WHERE id = $1
		 RETURNING id`, qbank.ID, qbank.AuthorID, qbank.SubjectID, qbank.Name, qbank.Description,
	).Scan(&qbank.ID)
}

// DeleteQBanks deletes a specific question bank.
func (r *QuestionRepository) DeleteQBanks(ctx context.Context, qbankID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM question_banks WHERE id = $1`, qbankID,
	)
	return err
}

// ListByQBank retrieves all questions for a given qbank, ordered by order_num.
func (r *QuestionRepository) ListByQBank(ctx context.Context, qbankID uuid.UUID) ([]model.Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, qbank_id, question_text, question_type, options, correct_option, order_num
		 FROM questions WHERE qbank_id = $1
		 ORDER BY order_num`, qbankID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []model.Question
	for rows.Next() {
		var q model.Question
		if err := rows.Scan(&q.ID, &q.QBankID, &q.QuestionText, &q.QuestionType, &q.Options, &q.CorrectOption, &q.OrderNum); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// ListByExam retrieves all questions by exam id
func (r *QuestionRepository) ListByExam(ctx context.Context, examID uuid.UUID) ([]model.Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.id, q.qbank_id, q.question_text, q.question_type, q.options, q.correct_option, q.order_num
		 FROM 
		 	questions q 
		INNER JOIN
			question_banks qb ON qb.id = q.qbank_id 
		INNER JOIN 
			exams e ON e.qbank_id = qb.id
		 WHERE 
		 	e.id = $1
		 ORDER BY q.order_num`, examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []model.Question
	for rows.Next() {
		var q model.Question
		if err := rows.Scan(&q.ID, &q.QBankID, &q.QuestionText, &q.QuestionType, &q.Options, &q.CorrectOption, &q.OrderNum); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// Create inserts a new question.
func (r *QuestionRepository) Create(ctx context.Context, q *model.Question) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO questions
			(qbank_id, question_text, question_type, options, correct_option, order_num)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		q.QBankID, q.QuestionText, q.QuestionType, q.Options, q.CorrectOption, q.OrderNum,
	).Scan(&q.ID)
}

// ReplaceAll replaces all questions for an exam in a single transaction.
func (r *QuestionRepository) ReplaceAll(ctx context.Context, qbankID uuid.UUID, questions []model.Question) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Step 1: Delete all existing questions for this exam
	if _, err := tx.Exec(ctx, `DELETE FROM questions WHERE qbank_id = $1`, qbankID); err != nil {
		return err
	}

	// Step 2: Insert the new questions
	for _, q := range questions {
		err := tx.QueryRow(ctx,
			`INSERT INTO questions
				(qbank_id, question_text, question_type, options, correct_option, order_num)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id`,
			qbankID, q.QuestionText, q.QuestionType, q.Options, q.CorrectOption, q.OrderNum,
		).Scan(&q.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
