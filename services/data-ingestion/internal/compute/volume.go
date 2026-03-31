package compute

// VolumeSMA returns the SMA of volume over the last `period` bars.
func VolumeSMA(volumes []float64, period int) (float64, bool) {
	return SMA(volumes, period)
}

// RelativeVolume returns the current bar's volume divided by the SMA of the preceding `period` bars.
// A value > 1 indicates above-average participation; < 1 indicates below-average.
// Returns (0, false) when there is insufficient data or the SMA is zero.
func RelativeVolume(volumes []float64, period int) (float64, bool) {
	if len(volumes) < period+1 {
		return 0, false
	}
	// SMA of the `period` bars that precede the current bar.
	preceding := volumes[:len(volumes)-1]
	sma, ok := SMA(preceding, period)
	if !ok || sma == 0 {
		return 0, false
	}
	return volumes[len(volumes)-1] / sma, true
}
