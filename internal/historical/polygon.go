package historical

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PolygonClient fetches historical market data from Polygon.io
type PolygonClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewPolygonClient creates a new Polygon.io API client
func NewPolygonClient(apiKey string) *PolygonClient {
	return &PolygonClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.polygon.io",
	}
}

// polygonAggResponse represents the API response from Polygon.io aggregates endpoint
type polygonAggResponse struct {
	Status       string          `json:"status"`
	ResultsCount int             `json:"resultsCount"`
	Results      []polygonBar    `json:"results"`
	Ticker       string          `json:"ticker"`
	QueryCount   int             `json:"queryCount"`
	RequestID    string          `json:"request_id"`
	Adjusted     bool            `json:"adjusted"`
	NextURL      string          `json:"next_url,omitempty"`
}

// polygonBar represents a single bar from Polygon.io
type polygonBar struct {
	Timestamp int64   `json:"t"`  // Unix timestamp in milliseconds
	Open      float64 `json:"o"`  // Open price
	High      float64 `json:"h"`  // High price
	Low       float64 `json:"l"`  // Low price
	Close     float64 `json:"c"`  // Close price
	Volume    float64 `json:"v"`  // Volume
	VW        float64 `json:"vw"` // Volume weighted average price
	N         int     `json:"n"`  // Number of transactions
}

// FetchDay fetches minute bar data for a specific symbol and date
// date should be in "2006-01-02" format
func (c *PolygonClient) FetchDay(symbol string, date time.Time) (*TradingDay, error) {
	dateStr := date.Format("2006-01-02")

	// Polygon API: GET /v2/aggs/ticker/{stocksTicker}/range/{multiplier}/{timespan}/{from}/{to}
	url := fmt.Sprintf("%s/v2/aggs/ticker/%s/range/1/minute/%s/%s?adjusted=true&sort=asc&limit=50000&apiKey=%s",
		c.baseURL, symbol, dateStr, dateStr, c.apiKey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("polygon request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polygon returned status %d", resp.StatusCode)
	}

	var aggResp polygonAggResponse
	if err := json.NewDecoder(resp.Body).Decode(&aggResp); err != nil {
		return nil, fmt.Errorf("failed to decode polygon response: %w", err)
	}

	if aggResp.Status != "OK" {
		return nil, fmt.Errorf("polygon status: %s", aggResp.Status)
	}

	// Convert Polygon bars to our format
	bars := make([]MinuteBar, 0, len(aggResp.Results))
	for _, pb := range aggResp.Results {
		bar := MinuteBar{
			Timestamp: time.UnixMilli(pb.Timestamp),
			Open:      dollarsToCents(pb.Open),
			High:      dollarsToCents(pb.High),
			Low:       dollarsToCents(pb.Low),
			Close:     dollarsToCents(pb.Close),
			Volume:    int64(pb.Volume),
		}
		bars = append(bars, bar)
	}

	return &TradingDay{
		Symbol: symbol,
		Date:   date,
		Bars:   bars,
	}, nil
}

// dollarsToCents converts a dollar amount to cents
func dollarsToCents(dollars float64) int64 {
	return int64(dollars * 100)
}

// ValidAPIKey checks if the API key is valid by making a test request
func (c *PolygonClient) ValidAPIKey() bool {
	// Use a known date to test the API key
	url := fmt.Sprintf("%s/v2/aggs/ticker/SPY/range/1/day/2024-01-02/2024-01-02?apiKey=%s",
		c.baseURL, c.apiKey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
