package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
	"github.com/stemsi/exstem-backend/internal/service"
)

// MediaHandler handles media upload endpoints.
type MediaHandler struct {
	mediaService *service.MediaService
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(mediaService *service.MediaService) *MediaHandler {
	return &MediaHandler{mediaService: mediaService}
}

// UploadMedia godoc
// POST /api/v1/admin/media/upload
// Uploads an image file and returns its URL.
func (h *MediaHandler) UploadMedia(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.ErrFileRequired)
		return
	}
	defer file.Close()

	url, err := h.mediaService.SaveUpload(file, header)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnsupportedFileType):
			response.Fail(c, http.StatusBadRequest, response.ErrUnsupportedFile)
		case errors.Is(err, service.ErrFileTooLarge):
			response.Fail(c, http.StatusBadRequest, response.ErrFileTooLarge)
		default:
			response.Fail(c, http.StatusInternalServerError, response.ErrInternal)
		}
		return
	}

	response.Success(c, http.StatusOK, gin.H{"url": url})
}
