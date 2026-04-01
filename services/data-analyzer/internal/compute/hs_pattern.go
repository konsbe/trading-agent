package compute

import "math"

// HSPatternResult holds detection outcomes for Head & Shoulders patterns.
type HSPatternResult struct {
	// ── Head & Shoulders (bearish reversal) ──────────────────────────────────
	HSFound                bool
	HSLeftShoulder         float64 // left shoulder peak price
	HSHead                 float64 // head peak price
	HSRightShoulder        float64 // right shoulder peak price
	HSNeckline             float64 // support line (close below = pattern confirmed)
	HSShouldersSymmetryPct float64 // % difference between left and right shoulder
	HSNecklineBreak        bool    // current price already broke below neckline

	// ── Inverse Head & Shoulders (bullish reversal) ───────────────────────────
	InvHSFound                bool
	InvHSLeftShoulder         float64
	InvHSHead                 float64
	InvHSRightShoulder        float64
	InvHSNeckline             float64
	InvHSShouldersSymmetryPct float64
	InvHSNecklineBreak        bool // current price already broke above neckline
}

// DetectHSPattern looks for Head & Shoulders and Inverse H&S using swing pivots.
//
//   - swingStrength:  bars on each side to confirm a swing pivot.
//   - tolerancePct:   max % difference allowed between left and right shoulder (e.g. 15.0).
//   - lookback:       max bars to scan (0 = full series).
func DetectHSPattern(bars []Bar, swingStrength int, tolerancePct float64, lookback int) HSPatternResult {
	n := len(bars)
	minBars := swingStrength*6 + 1
	if n < minBars {
		return HSPatternResult{}
	}

	start := 0
	if lookback > 0 && lookback < n {
		start = n - lookback
	}

	highs := Highs(bars)
	lows := Lows(bars)
	shIdxs := SwingHighIndices(highs, swingStrength)
	slIdxs := SwingLowIndices(lows, swingStrength)

	// Filter to only those within the scan window.
	var validSH, validSL []int
	for _, idx := range shIdxs {
		if idx >= start {
			validSH = append(validSH, idx)
		}
	}
	for _, idx := range slIdxs {
		if idx >= start {
			validSL = append(validSL, idx)
		}
	}

	result := HSPatternResult{}
	lastClose := bars[n-1].Close

	// ── Head & Shoulders (bearish) ────────────────────────────────────────────
	// Scan newest triplets first so we capture the most recent pattern.
	for i := len(validSH) - 1; i >= 2 && !result.HSFound; i-- {
		lsIdx := validSH[i-2]
		hIdx := validSH[i-1]
		rsIdx := validSH[i]

		ls := highs[lsIdx]
		h := highs[hIdx]
		rs := highs[rsIdx]

		if ls <= 0 || h <= ls || h <= rs {
			continue
		}

		symPct := math.Abs(rs-ls) / ls * 100
		if symPct > tolerancePct {
			continue
		}

		// Neckline = average of the two trough lows between the three peaks.
		var troughs []float64
		for _, slIdx := range slIdxs {
			if slIdx > lsIdx && slIdx < rsIdx {
				troughs = append(troughs, lows[slIdx])
			}
		}
		if len(troughs) < 1 {
			continue
		}
		neckline := troughs[0]
		for _, t := range troughs[1:] {
			neckline += t
		}
		neckline /= float64(len(troughs))

		result.HSFound = true
		result.HSLeftShoulder = ls
		result.HSHead = h
		result.HSRightShoulder = rs
		result.HSNeckline = neckline
		result.HSShouldersSymmetryPct = symPct
		result.HSNecklineBreak = lastClose < neckline
	}

	// ── Inverse Head & Shoulders (bullish) ────────────────────────────────────
	for i := len(validSL) - 1; i >= 2 && !result.InvHSFound; i-- {
		lsIdx := validSL[i-2]
		hIdx := validSL[i-1]
		rsIdx := validSL[i]

		ls := lows[lsIdx]
		h := lows[hIdx]
		rs := lows[rsIdx]

		if ls <= 0 || h >= ls || h >= rs {
			continue
		}

		symPct := math.Abs(rs-ls) / ls * 100
		if symPct > tolerancePct {
			continue
		}

		var peaks []float64
		for _, shIdx := range shIdxs {
			if shIdx > lsIdx && shIdx < rsIdx {
				peaks = append(peaks, highs[shIdx])
			}
		}
		if len(peaks) < 1 {
			continue
		}
		neckline := peaks[0]
		for _, p := range peaks[1:] {
			neckline += p
		}
		neckline /= float64(len(peaks))

		result.InvHSFound = true
		result.InvHSLeftShoulder = ls
		result.InvHSHead = h
		result.InvHSRightShoulder = rs
		result.InvHSNeckline = neckline
		result.InvHSShouldersSymmetryPct = symPct
		result.InvHSNecklineBreak = lastClose > neckline
	}

	return result
}

