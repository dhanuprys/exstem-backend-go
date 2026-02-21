package model

import "github.com/google/uuid"

// ExamTargetRule defines which students can see an exam by acting as a dynamic filter.
type ExamTargetRule struct {
	ID         int       `json:"id"`
	ExamID     uuid.UUID `json:"exam_id"`
	ClassID    *int      `json:"class_id,omitempty"`
	GradeLevel *string   `json:"grade_level,omitempty"`
	MajorCode  *string   `json:"major_code,omitempty"`
	Religion   *string   `json:"religion,omitempty"`
}

// AddTargetRuleRequest is the payload for adding a target rule.
type AddTargetRuleRequest struct {
	ClassID    *int    `json:"class_id,omitempty"`
	GradeLevel *string `json:"grade_level,omitempty"`
	MajorCode  *string `json:"major_code,omitempty"`
	Religion   *string `json:"religion,omitempty"`
}
