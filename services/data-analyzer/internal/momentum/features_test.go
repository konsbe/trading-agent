package momentum

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

// Fixtures are hand-computed, per §8.2: "Unit-test every feature formula in §3
// against hand-computed fixtures — these formulas are the whole product, and a
// silent off-by-one in a window boundary is invisible in output but fatal to the
// results."
//
// Every expected value below is derived by hand in a comment beside it. Where a
// test asserts a window boundary, it is constructed so that an off-by-one
// produces a *different* number rather than a coincidentally equal one — a
// fixture where the boundary bar happens to match its neighbour proves nothing.

const eps = 1e-9

func day(i int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
}

// bar builds one bar with explicit OHLCV.
func bar(i int, o, h, l, c, v float64) compute.Bar {
	return compute.Bar{TS: day(i), Open: o, High: h, Low: l, Close: c, Volume: v}
}

// flat builds n bars all with the same OHLC and volume, as a neutral backdrop
// that contributes no accidental extremes.
func flat(n int, price, vol float64) []compute.Bar {
	out := make([]compute.Bar, n)
	for i := range out {
		out[i] = bar(i, price, price, price, price, vol)
	}
	return out
}

func mustF(t *testing.T, name string, got *float64) float64 {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want a value", name)
	}
	return *got
}

func closeTo(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	g := mustF(t, name, got)
	if math.Abs(g-want) > eps {
		t.Errorf("%s = %.10f, want %.10f", name, g, want)
	}
}

func mustNil(t *testing.T, name string, got any) {
	t.Helper()
	switch v := got.(type) {
	case *float64:
		if v != nil {
			t.Errorf("%s = %v, want nil — a missing input must stay null, never become a number", name, *v)
		}
	case *bool:
		if v != nil {
			t.Errorf("%s = %v, want nil", name, *v)
		}
	case *BreakoutState:
		if v != nil {
			t.Errorf("%s = %v, want nil", name, *v)
		}
	default:
		t.Fatalf("mustNil: unhandled type for %s", name)
	}
}

// ── §3.3 price and change ──────────────────────────────────────────────────

func TestChangeGapAndDollarVolume(t *testing.T) {
	// prior close 100; today opens 104, closes 110, volume 2,000.
	//   change_pct    = (110/100 - 1) * 100 = 10
	//   gap_pct       = (104/100 - 1) * 100 = 4
	//   dollar_volume = 110 * 2000          = 220,000
	bars := []compute.Bar{
		bar(0, 99, 101, 98, 100, 1000),
		bar(1, 104, 112, 103, 110, 2000),
	}
	f := Compute(bars, DefaultConfig())

	closeTo(t, "close", f.Close, 110)
	closeTo(t, "prior_close", f.PriorClose, 100)
	closeTo(t, "change_pct", f.ChangePct, 10)
	closeTo(t, "gap_pct", f.GapPct, 4)
	closeTo(t, "dollar_volume", f.DollarVolume, 220000)
}

// A single bar has no prior close, so change and gap must be null rather than
// zero — "flat" and "unknown" are different states and §3.2 gates on the value.
func TestSingleBarLeavesChangeNull(t *testing.T) {
	f := Compute([]compute.Bar{bar(0, 10, 11, 9, 10, 100)}, DefaultConfig())
	closeTo(t, "close", f.Close, 10)
	closeTo(t, "dollar_volume", f.DollarVolume, 1000)
	mustNil(t, "change_pct", f.ChangePct)
	mustNil(t, "gap_pct", f.GapPct)
	mustNil(t, "prior_close", f.PriorClose)
}

func TestEmptySeriesYieldsEmptyFeatures(t *testing.T) {
	f := Compute(nil, DefaultConfig())
	if f.BarsAvailable != 0 {
		t.Errorf("bars_available = %d, want 0", f.BarsAvailable)
	}
	mustNil(t, "close", f.Close)
	mustNil(t, "rvol_20", f.RVol20)
}

