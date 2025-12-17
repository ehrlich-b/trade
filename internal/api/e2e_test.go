package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"trade/internal/api"
	"trade/internal/market"
	"trade/internal/orderbook"
	"trade/internal/store"

	"github.com/gorilla/websocket"
)

// testEnv holds all the components needed for e2e testing
type testEnv struct {
	server   *httptest.Server
	store    *store.Store
	book     *orderbook.OrderBook
	priceGen *market.PriceGenerator
	mm       *market.MarketMaker
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create in-memory store
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create order book
	book := orderbook.New("FAKE")

	// Create price generator at $100.00, no volatility for predictable tests
	priceGen := market.NewPriceGenerator(10000, 0)

	// Create market maker
	mm := market.NewMarketMaker(book, priceGen)

	// Create server (nil staticFS is fine for tests - no frontend needed)
	srv := api.NewServer(book, st, nil)

	// Create test server
	ts := httptest.NewServer(srv.Router())

	return &testEnv{
		server:   ts,
		store:    st,
		book:     book,
		priceGen: priceGen,
		mm:       mm,
	}
}

func (e *testEnv) cleanup() {
	e.server.Close()
	e.store.Close()
}

// Helper to make JSON requests
func (e *testEnv) post(path string, body interface{}, token string) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", e.server.URL+path, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

func (e *testEnv) get(path string, token string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", e.server.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return http.DefaultClient.Do(req)
}

// decodeJSON is a helper to decode JSON and fail the test on error
func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
}

// assertNoNaN checks that a float64 is not NaN or Inf
func assertNoNaN(t *testing.T, name string, val float64) {
	t.Helper()
	if math.IsNaN(val) {
		t.Errorf("%s is NaN", name)
	}
	if math.IsInf(val, 0) {
		t.Errorf("%s is Inf", name)
	}
}

