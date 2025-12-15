package market

import (
	"log"
	"sync"
	"time"

	"trade/internal/orderbook"
)

// MarketMaker maintains liquidity around a synthetic price
type MarketMaker struct {
	mu           sync.Mutex
	book         *orderbook.OrderBook
	priceGen     *PriceGenerator
	userID       string
	spread       int64 // Half-spread in cents (distance from mid to quote)
	levels       int   // Number of price levels to quote
	sizePerLevel int64 // Quantity per level
	orderIDs     []string
	stopCh       chan struct{}
	onUpdate     func() // Callback when book is updated
}

// NewMarketMaker creates a new market maker bot
func NewMarketMaker(book *orderbook.OrderBook, priceGen *PriceGenerator) *MarketMaker {
	return &MarketMaker{
		book:         book,
		priceGen:     priceGen,
		userID:       "market_maker",
		spread:       10,  // $0.10 half-spread
		levels:       5,   // 5 levels each side
		sizePerLevel: 100, // 100 shares per level
		stopCh:       make(chan struct{}),
	}
}

// SetSpread sets the half-spread in cents
func (mm *MarketMaker) SetSpread(cents int64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.spread = cents
}

// SetLevels sets the number of price levels to quote
func (mm *MarketMaker) SetLevels(n int) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.levels = n
}

// SetSizePerLevel sets the quantity at each price level
func (mm *MarketMaker) SetSizePerLevel(size int64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.sizePerLevel = size
}

// SetOnUpdate sets a callback to be called when orders are updated
func (mm *MarketMaker) SetOnUpdate(fn func()) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.onUpdate = fn
}

// Start begins the market maker loop
func (mm *MarketMaker) Start() {
	// Subscribe to price updates
	priceCh := mm.priceGen.Subscribe()

	// Initial quote
	mm.requote()

	go func() {
		// Also requote periodically even without price changes
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-priceCh:
				mm.requote()
			case <-ticker.C:
				mm.requote()
			case <-mm.stopCh:
				mm.priceGen.Unsubscribe(priceCh)
				mm.cancelAllOrders()
				return
			}
		}
	}()
}

// Stop halts the market maker
func (mm *MarketMaker) Stop() {
	close(mm.stopCh)
}

// requote cancels existing orders and places new ones around current price
func (mm *MarketMaker) requote() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Cancel existing orders
	for _, orderID := range mm.orderIDs {
		if err := mm.book.Cancel(orderID); err != nil {
			// Order may have been filled, that's OK
		}
	}
	mm.orderIDs = nil

	midPrice := mm.priceGen.Price()
	if midPrice == 0 {
		return
	}

	// Place new bids (below mid)
	for i := 1; i <= mm.levels; i++ {
		price := midPrice - mm.spread*int64(i)
		if price <= 0 {
			continue
		}
		order := &orderbook.Order{
			UserID:   mm.userID,
			Side:     orderbook.Buy,
			Type:     orderbook.Limit,
			Price:    price,
			Quantity: mm.sizePerLevel,
		}
		if _, err := mm.book.Submit(order); err != nil {
			log.Printf("MM bid submit error: %v", err)
		} else {
			mm.orderIDs = append(mm.orderIDs, order.ID)
		}
	}

	// Place new asks (above mid)
	for i := 1; i <= mm.levels; i++ {
		price := midPrice + mm.spread*int64(i)
		order := &orderbook.Order{
			UserID:   mm.userID,
			Side:     orderbook.Sell,
			Type:     orderbook.Limit,
			Price:    price,
			Quantity: mm.sizePerLevel,
		}
		if _, err := mm.book.Submit(order); err != nil {
			log.Printf("MM ask submit error: %v", err)
		} else {
			mm.orderIDs = append(mm.orderIDs, order.ID)
		}
	}

	// Notify listener
	if mm.onUpdate != nil {
		mm.onUpdate()
	}
}

// cancelAllOrders removes all market maker orders from the book
func (mm *MarketMaker) cancelAllOrders() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, orderID := range mm.orderIDs {
		mm.book.Cancel(orderID)
	}
	mm.orderIDs = nil
}
