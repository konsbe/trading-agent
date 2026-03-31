package compute

import "math"

// FibRetracementRule documents how the swing leg is chosen for retracement levels.
//
// 1. Take the last swing-high index and last swing-low index in the window (same pivot rules as S/R).
// 2. If the swing high occurs after the swing low (iH > iL), treat the completed impulse as UP
//    from low → high; retracements are measured down from the high toward the low.
// 3. If the swing low occurs after the swing high (iL > iH), treat the impulse as DOWN from
//    high → low; retracements are measured up from the low toward the high.
// 4. Ratios: 0, 0.236, 0.382, 0.5, 0.618, 0.786, 1.0 map linearly along the leg.
// 5. Extensions (optional): 1.272 and 1.618 of the leg length beyond the impulse extreme.
type FibRetracementResult struct {
	Direction     string             // "up_impulse" or "down_impulse"
	ImpulseLow    float64            // price at start of impulse leg
	ImpulseHigh   float64            // price at end of impulse leg
	LegSize       float64            // absolute range of the impulse
	Levels        map[string]float64 // e.g. "0.382" -> price
	Extensions    map[string]float64 // "1.272", "1.618" beyond impulse end
	CurrentClose  float64
	NearestLevel  string
	NearestPrice  float64
	DistPctToNear float64 // distance from close to nearest level as % of leg size
}

var defaultFibRatios = []float64{0, 0.236, 0.382, 0.5, 0.618, 0.786, 1.0}

// FibRetracementFromSwings builds retracement/extension prices from the last swing high/low.
func FibRetracementFromSwings(highs, lows, closes []float64, strength int, ext bool) (FibRetracementResult, bool) {
	if len(highs) != len(lows) || len(closes) != len(highs) {
		return FibRetracementResult{}, false
	}
	hi := SwingHighIndices(highs, strength)
	lo := SwingLowIndices(lows, strength)
	if len(hi) == 0 || len(lo) == 0 {
		return FibRetracementResult{}, false
	}
	iH := hi[len(hi)-1]
	iL := lo[len(lo)-1]
	last := closes[len(closes)-1]

	var impulseLow, impulseHigh float64
	var dir string
	if iH > iL {
		dir = "up_impulse"
		impulseLow = lows[iL]
		impulseHigh = highs[iH]
	} else if iL > iH {
		dir = "down_impulse"
		impulseHigh = highs[iH]
		impulseLow = lows[iL]
	} else {
		return FibRetracementResult{}, false
	}

	leg := impulseHigh - impulseLow
	if leg <= 0 || math.IsNaN(leg) {
		return FibRetracementResult{}, false
	}

	levels := make(map[string]float64)
	for _, r := range defaultFibRatios {
		key := fibKey(r)
		if dir == "up_impulse" {
			// Retrace down from high toward low: price = high - r * leg
			levels[key] = impulseHigh - r*leg
		} else {
			levels[key] = impulseLow + r*leg
		}
	}

	exts := make(map[string]float64)
	if ext {
		if dir == "up_impulse" {
			exts["1.272"] = impulseHigh + 0.272*leg
			exts["1.618"] = impulseHigh + 0.618*leg
		} else {
			exts["1.272"] = impulseLow - 0.272*leg
			exts["1.618"] = impulseLow - 0.618*leg
		}
	}

	nearestName, nearestPx, distPct := nearestFibLevel(last, levels, leg)

	return FibRetracementResult{
		Direction:     dir,
		ImpulseLow:    impulseLow,
		ImpulseHigh:   impulseHigh,
		LegSize:       leg,
		Levels:        levels,
		Extensions:    exts,
		CurrentClose:  last,
		NearestLevel:  nearestName,
		NearestPrice:  nearestPx,
		DistPctToNear: distPct,
	}, true
}

func fibKey(r float64) string {
	// stable string keys for JSON
	switch r {
	case 0:
		return "0"
	case 0.236:
		return "0.236"
	case 0.382:
		return "0.382"
	case 0.5:
		return "0.5"
	case 0.618:
		return "0.618"
	case 0.786:
		return "0.786"
	case 1.0:
		return "1"
	default:
		return "custom"
	}
}

func nearestFibLevel(close float64, levels map[string]float64, leg float64) (name string, price float64, distPct float64) {
	var best float64 = 1e99
	for k, p := range levels {
		d := math.Abs(close - p)
		if d < best {
			best = d
			name = k
			price = p
		}
	}
	if leg > 0 {
		distPct = math.Abs(close-price) / leg * 100
	}
	return name, price, distPct
}
