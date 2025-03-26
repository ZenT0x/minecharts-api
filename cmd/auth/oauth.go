package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"minecharts/cmd/config"
	"minecharts/cmd/database"
	"minecharts/cmd/logging"

	"golang.org/x/oauth2"
)

var (
	ErrInvalidOAuthConfig    = errors.New("invalid OAuth configuration")
	ErrOAuthExchangeFailed   = errors.New("OAuth code exchange failed")
	ErrOAuthUserInfoFailed   = errors.New("failed to get OAuth user info")
	ErrOAuthNotEnabled       = errors.New("oauth is not enabled")
	ErrUnsupportedProvider   = errors.New("unsupported oauth provider")
	ErrMissingProviderConfig = errors.New("missing oauth provider configuration")
	ErrUserInfoRetrieval     = errors.New("failed to retrieve user information")
)

// OAuthProvider represents an OAuth 2.0 provider
type OAuthProvider struct {
	Config *oauth2.Config
	Name   string
}

// OAuthUserInfo contains user information from the OAuth provider
type OAuthUserInfo struct {
	ID            string
	Email         string
	Username      string
	Name          string
	EmailVerified bool
	FirstName     string
	LastName      string
	Picture       string
	Provider      string
}

// NewAuthentikProvider creates a new OAuth provider for Authentik
func NewAuthentikProvider() (*OAuthProvider, error) {
	logging.Debug("Initializing Authentik OAuth provider")

	if !config.OAuthEnabled || !config.AuthentikEnabled {
		logging.WithFields(
			logging.F("oauth_enabled", config.OAuthEnabled),
			logging.F("authentik_enabled", config.AuthentikEnabled),
		).Warn("Authentik OAuth is not enabled")
		return nil, ErrOAuthNotEnabled
	}

	if config.AuthentikClientID == "" || config.AuthentikClientSecret == "" ||
		config.AuthentikIssuer == "" || config.AuthentikRedirectURL == "" {
		logging.WithFields(
			logging.F("client_id_set", config.AuthentikClientID != ""),
			logging.F("client_secret_set", config.AuthentikClientSecret != ""),
			logging.F("issuer_set", config.AuthentikIssuer != ""),
			logging.F("redirect_url_set", config.AuthentikRedirectURL != ""),
		).Error("Authentik OAuth configuration is incomplete")
		return nil, ErrMissingProviderConfig
	}

	// Construct OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     config.AuthentikClientID,
		ClientSecret: config.AuthentikClientSecret,
		RedirectURL:  config.AuthentikRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.AuthentikIssuer + "/oauth2/authorize",
			TokenURL: config.AuthentikIssuer + "/oauth2/token",
		},
	}

	logging.WithFields(
		logging.F("issuer", config.AuthentikIssuer),
		logging.F("redirect_url", config.AuthentikRedirectURL),
	).Info("Authentik OAuth provider initialized successfully")

	return &OAuthProvider{
		Config: oauthConfig,
		Name:   "authentik",
	}, nil
}

// GetAuthURL returns the URL to redirect the user to for authorization
func (p *OAuthProvider) GetAuthURL(state string) string {
	url := p.Config.AuthCodeURL(state, oauth2.AccessTypeOnline)

	logging.WithFields(
		logging.F("url", url),
		logging.F("state", state),
	).Debug("Generated Authentik OAuth authorization URL")

	return url
}

// Exchange exchanges the authorization code for a token
func (p *OAuthProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	logging.Debug("Exchanging OAuth code for token")

	token, err := p.Config.Exchange(ctx, code)
	if err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Failed to exchange OAuth code for token")
		return nil, err
	}

	logging.WithFields(
		logging.F("token_type", token.TokenType),
		logging.F("expiry", token.Expiry),
	).Debug("Successfully exchanged OAuth code for token")

	return token, nil
}

// GetUserInfo retrieves user information from the OAuth provider
func (p *OAuthProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	logging.Debug("Fetching user info from Authentik")

	client := p.Config.Client(ctx, token)

	// Get user info from Authentik's userinfo endpoint
	resp, err := client.Get(config.AuthentikIssuer + "/oauth2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrOAuthUserInfoFailed
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
	}

	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	// Use preferred_username or derive username from email if not provided
	username := userInfo.PreferredUsername
	if username == "" {
		if userInfo.Email != "" {
			parts := strings.Split(userInfo.Email, "@")
			username = parts[0]
		} else {
			username = "user_" + userInfo.Sub
		}
	}

	logging.WithFields(
		logging.F("provider", "authentik"),
		logging.F("username", username),
	).Debug("Successfully retrieved user info from Authentik")

	return &OAuthUserInfo{
		ID:            userInfo.Sub,
		Email:         userInfo.Email,
		Username:      username,
		Name:          userInfo.Name,
		EmailVerified: userInfo.EmailVerified,
		Provider:      "authentik",
	}, nil
}

// SyncOAuthUser creates or updates a user based on OAuth information
func SyncOAuthUser(ctx context.Context, userInfo *OAuthUserInfo) (*database.User, error) {
	logging.WithFields(
		logging.F("provider", userInfo.Provider),
		logging.F("username", userInfo.Username),
		logging.F("email", userInfo.Email),
	).Info("Syncing OAuth user with database")

	db := database.GetDB()

	// Check if user exists by email
	user, err := db.GetUserByUsername(ctx, userInfo.Username)

	// If user doesn't exist, create one
	if err == database.ErrUserNotFound {
		// Generate a secure random password (user will login via OAuth)
		randomPassword, err := GenerateRandomString(32)
		if err != nil {
			logging.WithFields(
				logging.F("error", err.Error()),
			).Error("Failed to generate random password for OAuth user")
			return nil, err
		}

		passwordHash, err := HashPassword(randomPassword)
		if err != nil {
			logging.WithFields(
				logging.F("error", err.Error()),
			).Error("Failed to hash random password for OAuth user")
			return nil, err
		}

		// Create new user with read-only permissions by default
		now := time.Now()
		newUser := &database.User{
			Username:     userInfo.Username,
			Email:        userInfo.Email,
			PasswordHash: passwordHash,
			Permissions:  int64(database.PermReadOnly), // Default to read-only permissions
			Active:       true,
			LastLogin:    &now,
		}

		if err := db.CreateUser(ctx, newUser); err != nil {
			logging.WithFields(
				logging.F("username", userInfo.Username),
				logging.F("email", userInfo.Email),
				logging.F("error", err.Error()),
			).Error("Failed to create user from OAuth information")
			return nil, err
		}

		logging.WithFields(
			logging.F("user_id", newUser.ID),
			logging.F("username", newUser.Username),
		).Info("New user created from OAuth information")

		return newUser, nil
	} else if err != nil {
		logging.WithFields(
			logging.F("username", userInfo.Username),
			logging.F("error", err.Error()),
		).Error("Database error while looking up user by username")
		return nil, err
	}

	// Update last login time
	now := time.Now()
	user.LastLogin = &now
	if err := db.UpdateUser(ctx, user); err != nil {
		logging.WithFields(
			logging.F("user_id", user.ID),
			logging.F("username", user.Username),
			logging.F("error", err.Error()),
		).Warn("Failed to update last login time for OAuth user")
	}

	logging.WithFields(
		logging.F("user_id", user.ID),
		logging.F("username", user.Username),
	).Info("Existing user updated from OAuth information")

	return user, nil
}
