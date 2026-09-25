package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Reads for the Daily Market Report (docs/MOMENTUM_SCANNER_API_DAILY_MARKET_REPORT.md).
// Pass-through sections come back as JSON built by Postgres, so the API serves
// what the pipeline stored rather than a re-typed copy of it.

// MacroRow is the newest macro_derived row for one metric.
type MacroRow struct {
	Metric  string
	TS      time.Time
	Value   *float64
	Payload json.RawMessage
}

// LatestMacroDerived returns the newest row per metric whose name starts with
// any of prefixes (e.g. "mp_", "mc_price_phase:"), keyed by metric.
func LatestMacroDerived(ctx context.Context, q Querier, prefixes []string) (map[string]MacroRow, error) {
	patterns := make([]string, len(prefixes))
	for i, p := range prefixes {
		patterns[i] = likeEscaper.Replace(p) + "%"
	}
	rows, err := q.Query(ctx, `
SELECT DISTINCT ON (metric) metric, ts, value, COALESCE(payload, 'null'::jsonb)
FROM macro_derived
WHERE source = 'macro_analysis' AND metric LIKE ANY($1)
ORDER BY metric, ts DESC`, patterns)
	if err != nil {
		return nil, fmt.Errorf("latest macro derived: %w", err)
	}
	defer rows.Close()
	out := map[string]MacroRow{}
	for rows.Next() {
		var r MacroRow
		if err := rows.Scan(&r.Metric, &r.TS, &r.Value, &r.Payload); err != nil {
			return nil, fmt.Errorf("scan macro derived: %w", err)
		}
		r.TS = r.TS.UTC()
		out[r.Metric] = r
	}
	return out, rows.Err()
}

// FredPoint is one series' newest observation.
type FredPoint struct {
	Value float64
	TS    time.Time
}

// LatestFred returns each series' newest observation since `since` (absent key
// = no observation in that window). macro_fred is a hypertable of ~4,000
// 7-day chunks (FRED history back to 1962); an unbounded "latest row" query
// plans across all of them (~4.7s). The time bound lets chunks be excluded.
func LatestFred(ctx context.Context, q Querier, series []string, since time.Time) (map[string]FredPoint, error) {
	rows, err := q.Query(ctx, `
SELECT s.id, f.value, f.ts
FROM unnest($1::text[]) AS s(id)
CROSS JOIN LATERAL (
    SELECT value, ts FROM macro_fred WHERE series_id = s.id AND ts >= $2 ORDER BY ts DESC LIMIT 1
) f`, series, since)
	if err != nil {
		return nil, fmt.Errorf("latest fred: %w", err)
	}
	defer rows.Close()
	out := map[string]FredPoint{}
	for rows.Next() {
		var id string
		var p FredPoint
		if err := rows.Scan(&id, &p.Value, &p.TS); err != nil {
			return nil, fmt.Errorf("scan fred: %w", err)
		}
		p.TS = p.TS.UTC()
		out[id] = p
	}
	return out, rows.Err()
}

// jsonArray runs a query built as `SELECT COALESCE(json_agg(t), '[]') FROM (...) t`.
func jsonArray(ctx context.Context, q Querier, what, sql string, args ...any) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := q.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return raw, nil
}

// UpcomingEconomicEvents: the economic calendar between from and to.
func UpcomingEconomicEvents(ctx context.Context, q Querier, from, to time.Time, limit int) (json.RawMessage, error) {
	return jsonArray(ctx, q, "economic calendar", `
SELECT COALESCE(json_agg(t ORDER BY t.event_ts), '[]'::json) FROM (
  SELECT event_ts, country, event_name, impact, actual, estimate, previous, unit
  FROM economic_calendar_events WHERE event_ts BETWEEN $1 AND $2
  ORDER BY event_ts LIMIT $3) t`, from, to, limit)
}

// UpcomingEarnings: earnings dates for symbols between from and to.
func UpcomingEarnings(ctx context.Context, q Querier, symbols []string, from, to time.Time) (json.RawMessage, error) {
	return jsonArray(ctx, q, "earnings calendar", `
SELECT COALESCE(json_agg(t ORDER BY t.date, t.symbol), '[]'::json) FROM (
  SELECT symbol, earnings_date AS date, quarter AS period, year, hour, eps_estimate
  FROM earnings_calendar_events
  WHERE symbol = ANY($1) AND earnings_date BETWEEN $2::date AND $3::date) t`, symbols, from, to)
}

// MacroHeadlines: the same macro-tagged headlines the Discord report shows.
// Headline, source, url and time only — never article text.
func MacroHeadlines(ctx context.Context, q Querier, limit int) (json.RawMessage, error) {
	return jsonArray(ctx, q, "macro headlines", `
SELECT COALESCE(json_agg(t ORDER BY t.ts DESC), '[]'::json) FROM (
  SELECT ts, source, headline, url FROM news_headlines
  WHERE source LIKE 'rss\_macro\_%' OR source = 'finnhub_macro_general'
  ORDER BY ts DESC LIMIT $1) t`, limit)
}

// latestJSONRow returns one row as JSON, or nil when the table is empty.
func latestJSONRow(ctx context.Context, q Querier, what, sql string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := q.QueryRow(ctx, sql).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return raw, nil
}

