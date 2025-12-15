package orderbook

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PriceLevel holds all orders at a specific price
type PriceLevel struct {
	Price  int64
	Orders []*Order
}

func (pl *PriceLevel) TotalQuantity() int64 {
	var total int64
	for _, o := range pl.Orders {
		total += o.Remaining()
	}
	return total
}

// OrderBook is an in-memory order book for a single symbol
type OrderBook struct {
	Symbol string

	mu     sync.RWMutex
	bids   []*PriceLevel // Sorted descending by price (best bid first)
	asks   []*PriceLevel // Sorted ascending by price (best ask first)
	orders map[string]*Order

	trades     []Trade
	tradeIndex int
}

func New(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		bids:   make([]*PriceLevel, 0),
		asks:   make([]*PriceLevel, 0),
		orders: make(map[string]*Order),
		trades: make([]Trade, 0),
	}
}

// Submit places an order and returns any resulting trades
func (ob *OrderBook) Submit(order *Order) ([]Trade, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if order.ID == "" {
		order.ID = uuid.New().String()
	}
	if order.Timestamp.IsZero() {
		order.Timestamp = time.Now()
	}
	order.Symbol = ob.Symbol

	var trades []Trade

	if order.Type == Market {
		trades = ob.matchMarketOrder(order)
	} else {
		trades = ob.matchLimitOrder(order)
	}

	// If limit order has remaining quantity, add to book
	if order.Type == Limit && !order.IsFilled() {
		ob.addToBook(order)
	}

	return trades, nil
}

func (ob *OrderBook) matchMarketOrder(order *Order) []Trade {
	var trades []Trade

	if order.Side == Buy {
		// Match against asks (ascending price)
		for len(ob.asks) > 0 && !order.IsFilled() {
			level := ob.asks[0]
			trades = append(trades, ob.matchAtLevel(order, level)...)
			if len(level.Orders) == 0 {
				ob.asks = ob.asks[1:]
			}
		}
	} else {
		// Match against bids (descending price)
		for len(ob.bids) > 0 && !order.IsFilled() {
			level := ob.bids[0]
			trades = append(trades, ob.matchAtLevel(order, level)...)
			if len(level.Orders) == 0 {
				ob.bids = ob.bids[1:]
			}
		}
	}

	return trades
}

func (ob *OrderBook) matchLimitOrder(order *Order) []Trade {
	var trades []Trade

	if order.Side == Buy {
		// Match against asks where ask price <= order price
		for len(ob.asks) > 0 && !order.IsFilled() {
			level := ob.asks[0]
			if level.Price > order.Price {
				break // No more matching prices
			}
			trades = append(trades, ob.matchAtLevel(order, level)...)
			if len(level.Orders) == 0 {
				ob.asks = ob.asks[1:]
			}
		}
	} else {
		// Match against bids where bid price >= order price
		for len(ob.bids) > 0 && !order.IsFilled() {
			level := ob.bids[0]
			if level.Price < order.Price {
				break // No more matching prices
			}
			trades = append(trades, ob.matchAtLevel(order, level)...)
			if len(level.Orders) == 0 {
				ob.bids = ob.bids[1:]
			}
		}
	}

	return trades
}

func (ob *OrderBook) matchAtLevel(incoming *Order, level *PriceLevel) []Trade {
	var trades []Trade

	for len(level.Orders) > 0 && !incoming.IsFilled() {
		resting := level.Orders[0]
		matchQty := min(incoming.Remaining(), resting.Remaining())

		incoming.Filled += matchQty
		resting.Filled += matchQty

		var buyOrder, sellOrder *Order
		if incoming.Side == Buy {
			buyOrder, sellOrder = incoming, resting
		} else {
			buyOrder, sellOrder = resting, incoming
		}

		trade := Trade{
			ID:          uuid.New().String(),
			Symbol:      ob.Symbol,
			Price:       level.Price, // Trade at resting order's price
			Quantity:    matchQty,
			BuyOrderID:  buyOrder.ID,
			SellOrderID: sellOrder.ID,
			BuyerID:     buyOrder.UserID,
			SellerID:    sellOrder.UserID,
			Timestamp:   time.Now(),
		}
		trades = append(trades, trade)
		ob.trades = append(ob.trades, trade)

		if resting.IsFilled() {
			delete(ob.orders, resting.ID)
			level.Orders = level.Orders[1:]
		}
	}

	return trades
}