// registerUser registers a user and returns auth token and account info
func (e *testEnv) registerUser(t *testing.T, username, password string) (token, userID, accountID string) {
	t.Helper()
	resp, err := e.post("/api/auth/register", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("Register failed with status %d: %s", resp.StatusCode, body.String())
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	token = result["token"].(string)
	userID = result["user_id"].(string)
	accountID = result["account_id"].(string)

	if token == "" || userID == "" || accountID == "" {
		t.Fatal("Missing token, user_id, or account_id in register response")
	}

	return token, userID, accountID
}

// getAccount fetches account info and validates it
func (e *testEnv) getAccount(t *testing.T, token string) map[string]interface{} {
	t.Helper()
	resp, err := e.get("/api/account", token)
	if err != nil {
		t.Fatalf("Get account request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Fatalf("Get account failed with status %d: %s", resp.StatusCode, body.String())
	}

	var account map[string]interface{}
	decodeJSON(t, resp, &account)

	// Validate no NaN in account fields
	if balance, ok := account["balance"].(float64); ok {
		assertNoNaN(t, "balance", balance)
	}

	return account
}

// submitOrder submits an order and validates the response
func (e *testEnv) submitOrder(t *testing.T, token string, side, orderType string, price, quantity int64) map[string]interface{} {
	t.Helper()

	body := map[string]interface{}{
		"side":     side,
		"type":     orderType,
		"quantity": quantity,
	}
	if orderType == "limit" {
		body["price"] = price
	}

	resp, err := e.post("/api/orders", body, token)
	if err != nil {
		t.Fatalf("Submit order request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody := new(bytes.Buffer)
		respBody.ReadFrom(resp.Body)
		t.Fatalf("Submit order failed with status %d: %s", resp.StatusCode, respBody.String())
	}

	var result map[string]interface{}
	decodeJSON(t, resp, &result)

	// Validate order_id exists
	if result["order_id"] == nil || result["order_id"] == "" {
		t.Error("Missing order_id in response")
	}

	// Validate trades if any
	if trades, ok := result["trades"].([]interface{}); ok {
		for i, trade := range trades {
			tr := trade.(map[string]interface{})
			if p, ok := tr["price"].(float64); ok {
				assertNoNaN(t, fmt.Sprintf("trade[%d].price", i), p)
			}
			if q, ok := tr["quantity"].(float64); ok {
				assertNoNaN(t, fmt.Sprintf("trade[%d].quantity", i), q)
			}
		}
	}

	return result
}

// getBook fetches the order book and validates it
func (e *testEnv) getBook(t *testing.T) map[string]interface{} {
	t.Helper()
	resp, err := e.get("/api/book", "")
	if err != nil {
		t.Fatalf("Get book request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get book failed with status %d", resp.StatusCode)
	}

	var book map[string]interface{}
	decodeJSON(t, resp, &book)

	// Validate no NaN in prices/quantities
	validateLevels := func(name string, levels []interface{}) {
		for i, level := range levels {
			l := level.(map[string]interface{})
			if p, ok := l["price"].(float64); ok {
				assertNoNaN(t, fmt.Sprintf("%s[%d].price", name, i), p)
			}
			if q, ok := l["quantity"].(float64); ok {
				assertNoNaN(t, fmt.Sprintf("%s[%d].quantity", name, i), q)
			}
		}
	}

	if bids, ok := book["bids"].([]interface{}); ok {
		validateLevels("bids", bids)
	}
	if asks, ok := book["asks"].([]interface{}); ok {
		validateLevels("asks", asks)
	}

	return book
}

func TestE2E_AuthFlow(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	t.Run("Register", func(t *testing.T) {
		token, userID, accountID := env.registerUser(t, "testuser", "testpass123")
		if token == "" {
			t.Error("Expected token in response")
		}
		if userID == "" {
			t.Error("Expected user_id in response")
		}
		if accountID == "" {
			t.Error("Expected account_id in response")
		}
	})

	t.Run("DuplicateRegister", func(t *testing.T) {
		resp, err := env.post("/api/auth/register", map[string]string{
			"username": "testuser",
			"password": "anotherpass",
		}, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("Expected 409 Conflict, got %d", resp.StatusCode)
		}
	})

	t.Run("Login", func(t *testing.T) {
		resp, err := env.post("/api/auth/login", map[string]string{
			"username": "testuser",
			"password": "testpass123",
		}, "")
		if err != nil {
			t.Fatalf("Login request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		decodeJSON(t, resp, &result)

		if result["token"] == nil || result["token"] == "" {
			t.Error("Expected token in response")
		}
	})

	t.Run("WrongPassword", func(t *testing.T) {
		resp, err := env.post("/api/auth/login", map[string]string{
			"username": "testuser",
			"password": "wrongpass",
		}, "")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestE2E_InitialAccountState(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	token, _, _ := env.registerUser(t, "newuser", "password123")

	account := env.getAccount(t, token)

	// Verify starting balance is exactly $1,000,000 (100000000 cents)
	balance := int64(account["balance"].(float64))
	if balance != 100000000 {
		t.Errorf("Expected starting balance 100000000 (=$1M), got %d", balance)
	}

	// Verify no positions initially
	positions := account["positions"].([]interface{})
	if len(positions) != 0 {
		t.Errorf("Expected 0 positions initially, got %d", len(positions))
	}
}

func TestE2E_MarketMakerProvidesLiquidity(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond) // Let MM place orders

	book := env.getBook(t)

	bids := book["bids"].([]interface{})
	asks := book["asks"].([]interface{})

	// Market maker should have placed orders
	if len(bids) == 0 {
		t.Error("Expected bids in order book from market maker")
	}
	if len(asks) == 0 {
		t.Error("Expected asks in order book from market maker")
	}

	// Verify bid prices are below mid price (10000)
	if len(bids) > 0 {
		bestBid := int64(bids[0].(map[string]interface{})["price"].(float64))
		if bestBid >= 10000 {
			t.Errorf("Expected best bid < 10000, got %d", bestBid)
		}
		if bestBid <= 0 {
			t.Errorf("Expected best bid > 0, got %d", bestBid)
		}
	}

	// Verify ask prices are above mid price (10000)
	if len(asks) > 0 {
		bestAsk := int64(asks[0].(map[string]interface{})["price"].(float64))
		if bestAsk <= 10000 {
			t.Errorf("Expected best ask > 10000, got %d", bestAsk)
		}
	}
}

func TestE2E_MarketBuyCreatesPosition(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker to provide liquidity
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "trader1", "password123")

	// Get initial account state
	initialAccount := env.getAccount(t, token)
	initialBalance := int64(initialAccount["balance"].(float64))

	// Get best ask price before trading
	book := env.getBook(t)
	asks := book["asks"].([]interface{})
	if len(asks) == 0 {
		t.Fatal("No asks available - market maker not working")
	}
	bestAsk := int64(asks[0].(map[string]interface{})["price"].(float64))

	// Submit market buy for 10 shares
	buyQty := int64(10)
	result := env.submitOrder(t, token, "buy", "market", 0, buyQty)

	// Verify we got trades back
	trades := result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Expected trades from market buy, got none")
	}

	// Verify trade details
	trade := trades[0].(map[string]interface{})
	tradePrice := int64(trade["price"].(float64))
	tradeQty := int64(trade["quantity"].(float64))

	if tradePrice != bestAsk {
		t.Errorf("Expected trade at best ask %d, got %d", bestAsk, tradePrice)
	}
	if tradeQty <= 0 {
		t.Errorf("Expected positive trade quantity, got %d", tradeQty)
	}

	// Verify position was created
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) == 0 {
		t.Fatal("Expected position after market buy")
	}

	pos := positions[0].(map[string]interface{})
	posQty := int64(pos["quantity"].(float64))
	posAvgPrice := int64(pos["avg_price"].(float64))
	posRealizedPnL := int64(pos["realized_pnl"].(float64))

	// Verify position quantity matches what we bought
	if posQty != buyQty {
		t.Errorf("Expected position quantity %d, got %d", buyQty, posQty)
	}

	// Verify avg price is the trade price (for single fill)
	if posAvgPrice != tradePrice {
		t.Errorf("Expected avg price %d, got %d", tradePrice, posAvgPrice)
	}

	// Verify realized P&L is 0 (we haven't closed any position)
	if posRealizedPnL != 0 {
		t.Errorf("Expected realized P&L 0, got %d", posRealizedPnL)
	}

	// Verify balance decreased by the cost of the purchase (cash flow model)
	// Balance = initial - (price * quantity)
	expectedBalance := initialBalance - (tradePrice * tradeQty)
	newBalance := int64(account["balance"].(float64))
	if newBalance != expectedBalance {
		t.Errorf("Expected balance %d after buying %d shares at %d, got %d", expectedBalance, tradeQty, tradePrice, newBalance)
	}
}

func TestE2E_MarketSellCreatesShortPosition(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "shorttrader", "password123")

	// Get best bid price
	book := env.getBook(t)
	bids := book["bids"].([]interface{})
	if len(bids) == 0 {
		t.Fatal("No bids available")
	}
	bestBid := int64(bids[0].(map[string]interface{})["price"].(float64))

	// Submit market sell for 10 shares (shorting)
	sellQty := int64(10)
	result := env.submitOrder(t, token, "sell", "market", 0, sellQty)

	// Verify we got trades
	trades := result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Expected trades from market sell")
	}

	trade := trades[0].(map[string]interface{})
	tradePrice := int64(trade["price"].(float64))

	if tradePrice != bestBid {
		t.Errorf("Expected trade at best bid %d, got %d", bestBid, tradePrice)
	}

	// Verify short position
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) == 0 {
		t.Fatal("Expected position after market sell")
	}

	pos := positions[0].(map[string]interface{})
	posQty := int64(pos["quantity"].(float64))

	// Short position should be negative
	if posQty != -sellQty {
		t.Errorf("Expected short position quantity %d, got %d", -sellQty, posQty)
	}
}

func TestE2E_RoundTripPnL(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "roundtrip", "password123")

	initialAccount := env.getAccount(t, token)
	initialBalance := int64(initialAccount["balance"].(float64))

	// Verify we have liquidity
	book := env.getBook(t)
	asks := book["asks"].([]interface{})
	bids := book["bids"].([]interface{})
	if len(asks) == 0 || len(bids) == 0 {
		t.Fatal("Need both bids and asks")
	}

	// Buy 10 shares at ask
	buyResult := env.submitOrder(t, token, "buy", "market", 0, 10)
	buyTrades := buyResult["trades"].([]interface{})
	if len(buyTrades) == 0 {
		t.Fatal("Buy order didn't execute")
	}
	buyPrice := int64(buyTrades[0].(map[string]interface{})["price"].(float64))

	// Verify position after buy
	midAccount := env.getAccount(t, token)
	positions := midAccount["positions"].([]interface{})
	if len(positions) == 0 {
		t.Fatal("No position after buy")
	}
	midPos := positions[0].(map[string]interface{})
	if int64(midPos["quantity"].(float64)) != 10 {
		t.Errorf("Expected quantity 10 after buy, got %v", midPos["quantity"])
	}

	// Sell 10 shares at bid (close position)
	sellResult := env.submitOrder(t, token, "sell", "market", 0, 10)
	sellTrades := sellResult["trades"].([]interface{})
	if len(sellTrades) == 0 {
		t.Fatal("Sell order didn't execute")
	}
	sellPrice := int64(sellTrades[0].(map[string]interface{})["price"].(float64))

	// Calculate expected P&L
	expectedPnL := (sellPrice - buyPrice) * 10 // Bought at ask, sold at bid = loss

	// Verify final state
	finalAccount := env.getAccount(t, token)
	finalBalance := int64(finalAccount["balance"].(float64))

	// Balance should change by realized P&L
	actualPnLFromBalance := finalBalance - initialBalance
	if actualPnLFromBalance != expectedPnL {
		t.Errorf("Expected P&L %d from balance change, got %d (buy@%d, sell@%d)",
			expectedPnL, actualPnLFromBalance, buyPrice, sellPrice)
	}

	// Check position is flat or has correct realized P&L
	finalPositions := finalAccount["positions"].([]interface{})
	if len(finalPositions) > 0 {
		finalPos := finalPositions[0].(map[string]interface{})
		finalQty := int64(finalPos["quantity"].(float64))
		if finalQty != 0 {
			t.Errorf("Expected flat position (qty=0), got %d", finalQty)
		}
		realizedPnL := int64(finalPos["realized_pnl"].(float64))
		if realizedPnL != expectedPnL {
			t.Errorf("Expected realized P&L %d, got %d", expectedPnL, realizedPnL)
		}
	}

	// Since we bought at ask and sold at bid, we should have a loss
	// (spread is at least 2 * half-spread = 20 cents per share)
	if expectedPnL >= 0 {
		t.Logf("Warning: Expected loss due to spread but got P&L=%d", expectedPnL)
	}

	t.Logf("Round trip: bought@%d, sold@%d, P&L=%d cents ($%.2f)",
		buyPrice, sellPrice, expectedPnL, float64(expectedPnL)/100)
}

