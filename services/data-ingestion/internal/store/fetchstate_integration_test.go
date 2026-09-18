//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// Integration coverage for step 3b: the union symbol resolver and the
// per-(symbol, task) fetch checkpoint.
//
//	go test -tags=integration ./internal/store/ -run 'Metrics|FetchState|Fundamental' -v

const (
	ffsRefresh     = 168 * time.Hour
	ffsLease       = 30 * time.Minute
	ffsMaxAttempts = 3
)

// setupFetchState clears both tables and seeds an eligible universe.
func setupFetchState(t *testing.T, eligible []string) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	if _, err := pool.Exec(ctx, `DELETE FROM fundamental_fetch_state`); err != nil {
		t.Fatalf("clear fetch state: %v", err)
	}
	clearUniverse(t, pool)
	rows := make([]UniverseRow, 0, len(eligible))
	for _, s := range eligible {
		rows = append(rows, UniverseRow{
			Symbol: s, Exchange: "NASDAQ", MIC: "XNAS",
			Type: "Common Stock", IsEligible: true,
		})
	}
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatalf("seed universe: %v", err)
	}
}

// §8.4's load-bearing property: the configured list is UNIONED with the
// universe, never replaced by it. SPY is the case that matters — an ETF, so
// §3.1 guarantees it is absent from universe_symbols, and replacement would
// silently drop the most visible symbol in the existing daily report.
func TestResolveMetricsSymbols_UnionsAndPreservesConfiguredETFs(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"AAPL", "MSFT", "TINYCO"})
	pool := testPool(t)

	configured := []string{"AAPL", "MSFT", "SPY"} // SPY is an ETF, never eligible
	got, err := ResolveMetricsSymbols(ctx, pool, configured, ScopeEligible)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	set := map[string]int{}
	for _, s := range got {
		set[s]++
	}

	// Every configured symbol survives, including the one the universe excludes.
	for _, s := range configured {
		if set[s] == 0 {
			t.Errorf("configured symbol %q was dropped — replacement instead of union", s)
		}
	}
	if set["SPY"] == 0 {
		t.Error("SPY dropped: this is the regression the union exists to prevent")
	}
	// Universe-only symbols are added.
	if set["TINYCO"] == 0 {
		t.Error("universe symbol TINYCO missing from the union")
	}
	// Overlap is not duplicated — a duplicate would double-spend the shared
	// Finnhub budget on the same symbol.
	for s, n := range set {
		if n != 1 {
			t.Errorf("symbol %q appears %d times, want 1", s, n)
		}
	}
	// Configured symbols come first, so a partial pass refreshes existing
	// consumers before the long universe tail.
	if len(got) < 3 || got[0] != "AAPL" || got[1] != "MSFT" || got[2] != "SPY" {
		t.Errorf("configured symbols should lead the list, got %v", got[:min(4, len(got))])
	}
}

// An empty universe must degrade to the configured list, never to an empty pass.
func TestResolveMetricsSymbols_EmptyUniverseKeepsConfiguredList(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, nil)
	pool := testPool(t)

	got, err := ResolveMetricsSymbols(ctx, pool, []string{"AAPL", "MSFT", "SPY"}, ScopeEligible)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d symbols, want the 3 configured ones: %v", len(got), got)
	}
}

func TestSeedFundamentalFetchState_IsIdempotentAndPicksUpNewSymbols(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"AAA", "BBB"})
	pool := testPool(t)

	n, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("seeded %d, want 2", n)
	}
	// Re-seeding must add nothing.
	if n, err = SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil || n != 0 {
		t.Errorf("re-seed added %d rows (err %v), want 0", n, err)
	}

	// A symbol already fetched must keep its state when a new one is seeded.
	if err := MarkFundamentalFetchDone(ctx, pool, "AAA", TaskMetrics); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "CCC", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if n, err = SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil || n != 1 {
		t.Errorf("seeded %d for the new symbol (err %v), want 1", n, err)
	}

	var status string
	var success *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT status, last_success_ts FROM fundamental_fetch_state WHERE symbol='AAA' AND task=$1`,
		TaskMetrics).Scan(&status, &success); err != nil {
		t.Fatal(err)
	}
	if status != FetchDone || success == nil {
		t.Errorf("re-seeding clobbered existing state: status=%q success=%v", status, success)
	}
}

func TestClaimFundamentalFetch_LeasesPendingFirstAndSkipsFresh(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"FRESH", "STALE", "NEW"})
	pool := testPool(t)
	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil {
		t.Fatal(err)
	}

	// FRESH succeeded just now; STALE succeeded long ago; NEW is untouched.
	if err := MarkFundamentalFetchDone(ctx, pool, "FRESH", TaskMetrics); err != nil {
		t.Fatal(err)
	}
	if err := MarkFundamentalFetchDone(ctx, pool, "STALE", TaskMetrics); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE fundamental_fetch_state SET last_success_ts = now() - interval '30 days'
WHERE symbol='STALE' AND task=$1`, TaskMetrics); err != nil {
		t.Fatal(err)
	}

	claims, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range claims {
		got[c.Symbol] = true
	}
	if !got["NEW"] {
		t.Error("a pending symbol must be claimable")
	}
	if !got["STALE"] {
		t.Error("a symbol past the refresh interval must be claimable — this is what drives the weekly cadence")
	}
	if got["FRESH"] {
		t.Error("a freshly-succeeded symbol must not be re-claimed inside the refresh interval")
	}
	// Pending is prioritised over stale, so a first pass covers new ground before
	// re-walking what already has data.
	if len(claims) > 0 && claims[0].Symbol != "NEW" {
		t.Errorf("first claim = %q, want NEW (pending is highest priority)", claims[0].Symbol)
	}
}

