package store

import (
	"context"
	"fmt"
	"time"
)

// Reads for the Data Source status page (docs/MOMENTUM_SCANNER_API.md,
// Addendum: Data Source). api_rate_budget and momentum_chain_runs were written
// by the limiter and the daily runner; these are their first read-back paths.

// ProviderBudget is one api_rate_budget row as the status page reports it.
type ProviderBudget struct {
	Key          string
	RefillPerSec float64
	Burst        float64
	// DailyLimit is nil when the provider has no daily cap (Finnhub is limited
	// per second only). Never replaced by a computed ceiling.
	DailyLimit *float64
	// DailyUsed is TODAY's count: the stored value when the stored window is
	// the current one, else 0. The limiter only rolls the window on the next
	// request, so after a quiet day the row still holds yesterday's count.
	DailyUsed        float64
	StoredDailyUsed  float64
	DailyWindowStart *time.Time
	WindowIsCurrent  bool
	DailyResetTZ     string
	UpdatedAt        time.Time
}

// ProviderBudgets returns the rows for keys, in no particular order; a key with
// no row is simply absent. The current-window test is the limiter's own
// expression, so both sides agree on when "today" starts for each budget.
func ProviderBudgets(ctx context.Context, q Querier, keys []string) ([]ProviderBudget, error) {
	rows, err := q.Query(ctx, `
SELECT budget_key, refill_per_sec, burst, daily_limit, daily_used, daily_window_start,
       COALESCE(daily_window_start = (now() AT TIME ZONE daily_reset_tz)::date, false),
       daily_reset_tz, updated_at
FROM api_rate_budget
WHERE budget_key = ANY($1)`, keys)
	if err != nil {
		return nil, fmt.Errorf("provider budgets: %w", err)
	}
	defer rows.Close()
	var out []ProviderBudget
	for rows.Next() {
		var b ProviderBudget
		if err := rows.Scan(&b.Key, &b.RefillPerSec, &b.Burst, &b.DailyLimit, &b.StoredDailyUsed,
			&b.DailyWindowStart, &b.WindowIsCurrent, &b.DailyResetTZ, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider budget: %w", err)
		}
		if b.WindowIsCurrent {
			b.DailyUsed = b.StoredDailyUsed
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ChainRunsBetween returns momentum_chain_runs rows with from <= session <= to,
// plus the earliest session ever recorded (ok=false when the table is empty):
// sessions before it predate the markers and are "not recorded", not "not run".
func ChainRunsBetween(ctx context.Context, q Querier, from, to time.Time) (runs map[string]ChainRun, first time.Time, ok bool, err error) {
	var firstPtr *time.Time
	if err := q.QueryRow(ctx, `SELECT min(session) FROM momentum_chain_runs`).Scan(&firstPtr); err != nil {
		return nil, time.Time{}, false, fmt.Errorf("first chain run: %w", err)
	}
	rows, err := q.Query(ctx, `
SELECT session, attempts, scanner_completed_at, tracker_completed_at, gave_up_at, last_error
FROM momentum_chain_runs
WHERE session BETWEEN $1 AND $2`, from, to)
	if err != nil {
		return nil, time.Time{}, false, fmt.Errorf("chain runs: %w", err)
	}
	defer rows.Close()
	runs = map[string]ChainRun{}
	for rows.Next() {
		var r ChainRun
		if err := rows.Scan(&r.Session, &r.Attempts, &r.ScannerCompletedAt, &r.TrackerCompletedAt, &r.GaveUpAt, &r.LastError); err != nil {
			return nil, time.Time{}, false, fmt.Errorf("scan chain run: %w", err)
		}
		runs[r.Session.Format(time.DateOnly)] = r
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, false, err
	}
	if firstPtr == nil {
		return runs, time.Time{}, false, nil
	}
	return runs, firstPtr.UTC(), true, nil
}

// LastCleanSession is the newest session that finished first time with no
// error: attempts = 1, both markers set, no give-up, no error.
func LastCleanSession(ctx context.Context, q Querier) (*time.Time, error) {
	var d *time.Time
	err := q.QueryRow(ctx, `
SELECT max(session) FROM momentum_chain_runs
WHERE attempts = 1 AND scanner_completed_at IS NOT NULL AND tracker_completed_at IS NOT NULL
  AND gave_up_at IS NULL AND last_error IS NULL`).Scan(&d)
	if err != nil {
		return nil, fmt.Errorf("last clean session: %w", err)
	}
	return d, nil
}

// SessionCoverage is the share of today's scannable universe with a daily bar
// for each session — momentum-daily's exact definition, computed NOW. It can
// differ from the coverage that gated that session's run if bars were
// corrected or backfilled since; callers must label it "coverage now".
func SessionCoverage(ctx context.Context, q Querier, sessions []time.Time, source string) (map[string]float64, error) {
	var total float64
	if err := q.QueryRow(ctx, `
SELECT count(*) FROM universe_symbols WHERE is_eligible AND data_unavailable_reason IS NULL`).Scan(&total); err != nil {
		return nil, fmt.Errorf("session coverage: scannable universe: %w", err)
	}
	out := map[string]float64{}
	if total == 0 {
		return out, nil
	}
	// One grouped pass over all sessions (a correlated count per session took
	// ~3.6s against the live table; this takes ~0.5s). Bars are stored at
	// midnight UTC, so the date is taken in UTC, never the connection's zone.
	rows, err := q.Query(ctx, `
SELECT (o.ts AT TIME ZONE 'UTC')::date, count(DISTINCT o.symbol)
FROM equity_ohlcv o
JOIN universe_symbols u ON u.symbol = o.symbol AND u.is_eligible AND u.data_unavailable_reason IS NULL
WHERE o.interval = '1Day' AND o.source = $2
  AND o.ts = ANY (ARRAY(SELECT d::timestamp AT TIME ZONE 'UTC' FROM unnest($1::date[]) AS d))
GROUP BY 1`, sessions, source)
	if err != nil {
		return nil, fmt.Errorf("session coverage: %w", err)
	}
	defer rows.Close()
	for _, d := range sessions {
		out[d.Format(time.DateOnly)] = 0
	}
	for rows.Next() {
		var d time.Time
		var landed float64
		if err := rows.Scan(&d, &landed); err != nil {
			return nil, fmt.Errorf("scan session coverage: %w", err)
		}
		out[d.Format(time.DateOnly)] = landed / total * 100
	}
	return out, rows.Err()
}