func (ob *OrderBook) addToBook(order *Order) {
	ob.orders[order.ID] = order

	if order.Side == Buy {
		ob.insertBid(order)
	} else {
		ob.insertAsk(order)
	}
}

func (ob *OrderBook) insertBid(order *Order) {
	// Find or create price level (bids sorted descending)
	for i, level := range ob.bids {
		if level.Price == order.Price {
			level.Orders = append(level.Orders, order)
			return
		}
		if level.Price < order.Price {
			// Insert new level here
			newLevel := &PriceLevel{Price: order.Price, Orders: []*Order{order}}
			ob.bids = append(ob.bids[:i], append([]*PriceLevel{newLevel}, ob.bids[i:]...)...)
			return
		}
	}
	// Append at end
	ob.bids = append(ob.bids, &PriceLevel{Price: order.Price, Orders: []*Order{order}})
}

func (ob *OrderBook) insertAsk(order *Order) {
	// Find or create price level (asks sorted ascending)
	for i, level := range ob.asks {
		if level.Price == order.Price {
			level.Orders = append(level.Orders, order)
			return
		}
		if level.Price > order.Price {
			// Insert new level here
			newLevel := &PriceLevel{Price: order.Price, Orders: []*Order{order}}
			ob.asks = append(ob.asks[:i], append([]*PriceLevel{newLevel}, ob.asks[i:]...)...)
			return
		}
	}
	// Append at end
	ob.asks = append(ob.asks, &PriceLevel{Price: order.Price, Orders: []*Order{order}})
}

// Cancel removes an order from the book
func (ob *OrderBook) Cancel(orderID string) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	order, exists := ob.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	delete(ob.orders, orderID)

	if order.Side == Buy {
		ob.removeFromLevels(order, &ob.bids)
	} else {
		ob.removeFromLevels(order, &ob.asks)
	}

	return nil
}

func (ob *OrderBook) removeFromLevels(order *Order, levels *[]*PriceLevel) {
	for i, level := range *levels {
		if level.Price == order.Price {
			for j, o := range level.Orders {
				if o.ID == order.ID {
					level.Orders = append(level.Orders[:j], level.Orders[j+1:]...)
					break
				}
			}
			if len(level.Orders) == 0 {
				*levels = append((*levels)[:i], (*levels)[i+1:]...)
			}
			return
		}
	}
}

// GetOrder returns an order by ID
func (ob *OrderBook) GetOrder(orderID string) (*Order, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	order, exists := ob.orders[orderID]
	return order, exists
}

// Snapshot returns current book state
type BookSnapshot struct {
	Symbol string           `json:"symbol"`
	Bids   []LevelSnapshot  `json:"bids"`
	Asks   []LevelSnapshot  `json:"asks"`
}

type LevelSnapshot struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

func (ob *OrderBook) Snapshot() BookSnapshot {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	snap := BookSnapshot{
		Symbol: ob.Symbol,
		Bids:   make([]LevelSnapshot, len(ob.bids)),
		Asks:   make([]LevelSnapshot, len(ob.asks)),
	}

	for i, level := range ob.bids {
		snap.Bids[i] = LevelSnapshot{
			Price:    level.Price,
			Quantity: level.TotalQuantity(),
		}
	}

	for i, level := range ob.asks {
		snap.Asks[i] = LevelSnapshot{
			Price:    level.Price,
			Quantity: level.TotalQuantity(),
		}
	}

	return snap
}

// RecentTrades returns the last n trades
func (ob *OrderBook) RecentTrades(n int) []Trade {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if n > len(ob.trades) {
		n = len(ob.trades)
	}
	start := len(ob.trades) - n
	result := make([]Trade, n)
	copy(result, ob.trades[start:])
	return result
}

// BestBid returns the highest bid price, or 0 if no bids
func (ob *OrderBook) BestBid() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.bids) == 0 {
		return 0
	}
	return ob.bids[0].Price
}

// BestAsk returns the lowest ask price, or 0 if no asks
func (ob *OrderBook) BestAsk() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.asks) == 0 {
		return 0
	}
	return ob.asks[0].Price
}

// MidPrice returns the midpoint between best bid and ask
func (ob *OrderBook) MidPrice() int64 {
	bid := ob.BestBid()
	ask := ob.BestAsk()
	if bid == 0 || ask == 0 {
		return 0
	}
	return (bid + ask) / 2
}
