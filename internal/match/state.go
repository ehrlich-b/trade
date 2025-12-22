package match

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"trade/internal/historical"
)

// State represents the current state of a match
type State int

const (
	StateLobby      State = iota // Waiting for players, spinning for day
	StatePreMatch                // Day selected, 30 second countdown
	StateTrading                 // Active trading
	StateSettlement              // Match ended, calculating results
	StateComplete                // Results finalized
)

func (s State) String() string {
	switch s {
	case StateLobby:
		return "LOBBY"
	case StatePreMatch:
		return "PRE_MATCH"
	case StateTrading:
		return "TRADING"
	case StateSettlement:
		return "SETTLEMENT"
	case StateComplete:
		return "COMPLETE"
	default:
		return "UNKNOWN"
	}
}

// Participant represents a player in the match
type Participant struct {
	UserID        string
	AccountID     string
	StartingCash  int64 // Starting cash in cents
	StartingShares int64 // Starting shares
	StartingValue int64 // Total starting value (cash + shares * NAV)
	FinalCash     int64 // Cash at settlement
	FinalShares   int64 // Shares at settlement
	FinalValue    int64 // Total value at settlement
	PnL           int64 // Profit/Loss in cents
	Rank          int   // Final ranking
}

// MatchConfig contains configuration for a match
type MatchConfig struct {
	Duration    historical.MatchDuration // 10, 15, or 30 minutes
	Symbol      string                   // Symbol to trade (e.g., "SPY")
	TargetNAV   int64                    // Target opening NAV in cents (e.g., 48000 for $480)
	PreMatchSec int                      // Pre-match countdown in seconds
	StartValue  int64                    // Starting account value in cents ($1M = 100000000)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() MatchConfig {
	return MatchConfig{
		Duration:    historical.Match15Min,
		Symbol:      "SPY",
		TargetNAV:   48000,     // $480.00
		PreMatchSec: 30,
		StartValue:  100000000, // $1,000,000
	}
}

// Match represents a single trading match
type Match struct {
	mu sync.RWMutex

	ID           string
	Config       MatchConfig
	State        State
	CreatedAt    time.Time
	StartedAt    time.Time // When trading began
	EndedAt      time.Time // When trading ended

	// Historical data
	Day        *historical.NormalizedDay
	TimeScaler *historical.TimeScaler

	// Price state
	CurrentNAV   int64 // Current NAV (moves with historical data)
	CurrentBar   int   // Current bar index

	// Participants
	Participants map[string]*Participant

	// Callbacks
	onStateChange func(State)
	onPriceTick   func(int64)
	onMatchEnd    func(*Match)

	// Control
	stopCh chan struct{}
	rng    *rand.Rand
}

// NewMatch creates a new match in lobby state
func NewMatch(config MatchConfig) *Match {
	return &Match{
		ID:           generateMatchID(),
		Config:       config,
		State:        StateLobby,
		CreatedAt:    time.Now(),
		Participants: make(map[string]*Participant),
		stopCh:       make(chan struct{}),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetDay sets the historical day for this match
func (m *Match) SetDay(day *historical.NormalizedDay) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Day = day
	m.CurrentNAV = day.Open()
}

// Join adds a participant to the match
func (m *Match) Join(userID, accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.State != StateLobby && m.State != StatePreMatch {
		return fmt.Errorf("cannot join match in state %s", m.State)
	}

	if _, exists := m.Participants[userID]; exists {
		return nil // Already joined
	}

	// Generate random starting position (20-80% in shares)
	sharePct := 0.20 + m.rng.Float64()*0.60
	shareValue := int64(float64(m.Config.StartValue) * sharePct)
	shares := shareValue / m.Config.TargetNAV
	cash := m.Config.StartValue - (shares * m.Config.TargetNAV)

	m.Participants[userID] = &Participant{
		UserID:         userID,
		AccountID:      accountID,
		StartingCash:   cash,
		StartingShares: shares,
		StartingValue:  m.Config.StartValue,
	}

	return nil
}

// Leave removes a participant from the match
func (m *Match) Leave(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.State != StateLobby {
		return // Can't leave once match has started
	}

	delete(m.Participants, userID)
}

// TransitionToPreMatch moves from lobby to pre-match countdown
func (m *Match) TransitionToPreMatch() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.State != StateLobby {
		return fmt.Errorf("can only transition to pre-match from lobby")
	}

	if m.Day == nil {
		return fmt.Errorf("historical day not set")
	}

	m.State = StatePreMatch
	m.notifyStateChange()

	return nil
}

// Start begins the trading phase
func (m *Match) Start() error {
	m.mu.Lock()

	if m.State != StatePreMatch {
		m.mu.Unlock()
		return fmt.Errorf("can only start from pre-match state")
	}

	m.State = StateTrading
	m.StartedAt = time.Now()
	m.TimeScaler = historical.NewTimeScaler(m.Config.Duration)
	m.TimeScaler.Start()
	m.CurrentBar = 0

	m.mu.Unlock() // Release lock before callback to avoid deadlock

	m.notifyStateChange()

	// Start the price tick loop
	go m.priceTickLoop()

	return nil
}

// priceTickLoop advances the price according to time scaling
func (m *Match) priceTickLoop() {
	// Tick every 100ms for smooth updates
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.updatePrice() {
				return // Match complete
			}
		case <-m.stopCh:
			return
		}
	}
}

