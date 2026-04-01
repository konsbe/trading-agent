package compute

// Order Blocks & Liquidity Sweeps — Smart Money Concepts (SMC)
//
// Order Block: the last opposing candle before a significant impulse move.
// Institutions are believed to have placed large orders at these levels. Price
// frequently returns to test them ("mitigation") before continuing.
//
//   Bullish OB: last bearish candle before a strong bullish impulse → support
//   Bearish OB: last bullish candle before a strong bearish impulse → resistance
//
// Liquidity Sweep: price briefly breaks a prior swing high/low (triggering
// stop-loss orders) but closes back on the original side — a classic stop-hunt
// by institutional players before a directional move.

// OBKind classifies the direction of an Order Block.
type OBKind string

const (
	OBBullish OBKind = "bullish"
	OBBearish OBKind = "bearish"
)

// OrderBlock represents one detected Order Block.
type OrderBlock struct {
	BarIndex    int
	Kind        OBKind
	High        float64
	Low         float64
	Open        float64
	Close       float64
	Invalidated bool // price closed through the OB (OB is "mitigated / consumed")
}

// OrderBlocksResult holds detected Order Blocks.
type OrderBlocksResult struct {
	All         []OrderBlock
	LastBullish *OrderBlock // nearest active bullish OB (potential support)
	LastBearish *OrderBlock // nearest active bearish OB (potential resistance)
}

// DetectOrderBlocks finds Order Blocks by identifying the last opposing candle
// before each confirmed swing-based impulse move.
//
//   - swingStrength: bars on each side required to confirm a swing pivot.
//   - impulsePct:    minimum % move from swing to confirm an impulse (e.g. 1.5).
//   - lookback:      max bars to scan (0 = full series).
func DetectOrderBlocks(bars []Bar, swingStrength int, impulsePct float64, lookback int) OrderBlocksResult {
	n := len(bars)
	if n < swingStrength*2+3 {
		return OrderBlocksResult{}
	}

	start := 0
	if lookback > 0 && lookback < n {
		start = n - lookback
	}

	highs := Highs(bars)
	lows := Lows(bars)
	shIdxs := SwingHighIndices(highs, swingStrength)
	slIdxs := SwingLowIndices(lows, swingStrength)

	var all []OrderBlock

	// ── Bullish OBs: swing low → subsequent impulse up ───────────────────────
	for i, slIdx := range slIdxs {
		if slIdx < start {
			continue
		}

		// Measure up-move from swing low to the next swing high (or end of bars).
		nextSH := n - 1
		for _, shIdx := range shIdxs {
			if shIdx > slIdx {
				nextSH = shIdx
				break
			}
		}

		swingLowPrice := lows[slIdx]
		if swingLowPrice <= 0 {
			continue
		}
		postHigh := swingLowPrice
		for k := slIdx + 1; k <= nextSH && k < n; k++ {
			if highs[k] > postHigh {
				postHigh = highs[k]
			}
		}
		moveUpPct := (postHigh - swingLowPrice) / swingLowPrice * 100
		if moveUpPct < impulsePct {
			continue
		}

		// Determine search boundary: back to the previous swing high (or start).
		searchFrom := start
		if i > 0 {
			for j := len(shIdxs) - 1; j >= 0; j-- {
				if shIdxs[j] < slIdx {
					searchFrom = shIdxs[j]
					break
				}
			}
		}

		// Last bearish candle at or before the swing low = Bullish OB.
		for k := slIdx; k >= searchFrom; k-- {
			b := bars[k]
			if b.Close < b.Open {
				ob := OrderBlock{
					BarIndex: k,
					Kind:     OBBullish,
					High:     b.High,
					Low:      b.Low,
					Open:     b.Open,
					Close:    b.Close,
				}
				// Invalidated when price closes below ob.Low after the impulse.
				for j := slIdx + 1; j < n; j++ {
					if bars[j].Close < ob.Low {
						ob.Invalidated = true
						break
					}
				}
				all = append(all, ob)
				break
			}
		}
	}

	// ── Bearish OBs: swing high → subsequent impulse down ────────────────────
	for i, shIdx := range shIdxs {
		if shIdx < start {
			continue
		}

		nextSL := n - 1
		for _, slIdx := range slIdxs {
			if slIdx > shIdx {
				nextSL = slIdx
				break
			}
		}

		swingHighPrice := highs[shIdx]
		if swingHighPrice <= 0 {
			continue
		}
		postLow := swingHighPrice
		for k := shIdx + 1; k <= nextSL && k < n; k++ {
			if lows[k] < postLow {
				postLow = lows[k]
			}
		}
		moveDownPct := (swingHighPrice - postLow) / swingHighPrice * 100
		if moveDownPct < impulsePct {
			continue
		}

		searchFrom := start
		if i > 0 {
			for j := len(slIdxs) - 1; j >= 0; j-- {
				if slIdxs[j] < shIdx {
					searchFrom = slIdxs[j]
					break
				}
			}
		}

		// Last bullish candle at or before the swing high = Bearish OB.
		for k := shIdx; k >= searchFrom; k-- {
			b := bars[k]
			if b.Close > b.Open {
				ob := OrderBlock{
					BarIndex: k,
					Kind:     OBBearish,
					High:     b.High,
					Low:      b.Low,
					Open:     b.Open,
					Close:    b.Close,
				}
				for j := shIdx + 1; j < n; j++ {
					if bars[j].Close > ob.High {
						ob.Invalidated = true
						break
					}
				}
				all = append(all, ob)
				break
			}
		}
	}

	result := OrderBlocksResult{All: all}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Invalidated {
			continue
		}
		cp := all[i]
		if all[i].Kind == OBBullish && result.LastBullish == nil {
			result.LastBullish = &cp
		}
		if all[i].Kind == OBBearish && result.LastBearish == nil {
			result.LastBearish = &cp
		}
		if result.LastBullish != nil && result.LastBearish != nil {
			break
		}
	}
	return result
}

