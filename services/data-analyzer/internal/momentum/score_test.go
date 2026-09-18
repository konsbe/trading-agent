package momentum

import (
	"math"
	"testing"
)

func passingGate(b Bucket) GateResult {
	return GateResult{Bucket: b, Bucketed: true, Passed: true}
}

// scoreFeatures builds a mid-range candidate so each test can vary one input.
func scoreFeatures() *Features {
	return &Features{
		BarsAvailable: 300,
		Close:         ptr(50.0),
		ChangePct:     ptr(12.0),
		DollarVolume:  ptr(10e6),
		RVol20:        ptr(4.0),
		VolAccel:      ptr(2.0),
		BreakoutState: ptrState(Breakout),
		AboveVWAP:     ptr(true),
		PctOf52wHigh:  ptr(0.97),
		RSI14:         ptr(65.0),
	}
}

func ptrState(s BreakoutState) *BreakoutState { return &s }

// ─── The weight table (§4.1's correction) ──────────────────────────────────────

// §4.1 resolved the original table's double-count (it listed both "Volume +25"
// and "Relative Volume +20" for the same underlying quantity) into volume
// ACCELERATION 25 plus relative volume 20. The total must still be 100, or the
// score is no longer out of 100.
func TestWeights_SumToOneHundred(t *testing.T) {
	if WeightTotal != 100 {
		t.Fatalf("weights sum to %d, want 100", WeightTotal)
	}
	sum := WeightVolAccel + WeightRVol + WeightBreakout + WeightCatalyst + WeightFloat + WeightVWAP + WeightHigh52w
	if sum != 100 {
		t.Errorf("component weights sum to %d, want 100", sum)
	}
	// Acceleration must outweigh relative volume: §4.1's whole point is "volume
	// increasing, not just high volume".
	if WeightVolAccel <= WeightRVol {
		t.Errorf("vol_accel weight %d should exceed rvol weight %d", WeightVolAccel, WeightRVol)
	}
}

// A perfect candidate must reach exactly 100, otherwise the scale is wrong.
func TestScoreCandidate_PerfectCandidateScores100(t *testing.T) {
	f := scoreFeatures()
	f.VolAccel = ptr(5.0)                                 // > 4.0 -> 25
	f.RVol20 = ptr(12.0)                                  // > 10  -> 20
	f.BreakoutState = ptrState(BreakoutFromConsolidation) // -> 20
	f.AboveVWAP = ptr(true)                               // -> 5
	f.PctOf52wHigh = ptr(1.05)                            // >= 1.00 -> 5
	f.RSI14 = ptr(70.0)                                   // no penalty
	f.ChangePct = ptr(12.0)                               // no penalty

	in := ScoreInput{CatalystTier: CatalystA, FloatSharesEst: ptr(10e6)} // 15 + 10

	s, ok := ScoreCandidate(f, in, passingGate(BucketMarket))
	if !ok {
		t.Fatal("should score")
	}
	if s.Total != 100 {
		t.Errorf("Total = %d, want 100. Breakdown: %+v", s.Total, s.Sub)
	}
	if len(s.Penalties) != 0 {
		t.Errorf("unexpected penalties: %v", s.Penalties)
	}
	if len(s.NullInputs) != 0 {
		t.Errorf("unexpected nulls: %v", s.NullInputs)
	}
}

