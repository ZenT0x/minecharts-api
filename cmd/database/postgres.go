package database

import (
	"context"
	"database/sql"
	"time"

	"minecharts/cmd/logging"

	_ "github.com/lib/pq"
)

// PostgresDB implements the DB interface for PostgreSQL
type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(connString string) (*PostgresDB, error) {
	logging.WithFields(
		logging.F("db_type", "postgres"),
	).Info("Creating new PostgreSQL database connection")

	db, err := sql.Open("postgres", connString)
	if err != nil {
		logging.WithFields(
			logging.F("db_type", "postgres"),
			logging.F("error", err.Error()),
		).Error("Failed to open PostgreSQL database connection")
		return nil, err
	}

	logging.Debug("PostgreSQL database connection established")
	return &PostgresDB{db: db}, nil
}

// Init initializes the database schema
func (p *PostgresDB) Init() error {
	logging.Info("Initializing PostgreSQL database schema")

	// Create users table
	logging.Debug("Creating users table if not exists")
	_, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			permissions BIGINT NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			last_login TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Failed to create users table")
		return err
	}

	// Create API keys table
	logging.Debug("Creating api_keys table if not exists")
	_, err = p.db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			key TEXT UNIQUE NOT NULL,
			description TEXT,
			last_used TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Failed to create api_keys table")
		return err
	}

	// Check if we need to create an admin user
	logging.Debug("Checking if admin user needs to be created")
	var count int
	err = p.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Failed to count users")
		return err
	}

	// If no users exist, create a default admin user
	if count == 0 {
		logging.Info("Creating default admin user")
		now := time.Now()
		_, err = p.db.Exec(
			"INSERT INTO users (username, email, password_hash, permissions, active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			"admin",
			"admin@example.com",
			"$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", // password: admin
			PermAll,
			true,
			now,
			now,
		)
		if err != nil {
			logging.WithFields(
				logging.F("error", err.Error()),
			).Error("Failed to create default admin user")
			return err
		}
		logging.Info("Default admin user created successfully")
	}

	logging.Info("PostgreSQL database schema initialized successfully")
	return nil
}

// Close closes the database connection
func (p *PostgresDB) Close() error {
	logging.Info("Closing PostgreSQL database connection")
	err := p.db.Close()
	if err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Error closing PostgreSQL database connection")
		return err
	}
	logging.Debug("PostgreSQL database connection closed successfully")
	return nil
}

// User operations

// CreateUser creates a new user
func (p *PostgresDB) CreateUser(ctx context.Context, user *User) error {
	logging.WithFields(
		logging.F("username", user.Username),
		logging.F("email", user.Email),
	).Info("Creating new user in PostgreSQL")

	// Check if user already exists
	var exists bool
	err := p.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)",
		user.Username, user.Email,
	).Scan(&exists)
	if err != nil {
		logging.WithFields(
			logging.F("username", user.Username),
			logging.F("email", user.Email),
			logging.F("error", err.Error()),
		).Error("Database error when checking if user exists")
		return err
	}
	if exists {
		logging.WithFields(
			logging.F("username", user.Username),
			logging.F("email", user.Email),
			logging.F("error", "user_exists"),
		).Warn("Cannot create user: username or email already exists")
		return ErrUserExists
	}

	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Insert user
	err = p.db.QueryRowContext(ctx,
		"INSERT INTO users (username, email, password_hash, permissions, active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
		user.Username, user.Email, user.PasswordHash, user.Permissions, user.Active, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID)
	if err != nil {
		logging.WithFields(
			logging.F("username", user.Username),
			logging.F("email", user.Email),
			logging.F("error", err.Error()),
		).Error("Failed to insert new user")
		return err
	}

	logging.WithFields(
		logging.F("username", user.Username),
		logging.F("email", user.Email),
		logging.F("user_id", user.ID),
	).Info("User created successfully in PostgreSQL")
	return nil
}

// GetUserByID retrieves a user by ID
func (p *PostgresDB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	logging.WithFields(
		logging.F("user_id", id),
		logging.F("db_type", "postgres"),
	).Debug("Getting user by ID")

	user := &User{}
	err := p.db.QueryRowContext(ctx,
		"SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users WHERE id = $1",
		id,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Permissions,
		&user.Active, &user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		logging.WithFields(
			logging.F("user_id", id),
			logging.F("error", "user_not_found"),
		).Debug("User not found by ID")
		return nil, ErrUserNotFound
	}
	if err != nil {
		logging.WithFields(
			logging.F("user_id", id),
			logging.F("error", err.Error()),
		).Error("Database error when getting user by ID")
		return nil, err
	}

	logging.WithFields(
		logging.F("user_id", id),
		logging.F("username", user.Username),
	).Debug("Successfully retrieved user by ID")
	return user, nil
}

