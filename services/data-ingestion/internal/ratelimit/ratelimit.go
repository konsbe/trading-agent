// Package ratelimit provides a rate limiter whose token bucket is shared across
// processes via Postgres, so several worker binaries using one upstream API key
// respect a single budget.
//
// The problem it solves is described in migration 008_api_rate_budget.sql: each
// worker's in-process bucket is correct on its own and wrong in aggregate, and
// the failure surfaces as 429s in a different service from the one that caused
// them.
//
// Design constraints, in priority order:
//
//  1. Never go unlimited. If coordination fails, fall back to the caller's
//     existing in-process limiter — i.e. exactly today's behaviour, which is
//     known to be survivable at current load.
//  2. Never block indefinitely. Every shared acquisition has a bounded
//     deadline; exceeding it degrades to the fallback rather than stalling the
//     worker.
//  3. Make degradation visible. A counter and a throttled warn log exist so
//     "why are there 429s in data-sentiment" resolves to "the shared limiter
//     degraded" instead of becoming a fresh investigation.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Limiter is the subset of *golang.org/x/time/rate.Limiter that the fetch
// clients use. Declaring it here means retrofitting a client is a constructor
// change, not a call-site rewrite, and *rate.Limiter satisfies it as-is.
type Limiter interface {
	Wait(ctx context.Context) error
}

// Budget identifies an upstream quota and its shape.
type Budget struct {
	// Key names the quota, not the worker: every caller using the same API key
	// must pass the same Key or they are not sharing anything.
	Key string

	// RefillPerSec is the sustained request rate. 1.0 for Finnhub's free tier.
	RefillPerSec float64

	// Burst is the maximum accumulation, i.e. the largest allowed spike.
	Burst float64
}

// Options tunes the shared limiter's behaviour.
type Options struct {
	// AcquireTimeout bounds a single acquisition attempt. Exceeding it is
	// treated as coordination failure and degrades to the fallback. Deliberately
	// short: the point is to avoid stalling a worker, and the fallback is safe.
	AcquireTimeout time.Duration

	// MaxSleep caps how long a single denied acquisition will wait before
	// re-attempting, so a misconfigured budget cannot park a worker for minutes.
	MaxSleep time.Duration

	// WarnEvery throttles the degradation warning. Without this, a Postgres blip
	// during a 6,000-symbol pass would emit thousands of identical lines.
	WarnEvery time.Duration
}

func DefaultOptions() Options {
	return Options{
		AcquireTimeout: 250 * time.Millisecond,
		MaxSleep:       5 * time.Second,
		WarnEvery:      30 * time.Second,
	}
}

// Stats reports coordination health. Read it to distinguish "the shared budget
// is working and we are simply rate-limited" from "we have been running on the
// local fallback for an hour".
type Stats struct {
	Granted  uint64 // acquisitions satisfied by the shared budget
	Throttle uint64 // times the shared budget made us wait
	Degraded uint64 // times we fell back to the local limiter
}

// Shared is a Postgres-backed token bucket with a local fallback.
//
// It satisfies Limiter, so it drops into any client that currently holds a
// *rate.Limiter.
type Shared struct {
	pool     *pgxpool.Pool
	budget   Budget
	fallback Limiter
	opts     Options
	log      *slog.Logger

	granted  atomic.Uint64
	throttle atomic.Uint64
	degraded atomic.Uint64

	warnMu   sync.Mutex
	lastWarn time.Time
}

// NewShared builds a shared limiter.
//
// fallback is required and must not be nil: it is what keeps requirement 1
// (never unlimited) true. Passing the caller's existing in-process limiter means
// a coordination outage degrades to precisely the behaviour that shipped before
// this package existed.
func NewShared(pool *pgxpool.Pool, b Budget, fallback Limiter, o Options, log *slog.Logger) (*Shared, error) {
	if pool == nil {
		return nil, errors.New("ratelimit: nil pool")
	}
	if b.Key == "" {
		return nil, errors.New("ratelimit: empty budget key")
	}
	if b.RefillPerSec <= 0 {
		return nil, fmt.Errorf("ratelimit: budget %q needs a positive refill rate", b.Key)
	}
	if b.Burst < 1 {
		// A burst below one token can never satisfy a request, which would make
		// every acquisition degrade silently to the fallback.
		return nil, fmt.Errorf("ratelimit: budget %q needs burst >= 1, got %v", b.Key, b.Burst)
	}
	if fallback == nil {
		return nil, errors.New("ratelimit: nil fallback — a coordination outage must degrade to a local limiter, never to unlimited")
	}
	d := DefaultOptions()
	if o.AcquireTimeout <= 0 {
		o.AcquireTimeout = d.AcquireTimeout
	}
	if o.MaxSleep <= 0 {
		o.MaxSleep = d.MaxSleep
	}
	if o.WarnEvery <= 0 {
		o.WarnEvery = d.WarnEvery
	}
	if log == nil {
		log = slog.Default()
	}
	return &Shared{pool: pool, budget: b, fallback: fallback, opts: o, log: log}, nil
}

