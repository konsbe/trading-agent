package store

import (
	"context"
	"fmt"
	"time"
)

// TrackerChain is what Tracked Positions' freshness banner needs from
// momentum_chain_runs (read-only; the chain itself writes it): the chain row of
// the latest scanned session, and the latest session the tracker completed.
type TrackerChain struct {
	LatestScanRun      *ChainRun  // nil when that session has no chain row
	LastTrackedSession *time.Time // nil when the tracker never completed
}

func LoadTrackerChain(ctx context.Context, q Querier, lastScan *time.Time) (TrackerChain, error) {
	var c TrackerChain
	if lastScan != nil {
		run, ok, err := LoadChainRun(ctx, q, *lastScan)
		if err != nil {
			return c, err
		}
		if ok {
			c.LatestScanRun = &run
		}
	}
	if err := q.QueryRow(ctx, `
SELECT max(session) FROM momentum_chain_runs WHERE tracker_completed_at IS NOT NULL`).Scan(&c.LastTrackedSession); err != nil {
		return c, fmt.Errorf("last tracked session: %w", err)
	}
	return c, nil
}
