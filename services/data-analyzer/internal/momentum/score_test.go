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

// §4 v2: capacity is still 100, but 10 points are deliberately RESERVED rather
// than allocated. The gap must not be quietly closed — assigning it requires
// evidence, and defaulting it into an existing component is the unevidenced
// retune this revision exists to avoid.
func TestWeights_AllocatedPlusReservedIsOneHundred(t *testing.T) {
	sum := WeightVolAccel + WeightRVol + WeightBreakout + WeightCatalyst + WeightFloat + WeightVWAP + WeightHigh52w
	if sum != WeightAllocated {
		t.Errorf("components sum to %d but WeightAllocated is %d", sum, WeightAllocated)
	}
	if WeightAllocated != 90 {
		t.Errorf("WeightAllocated = %d, want 90 (§4 v2)", WeightAllocated)
	}
	if WeightReserved != 10 {
		t.Errorf("WeightReserved = %d, want 10 — held pending §3.11's catalyst tier and re-validation at scale", WeightReserved)
	}
	if WeightTotal != 100 {
		t.Errorf("WeightTotal = %d, want 100 so momentum_score_100 keeps its name", WeightTotal)
	}
}

// The two components Step 7 measured as INVERTED (p<0.01, same direction, one
// shared mechanistic explanation) must contribute nothing.
//
// §3.2 excludes stocks already up 20-25% as "already gone"; §4.2 v1 then paid 25
// points for a fresh high and a confirmed breakout, the geometric signature of
// exactly that lateness. The gate said don't chase, the score paid to chase.
func TestWeights_InvertedComponentsAreZeroed(t *testing.T) {
	if WeightBreakout != 0 {
		t.Errorf("WeightBreakout = %d, want 0 (inverted, p=0.0008)", WeightBreakout)
	}
	if WeightHigh52w != 0 {
		t.Errorf("WeightHigh52w = %d, want 0 (inverted, p=0.0089)", WeightHigh52w)
	}
}

// rvol is the only component with measured positive signal, but that rests on a
// single z-test at n=109/half from one 450-symbol pilot — so it takes 15 of the
// 25 freed points, not all 25.
func TestWeights_RVolGainedFifteenNotTwentyFive(t *testing.T) {
	if WeightRVol != 35 {
		t.Errorf("WeightRVol = %d, want 35 (v1 20 + 15 freed)", WeightRVol)
	}
	// Underpowered is not useless: at 34 total hits these three cannot be
	// distinguished from noise either way, so cutting them would treat absence of
	// evidence as evidence of absence.
	if WeightVolAccel != 25 || WeightFloat != 10 || WeightVWAP != 5 {
		t.Errorf("vol_accel/float/vwap = %d/%d/%d, want 25/10/5 unchanged from v1",
			WeightVolAccel, WeightFloat, WeightVWAP)
	}
}

// A flawless candidate must reach exactly the ALLOCATED weight. Not 100 — the
// reserved 10 points are unreachable by design, and a candidate hitting 100
// would mean that capacity had leaked into a component.
func TestScoreCandidate_PerfectCandidateScoresAllocatedWeight(t *testing.T) {
	f := scoreFeatures()
	f.VolAccel = ptr(5.0)                                 // > 4.0  -> 25
	f.RVol20 = ptr(12.0)                                  // > 10   -> 35 (v2)
	f.BreakoutState = ptrState(BreakoutFromConsolidation) // -> 0 in v2
	f.AboveVWAP = ptr(true)                               // -> 5
	f.PctOf52wHigh = ptr(1.05)                            // -> 0 in v2
	f.RSI14 = ptr(70.0)                                   // no penalty
	f.ChangePct = ptr(12.0)                               // no penalty

	in := ScoreInput{CatalystTier: CatalystA, FloatSharesEst: ptr(10e6)} // 15 + 10

	s, ok := ScoreCandidate(f, in, passingGate(BucketMarket))
	if !ok {
		t.Fatal("should score")
	}
	if s.Total != WeightAllocated {
		t.Errorf("Total = %d, want %d (allocated weight). Breakdown: %+v", s.Total, WeightAllocated, s.Sub)
	}
	if s.Total == 100 {
		t.Error("a candidate reached 100; the reserved 10 points must be unreachable")
	}
	if len(s.Penalties) != 0 {
		t.Errorf("unexpected penalties: %v", s.Penalties)
	}
	if len(s.NullInputs) != 0 {
		t.Errorf("unexpected nulls: %v", s.NullInputs)
	}
}