const ensureBudgetSQL = `
INSERT INTO api_rate_budget (budget_key, tokens, refill_per_sec, burst, updated_at)
VALUES ($1, $2, $2, $3, now())
ON CONFLICT (budget_key) DO UPDATE SET
    refill_per_sec = EXCLUDED.refill_per_sec,
    burst          = EXCLUDED.burst,
    -- tokens and updated_at are deliberately NOT reset: a worker restarting
    -- must not refill the shared bucket, or a rolling restart would hand out a
    -- free burst per process. That is the Redis-TTL bug this design avoids.
    tokens         = LEAST(api_rate_budget.tokens, EXCLUDED.burst)`

// EnsureBudget creates or reconciles the budget row. Idempotent, so every worker
// can call it at startup; whichever runs first creates it and the rest confirm
// the rate and burst.
//
// The initial token count is one second's worth rather than a full burst, so a
// cold start does not immediately spend the entire allowance.
func (s *Shared) EnsureBudget(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, ensureBudgetSQL, s.budget.Key, s.budget.RefillPerSec, s.budget.Burst)
	if err != nil {
		return fmt.Errorf("ensure budget %q: %w", s.budget.Key, err)
	}
	return nil
}

// acquireSQL grants one token if the lazily-refilled balance allows it.
//
// The refill expression is repeated in the WHERE clause on purpose: that makes
// the check and the deduction one atomic statement. Two callers cannot both take
// the last token — the second blocks on the row lock, then re-evaluates the
// predicate against the committed row under READ COMMITTED and returns no rows
// if the balance has fallen below 1.
const acquireSQL = `
UPDATE api_rate_budget SET
    tokens     = LEAST(burst, tokens + refill_per_sec * GREATEST(0, EXTRACT(EPOCH FROM (now() - updated_at)))) - 1,
    updated_at = now()
WHERE budget_key = $1
  AND LEAST(burst, tokens + refill_per_sec * GREATEST(0, EXTRACT(EPOCH FROM (now() - updated_at)))) >= 1
RETURNING tokens`

// waitSQL computes how long until one token is available. Only issued on the
// denied path, where the caller is about to sleep anyway.
const waitSQL = `
SELECT GREATEST(0, (1 - LEAST(burst, tokens + refill_per_sec * GREATEST(0, EXTRACT(EPOCH FROM (now() - updated_at))))) / refill_per_sec)
FROM api_rate_budget
WHERE budget_key = $1`

// Wait blocks until the shared budget grants a token, the context is done, or
// coordination fails and the local fallback takes over.
func (s *Shared) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		granted, sleep, err := s.tryAcquire(ctx)
		if err != nil {
			// Coordination failed. Degrade to the local limiter for this request
			// only — never unlimited, never a permanent switch.
			s.degraded.Add(1)
			s.warn("shared rate limiter unavailable; falling back to the local in-process limiter for this request", err)
			return s.fallback.Wait(ctx)
		}
		if granted {
			s.granted.Add(1)
			return nil
		}

		s.throttle.Add(1)
		if sleep <= 0 {
			// Denied with no computable wait: yield briefly rather than spinning.
			sleep = 10 * time.Millisecond
		}
		if sleep > s.opts.MaxSleep {
			sleep = s.opts.MaxSleep
		}
		t := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// tryAcquire makes one bounded attempt. Returns (granted, suggestedSleep, err).
//
// A missing budget row is an error, not a denial: silently waiting forever on a
// budget nobody created would look like a hang, whereas degrading to the local
// limiter keeps the worker working and logs why.
func (s *Shared) tryAcquire(ctx context.Context) (bool, time.Duration, error) {
	actx, cancel := context.WithTimeout(ctx, s.opts.AcquireTimeout)
	defer cancel()

	var tokens float64
	err := s.pool.QueryRow(actx, acquireSQL, s.budget.Key).Scan(&tokens)
	switch {
	case err == nil:
		return true, 0, nil
	case errors.Is(err, pgx.ErrNoRows):
		// Either out of tokens or the row is absent. Ask which.
	default:
		return false, 0, err
	}

	var secs float64
	err = s.pool.QueryRow(actx, waitSQL, s.budget.Key).Scan(&secs)
	switch {
	case err == nil:
		return false, time.Duration(secs * float64(time.Second)), nil
	case errors.Is(err, pgx.ErrNoRows):
		return false, 0, fmt.Errorf("ratelimit: budget %q missing; call EnsureBudget at startup", s.budget.Key)
	default:
		return false, 0, err
	}
}

// Stats returns a snapshot of coordination health.
func (s *Shared) Stats() Stats {
	return Stats{
		Granted:  s.granted.Load(),
		Throttle: s.throttle.Load(),
		Degraded: s.degraded.Load(),
	}
}

// warn logs at most once per WarnEvery. The counter in Stats is the durable
// signal; this is the one that gets noticed.
func (s *Shared) warn(msg string, err error) {
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	if time.Since(s.lastWarn) < s.opts.WarnEvery {
		return
	}
	s.lastWarn = time.Now()
	s.log.Warn(msg,
		"budget", s.budget.Key,
		"err", err,
		"degraded_total", s.degraded.Load(),
		"acquire_timeout", s.opts.AcquireTimeout.String())
}