// Hand-computed, component by component, so a wrong band boundary fails on a
// specific number rather than on a vague total.
func TestScoreCandidate_HandComputedBreakdown(t *testing.T) {
	f := scoreFeatures()
	f.VolAccel = ptr(2.0)                // band 1.5-2.5 -> 10 + (0.5/1.0)*10 = 15
	f.RVol20 = ptr(4.0)                  // band 3.0-5.0 -> 12 + (1.0/2.0)*6  = 15
	f.BreakoutState = ptrState(Breakout) // 14
	f.AboveVWAP = ptr(true)              // 5
	f.PctOf52wHigh = ptr(0.97)           // 0.95-1.00 -> 4
	f.RSI14 = ptr(65.0)                  // no penalty
	f.ChangePct = ptr(12.0)              // no penalty

	in := ScoreInput{CatalystTier: CatalystB, FloatSharesEst: ptr(30e6)} // 8, and 20M-50M -> 7

	s, ok := ScoreCandidate(f, in, passingGate(BucketMarket))
	if !ok {
		t.Fatal("should score")
	}

	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"vol_accel", s.Sub.VolAccel, 15},
		{"rvol", s.Sub.RVol, 15},
		{"breakout", s.Sub.Breakout, 14},
		{"catalyst", s.Sub.Catalyst, 8},
		{"float", s.Sub.Float, 7},
		{"vwap", s.Sub.VWAP, 5},
		{"high52w", s.Sub.High52w, 4},
	} {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %.4f, want %.4f", c.name, c.got, c.want)
		}
	}
	// 15+15+14+8+7+5+4 = 68
	if math.Abs(s.Raw-68) > 1e-9 {
		t.Errorf("Raw = %.4f, want 68", s.Raw)
	}
	if s.Total != 68 {
		t.Errorf("Total = %d, want 68", s.Total)
	}
}

// ─── Band interpolation ────────────────────────────────────────────────────────

func TestScoreCandidate_VolAccelBands(t *testing.T) {
	for _, c := range []struct {
		accel, want float64
		why         string
	}{
		{0.5, 0, "below 1.0 scores nothing: volume is decaying"},
		{1.0, 0, "the band's left edge"},
		{1.25, 5, "midpoint of 1.0-1.5 -> midpoint of 0-10"},
		{1.5, 10, "boundary belongs to the next band's left edge value"},
		{2.5, 20, "boundary"},
		{3.25, 22.5, "midpoint of 2.5-4.0 -> midpoint of 20-25"},
		{4.0, 25, "top of the ramp"},
		{99, 25, "clamped, never extrapolated"},
	} {
		f := scoreFeatures()
		f.VolAccel = ptr(c.accel)
		// Suppress the decaying-volume penalty so this isolates the sub-score.
		f.RVol20 = ptr(2.0)
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if math.Abs(s.Sub.VolAccel-c.want) > 1e-9 {
			t.Errorf("vol_accel %.2f -> %.4f, want %.4f (%s)", c.accel, s.Sub.VolAccel, c.want, c.why)
		}
	}
}

func TestScoreCandidate_RVolBands(t *testing.T) {
	for _, c := range []struct{ rvol, want float64 }{
		{1.0, 0}, {1.5, 0}, {2.25, 6}, {3.0, 12}, {4.0, 15}, {5.0, 18}, {7.5, 19}, {10.0, 20}, {50, 20},
	} {
		f := scoreFeatures()
		f.RVol20 = ptr(c.rvol)
		f.VolAccel = ptr(2.0) // keep it from tripping the decay penalty
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if math.Abs(s.Sub.RVol-c.want) > 1e-9 {
			t.Errorf("rvol %.2f -> %.4f, want %.4f", c.rvol, s.Sub.RVol, c.want)
		}
	}
}

// Float and 52-week proximity are STEP bands in §4.2, not ramps. Interpolating
// float would invent precision the §3.9 estimate does not have.
func TestScoreCandidate_FloatAndHigh52wAreStepsNotRamps(t *testing.T) {
	for _, c := range []struct{ shares, want float64 }{
		{5e6, 10}, {19.9e6, 10}, {20e6, 7}, {49e6, 7}, {50e6, 4}, {99e6, 4},
		{100e6, 2}, {299e6, 2}, {300e6, 0}, {1e9, 0},
	} {
		f := scoreFeatures()
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone, FloatSharesEst: ptr(c.shares)}, passingGate(BucketMarket))
		if s.Sub.Float != c.want {
			t.Errorf("float %.0f -> %.1f, want %.1f", c.shares, s.Sub.Float, c.want)
		}
	}

	for _, c := range []struct{ ratio, want float64 }{
		{0.80, 0}, {0.899, 0}, {0.90, 2}, {0.949, 2}, {0.95, 4}, {0.999, 4}, {1.00, 5}, {1.30, 5},
	} {
		f := scoreFeatures()
		f.PctOf52wHigh = ptr(c.ratio)
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if s.Sub.High52w != c.want {
			t.Errorf("pct_of_52w_high %.3f -> %.1f, want %.1f", c.ratio, s.Sub.High52w, c.want)
		}
	}
}

