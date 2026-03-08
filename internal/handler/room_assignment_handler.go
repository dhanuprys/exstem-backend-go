package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

// RoomAssignmentHandler handles standalone room distribution endpoints.
type RoomAssignmentHandler struct {
	assignmentService *service.RoomAssignmentService
}

// NewRoomAssignmentHandler creates a new RoomAssignmentHandler.
func NewRoomAssignmentHandler(assignmentService *service.RoomAssignmentService) *RoomAssignmentHandler {
	return &RoomAssignmentHandler{assignmentService: assignmentService}
}

// AutoDistribute godoc
// POST /api/v1/admin/room-assignments/distribute
func (h *RoomAssignmentHandler) AutoDistribute(c *gin.Context) {
	var req model.AutoDistributeRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	if err := h.assignmentService.AutoDistribute(c.Request.Context(), req); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	dist, err := h.assignmentService.GetDistribution(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, dist)
}

// GetDistribution godoc
// GET /api/v1/admin/room-assignments
func (h *RoomAssignmentHandler) GetDistribution(c *gin.Context) {
	dist, err := h.assignmentService.GetDistribution(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, dist)
}

// ClearDistribution godoc
// DELETE /api/v1/admin/room-assignments
func (h *RoomAssignmentHandler) ClearDistribution(c *gin.Context) {
	if err := h.assignmentService.ClearDistribution(c.Request.Context()); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "distribution cleared"})
}

// UpdateSessionTimes godoc
// PUT /api/v1/admin/room-assignments/sessions
func (h *RoomAssignmentHandler) UpdateSessionTimes(c *gin.Context) {
	var req model.UpdateSessionTimesRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	if err := h.assignmentService.UpdateSessionTimes(c.Request.Context(), req); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	dist, err := h.assignmentService.GetDistribution(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, dist)
}

// ExportPresenceXLSX godoc
// GET /api/v1/admin/room-assignments/export
func (h *RoomAssignmentHandler) ExportPresenceXLSX(c *gin.Context) {
	sessionFilterStr := c.Query("session")
	roomFilterStr := c.Query("room")

	var sessionFilter, roomFilter int
	sessionOk, roomOk := false, false

	if sessionFilterStr != "" {
		_, err := fmt.Sscanf(sessionFilterStr, "%d", &sessionFilter)
		sessionOk = err == nil
	}
	if roomFilterStr != "" {
		_, err := fmt.Sscanf(roomFilterStr, "%d", &roomFilter)
		roomOk = err == nil
	}

	b, err := h.assignmentService.ExportPresenceXLSX(c.Request.Context(), sessionOk, sessionFilter, roomOk, roomFilter)
	if err != nil {
		if err.Error() == "no distribution data available to export" {
			c.JSON(http.StatusBadRequest, response.Response{
				Error: &response.ErrorBody{
					Code:    response.ErrValidation,
					Message: "No distribution data available to export",
				},
			})
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=Pembagian_Ruangan.xlsx")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
}
