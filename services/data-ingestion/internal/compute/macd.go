package compute

import "math"

// MACDResult holds MACD line, signal line, and histogram for one bar.
type MACDResult struct {
	Line     float64
	Signal   float64
	Hist     float64
	StartIdx int // first bar index where MACD line is defined (same indexing as closes)
}

// macdSignalSeries builds MACD line (fast EMA − slow EMA) and signal EMA on the MACD line.
// Returns macd[i], sig[i] for global indices; NaN where undefined.
func macdSignalSeries(closes []float64, fast, slow, signal int) (macd, sig []float64, start int, ok bool) {
	if fast <= 0 || slow <= 0 || signal <= 0 {
		return nil, nil, 0, false
	}
	maxFS := slow
	if fast > slow {
		maxFS = fast
	}
	if len(closes) < maxFS+signal {
		return nil, nil, 0, false
	}
	n := len(closes)
	fe := EMASeries(closes, fast)
	se := EMASeries(closes, slow)
	start = maxFS - 1
	macd = make([]float64, n)
	sig = make([]float64, n)
	for i := range macd {
		macd[i] = math.NaN()
		sig[i] = math.NaN()
	}
	for i := start; i < n; i++ {
		if !math.IsNaN(fe[i]) && !math.IsNaN(se[i]) {
			macd[i] = fe[i] - se[i]
		}
	}
	trim := macd[start:]
	sigTrim := EMASeries(trim, signal)
	if len(sigTrim) == 0 {
		return nil, nil, 0, false
	}
	for j := range sigTrim {
		if math.IsNaN(sigTrim[j]) {
			continue
		}
		sig[start+j] = sigTrim[j]
	}
	return macd, sig, start, true
}

// MACDLast computes MACD for the final bar.
func MACDLast(closes []float64, fast, slow, signal int) (MACDResult, bool) {
	macd, sig, start, ok := macdSignalSeries(closes, fast, slow, signal)
	if !ok {
		return MACDResult{}, false
	}
	n := len(closes)
	li := n - 1
	if math.IsNaN(macd[li]) || math.IsNaN(sig[li]) {
		return MACDResult{}, false
	}
	line := macd[li]
	s := sig[li]
	return MACDResult{
		Line:     line,
		Signal:   s,
		Hist:     line - s,
		StartIdx: start,
	}, true
}

// MACDSnapshot includes current and prior bar MACD plus crossover / histogram flip flags.
type MACDSnapshot struct {
	Cur  MACDResult
	Prev MACDResult // prior closed bar; ok false if unavailable

	PrevBarAvailable bool

	// Line vs signal crosses (strict: equality does not count as cross).
	BullishCross bool
	BearishCross bool

	// Histogram crosses zero.
	HistBullZeroCross bool
	HistBearZeroCross bool
}

// MACDSnapshotWithPrev computes MACD on the last bar and the bar before it without re-querying DB.
func MACDSnapshotWithPrev(closes []float64, fast, slow, signal int) (MACDSnapshot, bool) {
	macd, sig, start, ok := macdSignalSeries(closes, fast, slow, signal)
	if !ok {
		return MACDSnapshot{}, false
	}
	n := len(closes)
	li := n - 1
	if math.IsNaN(macd[li]) || math.IsNaN(sig[li]) {
		return MACDSnapshot{}, false
	}
	cur := MACDResult{
		Line:     macd[li],
		Signal:   sig[li],
		Hist:     macd[li] - sig[li],
		StartIdx: start,
	}
	var snap MACDSnapshot
	snap.Cur = cur
	snap.PrevBarAvailable = false

	pi := n - 2
	if pi >= start && !math.IsNaN(macd[pi]) && !math.IsNaN(sig[pi]) {
		snap.Prev = MACDResult{
			Line:     macd[pi],
			Signal:   sig[pi],
			Hist:     macd[pi] - sig[pi],
			StartIdx: start,
		}
		snap.PrevBarAvailable = true
		pl, ps, ph := snap.Prev.Line, snap.Prev.Signal, snap.Prev.Hist
		cl, cs, ch := snap.Cur.Line, snap.Cur.Signal, snap.Cur.Hist
		snap.BullishCross = pl <= ps && cl > cs
		snap.BearishCross = pl >= ps && cl < cs
		snap.HistBullZeroCross = ph <= 0 && ch > 0
		snap.HistBearZeroCross = ph >= 0 && ch < 0
	}
	return snap, true
}
