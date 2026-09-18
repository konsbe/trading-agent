package barquality

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

// ─── The real observed failure ─────────────────────────────────────────────────

// abtsTwelveData is a VERBATIM capture of what Twelve Data returned for ABTS
// with adjust=all, 2025-02-20 to 2025-03-14, taken from the database before
// those rows were deleted.
//
// This is the defect that motivated the package: the provider alternates, bar by
// bar, between unadjusted (~0.42) and split-adjusted (~6.24) values for a
// 1-for-15 reverse split, inside a single response. Feb 28 and Mar 3 are
// adjusted, Feb 20-27 and Mar 4-7 are not, and Mar 10 onward are.
//
// It is kept as a fixture rather than reduced to a synthetic case because the
// synthetic version is the one I would have written before seeing this, and it
// would have been too clean: note that the volume rescaling is NOT an exact 15x
// (6600 -> 778), so a detector requiring exact inverse proportionality would
// have missed it.
var abtsTwelveData = []Bar{
	{TS: d("2025-02-20"), Open: 0.45199999, High: 0.46000001, Low: 0.41999999, Close: 0.41999999, Volume: 5900},
	{TS: d("2025-02-21"), Open: 0.44, High: 0.449, Low: 0.41100001, Close: 0.41100001, Volume: 275100},
	{TS: d("2025-02-24"), Open: 0.44, High: 0.44400001, Low: 0.41499999, Close: 0.44400001, Volume: 11400},
	{TS: d("2025-02-25"), Open: 0.41600001, High: 0.449, Low: 0.41600001, Close: 0.42199999, Volume: 19500},
	{TS: d("2025-02-26"), Open: 0.41999999, High: 0.44, Low: 0.41999999, Close: 0.41999999, Volume: 21000},
	{TS: d("2025-02-27"), Open: 0.435, High: 0.43599999, Low: 0.41999999, Close: 0.421, Volume: 6600},
	{TS: d("2025-02-28"), Open: 6.225, High: 6.74535, Low: 6.225, Close: 6.24, Volume: 778}, // seam: unadjusted -> adjusted
	{TS: d("2025-03-03"), Open: 6.57, High: 6.9, Low: 6.225, Close: 6.225, Volume: 23872},
	{TS: d("2025-03-04"), Open: 0.32300001, High: 0.35699999, Low: 0.29499999, Close: 0.352, Volume: 3939400}, // seam: back to unadjusted
	{TS: d("2025-03-05"), Open: 0.34999999, High: 0.41100001, Low: 0.33000001, Close: 0.41100001, Volume: 132800},
	{TS: d("2025-03-06"), Open: 0.34099999, High: 0.36000001, Low: 0.31999999, Close: 0.31999999, Volume: 307700},
	{TS: d("2025-03-07"), Open: 0.29499999, High: 0.29899999, Low: 0.25, Close: 0.252, Volume: 326400},
	{TS: d("2025-03-10"), Open: 3.4, High: 4.19, Low: 3.123, Close: 3.732, Volume: 110400}, // seam: adjusted again
	{TS: d("2025-03-11"), Open: 3.54, High: 3.7, Low: 3.23, Close: 3.33, Volume: 17900},
	{TS: d("2025-03-12"), Open: 3.3, High: 3.743, Low: 3.074, Close: 3.5, Volume: 50400},
	{TS: d("2025-03-13"), Open: 3.39, High: 3.5, Low: 3.31, Close: 3.31, Volume: 6100},
	{TS: d("2025-03-14"), Open: 3.13, High: 3.67, Low: 3.13, Close: 3.16, Volume: 28400},
}

