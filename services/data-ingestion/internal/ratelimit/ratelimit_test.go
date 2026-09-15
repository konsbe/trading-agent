package ratelimit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// countingLimiter records how often it was used, standing in for a worker's
// existing in-process bucket.
type countingLimiter struct {
	calls atomic.Uint64
	err   error
	delay time.Duration
}

func (c *countingLimiter) Wait(ctx context.Context) error {
	c.calls.Add(1)
	if c.delay > 0 {
		t := time.NewTimer(c.delay)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return c.err
}

// A *rate.Limiter must satisfy Limiter without adaptation — that is what makes
// the retrofit a constructor swap rather than a call-site rewrite.
func TestRateLimiterSatisfiesInterface(t *testing.T) {
	var _ Limiter = rate.NewLimiter(rate.Every(time.Second), 1)
	var _ Limiter = (*Shared)(nil)
	var _ Limiter = &countingLimiter{}
}

func TestNewShared_RejectsConfigurationThatCouldGoUnlimited(t *testing.T) {
	fb := &countingLimiter{}
	good := Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2}

	// A nil fallback is the one mistake that would make a coordination outage
	// unbounded, so it must be impossible to construct.
	if _, err := NewShared(nil, good, fb, Options{}, quietLogger()); err == nil {
		t.Error("expected an error for a nil pool")
	}

	cases := map[string]struct {
		b  Budget
		fb Limiter
	}{
		"empty key":       {Budget{Key: "", RefillPerSec: 1, Burst: 2}, fb},
		"zero refill":     {Budget{Key: "k", RefillPerSec: 0, Burst: 2}, fb},
		"negative refill": {Budget{Key: "k", RefillPerSec: -1, Burst: 2}, fb},
		// A burst below one token can never satisfy a request, so every
		// acquisition would silently degrade to the fallback forever.
		"burst below one": {Budget{Key: "k", RefillPerSec: 1, Burst: 0.5}, fb},
		"nil fallback":    {good, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Use a non-nil pool sentinel so validation order does not mask these.
			if _, err := NewShared(nonNilPool(), c.b, c.fb, Options{}, quietLogger()); err == nil {
				t.Errorf("expected an error for %s", name)
			}
		})
	}
}

func TestDefaultOptions_ZeroFieldsAreFilled(t *testing.T) {
	s, err := NewShared(nonNilPool(), Budget{Key: "k", RefillPerSec: 1, Burst: 2},
		&countingLimiter{}, Options{}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	d := DefaultOptions()
	if s.opts.AcquireTimeout != d.AcquireTimeout || s.opts.MaxSleep != d.MaxSleep || s.opts.WarnEvery != d.WarnEvery {
		t.Errorf("zero options not defaulted: %+v", s.opts)
	}
}

// THE fallback test, and the reason it is worth having separately from the
// Postgres tests: a limiter pointed at an unreachable database must keep the
// worker running at its old local rate, not stall and not go unlimited.
func TestWait_DegradesToFallbackWhenCoordinationFails(t *testing.T) {
	s, err := NewShared(unreachablePool(t), Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2},
		&countingLimiter{}, Options{AcquireTimeout: 50 * time.Millisecond}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	fb := s.fallback.(*countingLimiter)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := s.Wait(context.Background()); err != nil {
			t.Fatalf("Wait must succeed via the fallback, got %v", err)
		}
	}
	elapsed := time.Since(start)

	if got := fb.calls.Load(); got != 3 {
		t.Errorf("fallback used %d times, want 3 — every request must be served locally", got)
	}
	if st := s.Stats(); st.Degraded != 3 {
		t.Errorf("degraded counter = %d, want 3 — this counter is what makes the outage diagnosable", st.Degraded)
	}
	if st := s.Stats(); st.Granted != 0 {
		t.Errorf("granted = %d, want 0", st.Granted)
	}
	// Bounded: three attempts at a 50ms acquire timeout must not take seconds.
	if elapsed > 2*time.Second {
		t.Errorf("took %v; a coordination outage must not stall the worker", elapsed)
	}
}

// Degradation is per-request, not a latch: the limiter must resume using the
// shared budget as soon as it is reachable again, rather than staying local.
func TestWait_DegradationIsPerRequestNotPermanent(t *testing.T) {
	s, err := NewShared(unreachablePool(t), Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2},
		&countingLimiter{}, Options{AcquireTimeout: 20 * time.Millisecond}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Nothing in Shared records a sticky "degraded" mode, so the next call will
	// try Postgres again. Assert that no such state exists.
	if s.Stats().Degraded != 1 {
		t.Fatalf("degraded = %d, want 1", s.Stats().Degraded)
	}
	if err := s.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := s.Stats().Degraded; got != 2 {
		t.Errorf("degraded = %d, want 2 — each request must re-attempt coordination", got)
	}
}

// A failing fallback must surface, not be swallowed: if both coordination and
// the local limiter are broken, the caller needs to know.
func TestWait_PropagatesFallbackError(t *testing.T) {
	sentinel := errors.New("local limiter closed")
	s, err := NewShared(unreachablePool(t), Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2},
		&countingLimiter{err: sentinel}, Options{AcquireTimeout: 20 * time.Millisecond}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Wait(context.Background()); !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want the fallback's error", err)
	}
}

func TestWait_HonoursContextCancellation(t *testing.T) {
	s, err := NewShared(unreachablePool(t), Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2},
		&countingLimiter{delay: time.Hour}, Options{AcquireTimeout: 20 * time.Millisecond}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := s.Wait(ctx); err == nil {
		t.Fatal("expected a context error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %v; cancellation must abort promptly", elapsed)
	}
}

// An already-cancelled context must not perform work.
func TestWait_ReturnsImmediatelyOnCancelledContext(t *testing.T) {
	s, err := NewShared(unreachablePool(t), Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2},
		&countingLimiter{}, Options{}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if s.Stats().Degraded != 0 {
		t.Error("a cancelled context must not count as degradation")
	}
}

func TestWarn_IsThrottled(t *testing.T) {
	s, err := NewShared(unreachablePool(t), Budget{Key: "finnhub", RefillPerSec: 1, Burst: 2},
		&countingLimiter{}, Options{AcquireTimeout: 10 * time.Millisecond, WarnEvery: time.Hour}, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	// Many degradations, but the throttle must keep the log quiet. The counter,
	// not the log, is the complete record.
	for i := 0; i < 5; i++ {
		_ = s.Wait(context.Background())
	}
	if got := s.Stats().Degraded; got != 5 {
		t.Errorf("degraded = %d, want 5 — the counter must record every occurrence", got)
	}
	s.warnMu.Lock()
	last := s.lastWarn
	s.warnMu.Unlock()
	if last.IsZero() {
		t.Error("expected at least one warning to have fired")
	}
}
