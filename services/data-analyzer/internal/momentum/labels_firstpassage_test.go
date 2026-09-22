package momentum

import (
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

func fpBars(closes ...float64) []compute.Bar {
	out := make([]compute.Bar, len(closes))
	base := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		out[i] = compute.Bar{TS: base.AddDate(0, 0, i), Open: c, High: c, Low: c, Close: c, Volume: 1000}
	}
	return out
}

// ── the no-lookahead guarantee, extended to the new labels ──────────────────

// The label must not read the bar it labels. Mutating bar i must not change a
// label computed at i — if it does, the feature row and its own outcome share
// information.
func TestFirstPassageAt_DoesNotReadTheBarItLabels(t *testing.T) {
	bars := fpBars(10, 11, 12, 25, 9)
	before := FirstPassageAt(bars, 0, 4, 100, 50, 0, 0)

	// Rewrite bar 0's OHLC wildly, leaving the forward path alone.
	bars[0].High, bars[0].Low, bars[0].Open = 999, 0.01, 500
	after := FirstPassageAt(bars, 0, 4, 100, 50, 0, 0)

	if before.Hit != after.Hit || before.SessionsToResolve != after.SessionsToResolve {
		t.Fatalf("label changed when bar i's own OHLC changed (%+v -> %+v); the label is reading the bar it labels", before, after)
	}
}

func TestFirstPassageAt_WindowStartsStrictlyAfterT(t *testing.T) {
	// close[0] is ALREADY at the target relative to itself only if the window
	// included it. It must not.
	bars := fpBars(10, 10.1, 10.2)
	fp := FirstPassageAt(bars, 0, 2, 0.0001, 50, 0, 0)
	if fp == nil {
		t.Fatal("nil label")
	}
	if fp.SessionsToResolve == 0 && fp.Hit {
		t.Error("resolved on session 0 — the window must start at i+1")
	}
}

// Closes only, never intraday extremes: on a daily bar the ORDER of the high
// and the low within the session is unknown, so using them would require
// assuming which came first. Assuming the favourable order inflates every
// first-passage rate.
func TestFirstPassageAt_IgnoresIntradayHighsAndLows(t *testing.T) {
	bars := fpBars(10, 10.5, 10.5)
	// Bar 1's LOW pierces a -50% stop and its HIGH pierces a +100% target,
	// but its CLOSE does neither.
	bars[1].Low = 1.0
	bars[1].High = 100.0

	fp := FirstPassageAt(bars, 0, 2, 100, 50, 0, 0)
	if fp == nil {
		t.Fatal("nil label")
	}
	if fp.Hit || fp.Stopped {
		t.Errorf("resolved on intraday extremes (%+v); daily bars do not say whether the high or the low came first, "+
			"so only closes may decide the race", fp)
	}
	if !fp.Neither {
		t.Error("expected Neither")
	}
}

// ── the race itself ─────────────────────────────────────────────────────────

func TestFirstPassageAt_StopBeforeTargetIsNotAHit(t *testing.T) {
	// Falls to -60% on session 1, then rallies past +100% on session 3.
	// Touch-anytime would call this a WIN; first passage must not.
	bars := fpBars(10, 4, 8, 30)

	l := LabelAt(bars, 0, 3)
	if l == nil || !l.Hits[100] {
		t.Fatal("precondition: touch-anytime should record a +100% hit here")
	}

	fp := FirstPassageAt(bars, 0, 3, 100, 50, 0, 0)
	if fp.Hit {
		t.Error("first passage counted a hit although the stop was breached two sessions earlier — " +
			"this is exactly the trade the label exists to exclude")
	}
	if !fp.Stopped || fp.SessionsToResolve != 1 {
		t.Errorf("expected Stopped on session 1, got %+v", fp)
	}
}

func TestFirstPassageAt_TargetBeforeStopIsAHit(t *testing.T) {
	bars := fpBars(10, 12, 21, 3)
	fp := FirstPassageAt(bars, 0, 3, 100, 50, 0, 0)
	if !fp.Hit || fp.SessionsToResolve != 2 {
		t.Errorf("expected Hit on session 2, got %+v", fp)
	}
}

