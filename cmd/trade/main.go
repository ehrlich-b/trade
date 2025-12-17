package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"trade/internal/api"
	"trade/internal/market"
	"trade/internal/orderbook"
	"trade/internal/store"
	"trade/web"
)

func main() {
	port := flag.String("port", "8088", "server port")
	dbPath := flag.String("db", "trade.db", "SQLite database path")
	corsOrigins := flag.String("cors", "", "comma-separated allowed CORS origins (empty = allow all for dev)")
	flag.Parse()

	// Initialize SQLite store
	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create order book for our single fake stock
	book := orderbook.New("FAKE")

	// Create synthetic price generator starting at $100.00
	// Volatility of 5 cents per tick creates reasonable movement
	priceGen := market.NewPriceGenerator(10000, 5)
	priceGen.Start(500 * time.Millisecond)

	// Create market maker bot
	mm := market.NewMarketMaker(book, priceGen)

	// Get embedded frontend files
	staticFS, err := web.GetDistFS()
	if err != nil {
		log.Fatalf("Failed to load embedded frontend: %v", err)
	}

	// Create and start server
	server := api.NewServer(book, st, staticFS)

	// Configure CORS if specified
	if *corsOrigins != "" {
		origins := strings.Split(*corsOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		server.SetCORSOrigins(origins)
		log.Printf("CORS restricted to: %v", origins)
	}

	// Wire up market maker to broadcast book updates
	mm.SetOnUpdate(func() {
		server.BroadcastBook()
	})

	// Wire up market maker trades to server (for counterparty position updates + WebSocket broadcast)
	mm.SetOnTrade(server.HandleTrade)

	// Wire up server to notify market maker of trades (for MM's own position tracking)
	server.OnTrade(mm.ProcessTrade)

	mm.Start()

	addr := ":" + *port
	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting trade server on http://localhost%s", addr)
		log.Printf("Synthetic price starting at $%.2f with random walk", float64(priceGen.Price())/100)
		log.Printf("Using database: %s", *dbPath)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop market maker
	mm.Stop()
	log.Println("Market maker stopped")

	// Stop price generator
	priceGen.Stop()
	log.Println("Price generator stopped")

	// Stop server internal goroutines (session cleanup)
	server.Shutdown()
	log.Println("Server internal goroutines stopped")

	// Graceful HTTP shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("HTTP server stopped")

	// Close database
	if err := st.Close(); err != nil {
		log.Printf("Database close error: %v", err)
	}
	log.Println("Database closed")

	log.Println("Server shutdown complete")
}
