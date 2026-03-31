package compute

import "math"

// IchimokuSnapshot is evaluated at the last bar index L using standard displacement rules:
// Leading spans plotted at L use Tenkan/Kijun and span-B range computed at L−displace
// (displace defaults to kijun period, usually 26).
type IchimokuSnapshot struct {
	Tenkan      float64 // tenkanN-period mid-range at L
	Kijun       float64 // kijunN-period mid-range at L
	SenkouA     float64 // (Tenkan+Kijun)/2 at L−displace
	SenkouB     float64 // spanBN mid-range at L−displace
	CloudTop    float64
	CloudBot    float64
	ChikouClose float64 // close at L (drawn `displace` bars behind on chart)
	Displace    int
}

// IchimokuLast requires L ≥ displace+spanBN−1 so span B at L−displace is defined.
func IchimokuLast(highs, lows, closes []float64, tenkanN, kijunN, spanBN, displace int) (IchimokuSnapshot, bool) {
	if tenkanN < 2 || kijunN < 2 || spanBN < 2 {
		return IchimokuSnapshot{}, false
	}
	if displace <= 0 {
		displace = kijunN
	}
	n := len(closes)
	if n != len(highs) || n != len(lows) {
		return IchimokuSnapshot{}, false
	}
	L := n - 1
	if L < displace+spanBN-1 {
		return IchimokuSnapshot{}, false
	}
	tk, ok1 := midRange(highs, lows, L, tenkanN)
	kj, ok2 := midRange(highs, lows, L, kijunN)
	if !ok1 || !ok2 {
		return IchimokuSnapshot{}, false
	}
	idx := L - displace
	tkP, ok3 := midRange(highs, lows, idx, tenkanN)
	kjP, ok4 := midRange(highs, lows, idx, kijunN)
	sb, ok5 := midRange(highs, lows, idx, spanBN)
	if !ok3 || !ok4 || !ok5 {
		return IchimokuSnapshot{}, false
	}
	sa := (tkP + kjP) / 2
	top := math.Max(sa, sb)
	bot := math.Min(sa, sb)
	return IchimokuSnapshot{
		Tenkan:      tk,
		Kijun:       kj,
		SenkouA:     sa,
		SenkouB:     sb,
		CloudTop:    top,
		CloudBot:    bot,
		ChikouClose: closes[L],
		Displace:    displace,
	}, true
}

func midRange(highs, lows []float64, end, period int) (float64, bool) {
	start := end - period + 1
	if start < 0 {
		return 0, false
	}
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
	return (hh + ll) / 2, true
}
