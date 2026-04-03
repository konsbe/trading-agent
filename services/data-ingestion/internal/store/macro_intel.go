package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UpsertEconomicCalendarEvent inserts or updates one economic calendar row.
func UpsertEconomicCalendarEvent(ctx context.Context, pool *pgxpool.Pool,
	eventTS time.Time, country, eventName, impact string,
	actual, estimate, previous *float64, unit, source, externalID string, payload any,
) error {
	var jb []byte
	var err error
	if payload != nil {
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	const q = `
INSERT INTO economic_calendar_events (
  event_ts, country, event_name, impact, actual, estimate, previous, unit, source, external_id, payload
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (source, external_id) DO UPDATE SET
  event_ts = EXCLUDED.event_ts,
  country = EXCLUDED.country,
  event_name = EXCLUDED.event_name,
  impact = EXCLUDED.impact,
  actual = EXCLUDED.actual,
  estimate = EXCLUDED.estimate,
  previous = EXCLUDED.previous,
  unit = EXCLUDED.unit,
  ingested_at = now(),
  payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, eventTS, country, eventName, impact, actual, estimate, previous, unit, source, externalID, jb)
	return err
}

// UpsertEarningsCalendarEvent inserts or updates one earnings calendar row.
func UpsertEarningsCalendarEvent(ctx context.Context, pool *pgxpool.Pool,
	earningsDate time.Time, symbol, hour string, year *int, quarter string,
	epsEst, epsAct, revEst, revAct *float64, source, externalID string, payload any,
) error {
	var jb []byte
	var err error
	if payload != nil {
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	const q = `
INSERT INTO earnings_calendar_events (
  earnings_date, symbol, hour, year, quarter,
  eps_estimate, eps_actual, revenue_estimate, revenue_actual, source, external_id, payload
) VALUES ($1::date,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (source, external_id) DO UPDATE SET
  hour = EXCLUDED.hour,
  year = EXCLUDED.year,
  quarter = EXCLUDED.quarter,
  eps_estimate = EXCLUDED.eps_estimate,
  eps_actual = EXCLUDED.eps_actual,
  revenue_estimate = EXCLUDED.revenue_estimate,
  revenue_actual = EXCLUDED.revenue_actual,
  ingested_at = now(),
  payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, earningsDate.Format("2006-01-02"), symbol, hour, year, quarter,
		epsEst, epsAct, revEst, revAct, source, externalID, jb)
	return err
}

// UpsertGPRMonthly stores one GPR monthly observation.
func UpsertGPRMonthly(ctx context.Context, pool *pgxpool.Pool, monthTS time.Time,
	gprTotal, gprAct, gprThreat *float64, source string, payload any,
) error {
	var jb []byte
	var err error
	if payload != nil {
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	const q = `
INSERT INTO geopolitical_risk_monthly (month_ts, gpr_total, gpr_act, gpr_threat, source, payload)
VALUES ($1::date,$2,$3,$4,$5,$6)
ON CONFLICT (month_ts, source) DO UPDATE SET
  gpr_total = EXCLUDED.gpr_total,
  gpr_act = EXCLUDED.gpr_act,
  gpr_threat = EXCLUDED.gpr_threat,
  ingested_at = now(),
  payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, monthTS.Format("2006-01-02"), gprTotal, gprAct, gprThreat, source, jb)
	return err
}

// UpsertGDELTDaily stores one GDELT daily aggregate row.
func UpsertGDELTDaily(ctx context.Context, pool *pgxpool.Pool, dayTS time.Time, queryLabel string,
	articleCount int, avgTone, avgGoldstein *float64, payload any,
) error {
	var jb []byte
	var err error
	if payload != nil {
		jb, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	const q = `
INSERT INTO gdelt_macro_daily (day_ts, query_label, article_count, avg_tone, avg_goldstein, payload)
VALUES ($1::date,$2,$3,$4,$5,$6)
ON CONFLICT (day_ts, query_label) DO UPDATE SET
  article_count = EXCLUDED.article_count,
  avg_tone = EXCLUDED.avg_tone,
  avg_goldstein = EXCLUDED.avg_goldstein,
  ingested_at = now(),
  payload = EXCLUDED.payload
`
	_, err = pool.Exec(ctx, q, dayTS.Format("2006-01-02"), queryLabel, articleCount, avgTone, avgGoldstein, jb)
	return err
}
