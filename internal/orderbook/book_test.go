package orderbook

import (
	"testing"
)

func TestLimitOrderAddToBook(t *testing.T) {
	book := New("FAKE")

	order := &Order{
		ID:       "order1",
		UserID:   "user1",
		Side:     Buy,
		Type:     Limit,
		Price:    10000, // $100.00
		Quantity: 10,
	}

	trades, err := book.Submit(order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 0 {
		t.Errorf("expected 0 trades, got %d", len(trades))
	}

	snap := book.Snapshot()
	if len(snap.Bids) != 1 {
		t.Fatalf("expected 1 bid level, got %d", len(snap.Bids))
	}
	if snap.Bids[0].Price != 10000 {
		t.Errorf("expected bid price 10000, got %d", snap.Bids[0].Price)
	}
	if snap.Bids[0].Quantity != 10 {
		t.Errorf("expected bid quantity 10, got %d", snap.Bids[0].Quantity)
	}
}

func TestLimitOrderMatching(t *testing.T) {
	book := New("FAKE")

	// Place a sell order
	sell := &Order{
		ID:       "sell1",
		UserID:   "seller",
		Side:     Sell,
		Type:     Limit,
		Price:    10000,
		Quantity: 10,
	}
	book.Submit(sell)

	// Place a matching buy order
	buy := &Order{
		ID:       "buy1",
		UserID:   "buyer",
		Side:     Buy,
		Type:     Limit,
		Price:    10000,
		Quantity: 10,
	}
	trades, err := book.Submit(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	trade := trades[0]
	if trade.Price != 10000 {
		t.Errorf("expected trade price 10000, got %d", trade.Price)
	}
	if trade.Quantity != 10 {
		t.Errorf("expected trade quantity 10, got %d", trade.Quantity)
	}
	if trade.BuyerID != "buyer" {
		t.Errorf("expected buyer 'buyer', got %s", trade.BuyerID)
	}
	if trade.SellerID != "seller" {
		t.Errorf("expected seller 'seller', got %s", trade.SellerID)
	}

	// Book should be empty
	snap := book.Snapshot()
	if len(snap.Bids) != 0 || len(snap.Asks) != 0 {
		t.Errorf("expected empty book, got %d bids and %d asks", len(snap.Bids), len(snap.Asks))
	}
}

func TestPartialFill(t *testing.T) {
	book := New("FAKE")

	// Sell 20 shares
	sell := &Order{
		ID:       "sell1",
		UserID:   "seller",
		Side:     Sell,
		Type:     Limit,
		Price:    10000,
		Quantity: 20,
	}
	book.Submit(sell)

	// Buy only 10 shares
	buy := &Order{
		ID:       "buy1",
		UserID:   "buyer",
		Side:     Buy,
		Type:     Limit,
		Price:    10000,
		Quantity: 10,
	}
	trades, _ := book.Submit(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 10 {
		t.Errorf("expected trade quantity 10, got %d", trades[0].Quantity)
	}

	// 10 shares should remain on the ask
	snap := book.Snapshot()
	if len(snap.Asks) != 1 {
		t.Fatalf("expected 1 ask level, got %d", len(snap.Asks))
	}
	if snap.Asks[0].Quantity != 10 {
		t.Errorf("expected 10 remaining, got %d", snap.Asks[0].Quantity)
	}
}

func TestPriceTimePriority(t *testing.T) {
	book := New("FAKE")

	// Two sells at same price - first should match first
	sell1 := &Order{ID: "sell1", UserID: "seller1", Side: Sell, Type: Limit, Price: 10000, Quantity: 10}
	sell2 := &Order{ID: "sell2", UserID: "seller2", Side: Sell, Type: Limit, Price: 10000, Quantity: 10}
	book.Submit(sell1)
	book.Submit(sell2)

	// Buy 10 - should match sell1
	buy := &Order{ID: "buy1", UserID: "buyer", Side: Buy, Type: Limit, Price: 10000, Quantity: 10}
	trades, _ := book.Submit(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].SellerID != "seller1" {
		t.Errorf("expected seller1 to match first, got %s", trades[0].SellerID)
	}

	// sell2 should still be on book
	snap := book.Snapshot()
	if len(snap.Asks) != 1 || snap.Asks[0].Quantity != 10 {
		t.Errorf("expected sell2 remaining on book")
	}
}

func TestPricePriority(t *testing.T) {
	book := New("FAKE")

	// Better price should match first
	sell1 := &Order{ID: "sell1", UserID: "expensive", Side: Sell, Type: Limit, Price: 10100, Quantity: 10}
	sell2 := &Order{ID: "sell2", UserID: "cheap", Side: Sell, Type: Limit, Price: 10000, Quantity: 10}
	book.Submit(sell1)
	book.Submit(sell2)

	// Buy at 10100 - should match cheaper sell2 first
	buy := &Order{ID: "buy1", UserID: "buyer", Side: Buy, Type: Limit, Price: 10100, Quantity: 10}
	trades, _ := book.Submit(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Price != 10000 {
		t.Errorf("expected trade at 10000, got %d", trades[0].Price)
	}
	if trades[0].SellerID != "cheap" {
		t.Errorf("expected cheap seller to match, got %s", trades[0].SellerID)
	}
}

func TestMarketOrder(t *testing.T) {
	book := New("FAKE")

	// Place some asks
	book.Submit(&Order{ID: "sell1", UserID: "seller1", Side: Sell, Type: Limit, Price: 10000, Quantity: 10})
	book.Submit(&Order{ID: "sell2", UserID: "seller2", Side: Sell, Type: Limit, Price: 10100, Quantity: 10})

	// Market buy 15 shares - should sweep through both levels
	buy := &Order{ID: "buy1", UserID: "buyer", Side: Buy, Type: Market, Quantity: 15}
	trades, _ := book.Submit(buy)

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if trades[0].Quantity != 10 || trades[0].Price != 10000 {
		t.Errorf("first trade wrong: qty=%d price=%d", trades[0].Quantity, trades[0].Price)
	}
	if trades[1].Quantity != 5 || trades[1].Price != 10100 {
		t.Errorf("second trade wrong: qty=%d price=%d", trades[1].Quantity, trades[1].Price)
	}

	// 5 shares should remain at 10100
	snap := book.Snapshot()
	if len(snap.Asks) != 1 || snap.Asks[0].Quantity != 5 {
		t.Errorf("expected 5 remaining at 10100")
	}
}

func TestCancelOrder(t *testing.T) {
	book := New("FAKE")

	order := &Order{ID: "order1", UserID: "user1", Side: Buy, Type: Limit, Price: 10000, Quantity: 10}
	book.Submit(order)

	err := book.Cancel("order1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := book.Snapshot()
	if len(snap.Bids) != 0 {
		t.Errorf("expected empty bids after cancel")
	}

	// Cancel again should error
	err = book.Cancel("order1")
	if err == nil {
		t.Error("expected error canceling non-existent order")
	}
}

func TestBestBidAsk(t *testing.T) {
	book := New("FAKE")

	if book.BestBid() != 0 || book.BestAsk() != 0 {
		t.Error("expected 0 for empty book")
	}

	book.Submit(&Order{ID: "bid1", Side: Buy, Type: Limit, Price: 9900, Quantity: 10})
	book.Submit(&Order{ID: "bid2", Side: Buy, Type: Limit, Price: 10000, Quantity: 10})
	book.Submit(&Order{ID: "ask1", Side: Sell, Type: Limit, Price: 10100, Quantity: 10})
	book.Submit(&Order{ID: "ask2", Side: Sell, Type: Limit, Price: 10200, Quantity: 10})

	if book.BestBid() != 10000 {
		t.Errorf("expected best bid 10000, got %d", book.BestBid())
	}
	if book.BestAsk() != 10100 {
		t.Errorf("expected best ask 10100, got %d", book.BestAsk())
	}
	if book.MidPrice() != 10050 {
		t.Errorf("expected mid 10050, got %d", book.MidPrice())
	}
}

func TestRecentTrades(t *testing.T) {
	book := New("FAKE")

	// Create some trades
	book.Submit(&Order{ID: "sell1", UserID: "seller", Side: Sell, Type: Limit, Price: 10000, Quantity: 30})
	book.Submit(&Order{ID: "buy1", UserID: "buyer1", Side: Buy, Type: Limit, Price: 10000, Quantity: 10})
	book.Submit(&Order{ID: "buy2", UserID: "buyer2", Side: Buy, Type: Limit, Price: 10000, Quantity: 10})
	book.Submit(&Order{ID: "buy3", UserID: "buyer3", Side: Buy, Type: Limit, Price: 10000, Quantity: 10})

	trades := book.RecentTrades(2)
	if len(trades) != 2 {
		t.Fatalf("expected 2 recent trades, got %d", len(trades))
	}
	// Should be most recent last
	if trades[0].BuyerID != "buyer2" || trades[1].BuyerID != "buyer3" {
		t.Errorf("unexpected trade order")
	}
}

func TestNoSelfTrade(t *testing.T) {
	// This tests that orders from same user can technically match
	// In a real system you might want to prevent this, but for now we allow it
	book := New("FAKE")

	book.Submit(&Order{ID: "sell1", UserID: "user1", Side: Sell, Type: Limit, Price: 10000, Quantity: 10})
	trades, _ := book.Submit(&Order{ID: "buy1", UserID: "user1", Side: Buy, Type: Limit, Price: 10000, Quantity: 10})

	// Currently allows self-trade - this is a design decision
	if len(trades) != 1 {
		t.Errorf("expected trade (self-trade allowed), got %d trades", len(trades))
	}
}
