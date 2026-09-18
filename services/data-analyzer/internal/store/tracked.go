package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// §5 exit tracking on prior alerts.
//
// Buy and sell are not symmetric: a sell signal requires knowing what was
// previously alerted, which is why this table exists rather than the sell side
// being a mirror of the gates.

// TrackedRow is one momentum_tracked row.
type TrackedRow struct {
	Symbol         string
	AlertedTS      time.Time
	Bucket         string
	Status         string
	ReferencePrice float64
	ScoreAtAlert   int

	// Alert-time snapshots. Nil when the feature was absent at alert time, which
	// makes the corresponding exit condition unevaluable rather than passing.
	Resistance20AtAlert *float64
	ATR14AtAlert        *float64

	LastEvaluatedTS   *time.Time
	HighestCloseSince float64
	MaxGainPct        float64
	LowRVolStreak     int
	SessionsElapsed   int

	ExitReason *string
	ExitTS     *time.Time
	ExitPrice  *float64
	ExitPct    *float64
}

const openTrackedSQL = `
INSERT INTO momentum_tracked
    (symbol, alerted_ts, bucket, status, reference_price, score_at_alert,
     resistance_20_at_alert, atr_14_at_alert,
     highest_close_since, max_gain_pct, low_rvol_streak, sessions_elapsed,
     created_at, updated_at)
VALUES ($1, $2, $3, 'active', $4, $5, $6, $7, $4, 0, 0, 0, now(), now())
ON CONFLICT (symbol, alerted_ts) DO NOTHING`

// OpenTracked records a buy alert, idempotently.
//
// DO NOTHING rather than an upsert: re-running a scan for the same session must
// not reset an already-tracked position's carried state. Resetting
// sessions_elapsed or low_rvol_streak would silently postpone every exit
// condition that depends on elapsed time, so a re-run would keep a position
// alive indefinitely.
//
// highest_close_since seeds to the reference price so max_gain_pct starts at 0
// rather than at a spurious gain measured from zero.
func OpenTracked(ctx context.Context, pool *pgxpool.Pool, r TrackedRow) (bool, error) {
	ct, err := pool.Exec(ctx, openTrackedSQL,
		r.Symbol, r.AlertedTS, r.Bucket, r.ReferencePrice, r.ScoreAtAlert,
		r.Resistance20AtAlert, r.ATR14AtAlert)
	if err != nil {
		return false, fmt.Errorf("open tracked %s: %w", r.Symbol, err)
	}
	return ct.RowsAffected() > 0, nil
}

