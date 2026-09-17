package store

// TODO: when compute package is migrated to Python, replace these query functions
// with asyncpg calls in the Python service. The SQL queries themselves remain identical.

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
)

// QueryEquityBars returns up to limit equity OHLCV bars for the given symbol
// and interval, ordered oldest-first (chronological).
//
// DISTINCT ON (ts) ensures each timestamp appears only once even when multiple
// sources have rows for the same bar.
//
// PREFERENCE ORDER IS A CORRECTNESS CONCERN, NOT A TIE-BREAK. The sources do not
// agree on what their prices mean:
//
//	tiingo         split AND dividend adjusted (reads Tiingo's adj* fields)
//	yahoo_finance  NOT dividend adjusted — internal/fetch/yahoo decodes
//	               indicators.quote and never indicators.adjclose, so whatever
//	               dividend adjustment Yahoo offers there is absent by
//	               construction
//	alpaca         IEX-only volume on the free tier (§2.2 rejects it outright)
//
// Tiingo is therefore preferred first. This ordering used to prefer
// yahoo_finance, which meant a symbol covered by both silently resolved to the
// unadjusted series — strictly worse than either source alone, because the
// symbol *looked* fully covered while serving prices that drift from the
// adjusted series by the cumulative dividend. That shifts every price-derived
// feature: 52-week ratios, resistance levels, and change_pct across any
// ex-dividend date.
//
// See services/data-ingestion/data_ingestion.md for the full caveat.
func QueryEquityBars(ctx context.Context, pool *pgxpool.Pool, symbol, interval string, limit int) ([]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
		SELECT ts, open, high, low, close, volume FROM (
			SELECT DISTINCT ON (ts) ts, open, high, low, close, volume
			FROM equity_ohlcv
			WHERE symbol=$1 AND interval=$2
			ORDER BY ts,
				(source = 'tiingo')        DESC,
				(source = 'yahoo_finance') DESC
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

// QueryCryptoBars returns up to limit crypto OHLCV bars for the given symbol
// and interval, ordered oldest-first (chronological).
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

// QueryLatestEquityClose returns the most recent closing price for symbol from
// equity_ohlcv, or (0, false) if no row exists.
// Used by fundamental-analysis scoreTier3 to compute analyst target upside.
func QueryLatestEquityClose(ctx context.Context, pool *pgxpool.Pool, symbol, interval string) (float64, bool, error) {
	var close float64
	err := pool.QueryRow(ctx, `
		SELECT close FROM equity_ohlcv
		WHERE symbol=$1 AND interval=$2
		ORDER BY ts DESC LIMIT 1`,
		symbol, interval).Scan(&close)
	if err != nil {
		return 0, false, err
	}
	return close, true, nil
}

// scanAndReverse reads pgx rows into []compute.Bar and reverses the slice so
// bars are ordered oldest-first, which is what all compute functions expect.
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
