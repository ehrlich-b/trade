package match

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"trade/internal/orderbook"
)

// PriceFeed provides different price views for the match
type PriceFeed struct {
	mu sync.RWMutex

	match     *Match
	orderBook *orderbook.OrderBook
	rng       *rand.Rand

	// Configuration
	mmFuzzCents int64 // Max fuzz for MM reference price (e.g., 10 = ±$0.10)

	// Cached values
	trueNAV     int64 // True NAV from historical data
	mmReference int64 // Fuzzed reference for market makers
	bookMid     int64 // Order book mid price

	// Subscribers
	subscribers []chan PriceTick
	stopCh      chan struct{}
}

// PriceTick represents a price update
type PriceTick struct {
	Timestamp   time.Time `json:"timestamp"`
	TrueNAV     int64     `json:"true_nav"`     // Historical NAV (internal only)
	MMReference int64     `json:"mm_reference"` // Fuzzed price for MMs
	BookMid     int64     `json:"book_mid"`     // Order book mid price
	MarketTime  string    `json:"market_time"`  // Simulated market time
	Progress    float64   `json:"progress"`     // Match progress 0-1
}

// NewPriceFeed creates a new price feed
func NewPriceFeed(match *Match, orderBook *orderbook.OrderBook) *PriceFeed {
	return &PriceFeed{
		match:       match,
		orderBook:   orderBook,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		mmFuzzCents: 10, // Default ±$0.10 fuzz
		stopCh:      make(chan struct{}),
	}
}

// SetMMFuzz sets the maximum fuzz amount for MM reference price
func (pf *PriceFeed) SetMMFuzz(cents int64) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	pf.mmFuzzCents = cents
}

// Start begins the price feed updates
func (pf *PriceFeed) Start() {
	// Connect to match price ticks
	pf.match.OnPriceTick(func(nav int64) {
		pf.updatePrices(nav)
	})

	// Send initial price tick immediately
	initialNAV := pf.match.GetNAV()
	log.Printf("[PriceFeed] Starting with initial NAV: %d cents ($%.2f)", initialNAV, float64(initialNAV)/100)
	if initialNAV > 0 {
		pf.updatePrices(initialNAV)
		log.Printf("[PriceFeed] Set initial MM reference: %d cents ($%.2f)", pf.mmReference, float64(pf.mmReference)/100)
	} else {
		log.Printf("[PriceFeed] WARNING: Initial NAV is 0, bots won't quote!")
	}

	// Also poll periodically for book mid updates
	go pf.updateLoop()
}

// updateLoop periodically updates prices
func (pf *PriceFeed) updateLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pf.refreshBookMid()
		case <-pf.stopCh:
			return
		}
	}
}

// updatePrices updates all price views based on new NAV
func (pf *PriceFeed) updatePrices(nav int64) {
	// Get external values BEFORE acquiring lock to avoid deadlock
	bookMid := pf.orderBook.MidPrice()
	marketTime := pf.match.MarketTime()
	progress := pf.match.Progress()

	pf.mu.Lock()

	pf.trueNAV = nav

	// Generate fuzzed MM reference
	fuzz := pf.rng.Int63n(pf.mmFuzzCents*2+1) - pf.mmFuzzCents
	pf.mmReference = nav + fuzz

	// Get order book mid
	pf.bookMid = bookMid
	if pf.bookMid == 0 {
		pf.bookMid = nav
	}

	// Prepare tick
	tick := PriceTick{
		Timestamp:   time.Now(),
		TrueNAV:     pf.trueNAV,
		MMReference: pf.mmReference,
		BookMid:     pf.bookMid,
		MarketTime:  marketTime,
		Progress:    progress,
	}

	// Copy subscribers
	subs := make([]chan PriceTick, len(pf.subscribers))
	copy(subs, pf.subscribers)

	pf.mu.Unlock()

	// Notify subscribers (non-blocking)
	for _, ch := range subs {
		select {
		case ch <- tick:
		default:
		}
	}
}

// refreshBookMid updates just the book mid price
func (pf *PriceFeed) refreshBookMid() {
	mid := pf.orderBook.MidPrice()
	pf.mu.Lock()
	pf.bookMid = mid
	pf.mu.Unlock()
}

// Stop halts the price feed
func (pf *PriceFeed) Stop() {
	close(pf.stopCh)
}

