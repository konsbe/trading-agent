package compute

// DonchianLast returns highest high and lowest low over the last `period` bars (inclusive).
func DonchianLast(highs, lows []float64, period int) (upper, lower, mid float64, ok bool) {
	if period <= 0 || len(highs) != len(lows) || len(highs) < period {
		return 0, 0, 0, false
	}
	start := len(highs) - period
	hh := highs[start]
	ll := lows[start]
	for i := start + 1; i < len(highs); i++ {
		if highs[i] > hh {
			hh = highs[i]
		}
		if lows[i] < ll {
			ll = lows[i]
		}
	}
	return hh, ll, (hh + ll) / 2, true
}
