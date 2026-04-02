package store

// TODO: when fundamental-analysis is migrated to Python, replace with asyncpg upserts.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	pool *pgxpool.Pool,
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
// of a given symbol, across all raw source rows.
func QueryLatestMetrics(ctx context.Context, pool *pgxpool.Pool, symbol string) ([]FundamentalRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (metric, period) ts, period, metric, value, payload, source
		FROM equity_fundamentals
		WHERE symbol = $1
		  AND source != 'fundamental_analysis'
		ORDER BY metric, period, ts DESC`,
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
func QueryLatestDerived(ctx context.Context, pool *pgxpool.Pool, symbol string) ([]FundamentalRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (metric) ts, period, metric, value, payload, source
		FROM equity_fundamentals
		WHERE symbol = $1
		  AND source = 'fundamental_analysis'
		ORDER BY metric, ts DESC`,
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

// QueryMetricSeries returns the last `limit` distinct quarterly periods for a
// single metric of one symbol, ordered newest-first.
//
// Use this for 8-quarter trend analysis: pass limit=8, metric="gross_profit_reported".
// Only rows from finnhub_financials_reported are returned so we don't mix TTM
// and quarterly figures.
func QueryMetricSeries(ctx context.Context, pool *pgxpool.Pool, symbol, metric string, limit int) ([]FundamentalRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (period) period, ts, metric, value, payload, source
		FROM equity_fundamentals
		WHERE symbol = $1
		  AND metric  = $2
		  AND source  = 'finnhub_financials_reported'
		  AND value IS NOT NULL
		ORDER BY period DESC, ts DESC
		LIMIT $3`,
		symbol, metric, limit)
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