// ── Triangle Patterns ─────────────────────────────────────────────────────────

// TriangleKind classifies the detected triangle type.
type TriangleKind string

const (
	TriangleAscending   TriangleKind = "ascending"   // higher lows, flat resistance
	TriangleDescending  TriangleKind = "descending"  // lower highs, flat support
	TriangleSymmetrical TriangleKind = "symmetrical" // lower highs AND higher lows
	TriangleNone        TriangleKind = "none"
)

// TriangleResult holds the detected triangle pattern characteristics.
type TriangleResult struct {
	Kind         TriangleKind
	HighSlopePct float64 // regression slope of swing highs (% per bar vs current price)
	LowSlopePct  float64 // regression slope of swing lows (% per bar vs current price)
	ApexBarsAway int     // estimated bars until resistance/support lines converge
	Breakout     string  // "none" | "up" | "down"
}

// DetectTriangle identifies triangle patterns using linear regression on swing pivots.
//
//   - swingStrength:   bars on each side for swing confirmation.
//   - minPivots:       minimum swing pivots on each side (e.g. 3).
//   - flatThresholdPct: slope (% per bar) below which a trendline is "flat" (e.g. 0.05).
//   - lookback:        max bars to scan (0 = full series).
func DetectTriangle(bars []Bar, swingStrength, minPivots int, flatThresholdPct float64, lookback int) TriangleResult {
	n := len(bars)
	if n < swingStrength*2+minPivots*2 {
		return TriangleResult{Kind: TriangleNone}
	}

	start := 0
	if lookback > 0 && lookback < n {
		start = n - lookback
	}

	highs := Highs(bars)
	lows := Lows(bars)
	shIdxs := SwingHighIndices(highs, swingStrength)
	slIdxs := SwingLowIndices(lows, swingStrength)

	var shPts, slPts [][2]float64
	for _, idx := range shIdxs {
		if idx >= start {
			shPts = append(shPts, [2]float64{float64(idx), highs[idx]})
		}
	}
	for _, idx := range slIdxs {
		if idx >= start {
			slPts = append(slPts, [2]float64{float64(idx), lows[idx]})
		}
	}

	if len(shPts) < minPivots || len(slPts) < minPivots {
		return TriangleResult{Kind: TriangleNone}
	}

	curPrice := bars[n-1].Close
	if curPrice == 0 {
		return TriangleResult{Kind: TriangleNone}
	}

	highSlope := linearSlope(shPts)
	lowSlope := linearSlope(slPts)

	highSlopePct := highSlope / curPrice * 100
	lowSlopePct := lowSlope / curPrice * 100

	kind := TriangleNone
	switch {
	case math.Abs(highSlopePct) < flatThresholdPct && lowSlopePct > flatThresholdPct:
		kind = TriangleAscending
	case highSlopePct < -flatThresholdPct && math.Abs(lowSlopePct) < flatThresholdPct:
		kind = TriangleDescending
	case highSlopePct < -flatThresholdPct && lowSlopePct > flatThresholdPct:
		kind = TriangleSymmetrical
	}

	if kind == TriangleNone {
		return TriangleResult{Kind: TriangleNone}
	}

	// Estimate the apex (where the two trendlines would converge).
	lastSH := shPts[len(shPts)-1]
	lastSL := slPts[len(slPts)-1]
	highIntercept := lastSH[1] - highSlope*lastSH[0]
	lowIntercept := lastSL[1] - lowSlope*lastSL[0]

	apexBars := 0
	denom := highSlope - lowSlope
	if math.Abs(denom) > 1e-9 {
		apexX := (lowIntercept - highIntercept) / denom
		apexBars = int(apexX) - (n - 1)
		if apexBars < 0 {
			apexBars = 0
		}
	}

	// Check whether price has already broken out.
	highLineEnd := highIntercept + highSlope*float64(n-1)
	lowLineEnd := lowIntercept + lowSlope*float64(n-1)
	lastBar := bars[n-1]

	breakout := "none"
	switch {
	case lastBar.High > highLineEnd && lastBar.Close > highLineEnd:
		breakout = "up"
	case lastBar.Low < lowLineEnd && lastBar.Close < lowLineEnd:
		breakout = "down"
	}

	return TriangleResult{
		Kind:         kind,
		HighSlopePct: highSlopePct,
		LowSlopePct:  lowSlopePct,
		ApexBarsAway: apexBars,
		Breakout:     breakout,
	}
}