func TestFirstPassageAt_NeitherBoundIsAMissNotAnError(t *testing.T) {
	bars := fpBars(10, 10.5, 11, 10.8)
	fp := FirstPassageAt(bars, 0, 3, 100, 50, 0, 0)
	if fp.Hit || fp.Stopped || !fp.Neither {
		t.Errorf("a window that touched neither bound must be Neither (a miss), got %+v", fp)
	}
}

// The ATR variant places the stop differently, and must actually differ from
// the percentage variant — otherwise running both brackets nothing.
func TestFirstPassageAt_ATRStopDiffersFromPercentStop(t *testing.T) {
	// -2 x ATR(0.5) = -1.0 from 10 -> stop 9.0, which bar 1 breaches.
	// -50% -> stop 5.0, which it does not.
	bars := fpBars(10, 8.5, 25)

	pct := FirstPassageAt(bars, 0, 2, 100, 50, 0, 0)
	atr := FirstPassageAt(bars, 0, 2, 100, 0, 2, 0.5)

	if !pct.Hit {
		t.Errorf("percent stop at 5.0 should not have been breached by 8.5: %+v", pct)
	}
	if !atr.Stopped {
		t.Errorf("ATR stop at 9.0 should have been breached by 8.5: %+v", atr)
	}
	if atr.StopLevel != 9.0 {
		t.Errorf("StopLevel = %v, want 9.0", atr.StopLevel)
	}
}

// A label with no usable stop is touch-anytime wearing another name.
// Degrading silently would make hypotheses (a) and hit_100 indistinguishable
// in the results table.
func TestFirstPassageAt_RefusesWhenNoStopIsUsable(t *testing.T) {
	bars := fpBars(10, 25)
	if fp := FirstPassageAt(bars, 0, 1, 100, 0, 0, 0); fp != nil {
		t.Error("no stop supplied must return nil, not a stopless label")
	}
	if fp := FirstPassageAt(bars, 0, 1, 100, 0, 2, 0); fp != nil {
		t.Error("ATR stop requested with atr14=0 must return nil")
	}
	if fp := FirstPassageAt(bars, 0, 1, 100, 150, 0, 0); fp != nil {
		t.Error("a stop at or below zero can never be breached and must return nil")
	}
}

// ── short-horizon target (hypothesis b) ─────────────────────────────────────

func TestShortHorizonHit_IncompleteWindowIsNotAMiss(t *testing.T) {
	bars := fpBars(10, 10.5, 11) // only 2 forward bars
	hit, ok := ShortHorizonHit(bars, 0, 10, 20)
	if ok {
		t.Error("a window with fewer forward bars than the horizon must report ok=false; " +
			"counting it as a miss biases the rate downward exactly as §6 warns")
	}
	if hit {
		t.Error("hit must be false when not evaluable")
	}
}

func TestShortHorizonHit_FindsTheTargetInsideTheWindow(t *testing.T) {
	bars := fpBars(10, 10.5, 11, 12.1, 11, 11, 11, 11, 11, 11, 11)
	hit, ok := ShortHorizonHit(bars, 0, 10, 20)
	if !ok || !hit {
		t.Errorf("+21%% on session 3 should hit a +20%% target within 10 sessions (hit=%v ok=%v)", hit, ok)
	}
}

func TestShortHorizonHit_DoesNotReadTheBarItLabels(t *testing.T) {
	bars := fpBars(10, 10.5, 11, 12.1, 11, 11, 11, 11, 11, 11, 11)
	h1, _ := ShortHorizonHit(bars, 0, 10, 20)
	bars[0].High, bars[0].Low = 999, 0.01
	h2, _ := ShortHorizonHit(bars, 0, 10, 20)
	if h1 != h2 {
		t.Error("label changed when bar i's own OHLC changed")
	}
}

func TestShortHorizonHit_WindowStartsStrictlyAfterT(t *testing.T) {
	// A flat series: nothing after t reaches the target, so any hit would
	// have to have come from bar i itself.
	bars := fpBars(10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10)
	if hit, ok := ShortHorizonHit(bars, 0, 10, 0); ok && !hit {
		t.Log("0% target on a flat series is reached at i+1, which is correct")
	}
	hit, ok := ShortHorizonHit(bars, 0, 10, 20)
	if !ok || hit {
		t.Errorf("flat series must not hit +20%% (hit=%v ok=%v)", hit, ok)
	}
}
