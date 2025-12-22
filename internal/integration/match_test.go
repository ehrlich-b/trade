package integration

import (
	"testing"
	"time"

	"trade/internal/bots"
	"trade/internal/historical"
	"trade/internal/match"
	"trade/internal/orderbook"
)

// TestFullMatchSimulation runs a complete match with bots trading
func TestFullMatchSimulation(t *testing.T) {
	// Create order book
	book := orderbook.New("SPY")

	// Create a realistic test day with price movement
	bars := generateRealisticBars(390, 48000, 200)
	day := &historical.TradingDay{
		Symbol: "SPY",
		Date:   time.Now(),
		Bars:   bars,
	}

	// Create match with SHORT duration for testing
	config := match.MatchConfig{
		Duration:    historical.Match10Min,
		Symbol:      "SPY",
		TargetNAV:   48000,
		PreMatchSec: 1,
		StartValue:  100000000,
	}
	m := match.NewMatch(config)
	m.SetDay(day.Normalize(48000))

	// Create price feed
	priceFeed := match.NewPriceFeed(m, book)

	// Create minimal bot ecosystem
	botManager := bots.CreateMinimalEcosystem(book, priceFeed, 10*time.Second)

	// We'll check trades at the end using RecentTrades

	// Add participants
	m.Join("player1", "acc1")
	m.Join("player2", "acc2")

	p1 := m.GetParticipant("player1")
	t.Logf("Player1: %d shares + $%.2f cash", p1.StartingShares, float64(p1.StartingCash)/100)

	// Run match
	m.TransitionToPreMatch()
	time.Sleep(1500 * time.Millisecond)
	m.Start()
	priceFeed.Start()
	botManager.StartAll()

	time.Sleep(2 * time.Second)

	botManager.StopAll()
	priceFeed.Stop()
	m.Stop()

	trades := book.RecentTrades(1000)
	t.Logf("Trades: %d, Final NAV: $%.2f", len(trades), float64(priceFeed.TrueNAV())/100)

	if len(trades) == 0 {
		t.Error("no trades occurred")
	}

	snap := book.Snapshot()
	t.Logf("Book: %d bids, %d asks", len(snap.Bids), len(snap.Asks))
}

// TestBotsGenerateVolume verifies bot ecosystem creates activity
func TestBotsGenerateVolume(t *testing.T) {
	book := orderbook.New("SPY")
	bars := generateRealisticBars(100, 10000, 50)
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}

	config := match.DefaultConfig()
	config.TargetNAV = 10000
	m := match.NewMatch(config)
	m.SetDay(day.Normalize(10000))

	priceFeed := match.NewPriceFeed(m, book)
	botManager := bots.CreateEcosystem(book, priceFeed, 30*time.Second)

	t.Logf("Bot count: %d", botManager.Count())

	m.TransitionToPreMatch()
	m.Start()
	priceFeed.Start()
	botManager.StartAll()

	// Run for 6 seconds to allow bots with 3-5 second intervals to trade
	time.Sleep(6 * time.Second)

	botManager.StopAll()
	priceFeed.Stop()
	m.Stop()

	trades := book.RecentTrades(1000)
	var volume int64
	for _, trade := range trades {
		volume += trade.Quantity
	}

	t.Logf("Trades: %d, Volume: %d shares", len(trades), volume)

	// With 24 bots over 6 seconds, expect reasonable activity
	if len(trades) < 5 {
		t.Errorf("expected >=5 trades, got %d", len(trades))
	}
}

// TestPriceFeedTracksNAV verifies price feed updates
func TestPriceFeedTracksNAV(t *testing.T) {
	book := orderbook.New("SPY")

	// Clear uptrend
	bars := make([]historical.MinuteBar, 100)
	for i := range bars {
		price := int64(10000 + i*10)
		bars[i] = historical.MinuteBar{Open: price, High: price + 5, Low: price - 5, Close: price + 5}
	}
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}

	config := match.DefaultConfig()
	config.TargetNAV = 10000
	m := match.NewMatch(config)
	m.SetDay(day.Normalize(10000))

	priceFeed := match.NewPriceFeed(m, book)

	tickCount := 0
	ch := priceFeed.Subscribe()
	go func() {
		for range ch {
			tickCount++
		}
	}()

	m.TransitionToPreMatch()
	m.Start()
	priceFeed.Start()

	time.Sleep(500 * time.Millisecond)

	priceFeed.Stop()
	m.Stop()

	t.Logf("Price ticks received: %d, Final NAV: %d", tickCount, priceFeed.TrueNAV())

	if tickCount == 0 {
		t.Error("expected price tick updates")
	}
}

func generateRealisticBars(count int, startPrice, volatility int64) []historical.MinuteBar {
	bars := make([]historical.MinuteBar, count)
	price := startPrice

	for i := range bars {
		change := (int64(i%7) - 3) * (volatility / 10)
		if price > startPrice+1000 {
			change -= 10
		} else if price < startPrice-1000 {
			change += 10
		}

		open := price
		close := price + change
		high := maxInt(open, close) + volatility/5
		low := minInt(open, close) - volatility/5

		bars[i] = historical.MinuteBar{
			Open: open, High: high, Low: low, Close: close,
			Volume: 1000 + int64(i%10)*100,
		}
		price = close
	}
	return bars
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
