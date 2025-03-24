package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"minecharts/cmd/auth"
	"minecharts/cmd/config"
	"minecharts/cmd/database"

	"github.com/gin-gonic/gin"
)

// LoginRequest represents the login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents the register request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginHandler handles user login with username and password
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from database
	db := database.GetDB()
	user, err := db.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if err == database.ErrUserNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Check if user is active
	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Username, user.Email, user.Permissions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Update last login time
	user.LastLogin = time.Now()
	if err := db.UpdateUser(c.Request.Context(), user); err != nil {
		// Non-critical error, just log it
		// log.Printf("Failed to update last login time: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user_id":     user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
	})
}

// RegisterHandler handles user registration
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	user := &database.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Permissions:  int64(database.PermReadOnly), // Default to read-only permissions
		Active:       true,
	}

	db := database.GetDB()
	if err := db.CreateUser(c.Request.Context(), user); err != nil {
		if err == database.ErrUserExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Username, user.Email, user.Permissions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":       token,
		"user_id":     user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
	})
}

// GetUserInfoHandler returns information about the authenticated user
func GetUserInfoHandler(c *gin.Context) {
	user, ok := auth.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":     user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
		"active":      user.Active,
		"last_login":  user.LastLogin,
		"created_at":  user.CreatedAt,
	})
}

// GenerateStateValue creates a random state value for OAuth flows
func GenerateStateValue() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// OAuthLoginHandler initiates the OAuth login flow
func OAuthLoginHandler(c *gin.Context) {
	// Check if OAuth is enabled
	if !config.OAuthEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth is not enabled"})
		return
	}

	// Get provider from URL parameter
	provider := c.Param("provider")
	if provider != "authentik" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported OAuth provider"})
		return
	}

	// Initialize OAuth provider
	oauthProvider, err := auth.NewAuthentikProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize OAuth provider"})
		return
	}

	// Generate and store state parameter to prevent CSRF
	state, err := GenerateStateValue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state"})
		return
	}

	// Store state in a secure HTTP-only cookie for verification later
	c.SetCookie(
		"oauth_state",
		state,
		int(time.Now().Add(15*time.Minute).Unix()), // Expires after 15 minutes
		"/",
		"",
		true, // Secure (HTTPS only)
		true, // HTTP-only
	)

	// Redirect to OAuth provider's auth page
	authURL := oauthProvider.GetAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// OAuthCallbackHandler handles the OAuth callback
func OAuthCallbackHandler(c *gin.Context) {
	// Check if OAuth is enabled
	if !config.OAuthEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth is not enabled"})
		return
	}

	// Get provider from URL parameter
	provider := c.Param("provider")
	if provider != "authentik" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported OAuth provider"})
		return
	}

	// Get code and state from query parameters
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code parameter"})
		return
	}

	// Retrieve and verify the state from cookie
	savedState, err := c.Cookie("oauth_state")
	if err != nil || savedState == "" || savedState != state {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth state parameter"})
		c.Abort()
		return
	}

	// Clear the cookie after use
	c.SetCookie("oauth_state", "", -1, "/", "", true, true)

	// Initialize OAuth provider
	oauthProvider, err := auth.NewAuthentikProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize OAuth provider"})
		return
	}

	// Exchange code for token
	token, err := oauthProvider.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange OAuth code: " + err.Error()})
		return
	}

	// Get user info from token
	userInfo, err := oauthProvider.GetUserInfo(context.Background(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info: " + err.Error()})
		return
	}

	// Create or update user in database
	user, err := auth.SyncOAuthUser(c.Request.Context(), userInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync user: " + err.Error()})
		return
	}

	// Generate JWT token
	jwtToken, err := auth.GenerateJWT(user.ID, user.Username, user.Email, user.Permissions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Redirect to frontend with token
	// In a real app, you might want to use a better method for passing the token
	frontendRedirectURL := config.FrontendURL + "/oauth-callback?token=" + jwtToken
	c.Redirect(http.StatusTemporaryRedirect, frontendRedirectURL)
}
