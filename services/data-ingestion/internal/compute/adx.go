package compute

import "math"

// ADXResult contains Wilder ADX and directional indicators at the last bar.
type ADXResult struct {
	ADX     float64
	PlusDI  float64
	MinusDI float64
	DX      float64
}

// ADXWilderLast implements ADX(period), +DI, −DI using Welles Wilder smoothing.
func ADXWilderLast(highs, lows, closes []float64, period int) (ADXResult, bool) {
	n := len(closes)
	if period < 2 || n != len(highs) || n != len(lows) || n < period*2+1 {
		return ADXResult{}, false
	}

	tr := make([]float64, n)
	plusDM := make([]float64, n)
	minusDM := make([]float64, n)
	for i := 1; i < n; i++ {
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
		hl := highs[i] - lows[i]
		hc := math.Abs(highs[i] - closes[i-1])
		lc := math.Abs(lows[i] - closes[i-1])
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}

	var atr, sPlus, sMinus float64
	for i := 1; i <= period; i++ {
		atr += tr[i]
		sPlus += plusDM[i]
		sMinus += minusDM[i]
	}

	dxs := make([]float64, 0, n)
	var lastP, lastM, lastDX float64

	calcDX := func() float64 {
		if atr == 0 {
			lastP, lastM = 0, 0
			return 0
		}
		lastP = 100 * sPlus / atr
		lastM = 100 * sMinus / atr
		if lastP+lastM == 0 {
			return 0
		}
		return 100 * math.Abs(lastP-lastM) / (lastP + lastM)
	}

	for i := period + 1; i < n; i++ {
		atr = atr - atr/float64(period) + tr[i]
		sPlus = sPlus - sPlus/float64(period) + plusDM[i]
		sMinus = sMinus - sMinus/float64(period) + minusDM[i]
		lastDX = calcDX()
		dxs = append(dxs, lastDX)
	}

	if len(dxs) < period {
		return ADXResult{}, false
	}
	adx := sumSlice(dxs[:period]) / float64(period)
	for i := period; i < len(dxs); i++ {
		adx = (adx*float64(period-1) + dxs[i]) / float64(period)
	}

	return ADXResult{
		ADX:     adx,
		PlusDI:  lastP,
		MinusDI: lastM,
		DX:      lastDX,
	}, true
}

func sumSlice(x []float64) float64 {
	var s float64
	for _, v := range x {
		s += v
	}
	return s
}
