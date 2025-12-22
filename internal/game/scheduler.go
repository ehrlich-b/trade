package game

import (
	"log"
	"sync"
	"time"

	"trade/internal/bots"
	"trade/internal/historical"
	"trade/internal/match"
	"trade/internal/orderbook"
	"trade/internal/store"
)

// Scheduler manages the match lifecycle and automatic rotation
type Scheduler struct {
	mu sync.RWMutex

	store        *store.Store
	dataProvider *historical.DataProvider

	currentMatch  *match.Match
	priceFeed     *match.PriceFeed
	botManager    *bots.BotManager
	orderBook     *orderbook.OrderBook
	redemption    *match.RedemptionEngine

	// Configuration
	defaultDuration historical.MatchDuration
	preMatchSec     int
	intermissionSec int

	// State
	running bool
	stopCh  chan struct{}

	// Callbacks
	onMatchStart func(*match.Match)
	onMatchEnd   func(*match.Match, []store.MatchResult)
}

// SchedulerConfig configures the match scheduler
type SchedulerConfig struct {
	DefaultDuration historical.MatchDuration
	PreMatchSec     int
	IntermissionSec int
}

// DefaultSchedulerConfig returns sensible defaults
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		DefaultDuration: historical.Match10Min,
		PreMatchSec:     30,
		IntermissionSec: 10,
	}
}

// NewScheduler creates a new match scheduler
func NewScheduler(st *store.Store, dataProvider *historical.DataProvider, config SchedulerConfig) *Scheduler {
	return &Scheduler{
		store:           st,
		dataProvider:    dataProvider,
		defaultDuration: config.DefaultDuration,
		preMatchSec:     config.PreMatchSec,
		intermissionSec: config.IntermissionSec,
		stopCh:          make(chan struct{}),
	}
}

// Start begins the scheduler, creating the first match
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	// Create initial match
	if err := s.createNewMatch(); err != nil {
		return err
	}

	return nil
}

// Stop halts the scheduler and current match
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)

	// Stop current match components
	if s.botManager != nil {
		s.botManager.StopAll()
	}
	if s.priceFeed != nil {
		s.priceFeed.Stop()
	}
	if s.currentMatch != nil {
		s.currentMatch.Stop()
	}
	s.mu.Unlock()
}

// createNewMatch creates a new match and starts the lobby
func (s *Scheduler) createNewMatch() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create new order book
	s.orderBook = orderbook.New("SPY")

	// Fetch random historical day
	var day *historical.TradingDay
	var err error
	if s.dataProvider != nil {
		day, err = s.dataProvider.GetRandomDay("SPY", 10) // Random day from last 10 years
		if err != nil {
			log.Printf("Failed to get historical day: %v", err)
		}
	}
	if day == nil {
		// Use synthetic data as fallback
		day = s.createSyntheticDay()
	}

	// Create match config
	config := match.MatchConfig{
		Duration:    s.defaultDuration,
		Symbol:      "SPY",
		TargetNAV:   48000, // $480.00
		PreMatchSec: s.preMatchSec,
		StartValue:  100000000, // $1,000,000
	}

	// Create match
	s.currentMatch = match.NewMatch(config)
	s.currentMatch.SetDay(day.Normalize(config.TargetNAV))

	// Create price feed
	s.priceFeed = match.NewPriceFeed(s.currentMatch, s.orderBook)

	// Create redemption engine
	s.redemption = match.NewRedemptionEngine(int(s.defaultDuration))

	// Create bot ecosystem
	matchDuration := time.Duration(s.defaultDuration) * time.Minute
	s.botManager = bots.CreateEcosystem(s.orderBook, s.priceFeed, matchDuration)

	// Set up match callbacks
	s.currentMatch.OnStateChange(func(state match.State) {
		s.handleStateChange(state)
	})

	s.currentMatch.OnMatchEnd(func(m *match.Match) {
		s.handleMatchEnd(m)
	})

	log.Printf("Created new match %s in LOBBY state", s.currentMatch.ID)

	return nil
}

