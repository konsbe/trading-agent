package compute

import "math"

// RibbonResult summarises ordered SMAs and compression.
type RibbonResult struct {
	Periods       []int
	SMAs          []float64
	BullStack     bool // strictly increasing SMAs with shorter period > longer (fast above slow)?
	BearStack     bool // strictly decreasing
	Compression   float64 // (max-min)/mean of SMAs when mean!=0
	GoldenCross   bool    // SMA50 crosses above SMA200 vs prior bar
	DeathCross    bool    // SMA50 crosses below SMA200 vs prior bar
	CrossFast     int
	CrossSlow     int
}

// MARibbonEval evaluates strictly increasing `periods` (e.g. 10,20,50,200).
// BullStack: sma[0] > sma[1] > ... (shorter > longer = bullish stack).
func MARibbonEval(closes []float64, periods []int, crossFast, crossSlow int) (RibbonResult, bool) {
	if len(periods) < 2 {
		return RibbonResult{}, false
	}
	maxP := periods[0]
	for _, p := range periods {
		if p > maxP {
			maxP = p
		}
	}
	if len(closes) < maxP+1 {
		return RibbonResult{}, false
	}
	smas := make([]float64, len(periods))
	for i, p := range periods {
		v, ok := SMA(closes, p)
		if !ok {
			return RibbonResult{}, false
		}
		smas[i] = v
	}
	bull := true
	bear := true
	for i := 0; i < len(smas)-1; i++ {
		if smas[i] <= smas[i+1] {
			bull = false
		}
		if smas[i] >= smas[i+1] {
			bear = false
		}
	}
	minS, maxS := smas[0], smas[0]
	var sum float64
	for _, s := range smas {
		if s < minS {
			minS = s
		}
		if s > maxS {
			maxS = s
		}
		sum += s
	}
	mean := sum / float64(len(smas))
	var comp float64
	if mean != 0 {
		comp = (maxS - minS) / math.Abs(mean)
	}

	var gc, dc bool
	if crossFast > 0 && crossSlow > 0 && len(closes) >= crossSlow+2 {
		fNow, ok1 := SMA(closes, crossFast)
		sNow, ok2 := SMA(closes, crossSlow)
		prev := closes[:len(closes)-1]
		fPrev, ok3 := SMA(prev, crossFast)
		sPrev, ok4 := SMA(prev, crossSlow)
		if ok1 && ok2 && ok3 && ok4 {
			gc = fPrev <= sPrev && fNow > sNow
			dc = fPrev >= sPrev && fNow < sNow
		}
	}

	return RibbonResult{
		Periods:     append([]int(nil), periods...),
		SMAs:        smas,
		BullStack:   bull,
		BearStack:   bear,
		Compression: comp,
		GoldenCross: gc,
		DeathCross:  dc,
		CrossFast:   crossFast,
		CrossSlow:   crossSlow,
	}, true
}