// GetUserByUsername retrieves a user by username
func (p *PostgresDB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	logging.WithFields(
		logging.F("username", username),
		logging.F("db_type", "postgres"),
	).Debug("Getting user by username")

	user := &User{}
	query := "SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users WHERE username = $1"

	logging.WithFields(
		logging.F("username", username),
		logging.F("query", query),
	).Trace("Executing database query")

	err := p.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Permissions,
		&user.Active, &user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		logging.WithFields(
			logging.F("username", username),
			logging.F("error", "user_not_found"),
		).Debug("User not found")
		return nil, ErrUserNotFound
	}
	if err != nil {
		logging.WithFields(
			logging.F("username", username),
			logging.F("error", err.Error()),
		).Error("Database error when getting user by username")
		return nil, err
	}

	logging.WithFields(
		logging.F("username", user.Username),
		logging.F("user_id", user.ID),
	).Debug("Successfully retrieved user")
	return user, nil
}

// UpdateUser updates a user's information
func (p *PostgresDB) UpdateUser(ctx context.Context, user *User) error {
	logging.WithFields(
		logging.F("user_id", user.ID),
		logging.F("username", user.Username),
	).Info("Updating user information in PostgreSQL")

	user.UpdatedAt = time.Now()

	_, err := p.db.ExecContext(ctx,
		"UPDATE users SET username = $1, email = $2, password_hash = $3, permissions = $4, active = $5, updated_at = $6 WHERE id = $7",
		user.Username, user.Email, user.PasswordHash, user.Permissions, user.Active, user.UpdatedAt, user.ID,
	)
	if err != nil {
		logging.WithFields(
			logging.F("user_id", user.ID),
			logging.F("username", user.Username),
			logging.F("error", err.Error()),
		).Error("Failed to update user")
		return err
	}

	logging.WithFields(
		logging.F("user_id", user.ID),
		logging.F("username", user.Username),
	).Info("User updated successfully")
	return nil
}

// DeleteUser deletes a user by ID
func (p *PostgresDB) DeleteUser(ctx context.Context, id int64) error {
	logging.WithFields(
		logging.F("user_id", id),
	).Info("Deleting user from PostgreSQL")

	_, err := p.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		logging.WithFields(
			logging.F("user_id", id),
			logging.F("error", err.Error()),
		).Error("Failed to delete user")
		return err
	}

	logging.WithFields(
		logging.F("user_id", id),
	).Info("User deleted successfully")
	return nil
}

// ListUsers returns a list of all users
func (p *PostgresDB) ListUsers(ctx context.Context) ([]*User, error) {
	logging.Debug("Listing all users from PostgreSQL")

	rows, err := p.db.QueryContext(ctx,
		"SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users",
	)
	if err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Failed to query users")
		return nil, err
	}
	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		user := &User{}
		if err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Permissions,
			&user.Active, &user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			logging.WithFields(
				logging.F("error", err.Error()),
			).Error("Failed to scan user row")
			return nil, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		logging.WithFields(
			logging.F("error", err.Error()),
		).Error("Error during user rows iteration")
		return nil, err
	}

	logging.WithFields(
		logging.F("count", len(users)),
	).Debug("Successfully retrieved user list from PostgreSQL")
	return users, nil
}

// API Key operations

// CreateAPIKey creates a new API key
func (p *PostgresDB) CreateAPIKey(ctx context.Context, key *APIKey) error {
	logging.WithFields(
		logging.F("user_id", key.UserID),
		logging.F("description", key.Description),
	).Info("Creating new API key in PostgreSQL")

	now := time.Now()
	key.CreatedAt = now

	err := p.db.QueryRowContext(ctx,
		"INSERT INTO api_keys (user_id, key, description, expires_at, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		key.UserID, key.Key, key.Description, key.ExpiresAt, key.CreatedAt,
	).Scan(&key.ID)
	if err != nil {
		logging.WithFields(
			logging.F("user_id", key.UserID),
			logging.F("error", err.Error()),
		).Error("Failed to insert API key")
		return err
	}

	logging.WithFields(
		logging.F("user_id", key.UserID),
		logging.F("key_id", key.ID),
	).Info("API key created successfully in PostgreSQL")
	return nil
}

