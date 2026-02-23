package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		`SELECT id, exam_id, class_id, grade_level, major_code, religion
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
		if err := rows.Scan(&rule.ID, &rule.ExamID, &rule.ClassID, &rule.GradeLevel, &rule.MajorCode, &rule.Religion); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// Create inserts a new target rule.
func (r *ExamTargetRuleRepository) Create(ctx context.Context, rule *model.ExamTargetRule) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO exam_target_rules (exam_id, class_id, grade_level, major_code, religion)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		rule.ExamID, rule.ClassID, rule.GradeLevel, rule.MajorCode, rule.Religion,
	).Scan(&rule.ID)
}

// Delete removes a target rule by its ID, ensuring it belongs to the given exam.
func (r *ExamTargetRuleRepository) Delete(ctx context.Context, ruleID int, examID uuid.UUID) error {
	cmdTag, err := r.pool.Exec(ctx,
		`DELETE FROM exam_target_rules WHERE id = $1 AND exam_id = $2`,
		ruleID, examID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows // Or a custom not found error
	}
	return nil
}

// Update modifies an existing target rule.
func (r *ExamTargetRuleRepository) Update(ctx context.Context, rule *model.ExamTargetRule) error {
	cmdTag, err := r.pool.Exec(ctx,
		`UPDATE exam_target_rules
		 SET class_id = $1, grade_level = $2, major_code = $3, religion = $4
		 WHERE id = $5 AND exam_id = $6`,
		rule.ClassID, rule.GradeLevel, rule.MajorCode, rule.Religion, rule.ID, rule.ExamID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// FindExamsForStudent retrieves exam IDs that target a student's class/grade/major/religion.
func (r *ExamTargetRuleRepository) FindExamsForStudent(ctx context.Context, classID int) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT etr.exam_id
		 FROM exam_target_rules etr
		 JOIN classes c ON c.id = $1
		 JOIN students s ON s.class_id = c.id
		 WHERE
		   etr.class_id = c.id
		   OR (
			   (etr.grade_level IS NULL OR etr.grade_level = CAST(c.grade_level AS VARCHAR))
			   AND (etr.major_code IS NULL OR etr.major_code = c.major_code)
			   AND (etr.religion IS NULL OR etr.religion = s.religion)
		   )`,
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
