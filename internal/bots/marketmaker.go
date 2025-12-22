package bots

import (
	"math"
	"math/rand"
	"time"

	"trade/internal/match"
	"trade/internal/orderbook"
)

// MMConfig configures a market maker bot
type MMConfig struct {
	ID              string
	HalfSpread      int64         // Distance from mid to quote (in cents)
	SizePerLevel    int64         // Quantity per price level
	Levels          int           // Number of levels on each side
	QuoteInterval   time.Duration // How often to re-quote
	MaxPosition     int64         // Maximum allowed position
	InventorySkew   float64       // How much to skew quotes based on inventory (0-1)
	WidenOnVolatility bool        // Widen spread during volatility
}

// MarketMakerBot provides liquidity around the reference price
type MarketMakerBot struct {
	*BaseBot
	config   MMConfig
	orderIDs []string
	rng      *rand.Rand

	// Volatility tracking
	lastPrice    int64
	priceChanges []int64
}

// NewMarketMakerBot creates a new market maker bot
func NewMarketMakerBot(config MMConfig, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MarketMakerBot {
	return &MarketMakerBot{
		BaseBot: NewBaseBot(config.ID, book, priceFeed),
		config:  config,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (mm *MarketMakerBot) Start() {
	go mm.quoteLoop()
}

func (mm *MarketMakerBot) quoteLoop() {
	// Subscribe to price updates
	priceCh := mm.priceFeed.Subscribe()
	defer mm.priceFeed.Unsubscribe(priceCh)

	// Initial quote
	mm.requote()

	ticker := time.NewTicker(mm.config.QuoteInterval)
	defer ticker.Stop()

	for {
		select {
		case <-priceCh:
			mm.requote()
		case <-ticker.C:
			mm.requote()
		case <-mm.stopCh:
			mm.CancelAllOrders()
			return
		}
	}
}

func (mm *MarketMakerBot) requote() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Cancel existing orders
	for _, orderID := range mm.orderIDs {
		mm.book.Cancel(orderID)
	}
	mm.orderIDs = nil

	// Get reference price (fuzzed MM reference from price feed)
	refPrice := mm.priceFeed.MMReference()
	if refPrice == 0 {
		return
	}

	// Track volatility
	if mm.lastPrice != 0 {
		change := abs(refPrice - mm.lastPrice)
		mm.priceChanges = append(mm.priceChanges, change)
		if len(mm.priceChanges) > 20 {
			mm.priceChanges = mm.priceChanges[1:]
		}
	}
	mm.lastPrice = refPrice

	// Calculate spread (possibly widened for volatility)
	spread := mm.config.HalfSpread
	if mm.config.WidenOnVolatility && len(mm.priceChanges) > 5 {
		avgChange := average(mm.priceChanges)
		if avgChange > float64(spread) {
			spread = int64(avgChange * 1.5)
		}
	}

	// Calculate inventory skew
	// If long, lower our bid (less eager to buy more)
	// If short, raise our ask (less eager to sell more)
	skew := int64(0)
	if mm.config.InventorySkew > 0 && mm.position != 0 {
		skewAmount := float64(mm.position) * mm.config.InventorySkew
		skew = int64(skewAmount)
	}

	// Check position limits
	canBuy := mm.config.MaxPosition == 0 || mm.position < mm.config.MaxPosition
	canSell := mm.config.MaxPosition == 0 || mm.position > -mm.config.MaxPosition

	// Place bids (below mid, adjusted for skew)
	if canBuy {
		for i := 1; i <= mm.config.Levels; i++ {
			price := refPrice - spread*int64(i) - skew
			if price <= 0 {
				continue
			}
			order := &orderbook.Order{
				UserID:   mm.id,
				Side:     orderbook.Buy,
				Type:     orderbook.Limit,
				Price:    price,
				Quantity: mm.config.SizePerLevel,
			}
			if _, err := mm.book.Submit(order); err == nil {
				mm.orderIDs = append(mm.orderIDs, order.ID)
			}
		}
	}

	// Place asks (above mid, adjusted for skew)
	if canSell {
		for i := 1; i <= mm.config.Levels; i++ {
			price := refPrice + spread*int64(i) - skew
			order := &orderbook.Order{
				UserID:   mm.id,
				Side:     orderbook.Sell,
				Type:     orderbook.Limit,
				Price:    price,
				Quantity: mm.config.SizePerLevel,
			}
			if _, err := mm.book.Submit(order); err == nil {
				mm.orderIDs = append(mm.orderIDs, order.ID)
			}
		}
	}
}

// Preset market maker configurations

// TightMM: 5¢ spread, size 20, fast quotes
func NewTightMM(book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MarketMakerBot {
	return NewMarketMakerBot(MMConfig{
		ID:              "mm_tight",
		HalfSpread:      5,  // 5¢
		SizePerLevel:    20,
		Levels:          3,
		QuoteInterval:   500 * time.Millisecond,
		MaxPosition:     500,
		InventorySkew:   0.5,
		WidenOnVolatility: true,
	}, book, priceFeed)
}

// WideMM: 25¢ spread, size 200, slow quotes
func NewWideMM(book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MarketMakerBot {
	return NewMarketMakerBot(MMConfig{
		ID:              "mm_wide",
		HalfSpread:      25, // 25¢
		SizePerLevel:    200,
		Levels:          3,
		QuoteInterval:   2 * time.Second,
		MaxPosition:     2000,
		InventorySkew:   0.2,
		WidenOnVolatility: false, // "dumb" MM
	}, book, priceFeed)
}

// AdaptiveMM: 10¢ spread, adapts to inventory and volatility
func NewAdaptiveMM(book *orderbook.OrderBook, priceFeed *match.PriceFeed) *MarketMakerBot {
	return NewMarketMakerBot(MMConfig{
		ID:              "mm_adaptive",
		HalfSpread:      10,
		SizePerLevel:    50,
		Levels:          4,
		QuoteInterval:   1 * time.Second,
		MaxPosition:     1000,
		InventorySkew:   0.8, // Strong inventory skew
		WidenOnVolatility: true,
	}, book, priceFeed)
}

// NervousMM: Pulls quotes on big moves
type NervousMM struct {
	*MarketMakerBot
	volatilityThreshold int64 // Pull quotes if volatility exceeds this
	pullDuration        time.Duration
	pulledUntil         time.Time
}

func NewNervousMM(book *orderbook.OrderBook, priceFeed *match.PriceFeed) *NervousMM {
	base := NewMarketMakerBot(MMConfig{
		ID:              "mm_nervous",
		HalfSpread:      10,
		SizePerLevel:    30,
		Levels:          2,
		QuoteInterval:   1 * time.Second,
		MaxPosition:     300,
		InventorySkew:   1.0, // Very sensitive to inventory
		WidenOnVolatility: true,
	}, book, priceFeed)

	return &NervousMM{
		MarketMakerBot:      base,
		volatilityThreshold: 20, // 20¢ move triggers pulling quotes
		pullDuration:        5 * time.Second,
	}
}

func (mm *NervousMM) Start() {
	go mm.nervousQuoteLoop()
}

func (mm *NervousMM) nervousQuoteLoop() {
	priceCh := mm.priceFeed.Subscribe()
	defer mm.priceFeed.Unsubscribe(priceCh)

	ticker := time.NewTicker(mm.config.QuoteInterval)
	defer ticker.Stop()

	for {
		select {
		case <-priceCh:
			mm.nervousRequote()
		case <-ticker.C:
			mm.nervousRequote()
		case <-mm.stopCh:
			mm.CancelAllOrders()
			return
		}
	}
}

func (mm *NervousMM) nervousRequote() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Check if we should be pulled
	if time.Now().Before(mm.pulledUntil) {
		// Still pulled - cancel all orders
		for _, orderID := range mm.orderIDs {
			mm.book.Cancel(orderID)
		}
		mm.orderIDs = nil
		return
	}

	// Check for big move
	refPrice := mm.priceFeed.MMReference()
	if mm.lastPrice != 0 {
		change := abs(refPrice - mm.lastPrice)
		if change > mm.volatilityThreshold {
			// Pull quotes!
			for _, orderID := range mm.orderIDs {
				mm.book.Cancel(orderID)
			}
			mm.orderIDs = nil
			mm.pulledUntil = time.Now().Add(mm.pullDuration)
			mm.lastPrice = refPrice
			return
		}
	}
	mm.lastPrice = refPrice

	// Normal quoting
	mm.MarketMakerBot.requote()
}

// Helper functions

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func average(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func absFloat(x float64) float64 {
	return math.Abs(x)
}
