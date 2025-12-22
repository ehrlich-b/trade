package historical

import (
	"time"
)

const (
	// MarketMinutesPerDay is the number of trading minutes in a day (9:30-16:00 ET = 390 min)
	MarketMinutesPerDay = 390
)

// MatchDuration represents standard match lengths
type MatchDuration int

const (
	Match10Min MatchDuration = 10
	Match15Min MatchDuration = 15
	Match30Min MatchDuration = 30
)

// TimeScaler handles mapping between real time and accelerated market time
type TimeScaler struct {
	matchDuration   time.Duration // Real-world match duration
	marketDuration  time.Duration // Simulated market duration (always 390 minutes)
	accelerationFactor float64    // How many market seconds per real second
	matchStart      time.Time     // When the match started (real time)
}

// NewTimeScaler creates a new time scaler for a match
// matchMinutes: real-world duration of the match (10, 15, or 30)
func NewTimeScaler(matchMinutes MatchDuration) *TimeScaler {
	matchDur := time.Duration(matchMinutes) * time.Minute
	marketDur := MarketMinutesPerDay * time.Minute

	return &TimeScaler{
		matchDuration:      matchDur,
		marketDuration:     marketDur,
		accelerationFactor: float64(marketDur) / float64(matchDur),
	}
}

// Start marks the beginning of the match
func (ts *TimeScaler) Start() {
	ts.matchStart = time.Now()
}

// StartAt marks the beginning of the match at a specific time (for testing)
func (ts *TimeScaler) StartAt(t time.Time) {
	ts.matchStart = t
}

// AccelerationFactor returns how many market seconds pass per real second
// 10 min match: 39x acceleration (1 real sec = 39 market sec)
// 15 min match: 26x acceleration
// 30 min match: 13x acceleration
func (ts *TimeScaler) AccelerationFactor() float64 {
	return ts.accelerationFactor
}

// ElapsedReal returns real-world time elapsed since match start
func (ts *TimeScaler) ElapsedReal() time.Duration {
	if ts.matchStart.IsZero() {
		return 0
	}
	return time.Since(ts.matchStart)
}

// ElapsedMarket returns simulated market time elapsed since match start
func (ts *TimeScaler) ElapsedMarket() time.Duration {
	return time.Duration(float64(ts.ElapsedReal()) * ts.accelerationFactor)
}

// RemainingReal returns real-world time remaining in the match
func (ts *TimeScaler) RemainingReal() time.Duration {
	remaining := ts.matchDuration - ts.ElapsedReal()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RemainingMarket returns simulated market time remaining
func (ts *TimeScaler) RemainingMarket() time.Duration {
	remaining := ts.marketDuration - ts.ElapsedMarket()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Progress returns match progress as a fraction (0.0 to 1.0)
func (ts *TimeScaler) Progress() float64 {
	if ts.matchStart.IsZero() {
		return 0
	}
	progress := float64(ts.ElapsedReal()) / float64(ts.matchDuration)
	if progress > 1.0 {
		return 1.0
	}
	return progress
}

// IsComplete returns true if the match time has expired
func (ts *TimeScaler) IsComplete() bool {
	return ts.ElapsedReal() >= ts.matchDuration
}

// CurrentBarIndex returns which minute bar we're currently in (0-389)
func (ts *TimeScaler) CurrentBarIndex() int {
	elapsedMinutes := int(ts.ElapsedMarket().Minutes())
	if elapsedMinutes >= MarketMinutesPerDay {
		return MarketMinutesPerDay - 1
	}
	return elapsedMinutes
}

// BarIndexAtRealTime returns the bar index at a specific real elapsed time
func (ts *TimeScaler) BarIndexAtRealTime(realElapsed time.Duration) int {
	marketElapsed := time.Duration(float64(realElapsed) * ts.accelerationFactor)
	elapsedMinutes := int(marketElapsed.Minutes())
	if elapsedMinutes >= MarketMinutesPerDay {
		return MarketMinutesPerDay - 1
	}
	return elapsedMinutes
}

// RealTimeForBar returns the real elapsed time when a bar index begins
func (ts *TimeScaler) RealTimeForBar(barIndex int) time.Duration {
	if barIndex < 0 {
		barIndex = 0
	}
	if barIndex >= MarketMinutesPerDay {
		barIndex = MarketMinutesPerDay - 1
	}
	marketTime := time.Duration(barIndex) * time.Minute
	return time.Duration(float64(marketTime) / ts.accelerationFactor)
}

// InterpolatePrice calculates the price at the current time within a bar
// Uses linear interpolation between open and close
func (ts *TimeScaler) InterpolatePrice(bar MinuteBar) int64 {
	elapsedMarket := ts.ElapsedMarket()
	barStartMinute := int(elapsedMarket.Minutes())

	// Fraction through the current bar
	fractionInBar := elapsedMarket.Minutes() - float64(barStartMinute)
	if fractionInBar < 0 {
		fractionInBar = 0
	}
	if fractionInBar > 1 {
		fractionInBar = 1
	}

	// Linear interpolation between open and close
	return bar.Open + int64(float64(bar.Close-bar.Open)*fractionInBar)
}

// MatchDurationMinutes returns the match duration in minutes
func (ts *TimeScaler) MatchDurationMinutes() int {
	return int(ts.matchDuration.Minutes())
}

// MarketTimeString returns a formatted market time string (e.g., "10:45 AM")
func (ts *TimeScaler) MarketTimeString() string {
	elapsedMinutes := int(ts.ElapsedMarket().Minutes())
	if elapsedMinutes >= MarketMinutesPerDay {
		elapsedMinutes = MarketMinutesPerDay - 1
	}

	// Market opens at 9:30 AM
	hour := 9 + (elapsedMinutes+30)/60
	minute := (elapsedMinutes + 30) % 60

	ampm := "AM"
	if hour >= 12 {
		ampm = "PM"
		if hour > 12 {
			hour -= 12
		}
	}

	return time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC).Format("3:04") + " " + ampm
}