// §3.7's window excludes today, so a ratio above 1.0 IS the new-high case and
// the top band is genuinely reachable. An earlier spec version capped the ratio
// at 1.0, which made this band dead.
func TestScoreCandidate_NewFiftyTwoWeekHighBandIsReachable(t *testing.T) {
	f := scoreFeatures()
	f.PctOf52wHigh = ptr(1.02)
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if s.Sub.High52w != WeightHigh52w {
		t.Errorf("a ratio above 1.0 must score the full %d, got %.1f", WeightHigh52w, s.Sub.High52w)
	}
}

// ─── Penalties (§4.3) ──────────────────────────────────────────────────────────

// "Already extended" enforces the core thesis inside the score: the system fires
// at +8-15%, so a same-day +22% move is later in the sequence than the intended
// entry. It is the score-side companion to §3.2's change_pct ceiling.
func TestScoreCandidate_AlreadyExtendedPenalty(t *testing.T) {
	f := scoreFeatures()
	f.ChangePct = ptr(22.0)
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))

	if s.PenaltyPoints != PenaltyAlreadyExtended {
		t.Errorf("PenaltyPoints = %.0f, want %d", s.PenaltyPoints, PenaltyAlreadyExtended)
	}
	if !hasStr(s.Penalties, ReasonAlreadyExtended) {
		t.Errorf("want %s in %v", ReasonAlreadyExtended, s.Penalties)
	}
	if got := int(math.Round(s.Raw)) - s.Total; got != PenaltyAlreadyExtended {
		t.Errorf("penalty not applied to the total: raw %.1f total %d", s.Raw, s.Total)
	}

	// Exactly 20 is not "over 20".
	f.ChangePct = ptr(20.0)
	if s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket)); s.PenaltyPoints != 0 {
		t.Errorf("change_pct of exactly 20 must not be penalised, got %.0f", s.PenaltyPoints)
	}
}

// The explicit case from the original brief: high volume that is NOT increasing
// should be visibly marked down rather than silently scoring well. Both
// conditions are required, since decaying volume alone already scores 0 on the
// acceleration component.
func TestScoreCandidate_VolumeDecayingPenaltyNeedsBothConditions(t *testing.T) {
	// Decaying AND high RVOL -> penalised.
	f := scoreFeatures()
	f.VolAccel = ptr(0.8)
	f.RVol20 = ptr(5.0)
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if !hasStr(s.Penalties, ReasonVolumeDecaying) {
		t.Errorf("want %s in %v", ReasonVolumeDecaying, s.Penalties)
	}

	// Decaying but LOW RVOL -> no penalty; the 0 on acceleration is the whole
	// story, and double-counting would punish it twice.
	f.RVol20 = ptr(2.0)
	s, _ = ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if hasStr(s.Penalties, ReasonVolumeDecaying) {
		t.Errorf("rvol below 3 must not trigger the decay penalty: %v", s.Penalties)
	}

	// Accelerating with high RVOL -> no penalty.
	f.VolAccel = ptr(2.0)
	f.RVol20 = ptr(5.0)
	s, _ = ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if hasStr(s.Penalties, ReasonVolumeDecaying) {
		t.Errorf("accelerating volume must not be penalised: %v", s.Penalties)
	}
}

func TestScoreCandidate_ExhaustedRSIPenalty(t *testing.T) {
	f := scoreFeatures()
	f.RSI14 = ptr(86.0)
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if !hasStr(s.Penalties, ReasonExhaustedRSI) {
		t.Errorf("want %s in %v", ReasonExhaustedRSI, s.Penalties)
	}

	f.RSI14 = ptr(85.0) // "> 85", so 85 itself is not penalised
	if s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket)); hasStr(s.Penalties, ReasonExhaustedRSI) {
		t.Error("rsi of exactly 85 must not be penalised")
	}
}

