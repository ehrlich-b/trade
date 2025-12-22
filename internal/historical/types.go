package historical

import "time"

// MinuteBar represents OHLCV data for a single minute
type MinuteBar struct {
	Timestamp time.Time // Start of the minute
	Open      int64     // Price in cents
	High      int64     // Price in cents
	Low       int64     // Price in cents
	Close     int64     // Price in cents
	Volume    int64     // Number of shares
}

// TradingDay represents a full day of minute bar data
type TradingDay struct {
	Symbol string      // e.g., "SPY"
	Date   time.Time   // The trading date
	Bars   []MinuteBar // 390 bars for a full trading day (9:30-16:00 ET)
}

// Open returns the opening price of the day
func (td *TradingDay) Open() int64 {
	if len(td.Bars) == 0 {
		return 0
	}
	return td.Bars[0].Open
}

// Close returns the closing price of the day
func (td *TradingDay) Close() int64 {
	if len(td.Bars) == 0 {
		return 0
	}
	return td.Bars[len(td.Bars)-1].Close
}

// High returns the day's high
func (td *TradingDay) High() int64 {
	if len(td.Bars) == 0 {
		return 0
	}
	high := td.Bars[0].High
	for _, bar := range td.Bars {
		if bar.High > high {
			high = bar.High
		}
	}
	return high
}

// Low returns the day's low
func (td *TradingDay) Low() int64 {
	if len(td.Bars) == 0 {
		return 0
	}
	low := td.Bars[0].Low
	for _, bar := range td.Bars {
		if bar.Low < low {
			low = bar.Low
		}
	}
	return low
}

// TotalVolume returns total volume for the day
func (td *TradingDay) TotalVolume() int64 {
	var total int64
	for _, bar := range td.Bars {
		total += bar.Volume
	}
	return total
}

// NormalizedDay represents a trading day with prices scaled to a target range
type NormalizedDay struct {
	TradingDay
	ScaleFactor float64 // Multiplier used to normalize prices
	TargetOpen  int64   // The target opening price used for normalization
}

// Normalize scales all prices in the trading day to a target opening price
func (td *TradingDay) Normalize(targetOpen int64) *NormalizedDay {
	if len(td.Bars) == 0 || td.Bars[0].Open == 0 {
		return &NormalizedDay{TradingDay: *td, ScaleFactor: 1.0}
	}

	scaleFactor := float64(targetOpen) / float64(td.Bars[0].Open)

	normalizedBars := make([]MinuteBar, len(td.Bars))
	for i, bar := range td.Bars {
		normalizedBars[i] = MinuteBar{
			Timestamp: bar.Timestamp,
			Open:      int64(float64(bar.Open) * scaleFactor),
			High:      int64(float64(bar.High) * scaleFactor),
			Low:       int64(float64(bar.Low) * scaleFactor),
			Close:     int64(float64(bar.Close) * scaleFactor),
			Volume:    bar.Volume, // Volume stays the same
		}
	}

	return &NormalizedDay{
		TradingDay: TradingDay{
			Symbol: td.Symbol,
			Date:   td.Date,
			Bars:   normalizedBars,
		},
		ScaleFactor: scaleFactor,
		TargetOpen:  targetOpen,
	}
}