// The resumability property, at the sub-task level: a worker killed mid-symbol
// leaves the row in_progress, and it must become claimable once the lease lapses.
func TestClaimFundamentalFetch_ReclaimsAbandonedClaims(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"STUCK"})
	pool := testPool(t)
	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil {
		t.Fatal(err)
	}

	claims, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil || len(claims) != 1 {
		t.Fatalf("initial claim: %v (n=%d)", err, len(claims))
	}

	// Inside the lease: not reclaimable, or two workers would duplicate work.
	again, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("reclaimed inside the lease (n=%d)", len(again))
	}

	if _, err := pool.Exec(ctx, `
UPDATE fundamental_fetch_state SET claimed_at = now() - interval '2 hours'
WHERE symbol='STUCK' AND task=$1`, TaskMetrics); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil {
		t.Fatal(err)
	}
	if len(reclaimed) != 1 || reclaimed[0].Symbol != "STUCK" {
		t.Fatalf("expected STUCK reclaimed after the lease expired, got %+v", reclaimed)
	}
}

// A failure must not advance last_success_ts, or a persistently broken symbol
// would look fresh, drop out of the rotation, and hide its own staleness.
func TestMarkFundamentalFetchFailed_DoesNotAdvanceFreshness(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"BROKEN"})
	pool := testPool(t)
	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= ffsMaxAttempts; i++ {
		claims, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
		if err != nil {
			t.Fatal(err)
		}
		if len(claims) != 1 {
			t.Fatalf("attempt %d: claims = %d, want 1", i, len(claims))
		}
		if err := MarkFundamentalFetchFailed(ctx, pool, "BROKEN", TaskMetrics, "HTTP 502"); err != nil {
			t.Fatal(err)
		}
	}

	var status, lastErr string
	var attempts int
	var success *time.Time
	if err := pool.QueryRow(ctx, `
SELECT status, attempts, last_error, last_success_ts
FROM fundamental_fetch_state WHERE symbol='BROKEN' AND task=$1`, TaskMetrics,
	).Scan(&status, &attempts, &lastErr, &success); err != nil {
		t.Fatal(err)
	}
	if status != FetchFailed || attempts != ffsMaxAttempts {
		t.Errorf("status=%q attempts=%d, want failed/%d", status, attempts, ffsMaxAttempts)
	}
	if success != nil {
		t.Error("last_success_ts advanced on failure — a broken symbol must never look fresh")
	}
	if lastErr != "HTTP 502" {
		t.Errorf("last_error = %q, want the persisted reason", lastErr)
	}

	// Attempts exhausted: no longer claimed, and surfaced as needing attention.
	none, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("claimed past max attempts (n=%d)", len(none))
	}
	prog, err := LoadFundamentalFetchProgress(ctx, pool, TaskMetrics, ffsMaxAttempts, ffsRefresh)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Exhausted != 1 {
		t.Errorf("exhausted = %d, want 1", prog.Exhausted)
	}
	if prog.Fresh != 0 {
		t.Errorf("fresh = %d, want 0 — 'fresh' must mean data is current, not that we tried", prog.Fresh)
	}
}

func TestMarkFundamentalFetchDone_ClearsClaimAndError(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"GOOD"})
	pool := testPool(t)
	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if err := MarkFundamentalFetchFailed(ctx, pool, "GOOD", TaskMetrics, "transient"); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts); err != nil {
		t.Fatal(err)
	}
	if err := MarkFundamentalFetchDone(ctx, pool, "GOOD", TaskMetrics); err != nil {
		t.Fatal(err)
	}

	var status string
	var claimed *time.Time
	var lastErr *string
	var success *time.Time
	if err := pool.QueryRow(ctx, `
SELECT status, claimed_at, last_error, last_success_ts
FROM fundamental_fetch_state WHERE symbol='GOOD' AND task=$1`, TaskMetrics,
	).Scan(&status, &claimed, &lastErr, &success); err != nil {
		t.Fatal(err)
	}
	if status != FetchDone {
		t.Errorf("status = %q, want done", status)
	}
	if claimed != nil {
		t.Error("claimed_at must be cleared on success, or the lease logic sees a stale claim")
	}
	if lastErr != nil {
		t.Errorf("last_error = %q, want cleared after a successful retry", *lastErr)
	}
	if success == nil {
		t.Error("last_success_ts must advance on success")
	}
}

