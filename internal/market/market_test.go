package market

import (
	"testing"
	"time"

	"trade/internal/orderbook"
)

// ==================== PRICE GENERATOR TESTS ====================

func TestNewPriceGenerator(t *testing.T) {
	pg := NewPriceGenerator(10000, 50.0) // $100, volatility 50 cents

	if pg.Price() != 10000 {
		t.Errorf("expected initial price 10000, got %d", pg.Price())
	}
}

func TestPriceGenerator_Price(t *testing.T) {
	pg := NewPriceGenerator(5000, 10.0)

	price := pg.Price()
	if price != 5000 {
		t.Errorf("expected price 5000, got %d", price)
	}
}

func TestPriceGenerator_Tick(t *testing.T) {
	pg := NewPriceGenerator(10000, 100.0) // High volatility to ensure movement

	// Run several ticks
	initialPrice := pg.Price()
	changed := false
	for i := 0; i < 100; i++ {
		pg.tick()
		if pg.Price() != initialPrice {
			changed = true
			break
		}
	}

	// With high volatility, price should change
	if !changed {
		t.Error("expected price to change after multiple ticks with high volatility")
	}
}

func TestPriceGenerator_BoundsMin(t *testing.T) {
	pg := NewPriceGenerator(200, 0) // Start at $2
	pg.drift = -1000                // Strong downward drift

	// Run many ticks to push price down
	for i := 0; i < 100; i++ {
		pg.tick()
	}

	// Price should not go below minPrice (100 cents = $1)
	if pg.Price() < 100 {
		t.Errorf("price %d should not be below minimum 100", pg.Price())
	}
}

func TestPriceGenerator_BoundsMax(t *testing.T) {
	pg := NewPriceGenerator(99000, 0) // Start at $990
	pg.drift = 1000                   // Strong upward drift

	// Run many ticks to push price up
	for i := 0; i < 100; i++ {
		pg.tick()
	}

	// Price should not go above maxPrice (100000 cents = $1000)
	if pg.Price() > 100000 {
		t.Errorf("price %d should not be above maximum 100000", pg.Price())
	}
}

func TestPriceGenerator_Subscribe(t *testing.T) {
	pg := NewPriceGenerator(10000, 50.0)

	ch := pg.Subscribe()
	if ch == nil {
		t.Fatal("subscribe returned nil channel")
	}

	// Trigger a tick
	pg.tick()

	// Should receive update
	select {
	case price := <-ch:
		if price == 0 {
			t.Error("received zero price")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for price update")
	}

	pg.Unsubscribe(ch)
}

func TestPriceGenerator_Unsubscribe(t *testing.T) {
	pg := NewPriceGenerator(10000, 50.0)

	ch := pg.Subscribe()
	pg.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestPriceGenerator_SetVolatility(t *testing.T) {
	pg := NewPriceGenerator(10000, 50.0)

	pg.SetVolatility(100.0)

	pg.mu.RLock()
	if pg.volatility != 100.0 {
		t.Errorf("expected volatility 100.0, got %f", pg.volatility)
	}
	pg.mu.RUnlock()
}

func TestPriceGenerator_SetDrift(t *testing.T) {
	pg := NewPriceGenerator(10000, 50.0)

	pg.SetDrift(5.0)

	pg.mu.RLock()
	if pg.drift != 5.0 {
		t.Errorf("expected drift 5.0, got %f", pg.drift)
	}
	pg.mu.RUnlock()
}

func TestPriceGenerator_StartStop(t *testing.T) {
	pg := NewPriceGenerator(10000, 50.0)

	pg.Start(10 * time.Millisecond)

	// Wait for a few ticks
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	pg.Stop()

	// Give goroutine time to stop
	time.Sleep(20 * time.Millisecond)
}

func TestPriceGenerator_ZeroVolatility(t *testing.T) {
	pg := NewPriceGenerator(10000, 0) // Zero volatility
	pg.drift = 0                      // Zero drift

	initialPrice := pg.Price()
	for i := 0; i < 10; i++ {
		pg.tick()
	}

	// Price should not change with zero volatility and zero drift
	if pg.Price() != initialPrice {
		t.Errorf("expected price to remain %d with zero volatility, got %d", initialPrice, pg.Price())
	}
}

// ==================== MARKET MAKER TESTS ====================

func TestNewMarketMaker(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 50.0)

	mm := NewMarketMaker(book, pg)

	if mm.userID != "market_maker" {
		t.Errorf("expected userID 'market_maker', got '%s'", mm.userID)
	}
	if mm.spread != 10 {
		t.Errorf("expected spread 10, got %d", mm.spread)
	}
	if mm.levels != 5 {
		t.Errorf("expected levels 5, got %d", mm.levels)
	}
	if mm.sizePerLevel != 100 {
		t.Errorf("expected sizePerLevel 100, got %d", mm.sizePerLevel)
	}
}

func TestMarketMaker_SetSpread(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 50.0)
	mm := NewMarketMaker(book, pg)

	mm.SetSpread(25)

	mm.mu.Lock()
	if mm.spread != 25 {
		t.Errorf("expected spread 25, got %d", mm.spread)
	}
	mm.mu.Unlock()
}

