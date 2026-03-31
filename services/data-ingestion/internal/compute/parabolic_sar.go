package compute

import "math"

// ParabolicSARLast returns SAR at the final bar and whether trend is bullish (+1) or bearish (−1).
// step = initial/max acceleration increment (e.g. 0.02), maxAF caps acceleration (e.g. 0.2).
func ParabolicSARLast(highs, lows, closes []float64, step, maxAF float64) (sar float64, bull bool, ok bool) {
	n := len(closes)
	if n < 3 || step <= 0 || maxAF < step {
		return 0, false, false
	}
	if len(highs) != n || len(lows) != n {
		return 0, false, false
	}

	bull = closes[1] > closes[0]
	var ep, af float64
	if bull {
		sar = lows[0]
		ep = highs[0]
		if highs[1] > ep {
			ep = highs[1]
		}
	} else {
		sar = highs[0]
		ep = lows[0]
		if lows[1] < ep {
			ep = lows[1]
		}
	}
	af = step

	for i := 2; i < n; i++ {
		prevSar := sar
		if bull {
			sar = prevSar + af*(ep-prevSar)
			sar = math.Min(sar, lows[i-1])
			if i >= 3 {
				sar = math.Min(sar, lows[i-2])
			}
			if lows[i] < sar {
				bull = false
				sar = math.Max(ep, prevSar)
				ep = lows[i]
				af = step
			} else {
				if highs[i] > ep {
					ep = highs[i]
					af = math.Min(af+step, maxAF)
				}
			}
		} else {
			sar = prevSar + af*(ep-prevSar)
			sar = math.Max(sar, highs[i-1])
			if i >= 3 {
				sar = math.Max(sar, highs[i-2])
			}
			if highs[i] > sar {
				bull = true
				sar = math.Min(ep, prevSar)
				ep = highs[i]
				af = step
			} else {
				if lows[i] < ep {
					ep = lows[i]
					af = math.Min(af+step, maxAF)
				}
			}
		}
	}
	return sar, bull, true
}
