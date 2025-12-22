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
	"trade/internal/game"
	"trade/internal/historical"
	"trade/internal/orderbook"
	"trade/internal/store"
	"trade/web"
)

func main() {
	port := flag.String("port", "8088", "server port")
	dbPath := flag.String("db", "trade.db", "SQLite database path")
	corsOrigins := flag.String("cors", "", "comma-separated allowed CORS origins (empty = allow all for dev)")
	polygonKey := flag.String("polygon-key", os.Getenv("POLYGON_API_KEY"), "Polygon.io API key")
	matchDuration := flag.Int("duration", 10, "match duration in minutes (10, 15, or 30)")
	flag.Parse()

	// Initialize SQLite store
	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create historical data provider (optional - will use synthetic data if no API key)
	var dataProvider *historical.DataProvider
	if *polygonKey != "" {
		var err error
		dataProvider, err = historical.NewDataProvider(*polygonKey, *dbPath+".historical")
		if err != nil {
			log.Printf("Warning: Failed to create data provider: %v", err)
		} else {
			log.Printf("Using Polygon.io for historical data")
		}
	} else {
		log.Printf("No Polygon API key - using synthetic price data")
	}

	// Create match scheduler
	schedulerConfig := game.DefaultSchedulerConfig()
	switch *matchDuration {
	case 10:
		schedulerConfig.DefaultDuration = historical.Match10Min
	case 15:
		schedulerConfig.DefaultDuration = historical.Match15Min
	case 30:
		schedulerConfig.DefaultDuration = historical.Match30Min
	default:
		log.Printf("Invalid duration %d, using 10 minutes", *matchDuration)
		schedulerConfig.DefaultDuration = historical.Match10Min
	}
	schedulerConfig.PreMatchSec = 10  // Quick countdown for testing
	schedulerConfig.IntermissionSec = 5

	scheduler := game.NewScheduler(st, dataProvider, schedulerConfig)

	// Get embedded frontend files
	staticFS, err := web.GetDistFS()
	if err != nil {
		log.Fatalf("Failed to load embedded frontend: %v", err)
	}

	// Start scheduler (creates first match)
	if err := scheduler.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Create server with scheduler's order book
	server := api.NewServer(scheduler.OrderBook(), st, staticFS)

	// Wire up order book trade callbacks to broadcast via WebSocket
	scheduler.OrderBook().OnTrade(func(trade orderbook.Trade) {
		server.HandleTrade(trade)
	})

	// Configure CORS if specified
	if *corsOrigins != "" {
		origins := strings.Split(*corsOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		server.SetCORSOrigins(origins)
		log.Printf("CORS restricted to: %v", origins)
	}

	// Helper to broadcast current match state
	broadcastMatchState := func() {
		m := scheduler.CurrentMatch()
		if m == nil {
			return
		}
		pf := scheduler.PriceFeed()

		var nav int64
		if pf != nil {
			nav = pf.BookMid()
			if nav == 0 {
				nav = pf.TrueNAV()
			}
		}
		if nav == 0 {
			nav = m.GetNAV()
		}

		server.BroadcastMatchStateData(api.MatchStateData{
			State:        m.GetState().String(),
			Remaining:    int(m.RemainingTime().Seconds()),
			MarketTime:   m.MarketTime(),
			Progress:     m.Progress(),
			NAV:          nav,
			Participants: m.GetParticipants(),
			Bars:         m.GetRevealedBars(),
			CurrentBar:   m.GetCurrentBar(),
		})
	}

	// Set up match state broadcasting
	scheduler.OnMatchStart(func(m *game.Match) {
		log.Printf("Match %s started - broadcasting state", m.ID)
		broadcastMatchState()
	})

	// Broadcast match state periodically during trading
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			m := scheduler.CurrentMatch()
			if m != nil {
				broadcastMatchState()
				if m.GetState() == game.StateTrading {
					server.BroadcastBook()
				}
			}
		}
	}()

	// Auto-start trading after pre-match countdown
	go func() {
		for {
			m := scheduler.CurrentMatch()
			if m == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			state := m.GetState()
			switch state {
			case game.StateLobby:
				// Auto-transition to pre-match after a brief lobby period
				time.Sleep(3 * time.Second)
				if err := scheduler.TransitionToPreMatch(); err != nil {
					log.Printf("Failed to transition to pre-match: %v", err)
				}
			case game.StatePreMatch:
				// Wait for countdown, then start
				time.Sleep(time.Duration(schedulerConfig.PreMatchSec) * time.Second)
				if err := scheduler.StartTrading(); err != nil {
					log.Printf("Failed to start trading: %v", err)
				}
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	addr := ":" + *port
	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	// Start server in goroutine
	go func() {
		log.Printf("=== ARCADE MODE ===")
		log.Printf("Starting trade server on http://localhost%s", addr)
		log.Printf("Match duration: %d minutes", *matchDuration)
		log.Printf("Database: %s", *dbPath)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop scheduler (stops bots, price feed, match)
	scheduler.Stop()
	log.Println("Scheduler stopped")

	// Stop server internal goroutines
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