// ActiveTracked loads every position still being evaluated.
func ActiveTracked(ctx context.Context, pool *pgxpool.Pool) ([]TrackedRow, error) {
	const q = `
SELECT symbol, alerted_ts, bucket, status, reference_price, score_at_alert,
       resistance_20_at_alert, atr_14_at_alert, last_evaluated_ts,
       COALESCE(highest_close_since, reference_price), COALESCE(max_gain_pct, 0),
       COALESCE(low_rvol_streak, 0), COALESCE(sessions_elapsed, 0)
FROM momentum_tracked
WHERE status = 'active'
ORDER BY symbol`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("active tracked: %w", err)
	}
	defer rows.Close()

	var out []TrackedRow
	for rows.Next() {
		var t TrackedRow
		if err := rows.Scan(&t.Symbol, &t.AlertedTS, &t.Bucket, &t.Status,
			&t.ReferencePrice, &t.ScoreAtAlert, &t.Resistance20AtAlert, &t.ATR14AtAlert,
			&t.LastEvaluatedTS, &t.HighestCloseSince, &t.MaxGainPct,
			&t.LowRVolStreak, &t.SessionsElapsed); err != nil {
			return nil, fmt.Errorf("scan tracked: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ToTrackedState converts a stored row into the evaluator's input.
func (t TrackedRow) ToTrackedState() momentum.TrackedState {
	return momentum.TrackedState{
		Symbol:              t.Symbol,
		Bucket:              momentum.Bucket(t.Bucket),
		ReferencePrice:      t.ReferencePrice,
		Resistance20AtAlert: t.Resistance20AtAlert,
		ATR14AtAlert:        t.ATR14AtAlert,
		HighestCloseSince:   t.HighestCloseSince,
		LowRVolStreak:       t.LowRVolStreak,
		SessionsElapsed:     t.SessionsElapsed,
	}
}

const advanceTrackedSQL = `
UPDATE momentum_tracked SET
    last_evaluated_ts   = $3,
    highest_close_since = $4,
    max_gain_pct        = $5,
    low_rvol_streak     = $6,
    sessions_elapsed    = $7,
    updated_at          = now()
WHERE symbol = $1 AND alerted_ts = $2 AND status = 'active'`

// AdvanceTracked persists a session's carried state without closing the row.
func AdvanceTracked(ctx context.Context, pool *pgxpool.Pool, symbol string, alertedTS, evaluatedTS time.Time, d momentum.ExitDecision) error {
	_, err := pool.Exec(ctx, advanceTrackedSQL, symbol, alertedTS, evaluatedTS,
		d.HighestCloseSince, d.MaxGainPct, d.LowRVolStreak, d.SessionsElapsed)
	if err != nil {
		return fmt.Errorf("advance tracked %s: %w", symbol, err)
	}
	return nil
}

const closeTrackedSQL = `
UPDATE momentum_tracked SET
    status              = 'closed',
    exit_reason         = $3,
    exit_ts             = $4,
    exit_price          = $5,
    exit_pct            = $6,
    last_evaluated_ts   = $4,
    highest_close_since = $7,
    max_gain_pct        = $8,
    low_rvol_streak     = $9,
    sessions_elapsed    = $10,
    updated_at          = now()
WHERE symbol = $1 AND alerted_ts = $2 AND status = 'active'`

// CloseTracked closes a position and records the realized outcome.
//
// §5 calls max_gain_pct and exit_pct "free labeling data ... a live measure of
// whether the score is worth anything". Both are written here rather than
// derived later, because the peak is path-dependent: reconstructing
// max_gain_pct after the fact needs every intervening bar and is wrong the
// moment a bar is revised.
//
// The WHERE clause requires status='active', so a double-close is a no-op rather
// than overwriting the first exit's reason with a later one.
func CloseTracked(ctx context.Context, pool *pgxpool.Pool, symbol string, alertedTS, exitTS time.Time, exitPrice float64, d momentum.ExitDecision) error {
	_, err := pool.Exec(ctx, closeTrackedSQL, symbol, alertedTS,
		d.Reason, exitTS, exitPrice, d.ExitPct,
		d.HighestCloseSince, d.MaxGainPct, d.LowRVolStreak, d.SessionsElapsed)
	if err != nil {
		return fmt.Errorf("close tracked %s: %w", symbol, err)
	}
	return nil
}

// ExitOutcomeStats summarises closed positions by exit reason.
//
// This is the dataset §5's "starting default, not a validated strategy" needs in
// order to stop being a default. Reported per reason so the five conditions can
// be compared: a reason whose median max_gain_pct is high but whose exit_pct is
// low is firing too late, and one whose max_gain is low is firing on positions
// that never worked.
type ExitOutcomeStats struct {
	Reason        string
	N             int
	MedianMaxGain float64
	MedianExit    float64
}

func ExitOutcomes(ctx context.Context, pool *pgxpool.Pool) ([]ExitOutcomeStats, error) {
	const q = `
SELECT exit_reason,
       count(*),
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY max_gain_pct), 0),
       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY exit_pct), 0)
FROM momentum_tracked
WHERE status = 'closed' AND exit_reason IS NOT NULL
GROUP BY exit_reason
ORDER BY count(*) DESC, exit_reason`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("exit outcomes: %w", err)
	}
	defer rows.Close()
	var out []ExitOutcomeStats
	for rows.Next() {
		var s ExitOutcomeStats
		if err := rows.Scan(&s.Reason, &s.N, &s.MedianMaxGain, &s.MedianExit); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
