package compute

import "sort"

// Level is a price level and the number of times it was tested (touches).
type Level struct {
	Price   float64
	Touches int
}

// SRResult holds support and resistance levels relative to the current price.
type SRResult struct {
	Support    []Level // below current price, nearest first
	Resistance []Level // above current price, nearest first
}

// FindSR identifies support and resistance levels from swing highs and lows.
//
//   - strength: bars on each side required to qualify a swing point
//   - nLevels:  max levels to return per side
//   - clusterPct: price proximity tolerance (%) used to group nearby levels
func FindSR(highs, lows []float64, currentPrice float64, strength, nLevels int, clusterPct float64) SRResult {
	swingH := swingHighPrices(highs, strength)
	swingL := swingLowPrices(lows, strength)
	all := append(swingH, swingL...) //nolint:gocritic
	clusters := clusterLevels(all, clusterPct)

	var support, resistance []Level
	for _, c := range clusters {
		if c.Price < currentPrice {
			support = append(support, c)
		} else {
			resistance = append(resistance, c)
		}
	}

	// Sort each side so the nearest level comes first.
	sort.Slice(support, func(a, b int) bool { return support[a].Price > support[b].Price })
	sort.Slice(resistance, func(a, b int) bool { return resistance[a].Price < resistance[b].Price })

	if len(support) > nLevels {
		support = support[:nLevels]
	}
	if len(resistance) > nLevels {
		resistance = resistance[:nLevels]
	}
	return SRResult{Support: support, Resistance: resistance}
}

func swingHighPrices(highs []float64, strength int) []float64 {
	ix := SwingHighIndices(highs, strength)
	out := make([]float64, len(ix))
	for i, j := range ix {
		out[i] = highs[j]
	}
	return out
}

func swingLowPrices(lows []float64, strength int) []float64 {
	ix := SwingLowIndices(lows, strength)
	out := make([]float64, len(ix))
	for i, j := range ix {
		out[i] = lows[j]
	}
	return out
}

// clusterLevels groups prices within tolerancePct% of each other and returns
// the resulting clusters sorted by touch count descending.
func clusterLevels(prices []float64, tolerancePct float64) []Level {
	if len(prices) == 0 {
		return nil
	}
	sorted := make([]float64, len(prices))
	copy(sorted, prices)
	sort.Float64s(sorted)

	used := make([]bool, len(sorted))
	var clusters []Level

	for i := 0; i < len(sorted); i++ {
		if used[i] {
			continue
		}
		c := Level{Price: sorted[i], Touches: 1}
		used[i] = true
		tol := sorted[i] * tolerancePct / 100
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j]-sorted[i] <= tol {
				// Running mean to keep the cluster center accurate.
				c.Price = (c.Price*float64(c.Touches) + sorted[j]) / float64(c.Touches+1)
				c.Touches++
				used[j] = true
			}
		}
		clusters = append(clusters, c)
	}

	sort.Slice(clusters, func(a, b int) bool {
		return clusters[a].Touches > clusters[b].Touches
	})
	return clusters
}