// linearSlope returns the OLS slope for a set of (x, y) points.
func linearSlope(pts [][2]float64) float64 {
	nf := float64(len(pts))
	if nf < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range pts {
		sumX += p[0]
		sumY += p[1]
		sumXY += p[0] * p[1]
		sumX2 += p[0] * p[0]
	}
	denom := nf*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-9 {
		return 0
	}
	return (nf*sumXY - sumX*sumY) / denom
}

// ── Flag / Pennant ────────────────────────────────────────────────────────────

// FlagResult holds detection results for bull/bear flag patterns.
type FlagResult struct {
	BullFlag bool    // strong up pole + contained pullback consolidation
	BearFlag bool    // strong down pole + contained bounce consolidation
	PolePct  float64 // % magnitude of the flagpole move
}

// DetectFlag checks for bull/bear flag patterns.
//
// A flag requires:
//  1. A strong impulse move ("pole") over poleLen bars ≥ polePct %.
//  2. Followed by a consolidation over flagLen bars where total retracement
//     of the pole does not exceed maxRetracementPct %.
//
//   - polePct:            minimum % move for the pole (e.g. 5.0).
//   - maxRetracementPct:  max allowed retracement of the pole within the flag (e.g. 50.0).
//   - poleLen:            number of bars for the pole phase (e.g. 5).
//   - flagLen:            number of bars for the consolidation phase (e.g. 10).
func DetectFlag(bars []Bar, polePct, maxRetracementPct float64, poleLen, flagLen int) FlagResult {
	n := len(bars)
	if n < poleLen+flagLen {
		return FlagResult{}
	}

	poleStart := n - poleLen - flagLen
	poleEnd := n - flagLen - 1
	flagStart := n - flagLen

	if poleStart < 0 {
		return FlagResult{}
	}

	// Summarise the pole phase.
	poleLow := bars[poleStart].Low
	poleHigh := bars[poleStart].High
	for k := poleStart; k <= poleEnd; k++ {
		if bars[k].High > poleHigh {
			poleHigh = bars[k].High
		}
		if bars[k].Low < poleLow {
			poleLow = bars[k].Low
		}
	}
	poleClose := bars[poleEnd].Close

	// Summarise the flag / consolidation phase.
	flagHigh := bars[flagStart].High
	flagLow := bars[flagStart].Low
	for k := flagStart; k < n; k++ {
		if bars[k].High > flagHigh {
			flagHigh = bars[k].High
		}
		if bars[k].Low < flagLow {
			flagLow = bars[k].Low
		}
	}

	result := FlagResult{}

	// Bull flag: big up-pole, then limited pullback.
	if poleLow > 0 {
		upPct := (poleClose - poleLow) / poleLow * 100
		if upPct >= polePct {
			retracePct := 0.0
			if poleHigh > 0 {
				retracePct = (poleHigh - flagLow) / poleHigh * 100
			}
			if retracePct <= maxRetracementPct {
				result.BullFlag = true
				result.PolePct = upPct
			}
		}
	}

	// Bear flag: big down-pole, then limited bounce.
	if poleHigh > 0 {
		downPct := (poleHigh - poleClose) / poleHigh * 100
		if downPct >= polePct && !result.BullFlag {
			bouncePct := 0.0
			if poleLow > 0 {
				bouncePct = (flagHigh - poleLow) / poleLow * 100
			}
			if bouncePct <= maxRetracementPct {
				result.BearFlag = true
				result.PolePct = downPct
			}
		}
	}

	return result
}