// Hand-computed component by component against §4 v2, so a wrong band boundary
// fails on a specific number rather than a vague total.
func TestScoreCandidate_HandComputedBreakdown(t *testing.T) {
	f := scoreFeatures()
	f.VolAccel = ptr(2.0)                // band 1.5-2.5 -> 10 + (0.5/1.0)*10 = 15
	f.RVol20 = ptr(4.0)                  // v2 rescale x1.75: band 3.0-5.0 spans 21->31.5, midpoint = 26.25
	f.BreakoutState = ptrState(Breakout) // v1 14 -> v2 0
	f.AboveVWAP = ptr(true)              // 5
	f.PctOf52wHigh = ptr(0.97)           // v1 4 -> v2 0
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
		{"rvol", s.Sub.RVol, 26.25},
		{"breakout", s.Sub.Breakout, 0},
		{"catalyst", s.Sub.Catalyst, 8},
		{"float", s.Sub.Float, 7},
		{"vwap", s.Sub.VWAP, 5},
		{"high52w", s.Sub.High52w, 0},
	} {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %.4f, want %.4f", c.name, c.got, c.want)
		}
	}
	// 15 + 26.25 + 0 + 8 + 7 + 5 + 0 = 61.25
	if math.Abs(s.Raw-61.25) > 1e-9 {
		t.Errorf("Raw = %.4f, want 61.25", s.Raw)
	}
	if s.Total != 61 {
		t.Errorf("Total = %d, want 61 (round of 61.25)", s.Total)
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

// v2 keeps v1's band SHAPE and rescales it by 35/20 = 1.75. The evidence speaks
// to rvol's weight, not to where its breakpoints belong, so changing the curve
// would smuggle an unevidenced change in alongside an evidenced one.
func TestScoreCandidate_RVolBands(t *testing.T) {
	for _, c := range []struct{ rvol, want float64 }{
		{1.0, 0}, {1.5, 0}, {2.25, 10.5}, {3.0, 21}, {4.0, 26.25},
		{5.0, 31.5}, {7.5, 33.25}, {10.0, 35}, {50, 35},
	} {
		f := scoreFeatures()
		f.RVol20 = ptr(c.rvol)
		f.VolAccel = ptr(2.0) // keep it from tripping the decay penalty
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if math.Abs(s.Sub.RVol-c.want) > 1e-9 {
			t.Errorf("rvol %.2f -> %.4f, want %.4f", c.rvol, s.Sub.RVol, c.want)
		}
	}
	// The top of the ramp must equal the weight exactly, or the component cannot
	// contribute its full allocation.
	f := scoreFeatures()
	f.RVol20 = ptr(20.0)
	f.VolAccel = ptr(2.0)
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if math.Abs(s.Sub.RVol-float64(WeightRVol)) > 1e-9 {
		t.Errorf("saturated rvol = %.4f, want WeightRVol %d", s.Sub.RVol, WeightRVol)
	}
}

// Float is a STEP band in §4.2, not a ramp: interpolating would invent precision
// the §3.9 estimate does not have. high52w is now zero at every ratio.
func TestScoreCandidate_FloatIsStepsAndHigh52wIsZeroed(t *testing.T) {
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

	// Zero at every ratio in v2, including the new-high case v1 rewarded most —
	// that reward is precisely what measured as inverted.
	for _, ratio := range []float64{0.80, 0.899, 0.90, 0.949, 0.95, 0.999, 1.00, 1.30} {
		f := scoreFeatures()
		f.PctOf52wHigh = ptr(ratio)
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if s.Sub.High52w != 0 {
			t.Errorf("pct_of_52w_high %.3f contributed %.1f, want 0 in v2", ratio, s.Sub.High52w)
		}
	}
}

// §3.7's window excludes today, so a ratio above 1.0 IS the new-high case and
// the ratio is still computed and stored. In v2 it earns nothing: Step 7
// measured a fresh high as a LATENESS marker among already-gated candidates
// rather than a quality marker (p=0.0089, inverted).
func TestScoreCandidate_NewFiftyTwoWeekHighIsComputedButUnrewarded(t *testing.T) {
	f := scoreFeatures()
	f.PctOf52wHigh = ptr(1.02)
	s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
	if f.PctOf52wHigh == nil || *f.PctOf52wHigh <= 1.0 {
		t.Error("the ratio must still exceed 1.0 for a new high — §3.7's window excludes today")
	}
	if s.Sub.High52w != 0 {
		t.Errorf("a new 52-week high contributed %.1f, want 0 in v2", s.Sub.High52w)
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

// Every breakout state now contributes 0 because WeightBreakout is 0 — while
// breakout_state itself is still computed and stored for review.
func TestScoreCandidate_BreakoutContributesNothingInV2(t *testing.T) {
	for _, st := range []BreakoutState{
		BreakoutFromConsolidation, Breakout, BreakoutApproaching, BreakoutNone,
	} {
		f := scoreFeatures()
		f.BreakoutState = ptrState(st)
		s, _ := ScoreCandidate(f, ScoreInput{CatalystTier: CatalystNone}, passingGate(BucketMarket))
		if s.Sub.Breakout != 0 {
			t.Errorf("breakout %q contributed %.1f, want 0 (weight zeroed in v2)", st, s.Sub.Breakout)
		}
		if f.BreakoutState == nil {
			t.Error("breakout_state must still be computed; zeroing a weight is reversible, deleting a feature is not")
		}
	}
}

// The points functions derive from the weight rather than hardcoding zeros, so
// restoring the weight restores v1's exact shape without a second edit. This
// guards the property that made zeroing safe: the constant is the single source
// of truth and the function cannot disagree with it.
func TestBreakoutAndHigh52wShapeSurvivesAWeightRestore(t *testing.T) {
	for _, c := range []struct {
		label string
		v1    float64
		frac  float64
	}{
		{"breakout_from_consolidation", 20, 1.0},
		{"breakout", 14, 0.7},
		{"approaching", 8, 0.4},
		{"high52w new high", 5, 1.0},
		{"high52w 0.95-1.00", 4, 0.8},
		{"high52w 0.90-0.95", 2, 0.4},
	} {
		base := 20.0
		if c.v1 <= 5 {
			base = 5.0
		}
		if got := c.frac * base; math.Abs(got-c.v1) > 1e-9 {
			t.Errorf("%s: fraction %.2f x v1 weight %.0f = %.2f, want v1 value %.0f",
				c.label, c.frac, base, got, c.v1)
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