// §3.10 makes RSI a penalty input only. A missing RSI therefore means "no
// evidence of exhaustion", not "assume exhausted" — but it is still recorded.
func TestScoreCandidate_MissingRSIIsNotPenalised(t *testing.T) {
	f := scoreFeatures()
	f.RSI14 = nil
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if hasStr(s.Penalties, ReasonExhaustedRSI) {
		t.Error("a null RSI must not be treated as exhausted")
	}
	if !hasStr(s.NullInputs, NullRSI) {
		t.Errorf("a null RSI must still be recorded: %v", s.NullInputs)
	}
}

// Penalties can exceed the raw score; the result clamps at 0 rather than going
// negative, since momentum_score_100 is defined on [0,100].
func TestScoreCandidate_ClampsAtZeroRatherThanGoingNegative(t *testing.T) {
	f := scoreFeatures()
	f.VolAccel = ptr(0.5) // 0
	f.RVol20 = ptr(3.0)   // 12, and with accel<1 triggers the decay penalty
	f.BreakoutState = ptrState(BreakoutNone)
	f.AboveVWAP = ptr(false)
	f.PctOf52wHigh = ptr(0.5)
	f.RSI14 = ptr(90.0)     // -5
	f.ChangePct = ptr(30.0) // -10
	in := ScoreInput{CatalystTier: CatalystNone, FloatSharesEst: ptr(500e6)}

	s, _ := ScoreCandidate(f, in, passingGate(BucketMarket))
	if s.Total < 0 {
		t.Errorf("Total = %d, must clamp at 0", s.Total)
	}
	if s.PenaltyPoints != PenaltyExhaustedRSI+PenaltyAlreadyExtended+PenaltyVolumeDecaying {
		t.Errorf("PenaltyPoints = %.0f, want %d", s.PenaltyPoints,
			PenaltyExhaustedRSI+PenaltyAlreadyExtended+PenaltyVolumeDecaying)
	}
	// Raw must survive clamping, or a 0 is indistinguishable from "scored
	// nothing" versus "scored 12 and lost 20".
	if s.Raw <= 0 {
		t.Errorf("Raw = %.1f should record the pre-penalty sum", s.Raw)
	}
}

// ─── Gate coupling ─────────────────────────────────────────────────────────────

// §3.2 is explicit that a gate failure means EXCLUDED, not low-scored. Returning
// a score for a failed symbol would let it appear in a ranked list, so the
// refusal is structural rather than advisory.
func TestScoreCandidate_RefusesToScoreAGateFailure(t *testing.T) {
	f := scoreFeatures()
	failed := GateResult{Bucket: BucketMarket, Bucketed: true, Passed: false,
		Failures: []string{GateRVolTooLow}}

	if _, ok := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystA}, failed); ok {
		t.Error("a gate-failed symbol must not receive a score")
	}
	if _, ok := ScoreCandidate(nil, ScoreInput{}, passingGate(BucketMarket)); ok {
		t.Error("nil features must not receive a score")
	}
}

// ─── §4.4 output contract ──────────────────────────────────────────────────────

// §4.4: "A score with no visible breakdown is undebuggable, and Phase 2 needs
// the components independently." Every component must be separately recoverable
// and must reconstruct the raw total.
func TestScoreCandidate_BreakdownReconstructsTheTotal(t *testing.T) {
	f := scoreFeatures()
	in := ScoreInput{CatalystTier: CatalystB, FloatSharesEst: ptr(45e6)}
	s, _ := ScoreCandidate(f, in, passingGate(BucketMarket))

	if math.Abs(s.Sub.Sum()-s.Raw) > 1e-9 {
		t.Errorf("sub-scores sum to %.4f but Raw is %.4f", s.Sub.Sum(), s.Raw)
	}
	if int(math.Round(s.Raw-s.PenaltyPoints)) != s.Total {
		t.Errorf("raw %.2f - penalties %.2f != total %d", s.Raw, s.PenaltyPoints, s.Total)
	}
	if s.Bucket != BucketMarket {
		t.Errorf("Bucket = %q, want market — the bucket must travel with the score", s.Bucket)
	}
}

