package store

import (
	"context"
	"time"
)

// EquityOHLCVBar is one daily (or other interval) row from equity_ohlcv.
type EquityOHLCVBar struct {
	TS     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// QueryEquityOHLCVAsc returns the last `limit` bars for symbol×interval in ascending time order.
func QueryEquityOHLCVAsc(ctx context.Context, pool Querier, symbol, interval string, limit int) ([]EquityOHLCVBar, error) {
	// One row per SESSION, the preferred source (bar_source_rank, migration
	// 015) winning. Keyed on the UTC calendar date, not ts: the sources stamp
	// the same session differently (Tiingo 00:00 UTC, Yahoo 13:30 UTC for US
	// listings), so DISTINCT ON (ts) kept both and a "200-bar" SMA covered ~100
	// sessions. The UTC date is the session date for every source here.
	rows, err := pool.Query(ctx,
		`SELECT ts, open, high, low, close, volume FROM (
		     SELECT DISTINCT ON ((ts AT TIME ZONE 'UTC')::date) ts, open, high, low, close, volume
		     FROM equity_ohlcv
		     WHERE symbol = $1 AND interval = $2
		     ORDER BY (ts AT TIME ZONE 'UTC')::date DESC, bar_source_rank(source) DESC
		 ) one_per_session
		 ORDER BY ts DESC
		 LIMIT $3`,
		symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var desc []EquityOHLCVBar
	for rows.Next() {
		var b EquityOHLCVBar
		if err := rows.Scan(&b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		desc = append(desc, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse to ascending
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

// QueryCryptoClosedDailyAsc returns up to limit CLOSED daily candles for a
// crypto symbol, oldest first, one per timestamp (REST preferred over the
// websocket feed). Binance returns the still-open current-day candle as its
// last row and ingestion stores it, so its "close" is the live price; a candle
// whose 24h window has not ended by closedAsOf is excluded here.
func QueryCryptoClosedDailyAsc(ctx context.Context, q Querier, symbol, interval string, limit int, closedAsOf time.Time) ([]EquityOHLCVBar, error) {
	rows, err := q.Query(ctx,
		`SELECT ts, open, high, low, close, volume FROM (
		     SELECT DISTINCT ON ((ts AT TIME ZONE 'UTC')::date) ts, open, high, low, close, volume
		     FROM crypto_ohlcv
		     WHERE symbol = $1 AND interval = $2 AND ts + interval '1 day' <= $4
		     ORDER BY (ts AT TIME ZONE 'UTC')::date DESC, (source = 'binance_rest') DESC
		 ) one_per_ts
		 ORDER BY ts DESC
		 LIMIT $3`,
		symbol, interval, limit, closedAsOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var desc []EquityOHLCVBar
	for rows.Next() {
		var b EquityOHLCVBar
		if err := rows.Scan(&b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		desc = append(desc, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}