// ── §3.4 relative volume ───────────────────────────────────────────────────

// The single most important boundary in §3.4: "The baseline excludes the current
// bar. Including today deflates RVOL exactly when it matters most."
func TestRVol20_BaselineExcludesToday(t *testing.T) {
	// 20 prior bars at volume 1,000 each, then today at 8,000.
	//   avg_vol_20 = 1000                  (today excluded)
	//   rvol_20    = 8000 / 1000 = 8
	// If today were wrongly included the baseline would be
	//   (20*1000 + 8000) / 21 = 1333.33 and rvol would be 6.0 — a materially
	// different number, so this fixture actually catches the mistake.
	bars := flat(20, 100, 1000)
	bars = append(bars, bar(20, 100, 105, 99, 104, 8000))

	f := Compute(bars, DefaultConfig())
	closeTo(t, "avg_vol_20", f.AvgVol20, 1000)
	closeTo(t, "rvol_20", f.RVol20, 8)
}

// Exactly 20 prior bars is the minimum; 19 must yield null.
func TestRVol20_RequiresTwentyPriorBars(t *testing.T) {
	// 19 prior + today = 20 bars total: not enough.
	short := append(flat(19, 100, 1000), bar(19, 100, 101, 99, 100, 5000))
	if f := Compute(short, DefaultConfig()); f.RVol20 != nil || f.AvgVol20 != nil {
		t.Errorf("with 19 prior bars rvol must be nil, got rvol=%v avg=%v", f.RVol20, f.AvgVol20)
	}
	// 20 prior + today = 21 bars: exactly enough.
	ok := append(flat(20, 100, 1000), bar(20, 100, 101, 99, 100, 5000))
	if f := Compute(ok, DefaultConfig()); f.RVol20 == nil {
		t.Error("with 20 prior bars rvol must be computable")
	}
}

// §3.4: "If avg_vol_20 is 0 ... rvol_20 is null and the symbol fails the gate.
// Never substitute 0 or 1."
func TestRVol20_ZeroBaselineStaysNull(t *testing.T) {
	bars := flat(20, 100, 0) // twenty zero-volume bars
	bars = append(bars, bar(20, 100, 101, 99, 100, 5000))

	f := Compute(bars, DefaultConfig())
	closeTo(t, "avg_vol_20", f.AvgVol20, 0)
	mustNil(t, "rvol_20", f.RVol20)
}

// ── §3.5 volume acceleration ───────────────────────────────────────────────

func TestVolAccel_WindowBoundaries(t *testing.T) {
	// Eight bars, volumes chosen so each window has a distinct mean:
	//   index:   0    1    2    3    4    5    6    7
	//   volume: 10   20  100  200  300  700  800  900
	//   recent = mean(vol[5..7]) = (700+800+900)/3 = 800     (t-2..t)
	//   prior  = mean(vol[0..4]) = (10+20+100+200+300)/5 = 126
	//   vol_accel = 800 / 126 = 6.349206349...
	// Note bars 0 and 1 ARE in the prior window (t-7 = 0), so a window that
	// started one bar later would give 200 and a different answer.
	vols := []float64{10, 20, 100, 200, 300, 700, 800, 900}
	bars := make([]compute.Bar, len(vols))
	for i, v := range vols {
		bars[i] = bar(i, 100, 100, 100, 100, v)
	}

	f := Compute(bars, DefaultConfig())
	closeTo(t, "vol_accel", f.VolAccel, 800.0/126.0)
}

func TestVolAccel_RequiresEightBars(t *testing.T) {
	// The formula reads vol[t-7], so it needs 8 bars including today.
	if f := Compute(flat(7, 100, 1000), DefaultConfig()); f.VolAccel != nil {
		t.Errorf("with 7 bars vol_accel must be nil, got %v", *f.VolAccel)
	}
	if f := Compute(flat(8, 100, 1000), DefaultConfig()); f.VolAccel == nil {
		t.Error("with 8 bars vol_accel must be computable")
	}
}

