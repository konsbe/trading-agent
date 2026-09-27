package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Computation tracking (migration 030): computation_interest says why a symbol
// is refreshed; symbol_data_status records fetches and computations. Reasons
// are closed, never deleted, and a symbol's computed rows are never deleted
// when its last reason closes — candidate history is needed for evaluation
// (survivorship bias) and Tracked Positions follows symbols after they stop
// being candidates.

// InterestSymbol is one symbol to compute, with the asset type that decides
// which computations apply (equity/etf: technicals on 1Day bars +
// fundamentals; crypto: technicals on Binance bars).
type InterestSymbol struct {
	Symbol    string
	AssetType string
}

// desiredInterestSQL is every (symbol, asset_type, reason) that should be open
// now for the three source-driven reasons. A watchlist symbol's asset type
// comes from its universe_symbols type (funds are etf).
const desiredInterestSQL = `
SELECT symbol, 'equity' AS asset_type, 'candidate' AS reason FROM momentum_features
 WHERE ts = (SELECT max(ts) FROM momentum_features) AND gates_passed
UNION
SELECT DISTINCT w.symbol,
       CASE WHEN u.type IN ('ETP', 'ETF', 'Closed-End Fund') THEN 'etf' ELSE 'equity' END, 'watchlist'
  FROM watchlist_items w LEFT JOIN LATERAL (
       SELECT type FROM universe_symbols WHERE symbol = w.symbol LIMIT 1) u ON true
UNION
SELECT symbol, asset_type, 'followed' FROM followed_symbols`

// ReconcileInterest opens a reason for every desired (symbol, reason) without
// an open row, and closes every open candidate / watchlist / followed row that
// is no longer desired. Manual rows are left alone. One transaction.
func ReconcileInterest(ctx context.Context, db TxBeginner) (opened, closed int64, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("reconcile interest: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op after Commit
	tag, err := tx.Exec(ctx, `
UPDATE computation_interest ci SET active_until = now()
WHERE ci.active_until IS NULL AND ci.reason IN ('candidate', 'watchlist', 'followed')
  AND NOT EXISTS (SELECT 1 FROM (`+desiredInterestSQL+`) d WHERE d.symbol = ci.symbol AND d.reason = ci.reason)`)
	if err != nil {
		return 0, 0, fmt.Errorf("reconcile interest: close: %w", err)
	}
	closed = tag.RowsAffected()
	tag, err = tx.Exec(ctx, `
INSERT INTO computation_interest (symbol, asset_type, reason)
SELECT d.symbol, d.asset_type, d.reason FROM (`+desiredInterestSQL+`) d
WHERE NOT EXISTS (SELECT 1 FROM computation_interest ci
                  WHERE ci.symbol = d.symbol AND ci.reason = d.reason AND ci.active_until IS NULL)`)
	if err != nil {
		return 0, 0, fmt.Errorf("reconcile interest: open: %w", err)
	}
	opened = tag.RowsAffected()
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("reconcile interest: commit: %w", err)
	}
	return opened, closed, nil
}

// ComputationSet is every symbol with an open reason, plus followed symbols
// (so a followed symbol is computed even before the first reconcile).
func ComputationSet(ctx context.Context, q Querier) ([]InterestSymbol, error) {
	return interestRows(ctx, q, `
SELECT symbol, min(asset_type) FROM (
    SELECT symbol, asset_type FROM computation_interest WHERE active_until IS NULL
    UNION ALL
    SELECT symbol, asset_type FROM followed_symbols) s
GROUP BY symbol ORDER BY symbol`)
}

// PendingManualCompute is the manual queue's second half: Compute requests
// whose data has arrived since the request (bars; and fundamentals for
// equities and funds) and that have not been computed since.
func PendingManualCompute(ctx context.Context, q Querier) ([]InterestSymbol, error) {
	return interestRows(ctx, q, `
SELECT ci.symbol, ci.asset_type
FROM computation_interest ci
JOIN symbol_data_status s ON s.symbol = ci.symbol
WHERE ci.reason = 'manual' AND ci.active_until IS NULL
  AND s.bars_fetched_at >= ci.active_from
  AND (ci.asset_type = 'crypto' OR s.fundamentals_fetched_at >= ci.active_from)
  AND (s.computed_at IS NULL OR s.computed_at < ci.active_from)
ORDER BY ci.active_from`)
}

func interestRows(ctx context.Context, q Querier, sql string) ([]InterestSymbol, error) {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("interest symbols: %w", err)
	}
	defer rows.Close()
	var out []InterestSymbol
	for rows.Next() {
		var s InterestSymbol
		if err := rows.Scan(&s.Symbol, &s.AssetType); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// MarkComputed records a computation: computed_at on success, the error text
// otherwise (computed_at kept).
func MarkComputed(ctx context.Context, db Execer, symbol string, computeErr error) error {
	var errText any
	if computeErr != nil {
		errText = "compute: " + computeErr.Error()
	}
	_, err := db.Exec(ctx, `
INSERT INTO symbol_data_status (symbol, computed_at, last_error, updated_at)
VALUES (upper($1), CASE WHEN $2::text IS NULL THEN now() END, $2, now())
ON CONFLICT (symbol) DO UPDATE SET
    computed_at = CASE WHEN $2::text IS NULL THEN now() ELSE symbol_data_status.computed_at END,
    last_error = $2, updated_at = now()`, symbol, errText)
	if err != nil {
		return fmt.Errorf("mark computed %s: %w", symbol, err)
	}
	return nil
}

// PendingComputationPass is the latest session whose chain completed without
// a computation pass yet (momentum_chain_runs.computation_pass_at, migration
// 030). The pass has its own marker so a failing pass is retried and never
// marks a session's chain unclean.
func PendingComputationPass(ctx context.Context, q Querier) (time.Time, bool, error) {
	var session time.Time
	err := q.QueryRow(ctx, `
SELECT session FROM momentum_chain_runs
WHERE tracker_completed_at IS NOT NULL AND computation_pass_at IS NULL
ORDER BY session DESC LIMIT 1`).Scan(&session)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("pending computation pass: %w", err)
	}
	return session, true, nil
}

func MarkComputationPass(ctx context.Context, db Execer, session time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE momentum_chain_runs SET computation_pass_at = now() WHERE session = $1`, session); err != nil {
		return fmt.Errorf("mark computation pass %s: %w", session.Format(time.DateOnly), err)
	}
	return nil
}
