// Package database provides database access and models for the application.
//
// This package defines database interfaces, models, and implementations for
// various database backends like SQLite and PostgreSQL.
package database

import (
	"time"
)

// Permission flags define the bit flags for user permissions.
const (
	PermAdmin         = 1 << iota // Full administrator access
	PermCreateServer              // Can create new servers
	PermDeleteServer              // Can delete servers
	PermStartServer               // Can start servers
	PermStopServer                // Can stop servers
	PermRestartServer             // Can restart servers
	PermExecCommand               // Can execute commands on servers
	PermViewServer                // Can view server details
)

// Common permissions groups provide pre-defined combinations of permissions.
var (
	// PermAll grants all permissions
	PermAll = PermAdmin | PermCreateServer | PermDeleteServer | PermStartServer |
		PermStopServer | PermRestartServer | PermExecCommand | PermViewServer

	// PermReadOnly grants only view permissions
	PermReadOnly = PermViewServer

	// PermOperator grants everything except admin permissions
	PermOperator = PermCreateServer | PermDeleteServer | PermStartServer |
		PermStopServer | PermRestartServer | PermExecCommand | PermViewServer
)

// User represents a user in the system with their permissions and account details.
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

// HasPermission checks if the user has the specified permission.
// It always returns true for administrators.
func (u *User) HasPermission(permission int64) bool {
	// Admin always has all permissions
	if u.Permissions&PermAdmin != 0 {
		return true
	}
	return u.Permissions&permission != 0
}

// IsAdmin checks if the user is an administrator.
func (u *User) IsAdmin() bool {
	return u.HasPermission(PermAdmin)
}

// APIKey represents an API key for machine authentication.
type APIKey struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Key         string    `json:"key"`
	Description string    `json:"description"`
	LastUsed    time.Time `json:"last_used"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}
