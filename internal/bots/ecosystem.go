package bots

import (
	"fmt"
	"time"

	"trade/internal/match"
	"trade/internal/orderbook"
)

// CreateEcosystem creates the full bot ecosystem for a match
// Returns a BotManager with all bots configured and ready to start
func CreateEcosystem(book *orderbook.OrderBook, priceFeed *match.PriceFeed, matchDuration time.Duration) *BotManager {
	manager := NewBotManager()

	// Market Makers (4 bots)
	manager.AddBot(NewTightMM(book, priceFeed))
	manager.AddBot(NewWideMM(book, priceFeed))
	manager.AddBot(NewAdaptiveMM(book, priceFeed))
	manager.AddBot(NewNervousMM(book, priceFeed))

	// Momentum traders (4 bots - 2 fast, 2 slow)
	manager.AddBot(NewMomentumFast("momentum_fast_1", book, priceFeed))
	manager.AddBot(NewMomentumFast("momentum_fast_2", book, priceFeed))
	manager.AddBot(NewMomentumSlow("momentum_slow_1", book, priceFeed))
	manager.AddBot(NewMomentumSlow("momentum_slow_2", book, priceFeed))

	// Mean reversion traders (2 bots)
	manager.AddBot(NewMeanReversionStandard("mean_reversion_1", book, priceFeed))
	manager.AddBot(NewMeanReversionStandard("mean_reversion_2", book, priceFeed))

	// Breakout traders (2 bots)
	manager.AddBot(NewBreakoutStandard("breakout_1", book, priceFeed))
	manager.AddBot(NewBreakoutStandard("breakout_2", book, priceFeed))

	// Noise traders - random small (4 bots)
	for i := 1; i <= 4; i++ {
		manager.AddBot(NewRandomSmall(fmt.Sprintf("noise_small_%d", i), book, priceFeed))
	}

	// Noise traders - random large (2 bots)
	manager.AddBot(NewRandomLarge("noise_large_1", book, priceFeed))
	manager.AddBot(NewRandomLarge("noise_large_2", book, priceFeed))

	// Panic traders (2 bots)
	manager.AddBot(NewPanicStandard("panic_1", book, priceFeed))
	manager.AddBot(NewPanicStandard("panic_2", book, priceFeed))

	// Mandated agents - creates natural flow
	// Buy side
	manager.AddBot(NewTWAPBuyer("twap_buyer_1", 5000, matchDuration, book, priceFeed))
	manager.AddBot(NewOpportunisticBuyer("opp_buyer_1", 3000, matchDuration, book, priceFeed))

	// Sell side
	manager.AddBot(NewTWAPSeller("twap_seller_1", 5000, matchDuration, book, priceFeed))
	manager.AddBot(NewDesperateSeller("desperate_seller_1", 2000, matchDuration, book, priceFeed))

	return manager
}

// CreateMinimalEcosystem creates a smaller bot ecosystem for testing
func CreateMinimalEcosystem(book *orderbook.OrderBook, priceFeed *match.PriceFeed, matchDuration time.Duration) *BotManager {
	manager := NewBotManager()

	// Just essential market makers
	manager.AddBot(NewTightMM(book, priceFeed))
	manager.AddBot(NewWideMM(book, priceFeed))

	// One of each type
	manager.AddBot(NewMomentumFast("momentum_1", book, priceFeed))
	manager.AddBot(NewMeanReversionStandard("mean_reversion_1", book, priceFeed))
	manager.AddBot(NewRandomSmall("noise_1", book, priceFeed))

	// One buyer, one seller
	manager.AddBot(NewTWAPBuyer("buyer_1", 2000, matchDuration, book, priceFeed))
	manager.AddBot(NewTWAPSeller("seller_1", 2000, matchDuration, book, priceFeed))

	return manager
}

// BotStats returns statistics about a bot manager's bots
type BotStats struct {
	TotalBots      int            `json:"total_bots"`
	MarketMakers   int            `json:"market_makers"`
	Directional    int            `json:"directional"`
	NoiseTraders   int            `json:"noise_traders"`
	MandatedAgents int            `json:"mandated_agents"`
	BotIDs         []string       `json:"bot_ids"`
}

// Stats returns statistics about the bot ecosystem
func (m *BotManager) Stats() BotStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := BotStats{
		TotalBots: len(m.bots),
		BotIDs:    make([]string, len(m.bots)),
	}

	for i, bot := range m.bots {
		stats.BotIDs[i] = bot.ID()

		// Categorize by ID prefix
		id := bot.ID()
		switch {
		case len(id) >= 3 && id[:3] == "mm_":
			stats.MarketMakers++
		case len(id) >= 8 && id[:8] == "momentum" ||
			len(id) >= 4 && id[:4] == "mean" ||
			len(id) >= 8 && id[:8] == "breakout":
			stats.Directional++
		case len(id) >= 5 && id[:5] == "noise" ||
			len(id) >= 5 && id[:5] == "panic":
			stats.NoiseTraders++
		case len(id) >= 4 && id[:4] == "twap" ||
			len(id) >= 3 && id[:3] == "opp" ||
			len(id) >= 9 && id[:9] == "desperate":
			stats.MandatedAgents++
		}
	}

	return stats
}
