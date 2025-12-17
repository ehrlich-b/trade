package store

import (
	"database/sql"
	"time"
)

// Session represents an authenticated user session
type Session struct {
	Token     string
	UserID    string
	AccountID string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CreateSession creates a new session in the database
func (s *Store) CreateSession(token, userID, accountID string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO sessions (token, user_id, account_id, expires_at) VALUES (?, ?, ?, ?)",
		token, userID, accountID, expiresAt,
	)
	return err
}

// GetSession retrieves a session by token, returns nil if not found or expired
func (s *Store) GetSession(token string) (*Session, error) {
	session := &Session{}
	err := s.db.QueryRow(
		"SELECT token, user_id, account_id, expires_at, created_at FROM sessions WHERE token = ?",
		token,
	).Scan(&session.Token, &session.UserID, &session.AccountID, &session.ExpiresAt, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		// Delete expired session
		s.DeleteSession(token)
		return nil, nil
	}
	return session, nil
}

// DeleteSession removes a session from the database
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

// CleanupExpiredSessions removes all expired sessions
func (s *Store) CleanupExpiredSessions() error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now())
	return err
}
