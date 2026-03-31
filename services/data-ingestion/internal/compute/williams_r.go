package compute

import "math"

// WilliamsRLast returns Williams %R for the last bar over `period` (e.g. 14). Range [−100, 0].
func WilliamsRLast(highs, lows, closes []float64, period int) (float64, bool) {
	n := len(closes)
	if period <= 0 || n != len(highs) || n != len(lows) || n < period {
		return 0, false
	}
	end := n - 1
	start := end - period + 1
	hh := highs[start]
	ll := lows[start]
	for i := start + 1; i <= end; i++ {
		if highs[i] > hh {
			hh = highs[i]
		}
		if lows[i] < ll {
			ll = lows[i]
		}
	}
	if hh == ll {
		return -50, true
	}
	r := -100 * (hh - closes[end]) / (hh - ll)
	if math.IsNaN(r) {
		return 0, false
	}
	return r, true
}