// Leases are per (symbol, task): a second task for the same symbol is
// independent, so a failure in one never forces the other to redo its work.
func TestFetchState_TasksAreIndependentPerSymbol(t *testing.T) {
	ctx := context.Background()
	setupFetchState(t, []string{"MULTI"})
	pool := testPool(t)
	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible); err != nil {
		t.Fatal(err)
	}
	// A hypothetical second widened task, as §8.4 anticipates for runOverview.
	if _, err := pool.Exec(ctx,
		`INSERT INTO fundamental_fetch_state (symbol, task) VALUES ('MULTI','overview')
		 ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}

	if err := MarkFundamentalFetchDone(ctx, pool, "MULTI", TaskMetrics); err != nil {
		t.Fatal(err)
	}
	if err := MarkFundamentalFetchFailed(ctx, pool, "MULTI", "overview", "AV quota"); err != nil {
		t.Fatal(err)
	}

	// The metrics row stays done and fresh; only overview is claimable.
	claims, err := ClaimFundamentalFetchBatch(ctx, pool, TaskMetrics, 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 0 {
		t.Errorf("metrics re-claimed after a failure in a different task (n=%d) — leases must be per (symbol, task)", len(claims))
	}
	ovClaims, err := ClaimFundamentalFetchBatch(ctx, pool, "overview", 10, ffsRefresh, ffsLease, ffsMaxAttempts)
	if err != nil {
		t.Fatal(err)
	}
	if len(ovClaims) != 1 {
		t.Errorf("overview claims = %d, want 1", len(ovClaims))
	}
}

// The scope knob has to restrict BOTH halves of the fundamentals pass or it is
// cosmetic: scoping the resolver while seeding the full universe would leave
// ~4,500 permanently-pending checkpoint rows that the checkpointed pass would
// then work through anyway.
func TestMetricsScope_RestrictsBothResolutionAndSeeding(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	rows := []UniverseRow{
		{Symbol: "SELA", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "SELB", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "WIDEA", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "WIDEB", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}
	if _, err := UpsertUniverseSymbols(ctx, pool, rows); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE universe_symbols SET backfill_selected = true WHERE symbol IN ('SELA','SELB')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM fundamental_fetch_state`); err != nil {
		t.Fatal(err)
	}

	// Resolution.
	sel, err := ResolveMetricsSymbols(ctx, pool, nil, ScopeSelected)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel) != 2 {
		t.Errorf("ScopeSelected resolved %d symbols (%v), want the 2 selected", len(sel), sel)
	}
	wide, err := ResolveMetricsSymbols(ctx, pool, nil, ScopeEligible)
	if err != nil {
		t.Fatal(err)
	}
	if len(wide) != 4 {
		t.Errorf("ScopeEligible resolved %d symbols (%v), want all 4", len(wide), wide)
	}

	// Configured symbols survive regardless of scope: they are other consumers'
	// watchlists, and a pilot flag must not narrow what already works.
	withCfg, err := ResolveMetricsSymbols(ctx, pool, []string{"SPY", "QQQ"}, ScopeSelected)
	if err != nil {
		t.Fatal(err)
	}
	if len(withCfg) != 4 {
		t.Errorf("got %d (%v), want 2 configured + 2 selected", len(withCfg), withCfg)
	}
	if withCfg[0] != "SPY" || withCfg[1] != "QQQ" {
		t.Errorf("configured symbols must keep their leading order, got %v", withCfg)
	}

	// Seeding.
	n, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeSelected)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("ScopeSelected seeded %d rows, want 2 — an unscoped seed makes the scope setting cosmetic", n)
	}

	// Widening later must top up rather than duplicate.
	n, err = SeedFundamentalFetchState(ctx, pool, TaskMetrics, ScopeEligible)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("widening seeded %d additional rows, want the 2 not already present", n)
	}
	var total int
	pool.QueryRow(ctx, `SELECT count(*) FROM fundamental_fetch_state WHERE task=$1`, TaskMetrics).Scan(&total)
	if total != 4 {
		t.Errorf("total seeded rows = %d, want 4", total)
	}
}

// An unrecognised scope must fail rather than silently choosing the expensive
// branch — the same principle as UNIVERSE_BAR_SOURCE failing at startup.
func TestMetricsScope_UnknownValueIsAnError(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if _, err := ResolveMetricsSymbols(ctx, pool, nil, MetricsScope("everything")); err == nil {
		t.Error("an unknown scope must not silently resolve the full universe")
	}
	if _, err := SeedFundamentalFetchState(ctx, pool, TaskMetrics, MetricsScope("everything")); err == nil {
		t.Error("an unknown scope must not silently seed the full universe")
	}
}
