package compute

// OBVLast returns on-balance volume after processing all bars (oldest first) and the
// delta contributed by the final bar (volume signed by close vs previous close).
func OBVLast(bars []Bar) (total float64, lastDelta float64, ok bool) {
	if len(bars) < 2 {
		return 0, 0, false
	}
	var obv float64
	for i := 1; i < len(bars); i++ {
		v := bars[i].Volume
		switch {
		case bars[i].Close > bars[i-1].Close:
			obv += v
		case bars[i].Close < bars[i-1].Close:
			obv -= v
		}
	}
	prev, cur := bars[len(bars)-2].Close, bars[len(bars)-1].Close
	vol := bars[len(bars)-1].Volume
	switch {
	case cur > prev:
		lastDelta = vol
	case cur < prev:
		lastDelta = -vol
	default:
		lastDelta = 0
	}
	return obv, lastDelta, true
}