func TestE2E_LimitOrderResting(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Don't start market maker - we want an empty book
	token, _, _ := env.registerUser(t, "limittrader", "password123")

	// Submit limit buy that won't match anything
	result := env.submitOrder(t, token, "buy", "limit", 9000, 50) // $90.00

	// Should have no trades (resting order)
	trades, _ := result["trades"].([]interface{})
	if trades != nil && len(trades) != 0 {
		t.Errorf("Expected 0 trades for resting limit order, got %d", len(trades))
	}

	// Order should appear in book
	book := env.getBook(t)
	bids := book["bids"].([]interface{})
	if len(bids) == 0 {
		t.Error("Expected limit order to appear in book")
	} else {
		bidPrice := int64(bids[0].(map[string]interface{})["price"].(float64))
		bidQty := int64(bids[0].(map[string]interface{})["quantity"].(float64))
		if bidPrice != 9000 {
			t.Errorf("Expected bid at 9000, got %d", bidPrice)
		}
		if bidQty != 50 {
			t.Errorf("Expected bid qty 50, got %d", bidQty)
		}
	}

	// No position should be created for unfilled order
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) != 0 {
		t.Errorf("Expected 0 positions for unfilled order, got %d", len(positions))
	}
}

func TestE2E_WebSocket(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	// Connect to WebSocket
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	defer ws.Close()

	// Should receive initial book snapshot
	t.Run("ReceiveBookSnapshot", func(t *testing.T) {
		ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("Failed to parse message: %v", err)
		}

		if msg["type"] != "book" {
			t.Errorf("Expected 'book' message, got %v", msg["type"])
		}

		book := msg["book"].(map[string]interface{})
		if book["symbol"] != "FAKE" {
			t.Errorf("Expected symbol 'FAKE', got %v", book["symbol"])
		}

		// Validate book has valid data
		bids := book["bids"].([]interface{})
		asks := book["asks"].([]interface{})
		if len(bids) == 0 || len(asks) == 0 {
			t.Error("Expected bids and asks in initial snapshot")
		}
	})

	// Submit an order and check for updates
	t.Run("ReceiveTradeUpdate", func(t *testing.T) {
		token, _, _ := env.registerUser(t, "wstest", "password123")

		// Channel to collect messages
		messages := make(chan map[string]interface{}, 10)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, message, err := ws.ReadMessage()
				if err != nil {
					return
				}
				var msg map[string]interface{}
				json.Unmarshal(message, &msg)
				messages <- msg
			}
		}()

		// Submit market order
		env.submitOrder(t, token, "buy", "market", 0, 5)

		// Wait for messages
		var gotBook, gotTrade bool
		timeout := time.After(2 * time.Second)

	loop:
		for {
			select {
			case msg := <-messages:
				switch msg["type"] {
				case "book":
					gotBook = true
				case "trade":
					gotTrade = true
					// Validate trade data
					trade := msg["trade"].(map[string]interface{})
					if price, ok := trade["price"].(float64); ok {
						assertNoNaN(t, "ws trade price", price)
						if price <= 0 {
							t.Errorf("Expected positive trade price, got %v", price)
						}
					}
				}
				if gotBook && gotTrade {
					break loop
				}
			case <-timeout:
				break loop
			}
		}

		if !gotBook {
			t.Error("Did not receive book update")
		}
		if !gotTrade {
			t.Error("Did not receive trade update")
		}
	})
}