// §3.5: vol_accel < 1.0 means volume is decaying even if RVOL is high. §4 scores
// that near zero rather than excluding it, so the value must be produced, not
// suppressed.
func TestVolAccel_DecayingVolumeIsReportedNotSuppressed(t *testing.T) {
	// recent mean 100, prior mean 1000 → 0.1
	vols := []float64{1000, 1000, 1000, 1000, 1000, 100, 100, 100}
	bars := make([]compute.Bar, len(vols))
	for i, v := range vols {
		bars[i] = bar(i, 100, 100, 100, 100, v)
	}
	f := Compute(bars, DefaultConfig())
	closeTo(t, "vol_accel", f.VolAccel, 0.1)
}

// ── §3.6 breakout geometry ─────────────────────────────────────────────────

// breakoutSeries builds 20 prior bars with a known high/low band, then today.
func breakoutSeries(priorHigh, priorLow, todayClose float64) []compute.Bar {
	bars := make([]compute.Bar, 0, 21)
	for i := 0; i < 20; i++ {
		// Mid-range bars, with the extremes placed on two specific days so the
		// window boundary matters.
		h, l := priorLow+1, priorLow
		if i == 5 {
			h = priorHigh
		}
		if i == 12 {
			l = priorLow
		}
		bars = append(bars, bar(i, l, h, l, l, 1000))
	}
	bars = append(bars, bar(20, todayClose, todayClose+1, todayClose-1, todayClose, 1000))
	return bars
}

func TestBreakoutState_AllFourValues(t *testing.T) {
	cfg := DefaultConfig()

	// Tight prior range: high 102, low 100 → range_20 = 2/close.
	// close 110 → 2/110 = 0.01818 < 0.25 ⇒ consolidating, and 110 > 102.
	f := Compute(breakoutSeries(102, 100, 110), cfg)
	closeTo(t, "resistance_20", f.Resistance20, 102)
	closeTo(t, "range_20", f.Range20, 2.0/110.0)
	if f.WasConsolidating == nil || !*f.WasConsolidating {
		t.Error("expected was_consolidating = true for a 1.8% prior range")
	}
	if f.BreakoutState == nil || *f.BreakoutState != BreakoutFromConsolidation {
		t.Errorf("breakout_state = %v, want %s", f.BreakoutState, BreakoutFromConsolidation)
	}

	// Wide prior range: high 200, low 100 → 100/210 = 0.476 ≥ 0.25 ⇒ not
	// consolidating, and 210 > 200 ⇒ plain breakout.
	f = Compute(breakoutSeries(200, 100, 210), cfg)
	closeTo(t, "range_20", f.Range20, 100.0/210.0)
	if f.WasConsolidating == nil || *f.WasConsolidating {
		t.Error("expected was_consolidating = false for a 47.6% prior range")
	}
	if f.BreakoutState == nil || *f.BreakoutState != Breakout {
		t.Errorf("breakout_state = %v, want %s", f.BreakoutState, Breakout)
	}

	// Approaching: resistance 102, close 101 → 101 >= 102*0.98 = 99.96, and not
	// above resistance.
	f = Compute(breakoutSeries(102, 100, 101), cfg)
	if f.BreakoutState == nil || *f.BreakoutState != BreakoutApproaching {
		t.Errorf("breakout_state = %v, want %s", f.BreakoutState, BreakoutApproaching)
	}

	// None: resistance 200, close 150 → 150 < 200*0.98 = 196.
	f = Compute(breakoutSeries(200, 100, 150), cfg)
	if f.BreakoutState == nil || *f.BreakoutState != BreakoutNone {
		t.Errorf("breakout_state = %v, want %s", f.BreakoutState, BreakoutNone)
	}
}

