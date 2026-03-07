package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stemsi/exstem-backend/internal/model"
)

// RoomRepository handles database operations for rooms.
type RoomRepository struct {
	pool *pgxpool.Pool
}

// NewRoomRepository creates a new RoomRepository.
func NewRoomRepository(pool *pgxpool.Pool) *RoomRepository {
	return &RoomRepository{pool: pool}
}

// Create inserts a new room into the database.
func (r *RoomRepository) Create(ctx context.Context, room *model.Room) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO rooms (name, capacity) 
		 VALUES ($1, $2) 
		 RETURNING id, created_at, updated_at`,
		room.Name, room.Capacity,
	).Scan(&room.ID, &room.CreatedAt, &room.UpdatedAt)

	return err
}

// GetByID retrieves a room by its ID.
func (r *RoomRepository) GetByID(ctx context.Context, id int) (*model.Room, error) {
	var room model.Room
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, capacity, created_at, updated_at 
		 FROM rooms WHERE id = $1`, id,
	).Scan(&room.ID, &room.Name, &room.Capacity, &room.CreatedAt, &room.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return &room, nil
}

// Update modifies an existing room.
func (r *RoomRepository) Update(ctx context.Context, room *model.Room) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE rooms 
		 SET name = $1, capacity = $2, updated_at = NOW() 
		 WHERE id = $3`,
		room.Name, room.Capacity, room.ID,
	)
	return err
}

// Delete removes a room from the database.
func (r *RoomRepository) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id)
	return err
}

// List retrieves all rooms.
func (r *RoomRepository) List(ctx context.Context) ([]model.Room, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, capacity, created_at, updated_at 
		 FROM rooms 
		 ORDER BY name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []model.Room
	for rows.Next() {
		var room model.Room
		if err := rows.Scan(
			&room.ID, &room.Name, &room.Capacity, &room.CreatedAt, &room.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return rooms, nil
}
