package momentum

import "github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"

// §6 forward labels — the real Phase 1 deliverable.
//
// Labels answer "what happened next", features answer "what was true then", and
// the only thing that makes the exercise worth anything is that the two never
// touch. Features for day t use bars up to and including t; labels use ONLY bars
// strictly after t. LabelAt enforces that by construction: it starts reading at
// i+1 and never looks at index i except to take the reference close.

// DefaultHorizon is §6's H: 120 trading days.
const DefaultHorizon = 120

// Thresholds are §6's hit levels, in percent.
var HitThresholds = []float64{100, 200, 300, 500, 1000}

// Labels is one row's forward outcome.
type Labels struct {
	// RefClose is close[t], the denominator. Carried so a label can be audited
	// without re-reading the bar series.
	RefClose float64

	FwdMaxClose   float64
	FwdMaxGainPct float64

	// DaysToPeak is the offset (in trading days after t) of FwdMaxClose.
	DaysToPeak int

	// FwdMaxDrawdownPct is the largest peak-to-trough decline WITHIN the forward
	// window, as a positive percentage.
	//
	// Reported because a +100% label that required sitting through a -70%
	// drawdown is not the same trade as one that went up in a line, and a scanner
	// evaluated on gain alone would rate them identically. Phase 2 needs this to
	// reason about whether a signal is tradeable at all.
	FwdMaxDrawdownPct float64

	// BarsAhead is how many bars the window actually contained.
	BarsAhead int

	// Complete is false when fewer than Horizon bars follow t.
	//
	// §6: rows inside the last H sessions have incomplete labels and must be
	// EXCLUDED from evaluation. Including them would bias the base rate downward,
	// because a row with 5 forward bars has had almost no chance to reach +100%
	// and would be counted as a miss rather than as unknown.
	Complete bool

	// Hits maps each threshold to whether FwdMaxGainPct reached it.
	Hits map[float64]bool
}

// LabelAt computes §6's forward labels for index i.
//
// bars must be chronological. Returns nil when i is the last bar or out of
// range: a row with no forward bars has no label, which is different from a
// label of zero.
func LabelAt(bars []compute.Bar, i int, horizon int) *Labels {
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

	// The forward window is [i+1, i+horizon], clipped to the series. Index i is
	// deliberately excluded from the scan: including it would let a label be
	// satisfied by the very bar the features were computed on.
	end := i + horizon
	if end > len(bars)-1 {
		end = len(bars) - 1
	}

	l := &Labels{
		RefClose:  ref,
		BarsAhead: end - i,
		Complete:  end-i >= horizon,
		Hits:      make(map[float64]bool, len(HitThresholds)),
	}

	maxClose, maxIdx := 0.0, i+1
	// Running peak and worst decline from it, both confined to the window.
	peak := 0.0
	worstDD := 0.0

	for j := i + 1; j <= end; j++ {
		c := bars[j].Close
		if c > maxClose {
			maxClose, maxIdx = c, j
		}
		if c > peak {
			peak = c
		}
		// Drawdown measured against the running peak, and against the entry
		// reference on the way in: a position opened at t is underwater
		// immediately if price falls below ref before any new peak forms.
		base := peak
		if base < ref {
			base = ref
		}
		if base > 0 {
			if dd := (base - c) / base * 100; dd > worstDD {
				worstDD = dd
			}
		}
	}

	l.FwdMaxClose = maxClose
	l.FwdMaxGainPct = (maxClose/ref - 1) * 100
	l.DaysToPeak = maxIdx - i
	l.FwdMaxDrawdownPct = worstDD
	for _, th := range HitThresholds {
		l.Hits[th] = l.FwdMaxGainPct >= th
	}
	return l
}
