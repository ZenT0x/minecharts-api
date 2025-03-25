package database

import (
	"time"
)

// Permission flags
const (
	PermAdmin         = 1 << iota // 1: Full administrator access
	PermCreateServer              // 2: Can create new servers
	PermDeleteServer              // 4: Can delete servers
	PermStartServer               // 8: Can start servers
	PermStopServer                // 16: Can stop servers
	PermRestartServer             // 32: Can restart servers
	PermExecCommand               // 64: Can execute commands on servers
	PermViewServer                // 128: Can view server details
)

// Common permissions groups
var (
	// All permissions
	PermAll = PermAdmin | PermCreateServer | PermDeleteServer | PermStartServer |
		PermStopServer | PermRestartServer | PermExecCommand | PermViewServer

	// Read-only permissions
	PermReadOnly = PermViewServer

	// Operator permissions (everything except admin)
	PermOperator = PermCreateServer | PermDeleteServer | PermStartServer |
		PermStopServer | PermRestartServer | PermExecCommand | PermViewServer
)

// User represents a user in the system
type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never expose in JSON
	Permissions  int64      `json:"permissions"`
	Active       bool       `json:"active"`
	LastLogin    *time.Time `json:"last_login"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// HasPermission checks if the user has the specified permission
func (u *User) HasPermission(permission int64) bool {
	// Admin always has all permissions
	if u.Permissions&PermAdmin != 0 {
		return true
	}
	return u.Permissions&permission != 0
}

// IsAdmin checks if the user is an administrator
func (u *User) IsAdmin() bool {
	return u.HasPermission(PermAdmin)
}

// APIKey represents an API key for machine authentication
type APIKey struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Key         string    `json:"key"`
	Description string    `json:"description"`
	LastUsed    time.Time `json:"last_used"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}
