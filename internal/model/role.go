package model

import "time"

// Role represents an RBAC role.
type Role struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// RoleWithPermissions extends Role to include its associated permissions.
type RoleWithPermissions struct {
	*Role
	Permissions []string `json:"permissions"`
}