func TestMarketMaker_SetLevels(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 50.0)
	mm := NewMarketMaker(book, pg)

	mm.SetLevels(10)

	mm.mu.Lock()
	if mm.levels != 10 {
		t.Errorf("expected levels 10, got %d", mm.levels)
	}
	mm.mu.Unlock()
}

func TestMarketMaker_SetSizePerLevel(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 50.0)
	mm := NewMarketMaker(book, pg)

	mm.SetSizePerLevel(500)

	mm.mu.Lock()
	if mm.sizePerLevel != 500 {
		t.Errorf("expected sizePerLevel 500, got %d", mm.sizePerLevel)
	}
	mm.mu.Unlock()
}

func TestMarketMaker_Requote(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0) // $100, no volatility
	mm := NewMarketMaker(book, pg)

	// Set known parameters
	mm.SetSpread(100)      // $1 spread
	mm.SetLevels(3)        // 3 levels each side
	mm.SetSizePerLevel(50) // 50 shares per level

	// Trigger requote
	mm.requote()

	// Check book state
	snap := book.Snapshot()

	// Should have 3 bid levels and 3 ask levels
	if len(snap.Bids) != 3 {
		t.Errorf("expected 3 bid levels, got %d", len(snap.Bids))
	}
	if len(snap.Asks) != 3 {
		t.Errorf("expected 3 ask levels, got %d", len(snap.Asks))
	}

	// Check bid prices (should be mid - spread*i)
	// Mid = 10000, spread = 100
	expectedBids := []int64{9900, 9800, 9700} // 10000 - 100*1, 10000 - 100*2, 10000 - 100*3
	for i, level := range snap.Bids {
		if level.Price != expectedBids[i] {
			t.Errorf("bid level %d: expected price %d, got %d", i, expectedBids[i], level.Price)
		}
		if level.Quantity != 50 {
			t.Errorf("bid level %d: expected quantity 50, got %d", i, level.Quantity)
		}
	}

	// Check ask prices (should be mid + spread*i)
	expectedAsks := []int64{10100, 10200, 10300} // 10000 + 100*1, 10000 + 100*2, 10000 + 100*3
	for i, level := range snap.Asks {
		if level.Price != expectedAsks[i] {
			t.Errorf("ask level %d: expected price %d, got %d", i, expectedAsks[i], level.Price)
		}
		if level.Quantity != 50 {
			t.Errorf("ask level %d: expected quantity 50, got %d", i, level.Quantity)
		}
	}
}

func TestMarketMaker_RequoteUpdatesOrders(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0)
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(2)

	// First requote
	mm.requote()
	snap1 := book.Snapshot()

	// Change price
	pg.mu.Lock()
	pg.price = 11000 // Move up $10
	pg.mu.Unlock()

	// Requote again
	mm.requote()
	snap2 := book.Snapshot()

	// Orders should have moved up
	if snap1.Bids[0].Price >= snap2.Bids[0].Price {
		t.Error("expected bid prices to increase after price moved up")
	}
	if snap1.Asks[0].Price >= snap2.Asks[0].Price {
		t.Error("expected ask prices to increase after price moved up")
	}
}