// ── Liquidity Sweep ───────────────────────────────────────────────────────────

// SweepKind classifies a Liquidity Sweep direction.
type SweepKind string

const (
	SweepHigh SweepKind = "high_sweep" // stop-hunt above prior swing high
	SweepLow  SweepKind = "low_sweep"  // stop-hunt below prior swing low
)

// LiquiditySweep represents one detected liquidity sweep event.
type LiquiditySweep struct {
	BarIndex   int
	Kind       SweepKind
	SweptLevel float64 // the swing high/low that was breached
	BarHigh    float64
	BarLow     float64
	BarClose   float64
}

// DetectLiquiditySweeps identifies bars where price briefly breaks a prior
// swing high or low but closes back on the original side — a stop-hunt.
//
//   - swingStrength: bars on each side to confirm a swing pivot.
//   - lookback:      max bars to scan (0 = full series).
func DetectLiquiditySweeps(bars []Bar, swingStrength, lookback int) []LiquiditySweep {
	n := len(bars)
	if n < swingStrength*2+3 {
		return nil
	}

	start := 0
	if lookback > 0 && lookback < n {
		start = n - lookback
	}

	highs := Highs(bars)
	lows := Lows(bars)
	shIdxs := SwingHighIndices(highs, swingStrength)
	slIdxs := SwingLowIndices(lows, swingStrength)

	var sweeps []LiquiditySweep

	for i := start; i < n; i++ {
		b := bars[i]

		// Most recent prior swing high before bar i.
		var recentSH float64
		for j := len(shIdxs) - 1; j >= 0; j-- {
			if shIdxs[j] < i {
				recentSH = highs[shIdxs[j]]
				break
			}
		}

		// Most recent prior swing low before bar i.
		var recentSL float64
		for j := len(slIdxs) - 1; j >= 0; j-- {
			if slIdxs[j] < i {
				recentSL = lows[slIdxs[j]]
				break
			}
		}

		// High sweep: wick above prior swing high but close below it.
		if recentSH > 0 && b.High > recentSH && b.Close < recentSH {
			sweeps = append(sweeps, LiquiditySweep{
				BarIndex:   i,
				Kind:       SweepHigh,
				SweptLevel: recentSH,
				BarHigh:    b.High,
				BarLow:     b.Low,
				BarClose:   b.Close,
			})
		}

		// Low sweep: wick below prior swing low but close above it.
		if recentSL > 0 && b.Low < recentSL && b.Close > recentSL {
			sweeps = append(sweeps, LiquiditySweep{
				BarIndex:   i,
				Kind:       SweepLow,
				SweptLevel: recentSL,
				BarHigh:    b.High,
				BarLow:     b.Low,
				BarClose:   b.Close,
			})
		}
	}
	return sweeps
}
