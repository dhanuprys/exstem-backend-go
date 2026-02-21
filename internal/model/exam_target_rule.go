package model

import "github.com/google/uuid"

// TargetType enumerates exam targeting strategies.
type TargetType string

const (
	TargetTypeClass TargetType = "CLASS"
	TargetTypeGrade TargetType = "GRADE"
	TargetTypeMajor TargetType = "MAJOR"
)

// ExamTargetRule defines which students can see an exam.
type ExamTargetRule struct {
	ID          int        `json:"id"`
	ExamID      uuid.UUID  `json:"exam_id"`
	TargetType  TargetType `json:"target_type"`
	TargetValue string     `json:"target_value"`
}

// AddTargetRuleRequest is the payload for adding a target rule.
type AddTargetRuleRequest struct {
	TargetType  TargetType `json:"target_type" binding:"required,oneof=CLASS GRADE MAJOR"`
	TargetValue string     `json:"target_value" binding:"required,min=1,max=100"`
}
