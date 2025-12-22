package bots

import (
	"math/rand"
	"time"

	"trade/internal/match"
	"trade/internal/orderbook"
)

// MomentumBot chases price trends
type MomentumBot struct {
	*BaseBot
	lookbackPeriod time.Duration // How far back to look for trend
	tradeInterval  time.Duration // How often to consider trading
	minMove        int64         // Minimum price move to trigger (cents)
	tradeSize      int64         // Size per trade
	maxPosition    int64         // Maximum position

	priceHistory []pricePoint
	rng          *rand.Rand
}

type pricePoint struct {
	time  time.Time
	price int64
}

// NewMomentumBot creates a momentum-following bot
func NewMomentumBot(id string, lookback, interval time.Duration, minMove, size, maxPos int64, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MomentumBot {
	return &MomentumBot{
		BaseBot:        NewBaseBot(id, book, priceFeed),
		lookbackPeriod: lookback,
		tradeInterval:  interval,
		minMove:        minMove,
		tradeSize:      size,
		maxPosition:    maxPos,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *MomentumBot) Start() {
	go m.tradeLoop()
}

func (m *MomentumBot) tradeLoop() {
	priceCh := m.priceFeed.Subscribe()
	defer m.priceFeed.Unsubscribe(priceCh)

	tradeTicker := time.NewTicker(m.tradeInterval)
	defer tradeTicker.Stop()

	for {
		select {
		case tick := <-priceCh:
			m.recordPrice(tick.TrueNAV)
		case <-tradeTicker.C:
			m.considerTrade()
		case <-m.stopCh:
			return
		}
	}
}

func (m *MomentumBot) recordPrice(price int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.priceHistory = append(m.priceHistory, pricePoint{time: now, price: price})

	// Trim old entries
	cutoff := now.Add(-m.lookbackPeriod * 2)
	for len(m.priceHistory) > 0 && m.priceHistory[0].time.Before(cutoff) {
		m.priceHistory = m.priceHistory[1:]
	}
}

func (m *MomentumBot) considerTrade() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.priceHistory) < 2 {
		return
	}

	// Find price from lookback period ago
	cutoff := time.Now().Add(-m.lookbackPeriod)
	var oldPrice int64
	for _, pp := range m.priceHistory {
		if pp.time.After(cutoff) {
			break
		}
		oldPrice = pp.price
	}

	if oldPrice == 0 {
		return
	}

	currentPrice := m.priceHistory[len(m.priceHistory)-1].price
	move := currentPrice - oldPrice

	// Check if move exceeds threshold
	if abs(move) < m.minMove {
		return
	}

	// Check position limits
	if move > 0 && m.position >= m.maxPosition {
		return // Already max long
	}
	if move < 0 && m.position <= -m.maxPosition {
		return // Already max short
	}

	// Chase the trend
	if move > 0 {
		// Price going up, buy
		m.book.Submit(&orderbook.Order{
			UserID:   m.id,
			Side:     orderbook.Buy,
			Type:     orderbook.Market,
			Quantity: m.tradeSize,
		})
	} else {
		// Price going down, sell
		m.book.Submit(&orderbook.Order{
			UserID:   m.id,
			Side:     orderbook.Sell,
			Type:     orderbook.Market,
			Quantity: m.tradeSize,
		})
	}
}

// Preset momentum bots

