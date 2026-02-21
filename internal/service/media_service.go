package service

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/stemsi/exstem-backend/internal/config"
)

// Sentinel errors for media uploads.
var (
	ErrUnsupportedFileType = errors.New("unsupported file type")
	ErrFileTooLarge        = errors.New("file too large")
)

// Allowed image MIME types.
var allowedMIMETypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// MediaService handles file upload operations.
type MediaService struct {
	cfg *config.Config
}

// NewMediaService creates a new MediaService.
func NewMediaService(cfg *config.Config) *MediaService {
	return &MediaService{cfg: cfg}
}

// SaveUpload saves an uploaded file to local storage with a UUID filename.
// Returns the relative URL path to the saved file.
func (s *MediaService) SaveUpload(file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validate MIME type.
	contentType := header.Header.Get("Content-Type")
	ext, ok := allowedMIMETypes[contentType]
	if !ok {
		return "", fmt.Errorf("%w: %s (allowed: %s)",
			ErrUnsupportedFileType, contentType, strings.Join(allowedTypes(), ", "))
	}

	// Validate file size.
	if header.Size > s.cfg.MaxUploadBytes {
		return "", fmt.Errorf("%w: %d bytes (max: %d)", ErrFileTooLarge, header.Size, s.cfg.MaxUploadBytes)
	}

	// Ensure upload directory exists.
	if err := os.MkdirAll(s.cfg.UploadDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	// Generate UUID filename.
	filename := uuid.New().String() + ext
	destPath := filepath.Join(s.cfg.UploadDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// Return relative URL path.
	return "/uploads/" + filename, nil
}

func allowedTypes() []string {
	types := make([]string, 0, len(allowedMIMETypes))
	for t := range allowedMIMETypes {
		types = append(types, t)
	}
	return types
}
