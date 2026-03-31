package compute

import "time"

// AlignClosesByTimestamp pairs bars with identical TS (wall-clock equality).
// Returns parallel close series for overlapping timestamps only, oldest-first order preserved.
func AlignClosesByTimestamp(asset, other []Bar) (assetC, benchC []float64, times []time.Time) {
	j := 0
	for i := range asset {
		for j < len(other) && other[j].TS.Before(asset[i].TS) {
			j++
		}
		if j >= len(other) {
			break
		}
		if other[j].TS.Equal(asset[i].TS) {
			assetC = append(assetC, asset[i].Close)
			benchC = append(benchC, other[j].Close)
			times = append(times, asset[i].TS)
		}
	}
	return assetC, benchC, times
}