// NewMomentumFast chases 10-second trends
func NewMomentumFast(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MomentumBot {
	return NewMomentumBot(id, 10*time.Second, 2*time.Second, 5, 10, 100, book, priceFeed)
}

// NewMomentumSlow chases 1-minute trends
func NewMomentumSlow(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MomentumBot {
	return NewMomentumBot(id, 1*time.Minute, 15*time.Second, 15, 25, 200, book, priceFeed)
}

// MeanReversionBot fades large moves
type MeanReversionBot struct {
	*BaseBot
	referencePrice int64         // Price to revert to
	threshold      int64         // Minimum deviation to trade (cents)
	tradeInterval  time.Duration // How often to consider trading
	tradeSize      int64
	maxPosition    int64

	rng *rand.Rand
}

// NewMeanReversionBot creates a mean-reversion bot
func NewMeanReversionBot(id string, threshold, size, maxPos int64, interval time.Duration, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MeanReversionBot {
	return &MeanReversionBot{
		BaseBot:       NewBaseBot(id, book, priceFeed),
		threshold:     threshold,
		tradeInterval: interval,
		tradeSize:     size,
		maxPosition:   maxPos,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *MeanReversionBot) Start() {
	// Set reference price from current NAV
	m.referencePrice = m.priceFeed.TrueNAV()
	go m.tradeLoop()
}

func (m *MeanReversionBot) tradeLoop() {
	priceCh := m.priceFeed.Subscribe()
	defer m.priceFeed.Unsubscribe(priceCh)

	ticker := time.NewTicker(m.tradeInterval)
	defer ticker.Stop()

	for {
		select {
		case tick := <-priceCh:
			// Update reference with slow moving average
			if m.referencePrice == 0 {
				m.referencePrice = tick.TrueNAV
			} else {
				// Slow update: 99% old, 1% new
				m.referencePrice = (m.referencePrice*99 + tick.TrueNAV) / 100
			}
		case <-ticker.C:
			m.considerTrade()
		case <-m.stopCh:
			return
		}
	}
}

func (m *MeanReversionBot) considerTrade() {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentPrice := m.priceFeed.BookMid()
	if currentPrice == 0 || m.referencePrice == 0 {
		return
	}

	deviation := currentPrice - m.referencePrice

	// Only trade if deviation exceeds threshold
	if abs(deviation) < m.threshold {
		return
	}

	// Fade the move (trade against deviation)
	if deviation > 0 {
		// Price above reference, sell
		if m.position > -m.maxPosition {
			m.book.Submit(&orderbook.Order{
				UserID:   m.id,
				Side:     orderbook.Sell,
				Type:     orderbook.Market,
				Quantity: m.tradeSize,
			})
		}
	} else {
		// Price below reference, buy
		if m.position < m.maxPosition {
			m.book.Submit(&orderbook.Order{
				UserID:   m.id,
				Side:     orderbook.Buy,
				Type:     orderbook.Market,
				Quantity: m.tradeSize,
			})
		}
	}
}

// NewMeanReversionStandard fades 20¢+ moves
func NewMeanReversionStandard(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MeanReversionBot {
	return NewMeanReversionBot(id, 20, 15, 150, 5*time.Second, book, priceFeed)
}

// BreakoutBot jumps on range expansions
type BreakoutBot struct {
	*BaseBot
	windowSize    int   // Number of prices to track for range
	breakoutMult  float64 // Breakout threshold as multiplier of range
	tradeSize     int64
	maxPosition   int64
	cooldownTicks int // Ticks to wait after trading

	priceWindow   []int64
	cooldownLeft  int

	rng *rand.Rand
}

// NewBreakoutBot creates a breakout-following bot
func NewBreakoutBot(id string, windowSize int, breakoutMult float64, size, maxPos int64, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *BreakoutBot {
	return &BreakoutBot{
		BaseBot:       NewBaseBot(id, book, priceFeed),
		windowSize:    windowSize,
		breakoutMult:  breakoutMult,
		tradeSize:     size,
		maxPosition:   maxPos,
		cooldownTicks: 10,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *BreakoutBot) Start() {
	go b.tradeLoop()
}

func (b *BreakoutBot) tradeLoop() {
	priceCh := b.priceFeed.Subscribe()
	defer b.priceFeed.Unsubscribe(priceCh)

	for {
		select {
		case tick := <-priceCh:
			b.processTick(tick.TrueNAV)
		case <-b.stopCh:
			return
		}
	}
}

func (b *BreakoutBot) processTick(price int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Add to window
	b.priceWindow = append(b.priceWindow, price)
	if len(b.priceWindow) > b.windowSize {
		b.priceWindow = b.priceWindow[1:]
	}

	// Need full window
	if len(b.priceWindow) < b.windowSize {
		return
	}

	// Cooldown
	if b.cooldownLeft > 0 {
		b.cooldownLeft--
		return
	}

	// Calculate range (excluding last price)
	rangeHigh := b.priceWindow[0]
	rangeLow := b.priceWindow[0]
	for i := 0; i < len(b.priceWindow)-1; i++ {
		if b.priceWindow[i] > rangeHigh {
			rangeHigh = b.priceWindow[i]
		}
		if b.priceWindow[i] < rangeLow {
			rangeLow = b.priceWindow[i]
		}
	}

	rangeSize := rangeHigh - rangeLow
	if rangeSize == 0 {
		return
	}

	// Check for breakout
	lastPrice := b.priceWindow[len(b.priceWindow)-1]
	breakoutThreshold := int64(float64(rangeSize) * b.breakoutMult)

	if lastPrice > rangeHigh+breakoutThreshold {
		// Upside breakout - buy
		if b.position < b.maxPosition {
			b.book.Submit(&orderbook.Order{
				UserID:   b.id,
				Side:     orderbook.Buy,
				Type:     orderbook.Market,
				Quantity: b.tradeSize,
			})
			b.cooldownLeft = b.cooldownTicks
		}
	} else if lastPrice < rangeLow-breakoutThreshold {
		// Downside breakout - sell
		if b.position > -b.maxPosition {
			b.book.Submit(&orderbook.Order{
				UserID:   b.id,
				Side:     orderbook.Sell,
				Type:     orderbook.Market,
				Quantity: b.tradeSize,
			})
			b.cooldownLeft = b.cooldownTicks
		}
	}
}

// NewBreakoutStandard creates a standard breakout bot
func NewBreakoutStandard(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *BreakoutBot {
	return NewBreakoutBot(id, 20, 0.5, 20, 100, book, priceFeed)
}
