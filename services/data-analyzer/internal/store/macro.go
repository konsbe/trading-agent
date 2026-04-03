package store

// Macro store — read from macro_fred (raw FRED observations) and write to
// macro_derived (computed monetary-policy signals).
//
// TODO: when macro-analysis is migrated to Python, replace these helpers with
// asyncpg queries. The table schema is identical so SQL is directly portable.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MacroObs is one time-stamped observation from macro_fred.
type MacroObs struct {
	TS    time.Time
	Value float64
}

// QueryMacroFredLatest returns the most recent (value, ts, ok) from macro_fred
// for the given FRED series ID (e.g. "T10Y2Y", "FEDFUNDS").
func QueryMacroFredLatest(ctx context.Context, pool *pgxpool.Pool, seriesID string) (float64, time.Time, bool) {
	var val float64
	var ts time.Time
	row := pool.QueryRow(ctx,
		`SELECT value, ts FROM macro_fred
		 WHERE series_id = $1
		 ORDER BY ts DESC LIMIT 1`,
		seriesID)
	if err := row.Scan(&val, &ts); err != nil {
		return 0, time.Time{}, false
	}
	return val, ts, true
}

// QueryMacroFredSeries returns the last `limit` observations (newest-first) for
// a FRED series. Use this for trend / period-over-period comparisons.
func QueryMacroFredSeries(ctx context.Context, pool *pgxpool.Pool, seriesID string, limit int) ([]MacroObs, error) {
	rows, err := pool.Query(ctx,
		`SELECT ts, value FROM macro_fred
		 WHERE series_id = $1
		 ORDER BY ts DESC LIMIT $2`,
		seriesID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MacroObs
	for rows.Next() {
		var o MacroObs
		if err := rows.Scan(&o.TS, &o.Value); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

const upsertMacroDerivedSQL = `
INSERT INTO macro_derived (ts, metric, value, payload, source)
VALUES ($1, $2, $3, $4, 'macro_analysis')
ON CONFLICT (metric, source, ts) DO UPDATE SET
    value   = EXCLUDED.value,
    payload = EXCLUDED.payload`

// UpsertMacroDerived persists one computed macro signal to macro_derived.
// source is hard-coded to "macro_analysis" to match the constraint.
func UpsertMacroDerived(
	ctx context.Context,
	pool *pgxpool.Pool,
	ts time.Time,
	metric string,
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
	var v any
	if value != nil {
		v = *value
	}
	_, err := pool.Exec(ctx, upsertMacroDerivedSQL, ts, metric, v, jb)
	return err
}

// QueryLatestFREDValue is a convenience alias used by technical-analysis.
// Returns (value, ok, error) for the most recent macro_fred observation.
func QueryLatestFREDValue(ctx context.Context, pool *pgxpool.Pool, seriesID string) (float64, bool, error) {
	var val float64
	var ts time.Time
	row := pool.QueryRow(ctx,
		`SELECT value, ts FROM macro_fred
		 WHERE series_id = $1
		 ORDER BY ts DESC LIMIT 1`,
		seriesID)
	if err := row.Scan(&val, &ts); err != nil {
		// No rows is ok — just return (0, false, nil)
		return 0, false, nil
	}
	return val, true, nil
}

// QueryMacroDerivedLatest returns (value, payload-bytes, ok) for the most
// recent macro_derived row matching `metric`. Used by the analyst-bot.
func QueryMacroDerivedLatest(ctx context.Context, pool *pgxpool.Pool, metric string) (float64, []byte, bool) {
	var val float64
	var payload []byte
	row := pool.QueryRow(ctx,
		`SELECT value, payload FROM macro_derived
		 WHERE metric = $1 AND source = 'macro_analysis'
		 ORDER BY ts DESC LIMIT 1`,
		metric)
	if err := row.Scan(&val, &payload); err != nil {
		return 0, nil, false
	}
	return val, payload, true
}

// QueryMacroDerivedPayloadMap returns the JSON payload of the latest macro_derived row
// for `metric` (source = macro_analysis). Use this when `value` may be NULL — Scan into
// float64 would fail on NULL for QueryMacroDerivedLatest.
func QueryMacroDerivedPayloadMap(ctx context.Context, pool *pgxpool.Pool, metric string) (map[string]any, bool) {
	var raw []byte
	row := pool.QueryRow(ctx,
		`SELECT payload FROM macro_derived
		 WHERE metric = $1 AND source = 'macro_analysis' AND payload IS NOT NULL
		 ORDER BY ts DESC LIMIT 1`,
		metric)
	if err := row.Scan(&raw); err != nil || len(raw) == 0 {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	return m, true
}
