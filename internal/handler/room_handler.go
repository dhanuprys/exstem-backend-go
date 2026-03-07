package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// RoomHandler handles admin-facing room management.
type RoomHandler struct {
	roomService *service.RoomService
}

// NewRoomHandler creates a new RoomHandler.
func NewRoomHandler(roomService *service.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
}

// ListRooms godoc
// GET /api/v1/admin/rooms
func (h *RoomHandler) ListRooms(c *gin.Context) {
	rooms, err := h.roomService.List(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"rooms": rooms})
}

// CreateRoom godoc
// POST /api/v1/admin/rooms
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req model.CreateRoomRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	room := &model.Room{
		Name:     req.Name,
		Capacity: req.Capacity,
	}

	if err := h.roomService.Create(c.Request.Context(), room); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"room": room})
}

// UpdateRoom godoc
// PUT /api/v1/admin/rooms/:id
func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	var req model.UpdateRoomRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	room := &model.Room{
		ID:       id,
		Name:     req.Name,
		Capacity: req.Capacity,
	}

	if err := h.roomService.Update(c.Request.Context(), room); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique violation
			response.Fail(c, http.StatusConflict, response.ErrConflict)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	updatedRoom, _ := h.roomService.GetByID(c.Request.Context(), id)
	response.Success(c, http.StatusOK, gin.H{"room": updatedRoom})
}

// DeleteRoom godoc
// DELETE /api/v1/admin/rooms/:id
func (h *RoomHandler) DeleteRoom(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrInvalidID)
		return
	}

	if err := h.roomService.Delete(c.Request.Context(), id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key violation
			response.Fail(c, http.StatusConflict, response.ErrDependencyExists)
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "room deleted successfully"})
}