// handleStateChange responds to match state transitions
func (s *Scheduler) handleStateChange(state match.State) {
	log.Printf("Match state changed to: %s", state)

	switch state {
	case match.StateTrading:
		// Start price feed and bots
		s.mu.RLock()
		pf := s.priceFeed
		bm := s.botManager
		s.mu.RUnlock()

		log.Printf("[Scheduler] StateTrading: priceFeed=%v, botManager=%v", pf != nil, bm != nil)

		if pf != nil {
			log.Printf("[Scheduler] Calling pf.Start()...")
			pf.Start()
			log.Printf("[Scheduler] Started price feed")
		} else {
			log.Printf("[Scheduler] WARNING: priceFeed is nil!")
		}
		if bm != nil {
			log.Printf("[Scheduler] Calling bm.StartAll()...")
			bm.StartAll()
			log.Printf("[Scheduler] Started bot manager")
		} else {
			log.Printf("[Scheduler] WARNING: botManager is nil!")
		}
		log.Printf("[Scheduler] StateTrading handlers complete")

		// Notify callback
		if s.onMatchStart != nil {
			s.onMatchStart(s.currentMatch)
		}

	case match.StateSettlement:
		// Stop bots and price feed
		s.mu.RLock()
		bm := s.botManager
		pf := s.priceFeed
		s.mu.RUnlock()

		if bm != nil {
			bm.StopAll()
		}
		if pf != nil {
			pf.Stop()
		}
	}
}

// handleMatchEnd processes match completion and schedules next match
func (s *Scheduler) handleMatchEnd(m *match.Match) {
	log.Printf("Match %s ended", m.ID)

	// Collect results
	results := s.collectResults(m)

	// Save to store
	if s.store != nil {
		matchRecord := store.MatchRecord{
			ID:               m.ID,
			Symbol:           m.Config.Symbol,
			DurationMinutes:  int(m.Config.Duration),
			TargetNAV:        m.Config.TargetNAV,
			FinalNAV:         m.CurrentNAV,
			ParticipantCount: len(m.Participants),
			StartedAt:        m.StartedAt,
			EndedAt:          m.EndedAt,
		}

		if err := s.store.SaveMatch(matchRecord, results); err != nil {
			log.Printf("Failed to save match: %v", err)
		}
	}

	// Notify callback
	if s.onMatchEnd != nil {
		s.onMatchEnd(m, results)
	}

	// Schedule next match after intermission
	s.mu.RLock()
	running := s.running
	intermission := s.intermissionSec
	s.mu.RUnlock()

	if running {
		go func() {
			select {
			case <-time.After(time.Duration(intermission) * time.Second):
				if err := s.createNewMatch(); err != nil {
					log.Printf("Failed to create new match: %v", err)
				}
			case <-s.stopCh:
				return
			}
		}()
	}
}

// collectResults gathers final results for all participants
func (s *Scheduler) collectResults(m *match.Match) []store.MatchResult {
	var results []store.MatchResult

	for _, p := range m.Participants {
		results = append(results, store.MatchResult{
			MatchID:        m.ID,
			UserID:         p.UserID,
			StartingValue:  p.StartingValue,
			FinalValue:     p.FinalValue,
			PnL:            p.PnL,
			Rank:           p.Rank,
			StartingShares: p.StartingShares,
			FinalShares:    p.FinalShares,
			StartingCash:   p.StartingCash,
			FinalCash:      p.FinalCash,
		})
	}

	return results
}

// createSyntheticDay creates an exciting synthetic trading day
func (s *Scheduler) createSyntheticDay() *historical.TradingDay {
	generator := historical.NewSyntheticGenerator()
	day := generator.GenerateRandomDay(48000) // $480 base price
	log.Printf("Generated synthetic %s day with %.1f%% volatility",
		getDayTypeName(day), estimateVolatility(day))
	return day
}

