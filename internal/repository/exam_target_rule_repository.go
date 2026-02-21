package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// ExamTargetRuleRepository handles exam target rule data access.
type ExamTargetRuleRepository struct {
	pool *pgxpool.Pool
}

// NewExamTargetRuleRepository creates a new ExamTargetRuleRepository.
func NewExamTargetRuleRepository(pool *pgxpool.Pool) *ExamTargetRuleRepository {
	return &ExamTargetRuleRepository{pool: pool}
}

// ListByExam retrieves all target rules for a given exam.
func (r *ExamTargetRuleRepository) ListByExam(ctx context.Context, examID uuid.UUID) ([]model.ExamTargetRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, exam_id, target_type, target_value
		 FROM exam_target_rules
		 WHERE exam_id = $1`, examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.ExamTargetRule
	for rows.Next() {
		var rule model.ExamTargetRule
		if err := rows.Scan(&rule.ID, &rule.ExamID, &rule.TargetType, &rule.TargetValue); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// Create inserts a new target rule.
func (r *ExamTargetRuleRepository) Create(ctx context.Context, rule *model.ExamTargetRule) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO exam_target_rules (exam_id, target_type, target_value)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		rule.ExamID, rule.TargetType, rule.TargetValue,
	).Scan(&rule.ID)
}

// FindExamsForStudent retrieves exam IDs that target a student's class/grade/major.
// This query joins through classes to match all target types.
func (r *ExamTargetRuleRepository) FindExamsForStudent(ctx context.Context, classID int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT etr.exam_id
		 FROM exam_target_rules etr
		 JOIN classes c ON c.id = $1
		 WHERE
		   (etr.target_type = 'CLASS' AND etr.target_value = CAST(c.id AS VARCHAR))
		   OR (etr.target_type = 'GRADE' AND etr.target_value = CAST(c.grade_level AS VARCHAR))
		   OR (etr.target_type = 'MAJOR' AND etr.target_value = c.major_code)`,
		classID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var examIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		examIDs = append(examIDs, id)
	}
	return examIDs, rows.Err()
}
