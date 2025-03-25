package database

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// PostgresDB implements the DB interface for PostgreSQL
type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(connString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}

	return &PostgresDB{db: db}, nil
}

// Init initializes the database schema
func (p *PostgresDB) Init() error {
	// Create users table
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
		return err
	}

	// Create API keys table
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
		return err
	}

	// Check if we need to create an admin user
	var count int
	err = p.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	// If no users exist, create a default admin user
	if count == 0 {
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
			return err
		}
	}

	return nil
}

// Close closes the database connection
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// User operations

// CreateUser creates a new user
func (p *PostgresDB) CreateUser(ctx context.Context, user *User) error {
	// Check if user already exists
	var exists bool
	err := p.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 OR email = $2)",
		user.Username, user.Email,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
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

	return err
}

// GetUserByID retrieves a user by ID
func (p *PostgresDB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	err := p.db.QueryRowContext(ctx,
		"SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users WHERE id = $1",
		id,
	).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Permissions,
		&user.Active, &user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByUsername retrieves a user by username
func (p *PostgresDB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	log.Printf("Postgres: GetUserByUsername called for username: %s", username)

	user := &User{}
	query := "SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users WHERE username = $1"
	log.Printf("Postgres: Executing query: %s with username: %s", query, username)

	err := p.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Permissions,
		&user.Active, &user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		log.Printf("Postgres: User not found for username: %s", username)
		return nil, ErrUserNotFound
	}
	if err != nil {
		log.Printf("Postgres: Database error in GetUserByUsername: %v", err)
		return nil, err
	}

	log.Printf("Postgres: Successfully retrieved user: ID=%d, Username=%s", user.ID, user.Username)
	return user, nil
}

// UpdateUser updates a user's information
func (p *PostgresDB) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()

	_, err := p.db.ExecContext(ctx,
		"UPDATE users SET username = $1, email = $2, password_hash = $3, permissions = $4, active = $5, updated_at = $6 WHERE id = $7",
		user.Username, user.Email, user.PasswordHash, user.Permissions, user.Active, user.UpdatedAt, user.ID,
	)
	return err
}

// DeleteUser deletes a user by ID
func (p *PostgresDB) DeleteUser(ctx context.Context, id int64) error {
	_, err := p.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// ListUsers returns a list of all users
func (p *PostgresDB) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := p.db.QueryContext(ctx,
		"SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users",
	)
	if err != nil {
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
			return nil, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// API Key operations

// CreateAPIKey creates a new API key
func (p *PostgresDB) CreateAPIKey(ctx context.Context, key *APIKey) error {
	now := time.Now()
	key.CreatedAt = now

	err := p.db.QueryRowContext(ctx,
		"INSERT INTO api_keys (user_id, key, description, expires_at, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		key.UserID, key.Key, key.Description, key.ExpiresAt, key.CreatedAt,
	).Scan(&key.ID)

	return err
}

// GetAPIKey retrieves an API key by the key string
func (p *PostgresDB) GetAPIKey(ctx context.Context, keyStr string) (*APIKey, error) {
	key := &APIKey{}
	err := p.db.QueryRowContext(ctx,
		"SELECT id, user_id, key, description, last_used, expires_at, created_at FROM api_keys WHERE key = $1",
		keyStr,
	).Scan(
		&key.ID, &key.UserID, &key.Key, &key.Description, &key.LastUsed, &key.ExpiresAt, &key.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrInvalidAPIKey
	}
	if err != nil {
		return nil, err
	}

	// Update last used time
	now := time.Now()
	key.LastUsed = now
	_, err = p.db.ExecContext(ctx, "UPDATE api_keys SET last_used = $1 WHERE id = $2", now, key.ID)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// DeleteAPIKey deletes an API key by ID
func (p *PostgresDB) DeleteAPIKey(ctx context.Context, id int64) error {
	_, err := p.db.ExecContext(ctx, "DELETE FROM api_keys WHERE id = $1", id)
	return err
}

// ListAPIKeysByUser lists all API keys for a user
func (p *PostgresDB) ListAPIKeysByUser(ctx context.Context, userID int64) ([]*APIKey, error) {
	rows, err := p.db.QueryContext(ctx,
		"SELECT id, user_id, key, description, last_used, expires_at, created_at FROM api_keys WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []*APIKey{}
	for rows.Next() {
		key := &APIKey{}
		if err := rows.Scan(
			&key.ID, &key.UserID, &key.Key, &key.Description, &key.LastUsed, &key.ExpiresAt, &key.CreatedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}