// getDayTypeName returns a human-readable name for logging
func getDayTypeName(day *historical.TradingDay) string {
	if len(day.Bars) == 0 {
		return "empty"
	}

	open := day.Bars[0].Open
	close := day.Bars[len(day.Bars)-1].Close
	high := day.High()
	low := day.Low()

	pctChange := float64(close-open) / float64(open) * 100
	pctHigh := float64(high-open) / float64(open) * 100
	pctLow := float64(open-low) / float64(open) * 100

	// Find when the extreme occurred
	var highBar, lowBar int
	for i, bar := range day.Bars {
		if bar.High == high {
			highBar = i
		}
		if bar.Low == low {
			lowBar = i
		}
	}

	// Classify the day pattern
	if pctChange > 1.0 {
		return "trend-up"
	} else if pctChange < -1.0 {
		return "trend-down"
	} else if lowBar < 200 && pctLow > 1.5 {
		return "V-bottom"
	} else if highBar < 200 && pctHigh > 1.5 {
		return "inverted-V"
	}
	return "choppy"
}

// estimateVolatility calculates realized volatility
func estimateVolatility(day *historical.TradingDay) float64 {
	if len(day.Bars) < 2 {
		return 0
	}

	var sumSquaredReturns float64
	for i := 1; i < len(day.Bars); i++ {
		ret := float64(day.Bars[i].Close-day.Bars[i-1].Close) / float64(day.Bars[i-1].Close)
		sumSquaredReturns += ret * ret
	}

	// Annualized vol would be sqrt(252 * sum), but we just want daily
	return 100 * (sumSquaredReturns * float64(len(day.Bars)))
}

// Getters

// CurrentMatch returns the current match
func (s *Scheduler) CurrentMatch() *match.Match {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMatch
}

// OrderBook returns the current order book
func (s *Scheduler) OrderBook() *orderbook.OrderBook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orderBook
}

// PriceFeed returns the current price feed
func (s *Scheduler) PriceFeed() *match.PriceFeed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.priceFeed
}

// RedemptionEngine returns the current redemption engine
func (s *Scheduler) RedemptionEngine() *match.RedemptionEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.redemption
}

// BotManager returns the current bot manager
func (s *Scheduler) BotManager() *bots.BotManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.botManager
}

// Actions

// JoinMatch adds a user to the current match
func (s *Scheduler) JoinMatch(userID, accountID string) error {
	s.mu.RLock()
	m := s.currentMatch
	s.mu.RUnlock()

	if m == nil {
		return nil
	}

	return m.Join(userID, accountID)
}

// LeaveMatch removes a user from the current match
func (s *Scheduler) LeaveMatch(userID string) {
	s.mu.RLock()
	m := s.currentMatch
	s.mu.RUnlock()

	if m != nil {
		m.Leave(userID)
	}
}

// TransitionToPreMatch starts the pre-match countdown
func (s *Scheduler) TransitionToPreMatch() error {
	s.mu.RLock()
	m := s.currentMatch
	s.mu.RUnlock()

	if m == nil {
		return nil
	}

	return m.TransitionToPreMatch()
}

// StartTrading begins the trading phase
func (s *Scheduler) StartTrading() error {
	s.mu.RLock()
	m := s.currentMatch
	s.mu.RUnlock()

	if m == nil {
		return nil
	}

	return m.Start()
}

// SettleMatch settles the current match
func (s *Scheduler) SettleMatch() {
	s.mu.RLock()
	m := s.currentMatch
	book := s.orderBook
	s.mu.RUnlock()

	if m == nil {
		return
	}

	// Fetch final positions from order book's position tracking
	// For now, use starting positions since we don't have real position tracking yet
	m.Settle(func(userID string) (cash int64, shares int64) {
		p := m.GetParticipant(userID)
		if p == nil {
			return 0, 0
		}
		// TODO: Get real positions from position tracker
		_ = book
		return p.StartingCash, p.StartingShares
	})
}

// Callbacks

// OnMatchStart sets the callback for when a match starts
func (s *Scheduler) OnMatchStart(fn func(*match.Match)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMatchStart = fn
}

// OnMatchEnd sets the callback for when a match ends
func (s *Scheduler) OnMatchEnd(fn func(*match.Match, []store.MatchResult)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMatchEnd = fn
}

// SetDuration changes the default match duration
func (s *Scheduler) SetDuration(duration historical.MatchDuration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultDuration = duration
}

func maxInt(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