func TestMarketMaker_OnUpdate(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0)
	mm := NewMarketMaker(book, pg)

	called := false
	mm.SetOnUpdate(func() {
		called = true
	})

	mm.requote()

	if !called {
		t.Error("expected onUpdate callback to be called")
	}
}

func TestMarketMaker_StartStop(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 50.0)
	pg.Start(100 * time.Millisecond)
	defer pg.Stop()

	mm := NewMarketMaker(book, pg)
	mm.SetLevels(2)

	mm.Start()

	// Wait for initial quote
	time.Sleep(50 * time.Millisecond)

	// Should have orders in the book
	snap := book.Snapshot()
	if len(snap.Bids) == 0 {
		t.Error("expected bids after start")
	}
	if len(snap.Asks) == 0 {
		t.Error("expected asks after start")
	}

	mm.Stop()

	// Wait for stop to process
	time.Sleep(50 * time.Millisecond)

	// Orders should be cancelled
	snap = book.Snapshot()
	if len(snap.Bids) != 0 {
		t.Errorf("expected no bids after stop, got %d", len(snap.Bids))
	}
	if len(snap.Asks) != 0 {
		t.Errorf("expected no asks after stop, got %d", len(snap.Asks))
	}
}

func TestMarketMaker_ZeroPrice(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(0, 0) // Zero price
	mm := NewMarketMaker(book, pg)

	// Should not panic with zero price
	mm.requote()

	// Should have no orders when price is zero
	snap := book.Snapshot()
	if len(snap.Bids) != 0 || len(snap.Asks) != 0 {
		t.Error("expected no orders when price is zero")
	}
}

func TestMarketMaker_OrdersFilled(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0)
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(1)
	mm.SetSizePerLevel(100)
	mm.SetSpread(100)

	// Initial quote
	mm.requote()

	// Submit a market order that will fill the MM's ask
	order := &orderbook.Order{
		UserID:   "player1",
		Side:     orderbook.Buy,
		Type:     orderbook.Market,
		Quantity: 100,
	}
	trades, _ := book.Submit(order)

	if len(trades) == 0 {
		t.Error("expected trades when buying against MM")
	}

	// Requote should work even though order was filled
	mm.requote()

	// Should have new orders
	snap := book.Snapshot()
	if len(snap.Asks) != 1 {
		t.Errorf("expected 1 ask after requote, got %d", len(snap.Asks))
	}
}

func TestMarketMaker_CancelAllOrders(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0)
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(3)

	// Quote
	mm.requote()

	// Should have orders
	snap := book.Snapshot()
	if len(snap.Bids) != 3 || len(snap.Asks) != 3 {
		t.Fatal("expected orders before cancel")
	}

	// Cancel all
	mm.cancelAllOrders()

	// Should have no orders
	snap = book.Snapshot()
	if len(snap.Bids) != 0 || len(snap.Asks) != 0 {
		t.Error("expected no orders after cancelAllOrders")
	}

	// Order IDs should be cleared
	mm.mu.Lock()
	if len(mm.orderIDs) != 0 {
		t.Errorf("expected orderIDs to be empty, got %d", len(mm.orderIDs))
	}
	mm.mu.Unlock()
}

// ==================== POSITION TRACKING TESTS ====================

func TestMarketMaker_PositionTracking_SellFilled(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0) // $100
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(1)
	mm.SetSizePerLevel(100)
	mm.SetSpread(100) // $1 spread

	// Initial state
	if mm.Position() != 0 {
		t.Errorf("expected initial position 0, got %d", mm.Position())
	}

	// Quote orders (bid at $99, ask at $101)
	mm.requote()

	// Player buys 50 shares, filling part of MM's ask
	buyOrder := &orderbook.Order{
		UserID:   "player1",
		Side:     orderbook.Buy,
		Type:     orderbook.Market,
		Quantity: 50,
	}
	trades, _ := book.Submit(buyOrder)

	// Notify MM of the trade
	for _, trade := range trades {
		mm.ProcessTrade(trade)
	}

	// MM sold 50, should be short 50
	if mm.Position() != -50 {
		t.Errorf("expected position -50 (short), got %d", mm.Position())
	}

	// Avg price should be the ask price ($101)
	if mm.AvgPrice() != 10100 {
		t.Errorf("expected avg price 10100, got %d", mm.AvgPrice())
	}
}

