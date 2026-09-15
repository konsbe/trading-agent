//go:build integration

package ratelimit

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

// Integration coverage for the Postgres-backed shared budget.
//
//	go test -tags=integration ./internal/ratelimit/ -v
//
// Requires TEST_DATABASE_URL with migration 008 applied.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	p, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// generousFallback is a fallback that would never itself throttle, so any
// observed pacing in these tests must come from the shared budget rather than
// from a local limiter quietly doing the work.
func generousFallback() Limiter {
	return rate.NewLimiter(rate.Inf, 1)
}

func newTestLimiter(t *testing.T, pool *pgxpool.Pool, key string, perSec, burst float64) *Shared {
	t.Helper()
	s, err := NewShared(pool, Budget{Key: key, RefillPerSec: perSec, Burst: burst},
		generousFallback(), Options{AcquireTimeout: 2 * time.Second, MaxSleep: time.Second}, quietLogger())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := s.EnsureBudget(context.Background()); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return s
}

func resetBudget(t *testing.T, pool *pgxpool.Pool, key string, tokens float64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE api_rate_budget SET tokens = $2, updated_at = now() WHERE budget_key = $1`, key, tokens,
	); err != nil {
		t.Fatalf("reset: %v", err)
	}
}

func TestEnsureBudget_IsIdempotentAndReconcilesShape(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	key := "test_ensure"
	if _, err := pool.Exec(ctx, `DELETE FROM api_rate_budget WHERE budget_key=$1`, key); err != nil {
		t.Fatal(err)
	}

	newTestLimiter(t, pool, key, 1, 2)
	// A second worker starting up must reconcile, not duplicate or reset.
	newTestLimiter(t, pool, key, 5, 10)

	var n int
	var perSec, burst float64
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM api_rate_budget WHERE budget_key=$1`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
	if err := pool.QueryRow(ctx,
		`SELECT refill_per_sec, burst FROM api_rate_budget WHERE budget_key=$1`, key).Scan(&perSec, &burst); err != nil {
		t.Fatal(err)
	}
	if perSec != 5 || burst != 10 {
		t.Errorf("shape = %v/%v, want 5/10 — the latest startup should reconcile rate and burst", perSec, burst)
	}
}

// The Redis-TTL bug this design exists to avoid: a restarting worker must not
// refill the shared bucket, or a rolling restart hands out a free burst per
// process and produces unexplained 429 spikes after every deploy.
func TestEnsureBudget_DoesNotRefillTokensOnRestart(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	key := "test_no_refill"
	if _, err := pool.Exec(ctx, `DELETE FROM api_rate_budget WHERE budget_key=$1`, key); err != nil {
		t.Fatal(err)
	}

	s := newTestLimiter(t, pool, key, 1, 5)
	resetBudget(t, pool, key, 0) // simulate a drained bucket

	// Three "restarts".
	for i := 0; i < 3; i++ {
		if err := s.EnsureBudget(ctx); err != nil {
			t.Fatal(err)
		}
	}

	var tokens float64
	if err := pool.QueryRow(ctx,
		`SELECT tokens FROM api_rate_budget WHERE budget_key=$1`, key).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	// Lazy refill will have added a little real elapsed time, but nowhere near a
	// full burst. The point is that EnsureBudget itself granted nothing.
	if tokens > 1 {
		t.Errorf("tokens = %v after three restarts; EnsureBudget must not refill the shared bucket", tokens)
	}
}

// THE coordination test: two independently-constructed limiters, as two worker
// processes would be, must share one budget rather than each getting their own.
func TestSharedBudget_TwoLimitersShareOneAllowance(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	key := "test_share"

	a := newTestLimiter(t, pool, key, 1, 3)
	b := newTestLimiter(t, pool, key, 1, 3)
	resetBudget(t, pool, key, 3) // exactly three tokens available

	// Six acquisitions across the two limiters; only three can be immediate.
	var granted int
	deadline := time.Now().Add(400 * time.Millisecond)
	for _, l := range []*Shared{a, b, a, b, a, b} {
		cctx, cancel := context.WithDeadline(ctx, deadline)
		if err := l.Wait(cctx); err == nil {
			granted++
		}
		cancel()
	}

	if granted != 3 {
		t.Errorf("granted %d within the window, want 3 — the two limiters must share one bucket, not hold 3 each", granted)
	}
	// And the grants must be attributable to the shared budget, not the fallback.
	if a.Stats().Degraded != 0 || b.Stats().Degraded != 0 {
		t.Errorf("unexpected degradation: a=%+v b=%+v", a.Stats(), b.Stats())
	}
	if total := a.Stats().Granted + b.Stats().Granted; total != 3 {
		t.Errorf("shared grants = %d, want 3", total)
	}
}

