package momentum

import (
	"sort"
	"strings"
)

// §5 sell-side logic: exit tracking on prior alerts.
//
// §5 is explicit that these rules are "a starting default, not a validated
// strategy ... deliberately mechanical so the backtest can evaluate and replace
// them." Step 7 makes that more true rather than less: `breakout_failed` keys on
// resistance_20_at_alert, and breakout geometry is one of the two components
// measured INVERTED for predicting +100% moves.
//
// That does not make the exit rule wrong — an exit signal and an entry signal
// are different questions, and the drawdown finding (§4.1 v2) suggests breakout
// geometry does carry information about how a move resolves. It does mean the
// rules deserve the same measured-not-assumed treatment the score got.
//
// # Why every matching condition is recorded, not just the first
//
// §5 says emit on "the FIRST condition that matches", and EvaluateExit honours
// that for the alert. But it also returns every condition that was
// simultaneously true, because otherwise the ORDERING can never be evaluated: if
// only `breakout_failed` is ever recorded for a row where `stop_atr` also fired,
// no later analysis can ask whether the stop should have come first. The five
// conditions are neither validated nor known to be correctly ordered, so the
// data needed to test both is captured now.
//
// Same reasoning as §3.2's gate failures being collected rather than
// short-circuited.

// Exit reason strings. Persisted to momentum_tracked.exit_reason, so they are a
// schema: renaming one breaks every historical comparison.
const (
	ExitBreakoutFailed  = "breakout_failed"
	ExitLostVWAP        = "lost_vwap"
	ExitMomentumStalled = "momentum_stalled"
	ExitStopATR         = "stop_atr"
	ExitTimeout         = "timeout"
)

// exitPriority is §5's table order. The alert fires on the first match in this
// order; the order itself is a default, not a finding.
var exitPriority = []string{
	ExitBreakoutFailed,
	ExitLostVWAP,
	ExitMomentumStalled,
	ExitStopATR,
	ExitTimeout,
}

// ExitConfig holds §5's thresholds. Configuration rather than constants for the
// same reason as §3.2's gates: the numbers are what the pilot exists to test.
type ExitConfig struct {
	// StalledRVolBelow and StalledSessions define momentum_stalled: rvol_20
	// under the threshold for N consecutive sessions.
	StalledRVolBelow float64
	StalledSessions  int

	// ATRMultiple is the stop distance in ATR units below the reference price.
	ATRMultiple float64

	// TimeoutSessions is how long a position may sit with no exit condition met
	// and no new high.
	TimeoutSessions int
}

func DefaultExitConfig() ExitConfig {
	return ExitConfig{
		StalledRVolBelow: 1.5,
		StalledSessions:  3,
		ATRMultiple:      2.0,
		TimeoutSessions:  20,
	}
}

// TrackedState is one active momentum_tracked row's carried state.
//
// The alert-time levels are snapshots taken when the buy fired and must NOT be
// recomputed from current bars: `breakout_failed` asks whether price fell back
// under the level it broke out over, which is a fact about the alert date. A
// rolling resistance_20 would silently redefine the exit rule every session.
type TrackedState struct {
	Symbol string
	Bucket Bucket

	ReferencePrice float64

	// Resistance20AtAlert and ATR14AtAlert are nil when the feature was absent at
	// alert time. Their conditions are then UNEVALUABLE rather than passing —
	// same discipline as §3.4 forbidding a substituted rvol.
	Resistance20AtAlert *float64
	ATR14AtAlert        *float64

	// HighestCloseSince is the running peak since the alert, for timeout's "no
	// new high since alert" clause.
	HighestCloseSince float64

	// LowRVolStreak counts consecutive sessions with rvol under the threshold.
	LowRVolStreak int

	SessionsElapsed int
}