// §3.6's confirmation rule: "the breakout is judged on the close, never on an
// intrabar high. A wick above resistance that closes back below is not a
// breakout." This is called out as the main thing separating a real breakout
// from a failed one, so it gets its own test.
func TestBreakoutState_WickAboveResistanceIsNotABreakout(t *testing.T) {
	bars := breakoutSeries(102, 100, 101)
	// Today spikes to 150 intrabar but closes at 101, below resistance 102.
	last := len(bars) - 1
	bars[last] = bar(last, 101, 150, 100, 101, 1000)

	f := Compute(bars, DefaultConfig())
	closeTo(t, "resistance_20", f.Resistance20, 102)
	if f.BreakoutState == nil {
		t.Fatal("breakout_state is nil")
	}
	if *f.BreakoutState == Breakout || *f.BreakoutState == BreakoutFromConsolidation {
		t.Errorf("breakout_state = %s; a 150 wick closing at 101 under resistance 102 must NOT be a breakout", *f.BreakoutState)
	}
	if *f.BreakoutState != BreakoutApproaching {
		t.Errorf("breakout_state = %s, want %s", *f.BreakoutState, BreakoutApproaching)
	}
}

// Today's own high must not contribute to resistance_20, or a symbol could break
// out over a level it set itself.
func TestResistance20_ExcludesTodaysHigh(t *testing.T) {
	bars := breakoutSeries(102, 100, 101)
	last := len(bars) - 1
	bars[last] = bar(last, 101, 999, 100, 101, 1000) // enormous high today

	f := Compute(bars, DefaultConfig())
	closeTo(t, "resistance_20", f.Resistance20, 102) // unchanged by the 999
}

// ── §3.7 52-week high proximity ────────────────────────────────────────────

// The window excludes today, which is what makes §4.2's top band reachable: a
// genuine new high gives a ratio above 1.0. An earlier revision included today,
// capping the ratio at 1.0 and making that band unreachable.
func TestPctOf52wHigh_ExcludesTodayAndExceedsOneOnNewHigh(t *testing.T) {
	// 251 prior bars with a peak high of 200, then today closes at 220.
	bars := make([]compute.Bar, 0, 252)
	for i := 0; i < 251; i++ {
		h := 150.0
		if i == 100 {
			h = 200 // the 52-week peak
		}
		bars = append(bars, bar(i, 100, h, 90, 100, 1000))
	}
	bars = append(bars, bar(251, 210, 230, 205, 220, 1000))

	f := Compute(bars, DefaultConfig())
	closeTo(t, "high_52w", f.High52w, 200)
	//   pct_of_52w_high = 220 / 200 = 1.10  — above 1.0, i.e. a new high
	closeTo(t, "pct_of_52w_high", f.PctOf52wHigh, 1.10)
	if v := mustF(t, "pct_of_52w_high", f.PctOf52wHigh); v <= 1.0 {
		t.Errorf("ratio = %v; a new 52-week high must exceed 1.0 or §4.2's top band is unreachable", v)
	}
}

func TestPctOf52wHigh_BelowHighGivesRatioUnderOne(t *testing.T) {
	bars := make([]compute.Bar, 0, 252)
	for i := 0; i < 251; i++ {
		h := 150.0
		if i == 10 {
			h = 200
		}
		bars = append(bars, bar(i, 100, h, 90, 100, 1000))
	}
	// close 190 against a 200 peak → 0.95
	bars = append(bars, bar(251, 188, 192, 187, 190, 1000))

	f := Compute(bars, DefaultConfig())
	closeTo(t, "pct_of_52w_high", f.PctOf52wHigh, 0.95)
}

// The window reads high[t-251], so it needs 252 bars including today — which is
// exactly why §3.1's eligibility minimum is 252 and not 250.
func TestHigh52w_Requires252Bars(t *testing.T) {
	if f := Compute(flat(251, 100, 1000), DefaultConfig()); f.High52w != nil {
		t.Errorf("with 251 bars high_52w must be nil, got %v", *f.High52w)
	}
	if f := Compute(flat(252, 100, 1000), DefaultConfig()); f.High52w == nil {
		t.Error("with 252 bars high_52w must be computable")
	}
}

