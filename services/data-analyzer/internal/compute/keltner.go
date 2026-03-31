package compute

// KeltnerLast: middle = EMA(close, emaN), width = mult × ATR(atrN).
func KeltnerLast(highs, lows, closes []float64, emaN, atrN int, mult float64) (mid, upper, lower float64, ok bool) {
	if mult <= 0 || emaN <= 0 || atrN <= 0 {
		return 0, 0, 0, false
	}
	mid, ok = EMA(closes, emaN)
	if !ok {
		return 0, 0, 0, false
	}
	atr, ok2 := ATRWilder(highs, lows, closes, atrN)
	if !ok2 {
		return 0, 0, 0, false
	}
	w := mult * atr
	return mid, mid + w, mid - w, true
}
