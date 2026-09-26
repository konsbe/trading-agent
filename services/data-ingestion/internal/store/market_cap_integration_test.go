//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

//	go test -tags=integration ./internal/store/ -run 'NonUSD|ConfiguredSymbols' -v

// Consumers read the newest NON-NULL market_cap, so the repair has to null the
// old rows too; a NULL written only going forward would let the local-currency
// figure resurface.
func TestNullNonUSDMarketCapHistory_NullsOldRowsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	mustExec(t, pool, `DELETE FROM equity_fundamentals WHERE symbol IN ('ZZTWD','ZZUSD')`)
	t.Cleanup(func() {
		mustExec(t, pool, `DELETE FROM equity_fundamentals WHERE symbol IN ('ZZTWD','ZZUSD')`)
	})

	old := time.Now().UTC().Add(-48 * time.Hour)
	for _, sym := range []string{"ZZTWD", "ZZUSD"} {
		mustExec(t, pool, `
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source)
VALUES ($1, $2, 'ttm', 'market_cap', 62885996000000, 'finnhub_metric')`, old, sym)
	}

	reason := map[string]any{"currency": "TWD", "note": "nulled"}
	n, err := NullNonUSDMarketCapHistory(ctx, pool, "ZZTWD", reason)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("repaired %d rows, want 1", n)
	}

	v, err := LatestFundamental(ctx, pool, "ZZTWD", "market_cap")
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("latest non-null market_cap = %g, want none left", *v)
	}
	p, err := LatestFundamentalPayload(ctx, pool, "ZZTWD", "market_cap")
	if err != nil {
		t.Fatal(err)
	}
	if p["currency"] != "TWD" || p["market_cap_millions_local"] != 62885996.0 {
		t.Errorf("payload = %v, want currency TWD and the local figure in millions", p)
	}

	if n, _ := NullNonUSDMarketCapHistory(ctx, pool, "ZZTWD", reason); n != 0 {
		t.Errorf("second run repaired %d rows, want 0", n)
	}
	if v, _ := LatestFundamental(ctx, pool, "ZZUSD", "market_cap"); v == nil {
		t.Error("another symbol's market_cap was touched")
	}
}

// TSM is an ADR, so §3.1 excludes it from the universe; seeding only the
// universe left it with no checkpoint row and it was never refreshed.
func TestSeedFundamentalFetchStateSymbols_SeedsConfiguredSymbolsOutsideTheUniverse(t *testing.T) {
	setupFetchState(t, []string{"AAA"})
	ctx := context.Background()
	pool := testPool(t)

	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil {
		t.Fatal(err)
	}
	n, err := SeedFundamentalFetchStateSymbols(ctx, pool, TaskMetrics, []string{"TSM", "AAA", "", "SPY"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("seeded %d, want 2 (TSM, SPY; AAA already present, blank skipped)", n)
	}
	var got int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM fundamental_fetch_state
WHERE task = $1 AND symbol IN ('TSM','SPY') AND status = 'pending'`, TaskMetrics).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Errorf("pending configured rows = %d, want 2", got)
	}
}
