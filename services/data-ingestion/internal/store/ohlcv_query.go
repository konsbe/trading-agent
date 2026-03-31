package store

import (
	"context"

	"github.com/berdelis/trading-agent/services/data-ingestion/internal/compute"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CountEquityBars returns the number of distinct timestamps stored for the
// given symbol and interval. Using DISTINCT avoids overcounting when multiple
// sources (e.g. Yahoo Finance and Alpaca) have rows for the same bar.
func CountEquityBars(ctx context.Context, pool *pgxpool.Pool, symbol, interval string) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT ts) FROM equity_ohlcv WHERE symbol=$1 AND interval=$2`,
		symbol, interval).Scan(&n)
	return n, err
}

// CountCryptoBars returns how many stored bars match the given symbol and interval.
func CountCryptoBars(ctx context.Context, pool *pgxpool.Pool, symbol, interval string) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM crypto_ohlcv WHERE symbol=$1 AND interval=$2`,
		symbol, interval).Scan(&n)
	return n, err
}

// QueryEquityBars returns up to `limit` equity OHLCV bars for the given
// symbol and interval, ordered oldest-first (chronological).
//
// DISTINCT ON (ts) ensures each timestamp appears only once even when multiple
// sources (e.g. yahoo_finance and alpaca_data) have rows for the same bar.
// Yahoo Finance rows are preferred when both exist for a timestamp.
func QueryEquityBars(ctx context.Context, pool *pgxpool.Pool, symbol, interval string, limit int) ([]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
		SELECT ts, open, high, low, close, volume FROM (
			SELECT DISTINCT ON (ts) ts, open, high, low, close, volume
			FROM equity_ohlcv
			WHERE symbol=$1 AND interval=$2
			ORDER BY ts, (source = 'yahoo_finance') DESC
		) deduped
		ORDER BY ts DESC
		LIMIT $3`,
		symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAndReverse(rows)
}

// QueryCryptoBars returns up to `limit` crypto OHLCV bars for the given
// symbol and interval, ordered oldest-first (chronological).
func QueryCryptoBars(ctx context.Context, pool *pgxpool.Pool, symbol, interval string, limit int) ([]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
		SELECT ts, open, high, low, close, volume
		FROM crypto_ohlcv
		WHERE symbol=$1 AND interval=$2
		ORDER BY ts DESC
		LIMIT $3`,
		symbol, interval, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAndReverse(rows)
}

// scanAndReverse reads pgx rows into []compute.Bar and reverses the slice so
// that bars are ordered oldest-first, which is what all compute functions expect.
func scanAndReverse(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]compute.Bar, error) {
	var bars []compute.Bar
	for rows.Next() {
		var b compute.Bar
		if err := rows.Scan(&b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		bars = append(bars, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
	return bars, nil
}
