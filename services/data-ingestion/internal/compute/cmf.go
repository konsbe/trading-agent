package compute

import "math"

// ChaikinMoneyFlow returns sum(CLV×volume) / sum(volume) over the last `period` bars.
// CLV = ((close−low) − (high−close)) / (high−low), 0 if range is 0.
func ChaikinMoneyFlow(bars []Bar, period int) (float64, bool) {
	if period <= 0 || len(bars) < period {
		return 0, false
	}
	seg := bars[len(bars)-period:]
	var num, den float64
	for _, b := range seg {
		hl := b.High - b.Low
		var clv float64
		if hl <= 0 || math.IsNaN(hl) {
			clv = 0
		} else {
			clv = ((b.Close - b.Low) - (b.High - b.Close)) / hl
		}
		num += clv * b.Volume
		den += b.Volume
	}
	if den == 0 {
		return 0, false
	}
	return num / den, true
}