// abtsTiingo is the SAME symbol and window from Tiingo, which applies one
// consistent adjustment factor throughout. It is the control: the detector must
// stay silent here, or it would flag every correctly adjusted series too.
var abtsTiingo = []Bar{
	{TS: d("2025-02-20"), Open: 6.7725, High: 6.8985, Low: 6.3, Close: 6.3, Volume: 394},
	{TS: d("2025-02-21"), Open: 6.5985, High: 6.738, Low: 6.165, Close: 6.165, Volume: 18340},
	{TS: d("2025-02-24"), Open: 6.5985, High: 6.65985, Low: 6.225, Close: 6.65985, Volume: 760},
	{TS: d("2025-02-25"), Open: 6.237, High: 6.735, Low: 6.237, Close: 6.33147, Volume: 1303},
	{TS: d("2025-02-26"), Open: 6.3, High: 6.6, Low: 6.3, Close: 6.304485, Volume: 1430},
	{TS: d("2025-02-27"), Open: 6.528, High: 6.533295, Low: 6.3, Close: 6.315, Volume: 438},
	{TS: d("2025-02-28"), Open: 6.225, High: 6.74532, Low: 6.225, Close: 6.24, Volume: 778},
	{TS: d("2025-03-03"), Open: 6.57, High: 6.9, Low: 6.225, Close: 6.225, Volume: 23872},
	{TS: d("2025-03-04"), Open: 4.8375, High: 5.3475, Low: 4.425, Close: 5.2725, Volume: 262631},
	{TS: d("2025-03-05"), Open: 5.25, High: 6.1665, Low: 4.95, Close: 6.1665, Volume: 8853},
	{TS: d("2025-03-06"), Open: 5.1165, High: 5.4, Low: 4.8, Close: 4.8015, Volume: 20516},
	{TS: d("2025-03-07"), Open: 4.4235, High: 4.4835, Low: 3.75, Close: 3.7725, Volume: 21760},
	{TS: d("2025-03-10"), Open: 3.4, High: 4.19, Low: 3.123, Close: 3.732, Volume: 110375},
	{TS: d("2025-03-11"), Open: 3.54, High: 3.7, Low: 3.23, Close: 3.33, Volume: 17417},
	{TS: d("2025-03-12"), Open: 3.3, High: 3.7428, Low: 3.0743, Close: 3.5, Volume: 50410},
	{TS: d("2025-03-13"), Open: 3.39, High: 3.5, Low: 3.31, Close: 3.31, Volume: 6170},
	{TS: d("2025-03-14"), Open: 3.13, High: 3.67, Low: 3.13, Close: 3.16, Volume: 28433},
}

func TestDetectAdjustmentBreaks_CatchesTheRealABTSAlternatingSeries(t *testing.T) {
	got := DetectAdjustmentBreaks(abtsTwelveData, DefaultConfig())
	if len(got) == 0 {
		t.Fatal("detector missed the ABTS series entirely — this is the exact defect it was written for")
	}

	found := map[string]Break{}
	for _, b := range got {
		found[b.TS.Format(time.DateOnly)] = b
		t.Logf("  flagged: %s", b.Reason())
	}

	// The unadjusted->adjusted seam. Price steps ~14.8x while dollar volume
	// barely moves, which is the whole signal.
	seam, ok := found["2025-02-28"]
	if !ok {
		t.Fatalf("missed the 2025-02-28 seam; flagged only %v", keys(found))
	}
	if seam.PriceRatio < 13 || seam.PriceRatio > 17 {
		t.Errorf("price ratio = %.2f, want ~14.8", seam.PriceRatio)
	}
	if seam.NearFactor != 15 {
		t.Errorf("NearFactor = %v, want 15 (ABTS did a 1-for-15 reverse split)", seam.NearFactor)
	}
	if seam.DollarVolumeRatio > 4 {
		t.Errorf("dollar volume ratio = %.2f; the seam is only diagnostic because this stayed near 1", seam.DollarVolumeRatio)
	}

	// The adjusted->unadjusted seam going back down.
	if _, ok := found["2025-03-04"]; !ok {
		t.Errorf("missed the 2025-03-04 reverse seam; flagged only %v", keys(found))
	}
}

// The control that makes the test meaningful: a correctly adjusted series of the
// SAME symbol over the SAME window, including a genuine -16% day and a real
// intraday range, must not be flagged. Without this, a detector that simply
// returns every bar would pass the test above.
func TestDetectAdjustmentBreaks_StaysSilentOnTheCorrectlyAdjustedSeries(t *testing.T) {
	got := DetectAdjustmentBreaks(abtsTiingo, DefaultConfig())
	if len(got) != 0 {
		for _, b := range got {
			t.Errorf("false positive on a correctly adjusted series: %s", b.Reason())
		}
	}
}

func keys(m map[string]Break) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ─── Discriminating artifacts from real moves ──────────────────────────────────

