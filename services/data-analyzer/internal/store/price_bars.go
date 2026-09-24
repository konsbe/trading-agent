package store

import (
	"context"
	"fmt"
	"time"
)

// Price history for the detail view's chart. Read-only, from equity_ohlcv.

// PriceBar is one OHLCV bar.
type PriceBar struct {
	TS     time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// DailyBars returns the symbol's daily bars from `from` (inclusive; zero for
// all history), one per session. Where several sources wrote the same session,
// bar_source_rank picks one — the same preference the scanner reads through,
// so the chart and the features agree on which series is "the" price.
// Values are split- and dividend-adjusted (tiingo), which is what a chart
// needs for continuity; a session's adjusted close can therefore sit below the
// features row's as-traded close after a later dividend.
func DailyBars(ctx context.Context, q Querier, symbol string, from time.Time) ([]PriceBar, error) {
	rows, err := q.Query(ctx, `
SELECT DISTINCT ON (ts) ts, open, high, low, close, volume
FROM equity_ohlcv
WHERE symbol = upper($1) AND interval = '1Day' AND ts >= $2 AND close > 0
ORDER BY ts, bar_source_rank(source) DESC`, symbol, from)
	if err != nil {
		return nil, fmt.Errorf("daily bars %s: %w", symbol, err)
	}
	return scanPriceBars(rows)
}

// LatestDailyBarTS is the symbol's most recent daily bar, the anchor for
// ranges like 1M, so a stale symbol still shows a full month.
func LatestDailyBarTS(ctx context.Context, q Querier, symbol string) (time.Time, bool, error) {
	var ts *time.Time
	if err := q.QueryRow(ctx, `
SELECT max(ts) FROM equity_ohlcv WHERE symbol = upper($1) AND interval = '1Day' AND close > 0`,
		symbol).Scan(&ts); err != nil {
		return time.Time{}, false, fmt.Errorf("latest daily bar %s: %w", symbol, err)
	}
	if ts == nil {
		return time.Time{}, false, nil
	}
	return ts.UTC(), true, nil
}

// IntradayBars returns the symbol's bars at `interval` (e.g. "5Min") for its
// most recent `sessions` New York trading days that have any intraday data.
// Empty when none are stored; the caller decides the fallback.
func IntradayBars(ctx context.Context, q Querier, symbol, interval string, sessions int) ([]PriceBar, error) {
	rows, err := q.Query(ctx, `
WITH days AS (
    SELECT DISTINCT (ts AT TIME ZONE 'America/New_York')::date AS d
    FROM equity_ohlcv
    WHERE symbol = upper($1) AND interval = $2
    ORDER BY d DESC
    LIMIT $3
)
SELECT DISTINCT ON (ts) ts, open, high, low, close, volume
FROM equity_ohlcv
WHERE symbol = upper($1) AND interval = $2 AND close > 0
  AND (ts AT TIME ZONE 'America/New_York')::date >= (SELECT min(d) FROM days)
ORDER BY ts, bar_source_rank(source) DESC`, symbol, interval, sessions)
	if err != nil {
		return nil, fmt.Errorf("intraday bars %s %s: %w", symbol, interval, err)
	}
	return scanPriceBars(rows)
}

type barRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func scanPriceBars(rows barRows) ([]PriceBar, error) {
	defer rows.Close()
	out := []PriceBar{}
	for rows.Next() {
		var b PriceBar
		var vol *float64
		if err := rows.Scan(&b.TS, &b.Open, &b.High, &b.Low, &b.Close, &vol); err != nil {
			return nil, fmt.Errorf("scan bar: %w", err)
		}
		b.TS = b.TS.UTC()
		if vol != nil {
			b.Volume = *vol
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
