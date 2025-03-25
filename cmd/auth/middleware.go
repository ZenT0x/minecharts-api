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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// Check for Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be 'Bearer {token}'"})
			return
		}

		// Validate token
		claims, err := ValidateJWT(parts[1])
		if err != nil {
			if err == ErrExpiredToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Get user from database to ensure they still exist and have the right permissions
		db := database.GetDB()
		user, err := db.GetUserByID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		// Check if user is active
		if !user.Active {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
			return
		}

		// Set user in context for handlers to use
		c.Set(AuthUserKey, user)

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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
			return
		}

		// Validate API key
		db := database.GetDB()
		key, err := db.GetAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			return
		}

		// Check if API key is expired
		if !key.ExpiresAt.IsZero() && key.ExpiresAt.Before(c.Request.Context().Value("now").(time.Time)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
			return
		}

		// Get user associated with API key
		user, err := db.GetUserByID(c.Request.Context(), key.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		// Check if user is active
		if !user.Active {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
			return
		}

		// Set user in context for handlers to use
		c.Set(AuthUserKey, user)

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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		user, ok := value.(*database.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid user object in context"})
			return
		}

		// Check permission
		if !user.HasPermission(permission) {
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
