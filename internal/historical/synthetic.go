package historical

import (
	"math"
	"math/rand"
	"time"
)

// DayType represents different types of trading day patterns
type DayType int

const (
	DayTypeChoppy     DayType = iota // Oscillates in range, ends near open
	DayTypeTrendUp                   // Steady grind higher with pullbacks
	DayTypeTrendDown                 // Steady grind lower with bounces
	DayTypeVBottom                   // Sells off hard, then recovers (V shape)
	DayTypeInvertedV                 // Rallies hard, then sells off (inverted V)
	DayTypeVolExplosion              // Quiet then sudden big moves
	DayTypeDoubleBottom              // Two selloffs with recovery
	DayTypeBreakout                  // Consolidation then explosive move
)

// SyntheticConfig configures synthetic day generation
type SyntheticConfig struct {
	BasePrice      int64   // Starting price in cents (e.g., 48000 = $480)
	Volatility     float64 // Daily volatility as decimal (e.g., 0.02 = 2%)
	DayType        DayType // Type of day pattern
	ReturnToOpen   bool    // Whether to end near the open price
	EventCount     int     // Number of sudden "news" events
	EventMagnitude float64 // Size of events as % (e.g., 0.005 = 0.5%)
}

// DefaultSyntheticConfig returns a config for an exciting trading day
func DefaultSyntheticConfig() SyntheticConfig {
	return SyntheticConfig{
		BasePrice:      48000,  // $480
		Volatility:     0.025,  // 2.5% daily vol (exciting for a game)
		DayType:        DayTypeChoppy,
		ReturnToOpen:   true,   // Round trip for fairness
		EventCount:     3,      // 3 sudden moves
		EventMagnitude: 0.008,  // 0.8% events
	}
}

// SyntheticGenerator creates realistic synthetic trading days
type SyntheticGenerator struct {
	rng *rand.Rand
}

