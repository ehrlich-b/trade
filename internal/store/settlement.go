package store

import (
	"time"
)

const StartingBalance int64 = 100000000 // $1,000,000 in cents

// DailySnapshot represents an account's state at end of day
type DailySnapshot struct {
	ID          int64
	AccountID   string
	Date        time.Time
	Balance     int64
	RealizedPnL int64
	Positions   string // JSON-encoded positions
	IsBankrupt  bool
}

// SettlementResult contains the outcome of a settlement run
type SettlementResult struct {
	AccountID   string
	FinalPnL    int64
	IsBankrupt  bool
	WasReset    bool
	PreviousPnL int64
}

// InitSettlementSchema adds settlement-related tables
func (s *Store) InitSettlementSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS daily_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id TEXT NOT NULL REFERENCES accounts(id),
		date DATE NOT NULL,
		balance INTEGER NOT NULL,
		realized_pnl INTEGER NOT NULL,
		positions TEXT,  -- JSON
		is_bankrupt BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_id, date)
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_date ON daily_snapshots(date);
	`
	_, err := s.db.Exec(schema)
	return err
}

// SettleAccount performs end-of-day settlement for a single account
// markPrice is the current market price to use for unrealized P&L
func (s *Store) SettleAccount(accountID string, symbol string, markPrice int64) (*SettlementResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get current account
	var balance int64
	err = tx.QueryRow("SELECT balance FROM accounts WHERE id = ?", accountID).Scan(&balance)
	if err != nil {
		return nil, err
	}

	// Get position and calculate unrealized P&L
	var quantity, avgPrice, realizedPnL int64
	err = tx.QueryRow(
		"SELECT quantity, avg_price, realized_pnl FROM positions WHERE account_id = ? AND symbol = ?",
		accountID, symbol,
	).Scan(&quantity, &avgPrice, &realizedPnL)
	if err != nil {
		// No position, that's OK
		quantity = 0
		avgPrice = 0
		realizedPnL = 0
	}

	// Calculate unrealized P&L
	var unrealizedPnL int64
	if quantity > 0 {
		unrealizedPnL = quantity * (markPrice - avgPrice)
	} else if quantity < 0 {
		unrealizedPnL = quantity * (markPrice - avgPrice) // Negative quantity makes this work
	}

	// Total P&L
	totalPnL := realizedPnL + unrealizedPnL

	// Final balance after mark-to-market
	finalBalance := balance + unrealizedPnL

	result := &SettlementResult{
		AccountID:   accountID,
		FinalPnL:    totalPnL,
		IsBankrupt:  finalBalance <= 0,
		WasReset:    false,
		PreviousPnL: totalPnL,
	}

	// Save snapshot
	today := time.Now().Truncate(24 * time.Hour)
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO daily_snapshots (account_id, date, balance, realized_pnl, is_bankrupt)
		 VALUES (?, ?, ?, ?, ?)`,
		accountID, today, finalBalance, totalPnL, result.IsBankrupt,
	)
	if err != nil {
		return nil, err
	}

	// If bankrupt, reset the account
	if result.IsBankrupt {
		result.WasReset = true

		// Reset balance
		_, err = tx.Exec("UPDATE accounts SET balance = ? WHERE id = ?", StartingBalance, accountID)
		if err != nil {
			return nil, err
		}

		// Clear positions
		_, err = tx.Exec("DELETE FROM positions WHERE account_id = ?", accountID)
		if err != nil {
			return nil, err
		}
	} else {
		// Just realize the unrealized P&L and flatten positions
		_, err = tx.Exec("UPDATE accounts SET balance = ? WHERE id = ?", finalBalance, accountID)
		if err != nil {
			return nil, err
		}

		// Clear positions (daily reset means everyone starts flat)
		_, err = tx.Exec("DELETE FROM positions WHERE account_id = ?", accountID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}

// SettleAllAccounts performs settlement for all accounts
func (s *Store) SettleAllAccounts(symbol string, markPrice int64) ([]*SettlementResult, error) {
	// Get all account IDs
	rows, err := s.db.Query("SELECT id FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accountIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		accountIDs = append(accountIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Settle each account
	var results []*SettlementResult
	for _, accountID := range accountIDs {
		result, err := s.SettleAccount(accountID, symbol, markPrice)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// GetLeaderboard returns top accounts by total P&L
func (s *Store) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	rows, err := s.db.Query(`
		SELECT u.username, a.balance - 100000000 as pnl
		FROM accounts a
		JOIN users u ON a.user_id = u.id
		ORDER BY pnl DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.Username, &entry.TotalPnL); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// LeaderboardEntry represents a player on the leaderboard
type LeaderboardEntry struct {
	Username string `json:"username"`
	TotalPnL int64  `json:"total_pnl"`
}

// ResetAccount manually resets an account to starting balance
func (s *Store) ResetAccount(accountID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Reset balance
	_, err = tx.Exec("UPDATE accounts SET balance = ? WHERE id = ?", StartingBalance, accountID)
	if err != nil {
		return err
	}

	// Clear positions
	_, err = tx.Exec("DELETE FROM positions WHERE account_id = ?", accountID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CheckBankruptcy returns true if account balance is at or below zero
func (s *Store) CheckBankruptcy(accountID string, symbol string, markPrice int64) (bool, int64, error) {
	var balance int64
	err := s.db.QueryRow("SELECT balance FROM accounts WHERE id = ?", accountID).Scan(&balance)
	if err != nil {
		return false, 0, err
	}

	// Get position for unrealized P&L
	var quantity, avgPrice int64
	err = s.db.QueryRow(
		"SELECT quantity, avg_price FROM positions WHERE account_id = ? AND symbol = ?",
		accountID, symbol,
	).Scan(&quantity, &avgPrice)
	if err != nil {
		// No position
		quantity = 0
		avgPrice = 0
	}

	// Calculate unrealized P&L
	var unrealizedPnL int64
	if quantity != 0 {
		unrealizedPnL = quantity * (markPrice - avgPrice)
	}

	effectiveBalance := balance + unrealizedPnL
	return effectiveBalance <= 0, effectiveBalance, nil
}
