package momentum

import (
	"math"
	"testing"
)

func trackedAt(ref float64) TrackedState {
	return TrackedState{
		Symbol:              "AAA",
		Bucket:              BucketMarket,
		ReferencePrice:      ref,
		Resistance20AtAlert: ptr(ref * 0.95),
		ATR14AtAlert:        ptr(ref * 0.05),
		HighestCloseSince:   ref,
	}
}

// exitFeatures returns features that trip NO exit condition, so each test can
// break exactly one thing. Starting from a clean baseline matters here more than
// usual: five conditions evaluated in priority order means a sloppy fixture makes
// tests pass for the wrong reason.
func exitFeatures(close float64) *Features {
	return &Features{
		Close:  ptr(close),
		VWAP20: ptr(close * 0.9), // above VWAP
		RVol20: ptr(3.0),         // well above the stall threshold
	}
}

func hasReason(d ExitDecision, want string) bool {
	for _, r := range d.AllMatched {
		if r == want {
			return true
		}
	}
	return false
}

// ─── Each condition in isolation ──────────────────────────────────────────────

func TestEvaluateExit_BreakoutFailedUsesTheALERTTimeResistance(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)  // resistance_20_at_alert = 95
	f := exitFeatures(94) // fell back under the level it broke out over
	f.VWAP20 = ptr(90.0)  // keep VWAP from also firing

	d := EvaluateExit(st, f, cfg)
	if !d.Exit || d.Reason != ExitBreakoutFailed {
		t.Fatalf("Exit=%v Reason=%q, want breakout_failed (matched %v)", d.Exit, d.Reason, d.AllMatched)
	}

	// The level is an ALERT-TIME SNAPSHOT and must not be recomputed. If a
	// rolling resistance were used the rule would silently redefine itself every
	// session, and "fell back under what it broke out over" would stop meaning
	// that. Raising the snapshot must change the verdict; nothing about today's
	// bars should.
	st2 := trackedAt(100)
	st2.Resistance20AtAlert = ptr(80.0)
	if d2 := EvaluateExit(st2, f, cfg); hasReason(d2, ExitBreakoutFailed) {
		t.Error("close of 94 above a snapshot of 80 must not trigger breakout_failed")
	}
}

func TestEvaluateExit_LostVWAP(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	f := exitFeatures(96) // above the 95 resistance, so breakout_failed stays quiet
	f.VWAP20 = ptr(98.0)  // but under VWAP

	d := EvaluateExit(st, f, cfg)
	if !d.Exit || d.Reason != ExitLostVWAP {
		t.Fatalf("Reason=%q, want lost_vwap (matched %v)", d.Reason, d.AllMatched)
	}
}

// momentum_stalled needs rvol under the threshold for N CONSECUTIVE sessions, so
// it must not fire on the first low reading.
func TestEvaluateExit_MomentumStalledRequiresConsecutiveSessions(t *testing.T) {
	cfg := DefaultExitConfig() // 1.5 for 3 sessions
	st := trackedAt(100)

	for session := 1; session <= 3; session++ {
		f := exitFeatures(101) // nothing else trips
		f.RVol20 = ptr(1.0)    // below the threshold
		d := EvaluateExit(st, f, cfg)

		if d.LowRVolStreak != session {
			t.Errorf("session %d: streak = %d, want %d", session, d.LowRVolStreak, session)
		}
		wantExit := session >= cfg.StalledSessions
		if hasReason(d, ExitMomentumStalled) != wantExit {
			t.Errorf("session %d: stalled=%v want %v (streak %d)",
				session, hasReason(d, ExitMomentumStalled), wantExit, d.LowRVolStreak)
		}
		// carry state forward, as the job does
		st.LowRVolStreak = d.LowRVolStreak
		st.SessionsElapsed = d.SessionsElapsed
		st.HighestCloseSince = d.HighestCloseSince
	}
}