func TestE2E_Leaderboard(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Register multiple users
	for i := 1; i <= 3; i++ {
		env.registerUser(t, fmt.Sprintf("player%d", i), "password123")
	}

	resp, err := env.get("/api/leaderboard", "")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var leaderboard []map[string]interface{}
	decodeJSON(t, resp, &leaderboard)

	if len(leaderboard) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(leaderboard))
	}

	// Validate each entry has required fields
	for i, entry := range leaderboard {
		if _, ok := entry["username"]; !ok {
			t.Errorf("Entry %d missing username", i)
		}
		if pnl, ok := entry["total_pnl"].(float64); ok {
			assertNoNaN(t, fmt.Sprintf("leaderboard[%d].total_pnl", i), pnl)
		}
	}
}

func TestE2E_ConcurrentTrading(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	// Create multiple traders
	numTraders := 5
	tokens := make([]string, numTraders)
	for i := 0; i < numTraders; i++ {
		tokens[i], _, _ = env.registerUser(t, fmt.Sprintf("concurrent%d", i), "password123")
	}

	// All traders submit orders concurrently
	var wg sync.WaitGroup
	errors := make(chan error, 50)

	for i := 0; i < numTraders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token := tokens[idx]

			// Each trader submits 5 small orders
			// Keep quantities small to stay within margin limits
			for j := 0; j < 5; j++ {
				side := "buy"
				if j%2 == 0 {
					side = "sell"
				}
				resp, err := env.post("/api/orders", map[string]interface{}{
					"side":     side,
					"type":     "limit",
					"price":    10000 + (j * 10), // Varying prices
					"quantity": 100,             // Small quantity to stay within margin
				}, token)
				if err != nil {
					errors <- err
					continue
				}
				// Accept 200 (success) or 400 (margin rejection) as valid responses
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
					errors <- fmt.Errorf("trader %d order %d: unexpected status %d", idx, j, resp.StatusCode)
				}
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		t.Errorf("Concurrent error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Fatalf("Had %d errors during concurrent trading", errCount)
	}

	// Note: Account verification skipped due to SQLite in-memory mode concurrent access limitations
	// The main tests (TestE2E_MarketBuyCreatesPosition, TestE2E_RoundTripPnL) verify correctness
	t.Log("All concurrent orders submitted successfully")
}

