package compute

import "math"

// RelativeStrengthVsBenchmark: ratio and ratio-of-returns on aligned closes.
type RelativeStrengthVsBenchmark struct {
	Ratio              float64 // asset / benchmark at last bar
	RatioChange1       float64 // % change in ratio vs prior aligned bar
	AssetROC1          float64
	BenchmarkROC1      float64
	Outperformance1    float64 // asset ROC − bench ROC
	AlignedBars        int
}

// RelativeStrengthLast requires parallel series (same length, aligned timestamps).
func RelativeStrengthLast(asset, bench []float64) (RelativeStrengthVsBenchmark, bool) {
	var r RelativeStrengthVsBenchmark
	n := len(asset)
	if n != len(bench) || n < 2 {
		return r, false
	}
	r.AlignedBars = n
	a0, a1 := asset[n-2], asset[n-1]
	b0, b1 := bench[n-2], bench[n-1]
	if b1 == 0 || b0 == 0 {
		return r, false
	}
	r.Ratio = a1 / b1
	r.AssetROC1 = (a1 - a0) / a0 * 100
	r.BenchmarkROC1 = (b1 - b0) / b0 * 100
	r.Outperformance1 = r.AssetROC1 - r.BenchmarkROC1
	r0 := a0 / b0
	if r0 == 0 || math.IsNaN(r0) {
		return r, false
	}
	r.RatioChange1 = (r.Ratio - r0) / r0 * 100
	return r, true
}