// A null input scores 0 for its component AND is recorded, so a low score is
// attributable to missing data rather than to a weak signal.
func TestScoreCandidate_NullInputsScoreZeroAndAreRecorded(t *testing.T) {
	f := &Features{BarsAvailable: 300, Close: ptr(10.0), ChangePct: ptr(10.0)}
	s, ok := ScoreCandidate(f, ScoreInput{}, passingGate(BucketMarket))
	if !ok {
		t.Fatal("should still score")
	}
	if s.Total != 0 {
		t.Errorf("Total = %d, want 0 when every component is null", s.Total)
	}
	for _, want := range []string{NullVolAccel, NullRVol, NullBreakout, NullCatalyst, NullFloat, NullVWAP, NullHigh52w, NullRSI} {
		if !hasStr(s.NullInputs, want) {
			t.Errorf("missing %s from NullInputs %v", want, s.NullInputs)
		}
	}
}

// An unset catalyst tier means "news was never fetched"; CatalystNone means
// "fetched and found nothing". Both score 0, but conflating them would hide an
// ingestion gap behind a legitimate-looking result.
func TestScoreCandidate_UnfetchedCatalystIsDistinctFromNoCatalyst(t *testing.T) {
	f := scoreFeatures()

	unfetched, _ := ScoreCandidate(f, ScoreInput{}, passingGate(BucketMarket))
	if !hasStr(unfetched.NullInputs, NullCatalyst) {
		t.Errorf("an unset tier must be recorded as null: %v", unfetched.NullInputs)
	}

	fetched, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if hasStr(fetched.NullInputs, NullCatalyst) {
		t.Errorf("an explicit 'none' is a finding, not a null: %v", fetched.NullInputs)
	}
	if unfetched.Sub.Catalyst != 0 || fetched.Sub.Catalyst != 0 {
		t.Error("both should score 0 points")
	}
}

func TestScoreCandidate_CatalystTiers(t *testing.T) {
	for _, c := range []struct {
		tier CatalystTier
		want float64
	}{{CatalystA, 15}, {CatalystB, 8}, {CatalystNone, 0}} {
		f := scoreFeatures()
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: c.tier}, passingGate(BucketMarket))
		if s.Sub.Catalyst != c.want {
			t.Errorf("tier %q -> %.1f, want %.1f", c.tier, s.Sub.Catalyst, c.want)
		}
	}
}

func TestScoreCandidate_BreakoutStatePoints(t *testing.T) {
	for _, c := range []struct {
		st   BreakoutState
		want float64
	}{
		{BreakoutFromConsolidation, 20},
		{Breakout, 14},
		{BreakoutApproaching, 8},
		{BreakoutNone, 0},
	} {
		f := scoreFeatures()
		f.BreakoutState = ptrState(c.st)
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if s.Sub.Breakout != c.want {
			t.Errorf("breakout %q -> %.1f, want %.1f", c.st, s.Sub.Breakout, c.want)
		}
	}
}

// Rounding rather than truncating matters at the alert threshold of exactly 60:
// truncation biases every fractional score down by up to a point.
func TestScoreCandidate_RoundsRatherThanTruncates(t *testing.T) {
	f := scoreFeatures()
	f.VolAccel = ptr(1.13) // 1.0-1.5 band -> (0.13/0.5)*10 = 2.6
	f.RVol20 = ptr(2.0)    // 1.5-3.0 band -> (0.5/1.5)*12 = 4.0
	f.BreakoutState = ptrState(BreakoutNone)
	f.AboveVWAP = ptr(false)
	f.PctOf52wHigh = ptr(0.5)
	in := ScoreInput{CatalystTier: CatalystNone, FloatSharesEst: ptr(500e6)}

	s, _ := ScoreCandidate(f, in, passingGate(BucketMarket))
	// Raw ~6.6 -> rounds to 7, truncation would give 6.
	if s.Total != int(math.Round(s.Raw)) {
		t.Errorf("Total = %d, want %d (round of raw %.4f)", s.Total, int(math.Round(s.Raw)), s.Raw)
	}
}

func hasStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