// ── §3.8 rolling VWAP ──────────────────────────────────────────────────────

func TestVWAP20_TypicalPriceWeightedAndIncludesToday(t *testing.T) {
	// 19 bars with typical price 100 and volume 1,000, then today with
	// typical (120+80+100)/3 = 100 and volume 10,000. All typicals are 100, so
	// VWAP = 100 regardless of weights — and close 100 means above_vwap is false.
	bars := make([]compute.Bar, 0, 20)
	for i := 0; i < 19; i++ {
		bars = append(bars, bar(i, 100, 100, 100, 100, 1000))
	}
	bars = append(bars, bar(19, 100, 120, 80, 100, 10000))

	f := Compute(bars, DefaultConfig())
	closeTo(t, "vwap_20", f.VWAP20, 100)
	closeTo(t, "vwap_dist_pct", f.VWAPDistPct, 0)
	if f.AboveVWAP == nil || *f.AboveVWAP {
		t.Error("close equal to VWAP must give above_vwap = false (strict >)")
	}
}

func TestVWAP20_VolumeWeightingAndDistance(t *testing.T) {
	// 19 bars: typical 100, volume 1,000  → Σpv = 19 * 100,000 = 1,900,000
	// today:   typical 200, volume 81,000 → Σpv += 16,200,000
	//   Σpv = 18,100,000 ; Σv = 19,000 + 81,000 = 100,000
	//   vwap_20 = 181
	//   close 200 → vwap_dist_pct = (200/181 - 1)*100 = 10.4972375691%
	bars := make([]compute.Bar, 0, 20)
	for i := 0; i < 19; i++ {
		bars = append(bars, bar(i, 100, 100, 100, 100, 1000))
	}
	bars = append(bars, bar(19, 200, 200, 200, 200, 81000))

	f := Compute(bars, DefaultConfig())
	closeTo(t, "vwap_20", f.VWAP20, 181)
	closeTo(t, "vwap_dist_pct", f.VWAPDistPct, (200.0/181.0-1)*100)
	if f.AboveVWAP == nil || !*f.AboveVWAP {
		t.Error("close 200 above vwap 181 must give above_vwap = true")
	}
}

func TestVWAP20_RequiresTwentyBars(t *testing.T) {
	if f := Compute(flat(19, 100, 1000), DefaultConfig()); f.VWAP20 != nil {
		t.Errorf("with 19 bars vwap must be nil, got %v", *f.VWAP20)
	}
	if f := Compute(flat(20, 100, 1000), DefaultConfig()); f.VWAP20 == nil {
		t.Error("with 20 bars vwap must be computable")
	}
}

// ── §3.3 ATR / §3.10 RSI, delegated to internal/compute ────────────────────

func TestATR_MatchesComputeAndYieldsPercent(t *testing.T) {
	// Ramp so true range is a constant 2 per bar: each bar's high-low is 2 and
	// the gaps are smaller, so Wilder ATR converges to exactly 2.
	bars := make([]compute.Bar, 0, 30)
	for i := 0; i < 30; i++ {
		c := 100 + float64(i)*0.5
		bars = append(bars, bar(i, c, c+1, c-1, c, 1000))
	}
	f := Compute(bars, DefaultConfig())

	want, ok := compute.ATRWilder(compute.Highs(bars), compute.Lows(bars), compute.Closes(bars), 14)
	if !ok {
		t.Fatal("compute.ATRWilder failed on the fixture")
	}
	closeTo(t, "atr_14", f.ATR14, want)
	// atr_pct = atr / close * 100
	closeTo(t, "atr_pct", f.ATRPct, want/bars[len(bars)-1].Close*100)
}

