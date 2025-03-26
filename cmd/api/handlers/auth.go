// Package handlers contains the HTTP request handlers for the API endpoints.
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
	"minecharts/cmd/logging"

	"github.com/gin-gonic/gin"
)

// LoginRequest represents the login request payload.
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"secretpassword"`
}

// RegisterRequest represents the user registration request payload.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50" example:"newuser"`
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required,min=8" example:"securepass123"`
}

// LoginHandler authenticates users with username and password.
//
// @Summary      Login user
// @Description  Authenticate a user with username and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "Login credentials"
// @Success      200      {object}  map[string]interface{}  "Authentication successful"
// @Failure      400      {object}  map[string]string       "Invalid request format"
// @Failure      401      {object}  map[string]string       "Authentication failed"
// @Failure      403      {object}  map[string]string       "Account inactive"
// @Failure      500      {object}  map[string]string       "Server error"
// @Router       /auth/login [post]
func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Warn("Invalid login request format")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logging.WithFields(
		logging.F("username", req.Username),
		logging.F("remote_ip", c.ClientIP()),
	).Info("Login attempt")

	// Get user from database
	db := database.GetDB()
	logging.Debugf("Using database implementation: %T", db)

	user, err := db.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if err == database.ErrUserNotFound {
			logging.WithFields(
				logging.F("username", req.Username),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "user_not_found"),
			).Warn("Login failed: user not found")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("error", err.Error()),
		).Error("Database error during login")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	logging.WithFields(
		logging.F("username", user.Username),
		logging.F("user_id", user.ID),
	).Debug("User found during login process")

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("remote_ip", c.ClientIP()),
			logging.F("error", "invalid_password"),
		).Warn("Login failed: invalid password")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	logging.Debug("Password verification successful")

	// Check if user is active
	if !user.Active {
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("user_id", user.ID),
			logging.F("error", "account_inactive"),
		).Warn("Login failed: account inactive")
		c.JSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Username, user.Email, user.Permissions)
	if err != nil {
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("user_id", user.ID),
			logging.F("error", err.Error()),
		).Error("Failed to generate JWT token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Update last login time
	now := time.Now()
	user.LastLogin = &now
	if err := db.UpdateUser(c.Request.Context(), user); err != nil {
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("user_id", user.ID),
			logging.F("error", err.Error()),
		).Warn("Failed to update last login time")
	}

	logging.WithFields(
		logging.F("username", req.Username),
		logging.F("user_id", user.ID),
		logging.F("remote_ip", c.ClientIP()),
	).Info("Login successful")

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user_id":     user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
	})
}

// RegisterHandler creates a new user account.
//
// @Summary      Register new user
// @Description  Create a new user account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      RegisterRequest  true  "Registration information"
// @Success      201      {object}  map[string]interface{}  "Registration successful"
// @Failure      400      {object}  map[string]string       "Invalid request format"
// @Failure      409      {object}  map[string]string       "User already exists"
// @Failure      500      {object}  map[string]string       "Server error"
// @Router       /auth/register [post]
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
			logging.F("remote_ip", c.ClientIP()),
		).Warn("Invalid registration request format")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logging.WithFields(
		logging.F("username", req.Username),
		logging.F("email", req.Email),
		logging.F("remote_ip", c.ClientIP()),
	).Info("User registration attempt")

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("error", err.Error()),
		).Error("Failed to hash password during registration")
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
			logging.WithFields(
				logging.F("username", req.Username),
				logging.F("email", req.Email),
				logging.F("remote_ip", c.ClientIP()),
				logging.F("error", "user_exists"),
			).Warn("Registration failed: user already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		}
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("email", req.Email),
			logging.F("error", err.Error()),
		).Error("Database error during user registration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, user.Username, user.Email, user.Permissions)
	if err != nil {
		logging.WithFields(
			logging.F("username", req.Username),
			logging.F("user_id", user.ID),
			logging.F("error", err.Error()),
		).Error("Failed to generate JWT token during registration")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	logging.WithFields(
		logging.F("username", req.Username),
		logging.F("user_id", user.ID),
		logging.F("email", req.Email),
		logging.F("remote_ip", c.ClientIP()),
	).Info("User registration successful")

	c.JSON(http.StatusCreated, gin.H{
		"token":       token,
		"user_id":     user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"permissions": user.Permissions,
	})
}

// GetUserInfoHandler returns information about the authenticated user.
//
// @Summary      Get current user info
// @Description  Returns information about the currently authenticated user
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]interface{}  "User information"
// @Failure      401  {object}  map[string]string       "Authentication required"
// @Router       /auth/me [get]
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

// GenerateStateValue creates a random state value for OAuth flows.
// It returns a base64-encoded random string and any error encountered.
func GenerateStateValue() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// OAuthLoginHandler initiates the OAuth login flow.
//
// @Summary      Start OAuth login
// @Description  Redirects to OAuth provider's login page
// @Tags         auth
// @Produce      html
// @Param        provider  path      string  true  "OAuth provider (e.g., 'authentik')"
// @Success      307       {string}  string  "Redirect to OAuth provider"
// @Failure      400       {object}  map[string]string  "OAuth not enabled or invalid provider"
// @Failure      500       {object}  map[string]string  "Server error"
// @Router       /auth/oauth/{provider} [get]
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

// OAuthCallbackHandler handles the OAuth callback from providers.
//
// @Summary      OAuth callback
// @Description  Handles the callback from OAuth provider and creates/authenticates user
// @Tags         auth
// @Produce      html
// @Param        provider  path      string  true  "OAuth provider (e.g., 'authentik')"
// @Param        code      query     string  true  "OAuth code"
// @Param        state     query     string  true  "OAuth state"
// @Success      307       {string}  string  "Redirect to frontend with token"
// @Failure      400       {object}  map[string]string  "Invalid request or state mismatch"
// @Failure      500       {object}  map[string]string  "Server error"
// @Router       /auth/callback/{provider} [get]
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
