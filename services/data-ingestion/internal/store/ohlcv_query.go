package store

// CountEquityBars and CountCryptoBars are used by the data-technical worker
// to determine whether a backfill is needed on startup.
//
// QueryEquityBars / QueryCryptoBars have been moved to
// services/data-analyzer/internal/store/ohlcv.go because only the
// technical-analysis worker reads bars for computation.

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CountEquityBars returns the number of distinct timestamps stored for the
// given symbol and interval. DISTINCT avoids overcounting when multiple
// sources (e.g. Yahoo Finance and Alpaca) share a timestamp.
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
