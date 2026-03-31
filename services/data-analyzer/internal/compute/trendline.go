package compute

// TrendlineBreakResult fits OLS lines through the last `nPivots` swing highs / lows and
// compares the current vs previous close to the extrapolated line at the chart index of the last bar.
type TrendlineBreakResult struct {
	ResistanceBreak bool // close crossed above descending/flat high-line
	SupportBreak    bool // close crossed below ascending/flat low-line
	HighLineAtEnd   float64
	LowLineAtEnd    float64
	PrevHighLine    float64
	PrevLowLine     float64
}

// minPivots is clamped to >=2; uses last k swing highs and lows separately.
func TrendlineBreakLast(highs, lows, closes []float64, strength, minPivots int) (TrendlineBreakResult, bool) {
	var out TrendlineBreakResult
	n := len(closes)
	if n < 5 || strength < 1 || len(highs) != n || len(lows) != n {
		return out, false
	}
	if minPivots < 2 {
		minPivots = 2
	}
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)
	if len(hi) < minPivots || len(lo) < minPivots {
		return out, false
	}
	hi = hi[len(hi)-minPivots:]
	lo = lo[len(lo)-minPivots:]

	hx := make([]float64, len(hi))
	hy := make([]float64, len(hi))
	for i, ix := range hi {
		hx[i] = float64(ix)
		hy[i] = highs[ix]
	}
	lx := make([]float64, len(lo))
	ly := make([]float64, len(lo))
	for i, ix := range lo {
		lx[i] = float64(ix)
		ly[i] = lows[ix]
	}

	aH, bH, ok1 := olsLine(hx, hy)
	aL, bL, ok2 := olsLine(lx, ly)
	if !ok1 || !ok2 {
		return out, false
	}
	xEnd := float64(n - 1)
	xPrev := float64(n - 2)
	out.HighLineAtEnd = aH + bH*xEnd
	out.LowLineAtEnd = aL + bL*xEnd
	out.PrevHighLine = aH + bH*xPrev
	out.PrevLowLine = aL + bL*xPrev

	last := closes[n-1]
	prev := closes[n-2]
	// Resistance break: price closes above the projected high line (often downward-sloping supply).
	if last > out.HighLineAtEnd && prev <= out.PrevHighLine {
		out.ResistanceBreak = true
	}
	// Support break: price closes below projected low line (often upward-sloping demand).
	if last < out.LowLineAtEnd && prev >= out.PrevLowLine {
		out.SupportBreak = true
	}
	return out, true
}

func olsLine(x, y []float64) (a, b float64, ok bool) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, false
	}
	m := float64(len(x))
	var sx, sy, sxx, sxy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
		sxx += x[i] * x[i]
		sxy += x[i] * y[i]
	}
	denom := m*sxx - sx*sx
	if denom == 0 {
		return 0, 0, false
	}
	b = (m*sxy - sx*sy) / denom
	a = (sy - b*sx) / m
	return a, b, true
}
