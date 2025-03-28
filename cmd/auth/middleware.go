// Package auth provides authentication and authorization capabilities.
//
// This package handles JWT token generation and validation, API key authentication,
// password hashing and verification, and OAuth provider integration.
package auth

import (
	"net/http"
	"strings"
	"time"

	"minecharts/cmd/database"
	"minecharts/cmd/logging"

	"github.com/gin-gonic/gin"
)

// AuthUserKey is the key used to store authenticated user in the Gin context.
const (
	AuthUserKey = "auth_user"
)

// JWTMiddleware validates JWT tokens in the Authorization header.
// It extracts the token from the Authorization header, validates it,
// and sets the authenticated user in the Gin context for downstream handlers.
func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "missing_auth_header"),
			).Warn("Authentication failed: missing Authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// Check for Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "invalid_auth_format"),
			).Warn("Authentication failed: invalid Authorization header format")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be 'Bearer {token}'"})
			return
		}

		// Validate token
		claims, err := ValidateJWT(parts[1])
		if err != nil {
			if err == ErrExpiredToken {
				logging.WithFields(
					logging.F("path", c.Request.URL.Path),
					logging.F("remote_ip", c.ClientIP()),
					logging.F("error", "token_expired"),
				).Warn("Authentication failed: token expired")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
				return
			}
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "invalid_token"),
				logging.F("error_details", err.Error()),
			).Warn("Authentication failed: invalid token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		logging.WithFields(
			logging.F("path", c.Request.URL.Path),
			logging.F("user_id", claims.UserID),
			logging.F("username", claims.Username),
		).Debug("JWT token validated successfully")

		// Get user from database to ensure they still exist and have the right permissions
		db := database.GetDB()
		user, err := db.GetUserByID(c.Request.Context(), claims.UserID)
		if err != nil {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("user_id", claims.UserID),
				logging.F("error", "user_not_found"),
				logging.F("error_details", err.Error()),
			).Warn("Authentication failed: user not found")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		// Check if user is active
		if !user.Active {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("user_id", user.ID),
				logging.F("username", user.Username),
				logging.F("error", "account_inactive"),
			).Warn("Authentication failed: account inactive")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
			return
		}

		// Set user in context for handlers to use
		c.Set(AuthUserKey, user)

		logging.WithFields(
			logging.F("path", c.Request.URL.Path),
			logging.F("user_id", user.ID),
			logging.F("username", user.Username),
			logging.F("remote_ip", c.ClientIP()),
		).Debug("User authenticated successfully via JWT")

		c.Next()
	}
}

// APIKeyMiddleware validates API key in the X-API-Key header.
// It attempts API key authentication if JWT authentication hasn't already succeeded.
func APIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if JWT already authenticated
		if _, exists := c.Get(AuthUserKey); exists {
			c.Next()
			return
		}

		// Get API key from header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "missing_api_key"),
			).Warn("API key authentication failed: missing API key")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
			return
		}

		// Validate API key
		db := database.GetDB()
		key, err := db.GetAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "invalid_api_key"),
				logging.F("error_details", err.Error()),
			).Warn("API key authentication failed: invalid API key")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			return
		}

		logging.WithFields(
			logging.F("path", c.Request.URL.Path),
			logging.F("api_key_id", key.ID),
			logging.F("user_id", key.UserID),
		).Debug("API key validated")

		// Check if API key is expired
		if !key.ExpiresAt.IsZero() && key.ExpiresAt.Before(c.Request.Context().Value("now").(time.Time)) {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("api_key_id", key.ID),
				logging.F("user_id", key.UserID),
				logging.F("error", "expired_api_key"),
			).Warn("API key authentication failed: expired API key")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
			return
		}

		// Get user associated with API key
		user, err := db.GetUserByID(c.Request.Context(), key.UserID)
		if err != nil {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("api_key_id", key.ID),
				logging.F("user_id", key.UserID),
				logging.F("error", "user_not_found"),
				logging.F("error_details", err.Error()),
			).Warn("API key authentication failed: user not found")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		// Check if user is active
		if !user.Active {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("api_key_id", key.ID),
				logging.F("user_id", user.ID),
				logging.F("username", user.Username),
				logging.F("error", "account_inactive"),
			).Warn("API key authentication failed: account inactive")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
			return
		}

		// Set user in context for handlers to use
		c.Set(AuthUserKey, user)

		logging.WithFields(
			logging.F("path", c.Request.URL.Path),
			logging.F("api_key_id", key.ID),
			logging.F("user_id", user.ID),
			logging.F("username", user.Username),
			logging.F("remote_ip", c.ClientIP()),
		).Debug("User authenticated successfully via API key")

		c.Next()
	}
}

