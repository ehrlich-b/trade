package match

import (
	"fmt"
	"sync"
	"time"

	"trade/internal/historical"
)

// Broadcaster defines the interface for broadcasting match updates
type Broadcaster interface {
	Broadcast(message interface{})
}

// Engine manages match lifecycle and coordinates with the trading system
type Engine struct {
	mu sync.RWMutex

	dataProvider *historical.DataProvider
	broadcaster  Broadcaster

	currentMatch *Match
	matchHistory []*Match

	// Config
	config     MatchConfig
	yearsBack  int  // How many years of historical data to use
	autoStart  bool // Automatically start new matches

	stopCh chan struct{}
}

// NewEngine creates a new match engine
func NewEngine(dataProvider *historical.DataProvider, broadcaster Broadcaster) *Engine {
	return &Engine{
		dataProvider: dataProvider,
		broadcaster:  broadcaster,
		config:       DefaultConfig(),
		yearsBack:    10,
		autoStart:    false,
		stopCh:       make(chan struct{}),
	}
}

// SetConfig updates the match configuration
func (e *Engine) SetConfig(config MatchConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// SetAutoStart enables or disables automatic match start
func (e *Engine) SetAutoStart(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoStart = enabled
}

// CreateMatch creates a new match and fetches historical data
func (e *Engine) CreateMatch() (*Match, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentMatch != nil && e.currentMatch.GetState() != StateComplete {
		return nil, fmt.Errorf("match already in progress")
	}

	// Create new match
	match := NewMatch(e.config)

	// Fetch random historical day
	day, err := e.dataProvider.GetRandomNormalizedDay(e.config.Symbol, e.yearsBack, e.config.TargetNAV)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical day: %w", err)
	}

	match.SetDay(day)

	// Set up callbacks
	match.OnStateChange(func(state State) {
		e.broadcastMatchState(match)
	})

	match.OnPriceTick(func(nav int64) {
		e.broadcastPriceUpdate(match, nav)
	})

	match.OnMatchEnd(func(m *Match) {
		e.onMatchEnd(m)
	})

	e.currentMatch = match

	// Broadcast lobby creation
	e.broadcastMatchState(match)

	return match, nil
}

// StartPreMatch transitions the current match to pre-match countdown
func (e *Engine) StartPreMatch() error {
	e.mu.RLock()
	match := e.currentMatch
	e.mu.RUnlock()

	if match == nil {
		return fmt.Errorf("no match created")
	}

	if err := match.TransitionToPreMatch(); err != nil {
		return err
	}

	// Start pre-match countdown timer
	go e.preMatchCountdown(match)

	return nil
}

// preMatchCountdown handles the pre-match countdown
func (e *Engine) preMatchCountdown(match *Match) {
	countdown := match.Config.PreMatchSec

	for countdown > 0 {
		e.broadcastCountdown(match, countdown)

		select {
		case <-time.After(1 * time.Second):
			countdown--
		case <-e.stopCh:
			return
		}
	}

	// Start the match
	if err := match.Start(); err != nil {
		fmt.Printf("Failed to start match: %v\n", err)
		return
	}
}

// GetCurrentMatch returns the current match
func (e *Engine) GetCurrentMatch() *Match {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentMatch
}

// JoinMatch adds a player to the current match
func (e *Engine) JoinMatch(userID, accountID string) error {
	e.mu.RLock()
	match := e.currentMatch
	e.mu.RUnlock()

	if match == nil {
		return fmt.Errorf("no match available")
	}

	if err := match.Join(userID, accountID); err != nil {
		return err
	}

	// Broadcast updated participant list
	e.broadcastMatchState(match)

	return nil
}

// LeaveMatch removes a player from the current match
func (e *Engine) LeaveMatch(userID string) {
	e.mu.RLock()
	match := e.currentMatch
	e.mu.RUnlock()

	if match == nil {
		return
	}

	match.Leave(userID)
	e.broadcastMatchState(match)
}

// SettleMatch settles the current match with final positions
func (e *Engine) SettleMatch(positionFetcher func(userID string) (cash int64, shares int64)) {
	e.mu.RLock()
	match := e.currentMatch
	e.mu.RUnlock()

	if match == nil {
		return
	}

	match.Settle(positionFetcher)
}

