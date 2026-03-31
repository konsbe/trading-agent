package compute

import "math"

// ADLineLast returns the cumulative Accumulation/Distribution line at the last bar
// (Marc Chaikin / standard close-location weighted flow).
func ADLineLast(bars []Bar) (float64, bool) {
	if len(bars) == 0 {
		return 0, false
	}
	var ad float64
	for _, b := range bars {
		hl := b.High - b.Low
		var mfm float64
		if hl <= 0 || math.IsNaN(hl) {
			mfm = 0
		} else {
			mfm = ((b.Close - b.Low) - (b.High - b.Close)) / hl
		}
		ad += mfm * b.Volume
	}
	return ad, true
}
