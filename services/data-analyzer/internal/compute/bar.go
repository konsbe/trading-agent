package compute

import "time"

// Bar is a single OHLCV candlestick used across all compute functions.
type Bar struct {
	TS     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64

	// RawClose is the UNADJUSTED close, nil when not captured (bars predating
	// migration 012, or from a provider that does not report it).
	//
	// No feature reads this, and none should: Close is the adjusted series and
	// windowed features break at every corporate action if the two are mixed.
	// It exists for Phase 2 §3.2's point-in-time market cap, which must pair an
	// unadjusted price with an unadjusted share count — pairing an adjusted
	// price with an unadjusted count is wrong by the cumulative split factor.
	RawClose *float64
}

// Closes returns the close price series (oldest first).
func Closes(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

// Highs returns the high price series (oldest first).
func Highs(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.High
	}
	return out
}

// Lows returns the low price series (oldest first).
func Lows(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Low
	}
	return out
}

// Volumes returns the volume series (oldest first).
func Volumes(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Volume
	}
	return out
}
