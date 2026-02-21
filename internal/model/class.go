package model

import "time"

// Class represents a school class group.
type Class struct {
	ID          int       `json:"id"`
	GradeLevel  string    `json:"grade_level"`
	MajorCode   string    `json:"major_code"`
	GroupNumber int       `json:"group_number"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