// The reason the detector looks at dollar volume at all. Penny stocks really do
// go up 300% in a day, and flagging those would make the gate useless noise —
// the pilot's penny bucket is 20% of the sample by design.
func TestDetectAdjustmentBreaks_IgnoresRealMovesWhereDollarVolumeExplodes(t *testing.T) {
	bars := []Bar{
		{TS: d("2025-05-01"), Close: 1.00, Volume: 50_000},
		// A genuine 4x squeeze: price up 4x AND volume up 200x, so dollar volume
		// goes up ~800x. Nothing about this is a rescaling seam.
		{TS: d("2025-05-02"), Close: 4.00, Volume: 10_000_000},
		{TS: d("2025-05-05"), Close: 3.50, Volume: 4_000_000},
	}
	if got := DetectAdjustmentBreaks(bars, DefaultConfig()); len(got) != 0 {
		for _, b := range got {
			t.Errorf("flagged a real momentum move: %s", b.Reason())
		}
	}
}

// The complementary case: the same price move, but with volume rescaled
// inversely so dollar volume is preserved. This is a seam and must be caught.
func TestDetectAdjustmentBreaks_CatchesSeamWhereDollarVolumeIsPreserved(t *testing.T) {
	bars := []Bar{
		{TS: d("2025-05-01"), Close: 1.00, Volume: 1_000_000},
		{TS: d("2025-05-02"), Close: 10.00, Volume: 100_000}, // 10x price, 1/10 volume
	}
	got := DetectAdjustmentBreaks(bars, DefaultConfig())
	if len(got) != 1 {
		t.Fatalf("got %d breaks, want 1", len(got))
	}
	if got[0].NearFactor != 10 {
		t.Errorf("NearFactor = %v, want 10", got[0].NearFactor)
	}
	if r := got[0].DollarVolumeRatio; r < 0.9 || r > 1.1 {
		t.Errorf("dollar volume ratio = %.3f, want ~1.0", r)
	}
}

// Reverse splits move the series the other way, and micro-caps do them
// constantly — 41% of the affected population was the penny bucket. Detection
// must be symmetric.
func TestDetectAdjustmentBreaks_IsSymmetricForForwardAndReverseSplits(t *testing.T) {
	down := []Bar{
		{TS: d("2025-05-01"), Close: 20.00, Volume: 100_000},
		{TS: d("2025-05-02"), Close: 1.00, Volume: 2_000_000}, // 1/20 price, 20x volume
	}
	got := DetectAdjustmentBreaks(down, DefaultConfig())
	if len(got) != 1 {
		t.Fatalf("got %d breaks, want 1 — a downward seam is as much a defect as an upward one", len(got))
	}
	if got[0].NearFactor != 20 {
		t.Errorf("NearFactor = %v, want 20", got[0].NearFactor)
	}
	if got[0].PriceRatio > 1 {
		t.Errorf("PriceRatio = %.3f, want <1 for a downward step", got[0].PriceRatio)
	}
}

// ─── Robustness ────────────────────────────────────────────────────────────────

func TestDetectAdjustmentBreaks_SkipsUnusableBarsRatherThanDividingByZero(t *testing.T) {
	bars := []Bar{
		{TS: d("2025-05-01"), Close: 0, Volume: 100},
		{TS: d("2025-05-02"), Close: 10, Volume: 0},
		{TS: d("2025-05-05"), Close: 10, Volume: 100},
		{TS: d("2025-05-06"), Close: 1, Volume: 1000},
	}
	got := DetectAdjustmentBreaks(bars, DefaultConfig())
	for _, b := range got {
		if b.TS.Equal(d("2025-05-02")) || b.TS.Equal(d("2025-05-01")) {
			t.Errorf("flagged a bar adjacent to a zero close/volume: %s", b.Reason())
		}
	}
}

func TestDetectAdjustmentBreaks_HandlesShortAndReversedInput(t *testing.T) {
	if got := DetectAdjustmentBreaks(nil, DefaultConfig()); got != nil {
		t.Error("nil input should yield no breaks")
	}
	if got := DetectAdjustmentBreaks([]Bar{{TS: d("2025-05-01"), Close: 1, Volume: 1}}, DefaultConfig()); got != nil {
		t.Error("a single bar has no close-to-close step")
	}

	// Newest-first input must produce the same finding, not a mirrored one.
	rev := make([]Bar, len(abtsTwelveData))
	for i, b := range abtsTwelveData {
		rev[len(abtsTwelveData)-1-i] = b
	}
	a := DetectAdjustmentBreaks(abtsTwelveData, DefaultConfig())
	b := DetectAdjustmentBreaks(rev, DefaultConfig())
	if len(a) != len(b) {
		t.Errorf("reversed input gave %d breaks vs %d; the function must sort defensively", len(b), len(a))
	}
}

