package compute

import "math"

// CCILast is the Commodity Channel Index on typical price for the last bar.
func CCILast(highs, lows, closes []float64, period int) (float64, bool) {
	n := len(closes)
	if period <= 0 || n != len(highs) || n != len(lows) || n < period {
		return 0, false
	}
	tp := make([]float64, n)
	for i := range closes {
		tp[i] = (highs[i] + lows[i] + closes[i]) / 3
	}
	slice := tp[n-period:]
	var sma float64
	for _, v := range slice {
		sma += v
	}
	sma /= float64(period)
	var md float64
	for _, v := range slice {
		md += math.Abs(v - sma)
	}
	md /= float64(period)
	if md == 0 {
		return 0, false
	}
	lastTP := tp[n-1]
	return (lastTP - sma) / (0.015 * md), true
}
