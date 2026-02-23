package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

// DashboardHandler handles admin dashboard endpoints.
type DashboardHandler struct {
	dashboardService *service.DashboardService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: dashboardService}
}

// GetDashboardData godoc
// GET /api/v1/admin/dashboard
// Returns summary stat cards, exam status distribution, upcoming exams, and recent completions.
func (h *DashboardHandler) GetDashboardData(c *gin.Context) {
	data, err := h.dashboardService.GetDashboardData(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, data)
}
