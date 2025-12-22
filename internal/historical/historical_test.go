package historical

import (
	"testing"
	"time"
)

func TestMinuteBar(t *testing.T) {
	bar := MinuteBar{
		Timestamp: time.Now(),
		Open:      10000, // $100.00
		High:      10050,
		Low:       9950,
		Close:     10025,
		Volume:    1000,
	}

	if bar.Open != 10000 {
		t.Errorf("expected Open=10000, got %d", bar.Open)
	}
}

func TestTradingDay(t *testing.T) {
	bars := []MinuteBar{
		{Open: 10000, High: 10100, Low: 9900, Close: 10050, Volume: 1000},
		{Open: 10050, High: 10150, Low: 10000, Close: 10100, Volume: 1500},
		{Open: 10100, High: 10200, Low: 10050, Close: 10150, Volume: 2000},
	}

	day := &TradingDay{
		Symbol: "SPY",
		Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Bars:   bars,
	}

	if day.Open() != 10000 {
		t.Errorf("expected Open()=10000, got %d", day.Open())
	}

	if day.Close() != 10150 {
		t.Errorf("expected Close()=10150, got %d", day.Close())
	}

	if day.High() != 10200 {
		t.Errorf("expected High()=10200, got %d", day.High())
	}

	if day.Low() != 9900 {
		t.Errorf("expected Low()=9900, got %d", day.Low())
	}

	if day.TotalVolume() != 4500 {
		t.Errorf("expected TotalVolume()=4500, got %d", day.TotalVolume())
	}
}

func TestNormalize(t *testing.T) {
	bars := []MinuteBar{
		{Open: 20000, High: 20200, Low: 19800, Close: 20100, Volume: 1000}, // $200.00
		{Open: 20100, High: 20300, Low: 20000, Close: 20200, Volume: 1500},
	}

	day := &TradingDay{
		Symbol: "SPY",
		Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Bars:   bars,
	}

	// Normalize to $100.00 open
	normalized := day.Normalize(10000)

	if normalized.ScaleFactor != 0.5 {
		t.Errorf("expected ScaleFactor=0.5, got %f", normalized.ScaleFactor)
	}

	if normalized.Open() != 10000 {
		t.Errorf("expected normalized Open()=10000, got %d", normalized.Open())
	}

	// Close should be scaled proportionally
	expectedClose := int64(float64(20200) * 0.5) // 10100
	if normalized.Close() != expectedClose {
		t.Errorf("expected normalized Close()=%d, got %d", expectedClose, normalized.Close())
	}
}

func TestCache(t *testing.T) {
	cache, err := NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	day := &TradingDay{
		Symbol: "SPY",
		Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Bars: []MinuteBar{
			{Open: 10000, High: 10100, Low: 9900, Close: 10050, Volume: 1000},
		},
	}

	// Put
	if err := cache.Put(day); err != nil {
		t.Fatalf("failed to put day: %v", err)
	}

	// Get
	retrieved, err := cache.Get("SPY", day.Date)
	if err != nil {
		t.Fatalf("failed to get day: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected day to be cached")
	}
	if retrieved.Open() != 10000 {
		t.Errorf("expected Open()=10000, got %d", retrieved.Open())
	}

	// Count
	count, err := cache.CachedDayCount("SPY")
	if err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	// Get non-existent
	notFound, err := cache.Get("SPY", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent day")
	}
}

func TestTimeScaler(t *testing.T) {
	ts := NewTimeScaler(Match10Min)

	// Check acceleration factor: 390 market min / 10 real min = 39x
	expectedFactor := 39.0
	if ts.AccelerationFactor() != expectedFactor {
		t.Errorf("expected acceleration factor %f, got %f", expectedFactor, ts.AccelerationFactor())
	}

	// Start the match
	startTime := time.Now()
	ts.StartAt(startTime)

	// Simulate 5 minutes elapsed
	ts.matchStart = startTime.Add(-5 * time.Minute)

	// Should be 50% progress
	progress := ts.Progress()
	if progress < 0.49 || progress > 0.51 {
		t.Errorf("expected progress ~0.5, got %f", progress)
	}

	// Should have ~195 market minutes elapsed (5 real * 39)
	elapsedMarket := ts.ElapsedMarket()
	expectedMarket := 195 * time.Minute
	if elapsedMarket < expectedMarket-time.Minute || elapsedMarket > expectedMarket+time.Minute {
		t.Errorf("expected ~%v market elapsed, got %v", expectedMarket, elapsedMarket)
	}

	// Bar index should be ~195
	barIndex := ts.CurrentBarIndex()
	if barIndex < 194 || barIndex > 196 {
		t.Errorf("expected bar index ~195, got %d", barIndex)
	}
}

func TestTimeScaler15Min(t *testing.T) {
	ts := NewTimeScaler(Match15Min)

	// Check acceleration factor: 390 market min / 15 real min = 26x
	expectedFactor := 26.0
	if ts.AccelerationFactor() != expectedFactor {
		t.Errorf("expected acceleration factor %f, got %f", expectedFactor, ts.AccelerationFactor())
	}
}

func TestTimeScaler30Min(t *testing.T) {
	ts := NewTimeScaler(Match30Min)

	// Check acceleration factor: 390 market min / 30 real min = 13x
	expectedFactor := 13.0
	if ts.AccelerationFactor() != expectedFactor {
		t.Errorf("expected acceleration factor %f, got %f", expectedFactor, ts.AccelerationFactor())
	}
}

func TestTimeScalerMarketTimeString(t *testing.T) {
	ts := NewTimeScaler(Match10Min)
	ts.StartAt(time.Now().Add(-5 * time.Minute)) // 50% through match

	// 50% of 390 minutes = 195 minutes from market open
	// Market opens at 9:30, so 9:30 + 195 min = 12:45 PM
	marketTime := ts.MarketTimeString()
	if marketTime != "12:45 PM" {
		t.Errorf("expected '12:45 PM', got '%s'", marketTime)
	}
}

func TestTimeScalerComplete(t *testing.T) {
	ts := NewTimeScaler(Match10Min)
	ts.StartAt(time.Now().Add(-11 * time.Minute)) // Past match end

	if !ts.IsComplete() {
		t.Error("expected match to be complete")
	}

	if ts.Progress() != 1.0 {
		t.Errorf("expected progress=1.0, got %f", ts.Progress())
	}
}

func TestDataProviderWithoutAPIKey(t *testing.T) {
	dp, err := NewDataProvider("", ":memory:")
	if err != nil {
		t.Fatalf("failed to create data provider: %v", err)
	}
	defer dp.Close()

	// Should fail without cached data or API key
	_, err = dp.GetRandomDay("SPY", 10)
	if err == nil {
		t.Error("expected error without API key or cached data")
	}
}
