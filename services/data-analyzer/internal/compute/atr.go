package compute

import "math"

// ATRWilder returns the Average True Range (Wilder / smoothed) at the last bar.
func ATRWilder(highs, lows, closes []float64, period int) (float64, bool) {
	n := len(closes)
	if period <= 0 || n != len(highs) || n != len(lows) || n < period+1 {
		return 0, false
	}
	tr := make([]float64, n)
	tr[0] = highs[0] - lows[0]
	for i := 1; i < n; i++ {
		hl := highs[i] - lows[i]
		hc := math.Abs(highs[i] - closes[i-1])
		lc := math.Abs(lows[i] - closes[i-1])
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}
	var sum float64
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	atr := sum / float64(period)
	for i := period; i < n; i++ {
		atr = (atr*float64(period-1) + tr[i]) / float64(period)
	}
	return atr, true
}