// Subscribe returns a channel that receives price ticks
func (pf *PriceFeed) Subscribe() chan PriceTick {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	ch := make(chan PriceTick, 10)
	pf.subscribers = append(pf.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel
func (pf *PriceFeed) Unsubscribe(ch chan PriceTick) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	for i, sub := range pf.subscribers {
		if sub == ch {
			pf.subscribers = append(pf.subscribers[:i], pf.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Getters

// TrueNAV returns the true historical NAV (internal use only)
func (pf *PriceFeed) TrueNAV() int64 {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.trueNAV
}

// MMReference returns the fuzzed reference price for market makers
func (pf *PriceFeed) MMReference() int64 {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.mmReference
}

// BookMid returns the order book mid price
func (pf *PriceFeed) BookMid() int64 {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.bookMid
}

// CurrentTick returns the current price tick
func (pf *PriceFeed) CurrentTick() PriceTick {
	// Get external values first
	marketTime := pf.match.MarketTime()
	progress := pf.match.Progress()

	pf.mu.RLock()
	tick := PriceTick{
		Timestamp:   time.Now(),
		TrueNAV:     pf.trueNAV,
		MMReference: pf.mmReference,
		BookMid:     pf.bookMid,
		MarketTime:  marketTime,
		Progress:    progress,
	}
	pf.mu.RUnlock()
	return tick
}

// RedemptionEngine handles create/redeem share mechanics
type RedemptionEngine struct {
	mu sync.Mutex

	nav              int64   // Current NAV in cents
	cumulativeVolume int64   // Total shares redeemed/created this match
	matchDuration    float64 // Match length in minutes
	startTime        time.Time
}

const (
	BaseFee    = 0.005 // 0.5% starting fee
	MaxFee     = 0.03  // 3% asymptotic cap
	VolumeHalf = 50000 // Volume at which fee is halfway to max
)

// NewRedemptionEngine creates a new redemption engine
func NewRedemptionEngine(matchDurationMinutes int) *RedemptionEngine {
	return &RedemptionEngine{
		matchDuration: float64(matchDurationMinutes),
		startTime:     time.Now(),
	}
}

// SetNAV updates the current NAV
func (r *RedemptionEngine) SetNAV(nav int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nav = nav
}

// CurrentFee returns the current redemption fee (increases with volume and time)
func (r *RedemptionEngine) CurrentFee() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Time factor: fee pressure increases as match progresses
	elapsedMinutes := time.Since(r.startTime).Minutes()
	timeFactor := 1.0 + (elapsedMinutes / r.matchDuration)

	// Volume factor: asymptotic rise toward MaxFee
	effectiveVol := float64(r.cumulativeVolume) * timeFactor
	volRatio := effectiveVol / (effectiveVol + float64(VolumeHalf))

	return BaseFee + (MaxFee-BaseFee)*volRatio
}

// CreationPrice returns the price to create (buy) new shares at NAV + fee
func (r *RedemptionEngine) CreationPrice() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(float64(r.nav) * (1 + r.currentFeeUnlocked()))
}

// RedemptionPrice returns the price to redeem (sell) shares at NAV - fee
func (r *RedemptionEngine) RedemptionPrice() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(float64(r.nav) * (1 - r.currentFeeUnlocked()))
}

// currentFeeUnlocked calculates fee (caller must hold lock)
func (r *RedemptionEngine) currentFeeUnlocked() float64 {
	elapsedMinutes := time.Since(r.startTime).Minutes()
	timeFactor := 1.0 + (elapsedMinutes / r.matchDuration)
	effectiveVol := float64(r.cumulativeVolume) * timeFactor
	volRatio := effectiveVol / (effectiveVol + float64(VolumeHalf))
	return BaseFee + (MaxFee-BaseFee)*volRatio
}

// Create creates new shares at the creation price
// Returns the price paid per share
func (r *RedemptionEngine) Create(quantity int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cumulativeVolume += quantity
	return int64(float64(r.nav) * (1 + r.currentFeeUnlocked()))
}

// Redeem redeems shares at the redemption price
// Returns the price received per share
func (r *RedemptionEngine) Redeem(quantity int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cumulativeVolume += quantity
	return int64(float64(r.nav) * (1 - r.currentFeeUnlocked()))
}

// GetNAV returns the current NAV
func (r *RedemptionEngine) GetNAV() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nav
}

// Status returns redemption engine status
type RedemptionStatus struct {
	NAV              int64   `json:"nav"`
	CurrentFee       float64 `json:"current_fee"`
	CreationPrice    int64   `json:"creation_price"`
	RedemptionPrice  int64   `json:"redemption_price"`
	CumulativeVolume int64   `json:"cumulative_volume"`
}

// Status returns the current redemption status
func (r *RedemptionEngine) Status() RedemptionStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	fee := r.currentFeeUnlocked()
	return RedemptionStatus{
		NAV:              r.nav,
		CurrentFee:       fee,
		CreationPrice:    int64(float64(r.nav) * (1 + fee)),
		RedemptionPrice:  int64(float64(r.nav) * (1 - fee)),
		CumulativeVolume: r.cumulativeVolume,
	}
}
