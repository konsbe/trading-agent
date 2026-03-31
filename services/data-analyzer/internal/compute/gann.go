package compute

import "math"

// GannGeometryFromRegression projects a 1×1-style price delta from linear regression slope
// over `lookback` closes. This is a coarse OHLCV proxy, not full Gann square-of-nine mechanics.
type GannGeometryFromRegression struct {
	SlopePerBar   float64 // raw price change per bar index
	SlopeDegrees  float64 // atan(slope) in degrees (not same as true geometric angle without price/time scaling)
	OneToOneDelta float64 // slopePerBar × lookback (price units if time axis is 1 bar = 1 unit)
	Lookback      int
}

// GannRegressionHint fits closes over the last `lookback` bars.
func GannRegressionHint(closes []float64, lookback int) (GannGeometryFromRegression, bool) {
	var g GannGeometryFromRegression
	if lookback < 3 || len(closes) < lookback {
		return g, false
	}
	slice := closes[len(closes)-lookback:]
	x := make([]float64, lookback)
	y := make([]float64, lookback)
	for i := range slice {
		x[i] = float64(i)
		y[i] = slice[i]
	}
	_, b, ok := olsLine(x, y)
	if !ok {
		return g, false
	}
	g.SlopePerBar = b
	g.SlopeDegrees = math.Atan(b) * 180 / math.Pi
	g.OneToOneDelta = b * float64(lookback-1)
	g.Lookback = lookback
	return g, true
}
