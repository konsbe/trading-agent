package compute

import "math"

// RSI hidden (continuation) divergence uses the same swing pivots as regular divergence
// but compares **higher lows / lower highs** in price with **opposite** structure in RSI.
//
//   - Bullish hidden (uptrend continuation): last two swing lows — price higher low (p2 > p1),
//     RSI lower low (r2 < r1).
//   - Bearish hidden (downtrend continuation): last two swing highs — price lower high (p2 < p1),
//     RSI higher high (r2 > r1).
//
// Stricter pairing: optional minimum bar separation between pivots, and optional trend filter
// on the window ending at the second pivot (AnalyzeTrend must read "up" for bull hidden,
// "down" for bear hidden).
type RSIHiddenResult struct {
	Kind RSIHiddenKind

	// Pattern flags before tie-break when both structures appear.
	BearishHiddenPattern bool
	BullishHiddenPattern bool

	PriceHigh1, PriceHigh2 float64
	RSIHigh1, RSIHigh2    float64
	HighIdx1, HighIdx2    int

	PriceLow1, PriceLow2 float64
	RSILow1, RSILow2     float64
	LowIdx1, LowIdx2     int
}

// RSIHiddenKind is separate from regular RSI divergence scoring.
type RSIHiddenKind string

const (
	RSIHiddenNone    RSIHiddenKind = "none"
	RSIHiddenBullish RSIHiddenKind = "bullish_hidden"
	RSIHiddenBearish RSIHiddenKind = "bearish_hidden"
)

// DetectRSIHiddenDivergence applies hidden-divergence rules with stricter pivot pairing.
func DetectRSIHiddenDivergence(
	highs, lows, closes []float64,
	strength, rsiPeriod, minPivotSep int,
	requireTrend bool,
	trendLookback int,
) RSIHiddenResult {
	out := RSIHiddenResult{Kind: RSIHiddenNone}
	if minPivotSep < 1 {
		minPivotSep = 1
	}
	if len(closes) != len(highs) || len(closes) != len(lows) {
		return out
	}
	rsi := RSISeries(closes, rsiPeriod)
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)

	trendOK := func(endIdx int, want string) bool {
		if !requireTrend || want == "" {
			return true
		}
		if endIdx < 2 {
			return false
		}
		// Window ends at endIdx (inclusive); cap lookback.
		lb := trendLookback
		if lb < 5 {
			lb = 5
		}
		start := endIdx - lb + 1
		if start < 0 {
			start = 0
		}
		slice := closes[start : endIdx+1]
		hh := highs[start : endIdx+1]
		ll := lows[start : endIdx+1]
		tr, ok := AnalyzeTrend(slice, hh, ll, len(slice))
		if !ok {
			return false
		}
		return tr.Direction == want
	}

	// Bearish hidden: lower high in price, higher high in RSI.
	if len(hi) >= 2 {
		i1, i2 := hi[len(hi)-2], hi[len(hi)-1]
		if i2 > i1 && i2-i1 >= minPivotSep && i1 >= rsiPeriod && i2 >= rsiPeriod &&
			!math.IsNaN(rsi[i1]) && !math.IsNaN(rsi[i2]) {
			p1, p2 := highs[i1], highs[i2]
			r1, r2 := rsi[i1], rsi[i2]
			out.PriceHigh1, out.PriceHigh2 = p1, p2
			out.RSIHigh1, out.RSIHigh2 = r1, r2
			out.HighIdx1, out.HighIdx2 = i1, i2
			if p2 < p1 && r2 > r1 && trendOK(i2, "down") {
				out.BearishHiddenPattern = true
			}
		}
	}

	// Bullish hidden: higher low in price, lower low in RSI.
	if len(lo) >= 2 {
		i1, i2 := lo[len(lo)-2], lo[len(lo)-1]
		if i2 > i1 && i2-i1 >= minPivotSep && i1 >= rsiPeriod && i2 >= rsiPeriod &&
			!math.IsNaN(rsi[i1]) && !math.IsNaN(rsi[i2]) {
			p1, p2 := lows[i1], lows[i2]
			r1, r2 := rsi[i1], rsi[i2]
			out.PriceLow1, out.PriceLow2 = p1, p2
			out.RSILow1, out.RSILow2 = r1, r2
			out.LowIdx1, out.LowIdx2 = i1, i2
			if p2 > p1 && r2 < r1 && trendOK(i2, "up") {
				out.BullishHiddenPattern = true
			}
		}
	}

	switch {
	case out.BullishHiddenPattern && out.BearishHiddenPattern:
		if out.LowIdx2 > out.HighIdx2 {
			out.Kind = RSIHiddenBullish
		} else {
			out.Kind = RSIHiddenBearish
		}
	case out.BullishHiddenPattern:
		out.Kind = RSIHiddenBullish
	case out.BearishHiddenPattern:
		out.Kind = RSIHiddenBearish
	default:
		out.Kind = RSIHiddenNone
	}
	return out
}

// RSIHiddenScore maps Kind to a scalar for storage.
func RSIHiddenScore(r RSIHiddenResult) float64 {
	switch r.Kind {
	case RSIHiddenBullish:
		return 1
	case RSIHiddenBearish:
		return -1
	default:
		return 0
	}
}
