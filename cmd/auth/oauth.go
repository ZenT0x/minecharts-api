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

	"golang.org/x/oauth2"
)

var (
	ErrInvalidOAuthConfig  = errors.New("invalid OAuth configuration")
	ErrOAuthExchangeFailed = errors.New("OAuth code exchange failed")
	ErrOAuthUserInfoFailed = errors.New("failed to get OAuth user info")
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
}

// NewAuthentikProvider creates a new OAuth provider for Authentik
func NewAuthentikProvider() (*OAuthProvider, error) {
	// Check if Authentik is enabled
	if !config.OAuthEnabled || !config.AuthentikEnabled {
		return nil, ErrInvalidOAuthConfig
	}

	// Validate required settings
	if config.AuthentikIssuer == "" || config.AuthentikClientID == "" || config.AuthentikClientSecret == "" {
		return nil, ErrInvalidOAuthConfig
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

	return &OAuthProvider{
		Config: oauthConfig,
		Name:   "authentik",
	}, nil
}

// GetAuthURL returns the URL to redirect the user to for authorization
func (p *OAuthProvider) GetAuthURL(state string) string {
	return p.Config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange exchanges the authorization code for a token
func (p *OAuthProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.Config.Exchange(ctx, code)
}

// GetUserInfo retrieves user information from the OAuth provider
func (p *OAuthProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
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

	return &OAuthUserInfo{
		ID:            userInfo.Sub,
		Email:         userInfo.Email,
		Username:      username,
		Name:          userInfo.Name,
		EmailVerified: userInfo.EmailVerified,
	}, nil
}

// SyncOAuthUser creates or updates a user based on OAuth information
func SyncOAuthUser(ctx context.Context, userInfo *OAuthUserInfo) (*database.User, error) {
	db := database.GetDB()

	// Check if user exists by email
	user, err := db.GetUserByUsername(ctx, userInfo.Username)

	// If user doesn't exist, create one
	if err == database.ErrUserNotFound {
		// Generate a secure random password (user will login via OAuth)
		randomPassword, err := GenerateRandomString(32)
		if err != nil {
			return nil, err
		}

		passwordHash, err := HashPassword(randomPassword)
		if err != nil {
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
			return nil, err
		}

		return newUser, nil
	} else if err != nil {
		return nil, err
	}

	// Update last login time
	now := time.Now()
	user.LastLogin = &now
	if err := db.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
