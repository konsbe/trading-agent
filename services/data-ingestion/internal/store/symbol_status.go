package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// symbol_data_status bookkeeping (migration 030), written by the ingestion
// workers so the computed-symbols view and the manual "Compute" queue can tell
// "waiting for data" from "fetched" and say why a symbol has no statements.

// MarkFetched records a fetch attempt for kind "bars" or "fundamentals": the
// fetched-at time on success, the error text otherwise (the time is kept).
func MarkFetched(ctx context.Context, pool *pgxpool.Pool, symbol, kind string, fetchErr error) error {
	col := map[string]string{"bars": "bars_fetched_at", "fundamentals": "fundamentals_fetched_at"}[kind]
	if col == "" {
		return fmt.Errorf("mark fetched: unknown kind %q", kind)
	}
	var errText any
	if fetchErr != nil {
		errText = kind + ": " + fetchErr.Error()
	}
	_, err := pool.Exec(ctx, `
INSERT INTO symbol_data_status (symbol, `+col+`, last_error, updated_at)
VALUES (upper($1), CASE WHEN $2::text IS NULL THEN now() END, $2, now())
ON CONFLICT (symbol) DO UPDATE SET
    `+col+` = CASE WHEN $2::text IS NULL THEN now() ELSE symbol_data_status.`+col+` END,
    last_error = $2, updated_at = now()`, symbol, errText)
	if err != nil {
		return fmt.Errorf("mark %s fetched %s: %w", kind, symbol, err)
	}
	return nil
}

// Statement coverage, recorded from what the provider returned — never guessed
// from the symbol's type (US-listed common stocks can file 20-F: TTE, GFS).
const (
	StatementsAvailable    = "available"
	StatementsNoneReturned = "none_returned"
)

// MarkStatements records whether financial statements were returned.
func MarkStatements(ctx context.Context, pool *pgxpool.Pool, symbol, status, reason string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO symbol_data_status (symbol, statements_status, statements_reason, statements_checked_at, updated_at)
VALUES (upper($1), $2, NULLIF($3, ''), now(), now())
ON CONFLICT (symbol) DO UPDATE SET
    statements_status = EXCLUDED.statements_status, statements_reason = EXCLUDED.statements_reason,
    statements_checked_at = now(), updated_at = now()`, symbol, status, reason)
	if err != nil {
		return fmt.Errorf("mark statements %s: %w", symbol, err)
	}
	return nil
}

// PendingManual returns symbols with an open manual or watchlist
// computation_interest reason, of the given asset types, whose kind ("bars" |
// "fundamentals") has not been fetched since the newest such reason opened —
// the "Compute" queue. Adding to the watchlist queues the same fetch.
func PendingManual(ctx context.Context, pool *pgxpool.Pool, kind string, assetTypes []string) ([]string, error) {
	col := map[string]string{"bars": "bars_fetched_at", "fundamentals": "fundamentals_fetched_at"}[kind]
	if col == "" {
		return nil, fmt.Errorf("pending manual: unknown kind %q", kind)
	}
	rows, err := pool.Query(ctx, `
SELECT q.symbol
FROM (SELECT symbol, asset_type, max(active_from) AS since
      FROM computation_interest
      WHERE reason IN ('manual', 'watchlist') AND active_until IS NULL
      GROUP BY symbol, asset_type) q
LEFT JOIN symbol_data_status s ON s.symbol = q.symbol
WHERE q.asset_type = ANY($1)
  AND (s.`+col+` IS NULL OR s.`+col+` < q.since)
ORDER BY q.since`, assetTypes)
	if err != nil {
		return nil, fmt.Errorf("pending manual %s: %w", kind, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
