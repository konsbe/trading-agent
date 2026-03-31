package store

import (
	"context"
	"encoding/json"
	"time"

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
