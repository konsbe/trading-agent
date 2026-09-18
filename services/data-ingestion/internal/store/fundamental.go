package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const upsertFundamentalSQL = `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (symbol, period, metric, source, ts) DO UPDATE SET
    value   = EXCLUDED.value,
    payload = EXCLUDED.payload`

// UpsertFundamental persists one fundamental metric row.
//   - period:  "ttm", "annual_2024", "q_2024Q3", "point_in_time", etc.
//   - metric:  e.g. "eps_ttm", "revenue_ttm", "pe_ratio", "fcf_ttm", "gross_margin_ttm"
//   - value:   primary scalar (nil for payload-only rows)
//   - payload: structured context (nil for scalar-only rows)
//   - source:  "finnhub_metric", "finnhub_financials_reported", "finnhub_earnings"
func UpsertFundamental(
	ctx context.Context,
	pool *pgxpool.Pool,
	ts time.Time,
	symbol, period, metric string,
	value *float64,
	payload any,
	source string,
) error {
	var jb []byte
	if payload != nil {
		var err error
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, upsertFundamentalSQL, ts, symbol, period, metric, value, jb, source)
	return err
}

// LatestFundamental returns the most recent non-null value of one metric for a
// symbol, or nil when none exists.
//
// Used by the /stock/profile2 fallback so it does not overwrite a value that
// /stock/metric already supplied: the profile write is a gap-filler, and if the
// primary endpoint ever starts returning shareOutstanding the fallback should
// stand down rather than compete with it.
func LatestFundamental(ctx context.Context, pool *pgxpool.Pool, symbol, metric string) (*float64, error) {
	var v *float64
	err := pool.QueryRow(ctx, `
SELECT value FROM equity_fundamentals
WHERE symbol = $1 AND metric = $2 AND value IS NOT NULL
ORDER BY ts DESC LIMIT 1`, symbol, metric).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest fundamental %s/%s: %w", symbol, metric, err)
	}
	return v, nil
}
