package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("username already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrAccountNotFound   = errors.New("account not found")
)

// CreateUser creates a new user with the given username and password
func (s *Store) CreateUser(username, password string) (*User, error) {
	// Check if user exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Generate user ID
	id, err := generateID()
	if err != nil {
		return nil, err
	}

	// Insert user
	_, err = s.db.Exec(
		"INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)",
		id, username, string(hash),
	)
	if err != nil {
		return nil, err
	}

	// Create account for user
	accountID, err := generateID()
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(
		"INSERT INTO accounts (id, user_id) VALUES (?, ?)",
		accountID, id,
	)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:           id,
		Username:     username,
		PasswordHash: string(hash),
	}, nil
}

// AuthenticateUser checks username/password and returns the user if valid
func (s *Store) AuthenticateUser(username, password string) (*User, error) {
	user := &User{}
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (s *Store) GetUserByID(id string) (*User, error) {
	user := &User{}
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetAccountByUserID retrieves the account for a user
func (s *Store) GetAccountByUserID(userID string) (*Account, error) {
	acc := &Account{}
	err := s.db.QueryRow(
		"SELECT id, user_id, balance, created_at FROM accounts WHERE user_id = ?",
		userID,
	).Scan(&acc.ID, &acc.UserID, &acc.Balance, &acc.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// GetAccountByID retrieves an account by its ID
func (s *Store) GetAccountByID(accountID string) (*Account, error) {
	acc := &Account{}
	err := s.db.QueryRow(
		"SELECT id, user_id, balance, created_at FROM accounts WHERE id = ?",
		accountID,
	).Scan(&acc.ID, &acc.UserID, &acc.Balance, &acc.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
