package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
	"github.com/stemsi/exstem-backend/internal/validator"
)

type SettingHandler struct {
	settingService *service.SettingService
}

func NewSettingHandler(settingService *service.SettingService) *SettingHandler {
	return &SettingHandler{settingService: settingService}
}

// GetAllSettings godoc
// GET /api/v1/admin/settings
func (h *SettingHandler) GetAllSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"settings": settings})
}

// UpdateSettings godoc
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req model.UpdateSettingsRequest
	if fields := validator.Bind(c, &req); fields != nil {
		response.FailWithFields(c, http.StatusBadRequest, response.ErrValidation, fields)
		return
	}

	if err := h.settingService.UpdateSettings(c.Request.Context(), req.Settings); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"message": "settings updated successfully"})
}

// GetPublicSettings godoc
// GET /api/v1/public/settings
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		return
	}
	response.Success(c, http.StatusOK, settings)
}
