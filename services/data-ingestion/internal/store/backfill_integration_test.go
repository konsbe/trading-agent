//go:build integration

package store

import (
	"context"
	"testing"
	"time"
)

// Integration coverage for the backfill checkpoint state machine — §10 step 3's
// "job survives a kill and resumes".
//
//	go test -tags=integration ./internal/store/ -run Backfill -v

const (
	testLease       = 15 * time.Minute
	testMaxAttempts = 3
)

func TestBackfillClaim_LeasesPendingAndMarksInProgress(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	syms := []UniverseRow{
		{Symbol: "AAA", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "BBB", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "CCC", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		// Ineligible rows must never be claimed.
		{Symbol: "SKIP", Exchange: "NASDAQ", Type: "Warrant", IsEligible: false, ExcludedReason: "type_not_common_stock"},
	}
	if _, err := UpsertUniverseSymbols(ctx, pool, syms); err != nil {
		t.Fatal(err)
	}

	claims, err := ClaimBackfillBatch(ctx, pool, 2, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("claims = %d, want 2 (the batch limit)", len(claims))
	}

	// Claimed rows are in_progress with a stamped claim time.
	for _, c := range claims {
		var status string
		var claimedAt *time.Time
		if err := pool.QueryRow(ctx,
			`SELECT backfill_status, backfill_claimed_at FROM universe_symbols WHERE symbol=$1`, c.Symbol,
		).Scan(&status, &claimedAt); err != nil {
			t.Fatal(err)
		}
		if status != BackfillInProgress {
			t.Errorf("%s status = %q, want in_progress", c.Symbol, status)
		}
		if claimedAt == nil {
			t.Errorf("%s has no backfill_claimed_at — a crash would strand it forever", c.Symbol)
		}
	}

	// A second claim gets the remaining symbol, not the already-claimed ones.
	more, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(more) != 1 {
		t.Fatalf("second claim = %d, want 1 (fresh claims are not re-leased)", len(more))
	}
	for _, c := range more {
		if c.Symbol == "SKIP" {
			t.Error("an ineligible symbol was claimed")
		}
	}

	// Everything is now claimed, so a third round finds nothing.
	none, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("third claim = %d, want 0", len(none))
	}
}

// THE resumability test: simulate a worker killed mid-symbol by leaving rows
// in_progress, then prove a later round reclaims them once the lease expires.
// Without this, a crash silently finishes the backfill incomplete.
func TestBackfillClaim_ReclaimsAbandonedClaimsAfterLease(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "STUCK", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}

	// Worker claims it...
	claims, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil || len(claims) != 1 {
		t.Fatalf("initial claim: %v (n=%d)", err, len(claims))
	}

	// ...and is killed. The row stays in_progress. Within the lease it must NOT
	// be reclaimed, or two workers would duplicate work.
	fresh, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh) != 0 {
		t.Errorf("reclaimed inside the lease window (n=%d) — that would duplicate in-flight work", len(fresh))
	}

	// Age the claim past the lease, as a restart minutes later would see.
	if _, err := pool.Exec(ctx,
		`UPDATE universe_symbols SET backfill_claimed_at = now() - interval '1 hour' WHERE symbol='STUCK'`,
	); err != nil {
		t.Fatal(err)
	}

	reclaimed, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(reclaimed) != 1 || reclaimed[0].Symbol != "STUCK" {
		t.Fatalf("expected STUCK to be reclaimed after the lease expired, got %+v", reclaimed)
	}
}

func TestBackfillClaim_RetriesFailedUntilMaxAttempts(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "FLAKY", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= testMaxAttempts; attempt++ {
		claims, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(claims) != 1 {
			t.Fatalf("attempt %d: claims = %d, want 1", attempt, len(claims))
		}
		if got := claims[0].Attempts; got != attempt-1 {
			t.Errorf("attempt %d: reported prior attempts = %d, want %d", attempt, got, attempt-1)
		}
		if err := MarkBackfillFailed(ctx, pool, "FLAKY", "NASDAQ", "HTTP 503"); err != nil {
			t.Fatal(err)
		}
	}

	// Attempts are now exhausted: no further claims, and the error text is
	// retained so the stall is diagnosable from SQL alone.
	none, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("claimed a symbol past max attempts (n=%d)", len(none))
	}

	var status, lastErr string
	var attempts int
	if err := pool.QueryRow(ctx,
		`SELECT backfill_status, backfill_attempts, backfill_last_error FROM universe_symbols WHERE symbol='FLAKY'`,
	).Scan(&status, &attempts, &lastErr); err != nil {
		t.Fatal(err)
	}
	if status != BackfillFailed || attempts != testMaxAttempts {
		t.Errorf("status=%q attempts=%d, want failed/%d", status, attempts, testMaxAttempts)
	}
	if lastErr != "HTTP 503" {
		t.Errorf("last_error = %q, want the persisted failure reason", lastErr)
	}

	prog, err := LoadBackfillProgress(ctx, pool, testMaxAttempts, 252)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Exhausted != 1 {
		t.Errorf("exhausted = %d, want 1 — these are the rows needing human attention", prog.Exhausted)
	}
}

