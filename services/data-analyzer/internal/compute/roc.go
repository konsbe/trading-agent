package compute

// ROCLast is (close − close[n]) / close[n] × 100.
func ROCLast(closes []float64, n int) (float64, bool) {
	if n <= 0 || len(closes) < n+1 {
		return 0, false
	}
	cur := closes[len(closes)-1]
	prev := closes[len(closes)-1-n]
	if prev == 0 {
		return 0, false
	}
	return (cur - prev) / prev * 100, true
}
