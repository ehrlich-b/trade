package bots

import (
	"testing"
	"time"

	"trade/internal/historical"
	"trade/internal/match"
	"trade/internal/orderbook"
)

func setupTestEnv() (*orderbook.OrderBook, *match.PriceFeed, *match.Match) {
	config := match.DefaultConfig()
	config.TargetNAV = 10000 // $100

	m := match.NewMatch(config)

	// Create a fake day
	bars := make([]historical.MinuteBar, 390)
	for i := range bars {
		bars[i] = historical.MinuteBar{
			Open:  10000,
			High:  10100,
			Low:   9900,
			Close: 10050,
		}
	}
	day := &historical.TradingDay{Symbol: "SPY", Date: time.Now(), Bars: bars}
	m.SetDay(day.Normalize(10000))

	book := orderbook.New("SPY")
	priceFeed := match.NewPriceFeed(m, book)

	// Initialize price feed with a value
	priceFeed.SetMMFuzz(10)

	return book, priceFeed, m
}

func TestMarketMakerBot(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	mm := NewTightMM(book, priceFeed)

	if mm.ID() != "mm_tight" {
		t.Errorf("expected ID 'mm_tight', got '%s'", mm.ID())
	}

	// Check initial state
	if mm.Position() != 0 {
		t.Errorf("expected initial position 0, got %d", mm.Position())
	}
}

func TestNoiseTrader(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	noise := NewRandomSmall("noise_test", book, priceFeed)

	if noise.ID() != "noise_test" {
		t.Errorf("expected ID 'noise_test', got '%s'", noise.ID())
	}
}

func TestMomentumBot(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	mom := NewMomentumFast("mom_test", book, priceFeed)

	if mom.ID() != "mom_test" {
		t.Errorf("expected ID 'mom_test', got '%s'", mom.ID())
	}
}

func TestMeanReversionBot(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	mr := NewMeanReversionStandard("mr_test", book, priceFeed)

	if mr.ID() != "mr_test" {
		t.Errorf("expected ID 'mr_test', got '%s'", mr.ID())
	}
}

func TestBreakoutBot(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	bo := NewBreakoutStandard("bo_test", book, priceFeed)

	if bo.ID() != "bo_test" {
		t.Errorf("expected ID 'bo_test', got '%s'", bo.ID())
	}
}

func TestMandatedAgent(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	agent := NewTWAPBuyer("agent_test", 1000, 10*time.Minute, book, priceFeed)

	if agent.ID() != "agent_test" {
		t.Errorf("expected ID 'agent_test', got '%s'", agent.ID())
	}

	// Initial progress should be 0
	if agent.Progress() != 0 {
		t.Errorf("expected initial progress 0, got %f", agent.Progress())
	}
}

func TestBotManager(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	manager := NewBotManager()

	manager.AddBot(NewTightMM(book, priceFeed))
	manager.AddBot(NewRandomSmall("noise_1", book, priceFeed))

	if manager.Count() != 2 {
		t.Errorf("expected 2 bots, got %d", manager.Count())
	}
}

func TestCreateEcosystem(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	manager := CreateEcosystem(book, priceFeed, 15*time.Minute)

	// Should have 24 bots total
	// 4 MMs + 4 momentum + 2 mean reversion + 2 breakout + 6 noise + 2 panic + 4 mandated = 24
	expectedBots := 24
	if manager.Count() != expectedBots {
		t.Errorf("expected %d bots, got %d", expectedBots, manager.Count())
	}

	stats := manager.Stats()
	if stats.MarketMakers != 4 {
		t.Errorf("expected 4 market makers, got %d", stats.MarketMakers)
	}
}

func TestCreateMinimalEcosystem(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	manager := CreateMinimalEcosystem(book, priceFeed, 15*time.Minute)

	// Should have 7 bots
	expectedBots := 7
	if manager.Count() != expectedBots {
		t.Errorf("expected %d bots, got %d", expectedBots, manager.Count())
	}
}

func TestPositionTracking(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	bot := NewBaseBot("test_bot", book, priceFeed)

	// Simulate a buy trade
	trade := orderbook.Trade{
		BuyerID:  "test_bot",
		SellerID: "someone_else",
		Price:    10000,
		Quantity: 100,
	}

	bot.ProcessTrade(trade)

	if bot.Position() != 100 {
		t.Errorf("expected position 100 after buy, got %d", bot.Position())
	}

	// Simulate a sell trade
	trade2 := orderbook.Trade{
		BuyerID:  "someone_else",
		SellerID: "test_bot",
		Price:    10100,
		Quantity: 50,
	}

	bot.ProcessTrade(trade2)

	if bot.Position() != 50 {
		t.Errorf("expected position 50 after partial sell, got %d", bot.Position())
	}
}

func TestBotEcosystemStats(t *testing.T) {
	book, priceFeed, _ := setupTestEnv()

	manager := CreateEcosystem(book, priceFeed, 15*time.Minute)
	stats := manager.Stats()

	if stats.TotalBots != 24 {
		t.Errorf("expected 24 total bots, got %d", stats.TotalBots)
	}

	if len(stats.BotIDs) != 24 {
		t.Errorf("expected 24 bot IDs, got %d", len(stats.BotIDs))
	}
}