func TestEvaluateExit_StreakResetsOnARecoverySession(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	st.LowRVolStreak = 2 // one session away from stalling

	f := exitFeatures(101)
	f.RVol20 = ptr(4.0) // volume came back
	d := EvaluateExit(st, f, cfg)

	if d.LowRVolStreak != 0 {
		t.Errorf("streak = %d, want 0 — a recovery session must reset it", d.LowRVolStreak)
	}
	if hasReason(d, ExitMomentumStalled) {
		t.Error("stalled must not fire after a recovery session")
	}
}

// A NULL rvol breaks the streak rather than extending it: "we could not measure
// volume" is not evidence volume was low, and treating it as such would exit
// positions on a data gap.
func TestEvaluateExit_NullRVolBreaksTheStreakRatherThanExtendingIt(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	st.LowRVolStreak = 2

	f := exitFeatures(101)
	f.RVol20 = nil
	d := EvaluateExit(st, f, cfg)

	if d.LowRVolStreak != 0 {
		t.Errorf("streak = %d, want 0 — a missing rvol must not advance toward an exit", d.LowRVolStreak)
	}
	if hasReason(d, ExitMomentumStalled) {
		t.Error("a data gap must not trigger momentum_stalled")
	}
}

func TestEvaluateExit_StopATRUsesTheAlertTimeATR(t *testing.T) {
	cfg := DefaultExitConfig()         // 2x ATR
	st := trackedAt(100)               // atr_at_alert = 5, so the stop is 100 - 10 = 90
	st.Resistance20AtAlert = ptr(50.0) // keep breakout_failed quiet

	f := exitFeatures(89)
	f.VWAP20 = ptr(80.0) // keep lost_vwap quiet
	d := EvaluateExit(st, f, cfg)
	if !hasReason(d, ExitStopATR) {
		t.Fatalf("close 89 below the 90 stop must fire stop_atr (matched %v)", d.AllMatched)
	}

	// Exactly at the stop is not below it.
	f2 := exitFeatures(90)
	f2.VWAP20 = ptr(80.0)
	if d2 := EvaluateExit(st, f2, cfg); hasReason(d2, ExitStopATR) {
		t.Error("a close exactly at the stop must not fire it")
	}
}

// §5: "N sessions elapsed with no exit condition met AND NO NEW HIGH since
// alert." A position making new highs is working — timing it out would close the
// exact case the strategy exists to find.
func TestEvaluateExit_TimeoutRequiresNoNewHigh(t *testing.T) {
	cfg := DefaultExitConfig() // 20 sessions
	base := trackedAt(100)
	base.Resistance20AtAlert = ptr(50.0)
	base.ATR14AtAlert = ptr(50.0) // stop far away

	// 20 sessions, price flat at the reference: no new high -> timeout.
	flat := base
	flat.SessionsElapsed = 19
	flat.HighestCloseSince = 100
	f := exitFeatures(100)
	f.VWAP20 = ptr(90.0)
	if d := EvaluateExit(flat, f, cfg); !hasReason(d, ExitTimeout) {
		t.Errorf("20 flat sessions must time out (matched %v)", d.AllMatched)
	}

	// Same 20 sessions, but it has made a new high -> must NOT time out.
	winning := base
	winning.SessionsElapsed = 19
	winning.HighestCloseSince = 130
	f2 := exitFeatures(125)
	f2.VWAP20 = ptr(90.0)
	if d := EvaluateExit(winning, f2, cfg); hasReason(d, ExitTimeout) {
		t.Error("a position making new highs must not be timed out")
	}
}

// ─── Priority and the orderability of the rules ───────────────────────────────

