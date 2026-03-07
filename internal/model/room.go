package model

import "time"

// Room represents a physical exam room.
type Room struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Capacity  int       `json:"capacity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateRoomRequest is the payload for creating a new room.
type CreateRoomRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Capacity int    `json:"capacity" binding:"required,min=1"`
}

// UpdateRoomRequest is the payload for updating an existing room.
type UpdateRoomRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Capacity int    `json:"capacity" binding:"required,min=1"`
}
