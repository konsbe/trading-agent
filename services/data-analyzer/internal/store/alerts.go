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
}

// AlertFilter narrows ListAlerts. Nil fields do not filter; Limit must be > 0.
type AlertFilter struct {
	Symbol *string
	Since  *time.Time
	Limit  int
}

// ListAlerts returns fired_alerts rows newest first (ties by id, newest
// first), at most f.Limit.
func ListAlerts(ctx context.Context, q Querier, f AlertFilter) ([]AlertRow, error) {
	if f.Limit <= 0 {
		return nil, fmt.Errorf("list alerts: limit must be positive, got %d", f.Limit)
	}
	// Only the filters given are in the SQL, so each shape plans onto its index
	// (symbol, fired_at DESC) or (fired_at DESC).
	where, args := []string{"true"}, []any{}
	if f.Symbol != nil {
		args = append(args, *f.Symbol)
		where = append(where, fmt.Sprintf("symbol = $%d", len(args)))
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		where = append(where, fmt.Sprintf("fired_at >= $%d", len(args)))
	}
	args = append(args, f.Limit)
	rows, err := q.Query(ctx, `
SELECT id, symbol, exchange_type, alert_type, interval, value::float8, severity, message, fired_at
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
			&a.Value, &a.Severity, &a.Message, &a.FiredAt); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		a.FiredAt = a.FiredAt.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}