// NewSyntheticGenerator creates a new generator
func NewSyntheticGenerator() *SyntheticGenerator {
	return &SyntheticGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewSyntheticGeneratorWithSeed creates a generator with a specific seed (for testing)
func NewSyntheticGeneratorWithSeed(seed int64) *SyntheticGenerator {
	return &SyntheticGenerator{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// GenerateDay creates a synthetic trading day with the given configuration
func (g *SyntheticGenerator) GenerateDay(config SyntheticConfig) *TradingDay {
	bars := make([]MinuteBar, 390) // Full trading day

	// Generate the price path based on day type
	closes := g.generatePricePath(config)

	// Convert closes to full OHLCV bars
	baseTime := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)

	for i := 0; i < 390; i++ {
		prevClose := config.BasePrice
		if i > 0 {
			prevClose = closes[i-1]
		}

		bars[i] = g.generateBar(baseTime.Add(time.Duration(i)*time.Minute), prevClose, closes[i], config)
	}

	return &TradingDay{
		Symbol: "SPY",
		Date:   baseTime,
		Bars:   bars,
	}
}

// GenerateRandomDay creates a random exciting day
func (g *SyntheticGenerator) GenerateRandomDay(basePrice int64) *TradingDay {
	// Pick a random day type with weights favoring exciting patterns
	dayTypes := []DayType{
		DayTypeChoppy,      // 20%
		DayTypeChoppy,
		DayTypeVBottom,     // 20%
		DayTypeVBottom,
		DayTypeInvertedV,   // 20%
		DayTypeInvertedV,
		DayTypeTrendUp,     // 10%
		DayTypeTrendDown,   // 10%
		DayTypeVolExplosion,// 10%
		DayTypeBreakout,    // 10%
	}

	dayType := dayTypes[g.rng.Intn(len(dayTypes))]

	// Higher volatility for more excitement
	volatility := 0.02 + g.rng.Float64()*0.02 // 2-4% daily vol

	config := SyntheticConfig{
		BasePrice:      basePrice,
		Volatility:     volatility,
		DayType:        dayType,
		ReturnToOpen:   dayType != DayTypeTrendUp && dayType != DayTypeTrendDown,
		EventCount:     2 + g.rng.Intn(4), // 2-5 events
		EventMagnitude: 0.005 + g.rng.Float64()*0.01, // 0.5-1.5% events
	}

	return g.GenerateDay(config)
}

// generatePricePath creates the close prices for each minute
func (g *SyntheticGenerator) generatePricePath(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)

	switch config.DayType {
	case DayTypeVBottom:
		closes = g.generateVBottom(config)
	case DayTypeInvertedV:
		closes = g.generateInvertedV(config)
	case DayTypeTrendUp:
		closes = g.generateTrend(config, 1.0)
	case DayTypeTrendDown:
		closes = g.generateTrend(config, -1.0)
	case DayTypeVolExplosion:
		closes = g.generateVolExplosion(config)
	case DayTypeDoubleBottom:
		closes = g.generateDoubleBottom(config)
	case DayTypeBreakout:
		closes = g.generateBreakout(config)
	default: // DayTypeChoppy
		closes = g.generateChoppy(config)
	}

	// Add random events (sudden moves)
	g.addEvents(closes, config)

	// Apply intraday volatility pattern (higher at open/close, lower at lunch)
	g.applyIntradayPattern(closes, config)

	return closes
}

// generateChoppy creates a range-bound day that ends near the open
func (g *SyntheticGenerator) generateChoppy(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Use Ornstein-Uhlenbeck process for mean reversion
	meanReversionStrength := 0.02
	target := float64(config.BasePrice)

	// Per-minute volatility
	minuteVol := config.Volatility / math.Sqrt(390)

	for i := 0; i < 390; i++ {
		// Mean reversion pull
		drift := meanReversionStrength * (target - price)

		// Random walk component
		noise := g.rng.NormFloat64() * minuteVol * price

		price += drift + noise
		closes[i] = int64(price)
	}

	// Ensure we end near open if configured
	if config.ReturnToOpen {
		g.pullToTarget(closes, config.BasePrice)
	}

	return closes
}

// generateVBottom creates a V-shaped recovery day
func (g *SyntheticGenerator) generateVBottom(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Find the low point (somewhere in first half)
	lowPoint := 60 + g.rng.Intn(120) // Between minute 60-180
	maxDrawdown := config.Volatility * 1.5 // Deeper than normal vol

	minuteVol := config.Volatility / math.Sqrt(390) * 0.5 // Less noise

	for i := 0; i < 390; i++ {
		var drift float64

		if i < lowPoint {
			// Selling off toward the low
			progress := float64(i) / float64(lowPoint)
			targetPrice := float64(config.BasePrice) * (1 - maxDrawdown*progress)
			drift = (targetPrice - price) * 0.1
		} else {
			// Recovering back to open
			progress := float64(i-lowPoint) / float64(390-lowPoint)
			targetPrice := float64(config.BasePrice) * (1 - maxDrawdown*(1-progress))
			drift = (targetPrice - price) * 0.08
		}

		noise := g.rng.NormFloat64() * minuteVol * price
		price += drift + noise
		closes[i] = int64(price)
	}

	if config.ReturnToOpen {
		g.pullToTarget(closes, config.BasePrice)
	}

	return closes
}

// generateInvertedV creates an inverted V pattern (rally then selloff)
func (g *SyntheticGenerator) generateInvertedV(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Find the high point (somewhere in first half)
	highPoint := 60 + g.rng.Intn(120)
	maxRally := config.Volatility * 1.5

	minuteVol := config.Volatility / math.Sqrt(390) * 0.5

	for i := 0; i < 390; i++ {
		var drift float64

		if i < highPoint {
			// Rallying toward the high
			progress := float64(i) / float64(highPoint)
			targetPrice := float64(config.BasePrice) * (1 + maxRally*progress)
			drift = (targetPrice - price) * 0.1
		} else {
			// Selling off back to open
			progress := float64(i-highPoint) / float64(390-highPoint)
			targetPrice := float64(config.BasePrice) * (1 + maxRally*(1-progress))
			drift = (targetPrice - price) * 0.08
		}

		noise := g.rng.NormFloat64() * minuteVol * price
		price += drift + noise
		closes[i] = int64(price)
	}

	if config.ReturnToOpen {
		g.pullToTarget(closes, config.BasePrice)
	}

	return closes
}

// generateTrend creates a trending day
func (g *SyntheticGenerator) generateTrend(config SyntheticConfig, direction float64) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Total move over the day
	totalMove := config.Volatility * direction
	movePerMinute := totalMove / 390

	minuteVol := config.Volatility / math.Sqrt(390) * 0.7

	for i := 0; i < 390; i++ {
		// Steady drift with some mean reversion to trend line
		expectedPrice := float64(config.BasePrice) * (1 + movePerMinute*float64(i+1))
		drift := (expectedPrice - price) * 0.05
		drift += float64(config.BasePrice) * movePerMinute * 0.5

		noise := g.rng.NormFloat64() * minuteVol * price
		price += drift + noise
		closes[i] = int64(price)
	}

	return closes
}