// updatePrice updates the current price based on elapsed time
// Returns true if match is complete
func (m *Match) updatePrice() bool {
	m.mu.Lock()

	if m.State != StateTrading {
		m.mu.Unlock()
		return true
	}

	if m.TimeScaler.IsComplete() {
		m.endTrading()
		m.mu.Unlock()
		return true
	}

	// Get current bar index
	barIndex := m.TimeScaler.CurrentBarIndex()
	var navToNotify int64
	var shouldNotify bool

	if barIndex != m.CurrentBar && barIndex < len(m.Day.Bars) {
		m.CurrentBar = barIndex
		bar := m.Day.Bars[barIndex]
		m.CurrentNAV = m.TimeScaler.InterpolatePrice(bar)

		if m.onPriceTick != nil {
			navToNotify = m.CurrentNAV
			shouldNotify = true
		}
	}

	// Release lock BEFORE calling callback to avoid deadlock
	cb := m.onPriceTick
	m.mu.Unlock()

	if shouldNotify && cb != nil {
		cb(navToNotify)
	}

	return false
}

// endTrading ends the trading phase and begins settlement
func (m *Match) endTrading() {
	m.State = StateSettlement
	m.EndedAt = time.Now()
	m.notifyStateChange()

	// Set final NAV to last bar's close
	if len(m.Day.Bars) > 0 {
		m.CurrentNAV = m.Day.Bars[len(m.Day.Bars)-1].Close
	}

	// Calculate settlement (caller must provide final positions)
	m.notifyStateChange()
}

// Settle calculates final P&L for all participants
// positionFetcher should return (cash, shares) for a given userID
func (m *Match) Settle(positionFetcher func(userID string) (cash int64, shares int64)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.State != StateSettlement {
		return
	}

	// Calculate final values for each participant
	for _, p := range m.Participants {
		cash, shares := positionFetcher(p.UserID)
		p.FinalCash = cash
		p.FinalShares = shares
		p.FinalValue = cash + (shares * m.CurrentNAV)
		p.PnL = p.FinalValue - p.StartingValue
	}

	// Rank participants by P&L
	m.rankParticipants()

	m.State = StateComplete
	m.notifyStateChange()

	if m.onMatchEnd != nil {
		m.onMatchEnd(m)
	}
}

// rankParticipants assigns ranks based on P&L
func (m *Match) rankParticipants() {
	// Collect into slice for sorting
	participants := make([]*Participant, 0, len(m.Participants))
	for _, p := range m.Participants {
		participants = append(participants, p)
	}

	// Sort by P&L descending (simple bubble sort for small N)
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			if participants[j].PnL > participants[i].PnL {
				participants[i], participants[j] = participants[j], participants[i]
			}
		}
	}

	// Assign ranks
	for i, p := range participants {
		p.Rank = i + 1
	}
}

// Stop halts the match
func (m *Match) Stop() {
	close(m.stopCh)
}

// Getters

func (m *Match) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.State
}

func (m *Match) GetNAV() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CurrentNAV
}

func (m *Match) GetParticipant(userID string) *Participant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Participants[userID]
}

func (m *Match) GetParticipants() []*Participant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Participant, 0, len(m.Participants))
	for _, p := range m.Participants {
		result = append(result, p)
	}
	return result
}

func (m *Match) ParticipantCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.Participants)
}

func (m *Match) RemainingTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.TimeScaler == nil {
		return 0
	}
	return m.TimeScaler.RemainingReal()
}

func (m *Match) MarketTime() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.TimeScaler == nil {
		return "9:30 AM"
	}
	return m.TimeScaler.MarketTimeString()
}

func (m *Match) Progress() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.TimeScaler == nil {
		return 0
	}
	return m.TimeScaler.Progress()
}

// BarData represents a single OHLCV bar for the frontend
type BarData struct {
	Time   string `json:"time"`   // Market time string
	Open   int64  `json:"open"`   // Price in cents
	High   int64  `json:"high"`
	Low    int64  `json:"low"`
	Close  int64  `json:"close"`
	Volume int64  `json:"volume"`
}

// GetRevealedBars returns the bars that have been "played" so far
func (m *Match) GetRevealedBars() []BarData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Day == nil || m.CurrentBar <= 0 {
		return nil
	}

	// Return bars up to and including current bar
	endIdx := m.CurrentBar + 1
	if endIdx > len(m.Day.Bars) {
		endIdx = len(m.Day.Bars)
	}

	bars := make([]BarData, endIdx)
	for i := 0; i < endIdx; i++ {
		bar := m.Day.Bars[i]
		// Calculate market time for this bar
		hour := 9 + (30+i)/60
		min := (30 + i) % 60
		timeStr := fmt.Sprintf("%d:%02d", hour, min)
		if hour >= 12 {
			if hour > 12 {
				hour -= 12
			}
			timeStr = fmt.Sprintf("%d:%02d PM", hour, min)
		} else {
			timeStr = fmt.Sprintf("%d:%02d AM", hour, min)
		}

		bars[i] = BarData{
			Time:   timeStr,
			Open:   bar.Open,
			High:   bar.High,
			Low:    bar.Low,
			Close:  bar.Close,
			Volume: bar.Volume,
		}
	}

	return bars
}

// GetCurrentBar returns the current bar index
func (m *Match) GetCurrentBar() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CurrentBar
}

// Callbacks

func (m *Match) OnStateChange(fn func(State)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = fn
}

func (m *Match) OnPriceTick(fn func(int64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPriceTick = fn
}

func (m *Match) OnMatchEnd(fn func(*Match)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMatchEnd = fn
}

func (m *Match) notifyStateChange() {
	if m.onStateChange != nil {
		m.onStateChange(m.State)
	}
}

// generateMatchID generates a unique match ID
func generateMatchID() string {
	return fmt.Sprintf("match_%d", time.Now().UnixNano())
}