// §5 emits on the first match in table order. This pins the order so a refactor
// cannot quietly reshuffle which reason gets reported.
func TestEvaluateExit_FirstMatchWinsInSectionFiveOrder(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100) // resistance 95, stop at 90
	st.LowRVolStreak = 2

	// A collapse that trips breakout_failed, lost_vwap, momentum_stalled AND
	// stop_atr simultaneously.
	f := &Features{Close: ptr(85.0), VWAP20: ptr(99.0), RVol20: ptr(0.5)}
	d := EvaluateExit(st, f, cfg)

	if d.Reason != ExitBreakoutFailed {
		t.Errorf("Reason = %q, want breakout_failed as the highest-priority match", d.Reason)
	}
	for _, want := range []string{ExitBreakoutFailed, ExitLostVWAP, ExitMomentumStalled, ExitStopATR} {
		if !hasReason(d, want) {
			t.Errorf("AllMatched missing %s: %v", want, d.AllMatched)
		}
	}
	// AllMatched must follow §5's table order, since a later evaluation of the
	// ordering depends on the recorded sequence being meaningful.
	if d.AllMatched[0] != ExitBreakoutFailed || d.AllMatched[1] != ExitLostVWAP {
		t.Errorf("AllMatched not in priority order: %v", d.AllMatched)
	}
}

// The reason §5's ordering can be evaluated later at all. Recording only the
// first match would make "should the stop have fired before breakout_failed?"
// permanently unanswerable — the same reasoning as §3.2 collecting every gate
// failure instead of short-circuiting.
func TestEvaluateExit_RecordsEveryMatchNotJustTheReportedOne(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	f := &Features{Close: ptr(80.0), VWAP20: ptr(99.0), RVol20: ptr(3.0)}

	d := EvaluateExit(st, f, cfg)
	if len(d.AllMatched) < 2 {
		t.Fatalf("only %v recorded; the ordering cannot be evaluated from one reason", d.AllMatched)
	}
	if d.MatchedString() == d.Reason {
		t.Error("MatchedString should carry every match, not just the reported reason")
	}
}

// ─── Unevaluable conditions ───────────────────────────────────────────────────

// A missing alert-time input makes its condition UNEVALUABLE, not satisfied. A
// nil ATR means the stop was never armed — recording that as "did not match"
// would make a row that ran without a stop indistinguishable from one that
// stayed above it.
func TestEvaluateExit_MissingAlertLevelsAreUnevaluableNotPassing(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	st.Resistance20AtAlert = nil
	st.ATR14AtAlert = nil

	f := exitFeatures(10) // a collapse that would trip both if they were armed
	f.VWAP20 = ptr(5.0)
	d := EvaluateExit(st, f, cfg)

	if hasReason(d, ExitBreakoutFailed) || hasReason(d, ExitStopATR) {
		t.Errorf("conditions with no alert-time level must not fire: %v", d.AllMatched)
	}
	un := map[string]bool{}
	for _, u := range d.Unevaluable {
		un[u] = true
	}
	if !un[ExitBreakoutFailed] || !un[ExitStopATR] {
		t.Errorf("Unevaluable = %v, want both breakout_failed and stop_atr", d.Unevaluable)
	}
}

func TestEvaluateExit_NullVWAPMakesLostVWAPUnevaluable(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	st.Resistance20AtAlert = ptr(50.0)
	st.ATR14AtAlert = ptr(50.0)

	f := exitFeatures(96)
	f.VWAP20 = nil
	d := EvaluateExit(st, f, cfg)

	if hasReason(d, ExitLostVWAP) {
		t.Error("a null VWAP must not fire lost_vwap")
	}
	found := false
	for _, u := range d.Unevaluable {
		if u == ExitLostVWAP {
			found = true
		}
	}
	if !found {
		t.Errorf("Unevaluable = %v, want lost_vwap", d.Unevaluable)
	}
}

// ─── Realized outcome recording (§5's "free labeling data") ───────────────────

