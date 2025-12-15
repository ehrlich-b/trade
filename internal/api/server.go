package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"

	"trade/internal/orderbook"
	"trade/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
)

type Server struct {
	book     *orderbook.OrderBook
	hub      *Hub
	store    *store.Store
	sessions *SessionStore
	staticFS fs.FS
	upgrader websocket.Upgrader
}

func NewServer(book *orderbook.OrderBook, st *store.Store, staticFS fs.FS) *Server {
	return &Server{
		book:     book,
		hub:      NewHub(),
		store:    st,
		sessions: NewSessionStore(),
		staticFS: staticFS,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev
			},
		},
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Auth routes (no auth required)
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)

		// Account routes (auth required)
		r.Get("/account", s.handleGetAccount)

		// Public routes
		r.Get("/leaderboard", s.handleLeaderboard)

		// Trading routes
		r.Post("/orders", s.submitOrder)
		r.Delete("/orders/{id}", s.cancelOrder)
		r.Get("/book", s.getBook)
		r.Get("/trades", s.getTrades)
	})

	// WebSocket
	r.Get("/ws", s.handleWebSocket)

	// Serve static files (frontend)
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		r.Handle("/*", fileServer)
	}

	return r
}

type OrderRequest struct {
	UserID   string `json:"user_id"`
	Side     string `json:"side"`     // "buy" or "sell"
	Type     string `json:"type"`     // "limit" or "market"
	Price    int64  `json:"price"`    // in cents, required for limit orders
	Quantity int64  `json:"quantity"`
}

type OrderResponse struct {
	OrderID string            `json:"order_id"`
	Trades  []orderbook.Trade `json:"trades"`
}

func (s *Server) submitOrder(w http.ResponseWriter, r *http.Request) {
	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Check for authenticated session
	session := s.getSession(r)
	userID := req.UserID
	accountID := ""

	// If authenticated, use session user and track position
	if session != nil {
		userID = session.UserID
		accountID = session.AccountID
	}

	if userID == "" {
		http.Error(w, "user_id required (or use auth token)", http.StatusBadRequest)
		return
	}

	if req.Quantity <= 0 {
		http.Error(w, "quantity must be positive", http.StatusBadRequest)
		return
	}

	var side orderbook.Side
	switch req.Side {
	case "buy":
		side = orderbook.Buy
	case "sell":
		side = orderbook.Sell
	default:
		http.Error(w, "side must be 'buy' or 'sell'", http.StatusBadRequest)
		return
	}

	var orderType orderbook.OrderType
	switch req.Type {
	case "limit":
		orderType = orderbook.Limit
		if req.Price <= 0 {
			http.Error(w, "price must be positive for limit orders", http.StatusBadRequest)
			return
		}
	case "market":
		orderType = orderbook.Market
	default:
		http.Error(w, "type must be 'limit' or 'market'", http.StatusBadRequest)
		return
	}

	// Check margin if user is authenticated
	if accountID != "" && s.store != nil {
		// Get estimated price for margin calculation
		var estimatedPrice int64
		if orderType == orderbook.Limit {
			estimatedPrice = req.Price
		} else {
			// For market orders, use best bid/ask or mid price
			if side == orderbook.Buy {
				estimatedPrice = s.book.BestAsk()
			} else {
				estimatedPrice = s.book.BestBid()
			}
			// Fallback to mid price or a default
			if estimatedPrice == 0 {
				estimatedPrice = s.book.MidPrice()
			}
			if estimatedPrice == 0 {
				estimatedPrice = 10000 // $100 fallback
			}
		}

		if err := s.store.CheckMarginForOrder(accountID, s.book.Symbol, req.Side, req.Quantity, estimatedPrice); err != nil {
			if err == store.ErrInsufficientMargin {
				http.Error(w, "insufficient margin for this order", http.StatusBadRequest)
				return
			}
			http.Error(w, "margin check failed", http.StatusInternalServerError)
			return
		}
	}

	order := &orderbook.Order{
		UserID:   userID,
		Side:     side,
		Type:     orderType,
		Price:    req.Price,
		Quantity: req.Quantity,
	}

	trades, err := s.book.Submit(order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update position tracking if user is authenticated
	if accountID != "" && s.store != nil {
		for _, trade := range trades {
			// Determine if we're the buyer or seller
			if trade.BuyerID == userID {
				s.store.UpdatePositionOnTrade(accountID, trade.Symbol, "buy", trade.Price, trade.Quantity)
			}
			if trade.SellerID == userID {
				s.store.UpdatePositionOnTrade(accountID, trade.Symbol, "sell", trade.Price, trade.Quantity)
			}
		}
	}

	// Broadcast updates
	s.broadcastBookUpdate()
	for _, trade := range trades {
		s.hub.Broadcast(map[string]interface{}{
			"type":  "trade",
			"trade": trade,
		})
	}

	resp := OrderResponse{
		OrderID: order.ID,
		Trades:  trades,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		http.Error(w, "order id required", http.StatusBadRequest)
		return
	}

	if err := s.book.Cancel(orderID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.broadcastBookUpdate()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

func (s *Server) getBook(w http.ResponseWriter, r *http.Request) {
	snap := s.book.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

func (s *Server) getTrades(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	trades := s.book.RecentTrades(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trades)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	s.hub.Register(client)

	// Send initial book state
	snap := s.book.Snapshot()
	data, _ := json.Marshal(map[string]interface{}{
		"type": "book",
		"book": snap,
	})
	client.send <- data

	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) broadcastBookUpdate() {
	snap := s.book.Snapshot()
	s.hub.Broadcast(map[string]interface{}{
		"type": "book",
		"book": snap,
	})
}

// BroadcastBook is a public method for external callers (like market maker) to trigger book updates
func (s *Server) BroadcastBook() {
	s.broadcastBookUpdate()
}
