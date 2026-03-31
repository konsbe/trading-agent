package compute

import "math"

// ChartPatternHints returns lightweight structural hints from swing geometry.
// Full chart-pattern recognition (H&S, triangles, etc.) needs richer rules; these flags
// are intentionally conservative OHLCV-only heuristics.
type ChartPatternHints struct {
	DoubleTopCandidate    bool
	DoubleBottomCandidate bool
	High1, High2          float64
	Low1, Low2            float64
}

// DetectChartPatternHints compares the last two swing highs / lows for near-equal extremes
// and checks whether price has retreated from the mean (double top / bottom *candidate*).
func DetectChartPatternHints(highs, lows, closes []float64, strength int, clusterPct float64) ChartPatternHints {
	var h ChartPatternHints
	n := len(closes)
	if n != len(highs) || n != len(lows) || strength < 1 || clusterPct <= 0 {
		return h
	}
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)
	last := closes[n-1]
	if len(hi) >= 2 {
		i1, i2 := hi[len(hi)-2], hi[len(hi)-1]
		p1, p2 := highs[i1], highs[i2]
		m := (p1 + p2) / 2
		if m > 0 && math.Abs(p1-p2)/m*100 <= clusterPct && last < m {
			h.DoubleTopCandidate = true
			h.High1, h.High2 = p1, p2
		}
	}
	if len(lo) >= 2 {
		i1, i2 := lo[len(lo)-2], lo[len(lo)-1]
		p1, p2 := lows[i1], lows[i2]
		m := (p1 + p2) / 2
		if m > 0 && math.Abs(p1-p2)/m*100 <= clusterPct && last > m {
			h.DoubleBottomCandidate = true
			h.Low1, h.Low2 = p1, p2
		}
	}
	return h
}
