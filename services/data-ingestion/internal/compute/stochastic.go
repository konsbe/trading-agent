package compute

import "math"

// SlowStochasticLast computes full stochastic %K (smoothed) and %D (smoothed) for the last bar.
// Convention: raw %K from kPeriod, then K = SMA(raw, dSmooth), D = SMA(K, dSignal) — e.g. 14,3,3.
func SlowStochasticLast(highs, lows, closes []float64, kPeriod, dSmooth, dSignal int) (k, d, raw float64, ok bool) {
	if kPeriod <= 0 || dSmooth <= 0 || dSignal <= 0 {
		return 0, 0, 0, false
	}
	n := len(closes)
	minN := kPeriod + dSmooth + dSignal - 2
	if n != len(highs) || n != len(lows) || minN < 3 || n < minN {
		return 0, 0, 0, false
	}
	rawS := make([]float64, n)
	for i := range rawS {
		rawS[i] = math.NaN()
	}
	for i := kPeriod - 1; i < n; i++ {
		hh := highs[i-kPeriod+1]
		ll := lows[i-kPeriod+1]
		for j := i - kPeriod + 2; j <= i; j++ {
			if highs[j] > hh {
				hh = highs[j]
			}
			if lows[j] < ll {
				ll = lows[j]
			}
		}
		if hh == ll {
			rawS[i] = 50
		} else {
			rawS[i] = 100 * (closes[i] - ll) / (hh - ll)
		}
	}
	// Build valid raw tail for SMA
	kS := make([]float64, n)
	for i := range kS {
		kS[i] = math.NaN()
	}
	for i := kPeriod - 1 + dSmooth - 1; i < n; i++ {
		var sum float64
		for j := 0; j < dSmooth; j++ {
			v := rawS[i-j]
			if math.IsNaN(v) {
				sum = math.NaN()
				break
			}
			sum += v
		}
		if !math.IsNaN(sum) {
			kS[i] = sum / float64(dSmooth)
		}
	}
	dS := make([]float64, n)
	for i := range dS {
		dS[i] = math.NaN()
	}
	// First index where K has been smoothed dSmooth times and we can average dSignal K's.
	startD := kPeriod + dSmooth + dSignal - 3
	if startD < 0 {
		startD = 0
	}
	for i := startD; i < n; i++ {
		var sum float64
		for j := 0; j < dSignal; j++ {
			v := kS[i-j]
			if math.IsNaN(v) {
				sum = math.NaN()
				break
			}
			sum += v
		}
		if !math.IsNaN(sum) {
			dS[i] = sum / float64(dSignal)
		}
	}
	li := n - 1
	if math.IsNaN(kS[li]) || math.IsNaN(dS[li]) || math.IsNaN(rawS[li]) {
		return 0, 0, 0, false
	}
	return kS[li], dS[li], rawS[li], true
}