// A ratio far from any plausible corporate action is still reported when dollar
// volume is continuous — the seam is the finding, and NearFactor is supporting
// evidence rather than a precondition. Requiring a round factor would miss
// providers whose rescaling is partial.
func TestDetectAdjustmentBreaks_ReportsSeamsWithNoRoundFactor(t *testing.T) {
	bars := []Bar{
		{TS: d("2025-05-01"), Close: 1.00, Volume: 1_000_000},
		{TS: d("2025-05-02"), Close: 3.40, Volume: 294_118}, // 3.4x, between the 3x and 4x factors
	}
	got := DetectAdjustmentBreaks(bars, DefaultConfig())
	if len(got) != 1 {
		t.Fatalf("got %d breaks, want 1", len(got))
	}
	if got[0].NearFactor != 0 {
		t.Errorf("NearFactor = %v, want 0 (3.4 is not a plausible split factor)", got[0].NearFactor)
	}
}

func TestBreakReason_CarriesTheEvidence(t *testing.T) {
	got := DetectAdjustmentBreaks(abtsTwelveData, DefaultConfig())
	if len(got) == 0 {
		t.Fatal("no breaks")
	}
	r := got[0].Reason()
	for _, want := range []string{"close", "dollar volume", "split factor"} {
		if !contains(r, want) {
			t.Errorf("Reason() missing %q: %s", want, r)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ─── Characterised limitation ──────────────────────────────────────────────────

// A real gap-up that happens to land near a round factor is a FALSE POSITIVE of
// the gap signal. This is BATL on 2026-01-26, from Tiingo — a correctly adjusted
// series — found while validating the detector against 64 real symbols:
//
//	close 1.2800 -> 3.9900 (3.12x); overnight gap 5.10x (near 5x)
//	dollar volume 44.54x
//
// It is kept as a test rather than tuned away because the discriminator is
// visible in the output and belongs in a reviewer's hands: dollar volume of 44x
// is a genuine buying surge, and no rescaling seam produces that. A seam holds
// dollar volume near 1x (ABTS: 1.75x) or at worst moderate when a real move
// coincides (ABTS: 9.33x).
//
// Hence the triage rule, which the Signals field exists to support:
//
//	both signals                          -> almost certainly a seam
//	dollar_volume_continuous only          -> very likely a seam
//	overnight_gap only, dollar volume >20x -> probably a real move; verify
//
// The measured false-positive rate on a correct source is ~3% of symbols
// (2 of 64). The detector is a SCREEN that produces a review list, not a verdict
// that rejects rows — claiming otherwise would trade one silent error for
// another.
func TestDetectAdjustmentBreaks_KnownFalsePositiveOnARealGapUp(t *testing.T) {
	batl := []Bar{
		{TS: d("2026-01-23"), Open: 1.30, High: 1.35, Low: 1.25, Close: 1.28, Volume: 120_000},
		{TS: d("2026-01-26"), Open: 6.53, High: 7.10, Low: 3.80, Close: 3.99, Volume: 1_715_000},
	}
	got := DetectAdjustmentBreaks(batl, DefaultConfig())
	if len(got) != 1 {
		t.Fatalf("got %d breaks, want 1 — this case IS flagged, and that is the documented limitation", len(got))
	}
	b := got[0]
	if b.DollarVolumeRatio < 20 {
		t.Errorf("dollar volume ratio = %.2f; the whole point of this fixture is that it is large", b.DollarVolumeRatio)
	}
	// The output must let a reviewer separate this from a real seam: exactly one
	// signal, and the volume-based one must NOT be among them.
	if len(b.Signals) != 1 || b.Signals[0] != "overnight_gap_on_split_factor" {
		t.Errorf("signals = %v; a real gap-up should fire only the gap signal, which is what makes it triageable", b.Signals)
	}
}
