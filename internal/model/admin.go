package model

import "time"

// Admin represents an admin/teacher user.
type Admin struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	RoleID       int       `json:"role_id"`
	RoleName     string    `json:"role_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AdminLoginRequest is the payload for admin authentication.
type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=6,max=128"`
}

// AdminLoginResponse is returned after successful admin login.
type AdminLoginResponse struct {
	Token       string   `json:"token"`
	Admin       Admin    `json:"admin"`
	Permissions []string `json:"permissions"`
}
