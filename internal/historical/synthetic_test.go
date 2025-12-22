package historical

import (
	"fmt"
	"testing"
)

func TestSyntheticGenerator(t *testing.T) {
	gen := NewSyntheticGenerator()

	for i := 0; i < 5; i++ {
		day := gen.GenerateRandomDay(48000)

		open := day.Bars[0].Open
		close := day.Bars[len(day.Bars)-1].Close
		high := day.High()
		low := day.Low()

		pctChange := float64(close-open) / float64(open) * 100
		pctHigh := float64(high-open) / float64(open) * 100
		pctLow := float64(open-low) / float64(open) * 100

		t.Logf("Day %d: Open $%.2f → Close $%.2f (%.2f%%), High +%.2f%%, Low -%.2f%%",
			i+1,
			float64(open)/100,
			float64(close)/100,
			pctChange,
			pctHigh,
			pctLow)

		// Verify basic constraints
		if len(day.Bars) != 390 {
			t.Errorf("Expected 390 bars, got %d", len(day.Bars))
		}

		// Verify OHLC relationships
		for j, bar := range day.Bars {
			if bar.High < bar.Open || bar.High < bar.Close {
				t.Errorf("Bar %d: High (%d) should be >= Open (%d) and Close (%d)",
					j, bar.High, bar.Open, bar.Close)
			}
			if bar.Low > bar.Open || bar.Low > bar.Close {
				t.Errorf("Bar %d: Low (%d) should be <= Open (%d) and Close (%d)",
					j, bar.Low, bar.Open, bar.Close)
			}
		}
	}
}

func TestAllDayTypes(t *testing.T) {
	gen := NewSyntheticGeneratorWithSeed(42)

	dayTypes := []struct {
		name     string
		dayType  DayType
		returnTo bool
	}{
		{"Choppy", DayTypeChoppy, true},
		{"V-Bottom", DayTypeVBottom, true},
		{"Inverted-V", DayTypeInvertedV, true},
		{"Trend Up", DayTypeTrendUp, false},
		{"Trend Down", DayTypeTrendDown, false},
		{"Vol Explosion", DayTypeVolExplosion, true},
		{"Double Bottom", DayTypeDoubleBottom, true},
		{"Breakout", DayTypeBreakout, true},
	}

	for _, dt := range dayTypes {
		t.Run(dt.name, func(t *testing.T) {
			config := SyntheticConfig{
				BasePrice:      48000,
				Volatility:     0.025,
				DayType:        dt.dayType,
				ReturnToOpen:   dt.returnTo,
				EventCount:     2,
				EventMagnitude: 0.008,
			}

			day := gen.GenerateDay(config)

			open := day.Bars[0].Open
			close := day.Bars[len(day.Bars)-1].Close
			high := day.High()
			low := day.Low()

			pctChange := float64(close-open) / float64(open) * 100
			pctHigh := float64(high-open) / float64(open) * 100
			pctLow := float64(open-low) / float64(open) * 100

			t.Logf("%s: Change %.2f%%, High +%.2f%%, Low -%.2f%%",
				dt.name, pctChange, pctHigh, pctLow)

			// Should have some movement
			if pctHigh < 0.5 && pctLow < 0.5 {
				t.Errorf("Day has too little movement: high +%.2f%%, low -%.2f%%", pctHigh, pctLow)
			}
		})
	}
}

func TestPrintSampleDay(t *testing.T) {
	gen := NewSyntheticGeneratorWithSeed(123)

	config := SyntheticConfig{
		BasePrice:      48000,
		Volatility:     0.03, // 3% for more drama
		DayType:        DayTypeVBottom,
		ReturnToOpen:   true,
		EventCount:     3,
		EventMagnitude: 0.01,
	}

	day := gen.GenerateDay(config)

	// Print every 30 minutes to see the pattern
	fmt.Println("\nV-Bottom Day Pattern (every 30 min):")
	fmt.Println("Time     | Open     | High     | Low      | Close    | Vol")
	fmt.Println("---------+----------+----------+----------+----------+--------")

	for i := 0; i < 390; i += 30 {
		bar := day.Bars[i]
		hour := 9 + (i+30)/60
		min := (i + 30) % 60
		fmt.Printf("%02d:%02d    | $%6.2f  | $%6.2f  | $%6.2f  | $%6.2f  | %6d\n",
			hour, min,
			float64(bar.Open)/100,
			float64(bar.High)/100,
			float64(bar.Low)/100,
			float64(bar.Close)/100,
			bar.Volume)
	}
}
