package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"time"

	"trade/internal/orderbook"
	"trade/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
)

type Server struct {
	book          *orderbook.OrderBook
	hub           *Hub
	store         *store.Store
	sessions      *SessionStore
	rateLimiter   *RateLimiter
	staticFS      fs.FS
	upgrader      websocket.Upgrader
	onTradeCallbacks []func(orderbook.Trade)
	corsOrigins   []string // Allowed CORS origins (empty = allow all)
}

func NewServer(book *orderbook.OrderBook, st *store.Store, staticFS fs.FS) *Server {
	s := &Server{
		book:        book,
		hub:         NewHub(),
		store:       st,
		sessions:    NewSessionStore(st),
		rateLimiter: NewRateLimiter(100, 1*time.Minute), // 100 requests per minute per IP
		staticFS:    staticFS,
	}
	// Default upgrader with configurable origin check
	s.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return s.checkCORSOrigin(r.Header.Get("Origin"))
		},
	}
	return s
}

// SetCORSOrigins sets the allowed CORS origins.
// Pass an empty slice to allow all origins (default, for development).
// Pass specific origins like ["http://localhost:3000", "https://trade.example.com"] for production.
func (s *Server) SetCORSOrigins(origins []string) {
	s.corsOrigins = origins
}

// checkCORSOrigin checks if an origin is allowed
func (s *Server) checkCORSOrigin(origin string) bool {
	// Empty list = allow all (development mode)
	if len(s.corsOrigins) == 0 {
		return true
	}
	// Empty origin header = same-origin request, always allow
	if origin == "" {
		return true
	}
	// Check against whitelist
	for _, allowed := range s.corsOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Rate limiting disabled - was too aggressive
	// r.Use(s.rateLimiter.Middleware)
	// Configure CORS based on allowed origins
	allowedOrigins := s.corsOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"} // Allow all in development mode
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-User-ID", "X-Session-Token"},
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
		r.Get("/orders", s.getOrders) // Get user's open orders
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

	// Update position tracking for BOTH sides of each trade
	// This is critical - without this, counterparty positions don't update!
	if s.store != nil {
		for _, trade := range trades {
			log.Printf("[POSITION] Trade: buyer=%s seller=%s price=%d qty=%d symbol=%s",
				trade.BuyerID, trade.SellerID, trade.Price, trade.Quantity, trade.Symbol)
			// Update buyer's position (skip market maker - it has no real account)
			if trade.BuyerID != "" && trade.BuyerID != "market_maker" {
				buyerAccount, err := s.store.GetAccountByUserID(trade.BuyerID)
				if err == nil {
					log.Printf("[POSITION] Updating buyer %s (account %s)", trade.BuyerID, buyerAccount.ID)
					s.store.UpdatePositionOnTrade(buyerAccount.ID, trade.Symbol, "buy", trade.Price, trade.Quantity)
				} else {
					log.Printf("[POSITION] ERROR getting buyer account: %v", err)
				}
			}
			// Update seller's position (skip market maker - it has no real account)
			if trade.SellerID != "" && trade.SellerID != "market_maker" {
				sellerAccount, err := s.store.GetAccountByUserID(trade.SellerID)
				if err == nil {
					log.Printf("[POSITION] Updating seller %s (account %s)", trade.SellerID, sellerAccount.ID)
					s.store.UpdatePositionOnTrade(sellerAccount.ID, trade.Symbol, "sell", trade.Price, trade.Quantity)
				} else {
					log.Printf("[POSITION] ERROR getting seller account: %v", err)
				}
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
		// Notify trade callbacks (e.g., market maker position tracking)
		s.notifyTradeCallbacks(trade)
	}

	resp := OrderResponse{
		OrderID: order.ID,
		Trades:  trades,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) getOrders(w http.ResponseWriter, r *http.Request) {
	session := s.getSession(r)
	if session == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	orders := s.book.GetOrdersByUser(session.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if orderID == "" {
		http.Error(w, "order id required", http.StatusBadRequest)
		return
	}

	// Check order ownership
	order, exists := s.book.GetOrder(orderID)
	if !exists {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	// Verify the requester owns this order
	session := s.getSession(r)
	if session != nil {
		// Authenticated user must own the order
		if order.UserID != session.UserID {
			http.Error(w, "unauthorized: you can only cancel your own orders", http.StatusForbidden)
			return
		}
	} else {
		// Unauthenticated - check if user_id header matches (for anonymous users)
		// This is less secure but maintains backward compatibility
		userID := r.Header.Get("X-User-ID")
		if userID != "" && order.UserID != userID {
			http.Error(w, "unauthorized: you can only cancel your own orders", http.StatusForbidden)
			return
		}
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
		hub:      s.hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		lastPong: time.Now(),
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

// HandleTrade processes a trade from external sources (like market maker)
// Updates positions for both parties, broadcasts via WebSocket, and notifies callbacks
func (s *Server) HandleTrade(trade orderbook.Trade) {
	// Update positions for both buyer and seller
	if s.store != nil {
		if trade.BuyerID != "" {
			buyerAccount, err := s.store.GetAccountByUserID(trade.BuyerID)
			if err == nil {
				s.store.UpdatePositionOnTrade(buyerAccount.ID, trade.Symbol, "buy", trade.Price, trade.Quantity)
			}
		}
		if trade.SellerID != "" {
			sellerAccount, err := s.store.GetAccountByUserID(trade.SellerID)
			if err == nil {
				s.store.UpdatePositionOnTrade(sellerAccount.ID, trade.Symbol, "sell", trade.Price, trade.Quantity)
			}
		}
	}

	// Broadcast trade via WebSocket
	s.hub.Broadcast(map[string]interface{}{
		"type":  "trade",
		"trade": trade,
	})

	// Notify trade callbacks
	s.notifyTradeCallbacks(trade)
}

// OnTrade registers a callback to be called when trades occur
func (s *Server) OnTrade(fn func(orderbook.Trade)) {
	s.onTradeCallbacks = append(s.onTradeCallbacks, fn)
}

// notifyTradeCallbacks calls all registered trade callbacks
func (s *Server) notifyTradeCallbacks(trade orderbook.Trade) {
	for _, fn := range s.onTradeCallbacks {
		fn(trade)
	}
}

// Shutdown stops internal goroutines (session cleanup, rate limiter, hub, etc.)
func (s *Server) Shutdown() {
	s.sessions.Stop()
	s.rateLimiter.Stop()
	s.hub.Stop()
}
