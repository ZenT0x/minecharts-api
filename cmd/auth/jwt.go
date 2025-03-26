package auth

import (
	"errors"
	"fmt"
	"time"

	"minecharts/cmd/config"
	"minecharts/cmd/logging"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims represents the JWT claims used for authentication
type Claims struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Permissions int64  `json:"permissions"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a new JWT token for the given user information
func GenerateJWT(userID int64, username, email string, permissions int64) (string, error) {
	expirationTime := time.Now().Add(time.Duration(config.JWTExpiryHours) * time.Hour)

	claims := &Claims{
		UserID:      userID,
		Username:    username,
		Email:       email,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.JWTSecret))

	if err != nil {
		logging.WithFields(
			logging.F("user_id", userID),
			logging.F("username", username),
			logging.F("error", err.Error()),
		).Error("Failed to sign JWT token")
		return "", err
	}

	logging.WithFields(
		logging.F("user_id", userID),
		logging.F("username", username),
		logging.F("expires_at", expirationTime),
	).Debug("JWT token generated successfully")

	return tokenString, nil
}

// ValidateJWT validates the JWT token and returns the claims if valid
func ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				errMsg := fmt.Sprintf("unexpected signing method: %v", token.Header["alg"])
				logging.WithFields(
					logging.F("error", errMsg),
				).Warn("JWT validation failed")
				return nil, errors.New(errMsg)
			}
			return []byte(config.JWTSecret), nil
		},
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			logging.WithFields(
				logging.F("error", "token_expired"),
			).Debug("JWT validation failed: token expired")
			return nil, ErrExpiredToken
		}
		logging.WithFields(
			logging.F("error", err.Error()),
		).Warn("JWT validation failed")
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		logging.WithFields(
			logging.F("error", "invalid_token"),
		).Warn("JWT validation failed: invalid token")
		return nil, ErrInvalidToken
	}

	logging.WithFields(
		logging.F("user_id", claims.UserID),
		logging.F("username", claims.Username),
	).Trace("JWT token validated")

	return claims, nil
}
