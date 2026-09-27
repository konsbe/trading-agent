package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/compute"
	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

// Watchlist fallback for a symbol the scanner never scans (no momentum_features
// row, e.g. ADRs such as TSM or BP). Computed at read time with the scanner's
// own feature code and never stored: momentum_features is the scan's coverage
// record ("X of Y symbols scanned"), and a second store would go stale.

// fallbackBars is enough for momentum.ComputeAt's longest window (the 52-week
// high needs 252 sessions including today).
const fallbackBars = 300

// BarFallback is one symbol's features from its own daily bars.
type BarFallback struct {
	Features momentum.Features
	AsOf     time.Time // the latest bar's session
	Source   string    // equity_ohlcv.source of the latest bar
	// MarketCap is the newest finnhub_metric market_cap, nil when absent or
	// nulled by the non-USD guard; MarketCapNote is that guard's note.
	MarketCap     *float64
	MarketCapNote *string
}

// LoadBarFallback returns nil when the symbol has no daily bars.
func LoadBarFallback(ctx context.Context, q Querier, symbol string) (*BarFallback, error) {
	// One bar per session, preferred source first — the same selection the
	// price chart and technical-analysis read (bar_source_rank, migration 015).
	rows, err := q.Query(ctx, `
SELECT ts, open, high, low, close, volume, source FROM (
    SELECT DISTINCT ON ((ts AT TIME ZONE 'UTC')::date) ts, open, high, low, close, volume, source
    FROM equity_ohlcv
    WHERE symbol = upper($1) AND interval = '1Day' AND close > 0
    ORDER BY (ts AT TIME ZONE 'UTC')::date DESC, bar_source_rank(source) DESC
) one_per_session
ORDER BY ts DESC
LIMIT $2`, symbol, fallbackBars)
	if err != nil {
		return nil, fmt.Errorf("bar fallback %s: %w", symbol, err)
	}
	defer rows.Close()
	var desc []compute.Bar
	var latestSource string
	for rows.Next() {
		var b compute.Bar
		var src string
		if err := rows.Scan(&b.TS, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume, &src); err != nil {
			return nil, fmt.Errorf("bar fallback %s: %w", symbol, err)
		}
		if len(desc) == 0 {
			latestSource = src
		}
		desc = append(desc, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bar fallback %s: %w", symbol, err)
	}
	if len(desc) == 0 {
		return nil, nil
	}
	bars := make([]compute.Bar, len(desc))
	for i, b := range desc {
		bars[len(desc)-1-i] = b
	}
	last := len(bars) - 1
	fb := &BarFallback{
		Features: momentum.ComputeAt(bars, last, momentum.DefaultConfig()),
		AsOf:     bars[last].TS.UTC(),
		Source:   latestSource,
	}

	// The newest row even when its value is null: a non-USD cap is nulled by
	// data-fundamental, and an older non-null row must not stand in for it.
	err = q.QueryRow(ctx, `
SELECT value, payload->>'note' FROM equity_fundamentals
WHERE symbol = upper($1) AND metric = 'market_cap' AND source = 'finnhub_metric'
ORDER BY ts DESC LIMIT 1`, symbol).Scan(&fb.MarketCap, &fb.MarketCapNote)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("bar fallback %s: market cap: %w", symbol, err)
	}
	return fb, nil
}
