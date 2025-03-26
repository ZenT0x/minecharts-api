package database

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
)

// Supported database types
const (
	SQLite     = "sqlite"
	PostgreSQL = "postgres"
)

var (
	ErrUserExists      = errors.New("user already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidAPIKey   = errors.New("invalid API key")
)

// DB is the interface that must be implemented by database providers
type DB interface {
	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id int64) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id int64) error
	ListUsers(ctx context.Context) ([]*User, error)

	// API Key operations
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKey(ctx context.Context, key string) (*APIKey, error)
	DeleteAPIKey(ctx context.Context, id int64) error
	ListAPIKeysByUser(ctx context.Context, userID int64) ([]*APIKey, error)

	// Database operations
	Init() error
	Close() error
}

// Global database instance
var (
	db     DB
	dbOnce sync.Once
)

// InitDB initializes the database with the provided configuration
func InitDB(dbType string, connectionString string) error {
	log.Printf("Initializing database of type: %s", dbType)
	var err error
	dbOnce.Do(func() {
		switch dbType {
		case SQLite:
			log.Printf("Creating SQLite database connection to: %s", connectionString)
			db, err = NewSQLiteDB(connectionString)
		case PostgreSQL:
			log.Printf("Creating PostgreSQL database connection to: %s", connectionString)
			db, err = NewPostgresDB(connectionString)
		default:
			// Default to SQLite if not specified
			log.Printf("Unknown database type: %s, using SQLite as default", dbType)
			db, err = NewSQLiteDB(connectionString)
		}

		if err != nil {
			log.Printf("Failed to initialize database: %v", err)
			return
		}

		log.Printf("Database connection established, initializing schema")
		if err = db.Init(); err != nil {
			log.Printf("Failed to initialize database schema: %v", err)
		} else {
			log.Printf("Database schema initialized successfully")
		}
	})

	if err != nil {
		log.Printf("Database initialization failed: %v", err)
	} else {
		log.Printf("Database initialization completed successfully")
	}
	return err
}

// GetDB returns the global database instance
func GetDB() DB {
	if db == nil {
		log.Printf("Database instance not initialized, creating default SQLite instance")
		// Default to SQLite with a file in the data directory
		dataDir := os.Getenv("DATA_DIR")
		if dataDir == "" {
			dataDir = "./app/data"
			log.Printf("DATA_DIR environment variable not set, using default: %s", dataDir)
		} else {
			log.Printf("Using DATA_DIR from environment: %s", dataDir)
		}

		// Create data directory if it doesn't exist
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Printf("Failed to create data directory: %v", err)
		} else {
			log.Printf("Data directory created or already exists: %s", dataDir)
		}

		dbPath := dataDir + "/minecharts.db"
		log.Printf("Initializing default SQLite database at: %s", dbPath)
		InitDB(SQLite, dbPath)
	} else {
		log.Printf("Using existing database instance")
	}
	return db
}
