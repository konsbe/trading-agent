package store

import (
	"context"
	"fmt"
	"time"
)

// Read path for momentum-api's GET /api/v1/scanner/tracked (Tracked Positions
// addendum). Read-only; momentum-tracker is the only writer of momentum_tracked.

// TrackedStatusFilter is "active", "closed" or "all".
type TrackedStatusFilter string

// TrackedPositionRow is one momentum_tracked row with its display joins.
type TrackedPositionRow struct {
	Symbol      string
	Exchange    *string
	CompanyName *string
	Bucket      string
	Status      string
	AlertedTS   time.Time

	// SessionsElapsed is the tracker's own stored count — the value the exit
	// rules were evaluated against — as of LastEvaluatedTS.
	SessionsElapsed int
	LastEvaluatedTS *time.Time

	ReferencePrice float64
	// LatestClose is momentum_features.close for the latest scan date (as
	// traded, same source as reference_price). Nil when the symbol has no row
	// that day — stopped scanning, delisted — or when no scan exists.
	LatestClose *float64
	MaxGainPct  *float64

	ExitReason *string
	ExitTS     *time.Time
	ExitPrice  *float64
	ExitPct    *float64
}

// TrackedCounts are totals over the whole table, independent of any filter.
type TrackedCounts struct {
	Active int
	Closed int
}

// TrackedPositions lists rows most recently alerted first. latestScan is the
// scan date whose close is joined as the current price; pass nil when no scan
// exists and every LatestClose will be nil.
func TrackedPositions(ctx context.Context, q Querier, status TrackedStatusFilter, latestScan *time.Time) ([]TrackedPositionRow, error) {
	rows, err := q.Query(ctx, `
SELECT mt.symbol, u.exchange, u.name, mt.bucket, mt.status, mt.alerted_ts,
       mt.sessions_elapsed, mt.last_evaluated_ts, mt.reference_price, mf.close,
       mt.max_gain_pct, mt.exit_reason, mt.exit_ts, mt.exit_price, mt.exit_pct
FROM momentum_tracked mt
-- LEFT JOINs: a tracked row must never disappear because its symbol left the
-- directory or stopped appearing in scans.
LEFT JOIN universe_symbols u ON u.symbol = mt.symbol
LEFT JOIN momentum_features mf ON mf.symbol = mt.symbol AND mf.ts = $1
WHERE ($2 = 'all' OR mt.status = $2)
ORDER BY mt.alerted_ts DESC, mt.symbol`, latestScan, string(status))
	if err != nil {
		return nil, fmt.Errorf("tracked positions: %w", err)
	}
	defer rows.Close()
	out := []TrackedPositionRow{}
	for rows.Next() {
		var r TrackedPositionRow
		if err := rows.Scan(&r.Symbol, &r.Exchange, &r.CompanyName, &r.Bucket, &r.Status, &r.AlertedTS,
			&r.SessionsElapsed, &r.LastEvaluatedTS, &r.ReferencePrice, &r.LatestClose,
			&r.MaxGainPct, &r.ExitReason, &r.ExitTS, &r.ExitPrice, &r.ExitPct); err != nil {
			return nil, fmt.Errorf("scan tracked position: %w", err)
		}
		r.AlertedTS = r.AlertedTS.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

func GetTrackedCounts(ctx context.Context, q Querier) (TrackedCounts, error) {
	var c TrackedCounts
	if err := q.QueryRow(ctx, `
SELECT count(*) FILTER (WHERE status = 'active'), count(*) FILTER (WHERE status = 'closed')
FROM momentum_tracked`).Scan(&c.Active, &c.Closed); err != nil {
		return TrackedCounts{}, fmt.Errorf("tracked counts: %w", err)
	}
	return c, nil
}