// LatestGPR: the newest monthly geopolitical-risk reading (nil when none).
func LatestGPR(ctx context.Context, q Querier) (json.RawMessage, error) {
	return latestJSONRow(ctx, q, "latest gpr", `
SELECT row_to_json(t) FROM (
  SELECT month_ts, gpr_total, gpr_act, gpr_threat, source
  FROM geopolitical_risk_monthly ORDER BY month_ts DESC LIMIT 1) t`)
}

// LatestGDELT: the newest daily GDELT tone reading (nil when none).
func LatestGDELT(ctx context.Context, q Querier) (json.RawMessage, error) {
	return latestJSONRow(ctx, q, "latest gdelt", `
SELECT row_to_json(t) FROM (
  SELECT day_ts, query_label, article_count, avg_tone, avg_goldstein
  FROM gdelt_macro_daily ORDER BY day_ts DESC, ingested_at DESC LIMIT 1) t`)
}

// InstrumentCloses returns an instrument's last two daily closes, oldest first,
// through the same readers the market cycle uses: one row per day with the
// preferred source for equities, closed candles only for crypto.
func InstrumentCloses(ctx context.Context, q Querier, symbol, kind string, now time.Time) ([]EquityOHLCVBar, error) {
	if kind == "crypto" {
		return QueryCryptoClosedDailyAsc(ctx, q, symbol, "1d", 2, now)
	}
	// Same one-row-per-day, preferred-source rule as QueryEquityOHLCVAsc, bounded
	// to recent chunks: an unbounded read scanned the symbol's whole history
	// (~0.4s each) to return two rows.
	rows, err := q.Query(ctx, `
SELECT ts, open, high, low, close, volume FROM (
    SELECT DISTINCT ON ((ts AT TIME ZONE 'UTC')::date) ts, open, high, low, close, volume
    FROM equity_ohlcv
    WHERE symbol = $1 AND interval = '1Day' AND ts >= $2
    ORDER BY (ts AT TIME ZONE 'UTC')::date DESC, bar_source_rank(source) DESC
) one_per_ts
ORDER BY ts DESC
LIMIT 2`, symbol, now.AddDate(0, 0, -45))
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
	if len(desc) == 2 {
		desc[0], desc[1] = desc[1], desc[0]
	}
	return desc, nil
}

// MarketReportInputs is everything one report needs, read in one call.
type MarketReportInputs struct {
	Macro     map[string]MacroRow         // stance, signal, mc_*, aa_* rows
	Fred      map[string]FredPoint        // VIXCLS, DGS10, DEXUSEU, DGS5, DGS2
	Closes    map[string][]EquityOHLCVBar // instrument -> last two daily closes
	Economic  json.RawMessage
	Earnings  json.RawMessage
	Headlines json.RawMessage
	GPR       json.RawMessage // nil = none
	GDELT     json.RawMessage // nil = none
	Watchlist []WatchlistItem
}

// InstrumentRef names an instrument whose closes the report needs.
type InstrumentRef struct {
	Symbol string
	Kind   string // "equity" or "crypto"
}

// LoadMarketReport reads every input of the Daily Market Report. The watchlist
// is read first because its equities join the instrument and earnings lists.
func LoadMarketReport(ctx context.Context, q Querier, fixed []InstrumentRef, earningsSymbols []string,
	fredSeries []string, now time.Time) (MarketReportInputs, error) {
	var in MarketReportInputs
	var err error
	if in.Watchlist, err = ListWatchlist(ctx, q, nil); err != nil {
		return in, err
	}
	refs := append([]InstrumentRef{}, fixed...)
	seen := map[string]bool{}
	for _, r := range fixed {
		seen[r.Symbol] = true
	}
	earn := append([]string{}, earningsSymbols...)
	for _, w := range in.Watchlist {
		if !seen[w.Symbol] {
			seen[w.Symbol] = true
			refs = append(refs, InstrumentRef{w.Symbol, "equity"})
			earn = append(earn, w.Symbol)
		}
	}
	if in.Macro, err = LatestMacroDerived(ctx, q, []string{"mp_", "gc_", "inf_", "gg_", "mc_", "aa_"}); err != nil {
		return in, err
	}
	if in.Fred, err = LatestFred(ctx, q, fredSeries, now.AddDate(0, 0, -60)); err != nil {
		return in, err
	}
	in.Closes = map[string][]EquityOHLCVBar{}
	for _, r := range refs {
		bars, err := InstrumentCloses(ctx, q, r.Symbol, r.Kind, now)
		if err != nil {
			return in, fmt.Errorf("closes %s: %w", r.Symbol, err)
		}
		in.Closes[r.Symbol] = bars
	}
	from, to := now, now.AddDate(0, 0, 14)
	if in.Economic, err = UpcomingEconomicEvents(ctx, q, from, to, 40); err != nil {
		return in, err
	}
	if in.Earnings, err = UpcomingEarnings(ctx, q, earn, from, to); err != nil {
		return in, err
	}
	if in.Headlines, err = MacroHeadlines(ctx, q, 8); err != nil {
		return in, err
	}
	if in.GPR, err = LatestGPR(ctx, q); err != nil {
		return in, err
	}
	if in.GDELT, err = LatestGDELT(ctx, q); err != nil {
		return in, err
	}
	return in, nil
}