func TestBackfillDone_ClearsClaimAndRecordsCursor(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "GOOD", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "EMPTY", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false); err != nil {
		t.Fatal(err)
	}

	oldest := time.Date(2023, 9, 15, 0, 0, 0, 0, time.UTC)
	if err := MarkBackfillDone(ctx, pool, "GOOD", "NASDAQ", 750, &oldest); err != nil {
		t.Fatal(err)
	}
	// A symbol Yahoo has no data for: done with zero bars, not failed. Retrying
	// a delisting forever would waste the rate budget the universe needs.
	if err := MarkBackfillDone(ctx, pool, "EMPTY", "NASDAQ", 0, nil); err != nil {
		t.Fatal(err)
	}

	var status string
	var cursor, completed, claimed *time.Time
	var lastErr *string
	if err := pool.QueryRow(ctx, `
SELECT backfill_status, backfill_cursor_ts, backfill_completed_at, backfill_claimed_at, backfill_last_error
FROM universe_symbols WHERE symbol='GOOD'`).Scan(&status, &cursor, &completed, &claimed, &lastErr); err != nil {
		t.Fatal(err)
	}
	if status != BackfillDone {
		t.Errorf("status = %q, want done", status)
	}
	if cursor == nil || !cursor.Equal(oldest) {
		t.Errorf("cursor = %v, want the oldest stored bar %v", cursor, oldest)
	}
	if completed == nil {
		t.Error("backfill_completed_at not set")
	}
	if claimed != nil {
		t.Error("backfill_claimed_at must be cleared on completion, or the lease logic sees a stale claim")
	}
	if lastErr != nil {
		t.Errorf("last_error = %q, want NULL on success", *lastErr)
	}

	// The zero-bar case is marked so it is distinguishable from a real backfill.
	var emptyStatus string
	var emptyErr *string
	if err := pool.QueryRow(ctx,
		`SELECT backfill_status, backfill_last_error FROM universe_symbols WHERE symbol='EMPTY'`,
	).Scan(&emptyStatus, &emptyErr); err != nil {
		t.Fatal(err)
	}
	if emptyStatus != BackfillDone {
		t.Errorf("empty symbol status = %q, want done", emptyStatus)
	}
	if emptyErr == nil || *emptyErr != "no_bars_returned" {
		t.Errorf("empty symbol last_error = %v, want no_bars_returned", emptyErr)
	}

	// Completed symbols are never re-claimed.
	none, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("re-claimed a completed symbol (n=%d)", len(none))
	}
}

