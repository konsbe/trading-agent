package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// GET /api/v1/alerts (docs/MOMENTUM_SCANNER_FULL_STOCK_ANALYSIS_API.md §3.2):
// read-only over fired_alerts (migration 026); the analyst bot's alert scan is
// the only writer.

// AlertRow is one fired_alerts row.
type AlertRow struct {
	ID           int64
	Symbol       string
	ExchangeType string
	AlertType    string
	Interval     string
	Value        *float64
	Severity     string
	Message      string
	FiredAt      time.Time
	// BarDate is the bar the onset happened on (migration 031); nil for rows
	// from before onset alerts, which re-posted an ongoing condition.
	BarDate *time.Time
}

// AlertFilter narrows ListAlerts / ListAlertGroups. Nil or empty fields do
// not filter; Limit must be > 0. Since is inclusive, Until exclusive. Before is
// a keyset cursor: rows (raw) or groups (grouped) strictly older than that
// alert id's (fired_at, id).
type AlertFilter struct {
	Symbol     *string
	AlertTypes []string
	Severities []string
	Since      *time.Time
	Until      *time.Time
	Before     *int64
	Limit      int
}

// where returns the row filter (everything but the cursor) and its args.
func (f AlertFilter) where() ([]string, []any) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Symbol != nil {
		add("symbol = $%d", *f.Symbol)
	}
	if len(f.AlertTypes) > 0 {
		add("alert_type = ANY($%d)", f.AlertTypes)
	}
	if len(f.Severities) > 0 {
		add("severity = ANY($%d)", f.Severities)
	}
	if f.Since != nil {
		add("fired_at >= $%d", *f.Since)
	}
	if f.Until != nil {
		add("fired_at < $%d", *f.Until)
	}
	return where, args
}

// cursorCond is the keyset condition "(ts, id) older than alert $n".
func cursorCond(tsCol, idCol string, n int) string {
	return fmt.Sprintf("(%s, %s) < (SELECT fired_at, id FROM fired_alerts WHERE id = $%d)", tsCol, idCol, n)
}

// ListAlerts returns fired_alerts rows newest first (ties by id, newest
// first), at most f.Limit.
func ListAlerts(ctx context.Context, q Querier, f AlertFilter) ([]AlertRow, error) {
	if f.Limit <= 0 {
		return nil, fmt.Errorf("list alerts: limit must be positive, got %d", f.Limit)
	}
	// Only the filters given are in the SQL, so each shape plans onto its index
	// (symbol, fired_at DESC) or (fired_at DESC).
	where, args := f.where()
	if f.Before != nil {
		args = append(args, *f.Before)
		where = append(where, cursorCond("fired_at", "id", len(args)))
	}
	args = append(args, f.Limit)
	rows, err := q.Query(ctx, `
SELECT id, symbol, exchange_type, alert_type, interval, value::float8, severity, message, fired_at, bar_date
FROM fired_alerts
WHERE `+strings.Join(where, " AND ")+`
ORDER BY fired_at DESC, id DESC
LIMIT `+fmt.Sprintf("$%d", len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()
	out := []AlertRow{}
	for rows.Next() {
		var a AlertRow
		if err := rows.Scan(&a.ID, &a.Symbol, &a.ExchangeType, &a.AlertType, &a.Interval,
			&a.Value, &a.Severity, &a.Message, &a.FiredAt, &a.BarDate); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		a.FiredAt = a.FiredAt.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// AlertGroup is every matching alert of one symbol and alert type: the bot
// re-posts an ongoing condition after each cooldown, so repeats are usually
// one condition, not separate events. Latest is the newest matching row.
type AlertGroup struct {
	Symbol       string
	AlertType    string
	Count        int
	FirstFiredAt time.Time
	LastFiredAt  time.Time
	Latest       AlertRow
}

// ListAlertGroups groups the rows ListAlerts would return (same filters) by
// symbol and alert type, newest group first (by its latest row), at most
// f.Limit groups. Before pages by a group's latest alert id.
func ListAlertGroups(ctx context.Context, q Querier, f AlertFilter) ([]AlertGroup, error) {
	if f.Limit <= 0 {
		return nil, fmt.Errorf("list alert groups: limit must be positive, got %d", f.Limit)
	}
	where, args := f.where()
	outer := []string{"true"}
	if f.Before != nil {
		args = append(args, *f.Before)
		outer = append(outer, cursorCond("l.fired_at", "l.id", len(args)))
	}
	args = append(args, f.Limit)
	rows, err := q.Query(ctx, `
WITH m AS (
    SELECT * FROM fired_alerts WHERE `+strings.Join(where, " AND ")+`
), g AS (
    SELECT symbol, alert_type, count(*) AS n, min(fired_at) AS first_at, max(fired_at) AS last_at
    FROM m GROUP BY symbol, alert_type
), l AS (
    SELECT DISTINCT ON (symbol, alert_type) *
    FROM m ORDER BY symbol, alert_type, fired_at DESC, id DESC
)
SELECT g.symbol, g.alert_type, g.n, g.first_at, g.last_at,
       l.id, l.exchange_type, l.interval, l.value::float8, l.severity, l.message, l.fired_at, l.bar_date
FROM g JOIN l USING (symbol, alert_type)
WHERE `+strings.Join(outer, " AND ")+`
ORDER BY l.fired_at DESC, l.id DESC
LIMIT `+fmt.Sprintf("$%d", len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list alert groups: %w", err)
	}
	defer rows.Close()
	out := []AlertGroup{}
	for rows.Next() {
		var g AlertGroup
		a := &g.Latest
		if err := rows.Scan(&g.Symbol, &g.AlertType, &g.Count, &g.FirstFiredAt, &g.LastFiredAt,
			&a.ID, &a.ExchangeType, &a.Interval, &a.Value, &a.Severity, &a.Message, &a.FiredAt, &a.BarDate); err != nil {
			return nil, fmt.Errorf("scan alert group: %w", err)
		}
		a.Symbol, a.AlertType = g.Symbol, g.AlertType
		g.FirstFiredAt, g.LastFiredAt, a.FiredAt = g.FirstFiredAt.UTC(), g.LastFiredAt.UTC(), a.FiredAt.UTC()
		out = append(out, g)
	}
	return out, rows.Err()
}

// AlertMeta is table-wide: the alert types that have ever fired (for the
// page's filter) and the earliest record (the history starts there — earlier
// periods have no record, which is not the same as no alerts).
type AlertMeta struct {
	Types         []string
	EarliestFired *time.Time
	// OnsetsSince is the first onset alert (the first row with a bar_date):
	// before it, rows re-posted ongoing conditions. Nil until one exists.
	OnsetsSince *time.Time
}

func LoadAlertMeta(ctx context.Context, q Querier) (AlertMeta, error) {
	var m AlertMeta
	if err := q.QueryRow(ctx, `
SELECT COALESCE(array_agg(DISTINCT alert_type ORDER BY alert_type), '{}'), min(fired_at),
       min(fired_at) FILTER (WHERE bar_date IS NOT NULL)
FROM fired_alerts`).Scan(&m.Types, &m.EarliestFired, &m.OnsetsSince); err != nil {
		return m, fmt.Errorf("alert meta: %w", err)
	}
	for _, p := range []**time.Time{&m.EarliestFired, &m.OnsetsSince} {
		if *p != nil {
			t := (*p).UTC()
			*p = &t
		}
	}
	return m, nil
}
