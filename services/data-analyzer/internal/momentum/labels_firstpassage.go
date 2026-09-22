package momentum

import "github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"

// Research round 1 labels (Phase 2 §2.2 hypotheses (a) and (b)).
//
// WHY THESE EXIST
//
// `hit_100` asks whether the peak close EVER touched +100% within 120
// sessions. Touch-anytime is structurally easiest for the most volatile
// names, and the post-upgrade report measured exactly that: `atr_pct` above
// its median gave +15.86pp, `log(dollar_volume)` -6.09pp, `pct_of_52w_high`
// -7.65pp. All three restate "volatile, small, beaten-down things move more",
// which predicts the MAGNITUDE of motion and says nothing about direction.
//
// A first-passage label cancels most of that, because the same volatility
// that makes +100% reachable makes the stop reachable too, and the label
// requires the GOOD bound to arrive first. It encodes path, not extremum.

// FirstPassage is the outcome of a race between an upside target and a
// downside stop, both evaluated on CLOSES.
//
// CLOSES, not intraday highs and lows, and this is a correctness constraint
// rather than a convenience. On a daily bar we know the high and the low but
// NOT their order within the session. A label that used them would have to
// assume which came first, and assuming the favourable order is a lookahead
// that would inflate every first-passage rate. Phase 1 §12 forbids exactly
// this kind of silent assumption.
type FirstPassage struct {
	// Hit is true when the target was reached on a close BEFORE any close
	// breached the stop.
	Hit bool

	// Stopped is true when the stop was breached first.
	Stopped bool

	// Neither is true when the window ended with no bound touched. Such rows
	// count as a MISS for the binary label, which is the conservative
	// direction: a position that went nowhere did not achieve the target.
	Neither bool

	// SessionsToResolve is how many sessions after t the race was decided,
	// 0 when it never was. Reported so "resolved on day 2" and "resolved on
	// day 118" are distinguishable — they are very different trades.
	SessionsToResolve int

	// StopLevel is the absolute price the stop sat at, carried so a label can
	// be audited without recomputing the ATR or the percentage.
	StopLevel float64
}

// FirstPassageAt races a +targetPct gain against a stop, from close[i].
//
// stopPct and stopATR are alternative ways to place the stop, and exactly one
// must be supplied:
//
//   - stopPct > 0: a fixed percentage below entry, e.g. 50 for -50%.
//   - stopATR > 0: a multiple of atr14 below entry, e.g. 2 for -2x ATR.
//
// BOTH variants are run in round 1 rather than one, because a FIXED percentage
// is itself volatility-dependent: -50% is a far tighter stop for a low-ATR
// name than a high-ATR one, so a fixed stop partially re-imports the very
// confound the label is meant to remove. The ATR-scaled stop has the opposite
// bias — it is wider exactly where volatility is high. Running both brackets
// the truth instead of picking a side.
//
// Returns nil when the window cannot be evaluated at all (bad index, no
// forward bars, non-positive reference), matching LabelAt's contract.
func FirstPassageAt(bars []compute.Bar, i, horizon int, targetPct, stopPct, stopATR, atr14 float64) *FirstPassage {
	if horizon <= 0 {
		horizon = DefaultHorizon
	}
	if i < 0 || i >= len(bars)-1 {
		return nil
	}
	ref := bars[i].Close
	if ref <= 0 {
		return nil
	}

	var stop float64
	switch {
	case stopPct > 0:
		stop = ref * (1 - stopPct/100)
	case stopATR > 0 && atr14 > 0:
		stop = ref - stopATR*atr14
	default:
		// No usable stop. Returning nil rather than falling back to "no stop"
		// is deliberate: a first-passage label without a stop is just
		// touch-anytime wearing its name, and silently degrading to it would
		// make the two hypotheses indistinguishable in the results table.
		return nil
	}
	// A stop at or below zero can never be breached, which would again reduce
	// the label to touch-anytime.
	if stop <= 0 {
		return nil
	}

	target := ref * (1 + targetPct/100)

	// Window is [i+1, i+horizon], clipped. Index i is excluded for the same
	// reason LabelAt excludes it: the features were computed on that bar.
	end := i + horizon
	if end > len(bars)-1 {
		end = len(bars) - 1
	}

	fp := &FirstPassage{StopLevel: stop}
	for j := i + 1; j <= end; j++ {
		c := bars[j].Close
		// Stop checked FIRST within the session. Both bounds can be crossed by
		// one close only if the stop is above the target, which cannot happen
		// for a positive target and a stop below entry — so the order is
		// immaterial here and is fixed anyway, so the label is deterministic
		// rather than dependent on evaluation order.
		if c <= stop {
			fp.Stopped, fp.SessionsToResolve = true, j-i
			return fp
		}
		if c >= target {
			fp.Hit, fp.SessionsToResolve = true, j-i
			return fp
		}
	}
	fp.Neither = true
	return fp
}

// ShortHorizonHit is hypothesis (b): did the close reach +targetPct at any
// point within the next `sessions` sessions?
//
// Same touch-anytime shape as hit_100, deliberately: (b) varies the HORIZON
// and the threshold, not the label mechanics, so its result is comparable
// with the existing labels rather than confounded with (a)'s path dependence.
//
// +20% within 10 sessions is the round-1 setting: short enough that a
// daily-bar signal is plausible, long enough to clear ordinary noise, and it
// produces far more positives than hit_100 so the folds are better powered.
func ShortHorizonHit(bars []compute.Bar, i, sessions int, targetPct float64) (hit bool, ok bool) {
	if i < 0 || i >= len(bars)-1 || sessions <= 0 {
		return false, false
	}
	ref := bars[i].Close
	if ref <= 0 {
		return false, false
	}
	end := i + sessions
	if end > len(bars)-1 {
		// Incomplete window. Reported as not-ok rather than as a miss: a row
		// with 3 of 10 forward bars has had almost no chance to reach the
		// target, and counting it as a failure would bias the rate downward
		// exactly as §6 warns for hit_100.
		return false, false
	}
	target := ref * (1 + targetPct/100)
	for j := i + 1; j <= end; j++ {
		if bars[j].Close >= target {
			return true, true
		}
	}
	return false, true
}
