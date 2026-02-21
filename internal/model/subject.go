package model

import "time"

// Subject represents an academic course or subject.
type Subject struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateSubjectRequest is the payload for creating a subject.
type CreateSubjectRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

// UpdateSubjectRequest is the payload for updating a subject.
type UpdateSubjectRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}
