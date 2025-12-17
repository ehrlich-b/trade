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
	// For in-memory databases, use shared cache mode so multiple connections
	// can access the same database. This is required for concurrent access.
	if dbPath == ":memory:" {
		dbPath = "file::memory:?cache=shared"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Configure SQLite for concurrent access
	// WAL mode allows concurrent readers while writing
	// busy_timeout makes writers wait instead of failing immediately
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}

	s := &Store{db: db}
	if err := s.Migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
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
