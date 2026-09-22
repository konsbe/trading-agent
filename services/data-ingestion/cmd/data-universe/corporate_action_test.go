package main

import (
	"testing"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/store"
)

func f64(v float64) *float64 { return &v }

// THE DEFECT THIS GUARDS
//
// Adjusted prices are rewritten BACKWARDS on every split and every dividend.
// The daily refresh fetches only a short recent window, so a symbol with a
// corporate action after its backfill ends up with old-basis history joined to
// new-basis bars — an adjustment seam, self-inflicted, and invisible because
// each individual refresh looks correct on its own.
//
// hasCorporateAction is the trigger that prevents it. If it stops firing, the
// seam returns silently and every windowed feature spanning the boundary is
// computed across two price scales.
func TestHasCorporateAction_FiresOnASplitArrivingInTheDailyWindow(t *testing.T) {
	// An ordinary week, then a 1-for-11 reverse split — NEXR's real 2026-07-31
	// action, which is exactly the shape this has to catch.
	bars := []store.EquityBar{
		{Symbol: "NEXR", SplitFactor: f64(1.0), DivCash: f64(0)},
		{Symbol: "NEXR", SplitFactor: f64(1.0), DivCash: f64(0)},
		{Symbol: "NEXR", SplitFactor: f64(0.0909090909), DivCash: f64(0)},
	}
	if !hasCorporateAction(bars) {
		t.Fatal("a 1-for-11 reverse split in the refresh window did not trigger a full " +
			"re-fetch; the stored history stays on the pre-split adjustment basis while " +
			"these bars are on the post-split one")
	}
}

// Dividends rewrite the adjusted series too, and checking only splits was the
// obvious half-fix. Symbol A's 2016-09-30 divCash of 0.115 moved its cumulative
// adjustment factor from 0.9249 to 0.9272 — small, quarterly, and cumulative.
func TestHasCorporateAction_FiresOnADividendToo(t *testing.T) {
	bars := []store.EquityBar{
		{Symbol: "A", SplitFactor: f64(1.0), DivCash: f64(0)},
		{Symbol: "A", SplitFactor: f64(1.0), DivCash: f64(0.115)},
	}
	if !hasCorporateAction(bars) {
		t.Fatal("a cash dividend did not trigger a re-fetch; dividends rewrite the " +
			"adjusted series backwards exactly as splits do, and a split-only check " +
			"leaves every dividend-paying symbol accumulating seams")
	}
}

// The common case must NOT trigger: ~4,900 symbols refresh daily, and
// re-fetching all of their full histories every day would turn a 5,000-request
// job into a 5,000-symbol backfill.
func TestHasCorporateAction_QuietOnAnOrdinaryWeek(t *testing.T) {
	bars := []store.EquityBar{
		{Symbol: "AAPL", SplitFactor: f64(1.0), DivCash: f64(0)},
		{Symbol: "AAPL", SplitFactor: f64(1.0), DivCash: f64(0)},
		{Symbol: "AAPL", SplitFactor: f64(1.0), DivCash: f64(0)},
	}
	if hasCorporateAction(bars) {
		t.Error("an ordinary week triggered a full re-fetch; that would re-backfill the " +
			"whole universe daily")
	}
}

// A provider that reports neither field must not be read as "no action".
// Absence of evidence is recorded as NULL and surfaces in the seam audit; it
// must not silently satisfy the trigger's negative case by looking like 1.0/0.
func TestHasCorporateAction_NilFieldsAreNotEvidenceOfNoAction(t *testing.T) {
	bars := []store.EquityBar{{Symbol: "X", SplitFactor: nil, DivCash: nil}}
	if hasCorporateAction(bars) {
		t.Error("nil fields must not fire the trigger — they are unknown, not an action")
	}
	// The complementary guarantee lives in the seam audit: a symbol whose
	// corporate-action fields are NULL cannot be cleared by it either, so the
	// uncertainty stays visible rather than resolving to "fine".
}

// Floating point must not make a 1.0 split factor look like an action. Tiingo
// returns 1.0 on the overwhelming majority of bars, so a naive != 1 comparison
// would fire constantly.
func TestHasCorporateAction_ToleratesFloatingPointNoiseAroundOne(t *testing.T) {
	bars := []store.EquityBar{{Symbol: "X", SplitFactor: f64(1.0 + 1e-12), DivCash: f64(0)}}
	if hasCorporateAction(bars) {
		t.Error("floating-point noise around 1.0 fired the trigger; this would re-fetch " +
			"the universe daily")
	}
	// But a real 2:1 split must still fire, so the tolerance cannot be widened
	// carelessly.
	real2for1 := []store.EquityBar{{Symbol: "X", SplitFactor: f64(2.0), DivCash: f64(0)}}
	if !hasCorporateAction(real2for1) {
		t.Error("a 2:1 split did not fire")
	}
}
