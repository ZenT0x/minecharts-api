package config

import (
	"os"
	"strconv"
)

// Global configuration variables, configurable via environment variables.
var (
	// Server configuration
	DefaultNamespace = getEnv("MINECHARTS_NAMESPACE", "minecharts")
	DeploymentPrefix = getEnv("MINECHARTS_DEPLOYMENT_PREFIX", "minecraft-server-")
	PVCSuffix        = getEnv("MINECHARTS_PVC_SUFFIX", "-pvc")
	StorageSize      = getEnv("MINECHARTS_STORAGE_SIZE", "10Gi")
	StorageClass     = getEnv("MINECHARTS_STORAGE_CLASS", "rook-ceph-block")
	DefaultReplicas  = 1

	// Database configuration
	DatabaseType             = getEnv("MINECHARTS_DB_TYPE", "sqlite")                         // "sqlite" or "postgres"
	DatabaseConnectionString = getEnv("MINECHARTS_DB_CONNECTION", "./app/data/minecharts.db") // File path for SQLite or connection string for Postgres

	// Authentication configuration
	JWTSecret      = getEnv("MINECHARTS_JWT_SECRET", "your-secret-key-change-me-in-production")
	JWTExpiryHours = getEnvInt("MINECHARTS_JWT_EXPIRY_HOURS", 24)
	APIKeyPrefix   = getEnv("MINECHARTS_API_KEY_PREFIX", "mcapi")

	// OAuth configuration
	OAuthEnabled = getEnvBool("MINECHARTS_OAUTH_ENABLED", false)

	// Authentik OAuth configuration
	AuthentikEnabled      = getEnvBool("MINECHARTS_AUTHENTIK_ENABLED", false)
	AuthentikIssuer       = getEnv("MINECHARTS_AUTHENTIK_ISSUER", "") // e.g., https://auth.example.com/application/o/
	AuthentikClientID     = getEnv("MINECHARTS_AUTHENTIK_CLIENT_ID", "")
	AuthentikClientSecret = getEnv("MINECHARTS_AUTHENTIK_CLIENT_SECRET", "")
	AuthentikRedirectURL  = getEnv("MINECHARTS_AUTHENTIK_REDIRECT_URL", "") // e.g., http://localhost:8080/api/auth/callback/authentik

	// URL Frontend configuration
	FrontendURL = "http://localhost:3000"

	// Timezone configuration
	TimeZone = getEnv("MINECHARTS_TIMEZONE", "UTC") // Valeur par défaut: UTC

	// Logging configuration
	LogLevel  = getEnv("MINECHARTS_LOG_LEVEL", "info")  // Possible values: trace, debug, info, warn, error, fatal, panic
	LogFormat = getEnv("MINECHARTS_LOG_FORMAT", "json") // Possible values: json, text
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}
