// Package momentum computes the momentum scanner's per-symbol daily features.
//
// Spec: docs/MOMENTUM_SCANNER_PHASE1.md §3. Build-order step 4.
//
// Everything here is a pure function over a slice of daily bars. There is no
// database, no network and no clock: that is what makes the formulas testable
// against hand-computed fixtures, and §8.2 is explicit that these formulas are
// the whole product — "a silent off-by-one in a window boundary is invisible in
// output but fatal to the results".
//
// Two rules the whole package is built around:
//
//   - NO LOOKAHEAD. Features for day t may use only bars up to and including t.
//     ComputeAt exists so this is *testable* rather than merely asserted: it
//     takes a full series and an index, and TestNoLookahead proves its output
//     is byte-identical to Compute over a slice truncated at that index. A
//     forward index anywhere would break that equality.
//   - A MISSING INPUT IS NULL, NEVER ZERO. Every field is a pointer. §3.4 and
//     §12 are explicit that a null RVOL must fail the gate rather than become
//     0 or 1, and the only way to keep that true downstream is to never
//     manufacture a value here.
//
// Indicators are reused from internal/compute (§12: do not reimplement).
package momentum

import (
	"fmt"
	"math"
	"strconv"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

// BreakoutState enumerates §3.6's four values. Strings match the spec exactly
// because they are persisted and rendered verbatim.
type BreakoutState string

const (
	BreakoutFromConsolidation BreakoutState = "breakout_from_consolidation"
	Breakout                  BreakoutState = "breakout"
	BreakoutApproaching       BreakoutState = "approaching"
	BreakoutNone              BreakoutState = "none"
)

// Config holds every threshold and window from §3. All are env-var defaults in
// production (§3.2: "Every threshold below is an env-var default, not a
// constant"), so nothing here is a magic number at the call site.
type Config struct {
	ATRPeriod int // §3.3 atr_14
	RSIPeriod int // §3.10 rsi_14

	RVolLookback int // §3.4 avg_vol_20 — bars EXCLUDING today

	VolAccelRecent int // §3.5 recent window, INCLUDING today
	VolAccelPrior  int // §3.5 prior window, immediately before the recent one

	BreakoutLookback      int     // §3.6 resistance_20 / range_20 — EXCLUDING today
	ConsolidationMaxRange float64 // §3.6 was_consolidating threshold (0.25)
	ApproachingFraction   float64 // §3.6 approaching band (0.98)

	High52wLookback int // §3.7 — EXCLUDING today (251)

	VWAPLookback int // §3.8 vwap_20 — INCLUDING today (20)

	Change5dLookback int // §3.12 input change_pct_5d
}

// DefaultConfig returns §3's documented values.
func DefaultConfig() Config {
	return Config{
		ATRPeriod: 14,
		RSIPeriod: 14,

		RVolLookback: 20,

		VolAccelRecent: 3,
		VolAccelPrior:  5,

		BreakoutLookback:      20,
		ConsolidationMaxRange: 0.25,
		ApproachingFraction:   0.98,

		// §3.7 reads high[t-251], so the window excluding today is 251 bars.
		// A complete window therefore needs 252 bars including today, which is
		// why §3.1's eligibility minimum is 252 rather than 250.
		High52wLookback: 251,

		VWAPLookback: 20,

		Change5dLookback: 5,
	}
}

// Features is one symbol-day's §3 feature row.
//
// Every field is a pointer so "not computable" stays distinguishable from
// "computed as zero". A change_pct of exactly 0.0 is a flat day; a nil
// change_pct means there was no prior bar. Collapsing those is how a gate ends
// up passing a symbol it should have excluded.
//
// Fields requiring data outside the bar series — float_shares_est, market_cap,
// catalyst_tier, sector_strength_pct — are deliberately absent. They are joined
// on by the caller, so this type cannot accidentally fabricate them.
type Features struct {
	// §3.3 price and change
	Close        *float64
	Volume       *float64
	PriorClose   *float64
	ChangePct    *float64
	GapPct       *float64
	DollarVolume *float64
	ATR14        *float64
	ATRPct       *float64

	// §3.4 relative volume. RVol20 is nil when the baseline is unavailable or
	// zero — never 0, never 1 (§3.4, §12).
	AvgVol20 *float64
	RVol20   *float64

	// §3.5 volume acceleration
	VolAccel *float64

	// §3.6 breakout geometry, judged on the close and never on an intrabar high
	Resistance20     *float64
	Range20          *float64
	WasConsolidating *bool
	BreakoutState    *BreakoutState

	// §3.7 52-week high proximity. The window EXCLUDES today, so PctOf52wHigh
	// can exceed 1.0 and > 1.0 IS a new 52-week high. There is deliberately no
	// separate new-high boolean; it would be redundant with this ratio.
	High52w      *float64
	PctOf52wHigh *float64

	// §3.8 rolling VWAP (Phase 1 stand-in for intraday session VWAP)
	VWAP20      *float64
	AboveVWAP   *bool
	VWAPDistPct *float64

	// §3.10 RSI — penalty input only, never a positive score
	RSI14 *float64

	// §3.12 input
	ChangePct5d *float64

	// BarsAvailable is the number of bars at or before t. Recorded so a caller
	// can tell "feature is nil because the window was short" from "feature is
	// nil because the data was bad".
	BarsAvailable int
}

// Compute returns features for the LAST bar of the series.
//
// bars must be oldest-first and contain only completed daily bars for one
// symbol. Insufficient history yields nil fields rather than an error: a
// newly-listed symbol legitimately has no 52-week high, and that is a gate
// decision (§3.2), not a failure.
func Compute(bars []compute.Bar, cfg Config) Features {
	if len(bars) == 0 {
		return Features{}
	}
	return ComputeAt(bars, len(bars)-1, cfg)
}

// ComputeAt returns features for bars[i], using only bars at or before i.
//
// This is the primitive, and Compute is the convenience wrapper, rather than the
// other way around, for two reasons:
//
//   - §6's historical backfill computes one feature row per symbol per day over
//     a single fetched series. Re-slicing for every day would be O(n²) copying.
//   - It makes no-lookahead a *property that can be tested*. Because
//     ComputeAt(bars, i) must equal Compute(bars[:i+1]), any forward index
//     introduced here shows up as a failing equality rather than as a plausible
//     number. See TestNoLookahead.
func ComputeAt(bars []compute.Bar, i int, cfg Config) Features {
	if i < 0 || i >= len(bars) {
		return Features{}
	}
	// Everything below reads only w, the window ending at and including i.
	// Nothing in this function may index `bars` past i.
	w := bars[:i+1]
	n := len(w)

	f := Features{BarsAvailable: n}
	t := w[n-1]

	f.Close = ptr(t.Close)
	f.Volume = ptr(t.Volume)
	f.DollarVolume = ptr(t.Close * t.Volume)

	// ── §3.3 change and gap, both relative to the prior close ──────────────
	if n >= 2 {
		prior := w[n-2].Close
		f.PriorClose = ptr(prior)
		if prior > 0 {
			f.ChangePct = ptr((t.Close/prior - 1) * 100)
			f.GapPct = ptr((t.Open/prior - 1) * 100)
		}
	}

	// ── §3.3 ATR, reusing compute.ATRWilder ────────────────────────────────
	if atr, ok := compute.ATRWilder(compute.Highs(w), compute.Lows(w), compute.Closes(w), cfg.ATRPeriod); ok {
		f.ATR14 = ptr(atr)
		if t.Close > 0 {
			f.ATRPct = ptr(atr / t.Close * 100)
		}
	}

	// ── §3.4 relative volume. The baseline EXCLUDES today: including it
	// deflates RVOL exactly when it matters most. ───────────────────────────
	if lb := cfg.RVolLookback; lb > 0 && n >= lb+1 {
		base := w[n-1-lb : n-1] // lb bars, ending at t-1
		avg := mean(volumes(base))
		f.AvgVol20 = ptr(avg)
		// A zero baseline leaves RVOL nil rather than dividing by zero or
		// substituting a value. §3.4 is explicit: the symbol fails the gate.
		if avg > 0 {
			f.RVol20 = ptr(t.Volume / avg)
		}
	}

	// ── §3.5 volume acceleration: is volume building, or already fading ────
	if r, p := cfg.VolAccelRecent, cfg.VolAccelPrior; r > 0 && p > 0 && n >= r+p {
		recent := w[n-r:]       // r bars including today
		prior := w[n-r-p : n-r] // the p bars immediately before those
		pm := mean(volumes(prior))
		if pm > 0 {
			f.VolAccel = ptr(mean(volumes(recent)) / pm)
		}
	}

	// ── §3.6 breakout geometry ─────────────────────────────────────────────
	if lb := cfg.BreakoutLookback; lb > 0 && n >= lb+1 {
		prior := w[n-1-lb : n-1] // lb bars, excluding today
		hh, ll, _, ok := compute.DonchianLast(compute.Highs(prior), compute.Lows(prior), lb)
		if ok {
			f.Resistance20 = ptr(hh)
			if t.Close > 0 {
				rng := (hh - ll) / t.Close
				f.Range20 = ptr(rng)
				consolidating := rng < cfg.ConsolidationMaxRange
				f.WasConsolidating = ptr(consolidating)
				f.BreakoutState = ptr(classifyBreakout(t.Close, hh, consolidating, cfg.ApproachingFraction))
			}
		}
	}

	// ── §3.7 52-week high proximity, window EXCLUDING today ────────────────
	if lb := cfg.High52wLookback; lb > 0 && n >= lb+1 {
		prior := w[n-1-lb : n-1]
		hh := maxOf(compute.Highs(prior))
		f.High52w = ptr(hh)
		if hh > 0 {
			// Ratio, not percent. > 1.0 is a new 52-week high.
			f.PctOf52wHigh = ptr(t.Close / hh)
		}
	}

	// ── §3.8 rolling VWAP, window INCLUDING today ──────────────────────────
	if lb := cfg.VWAPLookback; lb > 0 && n >= lb {
		if v, ok := compute.VWAPRolling(w, lb, true); ok && v > 0 {
			f.VWAP20 = ptr(v)
			f.AboveVWAP = ptr(t.Close > v)
			f.VWAPDistPct = ptr((t.Close/v - 1) * 100)
		}
	}

	// ── §3.10 RSI, a penalty input only ────────────────────────────────────
	if r, ok := compute.RSI(compute.Closes(w), cfg.RSIPeriod); ok {
		f.RSI14 = ptr(r)
	}

	// ── §3.12 input: 5-day change, feeding sector_strength_pct ─────────────
	if lb := cfg.Change5dLookback; lb > 0 && n >= lb+1 {
		past := w[n-1-lb].Close
		if past > 0 {
			f.ChangePct5d = ptr((t.Close/past - 1) * 100)
		}
	}

	return f
}

// classifyBreakout implements §3.6's four-way decision.
//
// The breakout is judged on the CLOSE, never on an intrabar high. A wick above
// resistance that closes back below is not a breakout, and the spec calls this
// "the main thing separating a real breakout from a failed one".
func classifyBreakout(close, resistance float64, wasConsolidating bool, approachFrac float64) BreakoutState {
	switch {
	case close > resistance && wasConsolidating:
		return BreakoutFromConsolidation
	case close > resistance:
		return Breakout
	case close >= resistance*approachFrac:
		// Reached only when close <= resistance, since both breakout cases are
		// handled above.
		return BreakoutApproaching
	default:
		return BreakoutNone
	}
}

// ── small helpers, kept unexported and allocation-light ────────────────────

func ptr[T any](v T) *T { return &v }

func volumes(bars []compute.Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Volume
	}
	return out
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func maxOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

// Diff returns the names of fields that differ between two rows, formatted for
// a test failure message.
//
// Equal answers "is there a leak"; Diff answers "where". Without it a
// no-lookahead failure prints a struct full of pointer addresses, and whoever
// hits it has to re-instrument the code to find out which field leaked — at
// which point the test has cost more than it saved.
func (f Features) Diff(o Features) []string {
	var out []string
	add := func(name string, a, b any) {
		out = append(out, fmt.Sprintf("%s: %s != %s", name, showAny(a), showAny(b)))
	}
	if f.BarsAvailable != o.BarsAvailable {
		add("bars_available", f.BarsAvailable, o.BarsAvailable)
	}
	for _, c := range []struct {
		name string
		a, b *float64
	}{
		{"close", f.Close, o.Close},
		{"volume", f.Volume, o.Volume},
		{"prior_close", f.PriorClose, o.PriorClose},
		{"change_pct", f.ChangePct, o.ChangePct},
		{"gap_pct", f.GapPct, o.GapPct},
		{"dollar_volume", f.DollarVolume, o.DollarVolume},
		{"atr_14", f.ATR14, o.ATR14},
		{"atr_pct", f.ATRPct, o.ATRPct},
		{"avg_vol_20", f.AvgVol20, o.AvgVol20},
		{"rvol_20", f.RVol20, o.RVol20},
		{"vol_accel", f.VolAccel, o.VolAccel},
		{"resistance_20", f.Resistance20, o.Resistance20},
		{"range_20", f.Range20, o.Range20},
		{"high_52w", f.High52w, o.High52w},
		{"pct_of_52w_high", f.PctOf52wHigh, o.PctOf52wHigh},
		{"vwap_20", f.VWAP20, o.VWAP20},
		{"vwap_dist_pct", f.VWAPDistPct, o.VWAPDistPct},
		{"rsi_14", f.RSI14, o.RSI14},
		{"change_pct_5d", f.ChangePct5d, o.ChangePct5d},
	} {
		if !eqF(c.a, c.b) {
			add(c.name, c.a, c.b)
		}
	}
	if !eqB(f.WasConsolidating, o.WasConsolidating) {
		add("was_consolidating", f.WasConsolidating, o.WasConsolidating)
	}
	if !eqS(f.BreakoutState, o.BreakoutState) {
		add("breakout_state", f.BreakoutState, o.BreakoutState)
	}
	return out
}

func showAny(v any) string {
	switch x := v.(type) {
	case *float64:
		if x == nil {
			return "nil"
		}
		return strconv.FormatFloat(*x, 'g', -1, 64)
	case *bool:
		if x == nil {
			return "nil"
		}
		return strconv.FormatBool(*x)
	case *BreakoutState:
		if x == nil {
			return "nil"
		}
		return string(*x)
	default:
		return fmt.Sprint(v)
	}
}

// Equal reports whether two feature rows are identical field by field.
//
// Exists for the no-lookahead test, which needs to compare whole rows rather
// than spot-check fields — a leak in any single field must fail the test, and
// hand-listing fields in the test would silently miss newly-added ones.
func (f Features) Equal(o Features) bool {
	return f.BarsAvailable == o.BarsAvailable &&
		eqF(f.Close, o.Close) &&
		eqF(f.Volume, o.Volume) &&
		eqF(f.PriorClose, o.PriorClose) &&
		eqF(f.ChangePct, o.ChangePct) &&
		eqF(f.GapPct, o.GapPct) &&
		eqF(f.DollarVolume, o.DollarVolume) &&
		eqF(f.ATR14, o.ATR14) &&
		eqF(f.ATRPct, o.ATRPct) &&
		eqF(f.AvgVol20, o.AvgVol20) &&
		eqF(f.RVol20, o.RVol20) &&
		eqF(f.VolAccel, o.VolAccel) &&
		eqF(f.Resistance20, o.Resistance20) &&
		eqF(f.Range20, o.Range20) &&
		eqB(f.WasConsolidating, o.WasConsolidating) &&
		eqS(f.BreakoutState, o.BreakoutState) &&
		eqF(f.High52w, o.High52w) &&
		eqF(f.PctOf52wHigh, o.PctOf52wHigh) &&
		eqF(f.VWAP20, o.VWAP20) &&
		eqB(f.AboveVWAP, o.AboveVWAP) &&
		eqF(f.VWAPDistPct, o.VWAPDistPct) &&
		eqF(f.RSI14, o.RSI14) &&
		eqF(f.ChangePct5d, o.ChangePct5d)
}

func eqF(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	// Bit-identical, not approximate: this compares two runs of the same
	// arithmetic on the same inputs, so any difference is a real divergence.
	return *a == *b || (math.IsNaN(*a) && math.IsNaN(*b))
}

func eqB(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func eqS(a, b *BreakoutState) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