// Concurrency safety: the last token must go to exactly one caller. Without the
// atomic check-and-deduct, two racing acquisitions would both be granted.
func TestSharedBudget_LastTokenGoesToExactlyOneCaller(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	key := "test_race"

	const callers = 12
	limiters := make([]*Shared, callers)
	for i := range limiters {
		limiters[i] = newTestLimiter(t, pool, key, 0.001, 1) // refill effectively off
	}
	resetBudget(t, pool, key, 1) // exactly one token

	var wg sync.WaitGroup
	var mu sync.Mutex
	granted := 0

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(l *Shared) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
			defer cancel()
			if err := l.Wait(cctx); err == nil {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}(limiters[i])
	}
	wg.Wait()

	if granted != 1 {
		t.Errorf("granted = %d, want exactly 1 — the atomic check-and-deduct must prevent double-spending the last token", granted)
	}

	var tokens float64
	if err := pool.QueryRow(ctx,
		`SELECT tokens FROM api_rate_budget WHERE budget_key=$1`, key).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens < -0.01 {
		t.Errorf("tokens = %v; the balance must never go negative", tokens)
	}
}

// Sustained throughput must track refill_per_sec, which is the whole point:
// five workers sharing 1 req/s must collectively achieve 1 req/s, not 5.
func TestSharedBudget_SustainedRateTracksRefill(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	key := "test_rate"

	const perSec = 20.0 // fast enough to measure quickly, slow enough to be real
	s := newTestLimiter(t, pool, key, perSec, 1)
	resetBudget(t, pool, key, 1)

	const n = 10
	start := time.Now()
	for i := 0; i < n; i++ {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := s.Wait(cctx); err != nil {
			cancel()
			t.Fatalf("acquisition %d: %v", i, err)
		}
		cancel()
	}
	elapsed := time.Since(start)

	// n acquisitions with 1 burst token means ~(n-1)/perSec of enforced waiting.
	wantMin := time.Duration(float64(n-1) / perSec * 0.7 * float64(time.Second))
	if elapsed < wantMin {
		t.Errorf("10 acquisitions took %v, expected at least %v — the budget is not actually pacing", elapsed, wantMin)
	}
	if s.Stats().Degraded != 0 {
		t.Errorf("degraded = %d; the pacing must come from the shared budget, not a fallback",
			s.Stats().Degraded)
	}
	if s.Stats().Throttle == 0 {
		t.Error("throttle counter = 0; expected the budget to have made us wait")
	}
}

// A budget row that was never created must degrade loudly to the fallback
// rather than hang: silently waiting forever on a nonexistent budget would look
// like a stalled worker.
func TestSharedBudget_MissingRowDegradesRatherThanHangs(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	key := "test_missing"
	if _, err := pool.Exec(ctx, `DELETE FROM api_rate_budget WHERE budget_key=$1`, key); err != nil {
		t.Fatal(err)
	}

	fb := &countingLimiter{}
	s, err := NewShared(pool, Budget{Key: key, RefillPerSec: 1, Burst: 2}, fb,
		Options{AcquireTimeout: time.Second}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately NOT calling EnsureBudget.

	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.Wait(cctx); err != nil {
		t.Fatalf("expected the fallback to serve the request, got %v", err)
	}
	if fb.calls.Load() != 1 {
		t.Errorf("fallback calls = %d, want 1", fb.calls.Load())
	}
	if s.Stats().Degraded != 1 {
		t.Errorf("degraded = %d, want 1", s.Stats().Degraded)
	}
}

// Budgets are isolated by key: exhausting one must not affect another.
func TestSharedBudget_KeysAreIndependent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	a := newTestLimiter(t, pool, "test_iso_a", 0.001, 1)
	b := newTestLimiter(t, pool, "test_iso_b", 0.001, 1)
	resetBudget(t, pool, "test_iso_a", 0) // drained
	resetBudget(t, pool, "test_iso_b", 1) // has a token

	actx, acancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer acancel()
	if err := a.Wait(actx); err == nil {
		t.Error("budget A is drained; the acquisition should not have succeeded")
	}

	bctx, bcancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer bcancel()
	if err := b.Wait(bctx); err != nil {
		t.Errorf("budget B has a token but was denied: %v", err)
	}
}
