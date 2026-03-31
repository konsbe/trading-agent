package compute

import "sort"

// ValueAreaAroundPOC expands outward from the POC bin until cumulative volume
// reaches targetPct of total histogram volume (e.g. 0.70 = 70 %).
// Returns the combined price span (min bin low to max bin high) of included bins.
func ValueAreaAroundPOC(bins []VolumeProfileBin, pocIdx int, targetPct float64) (low, high float64, coveredVol, totalVol float64, ok bool) {
	if len(bins) == 0 || pocIdx < 0 || pocIdx >= len(bins) || targetPct <= 0 || targetPct > 1 {
		return 0, 0, 0, 0, false
	}
	var total float64
	for i := range bins {
		total += bins[i].Volume
	}
	if total <= 0 {
		return 0, 0, 0, total, false
	}
	target := targetPct * total

	type cand struct {
		idx int
		d   int // |idx - pocIdx|
	}
	order := make([]cand, len(bins))
	for i := range bins {
		d := i - pocIdx
		if d < 0 {
			d = -d
		}
		order[i] = cand{idx: i, d: d}
	}
	sort.Slice(order, func(a, b int) bool {
		if order[a].d != order[b].d {
			return order[a].d < order[b].d
		}
		return order[a].idx < order[b].idx
	})

	var cum float64
	first := true
	for _, c := range order {
		b := bins[c.idx]
		cum += b.Volume
		if first {
			low, high = b.PriceLow, b.PriceHigh
			first = false
		} else {
			if b.PriceLow < low {
				low = b.PriceLow
			}
			if b.PriceHigh > high {
				high = b.PriceHigh
			}
		}
		if cum >= target {
			return low, high, cum, total, true
		}
	}
	return low, high, cum, total, true
}
