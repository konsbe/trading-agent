package compute

// ElliottContextHint is not wave counting; it exposes zig-zag pivot density as a structural hint.
type ElliottContextHint struct {
	SwingHighCount int
	SwingLowCount  int
	LegEstimate    int // alternating pivots ≈ legs
	Note           string
}

// ElliottContextFromSwings counts pivots in the window — subjective wave labels are not inferred.
func ElliottContextFromSwings(highs, lows []float64, strength int) (ElliottContextHint, bool) {
	var h ElliottContextHint
	h.Note = "Not Elliott wave labels; pivot-count hint only. Human discretion required for wave degree."
	if strength < 1 || len(highs) != len(lows) || len(highs) < strength*2+1 {
		return h, false
	}
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)
	h.SwingHighCount = len(hi)
	h.SwingLowCount = len(lo)
	h.LegEstimate = len(hi) + len(lo)
	return h, true
}