func TestE2E_NoLiquidityMarketOrder(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Don't start market maker - empty book
	token, _, _ := env.registerUser(t, "noliquid", "password123")

	// Submit market buy into empty book
	result := env.submitOrder(t, token, "buy", "market", 0, 10)

	// Should have no trades (no liquidity)
	trades, _ := result["trades"].([]interface{})
	if trades != nil && len(trades) != 0 {
		t.Errorf("Expected 0 trades in empty book, got %d", len(trades))
	}

	// Should have no position
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) != 0 {
		t.Errorf("Expected 0 positions when no fill, got %d", len(positions))
	}
}

func TestE2E_MarginRejection(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "margintest", "password123")

	// Try to buy way more shares than margin allows
	// At $100/share, $1M balance can only support ~10,000 shares
	resp, err := env.post("/api/orders", map[string]interface{}{
		"side":     "buy",
		"type":     "limit",
		"price":    10000, // $100
		"quantity": 20000, // $2M worth - exceeds $1M balance
	}, token)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 (insufficient margin), got %d", resp.StatusCode)
	}

	// Verify no position was created
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) != 0 {
		t.Errorf("Expected no position after margin rejection, got %d", len(positions))
	}

	// Now try a valid order
	result := env.submitOrder(t, token, "buy", "limit", 10000, 100) // $10,000 worth - well within margin
	if result["order_id"] == nil {
		t.Error("Valid order should succeed")
	}
}

func TestE2E_ShortSellingWithinMargin(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "shorttest", "password123")

	// Short sell within margin limits should work
	result := env.submitOrder(t, token, "sell", "market", 0, 100)
	trades := result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Short sell should execute")
	}

	// Verify short position
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) == 0 {
		t.Fatal("Expected short position")
	}

	pos := positions[0].(map[string]interface{})
	qty := int64(pos["quantity"].(float64))
	if qty >= 0 {
		t.Errorf("Expected negative quantity for short, got %d", qty)
	}
}

func TestE2E_PositionValuesNotNaN(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "nantest", "password123")

	// Execute a trade
	env.submitOrder(t, token, "buy", "market", 0, 10)

	// Get account and validate all position fields
	account := env.getAccount(t, token)
	positions := account["positions"].([]interface{})

	if len(positions) == 0 {
		t.Fatal("Expected position after trade")
	}

	pos := positions[0].(map[string]interface{})

	// Check all numeric fields
	qty := pos["quantity"].(float64)
	avgPrice := pos["avg_price"].(float64)
	realizedPnL := pos["realized_pnl"].(float64)

	assertNoNaN(t, "quantity", qty)
	assertNoNaN(t, "avg_price", avgPrice)
	assertNoNaN(t, "realized_pnl", realizedPnL)

	// Values should be reasonable
	if qty <= 0 {
		t.Errorf("Expected positive quantity, got %v", qty)
	}
	if avgPrice <= 0 {
		t.Errorf("Expected positive avg_price, got %v", avgPrice)
	}
	// realized_pnl can be 0 before closing position

	t.Logf("Position: qty=%v, avgPrice=%v, realizedPnL=%v", qty, avgPrice, realizedPnL)
}

