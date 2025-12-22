package match

import (
	"testing"
	"time"

	"trade/internal/historical"
)

func TestMatchStateTransitions(t *testing.T) {
	config := DefaultConfig()
	match := NewMatch(config)

	// Should start in lobby
	if match.GetState() != StateLobby {
		t.Errorf("expected StateLobby, got %s", match.GetState())
	}

	// Create a fake day
	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{
			Open:  48000 + int64(i),
			High:  48100 + int64(i),
			Low:   47900 + int64(i),
			Close: 48050 + int64(i),
		}
	}
	day := &historical.TradingDay{
		Symbol: "SPY",
		Date:   time.Now(),
		Bars:   bars,
	}
	normalizedDay := day.Normalize(48000)
	match.SetDay(normalizedDay)

	// Can't start without going through pre-match
	if err := match.Start(); err == nil {
		t.Error("expected error starting from lobby")
	}

	// Transition to pre-match
	if err := match.TransitionToPreMatch(); err != nil {
		t.Fatalf("failed to transition to pre-match: %v", err)
	}
	if match.GetState() != StatePreMatch {
		t.Errorf("expected StatePreMatch, got %s", match.GetState())
	}

	// Now can start
	if err := match.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if match.GetState() != StateTrading {
		t.Errorf("expected StateTrading, got %s", match.GetState())
	}
}

func TestParticipantJoin(t *testing.T) {
	config := DefaultConfig()
	config.TargetNAV = 10000 // $100 for easier math
	config.StartValue = 100000 // $1000 for easier math

	match := NewMatch(config)

	// Join in lobby
	if err := match.Join("user1", "acc1"); err != nil {
		t.Fatalf("failed to join: %v", err)
	}

	p := match.GetParticipant("user1")
	if p == nil {
		t.Fatal("participant not found")
	}

	// Check starting position
	if p.StartingValue != config.StartValue {
		t.Errorf("expected StartingValue=%d, got %d", config.StartValue, p.StartingValue)
	}

	// Starting shares should be between 20% and 80% of value
	minShares := int64(float64(config.StartValue) * 0.20 / float64(config.TargetNAV))
	maxShares := int64(float64(config.StartValue) * 0.80 / float64(config.TargetNAV))

	if p.StartingShares < minShares || p.StartingShares > maxShares {
		t.Errorf("starting shares %d outside expected range [%d, %d]", p.StartingShares, minShares, maxShares)
	}

	// Cash + shares value should equal starting value
	totalValue := p.StartingCash + (p.StartingShares * config.TargetNAV)
	if totalValue != config.StartValue {
		t.Errorf("total value %d != starting value %d", totalValue, config.StartValue)
	}
}

func TestMatchSettlement(t *testing.T) {
	config := DefaultConfig()
	config.TargetNAV = 10000 // $100
	config.StartValue = 100000 // $1000

	match := NewMatch(config)

	// Add participants
	match.Join("user1", "acc1")
	match.Join("user2", "acc2")

	// Create a day
	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{
			Open:  10000,
			High:  10100,
			Low:   9900,
			Close: 10000,
		}
	}
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}
	match.SetDay(day.Normalize(10000))

	// Transition to pre-match
	match.TransitionToPreMatch()

	// Start
	match.Start()

	// Simulate time passing
	match.mu.Lock()
	match.State = StateSettlement // Force to settlement for testing
	match.CurrentNAV = 11000 // Price went up 10%
	match.mu.Unlock()

	// Position fetcher - user1 sold all shares, user2 bought more
	positionFetcher := func(userID string) (cash int64, shares int64) {
		if userID == "user1" {
			// Started ~50% shares (~5 shares), sold all
			return 150000, 0 // $1500 cash
		}
		// user2 bought more
		return 0, 15 // 15 shares @ $110 = $1650 value
	}

	match.Settle(positionFetcher)

	// Check rankings
	p1 := match.GetParticipant("user1")
	p2 := match.GetParticipant("user2")

	if p1 == nil || p2 == nil {
		t.Fatal("participants not found after settlement")
	}

	// user1 has $1500 value
	if p1.FinalValue != 150000 {
		t.Errorf("user1 final value: expected 150000, got %d", p1.FinalValue)
	}

	// user2 has 15 * $110 = $1650 value
	if p2.FinalValue != 165000 {
		t.Errorf("user2 final value: expected 165000, got %d", p2.FinalValue)
	}

	// user2 should be ranked higher (more profit)
	if p2.Rank != 1 {
		t.Errorf("expected user2 rank=1, got %d", p2.Rank)
	}
	if p1.Rank != 2 {
		t.Errorf("expected user1 rank=2, got %d", p1.Rank)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateLobby, "LOBBY"},
		{StatePreMatch, "PRE_MATCH"},
		{StateTrading, "TRADING"},
		{StateSettlement, "SETTLEMENT"},
		{StateComplete, "COMPLETE"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %s, want %s", tt.state, got, tt.expected)
		}
	}
}

// MockBroadcaster for testing
type MockBroadcaster struct {
	messages []interface{}
}

func (m *MockBroadcaster) Broadcast(message interface{}) {
	m.messages = append(m.messages, message)
}

func TestEngine(t *testing.T) {
	// Create a data provider with in-memory cache
	dp, err := historical.NewDataProvider("", ":memory:")
	if err != nil {
		t.Fatalf("failed to create data provider: %v", err)
	}
	defer dp.Close()

	// Pre-populate cache with a test day
	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{
			Open:      48000,
			High:      48100,
			Low:       47900,
			Close:     48050,
			Volume:    1000,
			Timestamp: time.Now(),
		}
	}
	testDay := &historical.TradingDay{
		Symbol: "SPY",
		Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Bars:   bars,
	}

	// Put directly in cache
	cache, _ := historical.NewCache(":memory:")
	cache.Put(testDay)

	// Create a new data provider with this cache
	// (In real usage, the cache would be shared, but for testing we'll use the provider's own cache)
	// For this test, we'll skip actual data fetching and test engine logic

	broadcaster := &MockBroadcaster{}
	engine := NewEngine(dp, broadcaster)

	status := engine.GetStatus()
	if status.HasMatch {
		t.Error("expected no match initially")
	}

	// Note: CreateMatch will fail without cached data or API key
	// This is expected behavior
}
