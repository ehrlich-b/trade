package store

import (
	"database/sql"
	"time"
)

// MatchRecord represents a completed match
type MatchRecord struct {
	ID               string
	Symbol           string
	DurationMinutes  int
	TargetNAV        int64
	FinalNAV         int64
	ParticipantCount int
	StartedAt        time.Time
	EndedAt          time.Time
	CreatedAt        time.Time
}

// MatchResult represents a user's result in a match
type MatchResult struct {
	ID            int64
	MatchID       string
	UserID        string
	StartingValue int64
	FinalValue    int64
	PnL           int64
	Rank          int
	StartingShares int64
	FinalShares   int64
	StartingCash  int64
	FinalCash     int64
	CreatedAt     time.Time
}

// UserStats represents aggregate stats for a user
type UserStats struct {
	UserID        string
	MatchesPlayed int
	MatchesWon    int
	TotalPnL      int64
	BestPnL       int64
	WorstPnL      int64
	CurrentStreak int
	BestStreak    int
	UpdatedAt     time.Time
}

// SaveMatch saves a completed match and all participant results
func (s *Store) SaveMatch(match MatchRecord, results []MatchResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert match record
	_, err = tx.Exec(`
		INSERT INTO matches (id, symbol, duration_minutes, target_nav, final_nav, participant_count, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, match.ID, match.Symbol, match.DurationMinutes, match.TargetNAV, match.FinalNAV,
		match.ParticipantCount, match.StartedAt, match.EndedAt)
	if err != nil {
		return err
	}

	// Insert results for each participant
	for _, r := range results {
		_, err = tx.Exec(`
			INSERT INTO match_results (match_id, user_id, starting_value, final_value, pnl, rank,
				starting_shares, final_shares, starting_cash, final_cash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, match.ID, r.UserID, r.StartingValue, r.FinalValue, r.PnL, r.Rank,
			r.StartingShares, r.FinalShares, r.StartingCash, r.FinalCash)
		if err != nil {
			return err
		}

		// Update user stats
		if err := s.updateUserStatsInTx(tx, r.UserID, r.PnL, r.Rank == 1); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// updateUserStatsInTx updates user stats within a transaction
func (s *Store) updateUserStatsInTx(tx *sql.Tx, userID string, pnl int64, won bool) error {
	// Get current stats
	var stats UserStats
	err := tx.QueryRow(`
		SELECT user_id, matches_played, matches_won, total_pnl, best_pnl, worst_pnl, current_streak, best_streak
		FROM user_stats WHERE user_id = ?
	`, userID).Scan(
		&stats.UserID, &stats.MatchesPlayed, &stats.MatchesWon,
		&stats.TotalPnL, &stats.BestPnL, &stats.WorstPnL,
		&stats.CurrentStreak, &stats.BestStreak,
	)

	if err == sql.ErrNoRows {
		// First match for this user
		stats = UserStats{
			UserID:        userID,
			MatchesPlayed: 0,
			MatchesWon:    0,
			TotalPnL:      0,
			BestPnL:       0,
			WorstPnL:      0,
			CurrentStreak: 0,
			BestStreak:    0,
		}
	} else if err != nil {
		return err
	}

	// Update stats
	stats.MatchesPlayed++
	stats.TotalPnL += pnl

	if pnl > stats.BestPnL {
		stats.BestPnL = pnl
	}
	if pnl < stats.WorstPnL || stats.MatchesPlayed == 1 {
		stats.WorstPnL = pnl
	}

	if won {
		stats.MatchesWon++
		stats.CurrentStreak++
		if stats.CurrentStreak > stats.BestStreak {
			stats.BestStreak = stats.CurrentStreak
		}
	} else {
		stats.CurrentStreak = 0
	}

	// Upsert stats
	_, err = tx.Exec(`
		INSERT INTO user_stats (user_id, matches_played, matches_won, total_pnl, best_pnl, worst_pnl, current_streak, best_streak, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			matches_played = excluded.matches_played,
			matches_won = excluded.matches_won,
			total_pnl = excluded.total_pnl,
			best_pnl = excluded.best_pnl,
			worst_pnl = excluded.worst_pnl,
			current_streak = excluded.current_streak,
			best_streak = excluded.best_streak,
			updated_at = CURRENT_TIMESTAMP
	`, stats.UserID, stats.MatchesPlayed, stats.MatchesWon, stats.TotalPnL,
		stats.BestPnL, stats.WorstPnL, stats.CurrentStreak, stats.BestStreak)
	return err
}

// GetUserStats returns stats for a user
func (s *Store) GetUserStats(userID string) (*UserStats, error) {
	var stats UserStats
	err := s.db.QueryRow(`
		SELECT user_id, matches_played, matches_won, total_pnl, best_pnl, worst_pnl, current_streak, best_streak, updated_at
		FROM user_stats WHERE user_id = ?
	`, userID).Scan(
		&stats.UserID, &stats.MatchesPlayed, &stats.MatchesWon,
		&stats.TotalPnL, &stats.BestPnL, &stats.WorstPnL,
		&stats.CurrentStreak, &stats.BestStreak, &stats.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Return empty stats for new user
		return &UserStats{UserID: userID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetUserMatchHistory returns recent matches for a user
func (s *Store) GetUserMatchHistory(userID string, limit int) ([]MatchResult, error) {
	rows, err := s.db.Query(`
		SELECT r.id, r.match_id, r.user_id, r.starting_value, r.final_value, r.pnl, r.rank,
			r.starting_shares, r.final_shares, r.starting_cash, r.final_cash, r.created_at
		FROM match_results r
		JOIN matches m ON r.match_id = m.id
		WHERE r.user_id = ?
		ORDER BY m.ended_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MatchResult
	for rows.Next() {
		var r MatchResult
		if err := rows.Scan(
			&r.ID, &r.MatchID, &r.UserID, &r.StartingValue, &r.FinalValue, &r.PnL, &r.Rank,
			&r.StartingShares, &r.FinalShares, &r.StartingCash, &r.FinalCash, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetMatch returns a match by ID
func (s *Store) GetMatch(matchID string) (*MatchRecord, error) {
	var m MatchRecord
	err := s.db.QueryRow(`
		SELECT id, symbol, duration_minutes, target_nav, final_nav, participant_count, started_at, ended_at, created_at
		FROM matches WHERE id = ?
	`, matchID).Scan(
		&m.ID, &m.Symbol, &m.DurationMinutes, &m.TargetNAV, &m.FinalNAV,
		&m.ParticipantCount, &m.StartedAt, &m.EndedAt, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetMatchResults returns all results for a match
func (s *Store) GetMatchResults(matchID string) ([]MatchResult, error) {
	rows, err := s.db.Query(`
		SELECT id, match_id, user_id, starting_value, final_value, pnl, rank,
			starting_shares, final_shares, starting_cash, final_cash, created_at
		FROM match_results
		WHERE match_id = ?
		ORDER BY rank ASC
	`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MatchResult
	for rows.Next() {
		var r MatchResult
		if err := rows.Scan(
			&r.ID, &r.MatchID, &r.UserID, &r.StartingValue, &r.FinalValue, &r.PnL, &r.Rank,
			&r.StartingShares, &r.FinalShares, &r.StartingCash, &r.FinalCash, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetRecentMatches returns recent completed matches
func (s *Store) GetRecentMatches(limit int) ([]MatchRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, symbol, duration_minutes, target_nav, final_nav, participant_count, started_at, ended_at, created_at
		FROM matches
		ORDER BY ended_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []MatchRecord
	for rows.Next() {
		var m MatchRecord
		if err := rows.Scan(
			&m.ID, &m.Symbol, &m.DurationMinutes, &m.TargetNAV, &m.FinalNAV,
			&m.ParticipantCount, &m.StartedAt, &m.EndedAt, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// GetMatchLeaderboard returns top users by total match P&L
func (s *Store) GetMatchLeaderboard(limit int) ([]UserStats, error) {
	rows, err := s.db.Query(`
		SELECT user_id, matches_played, matches_won, total_pnl, best_pnl, worst_pnl, current_streak, best_streak, updated_at
		FROM user_stats
		WHERE matches_played > 0
		ORDER BY total_pnl DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UserStats
	for rows.Next() {
		var s UserStats
		if err := rows.Scan(
			&s.UserID, &s.MatchesPlayed, &s.MatchesWon,
			&s.TotalPnL, &s.BestPnL, &s.WorstPnL,
			&s.CurrentStreak, &s.BestStreak, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}
