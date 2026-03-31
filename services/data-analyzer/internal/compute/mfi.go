package compute

// MFILast is the Money Flow Index over `period` bars (typical price × volume).
func MFILast(bars []Bar, period int) (float64, bool) {
	if period <= 0 || len(bars) < period+1 {
		return 0, false
	}
	var pos, neg float64
	start := len(bars) - period - 1
	for i := start + 1; i < len(bars); i++ {
		tp0 := (bars[i-1].High + bars[i-1].Low + bars[i-1].Close) / 3
		tp1 := (bars[i].High + bars[i].Low + bars[i].Close) / 3
		raw := tp1 * bars[i].Volume
		if tp1 > tp0 {
			pos += raw
		} else if tp1 < tp0 {
			neg += raw
		}
	}
	if neg == 0 {
		if pos == 0 {
			return 50, true
		}
		return 100, true
	}
	mfr := pos / neg
	return 100 - 100/(1+mfr), true
}
