package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Reads for GET /api/v1/scanner/today/{symbol}/analysis
// (docs/MOMENTUM_SCANNER_FULL_STOCK_ANALYSIS_API.md Part A). Every value is a
// row technical-analysis / fundamental-analysis / macro-analysis stored; the
// reads mirror the analyst bot's (db/queries/technical.py latest_indicators,
// fundamental.py latest_derived) so the web and Discord see the same row.

// AnalysisFreshness is what decides whether a symbol's stored analysis is
// current. Nil means no such row.
type AnalysisFreshness struct {
	// LatestBarTS is the ts technical-analysis would write under now: the last
	// bar of its own loader (QueryEquityBars, one bar per session).
	LatestBarTS *time.Time
	// TechnicalTS is the newest technical_indicators ts for the symbol.
	TechnicalTS *time.Time
	// DerivedTS is the newest fundamental_analysis row; RawTS the newest raw
	// fundamental row (any other source) — the derived rows' inputs.
	DerivedTS *time.Time
	RawTS     *time.Time
}

func AnalysisFreshnessFor(ctx context.Context, q Querier, symbol, interval string) (AnalysisFreshness, error) {
	var f AnalysisFreshness
	bars, err := QueryEquityBars(ctx, q, symbol, interval, 1)
	if err != nil {
		return f, fmt.Errorf("latest bar %s: %w", symbol, err)
	}
	if len(bars) == 1 {
		ts := bars[0].TS.UTC()
		f.LatestBarTS = &ts
	}
	if err := q.QueryRow(ctx, `
SELECT (SELECT max(ts) FROM technical_indicators
         WHERE symbol = $1 AND exchange = 'equity' AND interval = $2),
       (SELECT max(ts) FROM equity_fundamentals
         WHERE symbol = $1 AND source = 'fundamental_analysis'),
       (SELECT max(ts) FROM equity_fundamentals
         WHERE symbol = $1 AND source <> 'fundamental_analysis')`,
		symbol, interval).Scan(&f.TechnicalTS, &f.DerivedTS, &f.RawTS); err != nil {
		return f, fmt.Errorf("analysis freshness %s: %w", symbol, err)
	}
	for _, p := range []*time.Time{f.TechnicalTS, f.DerivedTS, f.RawTS} {
		if p != nil {
			*p = p.UTC()
		}
	}
	return f, nil
}

// StoredRow is one latest technical indicator or derived fundamental metric.
type StoredRow struct {
	TS      time.Time
	Value   *float64
	Payload json.RawMessage
}

// NewsHeadline is one news_headlines row for the symbol.
type NewsHeadline struct {
	Title       string
	URL         *string
	Source      string
	PublishedAt time.Time
	Sentiment   *float64
}

// AnalysisInputs is every stored row the analysis response is built from.
type AnalysisInputs struct {
	Indicators map[string]StoredRow // latest per indicator (symbol, equity, interval)
	Derived    map[string]StoredRow // latest per derived metric (period = 'derived')
	// MarketCap is the newest raw market_cap (finnhub_metric, USD).
	MarketCap *float64
	Headlines []NewsHeadline
	Macro     map[string]MacroRow // mc_market_cycle, mc_macro_correlation, mc_price_phase:<symbol>
	// VIX is the newest VIXCLS in macro_fred, what the bot's actions engine
	// classifies its own regime from. Nil when none is stored.
	VIX *float64
}

func LoadAnalysis(ctx context.Context, q Querier, symbol, interval string, headlines int) (AnalysisInputs, error) {
	in := AnalysisInputs{Indicators: map[string]StoredRow{}, Derived: map[string]StoredRow{}}
	if err := latestRows(ctx, q, in.Indicators, "indicators", `
SELECT DISTINCT ON (indicator) indicator, ts, value, payload
FROM technical_indicators
WHERE symbol = $1 AND exchange = 'equity' AND interval = $2
ORDER BY indicator, ts DESC`, symbol, interval); err != nil {
		return in, err
	}
	if err := latestRows(ctx, q, in.Derived, "derived fundamentals", `
SELECT DISTINCT ON (metric) metric, ts, value, payload
FROM equity_fundamentals
WHERE symbol = $1 AND period = 'derived'
ORDER BY metric, ts DESC,
         (value IS NULL AND payload IS NULL),
         fundamental_source_rank(source) DESC`, symbol); err != nil {
		return in, err
	}
	if err := q.QueryRow(ctx, `
SELECT value FROM equity_fundamentals
WHERE symbol = $1 AND metric = 'market_cap' AND source <> 'fundamental_analysis' AND value IS NOT NULL
ORDER BY ts DESC, fundamental_source_rank(source) DESC LIMIT 1`, symbol).Scan(&in.MarketCap); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return in, fmt.Errorf("analysis market cap %s: %w", symbol, err)
	}
	// The bot reads the newest VIXCLS ever stored. Unbounded, that plans across
	// every macro_fred chunk (seconds), so look at recent chunks first and fall
	// back to the full table only when they are empty.
	for _, bound := range []string{`AND ts > now() - interval '60 days'`, ``} {
		err := q.QueryRow(ctx, `SELECT value FROM macro_fred WHERE series_id = 'VIXCLS' `+bound+` ORDER BY ts DESC LIMIT 1`).Scan(&in.VIX)
		if err == nil {
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return in, fmt.Errorf("analysis vix: %w", err)
		}
	}
	rows, err := q.Query(ctx, `
SELECT headline, url, source, ts, sentiment::float8
FROM news_headlines
WHERE symbol = $1
ORDER BY ts DESC
LIMIT $2`, symbol, headlines)
	if err != nil {
		return in, fmt.Errorf("analysis headlines %s: %w", symbol, err)
	}
	for rows.Next() {
		var h NewsHeadline
		if err := rows.Scan(&h.Title, &h.URL, &h.Source, &h.PublishedAt, &h.Sentiment); err != nil {
			rows.Close()
			return in, fmt.Errorf("scan headline: %w", err)
		}
		h.PublishedAt = h.PublishedAt.UTC()
		in.Headlines = append(in.Headlines, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return in, err
	}
	macro, err := LatestMacroDerived(ctx, q, []string{"mc_market_cycle", "mc_macro_correlation", "mc_price_phase:" + symbol})
	if err != nil {
		return in, err
	}
	in.Macro = map[string]MacroRow{}
	for _, k := range []string{"mc_market_cycle", "mc_macro_correlation", "mc_price_phase:" + symbol} {
		if r, ok := macro[k]; ok {
			in.Macro[k] = r
		}
	}
	return in, nil
}

func latestRows(ctx context.Context, q Querier, into map[string]StoredRow, what, sql string, args ...any) error {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("analysis %s: %w", what, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var r StoredRow
		var payload []byte
		if err := rows.Scan(&name, &r.TS, &r.Value, &payload); err != nil {
			return fmt.Errorf("scan %s: %w", what, err)
		}
		r.TS = r.TS.UTC()
		r.Payload = payload
		into[name] = r
	}
	return rows.Err()
}
