package compute

import "math"

// VolumeProfileBin is one price bucket in a histogram built from bar-only OHLCV.
// Volume is accumulated at the bar's typical price (H+L+C)/3 or close when UseTypical is false.
type VolumeProfileBin struct {
	PriceLow  float64
	PriceHigh float64
	Volume    float64
}

// VolumeProfileProxy builds a fixed-bin histogram over [min(low), max(high)] of the window.
// This is a rough proxy: each bar's entire volume is assigned to a single price bucket, not true
// volume-at-price from tape or L2.
func VolumeProfileProxy(bars []Bar, bins int, useTypical bool) ([]VolumeProfileBin, float64, int, bool) {
	if bins < 2 || len(bars) == 0 {
		return nil, 0, -1, false
	}
	lo, hi := bars[0].Low, bars[0].High
	for _, b := range bars {
		if b.Low < lo {
			lo = b.Low
		}
		if b.High > hi {
			hi = b.High
		}
	}
	if hi <= lo {
		// Degenerate range: widen slightly so bins exist.
		pad := math.Max(lo*1e-6, 1e-8)
		lo -= pad
		hi += pad
	}
	step := (hi - lo) / float64(bins)
	if step <= 0 {
		return nil, 0, -1, false
	}

	out := make([]VolumeProfileBin, bins)
	for i := range out {
		out[i].PriceLow = lo + float64(i)*step
		out[i].PriceHigh = lo + float64(i+1)*step
	}

	for _, b := range bars {
		var px float64
		if useTypical {
			px = (b.High + b.Low + b.Close) / 3
		} else {
			px = b.Close
		}
		idx := int((px - lo) / step)
		if idx < 0 {
			idx = 0
		}
		if idx >= bins {
			idx = bins - 1
		}
		out[idx].Volume += b.Volume
	}

	maxV := -1.0
	poc := -1
	for i := range out {
		if out[i].Volume > maxV {
			maxV = out[i].Volume
			poc = i
		}
	}
	var pocMid float64
	if poc >= 0 {
		pocMid = (out[poc].PriceLow + out[poc].PriceHigh) / 2
	}
	return out, pocMid, poc, true
}
