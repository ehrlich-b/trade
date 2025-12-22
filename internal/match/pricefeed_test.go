package match

import (
	"testing"
	"time"

	"trade/internal/historical"
	"trade/internal/orderbook"
)

func TestPriceFeed(t *testing.T) {
	// Create a match
	config := DefaultConfig()
	match := NewMatch(config)

	// Create a day
	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{
			Open:  48000,
			High:  48100,
			Low:   47900,
			Close: 48050,
		}
	}
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}
	match.SetDay(day.Normalize(48000))

	// Create order book
	book := orderbook.New("SPY")

	// Create price feed
	pf := NewPriceFeed(match, book)

	// Initial state
	if pf.TrueNAV() != 0 {
		t.Errorf("expected initial TrueNAV=0, got %d", pf.TrueNAV())
	}

	// Simulate a price update
	pf.updatePrices(48000)

	if pf.TrueNAV() != 48000 {
		t.Errorf("expected TrueNAV=48000, got %d", pf.TrueNAV())
	}

	// MM reference should be within fuzz range
	mmRef := pf.MMReference()
	if mmRef < 47990 || mmRef > 48010 {
		t.Errorf("MM reference %d outside expected fuzz range [47990, 48010]", mmRef)
	}
}

func TestPriceFeedSubscription(t *testing.T) {
	config := DefaultConfig()
	match := NewMatch(config)

	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{Open: 48000, High: 48100, Low: 47900, Close: 48050}
	}
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}
	match.SetDay(day.Normalize(48000))

	book := orderbook.New("SPY")
	pf := NewPriceFeed(match, book)

	// Subscribe
	ch := pf.Subscribe()

	// Update prices
	go pf.updatePrices(48000)

	// Should receive tick
	select {
	case tick := <-ch:
		if tick.TrueNAV != 48000 {
			t.Errorf("expected TrueNAV=48000, got %d", tick.TrueNAV)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for price tick")
	}

	// Unsubscribe
	pf.Unsubscribe(ch)
}

func TestRedemptionEngine(t *testing.T) {
	re := NewRedemptionEngine(15) // 15 min match
	re.SetNAV(10000) // $100

	// Initial fee should be base fee (0.5%)
	fee := re.CurrentFee()
	if fee < 0.004 || fee > 0.006 {
		t.Errorf("expected initial fee ~0.005, got %f", fee)
	}

	// Creation price should be NAV + fee
	createPrice := re.CreationPrice()
	expectedCreate := int64(10000 * 1.005) // ~$100.50
	if createPrice < expectedCreate-5 || createPrice > expectedCreate+5 {
		t.Errorf("expected creation price ~%d, got %d", expectedCreate, createPrice)
	}

	// Redemption price should be NAV - fee
	redeemPrice := re.RedemptionPrice()
	expectedRedeem := int64(10000 * 0.995) // ~$99.50
	if redeemPrice < expectedRedeem-5 || redeemPrice > expectedRedeem+5 {
		t.Errorf("expected redemption price ~%d, got %d", expectedRedeem, redeemPrice)
	}
}

func TestRedemptionEngineFeeIncrease(t *testing.T) {
	re := NewRedemptionEngine(15)
	re.SetNAV(10000)

	initialFee := re.CurrentFee()

	// Create a large volume
	re.Create(50000)

	// Fee should have increased
	newFee := re.CurrentFee()
	if newFee <= initialFee {
		t.Errorf("expected fee to increase after volume, was %f, now %f", initialFee, newFee)
	}

	// Fee should be higher but below max
	if newFee >= MaxFee {
		t.Errorf("fee %f should be below max %f", newFee, MaxFee)
	}
}

func TestRedemptionEngineStatus(t *testing.T) {
	re := NewRedemptionEngine(15)
	re.SetNAV(10000)

	status := re.Status()

	if status.NAV != 10000 {
		t.Errorf("expected NAV=10000, got %d", status.NAV)
	}

	if status.CreationPrice <= status.NAV {
		t.Error("creation price should be above NAV")
	}

	if status.RedemptionPrice >= status.NAV {
		t.Error("redemption price should be below NAV")
	}
}

func TestBookMidFallback(t *testing.T) {
	config := DefaultConfig()
	match := NewMatch(config)

	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{Open: 48000, High: 48100, Low: 47900, Close: 48050}
	}
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}
	match.SetDay(day.Normalize(48000))

	book := orderbook.New("SPY")
	pf := NewPriceFeed(match, book)

	// Update with NAV, empty book
	pf.updatePrices(48000)

	// Book mid should fall back to NAV when book is empty
	if pf.BookMid() != 48000 {
		t.Errorf("expected BookMid to fall back to NAV 48000, got %d", pf.BookMid())
	}
}