// onMatchEnd is called when a match completes
func (e *Engine) onMatchEnd(match *Match) {
	e.mu.Lock()
	e.matchHistory = append(e.matchHistory, match)
	e.mu.Unlock()

	// Broadcast final results
	e.broadcastMatchResults(match)

	// Auto-start next match if enabled
	if e.autoStart {
		go func() {
			time.Sleep(10 * time.Second) // Brief intermission
			if _, err := e.CreateMatch(); err != nil {
				fmt.Printf("Failed to create next match: %v\n", err)
				return
			}
			if err := e.StartPreMatch(); err != nil {
				fmt.Printf("Failed to start next match: %v\n", err)
			}
		}()
	}
}

// Stop halts the engine
func (e *Engine) Stop() {
	close(e.stopCh)
	if e.currentMatch != nil {
		e.currentMatch.Stop()
	}
}

// Broadcast helpers

func (e *Engine) broadcastMatchState(match *Match) {
	if e.broadcaster == nil {
		return
	}

	participants := match.GetParticipants()
	participantInfo := make([]map[string]interface{}, len(participants))
	for i, p := range participants {
		participantInfo[i] = map[string]interface{}{
			"user_id":         p.UserID,
			"starting_shares": p.StartingShares,
			"starting_cash":   p.StartingCash,
		}
	}

	e.broadcaster.Broadcast(map[string]interface{}{
		"type":         "match_state",
		"match_id":     match.ID,
		"state":        match.GetState().String(),
		"symbol":       match.Config.Symbol,
		"duration":     int(match.Config.Duration),
		"nav":          match.GetNAV(),
		"participants": participantInfo,
	})
}

func (e *Engine) broadcastCountdown(match *Match, seconds int) {
	if e.broadcaster == nil {
		return
	}

	e.broadcaster.Broadcast(map[string]interface{}{
		"type":      "countdown",
		"match_id":  match.ID,
		"seconds":   seconds,
	})
}

func (e *Engine) broadcastPriceUpdate(match *Match, nav int64) {
	if e.broadcaster == nil {
		return
	}

	e.broadcaster.Broadcast(map[string]interface{}{
		"type":           "price_tick",
		"match_id":       match.ID,
		"nav":            nav,
		"market_time":    match.MarketTime(),
		"remaining_sec":  int(match.RemainingTime().Seconds()),
		"progress":       match.Progress(),
	})
}

func (e *Engine) broadcastMatchResults(match *Match) {
	if e.broadcaster == nil {
		return
	}

	participants := match.GetParticipants()
	results := make([]map[string]interface{}, len(participants))
	for i, p := range participants {
		results[i] = map[string]interface{}{
			"user_id":      p.UserID,
			"rank":         p.Rank,
			"pnl":          p.PnL,
			"final_value":  p.FinalValue,
			"start_value":  p.StartingValue,
			"start_shares": p.StartingShares,
			"final_shares": p.FinalShares,
		}
	}

	e.broadcaster.Broadcast(map[string]interface{}{
		"type":       "match_results",
		"match_id":   match.ID,
		"final_nav":  match.GetNAV(),
		"results":    results,
	})
}

// GetMatchHistory returns past matches
func (e *Engine) GetMatchHistory() []*Match {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.matchHistory
}

// MatchStatus returns a summary of current match status
type MatchStatus struct {
	HasMatch       bool    `json:"has_match"`
	MatchID        string  `json:"match_id,omitempty"`
	State          string  `json:"state,omitempty"`
	Symbol         string  `json:"symbol,omitempty"`
	Duration       int     `json:"duration,omitempty"`
	NAV            int64   `json:"nav,omitempty"`
	MarketTime     string  `json:"market_time,omitempty"`
	RemainingSec   int     `json:"remaining_sec,omitempty"`
	Progress       float64 `json:"progress,omitempty"`
	Participants   int     `json:"participants,omitempty"`
}

// GetStatus returns current match status
func (e *Engine) GetStatus() MatchStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.currentMatch == nil {
		return MatchStatus{HasMatch: false}
	}

	match := e.currentMatch
	return MatchStatus{
		HasMatch:     true,
		MatchID:      match.ID,
		State:        match.GetState().String(),
		Symbol:       match.Config.Symbol,
		Duration:     int(match.Config.Duration),
		NAV:          match.GetNAV(),
		MarketTime:   match.MarketTime(),
		RemainingSec: int(match.RemainingTime().Seconds()),
		Progress:     match.Progress(),
		Participants: match.ParticipantCount(),
	}
}
