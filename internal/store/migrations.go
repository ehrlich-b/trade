package store

import (
	"database/sql"
	"fmt"
)

// Migration represents a database schema migration
type Migration struct {
	Version     int
	Description string
	SQL         string
}

// migrations is the ordered list of all migrations
// New migrations should be appended to the end with incrementing version numbers
var migrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema",
		SQL: `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			balance INTEGER NOT NULL DEFAULT 100000000,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL REFERENCES accounts(id),
			symbol TEXT NOT NULL,
			quantity INTEGER NOT NULL DEFAULT 0,
			avg_price INTEGER NOT NULL DEFAULT 0,
			realized_pnl INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(account_id, symbol)
		);

		CREATE TABLE IF NOT EXISTS trade_history (
			id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL REFERENCES accounts(id),
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			price INTEGER NOT NULL,
			quantity INTEGER NOT NULL,
			pnl INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts(user_id);
		CREATE INDEX IF NOT EXISTS idx_positions_account ON positions(account_id);
		CREATE INDEX IF NOT EXISTS idx_trade_history_account ON trade_history(account_id);
		`,
	},
	{
		Version:     2,
		Description: "Settlement tables",
		SQL: `
		CREATE TABLE IF NOT EXISTS daily_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL REFERENCES accounts(id),
			date DATE NOT NULL,
			balance INTEGER NOT NULL,
			realized_pnl INTEGER NOT NULL,
			positions TEXT,
			is_bankrupt BOOLEAN DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(account_id, date)
		);

		CREATE INDEX IF NOT EXISTS idx_snapshots_date ON daily_snapshots(date);
		`,
	},
	{
		Version:     3,
		Description: "Persistent sessions",
		SQL: `
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			account_id TEXT NOT NULL REFERENCES accounts(id),
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
		`,
	},
	{
		Version:     4,
		Description: "Match history and user stats",
		SQL: `
		CREATE TABLE IF NOT EXISTS matches (
			id TEXT PRIMARY KEY,
			symbol TEXT NOT NULL,
			duration_minutes INTEGER NOT NULL,
			target_nav INTEGER NOT NULL,
			final_nav INTEGER NOT NULL,
			participant_count INTEGER NOT NULL,
			started_at DATETIME NOT NULL,
			ended_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS match_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			match_id TEXT NOT NULL REFERENCES matches(id),
			user_id TEXT NOT NULL REFERENCES users(id),
			starting_value INTEGER NOT NULL,
			final_value INTEGER NOT NULL,
			pnl INTEGER NOT NULL,
			rank INTEGER NOT NULL,
			starting_shares INTEGER NOT NULL,
			final_shares INTEGER NOT NULL,
			starting_cash INTEGER NOT NULL,
			final_cash INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(match_id, user_id)
		);

		CREATE TABLE IF NOT EXISTS user_stats (
			user_id TEXT PRIMARY KEY REFERENCES users(id),
			matches_played INTEGER NOT NULL DEFAULT 0,
			matches_won INTEGER NOT NULL DEFAULT 0,
			total_pnl INTEGER NOT NULL DEFAULT 0,
			best_pnl INTEGER NOT NULL DEFAULT 0,
			worst_pnl INTEGER NOT NULL DEFAULT 0,
			current_streak INTEGER NOT NULL DEFAULT 0,
			best_streak INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_matches_ended ON matches(ended_at);
		CREATE INDEX IF NOT EXISTS idx_match_results_user ON match_results(user_id);
		CREATE INDEX IF NOT EXISTS idx_match_results_match ON match_results(match_id);
		`,
	},
}

// initMigrationsTable creates the migrations tracking table
func (s *Store) initMigrationsTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// getCurrentVersion returns the highest applied migration version
func (s *Store) getCurrentVersion() (int, error) {
	var version int
	err := s.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}

// Migrate runs all pending migrations
func (s *Store) Migrate() error {
	if err := s.initMigrationsTable(); err != nil {
		return fmt.Errorf("failed to init migrations table: %w", err)
	}

	currentVersion, err := s.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		if err := s.applyMigration(m); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Description, err)
		}
	}

	return nil
}

// applyMigration runs a single migration in a transaction
func (s *Store) applyMigration(m Migration) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Run the migration SQL
	if _, err := tx.Exec(m.SQL); err != nil {
		return err
	}

	// Record the migration
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
		m.Version, m.Description,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// MigrationStatus returns applied and pending migrations
func (s *Store) MigrationStatus() (applied []int, pending []int, err error) {
	if err := s.initMigrationsTable(); err != nil {
		return nil, nil, err
	}

	// Get applied versions
	rows, err := s.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	appliedSet := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, nil, err
		}
		applied = append(applied, v)
		appliedSet[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Find pending
	for _, m := range migrations {
		if !appliedSet[m.Version] {
			pending = append(pending, m.Version)
		}
	}

	return applied, pending, nil
}

// GetDB returns the underlying database connection for advanced operations
func (s *Store) GetDB() *sql.DB {
	return s.db
}
