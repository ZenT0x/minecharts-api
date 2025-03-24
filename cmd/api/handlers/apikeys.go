package handlers

import (
	"net/http"
	"strconv"
	"time"

	"minecharts/cmd/auth"
	"minecharts/cmd/config"
	"minecharts/cmd/database"

	"github.com/gin-gonic/gin"
)

// CreateAPIKeyRequest represents a request to create a new API key
type CreateAPIKeyRequest struct {
	Description string    `json:"description"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// CreateAPIKeyHandler creates a new API key for the authenticated user
func CreateAPIKeyHandler(c *gin.Context) {
	user, ok := auth.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a new API key
	keyValue, err := auth.GenerateAPIKey(config.APIKeyPrefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}

	// Create API key record
	apiKey := &database.APIKey{
		UserID:      user.ID,
		Key:         keyValue,
		Description: req.Description,
		ExpiresAt:   req.ExpiresAt,
	}

	db := database.GetDB()
	if err := db.CreateAPIKey(c.Request.Context(), apiKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          apiKey.ID,
		"key":         apiKey.Key, // This is the only time the full key will be shown
		"description": apiKey.Description,
		"expires_at":  apiKey.ExpiresAt,
		"created_at":  apiKey.CreatedAt,
	})
}

// ListAPIKeysHandler returns all API keys for the authenticated user
func ListAPIKeysHandler(c *gin.Context) {
	user, ok := auth.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	db := database.GetDB()
	apiKeys, err := db.ListAPIKeysByUser(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list API keys"})
		return
	}

	// For security, only return partial key values
	response := make([]gin.H, len(apiKeys))
	for i, key := range apiKeys {
		// Create a masked version of the key (e.g., "mcapi.XXXX")
		maskedKey := key.Key[:8] + "..." // Only show first part of the key

		response[i] = gin.H{
			"id":          key.ID,
			"key":         maskedKey,
			"description": key.Description,
			"last_used":   key.LastUsed,
			"expires_at":  key.ExpiresAt,
			"created_at":  key.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

// DeleteAPIKeyHandler deletes an API key
func DeleteAPIKeyHandler(c *gin.Context) {
	user, ok := auth.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Get API key ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	// Verify the API key belongs to the user (unless admin)
	if !user.IsAdmin() {
		db := database.GetDB()
		keys, err := db.ListAPIKeysByUser(c.Request.Context(), user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify API key ownership"})
			return
		}

		found := false
		for _, key := range keys {
			if key.ID == id {
				found = true
				break
			}
		}

		if !found {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not own this API key"})
			return
		}
	}

	// Delete the API key
	db := database.GetDB()
	if err := db.DeleteAPIKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
}
