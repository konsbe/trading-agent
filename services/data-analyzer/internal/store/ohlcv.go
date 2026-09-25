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
// One bar per period even when several sources wrote it. For daily-or-longer
// intervals the key is the UTC calendar date, not ts: the sources stamp the
// same session differently (Tiingo 00:00 UTC, Yahoo 13:30 UTC for US
// listings), so a per-ts key kept both. Intraday intervals keep the exact ts.
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
func QueryEquityBars(ctx context.Context, pool Querier, symbol, interval string, limit int) ([]compute.Bar, error) {
	rows, err := pool.Query(ctx, `
		SELECT ts, open, high, low, close, volume FROM (
			SELECT DISTINCT ON (period) ts, open, high, low, close, volume
			FROM (
				SELECT *, CASE WHEN $2 IN ('1Day', '1Week', '1Month')
				               THEN date_trunc('day', ts AT TIME ZONE 'UTC')
				               ELSE ts AT TIME ZONE 'UTC' END AS period
				FROM equity_ohlcv
				WHERE symbol=$1 AND interval=$2
			) keyed
			-- This was the ONE latest-row query in the repo that broke the
			-- source tie explicitly, and it was right. Migration 015 promotes
			-- the same ranking to a shared function so the other callers
			-- inherit it instead of each remembering.
			ORDER BY period, bar_source_rank(source) DESC
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
