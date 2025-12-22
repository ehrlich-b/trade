package bots

import (
	"math/rand"
	"time"

	"trade/internal/match"
	"trade/internal/orderbook"
)

// NoiseTraderBot places random orders to create market texture
type NoiseTraderBot struct {
	*BaseBot
	avgInterval time.Duration // Average time between trades
	minSize     int64         // Minimum order size
	maxSize     int64         // Maximum order size
	bias        float64       // Directional bias (-1 to +1, 0 = neutral)

	rng *rand.Rand
}

// NewNoiseTraderBot creates a noise trader
func NewNoiseTraderBot(id string, avgInterval time.Duration, minSize, maxSize int64, bias float64, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *NoiseTraderBot {
	return &NoiseTraderBot{
		BaseBot:     NewBaseBot(id, book, priceFeed),
		avgInterval: avgInterval,
		minSize:     minSize,
		maxSize:     maxSize,
		bias:        bias,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (n *NoiseTraderBot) Start() {
	go n.tradeLoop()
}

func (n *NoiseTraderBot) tradeLoop() {
	for {
		// Random interval (exponential distribution around average)
		waitTime := time.Duration(float64(n.avgInterval) * (0.5 + n.rng.Float64()))

		select {
		case <-time.After(waitTime):
			n.placeRandomOrder()
		case <-n.stopCh:
			return
		}
	}
}

func (n *NoiseTraderBot) placeRandomOrder() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Random size
	size := n.minSize + n.rng.Int63n(n.maxSize-n.minSize+1)

	// Random side with bias
	side := orderbook.Buy
	if n.rng.Float64() > (0.5 + n.bias/2) {
		side = orderbook.Sell
	}

	n.book.Submit(&orderbook.Order{
		UserID:   n.id,
		Side:     side,
		Type:     orderbook.Market,
		Quantity: size,
	})
}

// Preset noise traders

// NewRandomSmall places small random orders frequently
func NewRandomSmall(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *NoiseTraderBot {
	return NewNoiseTraderBot(id, 3*time.Second, 5, 20, 0, book, priceFeed)
}

// NewRandomLarge places larger random orders infrequently
func NewRandomLarge(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *NoiseTraderBot {
	return NewNoiseTraderBot(id, 30*time.Second, 100, 500, 0, book, priceFeed)
}

// PanicBot overreacts to price moves
type PanicBot struct {
	*BaseBot
	panicThreshold int64         // Price move that triggers panic (cents)
	panicSize      int64         // Size of panic trades
	cooldown       time.Duration // Minimum time between panics
	lastPanic      time.Time
	lastPrice      int64

	rng *rand.Rand
}

// NewPanicBot creates a panic trader
func NewPanicBot(id string, threshold, size int64, cooldown time.Duration, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *PanicBot {
	return &PanicBot{
		BaseBot:        NewBaseBot(id, book, priceFeed),
		panicThreshold: threshold,
		panicSize:      size,
		cooldown:       cooldown,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *PanicBot) Start() {
	go p.watchLoop()
}

func (p *PanicBot) watchLoop() {
	priceCh := p.priceFeed.Subscribe()
	defer p.priceFeed.Unsubscribe(priceCh)

	for {
		select {
		case tick := <-priceCh:
			p.checkPanic(tick.BookMid)
		case <-p.stopCh:
			return
		}
	}
}

func (p *PanicBot) checkPanic(price int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.lastPrice == 0 {
		p.lastPrice = price
		return
	}

	// Check cooldown
	if time.Since(p.lastPanic) < p.cooldown {
		p.lastPrice = price
		return
	}

	move := price - p.lastPrice
	p.lastPrice = price

	// Check if move exceeds panic threshold
	if abs(move) < p.panicThreshold {
		return
	}

	// PANIC! Trade in the direction of the move (chase)
	p.lastPanic = time.Now()

	if move > 0 {
		// Price spiking up - panic buy!
		p.book.Submit(&orderbook.Order{
			UserID:   p.id,
			Side:     orderbook.Buy,
			Type:     orderbook.Market,
			Quantity: p.panicSize,
		})
	} else {
		// Price crashing - panic sell!
		p.book.Submit(&orderbook.Order{
			UserID:   p.id,
			Side:     orderbook.Sell,
			Type:     orderbook.Market,
			Quantity: p.panicSize,
		})
	}
}

// NewPanicStandard creates a standard panic bot
func NewPanicStandard(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *PanicBot {
	return NewPanicBot(id, 15, 50, 5*time.Second, book, priceFeed)
}

// MandatedAgent has a quota to fill during the match
type MandatedAgent struct {
	*BaseBot
	mandate       int64         // Target quantity (positive = buy, negative = sell)
	filled        int64         // Quantity filled so far
	deadline      time.Duration // Time to complete mandate
	startTime     time.Time
	strategy      string        // "TWAP", "VWAP", "opportunistic"
	urgency       float64       // 0.0 = patient, 1.0 = desperate

	rng *rand.Rand
}

// NewMandatedAgent creates a mandated execution agent
func NewMandatedAgent(id string, mandate int64, deadline time.Duration, strategy string, urgency float64, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MandatedAgent {
	return &MandatedAgent{
		BaseBot:  NewBaseBot(id, book, priceFeed),
		mandate:  mandate,
		deadline: deadline,
		strategy: strategy,
		urgency:  urgency,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *MandatedAgent) Start() {
	m.startTime = time.Now()
	go m.executeLoop()
}

func (m *MandatedAgent) executeLoop() {
	// Calculate trade interval based on strategy
	remaining := abs(m.mandate)
	slices := int(m.deadline.Seconds() / 5) // Trade every ~5 seconds
	if slices < 1 {
		slices = 1
	}
	tradeSize := remaining / int64(slices)
	if tradeSize < 1 {
		tradeSize = 1
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.executeTrade(tradeSize)
		case <-m.stopCh:
			return
		}
	}
}

func (m *MandatedAgent) executeTrade(baseSize int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mandate is complete
	remaining := abs(m.mandate) - abs(m.filled)
	if remaining <= 0 {
		return
	}

	// Calculate urgency based on time remaining
	elapsed := time.Since(m.startTime)
	timeProgress := elapsed.Seconds() / m.deadline.Seconds()
	fillProgress := float64(abs(m.filled)) / float64(abs(m.mandate))

	// If behind schedule, increase urgency
	currentUrgency := m.urgency
	if timeProgress > fillProgress {
		// Behind schedule
		behindBy := timeProgress - fillProgress
		currentUrgency = m.urgency + behindBy
		if currentUrgency > 1.0 {
			currentUrgency = 1.0
		}
	}

	// Adjust size based on urgency
	size := baseSize
	if currentUrgency > 0.7 {
		// Desperate - trade larger
		size = baseSize * 2
	}
	if size > remaining {
		size = remaining
	}

	// Determine order type based on strategy and urgency
	orderType := orderbook.Limit
	if m.strategy == "opportunistic" && currentUrgency < 0.5 {
		// Opportunistic: use limit orders when patient
		orderType = orderbook.Limit
	} else if currentUrgency > 0.8 {
		// Very urgent: use market orders
		orderType = orderbook.Market
	}

	// Submit order
	if m.mandate > 0 {
		// Buying mandate
		price := m.priceFeed.BookMid()
		if orderType == orderbook.Limit && price > 0 {
			// Bid slightly below mid
			price = price - 2
		}
		order := &orderbook.Order{
			UserID:   m.id,
			Side:     orderbook.Buy,
			Type:     orderType,
			Price:    price,
			Quantity: size,
		}
		trades, _ := m.book.Submit(order)
		for _, trade := range trades {
			if trade.BuyerID == m.id {
				m.filled += trade.Quantity
			}
		}
	} else {
		// Selling mandate
		price := m.priceFeed.BookMid()
		if orderType == orderbook.Limit && price > 0 {
			// Ask slightly above mid
			price = price + 2
		}
		order := &orderbook.Order{
			UserID:   m.id,
			Side:     orderbook.Sell,
			Type:     orderType,
			Price:    price,
			Quantity: size,
		}
		trades, _ := m.book.Submit(order)
		for _, trade := range trades {
			if trade.SellerID == m.id {
				m.filled += trade.Quantity
			}
		}
	}
}

// Progress returns the mandate completion progress
func (m *MandatedAgent) Progress() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mandate == 0 {
		return 1.0
	}
	return float64(abs(m.filled)) / float64(abs(m.mandate))
}

// NewTWAPBuyer creates a patient TWAP buyer
func NewTWAPBuyer(id string, quantity int64, duration time.Duration, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MandatedAgent {
	return NewMandatedAgent(id, quantity, duration, "TWAP", 0.3, book, priceFeed)
}

// NewTWAPSeller creates a patient TWAP seller
func NewTWAPSeller(id string, quantity int64, duration time.Duration, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MandatedAgent {
	return NewMandatedAgent(id, -quantity, duration, "TWAP", 0.3, book, priceFeed)
}

// NewOpportunisticBuyer creates an opportunistic buyer
func NewOpportunisticBuyer(id string, quantity int64, duration time.Duration, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MandatedAgent {
	return NewMandatedAgent(id, quantity, duration, "opportunistic", 0.4, book, priceFeed)
}

// NewDesperateSeller creates a desperate seller (like margin call)
func NewDesperateSeller(id string, quantity int64, duration time.Duration, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MandatedAgent {
	return NewMandatedAgent(id, -quantity, duration, "TWAP", 0.9, book, priceFeed)
}
