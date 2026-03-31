package compute

import "math"

// RSIDivergenceKind classifies the most recent swing-based RSI divergence vs price.
type RSIDivergenceKind string

const (
	RSIDivNone    RSIDivergenceKind = "none"
	RSIDivBearish RSIDivergenceKind = "bearish_regular"
	RSIDivBullish RSIDivergenceKind = "bullish_regular"
)

// RSIDivergenceResult compares the last two swing highs and last two swing lows
// (same pivot strength as S/R) against the RSI series built with `rsiPeriod`.
//
// Bearish regular: last two swing highs — price makes a higher high, RSI makes a lower high.
// Bullish regular: last two swing lows — price makes a lower low, RSI makes a higher low.
//
// Indices must be at least rsiPeriod so RSI is defined at both pivots.
type RSIDivergenceResult struct {
	Kind RSIDivergenceKind

	// BearishPattern / BullishPattern are true when the classic two-pivot test fires,
	// before tie-breaking when both appear in the same window.
	BearishPattern bool
	BullishPattern bool

	PriceHigh1, PriceHigh2 float64
	RSIHigh1, RSIHigh2    float64
	HighIdx1, HighIdx2    int

	PriceLow1, PriceLow2 float64
	RSILow1, RSILow2     float64
	LowIdx1, LowIdx2     int
}

// DetectRSIDivergence scans the window for regular bullish/bearish divergence.
// If both are structurally present, the one whose second pivot is more recent wins;
// if tied on recency, bearish takes precedence (more conservative for longs).
func DetectRSIDivergence(highs, lows, closes []float64, strength, rsiPeriod int) RSIDivergenceResult {
	out := RSIDivergenceResult{Kind: RSIDivNone}
	if len(closes) != len(highs) || len(closes) != len(lows) {
		return out
	}
	rsi := RSISeries(closes, rsiPeriod)
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)

	// --- Bearish: last two swing highs ---
	if len(hi) >= 2 {
		i1, i2 := hi[len(hi)-2], hi[len(hi)-1]
		if i2 > i1 && i1 >= rsiPeriod && i2 >= rsiPeriod &&
			!math.IsNaN(rsi[i1]) && !math.IsNaN(rsi[i2]) {
			p1, p2 := highs[i1], highs[i2]
			r1, r2 := rsi[i1], rsi[i2]
			out.PriceHigh1, out.PriceHigh2 = p1, p2
			out.RSIHigh1, out.RSIHigh2 = r1, r2
			out.HighIdx1, out.HighIdx2 = i1, i2
			if p2 > p1 && r2 < r1 {
				out.BearishPattern = true
			}
		}
	}

	// --- Bullish: last two swing lows ---
	if len(lo) >= 2 {
		i1, i2 := lo[len(lo)-2], lo[len(lo)-1]
		if i2 > i1 && i1 >= rsiPeriod && i2 >= rsiPeriod &&
			!math.IsNaN(rsi[i1]) && !math.IsNaN(rsi[i2]) {
			p1, p2 := lows[i1], lows[i2]
			r1, r2 := rsi[i1], rsi[i2]
			out.PriceLow1, out.PriceLow2 = p1, p2
			out.RSILow1, out.RSILow2 = r1, r2
			out.LowIdx1, out.LowIdx2 = i1, i2
			if p2 < p1 && r2 > r1 {
				out.BullishPattern = true
			}
		}
	}

	switch {
	case out.BullishPattern && out.BearishPattern:
		if out.LowIdx2 > out.HighIdx2 {
			out.Kind = RSIDivBullish
		} else {
			out.Kind = RSIDivBearish
		}
	case out.BullishPattern:
		out.Kind = RSIDivBullish
	case out.BearishPattern:
		out.Kind = RSIDivBearish
	default:
		out.Kind = RSIDivNone
	}
	return out
}

// RSIDivergenceScore maps Kind to a compact scalar for `technical_indicators.value`.
func RSIDivergenceScore(r RSIDivergenceResult) float64 {
	switch r.Kind {
	case RSIDivBullish:
		return 1
	case RSIDivBearish:
		return -1
	default:
		return 0
	}
}