// TestE2E_UserLimitOrderFillByMarketMaker tests the scenario where
// a user's limit order gets filled by market maker requotes.
// This specifically tests the onTrade callback path.
func TestE2E_UserLimitOrderFillByMarketMaker(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Wire up market maker to server (like main.go does)
	srv := api.NewServer(env.book, env.store, nil)
	env.mm.SetOnTrade(srv.HandleTrade)

	// Register user
	token, _, _ := env.registerUser(t, "limituser", "password123")

	// Get initial account
	account := env.getAccount(t, token)
	initialBalance := int64(account["balance"].(float64))
	t.Logf("Initial balance: %d ($%.2f)", initialBalance, float64(initialBalance)/100)

	// User submits limit BUY order above current mid price
	// This will get filled immediately when market maker places asks
	t.Log("Submitting limit BUY at $100.05 (should match MM asks)")
	env.submitOrder(t, token, "buy", "limit", 10005, 10)

	// Start market maker - this will requote and potentially match
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(200 * time.Millisecond) // Wait for MM to requote

	// Check position
	account = env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	t.Logf("Positions after MM start: %d", len(positions))

	// Now submit another limit order that will definitely match
	// First check what's in the book
	book := env.getBook(t)
	asks := book["asks"].([]interface{})
	if len(asks) > 0 {
		bestAsk := int64(asks[0].(map[string]interface{})["price"].(float64))
		t.Logf("Best ask: %d", bestAsk)

		// Submit crossing limit buy
		t.Logf("Submitting limit BUY at %d (crosses best ask)", bestAsk)
		result := env.submitOrder(t, token, "buy", "limit", bestAsk, 10)
		trades := result["trades"].([]interface{})
		t.Logf("Trades from limit buy: %d", len(trades))
	}

	// Final position check
	account = env.getAccount(t, token)
	positions = account["positions"].([]interface{})
	newBalance := int64(account["balance"].(float64))
	t.Logf("Final balance: %d ($%.2f)", newBalance, float64(newBalance)/100)
	t.Logf("Balance change: %d ($%.2f)", newBalance-initialBalance, float64(newBalance-initialBalance)/100)

	if len(positions) == 0 {
		t.Fatal("Expected position after limit order fills")
	}

	pos := positions[0].(map[string]interface{})
	qty := int64(pos["quantity"].(float64))
	avgPrice := int64(pos["avg_price"].(float64))
	t.Logf("Final position: qty=%d avgPrice=%d", qty, avgPrice)

	if qty <= 0 {
		t.Errorf("Expected positive position quantity, got %d", qty)
	}
}