// Pending work is claimed before retries, so a first pass covers the universe
// once before spending budget on known-flaky symbols.
func TestBackfillClaim_PrioritisesPendingOverFailed(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "PEND", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "FAIL", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE universe_symbols SET backfill_status='failed', backfill_attempts=1, backfill_claimed_at=NULL
WHERE symbol='FAIL'`); err != nil {
		t.Fatal(err)
	}

	claims, err := ClaimBackfillBatch(ctx, pool, 1, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 || claims[0].Symbol != "PEND" {
		t.Fatalf("claimed %+v, want PEND first", claims)
	}
}

func TestUpsertEquityOHLCVBatch_IsIdempotentAndTransactional(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if _, err := pool.Exec(ctx,
		`DELETE FROM equity_ohlcv WHERE symbol='BATCH' AND source='yahoo_finance'`); err != nil {
		t.Fatal(err)
	}

	day := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	bars := make([]EquityBar, 0, 5)
	for i := 0; i < 5; i++ {
		bars = append(bars, EquityBar{
			TS: day.AddDate(0, 0, -i), Symbol: "BATCH", Interval: "1Day",
			Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 1000, Source: "yahoo_finance",
		})
	}

	if _, err := UpsertEquityOHLCVBatch(ctx, pool, bars); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Re-running the same symbol must update, not duplicate — the backfill is
	// expected to be re-run and must be safe.
	bars[0].Close = 12.5
	if _, err := UpsertEquityOHLCVBatch(ctx, pool, bars); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var n int
	var closed float64
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM equity_ohlcv WHERE symbol='BATCH' AND source='yahoo_finance'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Errorf("rows = %d, want 5", n)
	}
	if err := pool.QueryRow(ctx,
		`SELECT close FROM equity_ohlcv WHERE symbol='BATCH' AND ts=$1 AND source='yahoo_finance'`, day,
	).Scan(&closed); err != nil {
		t.Fatal(err)
	}
	if closed != 12.5 {
		t.Errorf("close = %v, want the updated 12.5", closed)
	}

	// An empty batch is a no-op, not an error.
	if _, err := UpsertEquityOHLCVBatch(ctx, pool, nil); err != nil {
		t.Errorf("empty batch: %v", err)
	}
}

func TestResetBackfill_ClearsAllCheckpointState(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "RST", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false); err != nil {
		t.Fatal(err)
	}
	if err := MarkBackfillFailed(ctx, pool, "RST", "NASDAQ", "boom"); err != nil {
		t.Fatal(err)
	}

	if _, err := ResetBackfill(ctx, pool); err != nil {
		t.Fatalf("reset: %v", err)
	}

	var status string
	var attempts int
	var lastErr *string
	var claimed, completed *time.Time
	if err := pool.QueryRow(ctx, `
SELECT backfill_status, backfill_attempts, backfill_last_error, backfill_claimed_at, backfill_completed_at
FROM universe_symbols WHERE symbol='RST'`).Scan(&status, &attempts, &lastErr, &claimed, &completed); err != nil {
		t.Fatal(err)
	}
	if status != BackfillPending || attempts != 0 || lastErr != nil || claimed != nil || completed != nil {
		t.Errorf("reset left state behind: status=%q attempts=%d err=%v claimed=%v completed=%v",
			status, attempts, lastErr, claimed, completed)
	}
}

func TestLoadBarBounds_ReportsZeroForNeverBackfilledSymbols(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "HASBARS", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "NOBARS", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`DELETE FROM equity_ohlcv WHERE symbol='HASBARS' AND source='yahoo_finance'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source)
SELECT date_trunc('day', now()) - (g || ' days')::interval, 'HASBARS', '1Day', 1,1,1,1,1000, 'yahoo_finance'
FROM generate_series(1, 4) g`); err != nil {
		t.Fatal(err)
	}

	bounds, err := LoadBarBounds(ctx, pool, "1Day", "yahoo_finance")
	if err != nil {
		t.Fatalf("bounds: %v", err)
	}
	if got := bounds["HASBARS"].Count; got != 4 {
		t.Errorf("HASBARS count = %d, want 4", got)
	}
	// A LEFT JOIN, so never-backfilled symbols are present with zero rather than
	// missing — the daily refresh uses that to skip them instead of requesting a
	// useless 7-day history.
	nb, ok := bounds["NOBARS"]
	if !ok {
		t.Fatal("NOBARS absent; the daily refresh needs it present with count 0 to skip it")
	}
	if nb.Count != 0 || nb.FirstTS != nil || nb.LastTS != nil {
		t.Errorf("NOBARS = %+v, want zero/nil", nb)
	}
}

// The pilot and the full universe use different providers with different
// quotas, so the backfill must be able to claim only the selected subset —
// running it over the wrong population spends the wrong budget.
func TestClaimBackfillBatch_SelectedOnlyRestrictsToTheSubset(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	clearUniverse(t, pool)

	if _, err := UpsertUniverseSymbols(ctx, pool, []UniverseRow{
		{Symbol: "INSUB", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
		{Symbol: "OUTSUB", Exchange: "NASDAQ", Type: "Common Stock", IsEligible: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE universe_symbols SET backfill_selected = true WHERE symbol = 'INSUB'`); err != nil {
		t.Fatal(err)
	}

	// selectedOnly = true claims only the subset.
	claims, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 || claims[0].Symbol != "INSUB" {
		t.Fatalf("claims = %+v, want only INSUB", claims)
	}

	// selectedOnly = false claims the whole eligible universe. INSUB is already
	// in_progress from the claim above, so only OUTSUB is free.
	all, err := ClaimBackfillBatch(ctx, pool, 10, testLease, testMaxAttempts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Symbol != "OUTSUB" {
		t.Fatalf("claims = %+v, want OUTSUB when unrestricted", all)
	}
}