func TestEvaluateExit_TracksPeakAndRealizedMove(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	st.Resistance20AtAlert = ptr(50.0)
	st.ATR14AtAlert = ptr(50.0)

	// Session 1: runs to 140.
	f := exitFeatures(140)
	f.VWAP20 = ptr(90.0)
	d := EvaluateExit(st, f, cfg)
	if d.HighestCloseSince != 140 {
		t.Errorf("HighestCloseSince = %.1f, want 140", d.HighestCloseSince)
	}
	if math.Abs(d.MaxGainPct-40) > 1e-9 {
		t.Errorf("MaxGainPct = %.4f, want 40", d.MaxGainPct)
	}

	// Session 2: gives it back to 110 — the peak must persist, since max_gain_pct
	// is the whole point of the label.
	st.HighestCloseSince = d.HighestCloseSince
	st.SessionsElapsed = d.SessionsElapsed
	f2 := exitFeatures(110)
	f2.VWAP20 = ptr(90.0)
	d2 := EvaluateExit(st, f2, cfg)
	if d2.HighestCloseSince != 140 {
		t.Errorf("peak regressed to %.1f; max_gain_pct would understate the move", d2.HighestCloseSince)
	}
	if math.Abs(d2.MaxGainPct-40) > 1e-9 {
		t.Errorf("MaxGainPct = %.4f, want 40 (the peak, not the current level)", d2.MaxGainPct)
	}
}

func TestEvaluateExit_ExitPctIsTheRealizedMoveAtExit(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	f := exitFeatures(80) // breakout_failed and stop_atr
	f.VWAP20 = ptr(99.0)

	d := EvaluateExit(st, f, cfg)
	if !d.Exit {
		t.Fatal("expected an exit")
	}
	if math.Abs(d.ExitPct-(-20)) > 1e-9 {
		t.Errorf("ExitPct = %.4f, want -20", d.ExitPct)
	}

	// No exit means no realized move to report.
	clean := exitFeatures(105)
	clean.VWAP20 = ptr(90.0)
	st.Resistance20AtAlert = ptr(50.0)
	st.ATR14AtAlert = ptr(50.0)
	if d2 := EvaluateExit(st, clean, cfg); d2.Exit || d2.ExitPct != 0 {
		t.Errorf("a clean session must not report an exit: %+v", d2)
	}
}

// ─── Robustness ───────────────────────────────────────────────────────────────

// An unmeasurable session must not consume the timeout budget. Otherwise a
// symbol with a data outage would be timed out for the outage rather than for
// anything about its price.
func TestEvaluateExit_UnmeasurableSessionDoesNotAdvanceState(t *testing.T) {
	cfg := DefaultExitConfig()
	st := trackedAt(100)
	st.SessionsElapsed = 5

	for _, f := range []*Features{nil, {}, {Close: ptr(0.0)}, {Close: ptr(-1.0)}} {
		d := EvaluateExit(st, f, cfg)
		if d.Exit {
			t.Errorf("unmeasurable session produced an exit: %+v", d)
		}
		if d.SessionsElapsed != 5 {
			t.Errorf("SessionsElapsed advanced to %d on an unmeasurable session", d.SessionsElapsed)
		}
	}
}

func TestEvaluateExit_ZeroReferencePriceYieldsNoExit(t *testing.T) {
	st := trackedAt(0)
	if d := EvaluateExit(st, exitFeatures(50), DefaultExitConfig()); d.Exit {
		t.Error("a zero reference price cannot produce a percentage and must not exit")
	}
}

func TestDefaultExitConfig_MatchesSpec(t *testing.T) {
	c := DefaultExitConfig()
	if c.StalledRVolBelow != 1.5 || c.StalledSessions != 3 {
		t.Errorf("stall config = %.1f/%d, want 1.5/3 (§5)", c.StalledRVolBelow, c.StalledSessions)
	}
	if c.ATRMultiple != 2.0 {
		t.Errorf("ATRMultiple = %.1f, want 2.0 (§5)", c.ATRMultiple)
	}
	if c.TimeoutSessions != 20 {
		t.Errorf("TimeoutSessions = %d, want 20 (§5)", c.TimeoutSessions)
	}
}