// generateVolExplosion creates a quiet-then-explosive day
func (g *SyntheticGenerator) generateVolExplosion(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Explosion point
	explosionStart := 180 + g.rng.Intn(120) // Afternoon

	minuteVolQuiet := config.Volatility / math.Sqrt(390) * 0.3
	minuteVolLoud := config.Volatility / math.Sqrt(390) * 3.0

	for i := 0; i < 390; i++ {
		var vol float64
		if i < explosionStart {
			vol = minuteVolQuiet
		} else if i < explosionStart+30 {
			// Explosion period
			vol = minuteVolLoud
		} else {
			// Calming down
			vol = minuteVolQuiet * 1.5
		}

		// Mean revert to open
		drift := (float64(config.BasePrice) - price) * 0.01
		noise := g.rng.NormFloat64() * vol * price

		price += drift + noise
		closes[i] = int64(price)
	}

	if config.ReturnToOpen {
		g.pullToTarget(closes, config.BasePrice)
	}

	return closes
}

// generateDoubleBottom creates a W pattern
func (g *SyntheticGenerator) generateDoubleBottom(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Two low points
	low1 := 90 + g.rng.Intn(30)
	mid := 180 + g.rng.Intn(30)
	low2 := 270 + g.rng.Intn(30)

	maxDrawdown := config.Volatility * 1.2
	minuteVol := config.Volatility / math.Sqrt(390) * 0.4

	for i := 0; i < 390; i++ {
		var targetPrice float64

		if i < low1 {
			progress := float64(i) / float64(low1)
			targetPrice = float64(config.BasePrice) * (1 - maxDrawdown*progress)
		} else if i < mid {
			progress := float64(i-low1) / float64(mid-low1)
			targetPrice = float64(config.BasePrice) * (1 - maxDrawdown*(1-progress*0.5))
		} else if i < low2 {
			progress := float64(i-mid) / float64(low2-mid)
			targetPrice = float64(config.BasePrice) * (1 - maxDrawdown*0.5 - maxDrawdown*0.5*progress)
		} else {
			progress := float64(i-low2) / float64(390-low2)
			targetPrice = float64(config.BasePrice) * (1 - maxDrawdown*(1-progress))
		}

		drift := (targetPrice - price) * 0.08
		noise := g.rng.NormFloat64() * minuteVol * price
		price += drift + noise
		closes[i] = int64(price)
	}

	if config.ReturnToOpen {
		g.pullToTarget(closes, config.BasePrice)
	}

	return closes
}

// generateBreakout creates consolidation then breakout
func (g *SyntheticGenerator) generateBreakout(config SyntheticConfig) []int64 {
	closes := make([]int64, 390)
	price := float64(config.BasePrice)

	// Breakout point and direction
	breakoutPoint := 200 + g.rng.Intn(100)
	breakoutDir := 1.0
	if g.rng.Float64() < 0.5 {
		breakoutDir = -1.0
	}

	consolidationRange := config.Volatility * 0.3
	minuteVolTight := config.Volatility / math.Sqrt(390) * 0.3
	minuteVolBreakout := config.Volatility / math.Sqrt(390) * 2.0

	for i := 0; i < 390; i++ {
		if i < breakoutPoint {
			// Tight consolidation
			drift := (float64(config.BasePrice) - price) * 0.05

			// Bounce off range boundaries
			if price > float64(config.BasePrice)*(1+consolidationRange) {
				drift -= float64(config.BasePrice) * 0.002
			} else if price < float64(config.BasePrice)*(1-consolidationRange) {
				drift += float64(config.BasePrice) * 0.002
			}

			noise := g.rng.NormFloat64() * minuteVolTight * price
			price += drift + noise
		} else {
			// Breakout and fade back
			progress := float64(i-breakoutPoint) / float64(390-breakoutPoint)

			// Sharp move then fade
			breakoutMagnitude := config.Volatility * 1.5
			if progress < 0.3 {
				// Breakout phase
				targetPrice := float64(config.BasePrice) * (1 + breakoutDir*breakoutMagnitude*progress/0.3)
				drift := (targetPrice - price) * 0.15
				noise := g.rng.NormFloat64() * minuteVolBreakout * price
				price += drift + noise
			} else {
				// Fade back
				fadeProgress := (progress - 0.3) / 0.7
				targetPrice := float64(config.BasePrice) * (1 + breakoutDir*breakoutMagnitude*(1-fadeProgress))
				drift := (targetPrice - price) * 0.06
				noise := g.rng.NormFloat64() * minuteVolTight * price * 1.5
				price += drift + noise
			}
		}
		closes[i] = int64(price)
	}

	if config.ReturnToOpen {
		g.pullToTarget(closes, config.BasePrice)
	}

	return closes
}