func TestRSI_MatchesComputeAndIsNilWhenShort(t *testing.T) {
	bars := make([]compute.Bar, 0, 30)
	for i := 0; i < 30; i++ {
		c := 100 + float64(i) // monotonically rising
		bars = append(bars, bar(i, c, c+1, c-1, c, 1000))
	}
	f := Compute(bars, DefaultConfig())
	want, ok := compute.RSI(compute.Closes(bars), 14)
	if !ok {
		t.Fatal("compute.RSI failed on the fixture")
	}
	closeTo(t, "rsi_14", f.RSI14, want)
	// An unbroken uptrend pins RSI at 100.
	if v := mustF(t, "rsi_14", f.RSI14); v < 99.9 {
		t.Errorf("rsi = %v on a monotonic rise, want ~100", v)
	}
	// RSI needs period+1 bars.
	if f := Compute(flat(14, 100, 1000), DefaultConfig()); f.RSI14 != nil {
		t.Errorf("with 14 bars rsi must be nil, got %v", *f.RSI14)
	}
}

// ── §3.12 input ────────────────────────────────────────────────────────────

func TestChangePct5d(t *testing.T) {
	// closes: 100, 101, 102, 103, 104, 110 (six bars)
	//   change_pct_5d = (110 / close[t-5] - 1) * 100 = (110/100 - 1)*100 = 10
	// close[t-5] is the FIRST bar; an off-by-one would use 101 and give 8.91.
	closes := []float64{100, 101, 102, 103, 104, 110}
	bars := make([]compute.Bar, len(closes))
	for i, c := range closes {
		bars[i] = bar(i, c, c, c, c, 1000)
	}
	f := Compute(bars, DefaultConfig())
	closeTo(t, "change_pct_5d", f.ChangePct5d, 10)

	// Five bars is not enough: the formula reads close[t-5].
	if f := Compute(bars[:5], DefaultConfig()); f.ChangePct5d != nil {
		t.Errorf("with 5 bars change_pct_5d must be nil, got %v", *f.ChangePct5d)
	}
}

// ── §6 no-lookahead ────────────────────────────────────────────────────────

// ramp builds a long, irregular series. Deliberately not monotonic and not
// periodic: a leak of future data into a smooth series can cancel out, whereas
// here any forward read changes the result.
func ramp(n int) []compute.Bar {
	bars := make([]compute.Bar, n)
	price := 50.0
	vol := 1000.0
	for i := 0; i < n; i++ {
		// Pseudo-random but deterministic wobble.
		price += math.Sin(float64(i)*0.7)*1.5 + math.Cos(float64(i)*0.31)*0.9
		if price < 5 {
			price = 5
		}
		vol = 800 + math.Abs(math.Sin(float64(i)*0.23))*9000
		h := price * 1.02
		l := price * 0.98
		o := price * (1 + math.Sin(float64(i))*0.005)
		bars[i] = bar(i, o, h, l, price, vol)
	}
	return bars
}

// THE no-lookahead test §6 requires: "Features for day t may use only bars up to
// and including t. ... Any leakage makes the whole exercise worthless, and it is
// easy to introduce accidentally — write a test that asserts it."
//
// ComputeAt(bars, i) is given the FULL series and must produce exactly what
// Compute(bars[:i+1]) produces from a truncated one. Any forward index — an
// inclusive bound that should be exclusive, a window centred instead of trailing
// — makes these diverge. Checked field by field via Features.Equal, so a leak in
// any single field fails, including fields added later.
func TestNoLookahead_ComputeAtMatchesTruncatedSeries(t *testing.T) {
	full := ramp(600)
	cfg := DefaultConfig()

	checked := 0
	for i := range full {
		fromFull := ComputeAt(full, i, cfg)
		fromTruncated := Compute(full[:i+1], cfg)
		if !fromFull.Equal(fromTruncated) {
			t.Fatalf("lookahead detected at i=%d — these fields read data after i:\n  %s",
				i, strings.Join(fromFull.Diff(fromTruncated), "\n  "))
		}
		checked++
	}
	if checked < 600 {
		t.Fatalf("only checked %d indices", checked)
	}

	// Sanity: the fixture must actually exercise the long windows, or the test
	// would pass trivially on a series too short to have 52-week features.
	last := ComputeAt(full, len(full)-1, cfg)
	for name, v := range map[string]*float64{
		"rvol_20": last.RVol20, "vol_accel": last.VolAccel,
		"high_52w": last.High52w, "vwap_20": last.VWAP20, "rsi_14": last.RSI14,
	} {
		if v == nil {
			t.Errorf("%s is nil at the end of a 600-bar series; the no-lookahead test is not exercising it", name)
		}
	}
}

