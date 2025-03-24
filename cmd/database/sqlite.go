package database

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDB implements the DB interface for SQLite
type SQLiteDB struct {
	db *sql.DB
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	return &SQLiteDB{db: db}, nil
}

// Init initializes the database schema
func (s *SQLiteDB) Init() error {
	// Create users table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			permissions INTEGER NOT NULL DEFAULT 0,
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
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			key TEXT UNIQUE NOT NULL,
			description TEXT,
			last_used TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Check if we need to create an admin user
	var count int
	err = s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	// If no users exist, create a default admin user
	if count == 0 {
		now := time.Now()
		_, err = s.db.Exec(
			"INSERT INTO users (username, email, password_hash, permissions, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
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
func (s *SQLiteDB) Close() error {
	return s.db.Close()
}

// User operations

// CreateUser creates a new user
func (s *SQLiteDB) CreateUser(ctx context.Context, user *User) error {
	// Check if user already exists
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = ? OR email = ?)",
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
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO users (username, email, password_hash, permissions, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		user.Username, user.Email, user.PasswordHash, user.Permissions, user.Active, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id

	return nil
}

// GetUserByID retrieves a user by ID
func (s *SQLiteDB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users WHERE id = ?",
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
func (s *SQLiteDB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, username, email, password_hash, permissions, active, last_login, created_at, updated_at FROM users WHERE username = ?",
		username,
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

// UpdateUser updates a user's information
func (s *SQLiteDB) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET username = ?, email = ?, password_hash = ?, permissions = ?, active = ?, updated_at = ? WHERE id = ?",
		user.Username, user.Email, user.PasswordHash, user.Permissions, user.Active, user.UpdatedAt, user.ID,
	)
	return err
}

// DeleteUser deletes a user by ID
func (s *SQLiteDB) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

// ListUsers returns a list of all users
func (s *SQLiteDB) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx,
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
func (s *SQLiteDB) CreateAPIKey(ctx context.Context, key *APIKey) error {
	now := time.Now()
	key.CreatedAt = now

	result, err := s.db.ExecContext(ctx,
		"INSERT INTO api_keys (user_id, key, description, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
		key.UserID, key.Key, key.Description, key.ExpiresAt, key.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	key.ID = id

	return nil
}

// GetAPIKey retrieves an API key by the key string
func (s *SQLiteDB) GetAPIKey(ctx context.Context, keyStr string) (*APIKey, error) {
	key := &APIKey{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, key, description, last_used, expires_at, created_at FROM api_keys WHERE key = ?",
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
	_, err = s.db.ExecContext(ctx, "UPDATE api_keys SET last_used = ? WHERE id = ?", now, key.ID)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// DeleteAPIKey deletes an API key by ID
func (s *SQLiteDB) DeleteAPIKey(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM api_keys WHERE id = ?", id)
	return err
}

// ListAPIKeysByUser lists all API keys for a user
func (s *SQLiteDB) ListAPIKeysByUser(ctx context.Context, userID int64) ([]*APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, user_id, key, description, last_used, expires_at, created_at FROM api_keys WHERE user_id = ?",
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
