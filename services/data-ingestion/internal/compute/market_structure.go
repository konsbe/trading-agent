package compute

// MarketStructureResult encodes simple BOS-style breaks using recent swing pivots.
type MarketStructureResult struct {
	BullishBOS bool // close > prior swing high (second-last swing high)
	BearishBOS bool // close < prior swing low
	CHoCHUp    bool // after a down sequence, close takes out last significant high (heuristic)
	CHoCHDown  bool
	LastSwingHigh float64
	LastSwingLow  float64
	PriorSwingHigh float64
	PriorSwingLow  float64
}

// MarketStructureLast uses the last two swing highs / lows for break-of-structure hints.
func MarketStructureLast(highs, lows, closes []float64, strength int) (MarketStructureResult, bool) {
	var out MarketStructureResult
	n := len(closes)
	if n < 5 || strength < 1 || len(highs) != n || len(lows) != n {
		return out, false
	}
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)
	if len(hi) < 2 || len(lo) < 2 {
		return out, false
	}
	iH1, iH2 := hi[len(hi)-2], hi[len(hi)-1]
	iL1, iL2 := lo[len(lo)-2], lo[len(lo)-1]
	out.PriorSwingHigh = highs[iH1]
	out.LastSwingHigh = highs[iH2]
	out.PriorSwingLow = lows[iL1]
	out.LastSwingLow = lows[iL2]

	last := closes[n-1]
	prev := closes[n-2]

	// Classic BOS: break beyond the swing before the most recent one on the relevant side.
	out.BullishBOS = last > out.PriorSwingHigh && prev <= out.PriorSwingHigh
	out.BearishBOS = last < out.PriorSwingLow && prev >= out.PriorSwingLow

	// Break of the most recent swing extreme (often discussed alongside CHoCH in discretion-based MS).
	out.CHoCHUp = last > out.LastSwingHigh && prev <= out.LastSwingHigh
	out.CHoCHDown = last < out.LastSwingLow && prev >= out.LastSwingLow

	return out, true
}