// Appending future bars must not change an earlier day's features. This is the
// same property from the other direction, and it is the one that would catch a
// leak introduced through a cached or package-level value rather than an index.
func TestNoLookahead_AppendingFutureBarsChangesNothing(t *testing.T) {
	cfg := DefaultConfig()
	base := ramp(400)
	extended := append(append([]compute.Bar{}, base...), ramp(50)...)

	for _, i := range []int{260, 300, 350, 399} {
		before := ComputeAt(base, i, cfg)
		after := ComputeAt(extended, i, cfg)
		if !before.Equal(after) {
			t.Errorf("features for i=%d changed after appending future bars — affected fields:\n  %s",
				i, strings.Join(before.Diff(after), "\n  "))
		}
	}
}

// ComputeAt must reject out-of-range indices rather than silently clamping to
// the last bar, which would mislabel a day's features.
func TestComputeAt_RejectsOutOfRangeIndex(t *testing.T) {
	bars := flat(10, 100, 1000)
	for _, i := range []int{-1, 10, 999} {
		f := ComputeAt(bars, i, DefaultConfig())
		if f.BarsAvailable != 0 || f.Close != nil {
			t.Errorf("ComputeAt(i=%d) returned data; want an empty row", i)
		}
	}
}

// BarsAvailable must report the window size, not the input size, so a caller can
// distinguish "nil because history was short" from "nil because data was bad".
func TestBarsAvailableReflectsTheWindowNotTheSeries(t *testing.T) {
	bars := flat(100, 100, 1000)
	if f := ComputeAt(bars, 49, DefaultConfig()); f.BarsAvailable != 50 {
		t.Errorf("bars_available = %d at i=49, want 50", f.BarsAvailable)
	}
	if f := ComputeAt(bars, 99, DefaultConfig()); f.BarsAvailable != 100 {
		t.Errorf("bars_available = %d at i=99, want 100", f.BarsAvailable)
	}
}

// Config is honoured rather than the defaults being hardcoded — §3.2 requires
// every threshold to be env-configurable.
func TestConfigThresholdsAreHonoured(t *testing.T) {
	bars := flat(20, 100, 1000)
	bars = append(bars, bar(20, 100, 101, 99, 100, 5000))

	// A 10-bar RVOL baseline needs only 11 bars, and gives the same answer here,
	// but the shorter window must be what is used.
	cfg := DefaultConfig()
	cfg.RVolLookback = 10
	if f := Compute(bars[:12], cfg); f.RVol20 == nil {
		t.Error("with RVolLookback=10 and 12 bars rvol must be computable")
	}
	// The default 20 needs 21, so 12 bars must yield nil.
	if f := Compute(bars[:12], DefaultConfig()); f.RVol20 != nil {
		t.Error("with the default lookback of 20, 12 bars must yield nil")
	}

	// Consolidation threshold: a 2/100 = 2% range is consolidating at 0.25 but
	// not at 0.01.
	strict := DefaultConfig()
	strict.ConsolidationMaxRange = 0.01
	f := Compute(breakoutSeries(102, 100, 100), strict)
	if f.WasConsolidating == nil || *f.WasConsolidating {
		t.Error("a 2% range must not be consolidating when the threshold is 1%")
	}
}
