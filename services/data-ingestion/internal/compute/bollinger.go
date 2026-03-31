package compute

import "math"

// BollingerResult is Bollinger Band state on the last bar.
type BollingerResult struct {
	Middle    float64 // SMA(period)
	Upper     float64
	Lower     float64
	Bandwidth float64 // (upper - lower) / middle, when middle != 0
	PctB      float64 // (close - lower) / (upper - lower), when width > 0
}

// BollingerLast computes SMA ± k·σ (population σ) on the last `period` closes.
func BollingerLast(closes []float64, period int, k float64) (BollingerResult, bool) {
	if period <= 0 || k <= 0 || len(closes) < period {
		return BollingerResult{}, false
	}
	end := len(closes) - 1
	mid, ok := SMA(closes, period)
	if !ok {
		return BollingerResult{}, false
	}
	sd, ok := RollingStdPop(closes, period, end)
	if !ok || sd == 0 {
		return BollingerResult{}, false
	}
	last := closes[end]
	upper := mid + k*sd
	lower := mid - k*sd
	width := upper - lower
	var bw, pctB float64
	if mid != 0 {
		bw = width / math.Abs(mid)
	}
	if width > 0 {
		pctB = (last - lower) / width
	}
	return BollingerResult{
		Middle:    mid,
		Upper:     upper,
		Lower:     lower,
		Bandwidth: bw,
		PctB:      pctB,
	}, true
}
