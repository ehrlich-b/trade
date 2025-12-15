package orderbook

import (
	"time"
)

type Side int

const (
	Buy Side = iota
	Sell
)

func (s Side) String() string {
	if s == Buy {
		return "buy"
	}
	return "sell"
}

type OrderType int

const (
	Limit OrderType = iota
	Market
)

type Order struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Symbol    string    `json:"symbol"`
	Side      Side      `json:"side"`
	Type      OrderType `json:"type"`
	Price     int64     `json:"price"` // Price in cents to avoid float issues
	Quantity  int64     `json:"quantity"`
	Filled    int64     `json:"filled"`
	Timestamp time.Time `json:"timestamp"`
}

func (o *Order) Remaining() int64 {
	return o.Quantity - o.Filled
}

func (o *Order) IsFilled() bool {
	return o.Filled >= o.Quantity
}

type Trade struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Price       int64     `json:"price"`
	Quantity    int64     `json:"quantity"`
	BuyOrderID  string    `json:"buy_order_id"`
	SellOrderID string    `json:"sell_order_id"`
	BuyerID     string    `json:"buyer_id"`
	SellerID    string    `json:"seller_id"`
	Timestamp   time.Time `json:"timestamp"`
}
