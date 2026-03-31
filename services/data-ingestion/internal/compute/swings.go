package compute

// SwingHighIndices returns bar indices i (oldest-first series) where highs[i] is
// strictly greater than all highs within ±strength bars — same definition as the
// swing points used for support/resistance clustering.
func SwingHighIndices(highs []float64, strength int) []int {
	if strength < 1 || len(highs) < strength*2+1 {
		return nil
	}
	var out []int
	for i := strength; i < len(highs)-strength; i++ {
		pivot := true
		for j := i - strength; j <= i+strength; j++ {
			if j != i && highs[j] >= highs[i] {
				pivot = false
				break
			}
		}
		if pivot {
			out = append(out, i)
		}
	}
	return out
}

// SwingLowIndices returns bar indices i where lows[i] is strictly lower than all
// lows within ±strength bars.
func SwingLowIndices(lows []float64, strength int) []int {
	if strength < 1 || len(lows) < strength*2+1 {
		return nil
	}
	var out []int
	for i := strength; i < len(lows)-strength; i++ {
		pivot := true
		for j := i - strength; j <= i+strength; j++ {
			if j != i && lows[j] <= lows[i] {
				pivot = false
				break
			}
		}
		if pivot {
			out = append(out, i)
		}
	}
	return out
}
