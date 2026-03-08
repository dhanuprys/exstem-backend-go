package model

import (
	"time"

	"github.com/google/uuid"
)

// RoomSession represents a standalone session+room slot (not tied to any exam).
type RoomSession struct {
	ID            uuid.UUID `json:"id"`
	SessionNumber int       `json:"session_number"`
	RoomID        int       `json:"room_id"`
	StartTime     *string   `json:"start_time"`
	EndTime       *string   `json:"end_time"`
	CreatedAt     time.Time `json:"created_at"`

	// Join fields for convenience.
	RoomName     string `json:"room_name,omitempty"`
	RoomCapacity int    `json:"room_capacity,omitempty"`
}

// SessionTimePayload represents the time to set for a specific session number.
type SessionTimePayload struct {
	SessionNumber int     `json:"session_number" binding:"required,min=1"`
	StartTime     *string `json:"start_time"`
	EndTime       *string `json:"end_time"`
}

// UpdateSessionTimesRequest is the payload for bulk-updating session times.
type UpdateSessionTimesRequest struct {
	Sessions []SessionTimePayload `json:"sessions" binding:"required,dive"`
}

// StudentRoomAssignment maps a student to a specific room session and seat.
type StudentRoomAssignment struct {
	ID            uuid.UUID `json:"id"`
	RoomSessionID uuid.UUID `json:"room_session_id"`
	StudentID     int       `json:"student_id"`
	SeatNumber    int       `json:"seat_number"`
	CreatedAt     time.Time `json:"created_at"`

	// Join fields for convenience.
	StudentNIS  string `json:"student_nis,omitempty"`
	StudentName string `json:"student_name,omitempty"`
	ClassName   string `json:"class_name,omitempty"`
}

// AutoDistributeRequest is the payload for distributing students into rooms.
type AutoDistributeRequest struct {
	RoomIDs    []int `json:"room_ids" binding:"required,min=1"`
	ClassIDs   []int `json:"class_ids,omitempty"`
	StudentIDs []int `json:"student_ids,omitempty"`
}

// DistributionResult is the response payload containing all sessions and assignments.
type DistributionResult struct {
	Sessions    []RoomSession           `json:"sessions"`
	Assignments []StudentRoomAssignment `json:"assignments"`
}
