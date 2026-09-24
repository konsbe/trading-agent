//go:build integration

package store

import (
	"context"
	"testing"
	"time"

	"github.com/konsbe/trading-agent/services/data-analyzer/internal/momentum"
)

func scanRow(ts time.Time, sym string, close float64) FeatureRow {
	c := close
	return FeatureRow{TS: ts, Symbol: sym, Features: &momentum.Features{Close: &c}}
}

// The kill-mid-scan case: a scan that fails part-way must leave NOTHING — no
// feature rows (which would move max(ts) and make a partial scan look fresh)
// and no completion marker. A killed process is the same as this failure from
// Postgres's side: the open transaction is rolled back.
func TestWriteScan_FailurePartWayCommitsNothing(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	session := time.Date(2099, 3, 2, 0, 0, 0, 0, time.UTC)

	rows := []ScanWrite{
		{Feature: scanRow(session, "ZZS01", 1.5)},
		{Feature: scanRow(session, "ZZS02", 2.5)},
		{Feature: FeatureRow{TS: session, Symbol: "ZZS03"}}, // nil features: fails mid-scan
	}
	if err := WriteScan(ctx, tx, session, rows); err == nil {
		t.Fatal("want an error from the bad row")
	}
	var feats, marks int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM momentum_features WHERE ts = $1`, session).Scan(&feats); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM momentum_chain_runs WHERE session = $1`, session).Scan(&marks); err != nil {
		t.Fatal(err)
	}
	if feats != 0 || marks != 0 {
		t.Fatalf("partial scan leaked: %d feature rows, %d markers (want 0, 0)", feats, marks)
	}

	// The retry commits everything, marker included.
	if err := WriteScan(ctx, tx, session, rows[:2]); err != nil {
		t.Fatal(err)
	}
	r, ok, err := LoadChainRun(ctx, tx, session)
	if err != nil || !ok || r.ScannerCompletedAt == nil || r.TrackerCompletedAt != nil {
		t.Fatalf("after a clean scan: row=%+v ok=%v err=%v", r, ok, err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM momentum_features WHERE ts = $1`, session).Scan(&feats); err != nil || feats != 2 {
		t.Fatalf("feature rows = %d, err %v; want 2", feats, err)
	}
}

// Re-running a session replaces its candidate set: a symbol that no longer
// passes loses the score an earlier run wrote for the same bar.
func TestWriteScan_RerunClearsStaleScores(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	session := time.Date(2099, 3, 3, 0, 0, 0, 0, time.UTC)
	score := momentum.Score{Bucket: momentum.BucketPenny, Total: 40}

	first := []ScanWrite{{Feature: scanRow(session, "ZZS11", 1.2), Score: &score}}
	if err := WriteScan(ctx, tx, session, first); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM momentum_scores WHERE ts = $1 AND symbol = 'ZZS11'`, session).Scan(&n); err != nil || n != 1 {
		t.Fatalf("first run: %d scores, err %v; want 1", n, err)
	}
	rerun := []ScanWrite{{Feature: scanRow(session, "ZZS11", 1.2)}} // no longer passes
	if err := WriteScan(ctx, tx, session, rerun); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM momentum_scores WHERE ts = $1 AND symbol = 'ZZS11'`, session).Scan(&n); err != nil || n != 0 {
		t.Fatalf("re-run kept a stale score: %d, err %v; want 0", n, err)
	}
}

func TestChainRunAttemptsAndGiveUpPersist(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	session := time.Date(2099, 3, 4, 0, 0, 0, 0, time.UTC)

	if _, ok, err := LoadChainRun(ctx, tx, session); err != nil || ok {
		t.Fatalf("fresh session: ok=%v err=%v; want no row", ok, err)
	}
	for want := 1; want <= 3; want++ {
		got, err := BeginChainAttempt(ctx, tx, session)
		if err != nil || got != want {
			t.Fatalf("attempt %d: got %d, err %v", want, got, err)
		}
	}
	if err := RecordChainError(ctx, tx, session, "momentum-tracker: exit status 1"); err != nil {
		t.Fatal(err)
	}
	if err := MarkChainGaveUp(ctx, tx, session, "chain failed 3 times"); err != nil {
		t.Fatal(err)
	}
	r, ok, err := LoadChainRun(ctx, tx, session)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if r.Attempts != 3 || r.GaveUpAt == nil || r.LastError == nil || *r.LastError != "chain failed 3 times" {
		t.Fatalf("persisted state wrong: %+v", r)
	}
	if err := MarkTrackerCompleted(ctx, tx, session); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := LoadChainRun(ctx, tx, session); r.Attempts != 3 || r.TrackerCompletedAt == nil {
		t.Fatalf("tracker marker must not reset attempts: %+v", r)
	}
}