// GetAPIKey retrieves an API key by the key string
func (p *PostgresDB) GetAPIKey(ctx context.Context, keyStr string) (*APIKey, error) {
	// Mask the full key in logs for security
	maskedKey := keyStr
	if len(keyStr) > 8 {
		maskedKey = keyStr[:4] + "..." + keyStr[len(keyStr)-4:]
	}

	logging.WithFields(
		logging.F("key", maskedKey),
	).Debug("Looking up API key in PostgreSQL")

	key := &APIKey{}
	err := p.db.QueryRowContext(ctx,
		"SELECT id, user_id, key, description, last_used, expires_at, created_at FROM api_keys WHERE key = $1",
		keyStr,
	).Scan(
		&key.ID, &key.UserID, &key.Key, &key.Description, &key.LastUsed, &key.ExpiresAt, &key.CreatedAt,
	)
	if err == sql.ErrNoRows {
		logging.WithFields(
			logging.F("key", maskedKey),
			logging.F("error", "invalid_api_key"),
		).Warn("API key not found")
		return nil, ErrInvalidAPIKey
	}
	if err != nil {
		logging.WithFields(
			logging.F("key", maskedKey),
			logging.F("error", err.Error()),
		).Error("Database error when retrieving API key")
		return nil, err
	}

	// Update last used time
	now := time.Now()
	key.LastUsed = now
	_, err = p.db.ExecContext(ctx, "UPDATE api_keys SET last_used = $1 WHERE id = $2", now, key.ID)
	if err != nil {
		logging.WithFields(
			logging.F("key_id", key.ID),
			logging.F("error", err.Error()),
		).Error("Failed to update API key last used time")
		return nil, err
	}

	// Check if the key has expired
	if !key.ExpiresAt.IsZero() && key.ExpiresAt.Before(now) {
		logging.WithFields(
			logging.F("key_id", key.ID),
			logging.F("user_id", key.UserID),
			logging.F("expired_at", key.ExpiresAt),
		).Warn("Attempted to use expired API key")
		return nil, ErrInvalidAPIKey
	}

	logging.WithFields(
		logging.F("key_id", key.ID),
		logging.F("user_id", key.UserID),
	).Debug("API key found and last used time updated")
	return key, nil
}

// DeleteAPIKey deletes an API key by ID
func (p *PostgresDB) DeleteAPIKey(ctx context.Context, id int64) error {
	logging.WithFields(
		logging.F("key_id", id),
	).Info("Deleting API key from PostgreSQL")

	_, err := p.db.ExecContext(ctx, "DELETE FROM api_keys WHERE id = $1", id)
	if err != nil {
		logging.WithFields(
			logging.F("key_id", id),
			logging.F("error", err.Error()),
		).Error("Failed to delete API key")
		return err
	}

	logging.WithFields(
		logging.F("key_id", id),
	).Info("API key deleted successfully")
	return err
}

// ListAPIKeysByUser lists all API keys for a user
func (p *PostgresDB) ListAPIKeysByUser(ctx context.Context, userID int64) ([]*APIKey, error) {
	logging.WithFields(
		logging.F("user_id", userID),
	).Debug("Listing API keys for user from PostgreSQL")

	rows, err := p.db.QueryContext(ctx,
		"SELECT id, user_id, key, description, last_used, expires_at, created_at FROM api_keys WHERE user_id = $1",
		userID,
	)
	if err != nil {
		logging.WithFields(
			logging.F("user_id", userID),
			logging.F("error", err.Error()),
		).Error("Failed to query API keys")
		return nil, err
	}
	defer rows.Close()

	keys := []*APIKey{}
	for rows.Next() {
		key := &APIKey{}
		if err := rows.Scan(
			&key.ID, &key.UserID, &key.Key, &key.Description, &key.LastUsed, &key.ExpiresAt, &key.CreatedAt,
		); err != nil {
			logging.WithFields(
				logging.F("user_id", userID),
				logging.F("error", err.Error()),
			).Error("Failed to scan API key row")
			return nil, err
		}
		keys = append(keys, key)
	}

	if err = rows.Err(); err != nil {
		logging.WithFields(
			logging.F("user_id", userID),
			logging.F("error", err.Error()),
		).Error("Error during API key rows iteration")
		return nil, err
	}

	logging.WithFields(
		logging.F("user_id", userID),
		logging.F("count", len(keys)),
	).Debug("Successfully retrieved API keys from PostgreSQL")
	return keys, nil
}
