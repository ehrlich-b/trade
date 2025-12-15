package market

import (
	"math/rand"
	"sync"
	"time"
)

// PriceGenerator produces synthetic price movements via random walk
type PriceGenerator struct {
	mu          sync.RWMutex
	price       int64   // Current price in cents
	volatility  float64 // Standard deviation of price changes (in cents)
	drift       float64 // Mean drift per tick (in cents, usually 0)
	minPrice    int64   // Floor price
	maxPrice    int64   // Ceiling price
	subscribers []chan int64
	stopCh      chan struct{}
	rng         *rand.Rand
}

// NewPriceGenerator creates a new random walk price generator
func NewPriceGenerator(initialPrice int64, volatility float64) *PriceGenerator {
	return &PriceGenerator{
		price:      initialPrice,
		volatility: volatility,
		drift:      0,
		minPrice:   100,    // $0.01 minimum
		maxPrice:   100000, // $1000 maximum
		stopCh:     make(chan struct{}),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Price returns the current synthetic price
func (pg *PriceGenerator) Price() int64 {
	pg.mu.RLock()
	defer pg.mu.RUnlock()
	return pg.price
}

// Subscribe returns a channel that receives price updates
func (pg *PriceGenerator) Subscribe() chan int64 {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	ch := make(chan int64, 10)
	pg.subscribers = append(pg.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel
func (pg *PriceGenerator) Unsubscribe(ch chan int64) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	for i, sub := range pg.subscribers {
		if sub == ch {
			pg.subscribers = append(pg.subscribers[:i], pg.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Start begins generating price updates at the given interval
func (pg *PriceGenerator) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pg.tick()
			case <-pg.stopCh:
				return
			}
		}
	}()
}

// Stop halts price generation
func (pg *PriceGenerator) Stop() {
	close(pg.stopCh)
}

// tick performs one random walk step
func (pg *PriceGenerator) tick() {
	pg.mu.Lock()

	// Random walk: price += drift + volatility * N(0,1)
	change := pg.drift + pg.volatility*pg.rng.NormFloat64()
	newPrice := pg.price + int64(change)

	// Clamp to bounds
	if newPrice < pg.minPrice {
		newPrice = pg.minPrice
	}
	if newPrice > pg.maxPrice {
		newPrice = pg.maxPrice
	}

	pg.price = newPrice
	subs := make([]chan int64, len(pg.subscribers))
	copy(subs, pg.subscribers)
	pg.mu.Unlock()

	// Notify subscribers (non-blocking)
	for _, ch := range subs {
		select {
		case ch <- newPrice:
		default:
			// Skip if channel is full
		}
	}
}

// SetVolatility adjusts the volatility (in cents)
func (pg *PriceGenerator) SetVolatility(v float64) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	pg.volatility = v
}

// SetDrift adjusts the drift (in cents per tick)
func (pg *PriceGenerator) SetDrift(d float64) {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	pg.drift = d
}
