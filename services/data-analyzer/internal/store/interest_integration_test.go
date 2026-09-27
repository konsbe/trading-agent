//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// Reasons open from their sources and close when the source drops the symbol;
// closed rows are kept (history), manual rows are never touched by reconcile.
func TestReconcileInterest_OpensClosesAndKeepsHistory(t *testing.T) {
	ctx := context.Background()
	tx := fixtureTx(t)
	latest := time.Date(2099, 6, 2, 0, 0, 0, 0, time.UTC) // newest scan inside this tx
	if _, err := tx.Exec(ctx, `INSERT INTO universe_symbols (symbol, exchange, name, type) VALUES
		('ZZI1','NASDAQ','Cand','Common Stock'), ('ZZI2','NYSE','Watched Fund','ETP')`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO momentum_features (ts, symbol, bucket, gates_passed) VALUES ($1, 'ZZI1', 'market', true)`, latest); err != nil {
		t.Fatal(err)
	}
	if _, err := AddToWatchlist(ctx, tx, nil, "ZZI2"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO followed_symbols (symbol, asset_type, listing) VALUES ('ZZI3USDT', 'crypto', 'crypto');
		INSERT INTO computation_interest (symbol, asset_type, reason) VALUES ('ZZI4', 'equity', 'manual')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReconcileInterest(ctx, tx); err != nil {
		t.Fatal(err)
	}
	open := func() map[string]string {
		rows, err := tx.Query(ctx, `SELECT symbol || ':' || reason, asset_type FROM computation_interest
			WHERE symbol LIKE 'ZZI%' AND active_until IS NULL`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		m := map[string]string{}
		for rows.Next() {
			var k, v string
			_ = rows.Scan(&k, &v)
			m[k] = v
		}
		return m
	}
	got := open()
	want := map[string]string{"ZZI1:candidate": "equity", "ZZI2:watchlist": "etf", "ZZI3USDT:followed": "crypto", "ZZI4:manual": "equity"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("open %s = %q, want %q (all open: %v)", k, got[k], v, got)
		}
	}
	// Idempotent: a second run opens nothing new.
	if opened, closed, err := ReconcileInterest(ctx, tx); err != nil || opened != 0 || closed != 0 {
		t.Errorf("second reconcile opened %d closed %d err %v, want 0 0", opened, closed, err)
	}
	// Leaving the watchlist closes that reason; the row stays.
	if _, err := RemoveFromWatchlist(ctx, tx, nil, "ZZI2"); err != nil {
		t.Fatal(err)
	}
	if _, closed, err := ReconcileInterest(ctx, tx); err != nil || closed != 1 {
		t.Fatalf("reconcile after removal closed %d err %v, want 1", closed, err)
	}
	if _, still := open()["ZZI2:watchlist"]; still {
		t.Error("watchlist reason still open after removal")
	}
	var kept int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM computation_interest WHERE symbol='ZZI2' AND active_until IS NOT NULL`).Scan(&kept); err != nil || kept != 1 {
		t.Errorf("closed watchlist rows kept = %d, want 1 (history is never deleted)", kept)
	}
	if _, ok := open()["ZZI4:manual"]; !ok {
		t.Error("reconcile closed a manual reason")
	}
	set, err := ComputationSet(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]string{}
	for _, s := range set {
		have[s.Symbol] = s.AssetType
	}
	if have["ZZI1"] != "equity" || have["ZZI3USDT"] != "crypto" || have["ZZI4"] != "equity" || have["ZZI2"] != "" {
		t.Errorf("computation set = %v, want ZZI1, ZZI3USDT, ZZI4 and not ZZI2", have)
	}
}
