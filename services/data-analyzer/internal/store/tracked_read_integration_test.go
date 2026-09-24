//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// Tracked Positions read path against a real Postgres, inside a rolled-back
// transaction (see fixtureTx).
func TestTrackedPositions_JoinsFiltersAndOrder(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)

	var baseActive, baseClosed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='active'), count(*) FILTER (WHERE status='closed') FROM momentum_tracked`).
		Scan(&baseActive, &baseClosed); err != nil {
		t.Fatal(err)
	}

	scan := time.Date(2099, 2, 3, 0, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `INSERT INTO universe_symbols (symbol, exchange, name) VALUES ('ZZT01','NASDAQ','Tracked One')`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO momentum_features (ts, symbol, close) VALUES ($1, 'ZZT01', 1.71)`, scan); err != nil {
		t.Fatal(err)
	}
	insert := func(sym, status string, alerted time.Time, reason *string, exitPct *float64) {
		t.Helper()
		if _, err := tx.Exec(ctx, `
INSERT INTO momentum_tracked (symbol, alerted_ts, bucket, status, reference_price, sessions_elapsed,
                              max_gain_pct, exit_reason, exit_ts, exit_price, exit_pct)
VALUES ($1, $2::timestamptz, 'penny', $3, 1.64, 2, 9.15, $4, CASE WHEN $4::text IS NULL THEN NULL ELSE $2::timestamptz + interval '1 day' END,
        CASE WHEN $4::text IS NULL THEN NULL ELSE 1.5 END, $5)`,
			sym, alerted, status, reason, exitPct); err != nil {
			t.Fatalf("insert %s: %v", sym, err)
		}
	}
	day := func(d int) time.Time { return time.Date(2099, 1, d, 0, 0, 0, 0, time.UTC) }
	insert("ZZT01", "active", day(20), nil, nil) // join hit
	insert("ZZT02", "active", day(21), nil, nil) // not in universe, no features row: join misses
	for i, r := range []string{"breakout_failed", "lost_vwap", "momentum_stalled", "stop_atr"} {
		reason, pct := r, -5.0
		insert("ZZC0"+string(rune('1'+i)), "closed", day(10+i), &reason, &pct)
	}

	active, err := TrackedPositions(ctx, tx, "active", &scan)
	if err != nil {
		t.Fatal(err)
	}
	mine := func(rows []TrackedPositionRow) []TrackedPositionRow {
		var out []TrackedPositionRow
		for _, r := range rows {
			if len(r.Symbol) == 5 && r.Symbol[:2] == "ZZ" {
				out = append(out, r)
			}
		}
		return out
	}
	a := mine(active)
	if len(a) != 2 || a[0].Symbol != "ZZT02" || a[1].Symbol != "ZZT01" {
		t.Fatalf("active = %+v, want ZZT02 then ZZT01 (most recently alerted first)", a)
	}
	if a[1].LatestClose == nil || *a[1].LatestClose != 1.71 || a[1].CompanyName == nil || *a[1].CompanyName != "Tracked One" {
		t.Errorf("ZZT01 joins = %+v", a[1])
	}
	if a[0].LatestClose != nil || a[0].Exchange != nil {
		t.Errorf("ZZT02 should keep its row with null joins: %+v", a[0])
	}
	if a[1].SessionsElapsed != 2 || a[1].MaxGainPct == nil || *a[1].MaxGainPct != 9.15 {
		t.Errorf("stored fields = %+v", a[1])
	}

	closed, err := TrackedPositions(ctx, tx, "closed", &scan)
	if err != nil {
		t.Fatal(err)
	}
	c := mine(closed)
	if len(c) != 4 {
		t.Fatalf("closed = %d fixture rows, want 4", len(c))
	}
	for _, r := range c {
		if r.ExitReason == nil || r.ExitTS == nil || r.ExitPct == nil || *r.ExitPct != -5 {
			t.Errorf("closed row %s = %+v", r.Symbol, r)
		}
	}
	all, _ := TrackedPositions(ctx, tx, "all", nil)
	if n := len(mine(all)); n != 6 {
		t.Errorf("all = %d fixture rows, want 6", n)
	}
	for _, r := range mine(all) {
		if r.LatestClose != nil {
			t.Errorf("no scan date: %s joined a price", r.Symbol)
		}
	}

	counts, err := GetTrackedCounts(ctx, tx)
	if err != nil || counts.Active != baseActive+2 || counts.Closed != baseClosed+4 {
		t.Errorf("counts = %+v err %v, want base+2 / base+4", counts, err)
	}
}
