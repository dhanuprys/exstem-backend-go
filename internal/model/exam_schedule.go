package model

import (
	"time"

	"github.com/google/uuid"
)

// ExamSchedule represents a specific session and room for an exam.
type ExamSchedule struct {
	ID            uuid.UUID  `json:"id"`
	ExamID        uuid.UUID  `json:"exam_id"`
	SessionNumber int        `json:"session_number"`
	RoomID        int        `json:"room_id"`
	StartTime     *time.Time `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`

	// Join fields for convenience
	RoomName     string `json:"room_name,omitempty"`
	RoomCapacity int    `json:"room_capacity,omitempty"`
}

// ExamRoomAssignment maps a student to a specific schedule and seat.
type ExamRoomAssignment struct {
	ID             uuid.UUID `json:"id"`
	ExamScheduleID uuid.UUID `json:"exam_schedule_id"`
	StudentID      int       `json:"student_id"`
	SeatNumber     int       `json:"seat_number"`
	CreatedAt      time.Time `json:"created_at"`

	// Join fields for convenience
	StudentNIS  string `json:"student_nis,omitempty"`
	StudentName string `json:"student_name,omitempty"`
	ClassName   string `json:"class_name,omitempty"`
}

// AutoDistributeRequest is the payload for automatically distributing students into rooms and sessions.
type AutoDistributeRequest struct {
	RoomIDs      []int      `json:"room_ids" binding:"required,min=1"`
	SourceMode   string     `json:"source_mode" binding:"required,oneof=target_rules manual by_exam"`
	ClassIDs     []int      `json:"class_ids,omitempty"`      // For manual mode
	StudentIDs   []int      `json:"student_ids,omitempty"`    // For manual mode
	SourceExamID *uuid.UUID `json:"source_exam_id,omitempty"` // For by_exam mode
}

// UpdateScheduleTimeRequest is the payload for updating the start and end times of a schedule.
type UpdateScheduleTimeRequest struct {
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
}

// DistributionResult is the response payload containing all schedules and assignments for an exam.
type DistributionResult struct {
	Schedules   []ExamSchedule       `json:"schedules"`
	Assignments []ExamRoomAssignment `json:"assignments"`
}