func TestMarketMaker_PositionTracking_BidFilled(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0) // $100
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(1)
	mm.SetSizePerLevel(100)
	mm.SetSpread(100) // $1 spread

	// Quote orders (bid at $99, ask at $101)
	mm.requote()

	// Player sells 50 shares, filling part of MM's bid
	sellOrder := &orderbook.Order{
		UserID:   "player1",
		Side:     orderbook.Sell,
		Type:     orderbook.Market,
		Quantity: 50,
	}
	trades, _ := book.Submit(sellOrder)

	// Notify MM of the trade
	for _, trade := range trades {
		mm.ProcessTrade(trade)
	}

	// MM bought 50, should be long 50
	if mm.Position() != 50 {
		t.Errorf("expected position 50 (long), got %d", mm.Position())
	}

	// Avg price should be the bid price ($99)
	if mm.AvgPrice() != 9900 {
		t.Errorf("expected avg price 9900, got %d", mm.AvgPrice())
	}
}

func TestMarketMaker_PositionTracking_RealizedPnL(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0) // $100
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(1)
	mm.SetSizePerLevel(100)
	mm.SetSpread(100)

	mm.requote()

	// Player sells to MM at $99 (MM goes long)
	sellOrder := &orderbook.Order{
		UserID:   "player1",
		Side:     orderbook.Sell,
		Type:     orderbook.Market,
		Quantity: 100,
	}
	trades, _ := book.Submit(sellOrder)
	for _, trade := range trades {
		mm.ProcessTrade(trade)
	}

	// MM is long 100 @ $99
	if mm.Position() != 100 {
		t.Errorf("expected position 100, got %d", mm.Position())
	}

	// Requote to get new orders
	mm.requote()

	// Player buys from MM at $101 (MM closes long with profit)
	buyOrder := &orderbook.Order{
		UserID:   "player1",
		Side:     orderbook.Buy,
		Type:     orderbook.Market,
		Quantity: 100,
	}
	trades, _ = book.Submit(buyOrder)
	for _, trade := range trades {
		mm.ProcessTrade(trade)
	}

	// MM closed long, should be flat
	if mm.Position() != 0 {
		t.Errorf("expected position 0 after closing, got %d", mm.Position())
	}

	// Realized P&L: bought at $99, sold at $101 = $2 profit per share
	// 100 shares * 200 cents = 20000 cents = $200
	expectedPnL := int64(100 * 200)
	if mm.RealizedPnL() != expectedPnL {
		t.Errorf("expected realized P&L %d, got %d", expectedPnL, mm.RealizedPnL())
	}
}

func TestMarketMaker_UnrealizedPnL(t *testing.T) {
	book := orderbook.New("FAKE")
	pg := NewPriceGenerator(10000, 0) // $100
	mm := NewMarketMaker(book, pg)
	mm.SetLevels(1)
	mm.SetSizePerLevel(100)
	mm.SetSpread(100)

	mm.requote()

	// Player sells to MM at $99 (MM goes long)
	sellOrder := &orderbook.Order{
		UserID:   "player1",
		Side:     orderbook.Sell,
		Type:     orderbook.Market,
		Quantity: 100,
	}
	trades, _ := book.Submit(sellOrder)
	for _, trade := range trades {
		mm.ProcessTrade(trade)
	}

	// MM is long 100 @ $99, mid price is $100
	// Unrealized P&L = 100 * ($100 - $99) = $100 = 10000 cents
	expectedUnrealized := int64(100 * 100)
	if mm.UnrealizedPnL() != expectedUnrealized {
		t.Errorf("expected unrealized P&L %d, got %d", expectedUnrealized, mm.UnrealizedPnL())
	}
}
