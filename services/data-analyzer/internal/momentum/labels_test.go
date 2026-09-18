package momentum

import (
	"math"
	"testing"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

func bars(closes ...float64) []compute.Bar {
	out := make([]compute.Bar, len(closes))
	for i, c := range closes {
		out[i] = compute.Bar{Open: c, High: c, Low: c, Close: c, Volume: 1000}
	}
	return out
}

// ─── No lookahead: the property the whole exercise rests on ────────────────────

// §6: "Features for day t may use only bars up to and including t. Labels use
// only bars after t. Any leakage makes the whole exercise worthless, and it is
// easy to introduce accidentally — write a test that asserts it."
//
// This is that test. Mutating bar i itself must not move the label at all, and
// mutating i+1 must. A label that changes when its own bar changes is reading
// the present, which is how a backtest reports edge it does not have.
func TestLabelAt_DoesNotReadTheBarItLabels(t *testing.T) {
	base := bars(10, 11, 12, 13, 14)

	orig := LabelAt(base, 1, 3)
	if orig == nil {
		t.Fatal("nil label")
	}

	// Change bar 1's own close to something enormous. RefClose must change (it IS
	// bar 1's close) but the forward maximum must not, because it is drawn
	// entirely from bars 2..4.
	mutated := bars(10, 11, 12, 13, 14)
	mutated[1].Close = 1000
	mutated[1].High = 1000
	got := LabelAt(mutated, 1, 3)

	if got.FwdMaxClose != orig.FwdMaxClose {
		t.Errorf("FwdMaxClose changed from %.2f to %.2f when only bar 1 was mutated — "+
			"the label is reading its own bar", orig.FwdMaxClose, got.FwdMaxClose)
	}
	if got.DaysToPeak != orig.DaysToPeak {
		t.Errorf("DaysToPeak changed from %d to %d on a same-bar mutation", orig.DaysToPeak, got.DaysToPeak)
	}

	// And the converse: a forward bar MUST move the label, or the test above
	// would pass on a function that ignores everything.
	fwd := bars(10, 11, 12, 13, 14)
	fwd[2].Close = 500
	gotFwd := LabelAt(fwd, 1, 3)
	if gotFwd.FwdMaxClose == orig.FwdMaxClose {
		t.Error("mutating a FORWARD bar did not change the label; the label is not reading the future at all")
	}
}

// The window must start at i+1. Starting at i would let a label be satisfied by
// the very bar the features were computed on, which reads as a same-day signal
// that predicts itself.
func TestLabelAt_WindowStartsStrictlyAfterT(t *testing.T) {
	// Bar 1 is the highest in the series. If the window included it, FwdMaxGain
	// would be 0 (its own close) rather than negative.
	b := bars(10, 100, 50, 40)
	l := LabelAt(b, 1, 2)
	if l == nil {
		t.Fatal("nil label")
	}
	if l.FwdMaxClose != 50 {
		t.Errorf("FwdMaxClose = %.2f, want 50 (bar 2) — bar 1's own close of 100 must be excluded", l.FwdMaxClose)
	}
	if l.FwdMaxGainPct >= 0 {
		t.Errorf("FwdMaxGainPct = %.2f, want negative: the stock only fell after t", l.FwdMaxGainPct)
	}
}

// ─── Hand-computed arithmetic ──────────────────────────────────────────────────

func TestLabelAt_HandComputedGainAndPeak(t *testing.T) {
	// ref = 10 at index 0; forward closes 12, 25, 18, 20 over a horizon of 4.
	b := bars(10, 12, 25, 18, 20)
	l := LabelAt(b, 0, 4)
	if l == nil {
		t.Fatal("nil label")
	}
	if l.RefClose != 10 {
		t.Errorf("RefClose = %.2f, want 10", l.RefClose)
	}
	if l.FwdMaxClose != 25 {
		t.Errorf("FwdMaxClose = %.2f, want 25", l.FwdMaxClose)
	}
	// 25/10 - 1 = +150%
	if math.Abs(l.FwdMaxGainPct-150) > 1e-9 {
		t.Errorf("FwdMaxGainPct = %.4f, want 150", l.FwdMaxGainPct)
	}
	if l.DaysToPeak != 2 {
		t.Errorf("DaysToPeak = %d, want 2", l.DaysToPeak)
	}
	if !l.Complete {
		t.Error("4 forward bars with horizon 4 should be complete")
	}
	if !l.Hits[100] {
		t.Error("+150% must register hit_100")
	}
	if l.Hits[200] {
		t.Error("+150% must NOT register hit_200")
	}
}

// Drawdown is measured inside the window and against the entry reference too, so
// a position that is underwater from the start is not reported as drawdown-free.
func TestLabelAt_DrawdownMeasuredFromPeakAndFromEntry(t *testing.T) {
	// Rises to 20 (peak), falls to 10 -> 50% drawdown from peak.
	l := LabelAt(bars(10, 20, 10), 0, 2)
	if math.Abs(l.FwdMaxDrawdownPct-50) > 1e-9 {
		t.Errorf("drawdown = %.4f, want 50 (20 -> 10)", l.FwdMaxDrawdownPct)
	}

	// Falls immediately below entry with no new peak: still a real drawdown.
	l = LabelAt(bars(10, 6), 0, 1)
	if math.Abs(l.FwdMaxDrawdownPct-40) > 1e-9 {
		t.Errorf("drawdown = %.4f, want 40 (entry 10 -> 6); a position underwater from the "+
			"start must not report zero drawdown", l.FwdMaxDrawdownPct)
	}
}

// ─── Incomplete labels ────────────────────────────────────────────────────────

// §6 requires rows inside the last H sessions to be marked incomplete and
// excluded from evaluation. Counting them as misses would bias the base rate
// downward: a row with 5 forward bars has had almost no chance to reach +100%,
// so it is unknown rather than negative.
func TestLabelAt_MarksShortWindowsIncomplete(t *testing.T) {
	b := bars(10, 11, 12, 13, 14, 15)

	full := LabelAt(b, 0, 5)
	if !full.Complete {
		t.Error("5 forward bars with horizon 5 should be complete")
	}
	if full.BarsAhead != 5 {
		t.Errorf("BarsAhead = %d, want 5", full.BarsAhead)
	}

	short := LabelAt(b, 3, 5) // only 2 bars follow
	if short.Complete {
		t.Error("2 forward bars with horizon 5 must be incomplete")
	}
	if short.BarsAhead != 2 {
		t.Errorf("BarsAhead = %d, want 2", short.BarsAhead)
	}
	// It still computes a gain — it is usable for inspection, just not for
	// evaluation, which is why Complete is a flag rather than a nil return.
	if short.FwdMaxGainPct == 0 {
		t.Error("an incomplete label should still report the gain it did observe")
	}
}

func TestLabelAt_NoForwardBarsYieldsNoLabel(t *testing.T) {
	b := bars(10, 11)
	if l := LabelAt(b, 1, 5); l != nil {
		t.Error("the last bar has no forward window and must yield nil, not a zero label")
	}
	if l := LabelAt(b, 5, 5); l != nil {
		t.Error("an out-of-range index must yield nil")
	}
	if l := LabelAt(b, -1, 5); l != nil {
		t.Error("a negative index must yield nil")
	}
}

func TestLabelAt_RejectsNonPositiveReference(t *testing.T) {
	b := bars(0, 10, 20)
	if l := LabelAt(b, 0, 2); l != nil {
		t.Error("a zero reference close cannot produce a percentage and must yield nil, not +Inf")
	}
}

// ─── Thresholds ───────────────────────────────────────────────────────────────

func TestLabelAt_AllHitThresholds(t *testing.T) {
	for _, c := range []struct {
		mult float64
		want []float64
	}{
		{1.5, nil},
		{2.0, []float64{100}},
		{3.0, []float64{100, 200}},
		{4.0, []float64{100, 200, 300}},
		{6.0, []float64{100, 200, 300, 500}},
		{11.0, []float64{100, 200, 300, 500, 1000}},
	} {
		l := LabelAt(bars(10, 10*c.mult), 0, 1)
		for _, th := range HitThresholds {
			want := false
			for _, w := range c.want {
				if w == th {
					want = true
				}
			}
			if l.Hits[th] != want {
				t.Errorf("%.1fx (gain %.0f%%): hit_%.0f = %v, want %v",
					c.mult, l.FwdMaxGainPct, th, l.Hits[th], want)
			}
		}
	}
}

// The threshold is inclusive: exactly +100% is a hit, since §6 writes ">=".
func TestLabelAt_ThresholdIsInclusive(t *testing.T) {
	l := LabelAt(bars(10, 20), 0, 1)
	if math.Abs(l.FwdMaxGainPct-100) > 1e-9 {
		t.Fatalf("gain = %.6f, want exactly 100", l.FwdMaxGainPct)
	}
	if !l.Hits[100] {
		t.Error("exactly +100% must count as hit_100")
	}
}

func TestDefaultHorizon_MatchesSpec(t *testing.T) {
	if DefaultHorizon != 120 {
		t.Errorf("DefaultHorizon = %d, want 120 (§6's H)", DefaultHorizon)
	}
}
