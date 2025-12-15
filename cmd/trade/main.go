package main

import (
	"flag"
	"log"
	"net/http"
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
	flag.Parse()

	// Initialize SQLite store
	st, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer st.Close()

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

	// Wire up market maker to broadcast book updates
	mm.SetOnUpdate(func() {
		server.BroadcastBook()
	})
	mm.Start()

	addr := ":" + *port
	log.Printf("Starting trade server on http://localhost%s", addr)
	log.Printf("Synthetic price starting at $%.2f with random walk", float64(priceGen.Price())/100)
	log.Printf("Using database: %s", *dbPath)

	if err := http.ListenAndServe(addr, server.Router()); err != nil {
		log.Fatal(err)
	}
}
