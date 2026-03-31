package compute

import "math"

// TrendResult describes the trend of a price series over a lookback window.
type TrendResult struct {
	Direction   string  // "up", "down", or "sideways"
	SlopePct    float64 // linear regression slope normalised by mean price (% per bar)
	R2          float64 // coefficient of determination [0, 1]; higher = cleaner trend
	HigherHighs bool    // second half of window made higher highs than the first half
	HigherLows  bool    // second half of window made higher lows than the first half
}

// AnalyzeTrend determines trend direction from a sliding window of bars.
//
// Direction is primarily determined by the HH/HL (or LH/LL) structure of the
// window. The linear regression slope and R² are stored as supporting evidence
// and used as a fallback when structure is ambiguous (neither HH+HL nor LH+LL).
//
// Requires at least 5 bars within the lookback window.
func AnalyzeTrend(closes, highs, lows []float64, lookback int) (TrendResult, bool) {
	if lookback > len(closes) {
		lookback = len(closes)
	}
	if lookback < 5 {
		return TrendResult{}, false
	}

	c := closes[len(closes)-lookback:]
	h := highs[len(highs)-lookback:]
	l := lows[len(lows)-lookback:]

	lr, ok := linReg(c)
	if !ok {
		return TrendResult{}, false
	}

	// Compare first-half extremes to second-half extremes.
	mid := len(c) / 2
	fhHighMax, fhLowMin := h[0], l[0]
	shHighMax, shLowMin := h[mid], l[mid]

	for i := 1; i < mid; i++ {
		if h[i] > fhHighMax {
			fhHighMax = h[i]
		}
		if l[i] < fhLowMin {
			fhLowMin = l[i]
		}
	}
	for i := mid + 1; i < len(h); i++ {
		if h[i] > shHighMax {
			shHighMax = h[i]
		}
		if l[i] < shLowMin {
			shLowMin = l[i]
		}
	}

	higherHighs := shHighMax > fhHighMax
	higherLows := shLowMin > fhLowMin

	dir := "sideways"
	if higherHighs && higherLows {
		dir = "up"
	} else if !higherHighs && !higherLows {
		dir = "down"
	} else if lr.R2 >= 0.3 {
		// Ambiguous HH/HL structure — fall back to regression slope.
		if lr.SlopePct > 0 {
			dir = "up"
		} else if lr.SlopePct < 0 {
			dir = "down"
		}
	}

	return TrendResult{
		Direction:   dir,
		SlopePct:    lr.SlopePct,
		R2:          lr.R2,
		HigherHighs: higherHighs,
		HigherLows:  higherLows,
	}, true
}

type linRegResult struct {
	SlopePct float64
	R2       float64
}

// linReg performs ordinary least squares on a close series (oldest first).
// The slope is normalised by the series mean so it represents % change per bar.
func linReg(closes []float64) (linRegResult, bool) {
	n := len(closes)
	if n < 3 {
		return linRegResult{}, false
	}
	fn := float64(n)
	var sumX, sumY, sumXY, sumX2 float64
	for i, c := range closes {
		x := float64(i)
		sumX += x
		sumY += c
		sumXY += x * c
		sumX2 += x * x
	}
	denom := fn*sumX2 - sumX*sumX
	if denom == 0 {
		return linRegResult{}, false
	}
	slope := (fn*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / fn
	meanY := sumY / fn

	var ssTot, ssRes float64
	for i, c := range closes {
		pred := slope*float64(i) + intercept
		ssTot += (c - meanY) * (c - meanY)
		ssRes += (c - pred) * (c - pred)
	}
	r2 := 0.0
	if ssTot > 0 {
		r2 = math.Max(0, 1-ssRes/ssTot)
	}
	slopePct := 0.0
	if meanY != 0 {
		slopePct = slope / meanY * 100
	}
	return linRegResult{SlopePct: slopePct, R2: r2}, true
}
