package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LatestFundamentalPayload returns the newest non-null payload for a metric, or
// nil when none exists.
func LatestFundamentalPayload(ctx context.Context, pool *pgxpool.Pool, symbol, metric string) (map[string]any, error) {
	var raw []byte
	err := pool.QueryRow(ctx, `
SELECT payload FROM equity_fundamentals
WHERE symbol = $1 AND metric = $2 AND payload IS NOT NULL
ORDER BY ts DESC LIMIT 1`, symbol, metric).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest fundamental payload %s/%s: %w", symbol, metric, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode fundamental payload %s/%s: %w", symbol, metric, err)
	}
	return m, nil
}

// nullNonUSDMarketCapSQL nulls every finnhub_metric market_cap row still holding
// a value for the symbol. Every consumer reads the newest NON-NULL row, so
// writing NULL going forward is not enough: the older local-currency values
// would simply resurface. Idempotent via the value IS NOT NULL filter; the
// original local figure is kept in the payload, in millions as Finnhub sent it.
const nullNonUSDMarketCapSQL = `
UPDATE equity_fundamentals SET
    payload = COALESCE(payload, '{}'::jsonb) || jsonb_build_object(
        'currency', $2::text,
        'market_cap_millions_local', value / 1e6,
        'note', 'nulled: Finnhub marketCapitalization is in the reporting currency, not USD')
    , value = NULL
WHERE symbol = $1 AND metric = 'market_cap' AND source = 'finnhub_metric' AND value IS NOT NULL`

// NullNonUSDMarketCapHistory repairs market_cap rows written before the
// reporting currency was checked. Returns the number of rows repaired.
func NullNonUSDMarketCapHistory(ctx context.Context, pool *pgxpool.Pool, symbol, currency string) (int64, error) {
	ct, err := pool.Exec(ctx, nullNonUSDMarketCapSQL, symbol, currency)
	if err != nil {
		return 0, fmt.Errorf("null non-USD market_cap history %s: %w", symbol, err)
	}
	return ct.RowsAffected(), nil
}
