package service

import (
	"context"

	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
)

// RoomService encapsulates business logic for rooms.
type RoomService struct {
	repo *repository.RoomRepository
}

// NewRoomService creates a new RoomService.
func NewRoomService(repo *repository.RoomRepository) *RoomService {
	return &RoomService{repo: repo}
}

// Create creates a new room.
func (s *RoomService) Create(ctx context.Context, room *model.Room) error {
	return s.repo.Create(ctx, room)
}

// GetByID retrieves a room by ID.
func (s *RoomService) GetByID(ctx context.Context, id int) (*model.Room, error) {
	return s.repo.GetByID(ctx, id)
}

// Update modifies an existing room.
func (s *RoomService) Update(ctx context.Context, room *model.Room) error {
	return s.repo.Update(ctx, room)
}

// Delete removes a room.
func (s *RoomService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

// List returns all rooms.
func (s *RoomService) List(ctx context.Context) ([]model.Room, error) {
	return s.repo.List(ctx)
}
