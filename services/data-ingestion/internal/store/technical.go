package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const upsertIndicatorSQL = `
INSERT INTO technical_indicators (ts, symbol, exchange, interval, indicator, value, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (symbol, exchange, interval, indicator, ts) DO UPDATE SET
    value   = EXCLUDED.value,
    payload = EXCLUDED.payload`

// UpsertIndicator persists a single computed indicator value.
//
//   - ts:        timestamp of the last bar used in the computation
//   - value:     scalar result (nil for payload-only indicators)
//   - payload:   structured data (nil for scalar-only indicators)
func UpsertIndicator(
	ctx context.Context,
	pool *pgxpool.Pool,
	ts time.Time,
	symbol, exchange, interval, indicator string,
	value *float64,
	payload any,
) error {
	var jb []byte
	if payload != nil {
		var err error
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, upsertIndicatorSQL, ts, symbol, exchange, interval, indicator, value, jb)
	return err
}
