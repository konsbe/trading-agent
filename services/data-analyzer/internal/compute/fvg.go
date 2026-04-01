package compute

// FVG (Fair Value Gap / Imbalance) is a 3-candle pattern where price moved so
// fast that one side of the market was not filled. Price tends to "return to
// fill" these zones before continuing in the original direction.
//
// Bullish FVG:  bars[i-2].High < bars[i].Low   (gap up — buying pressure)
// Bearish FVG:  bars[i-2].Low  > bars[i].High  (gap down — selling pressure)
//
// Unfilled FVGs act as support (bullish) or resistance (bearish).
// Reference: Smart Money Concepts (SMC) — institutional imbalance analysis.

// FVGKind classifies the direction of a Fair Value Gap.
type FVGKind string

const (
	FVGBullish FVGKind = "bullish"
	FVGBearish FVGKind = "bearish"
)

// FVG represents one detected Fair Value Gap.
type FVG struct {
	BarIndex int
	Kind     FVGKind
	GapLow   float64 // lower boundary of the imbalance zone
	GapHigh  float64 // upper boundary of the imbalance zone
	GapPct   float64 // gap width as % of mid-price
	Filled   bool    // true when a subsequent bar re-entered the gap zone
}

// FVGsResult holds all FVGs detected in the bar series.
type FVGsResult struct {
	All         []FVG
	LastBullish *FVG // most recent unfilled bullish FVG → potential support
	LastBearish *FVG // most recent unfilled bearish FVG → potential resistance
	ActiveCount int  // total number of unfilled gaps
}

// DetectFVGs scans bars for Fair Value Gaps and flags which have been filled.
//
//   - minGapPct: minimum gap width as % of mid-price to filter noise (e.g. 0.1).
//   - lookback:  scan only the most recent N bars (0 = full series).
func DetectFVGs(bars []Bar, minGapPct float64, lookback int) FVGsResult {
	n := len(bars)
	if n < 3 {
		return FVGsResult{}
	}

	start := 0
	if lookback > 0 && lookback < n {
		start = n - lookback
	}

	var all []FVG

	for i := start + 2; i < n; i++ {
		b0 := bars[i-2]
		b2 := bars[i]

		// ── Bullish FVG: gap between b0.High and b2.Low ──────────────────────
		if b0.High < b2.Low {
			mid := (b0.High + b2.Low) / 2
			gapPct := 0.0
			if mid > 0 {
				gapPct = (b2.Low - b0.High) / mid * 100
			}
			if gapPct >= minGapPct {
				fvg := FVG{
					BarIndex: i,
					Kind:     FVGBullish,
					GapLow:   b0.High,
					GapHigh:  b2.Low,
					GapPct:   gapPct,
				}
				for j := i + 1; j < n; j++ {
					if bars[j].Low <= fvg.GapHigh && bars[j].High >= fvg.GapLow {
						fvg.Filled = true
						break
					}
				}
				all = append(all, fvg)
			}
		}

		// ── Bearish FVG: gap between b2.High and b0.Low ──────────────────────
		if b2.High < b0.Low {
			mid := (b2.High + b0.Low) / 2
			gapPct := 0.0
			if mid > 0 {
				gapPct = (b0.Low - b2.High) / mid * 100
			}
			if gapPct >= minGapPct {
				fvg := FVG{
					BarIndex: i,
					Kind:     FVGBearish,
					GapLow:   b2.High,
					GapHigh:  b0.Low,
					GapPct:   gapPct,
				}
				for j := i + 1; j < n; j++ {
					if bars[j].Low <= fvg.GapHigh && bars[j].High >= fvg.GapLow {
						fvg.Filled = true
						break
					}
				}
				all = append(all, fvg)
			}
		}
	}

	result := FVGsResult{All: all}
	for i := len(all) - 1; i >= 0; i-- {
		f := all[i]
		if !f.Filled {
			result.ActiveCount++
			cp := all[i]
			if f.Kind == FVGBullish && result.LastBullish == nil {
				result.LastBullish = &cp
			}
			if f.Kind == FVGBearish && result.LastBearish == nil {
				result.LastBearish = &cp
			}
		}
	}
	return result
}