// RequirePermission checks if the authenticated user has the required permission.
// It returns a 403 Forbidden response if the user doesn't have the required permission.
func RequirePermission(permission int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context
		value, exists := c.Get(AuthUserKey)
		if !exists {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("required_permission", permission),
				logging.F("error", "not_authenticated"),
			).Warn("Permission check failed: user not authenticated")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		user, ok := value.(*database.User)
		if !ok {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("required_permission", permission),
				logging.F("error", "invalid_user_object"),
			).Error("Permission check failed: invalid user object in context")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid user object in context"})
			return
		}

		// Check permission
		if !user.HasPermission(permission) {
			logging.WithFields(
				logging.F("path", c.Request.URL.Path),
				logging.F("user_id", user.ID),
				logging.F("username", user.Username),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("required_permission", permission),
				logging.F("user_permissions", user.Permissions),
				logging.F("error", "permission_denied"),
			).Warn("Permission check failed: insufficient permissions")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}

		logging.WithFields(
			logging.F("path", c.Request.URL.Path),
			logging.F("user_id", user.ID),
			logging.F("username", user.Username),
			logging.F("required_permission", permission),
		).Trace("Permission check passed")

		c.Next()
	}
}

// RequireServerPermission checks if the user has permission for the specific server
func RequireServerPermission(permission int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get current user
		value, exists := c.Get(AuthUserKey)
		if !exists {
			logging.Warn("Permission check failed: user not authenticated")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		user, ok := value.(*database.User)
		if !ok {
			logging.Error("Permission check failed: invalid user object")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid user object"})
			return
		}

		// Get server name from URL parameter
		serverName := c.Param("serverName")
		if serverName == "" {
			// If no serverName, use standard permission check
			if !user.HasPermission(permission) {
				logging.WithFields(
					logging.F("user_id", user.ID),
					logging.F("username", user.Username),
					logging.F("permission", permission),
				).Warn("Permission check failed: insufficient permissions")
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
				return
			}
			c.Next()
			return
		}

		// Get server info
		db := database.GetDB()
		server, err := db.GetServerByName(c.Request.Context(), serverName)
		if err != nil {
			// If server not found in DB but exists in K8s, default to standard permission check
			if !user.HasPermission(permission) {
				logging.WithFields(
					logging.F("user_id", user.ID),
					logging.F("username", user.Username),
					logging.F("server_name", serverName),
				).Warn("Permission check failed: server not found and insufficient permissions")
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
				return
			}
			c.Next()
			return
		}

		// Check permission with ownership logic
		if !user.HasServerPermission(server.OwnerID, permission) {
			logging.WithFields(
				logging.F("user_id", user.ID),
				logging.F("username", user.Username),
				logging.F("server_name", serverName),
				logging.F("server_owner_id", server.OwnerID),
			).Warn("Server permission check failed")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}

		c.Next()
	}
}

// GetCurrentUser retrieves the authenticated user from the Gin context.
// It returns the user object and a boolean indicating if the user was found.
func GetCurrentUser(c *gin.Context) (*database.User, bool) {
	value, exists := c.Get(AuthUserKey)
	if !exists {
		return nil, false
	}

	user, ok := value.(*database.User)
	return user, ok
}