// addEvents adds sudden price spikes to simulate news/events
func (g *SyntheticGenerator) addEvents(closes []int64, config SyntheticConfig) {
	for i := 0; i < config.EventCount; i++ {
		// Pick random time (avoid first and last 15 minutes)
		eventTime := 15 + g.rng.Intn(360)

		// Event direction and magnitude
		direction := 1.0
		if g.rng.Float64() < 0.5 {
			direction = -1.0
		}
		magnitude := config.EventMagnitude * (0.5 + g.rng.Float64())

		// Apply spike and quick partial reversion
		spike := int64(float64(config.BasePrice) * magnitude * direction)

		endTime := eventTime + 20
		if endTime > 390 {
			endTime = 390
		}
		for j := eventTime; j < endTime; j++ {
			decay := math.Exp(-float64(j-eventTime) * 0.15) // Quick decay
			closes[j] += int64(float64(spike) * decay)
		}
	}
}

// applyIntradayPattern adjusts volatility based on time of day
func (g *SyntheticGenerator) applyIntradayPattern(closes []int64, config SyntheticConfig) {
	// Intraday volatility multipliers
	// High at open, low at lunch, high at close
	for i := 0; i < 390; i++ {
		var volMult float64

		if i < 30 { // First 30 minutes - high vol
			volMult = 1.5 - float64(i)*0.02
		} else if i < 180 { // Morning settling
			volMult = 0.9
		} else if i < 270 { // Lunch doldrums
			volMult = 0.6
		} else if i < 360 { // Afternoon
			volMult = 0.9
		} else { // Power hour
			volMult = 1.3
		}

		// Add some noise based on pattern
		noise := g.rng.NormFloat64() * float64(config.BasePrice) * 0.001 * volMult
		closes[i] += int64(noise)
	}
}

// pullToTarget adjusts the end of the day to return near the open
func (g *SyntheticGenerator) pullToTarget(closes []int64, target int64) {
	if len(closes) < 60 {
		return
	}

	// Calculate how much we need to adjust in the last hour
	currentEnd := closes[len(closes)-1]
	diff := target - currentEnd

	// Gradually pull toward target over last 60 minutes
	startPull := len(closes) - 60
	for i := startPull; i < len(closes); i++ {
		progress := float64(i-startPull) / 60.0
		// Ease-in curve
		adjustment := float64(diff) * (progress * progress)
		closes[i] += int64(adjustment)
	}
}

// generateBar creates a full OHLCV bar from the close price
func (g *SyntheticGenerator) generateBar(timestamp time.Time, prevClose, close int64, config SyntheticConfig) MinuteBar {
	// Open near previous close with small gap
	gapPct := (g.rng.Float64() - 0.5) * 0.001 // ±0.05%
	open := prevClose + int64(float64(prevClose)*gapPct)

	// High and low based on the range of open-close plus some wick
	minPrice := min(open, close)
	maxPrice := max(open, close)

	// Add wicks (high above max, low below min)
	wickSize := int64(float64(config.BasePrice) * 0.001) // 0.1% typical wick
	wickHigh := int64(g.rng.Float64() * float64(wickSize) * 2)
	wickLow := int64(g.rng.Float64() * float64(wickSize) * 2)

	high := maxPrice + wickHigh
	low := minPrice - wickLow

	// Ensure low doesn't go negative
	if low < 100 {
		low = 100
	}

	// Volume varies with price movement
	priceChange := math.Abs(float64(close-open)) / float64(config.BasePrice)
	baseVolume := 5000 + g.rng.Intn(10000)
	volumeMult := 1.0 + priceChange*20 // More volume on bigger moves
	volume := int64(float64(baseVolume) * volumeMult)

	return MinuteBar{
		Timestamp: timestamp,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