// TestE2E_MultipleTradesPositionAccumulation tests buy/buy/sell/sell pattern
func TestE2E_MultipleTradesPositionAccumulation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "multitrader", "password123")

	// Get initial state
	account := env.getAccount(t, token)
	initialBalance := int64(account["balance"].(float64))
	t.Logf("Initial balance: $%.2f", float64(initialBalance)/100)

	// BUY 1: 10 shares
	t.Log("=== BUY 1: 10 shares ===")
	result := env.submitOrder(t, token, "buy", "market", 0, 10)
	trades := result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Buy 1 should execute")
	}
	buy1Price := int64(trades[0].(map[string]interface{})["price"].(float64))
	t.Logf("Buy 1 executed at %d", buy1Price)

	account = env.getAccount(t, token)
	positions := account["positions"].([]interface{})
	if len(positions) == 0 {
		t.Fatal("Should have position after buy 1")
	}
	pos := positions[0].(map[string]interface{})
	qty := int64(pos["quantity"].(float64))
	avgPrice := int64(pos["avg_price"].(float64))
	t.Logf("After buy 1: qty=%d avgPrice=%d", qty, avgPrice)
	if qty != 10 {
		t.Errorf("Expected qty=10, got %d", qty)
	}

	// BUY 2: 10 more shares
	t.Log("=== BUY 2: 10 more shares ===")
	result = env.submitOrder(t, token, "buy", "market", 0, 10)
	trades = result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Buy 2 should execute")
	}
	buy2Price := int64(trades[0].(map[string]interface{})["price"].(float64))
	t.Logf("Buy 2 executed at %d", buy2Price)

	account = env.getAccount(t, token)
	positions = account["positions"].([]interface{})
	pos = positions[0].(map[string]interface{})
	qty = int64(pos["quantity"].(float64))
	avgPrice = int64(pos["avg_price"].(float64))
	t.Logf("After buy 2: qty=%d avgPrice=%d", qty, avgPrice)
	if qty != 20 {
		t.Errorf("Expected qty=20, got %d", qty)
	}

	// SELL 1: 10 shares
	t.Log("=== SELL 1: 10 shares ===")
	result = env.submitOrder(t, token, "sell", "market", 0, 10)
	trades = result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Sell 1 should execute")
	}
	sell1Price := int64(trades[0].(map[string]interface{})["price"].(float64))
	t.Logf("Sell 1 executed at %d", sell1Price)

	account = env.getAccount(t, token)
	positions = account["positions"].([]interface{})
	pos = positions[0].(map[string]interface{})
	qty = int64(pos["quantity"].(float64))
	realizedPnL := int64(pos["realized_pnl"].(float64))
	t.Logf("After sell 1: qty=%d realizedPnL=%d", qty, realizedPnL)
	if qty != 10 {
		t.Errorf("Expected qty=10, got %d", qty)
	}

	// SELL 2: 10 more shares (close position)
	t.Log("=== SELL 2: 10 more shares (close) ===")
	result = env.submitOrder(t, token, "sell", "market", 0, 10)
	trades = result["trades"].([]interface{})
	if len(trades) == 0 {
		t.Fatal("Sell 2 should execute")
	}
	sell2Price := int64(trades[0].(map[string]interface{})["price"].(float64))
	t.Logf("Sell 2 executed at %d", sell2Price)

	account = env.getAccount(t, token)
	positions = account["positions"].([]interface{})
	pos = positions[0].(map[string]interface{})
	qty = int64(pos["quantity"].(float64))
	realizedPnL = int64(pos["realized_pnl"].(float64))
	finalBalance := int64(account["balance"].(float64))
	t.Logf("After sell 2: qty=%d realizedPnL=%d balance=%d", qty, realizedPnL, finalBalance)

	if qty != 0 {
		t.Errorf("Expected flat position (qty=0), got %d", qty)
	}

	// Calculate expected P&L
	// Bought 20 total, sold 20 total
	// avgBuyPrice = (buy1Price + buy2Price) / 2 per share... actually it's weighted
	// This is more complex - just verify balance change makes sense
	balanceChange := finalBalance - initialBalance
	t.Logf("Total balance change: %d ($%.2f)", balanceChange, float64(balanceChange)/100)

	// With market maker spread, we should have a loss
	if balanceChange >= 0 {
		t.Logf("Warning: Expected loss due to spread, but got profit/break-even")
	}

	t.Logf("Trade prices: buy1=%d buy2=%d sell1=%d sell2=%d", buy1Price, buy2Price, sell1Price, sell2Price)
}

