package bots

import (
	"sync"
	"time"

	"trade/internal/match"
	"trade/internal/orderbook"
)

// Bot is the interface all trading bots must implement
type Bot interface {
	ID() string
	Start()
	Stop()
	ProcessTrade(trade orderbook.Trade)
}

// BaseBot provides common functionality for all bots
type BaseBot struct {
	mu sync.Mutex

	id        string
	book      *orderbook.OrderBook
	priceFeed *match.PriceFeed

	position    int64 // Current position (positive = long)
	avgPrice    int64 // Average entry price
	realizedPnL int64 // Realized P&L

	stopCh chan struct{}
}

// NewBaseBot creates a new base bot
func NewBaseBot(id string, book *orderbook.OrderBook, priceFeed *match.PriceFeed) *BaseBot {
	return &BaseBot{
		id:        id,
		book:      book,
		priceFeed: priceFeed,
		stopCh:    make(chan struct{}),
	}
}

func (b *BaseBot) ID() string {
	return b.id
}

func (b *BaseBot) Stop() {
	close(b.stopCh)
}

// Position returns current position
func (b *BaseBot) Position() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.position
}

// ProcessTrade updates position based on a trade
func (b *BaseBot) ProcessTrade(trade orderbook.Trade) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var side string
	var qty int64 = trade.Quantity

	if trade.BuyerID == b.id {
		side = "buy"
	} else if trade.SellerID == b.id {
		side = "sell"
	} else {
		return // Not our trade
	}

	if side == "buy" {
		if b.position >= 0 {
			// Adding to long
			totalCost := b.avgPrice*b.position + trade.Price*qty
			b.position += qty
			if b.position > 0 {
				b.avgPrice = totalCost / b.position
			}
		} else {
			// Covering short
			coverQty := min(qty, -b.position)
			pnl := coverQty * (b.avgPrice - trade.Price)
			b.realizedPnL += pnl
			b.position += qty
			if b.position > 0 {
				b.avgPrice = trade.Price
			} else if b.position == 0 {
				b.avgPrice = 0
			}
		}
	} else {
		if b.position <= 0 {
			// Adding to short
			totalValue := b.avgPrice*(-b.position) + trade.Price*qty
			b.position -= qty
			if b.position < 0 {
				b.avgPrice = totalValue / (-b.position)
			}
		} else {
			// Closing long
			closeQty := min(qty, b.position)
			pnl := closeQty * (trade.Price - b.avgPrice)
			b.realizedPnL += pnl
			b.position -= qty
			if b.position < 0 {
				b.avgPrice = trade.Price
			} else if b.position == 0 {
				b.avgPrice = 0
			}
		}
	}
}

// SubmitOrder is a helper to submit orders
func (b *BaseBot) SubmitOrder(side orderbook.Side, orderType orderbook.OrderType, price, quantity int64) ([]orderbook.Trade, error) {
	order := &orderbook.Order{
		UserID:   b.id,
		Side:     side,
		Type:     orderType,
		Price:    price,
		Quantity: quantity,
	}
	return b.book.Submit(order)
}

// CancelAllOrders cancels all orders for this bot
func (b *BaseBot) CancelAllOrders() {
	orders := b.book.GetOrdersByUser(b.id)
	for _, order := range orders {
		b.book.Cancel(order.ID)
	}
}

// OnTradeCallback is a function to call when trades occur
type OnTradeCallback func(orderbook.Trade)

// BotManager manages a collection of bots
type BotManager struct {
	mu sync.Mutex

	bots          []Bot
	onTradeCallbacks []OnTradeCallback
}

// NewBotManager creates a new bot manager
func NewBotManager() *BotManager {
	return &BotManager{}
}

// AddBot adds a bot to the manager
func (m *BotManager) AddBot(bot Bot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bots = append(m.bots, bot)
}

// StartAll starts all bots
func (m *BotManager) StartAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, bot := range m.bots {
		bot.Start()
	}
}

// StopAll stops all bots
func (m *BotManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, bot := range m.bots {
		bot.Stop()
	}
}

// ProcessTrade notifies all bots of a trade
func (m *BotManager) ProcessTrade(trade orderbook.Trade) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, bot := range m.bots {
		bot.ProcessTrade(trade)
	}
}

// OnTrade registers a callback for when any bot trades
func (m *BotManager) OnTrade(fn OnTradeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTradeCallbacks = append(m.onTradeCallbacks, fn)
}

// notifyTrade notifies callbacks of a trade
func (m *BotManager) notifyTrade(trade orderbook.Trade) {
	m.mu.Lock()
	callbacks := make([]OnTradeCallback, len(m.onTradeCallbacks))
	copy(callbacks, m.onTradeCallbacks)
	m.mu.Unlock()

	for _, fn := range callbacks {
		fn(trade)
	}
}

// Count returns number of bots
func (m *BotManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.bots)
}

// Helper for running periodic tasks
func runPeriodic(interval time.Duration, stopCh <-chan struct{}, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fn()
		case <-stopCh:
			return
		}
	}
}
