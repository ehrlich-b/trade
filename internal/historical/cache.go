package historical

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Cache stores historical trading days locally to avoid repeated API calls
type Cache struct {
	db *sql.DB
}

// NewCache creates a new historical data cache
func NewCache(dbPath string) (*Cache, error) {
	if dbPath == ":memory:" {
		dbPath = "file::memory:?cache=shared"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Configure SQLite
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

	c := &Cache{db: db}
	if err := c.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return c, nil
}

// migrate creates the necessary tables
func (c *Cache) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS trading_days (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL,
		date TEXT NOT NULL,
		bars_json TEXT NOT NULL,
		total_volume INTEGER NOT NULL,
		day_open INTEGER NOT NULL,
		day_close INTEGER NOT NULL,
		day_high INTEGER NOT NULL,
		day_low INTEGER NOT NULL,
		fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(symbol, date)
	);

	CREATE INDEX IF NOT EXISTS idx_trading_days_symbol_date ON trading_days(symbol, date);
	`
	_, err := c.db.Exec(schema)
	return err
}

// Close closes the database connection
func (c *Cache) Close() error {
	return c.db.Close()
}

// Get retrieves a cached trading day
func (c *Cache) Get(symbol string, date time.Time) (*TradingDay, error) {
	dateStr := date.Format("2006-01-02")

	var barsJSON string
	err := c.db.QueryRow(
		"SELECT bars_json FROM trading_days WHERE symbol = ? AND date = ?",
		symbol, dateStr,
	).Scan(&barsJSON)

	if err == sql.ErrNoRows {
		return nil, nil // Not cached
	}
	if err != nil {
		return nil, err
	}

	var bars []MinuteBar
	if err := json.Unmarshal([]byte(barsJSON), &bars); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached bars: %w", err)
	}

	return &TradingDay{
		Symbol: symbol,
		Date:   date,
		Bars:   bars,
	}, nil
}

// Put stores a trading day in the cache
func (c *Cache) Put(day *TradingDay) error {
	barsJSON, err := json.Marshal(day.Bars)
	if err != nil {
		return fmt.Errorf("failed to marshal bars: %w", err)
	}

	dateStr := day.Date.Format("2006-01-02")

	_, err = c.db.Exec(`
		INSERT OR REPLACE INTO trading_days
		(symbol, date, bars_json, total_volume, day_open, day_close, day_high, day_low)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		day.Symbol, dateStr, string(barsJSON),
		day.TotalVolume(), day.Open(), day.Close(), day.High(), day.Low(),
	)
	return err
}

// ListCachedDates returns all cached dates for a symbol
func (c *Cache) ListCachedDates(symbol string) ([]time.Time, error) {
	rows, err := c.db.Query(
		"SELECT date FROM trading_days WHERE symbol = ? ORDER BY date",
		symbol,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			return nil, err
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		dates = append(dates, date)
	}

	return dates, rows.Err()
}

// CachedDayCount returns the number of cached days for a symbol
func (c *Cache) CachedDayCount(symbol string) (int, error) {
	var count int
	err := c.db.QueryRow(
		"SELECT COUNT(*) FROM trading_days WHERE symbol = ?",
		symbol,
	).Scan(&count)
	return count, err
}

// GetRandomCachedDay returns a random cached day for a symbol
func (c *Cache) GetRandomCachedDay(symbol string) (*TradingDay, error) {
	var barsJSON string
	var dateStr string

	err := c.db.QueryRow(`
		SELECT date, bars_json FROM trading_days
		WHERE symbol = ?
		ORDER BY RANDOM()
		LIMIT 1`,
		symbol,
	).Scan(&dateStr, &barsJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var bars []MinuteBar
	if err := json.Unmarshal([]byte(barsJSON), &bars); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached bars: %w", err)
	}

	date, _ := time.Parse("2006-01-02", dateStr)

	return &TradingDay{
		Symbol: symbol,
		Date:   date,
		Bars:   bars,
	}, nil
}
