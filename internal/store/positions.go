package store

import (
	"database/sql"
	"errors"
	"time"
)

var (
	ErrInsufficientMargin = errors.New("insufficient margin for this order")
)

// GetPosition retrieves the position for an account and symbol
func (s *Store) GetPosition(accountID, symbol string) (*Position, error) {
	pos := &Position{}
	err := s.db.QueryRow(
		"SELECT id, account_id, symbol, quantity, avg_price, realized_pnl, updated_at FROM positions WHERE account_id = ? AND symbol = ?",
		accountID, symbol,
	).Scan(&pos.ID, &pos.AccountID, &pos.Symbol, &pos.Quantity, &pos.AvgPrice, &pos.RealizedPnL, &pos.UpdatedAt)
	if err == sql.ErrNoRows {
		// Return empty position
		return &Position{
			AccountID: accountID,
			Symbol:    symbol,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return pos, nil
}

// CheckMarginForOrder validates if an account has sufficient margin for an order.
// Returns nil if the order is allowed, ErrInsufficientMargin if not.
//
// Margin rules:
// - Each share of position (long or short) requires margin equal to its value
// - Available margin = balance - sum(|position_qty| * estimated_price)
// - Order requires margin for the worst-case resulting position
func (s *Store) CheckMarginForOrder(accountID, symbol string, side string, quantity int64, estimatedPrice int64) error {
	// Get current account balance
	var balance int64
	err := s.db.QueryRow("SELECT balance FROM accounts WHERE id = ?", accountID).Scan(&balance)
	if err != nil {
		return err
	}

	// Get current position
	pos, err := s.GetPosition(accountID, symbol)
	if err != nil {
		return err
	}

	// Calculate new position after order
	var newQty int64
	if side == "buy" {
		newQty = pos.Quantity + quantity
	} else {
		newQty = pos.Quantity - quantity
	}

	// Calculate margin required for new position
	// Use absolute value since both long and short positions require margin
	var newPositionValue int64
	if newQty > 0 {
		newPositionValue = newQty * estimatedPrice
	} else {
		newPositionValue = (-newQty) * estimatedPrice
	}

	// Check if new position value exceeds balance
	// This prevents positions larger than your account can cover
	if newPositionValue > balance {
		return ErrInsufficientMargin
	}

	return nil
}

// UpdatePositionOnTrade updates position and P&L when a trade occurs
// Returns the realized P&L from this trade
func (s *Store) UpdatePositionOnTrade(accountID, symbol string, side string, price, quantity int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Get current position
	pos := &Position{}
	err = tx.QueryRow(
		"SELECT id, quantity, avg_price, realized_pnl FROM positions WHERE account_id = ? AND symbol = ?",
		accountID, symbol,
	).Scan(&pos.ID, &pos.Quantity, &pos.AvgPrice, &pos.RealizedPnL)
	if err == sql.ErrNoRows {
		pos = &Position{AccountID: accountID, Symbol: symbol}
	} else if err != nil {
		return 0, err
	}

	var realizedPnL int64
	var newQty, newAvgPrice int64

	if side == "buy" {
		if pos.Quantity >= 0 {
			// Adding to long or opening long
			totalCost := pos.AvgPrice*pos.Quantity + price*quantity
			newQty = pos.Quantity + quantity
			if newQty > 0 {
				newAvgPrice = totalCost / newQty
			}
		} else {
			// Covering short position
			coverQty := min(quantity, -pos.Quantity)
			realizedPnL = coverQty * (pos.AvgPrice - price) // Profit if price dropped

			remaining := quantity - coverQty
			newQty = pos.Quantity + quantity

			if newQty > 0 {
				// Went from short to long
				newAvgPrice = price
			} else if newQty < 0 {
				// Still short
				newAvgPrice = pos.AvgPrice
			} else {
				// Flat
				newAvgPrice = 0
			}

			if remaining > 0 && newQty > 0 {
				// The remaining buys establish new long position at trade price
				newAvgPrice = price
			}
		}
	} else { // sell
		if pos.Quantity <= 0 {
			// Adding to short or opening short
			totalValue := pos.AvgPrice*(-pos.Quantity) + price*quantity
			newQty = pos.Quantity - quantity
			if newQty < 0 {
				newAvgPrice = totalValue / (-newQty)
			}
		} else {
			// Closing long position
			sellQty := min(quantity, pos.Quantity)
			realizedPnL = sellQty * (price - pos.AvgPrice) // Profit if price rose

			remaining := quantity - sellQty
			newQty = pos.Quantity - quantity

			if newQty < 0 {
				// Went from long to short
				newAvgPrice = price
			} else if newQty > 0 {
				// Still long
				newAvgPrice = pos.AvgPrice
			} else {
				// Flat
				newAvgPrice = 0
			}

			if remaining > 0 && newQty < 0 {
				// The remaining sells establish new short position at trade price
				newAvgPrice = price
			}
		}
	}

	// Upsert position
	if pos.ID == 0 {
		_, err = tx.Exec(
			"INSERT INTO positions (account_id, symbol, quantity, avg_price, realized_pnl, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			accountID, symbol, newQty, newAvgPrice, realizedPnL, time.Now(),
		)
	} else {
		_, err = tx.Exec(
			"UPDATE positions SET quantity = ?, avg_price = ?, realized_pnl = realized_pnl + ?, updated_at = ? WHERE id = ?",
			newQty, newAvgPrice, realizedPnL, time.Now(), pos.ID,
		)
	}
	if err != nil {
		return 0, err
	}

	// Record trade history
	tradeID, _ := generateID()
	_, err = tx.Exec(
		"INSERT INTO trade_history (id, account_id, symbol, side, price, quantity, pnl, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		tradeID, accountID, symbol, side, price, quantity, realizedPnL, time.Now(),
	)
	if err != nil {
		return 0, err
	}

	// Update account balance with realized P&L
	_, err = tx.Exec(
		"UPDATE accounts SET balance = balance + ? WHERE id = (SELECT id FROM accounts WHERE user_id = (SELECT user_id FROM accounts WHERE id = ?) LIMIT 1)",
		realizedPnL, accountID,
	)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return realizedPnL, nil
}

// GetAllPositions returns all positions for an account
func (s *Store) GetAllPositions(accountID string) ([]*Position, error) {
	rows, err := s.db.Query(
		"SELECT id, account_id, symbol, quantity, avg_price, realized_pnl, updated_at FROM positions WHERE account_id = ?",
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*Position
	for rows.Next() {
		pos := &Position{}
		if err := rows.Scan(&pos.ID, &pos.AccountID, &pos.Symbol, &pos.Quantity, &pos.AvgPrice, &pos.RealizedPnL, &pos.UpdatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}
	return positions, rows.Err()
}

// GetTradeHistory returns recent trades for an account
func (s *Store) GetTradeHistory(accountID string, limit int) ([]*TradeRecord, error) {
	rows, err := s.db.Query(
		"SELECT id, account_id, symbol, side, price, quantity, pnl, created_at FROM trade_history WHERE account_id = ? ORDER BY created_at DESC LIMIT ?",
		accountID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*TradeRecord
	for rows.Next() {
		tr := &TradeRecord{}
		if err := rows.Scan(&tr.ID, &tr.AccountID, &tr.Symbol, &tr.Side, &tr.Price, &tr.Quantity, &tr.PnL, &tr.CreatedAt); err != nil {
			return nil, err
		}
		trades = append(trades, tr)
	}
	return trades, rows.Err()
}