// ExitDecision is the verdict for one session.
type ExitDecision struct {
	// Exit and Reason carry §5's first-match outcome.
	Exit   bool
	Reason string

	// AllMatched lists every condition true this session, in priority order.
	// Recorded so the ORDERING can be evaluated later rather than assumed.
	AllMatched []string

	// Unevaluable lists conditions that could not be checked because an
	// alert-time input was absent. Distinguished from "did not match", because a
	// missing ATR means the stop was never armed, not that price stayed above it.
	Unevaluable []string

	// Updated state to persist back, whether or not an exit fired.
	HighestCloseSince float64
	LowRVolStreak     int
	SessionsElapsed   int
	MaxGainPct        float64

	// ExitPct is the realized move at exit, populated only when Exit is true.
	ExitPct float64
}

// MatchedString renders AllMatched for storage.
func (d ExitDecision) MatchedString() string { return strings.Join(d.AllMatched, ",") }

// EvaluateExit applies §5 to one active tracked row against today's features.
//
// Returns the first matching condition as the alertable reason, every matching
// condition for later analysis, and the state to persist. A nil Features or a
// non-positive close yields no exit and no state advance: a session we cannot
// measure must not silently consume the timeout budget.
func EvaluateExit(st TrackedState, f *Features, cfg ExitConfig) ExitDecision {
	d := ExitDecision{
		HighestCloseSince: st.HighestCloseSince,
		LowRVolStreak:     st.LowRVolStreak,
		SessionsElapsed:   st.SessionsElapsed,
	}
	if cfg.StalledSessions <= 0 {
		cfg = DefaultExitConfig()
	}
	if f == nil || f.Close == nil || *f.Close <= 0 || st.ReferencePrice <= 0 {
		return d
	}
	close := *f.Close

	// ── state advance ──
	d.SessionsElapsed = st.SessionsElapsed + 1
	if close > d.HighestCloseSince {
		d.HighestCloseSince = close
	}
	d.MaxGainPct = (d.HighestCloseSince/st.ReferencePrice - 1) * 100

	// rvol streak: consecutive sessions under the threshold. A NULL rvol breaks
	// the streak rather than extending it — "we could not measure volume" is not
	// evidence that volume was low, and treating it as such would exit positions
	// on a data gap.
	switch {
	case f.RVol20 == nil:
		d.LowRVolStreak = 0
	case *f.RVol20 < cfg.StalledRVolBelow:
		d.LowRVolStreak = st.LowRVolStreak + 1
	default:
		d.LowRVolStreak = 0
	}

	// ── conditions ──
	matched := map[string]bool{}

	if st.Resistance20AtAlert == nil {
		d.Unevaluable = append(d.Unevaluable, ExitBreakoutFailed)
	} else if close < *st.Resistance20AtAlert {
		matched[ExitBreakoutFailed] = true
	}

	if f.VWAP20 == nil {
		d.Unevaluable = append(d.Unevaluable, ExitLostVWAP)
	} else if close < *f.VWAP20 {
		matched[ExitLostVWAP] = true
	}

	if d.LowRVolStreak >= cfg.StalledSessions {
		matched[ExitMomentumStalled] = true
	}

	if st.ATR14AtAlert == nil {
		// A missing ATR means the stop was never armed. Recorded as unevaluable
		// rather than passing, so a row that ran without a stop is identifiable.
		d.Unevaluable = append(d.Unevaluable, ExitStopATR)
	} else if close < st.ReferencePrice-cfg.ATRMultiple*(*st.ATR14AtAlert) {
		matched[ExitStopATR] = true
	}

	// Timeout requires BOTH the session count and no new high since the alert.
	// §5's wording is "N sessions elapsed with no exit condition met and no new
	// high since alert" — a position making new highs is working, and timing it
	// out would close the exact case the strategy is looking for.
	if d.SessionsElapsed >= cfg.TimeoutSessions && d.HighestCloseSince <= st.ReferencePrice {
		matched[ExitTimeout] = true
	}

	for _, r := range exitPriority {
		if matched[r] {
			d.AllMatched = append(d.AllMatched, r)
		}
	}
	if len(d.AllMatched) > 0 {
		d.Exit = true
		d.Reason = d.AllMatched[0] // §5: the FIRST condition that matches
		d.ExitPct = (close/st.ReferencePrice - 1) * 100
	}
	sort.Strings(d.Unevaluable)
	return d
}
