package model

import "time"

// AppSetting represents a key-value pair for global application configuration.
type AppSetting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateSettingsRequest is the payload for bulk updating settings.
type UpdateSettingsRequest struct {
	Settings map[string]string `json:"settings" binding:"required"`
}