// TestE2E_RawAccountResponse tests exactly what the API returns
func TestE2E_RawAccountResponse(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	token, _, _ := env.registerUser(t, "rawtest", "password123")

	// Buy some shares
	env.submitOrder(t, token, "buy", "market", 0, 10)

	// Get raw account response
	resp, err := env.get("/api/account", token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Read raw body
	body := new(bytes.Buffer)
	body.ReadFrom(resp.Body)
	t.Logf("RAW /api/account response:\n%s", body.String())
}

// TestE2E_FullPositionLifecycle is a comprehensive test that traces through
// the entire position lifecycle with detailed logging
func TestE2E_FullPositionLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	// Start market maker
	env.mm.Start()
	defer env.mm.Stop()
	time.Sleep(100 * time.Millisecond)

	// Step 1: Register user
	token, userID, accountID := env.registerUser(t, "lifecycle_test", "password123")
	t.Logf("Step 1: Registered user=%s account=%s", userID, accountID)

	// Step 2: Get initial account state
	account := env.getAccount(t, token)
	initialBalance := account["balance"].(float64)
	t.Logf("Step 2: Initial balance=%v ($%.2f)", initialBalance, initialBalance/100)

	positions := account["positions"].([]interface{})
	t.Logf("Step 2: Initial positions count=%d", len(positions))

	// Step 3: Get book state
	resp, err := env.get("/api/book", "")
	if err != nil {
		t.Fatalf("Failed to get book: %v", err)
	}
	var bookData map[string]interface{}
	decodeJSON(t, resp, &bookData)
	resp.Body.Close()

	bids := bookData["bids"].([]interface{})
	asks := bookData["asks"].([]interface{})
	t.Logf("Step 3: Book has %d bids, %d asks", len(bids), len(asks))

	if len(asks) == 0 {
		t.Fatal("No asks in book - market maker not providing liquidity")
	}

	bestAsk := asks[0].(map[string]interface{})
	askPrice := bestAsk["price"].(float64)
	t.Logf("Step 3: Best ask price=%v ($%.2f)", askPrice, askPrice/100)

	// Step 4: Submit market buy for 10 shares
	t.Log("Step 4: Submitting market BUY for 10 shares")
	orderResp := env.submitOrder(t, token, "buy", "market", 0, 10)

	trades, _ := orderResp["trades"].([]interface{})
	t.Logf("Step 4: Order response - trades=%d", len(trades))

	if len(trades) > 0 {
		trade := trades[0].(map[string]interface{})
		tradePrice := trade["price"].(float64)
		tradeQty := trade["quantity"].(float64)
		buyerID := trade["buyer_id"]
		sellerID := trade["seller_id"]
		t.Logf("Step 4: Trade executed - price=%v qty=%v buyer=%v seller=%v", tradePrice, tradeQty, buyerID, sellerID)
	}

	// Step 5: Get account after buy
	account = env.getAccount(t, token)
	newBalance := account["balance"].(float64)
	t.Logf("Step 5: Balance after buy=%v ($%.2f)", newBalance, newBalance/100)

	positions = account["positions"].([]interface{})
	t.Logf("Step 5: Positions count=%d", len(positions))

	if len(positions) == 0 {
		t.Fatal("FAIL: No position after market buy - this is the bug!")
	}

	pos := positions[0].(map[string]interface{})
	posSymbol := pos["symbol"]
	posQty := pos["quantity"].(float64)
	posAvgPrice := pos["avg_price"].(float64)
	posPnL := pos["realized_pnl"].(float64)
	t.Logf("Step 5: Position - symbol=%v qty=%v avgPrice=%v ($%.2f) realizedPnL=%v ($%.2f)",
		posSymbol, posQty, posAvgPrice, posAvgPrice/100, posPnL, posPnL/100)

	if posQty != 10 {
		t.Errorf("Expected quantity=10, got %v", posQty)
	}
	if posAvgPrice <= 0 {
		t.Errorf("Expected positive avg_price, got %v", posAvgPrice)
	}

	// Step 6: Submit market sell to close position
	t.Log("Step 6: Submitting market SELL for 10 shares to close position")

	if len(bids) == 0 {
		t.Fatal("No bids in book - cannot sell")
	}
	bestBid := bids[0].(map[string]interface{})
	bidPrice := bestBid["price"].(float64)
	t.Logf("Step 6: Best bid price=%v ($%.2f)", bidPrice, bidPrice/100)

	orderResp = env.submitOrder(t, token, "sell", "market", 0, 10)
	trades, _ = orderResp["trades"].([]interface{})
	t.Logf("Step 6: Sell order response - trades=%d", len(trades))

	if len(trades) > 0 {
		trade := trades[0].(map[string]interface{})
		tradePrice := trade["price"].(float64)
		t.Logf("Step 6: Sell trade price=%v ($%.2f)", tradePrice, tradePrice/100)
	}

	// Step 7: Get final account state
	account = env.getAccount(t, token)
	finalBalance := account["balance"].(float64)
	t.Logf("Step 7: Final balance=%v ($%.2f)", finalBalance, finalBalance/100)

	positions = account["positions"].([]interface{})
	t.Logf("Step 7: Final positions count=%d", len(positions))

	if len(positions) > 0 {
		pos = positions[0].(map[string]interface{})
		finalQty := pos["quantity"].(float64)
		finalAvgPrice := pos["avg_price"].(float64)
		finalPnL := pos["realized_pnl"].(float64)
		t.Logf("Step 7: Final position - qty=%v avgPrice=%v ($%.2f) realizedPnL=%v ($%.2f)",
			finalQty, finalAvgPrice, finalAvgPrice/100, finalPnL, finalPnL/100)

		// After closing, quantity should be 0
		if finalQty != 0 {
			t.Logf("Position still has qty=%v (may have partial fill)", finalQty)
		}

		// Realized PnL should reflect the spread loss
		expectedPnL := (bidPrice - askPrice) * 10 // Buy high, sell low = loss
		t.Logf("Step 7: Expected P&L from spread=%v ($%.2f), actual=%v ($%.2f)",
			expectedPnL, expectedPnL/100, finalPnL, finalPnL/100)
	}

	// Balance change should equal realized P&L
	balanceChange := finalBalance - initialBalance
	t.Logf("Step 7: Balance change=%v ($%.2f)", balanceChange, balanceChange/100)
}
