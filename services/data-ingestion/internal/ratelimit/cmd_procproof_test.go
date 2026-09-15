//go:build integration

package ratelimit

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSharedBudget_AcrossSeparateProcesses is the claim that actually matters:
// the budget is shared by separate OS processes, not merely by separate structs
// inside one test binary. In-process sharing would also be satisfied by a plain
// package-level limiter, which would not fix anything in production.
//
// The child processes are re-executions of this same test binary, dispatched via
// an env var, so no separate fixture binary is needed.
func TestSharedBudget_AcrossSeparateProcesses(t *testing.T) {
	if os.Getenv("RATELIMIT_CHILD") == "1" {
		childAcquire(t)
		return
	}

	ctx := context.Background()
	pool := testPool(t)
	key := "test_procs"

	// Refill effectively off, exactly 4 tokens available. Eight child processes
	// will each attempt one acquisition.
	newTestLimiter(t, pool, key, 0.001, 4)
	resetBudget(t, pool, key, 4)

	const procs = 8
	results := make([]string, procs)
	var wg sync.WaitGroup
	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run", "TestSharedBudget_AcrossSeparateProcesses")
			cmd.Env = append(os.Environ(),
				"RATELIMIT_CHILD=1",
				"RATELIMIT_CHILD_KEY="+key,
			)
			out, _ := cmd.CombinedOutput()
			results[i] = string(out)
		}(i)
	}
	wg.Wait()

	granted := 0
	denied := 0
	for i, out := range results {
		switch {
		case strings.Contains(out, "CHILD_RESULT=granted"):
			granted++
		case strings.Contains(out, "CHILD_RESULT=denied"):
			denied++
		default:
			t.Errorf("child %d produced no verdict; output:\n%s", i, out)
		}
	}

	if granted != 4 {
		t.Errorf("granted = %d across %d processes, want exactly 4 — separate processes must share one bucket",
			granted, procs)
	}
	if denied != procs-4 {
		t.Errorf("denied = %d, want %d", denied, procs-4)
	}

	// The balance must not have gone negative under cross-process contention.
	var tokens float64
	if err := pool.QueryRow(ctx,
		`SELECT tokens FROM api_rate_budget WHERE budget_key=$1`, key).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens < -0.01 {
		t.Errorf("tokens = %v; the balance must never go negative", tokens)
	}
}

// childAcquire runs in the subprocess: one bounded acquisition, verdict printed
// to stdout for the parent to tally.
func childAcquire(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		return
	}
	pool := testPool(t)
	key := os.Getenv("RATELIMIT_CHILD_KEY")

	fb := &countingLimiter{}
	s, err := NewShared(pool, Budget{Key: key, RefillPerSec: 0.001, Burst: 4}, fb,
		Options{AcquireTimeout: 3 * time.Second, MaxSleep: 200 * time.Millisecond}, quietLogger())
	if err != nil {
		os.Stdout.WriteString("CHILD_RESULT=error " + err.Error() + "\n")
		return
	}
	// Deliberately NOT calling EnsureBudget: the parent created the row, and a
	// child refilling it would invalidate the test (and is the deploy-burst bug).

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	if err := s.Wait(ctx); err == nil {
		// A grant only counts if it came from the shared budget. Had it come from
		// the fallback, the budget would not have been consulted at all.
		if fb.calls.Load() > 0 {
			os.Stdout.WriteString("CHILD_RESULT=error granted via fallback, not the shared budget\n")
			return
		}
		os.Stdout.WriteString("CHILD_RESULT=granted tokens_left=" +
			strconv.FormatUint(s.Stats().Granted, 10) + "\n")
		return
	}
	os.Stdout.WriteString("CHILD_RESULT=denied\n")
}
