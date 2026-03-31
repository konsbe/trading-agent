package compute

// PivotSet holds classic / Camarilla / Woodie levels from one prior period's OHLC.
type PivotSet struct {
	PP, R1, R2, R3, S1, S2, S3 float64
	Camarilla                    map[string]float64
	Woodie                       map[string]float64
}

// PivotsFromPriorBar uses the bar at index len−2 as the reference session (H/L/C).
// For the latest bar as "today", this approximates "yesterday's" range when bars are sequential.
func PivotsFromPriorBar(prev Bar) PivotSet {
	h, l, c := prev.High, prev.Low, prev.Close
	pp := (h + l + c) / 3
	r1 := 2*pp - l
	s1 := 2*pp - h
	r2 := pp + (h - l)
	s2 := pp - (h - l)
	r3 := h + 2*(pp-l)
	s3 := l - 2*(h-pp)

	// Camarilla (standard 6 levels + optional R4/S4).
	rng := h - l
	cam := map[string]float64{
		"R4": c + rng*1.1/2.0,
		"R3": c + rng*1.1/4.0,
		"R2": c + rng*1.1/6.0,
		"R1": c + rng*1.1/12.0,
		"S1": c - rng*1.1/12.0,
		"S2": c - rng*1.1/6.0,
		"S3": c - rng*1.1/4.0,
		"S4": c - rng*1.1/2.0,
	}

	// Woodie: pivot uses 2× close weight; R2/S2 use alternate wide formulas.
	wPP := (h + l + 2*c) / 4
	wR1 := 2*wPP - l
	wS1 := 2*wPP - h
	wR2 := wPP + (h - l)
	wS2 := wPP - (h - l)
	wood := map[string]float64{
		"PP": wPP,
		"R1": wR1,
		"S1": wS1,
		"R2": wR2,
		"S2": wS2,
	}

	return PivotSet{
		PP:        pp,
		R1:        r1,
		R2:        r2,
		R3:        r3,
		S1:        s1,
		S2:        s2,
		S3:        s3,
		Camarilla: cam,
		Woodie:    wood,
	}
}
