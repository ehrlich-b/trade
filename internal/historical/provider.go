package historical

import (
	"fmt"
	"math/rand"
	"time"
)

// DataProvider manages fetching and caching of historical data
type DataProvider struct {
	polygon *PolygonClient
	cache   *Cache
	rng     *rand.Rand
}

// NewDataProvider creates a new data provider
func NewDataProvider(apiKey string, cachePath string) (*DataProvider, error) {
	cache, err := NewCache(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	var polygon *PolygonClient
	if apiKey != "" {
		polygon = NewPolygonClient(apiKey)
	}

	return &DataProvider{
		polygon: polygon,
		cache:   cache,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// Close closes the data provider
func (dp *DataProvider) Close() error {
	return dp.cache.Close()
}

// GetDay fetches a trading day, using cache if available
func (dp *DataProvider) GetDay(symbol string, date time.Time) (*TradingDay, error) {
	// Try cache first
	day, err := dp.cache.Get(symbol, date)
	if err != nil {
		return nil, err
	}
	if day != nil {
		return day, nil
	}

	// Fetch from Polygon if not cached
	if dp.polygon == nil {
		return nil, fmt.Errorf("day not cached and no API key configured")
	}

	day, err = dp.polygon.FetchDay(symbol, date)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := dp.cache.Put(day); err != nil {
		// Log but don't fail - we have the data
		fmt.Printf("warning: failed to cache day: %v\n", err)
	}

	return day, nil
}

// GetRandomDay returns a random trading day from the last N years
// It prefers cached days but will fetch from API if needed
func (dp *DataProvider) GetRandomDay(symbol string, yearsBack int) (*TradingDay, error) {
	// First, try to get a random cached day
	day, err := dp.cache.GetRandomCachedDay(symbol)
	if err != nil {
		return nil, err
	}
	if day != nil {
		return day, nil
	}

	// No cached days, need to fetch one
	if dp.polygon == nil {
		return nil, fmt.Errorf("no cached days and no API key configured")
	}

	// Generate a random trading day
	date := dp.randomTradingDay(yearsBack)
	return dp.GetDay(symbol, date)
}

// GetRandomNormalizedDay returns a random day with prices normalized to target
func (dp *DataProvider) GetRandomNormalizedDay(symbol string, yearsBack int, targetOpen int64) (*NormalizedDay, error) {
	day, err := dp.GetRandomDay(symbol, yearsBack)
	if err != nil {
		return nil, err
	}
	return day.Normalize(targetOpen), nil
}

// randomTradingDay generates a random trading day from the last N years
// Avoids weekends and major US holidays
func (dp *DataProvider) randomTradingDay(yearsBack int) time.Time {
	now := time.Now()
	earliest := now.AddDate(-yearsBack, 0, 0)

	// Days between earliest and now
	dayRange := int(now.Sub(earliest).Hours() / 24)

	for attempts := 0; attempts < 100; attempts++ {
		// Random day in range
		daysAgo := dp.rng.Intn(dayRange)
		date := now.AddDate(0, 0, -daysAgo)

		// Skip weekends
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}

		// Skip major US holidays (approximate)
		if dp.isUSMarketHoliday(date) {
			continue
		}

		// Don't pick today or yesterday (data might not be ready)
		if daysAgo < 2 {
			continue
		}

		return date
	}

	// Fallback: just return a date 30 days ago
	return now.AddDate(0, 0, -30)
}

// isUSMarketHoliday checks for major US market holidays (approximate)
func (dp *DataProvider) isUSMarketHoliday(date time.Time) bool {
	month := date.Month()
	day := date.Day()

	// New Year's Day
	if month == time.January && day == 1 {
		return true
	}

	// MLK Day (3rd Monday of January)
	if month == time.January && date.Weekday() == time.Monday && day >= 15 && day <= 21 {
		return true
	}

	// Presidents Day (3rd Monday of February)
	if month == time.February && date.Weekday() == time.Monday && day >= 15 && day <= 21 {
		return true
	}

	// Good Friday (varies - skip this complex calculation, Polygon will return no data anyway)

	// Memorial Day (last Monday of May)
	if month == time.May && date.Weekday() == time.Monday && day >= 25 {
		return true
	}

	// Juneteenth
	if month == time.June && day == 19 {
		return true
	}

	// Independence Day
	if month == time.July && day == 4 {
		return true
	}

	// Labor Day (1st Monday of September)
	if month == time.September && date.Weekday() == time.Monday && day <= 7 {
		return true
	}

	// Thanksgiving (4th Thursday of November)
	if month == time.November && date.Weekday() == time.Thursday && day >= 22 && day <= 28 {
		return true
	}

	// Christmas
	if month == time.December && day == 25 {
		return true
	}

	return false
}

// PrefetchDays fetches and caches multiple random days
// Useful for warming up the cache before matches
func (dp *DataProvider) PrefetchDays(symbol string, count int, yearsBack int) error {
	if dp.polygon == nil {
		return fmt.Errorf("no API key configured for prefetching")
	}

	for i := 0; i < count; i++ {
		date := dp.randomTradingDay(yearsBack)

		// Check if already cached
		cached, err := dp.cache.Get(symbol, date)
		if err != nil {
			return err
		}
		if cached != nil {
			continue // Already have this day
		}

		// Fetch and cache
		day, err := dp.polygon.FetchDay(symbol, date)
		if err != nil {
			// Log and continue - some days might have no data
			fmt.Printf("warning: failed to fetch %s: %v\n", date.Format("2006-01-02"), err)
			continue
		}

		if err := dp.cache.Put(day); err != nil {
			return err
		}

		// Rate limit: Polygon free tier is 5 calls/min
		time.Sleep(15 * time.Second)
	}

	return nil
}

// CachedDayCount returns the number of cached days
func (dp *DataProvider) CachedDayCount(symbol string) (int, error) {
	return dp.cache.CachedDayCount(symbol)
}
