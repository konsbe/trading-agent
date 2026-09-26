package store

// TODO: when fundamental-analysis is migrated to Python, replace with asyncpg upserts.

import (
	"context"
	"encoding/json"
	"time"
)

// FundamentalRow is a single metric read from the equity_fundamentals table.
type FundamentalRow struct {
	TS      time.Time
	Period  string
	Metric  string
	Value   *float64
	Payload []byte
	Source  string
}

const upsertFundamentalSQL = `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, payload, source)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (symbol, period, metric, source, ts) DO UPDATE SET
    value   = EXCLUDED.value,
    payload = EXCLUDED.payload`

// UpsertFundamentalDerived persists one derived (computed) fundamental metric.
// source is always set to "fundamental_analysis" to distinguish from raw Finnhub rows.
func UpsertFundamentalDerived(
	ctx context.Context,
	pool Execer,
	ts time.Time,
	symbol, period, metric string,
	value *float64,
	payload any,
) error {
	var jb []byte
	if payload != nil {
		var err error
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	_, err := pool.Exec(ctx, upsertFundamentalSQL, ts, symbol, period, metric, value, jb, "fundamental_analysis")
	return err
}

// QueryLatestMetrics returns the most recent value for each (metric, period) pair
// of a given symbol, across all raw source rows. Rows come in ascending period
// LABEL order, which is not recency: callers pick a metric's newest period by
// its period end (fundamental.latestValues), never by row order.
func QueryLatestMetrics(ctx context.Context, pool Querier, symbol string) ([]FundamentalRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (metric, period) ts, period, metric, value, payload, source
		FROM equity_fundamentals
		WHERE symbol = $1
		  AND source != 'fundamental_analysis'
		-- NULL-last then source rank: equity_fundamentals is written by five
		-- sources and 4,912 (symbol, metric, period, ts) groups have more than
		-- one, so ts alone does not determine a winner.
		ORDER BY metric, period, ts DESC,
		         (value IS NULL AND payload IS NULL),
		         fundamental_source_rank(source) DESC`,
		symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FundamentalRow
	for rows.Next() {
		var r FundamentalRow
		if err := rows.Scan(&r.TS, &r.Period, &r.Metric, &r.Value, &r.Payload, &r.Source); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// QueryLatestDerived returns the most recent value and payload for each derived
// metric (source = 'fundamental_analysis') for a symbol. This is used by
// scoreCorrelations to read prior-pass outputs (Tier 1–3, qualitative) without
// re-computing them.
func QueryLatestDerived(ctx context.Context, pool Querier, symbol string) ([]FundamentalRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (metric) ts, period, metric, value, payload, source
		FROM equity_fundamentals
		WHERE symbol = $1
		  AND source = 'fundamental_analysis'
		-- This query pins source = fundamental_analysis, so it is NOT exposed
		-- to the multi-source tie. It was still ambiguous on PERIOD: the same
		-- derived metric can exist as annual and quarterly, and ordering by ts
		-- alone could return either depending on ingestion order.
		ORDER BY metric, ts DESC, period, (value IS NULL AND payload IS NULL)`,
		symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FundamentalRow
	for rows.Next() {
		var r FundamentalRow
		if err := rows.Scan(&r.TS, &r.Period, &r.Metric, &r.Value, &r.Payload, &r.Source); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// FilingKind selects which financials-reported filings a series reads.
type FilingKind int

const (
	// QuarterlyFilings are 10-Qs, period "q_YYYY-MM-DD" (the period end).
	QuarterlyFilings FilingKind = iota
	// AnnualFilings are 10-Ks, period "annual_YYYY" (the fiscal year).
	AnnualFilings
)

func (k FilingKind) periodPattern() string {
	if k == AnnualFilings {
		return `annual\_%`
	}
	return `q\_%`
}

// QueryMetricSeries returns the last `limit` filings of one kind for a single
// metric of one symbol, ordered newest-first.
//
// Use this for 8-quarter trend analysis: pass limit=8, metric="gross_profit_reported",
// kind=QuarterlyFilings. Only rows from finnhub_financials_reported are
// returned so we don't mix TTM figures in, and only one kind, because a 10-K
// and a 10-Q figure cover different spans. Within one kind the period labels
// sort chronologically as strings (zero-padded dates / four-digit years).
func QueryMetricSeries(ctx context.Context, pool Querier, symbol, metric string, kind FilingKind, limit int) ([]FundamentalRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (period) period, ts, metric, value, payload, source
		FROM equity_fundamentals
		WHERE symbol = $1
		  AND metric  = $2
		  AND source  = 'finnhub_financials_reported'
		  AND period LIKE $4
		  AND value IS NOT NULL
		ORDER BY period DESC, ts DESC,
		         (value IS NULL AND payload IS NULL),
		         fundamental_source_rank(source) DESC
		LIMIT $3`,
		symbol, metric, limit, kind.periodPattern())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FundamentalRow
	for rows.Next() {
		var r FundamentalRow
		if err := rows.Scan(&r.Period, &r.TS, &r.Metric, &r.Value, &r.Payload, &r.Source); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
