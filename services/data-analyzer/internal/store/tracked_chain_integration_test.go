//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

func TestLoadTrackerChain(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	s1 := time.Date(2099, 1, 5, 0, 0, 0, 0, time.UTC)
	s2 := time.Date(2099, 1, 6, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `INSERT INTO momentum_chain_runs (session, attempts, scanner_completed_at, tracker_completed_at) VALUES
		($1::date, 1, $1::date + interval '1 day', $1::date + interval '1 day 1 minute'),
		($2::date, 1, $2::date + interval '1 day', NULL)`, s1, s2); err != nil {
		t.Fatal(err)
	}
	c, err := LoadTrackerChain(ctx, tx, &s2)
	if err != nil {
		t.Fatal(err)
	}
	if c.LatestScanRun == nil || c.LatestScanRun.ScannerCompletedAt == nil || c.LatestScanRun.TrackerCompletedAt != nil {
		t.Errorf("latest scan run = %+v", c.LatestScanRun)
	}
	if c.LastTrackedSession == nil || !c.LastTrackedSession.Equal(s1) {
		t.Errorf("last tracked = %v, want %v", c.LastTrackedSession, s1)
	}
	none := time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC)
	if c, err := LoadTrackerChain(ctx, tx, &none); err != nil || c.LatestScanRun != nil {
		t.Errorf("session without a chain row = %+v, %v", c.LatestScanRun, err)
	}
}
