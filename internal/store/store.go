package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// Store provides SQLite persistence for the trading game
type Store struct {
	db *sql.DB
}

// New creates a new Store and initializes the schema
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.InitSettlementSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id),
		balance INTEGER NOT NULL DEFAULT 100000000,  -- $1,000,000 in cents
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id TEXT NOT NULL REFERENCES accounts(id),
		symbol TEXT NOT NULL,
		quantity INTEGER NOT NULL DEFAULT 0,
		avg_price INTEGER NOT NULL DEFAULT 0,  -- in cents
		realized_pnl INTEGER NOT NULL DEFAULT 0,  -- in cents
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_id, symbol)
	);

	CREATE TABLE IF NOT EXISTS trade_history (
		id TEXT PRIMARY KEY,
		account_id TEXT NOT NULL REFERENCES accounts(id),
		symbol TEXT NOT NULL,
		side TEXT NOT NULL,  -- 'buy' or 'sell'
		price INTEGER NOT NULL,
		quantity INTEGER NOT NULL,
		pnl INTEGER NOT NULL DEFAULT 0,  -- realized P&L from this trade
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts(user_id);
	CREATE INDEX IF NOT EXISTS idx_positions_account ON positions(account_id);
	CREATE INDEX IF NOT EXISTS idx_trade_history_account ON trade_history(account_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// User represents a registered user
type User struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Account represents a trading account
type Account struct {
	ID        string
	UserID    string
	Balance   int64 // in cents
	CreatedAt time.Time
}

// Position represents a user's position in a symbol
type Position struct {
	ID          int64
	AccountID   string
	Symbol      string
	Quantity    int64
	AvgPrice    int64 // in cents
	RealizedPnL int64 // in cents
	UpdatedAt   time.Time
}

// TradeRecord represents a historical trade
type TradeRecord struct {
	ID        string
	AccountID string
	Symbol    string
	Side      string
	Price     int64
	Quantity  int64
	PnL       int64 // realized P&L
	CreatedAt time.Time
}
