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

	// DailyResetTZ is the timezone whose midnight ends the daily window.
	//
	// Empty means UTC. It exists because NO provider here resets at UTC
	// midnight: Tiingo's spec says "daily requests (reset at midnight EST)", and
	// rolling our counter at 00:00 UTC meant that for five hours a day the
	// ceiling was not real — our counter read fresh while the provider's had not
	// reset, so a second full allowance could be authorised on top of one
	// already spent.
	//
	// Use "EST", not "America/New_York": Postgres resolves EST as a fixed UTC-5
	// offset, which is what "midnight EST" means, whereas the named zone follows
	// DST and would reset an hour EARLY every summer. The asymmetry matters —
	// resetting later than the provider merely under-uses the allowance, while
	// resetting earlier overspends it, so an undocumented reset should take the
	// latest plausible boundary rather than the most convenient one.
	DailyResetTZ string

	// DailyLimit is a hard requests-per-day ceiling, or nil for none.
	//
	// A separate mechanism from the token bucket above, not a second rate: the
	// bucket paces, this stops. Twelve Data's free tier is the motivating case —
	// 8 requests/minute AND 800 credits/day, where the daily cap binds first
	// (800 at 8/min is ~100 minutes of work, then blocked until the window
	// rolls).
	//
	// Pad this below the provider's documented figure until the window boundary
	// has been reconciled against the provider's own reported usage. Being wrong
	// in the "we have less room than we thought" direction costs a spurious
	// ErrDailyQuotaExhausted, which callers already handle by retrying tomorrow.
	// Being wrong the other way overruns the account.
	DailyLimit *float64
}

// ErrDailyQuotaExhausted reports that the shared budget's daily ceiling is spent.
//
// This is deliberately NOT a coordination failure, and the difference is the
// whole reason it exists. A coordination failure means "we do not know the
// state, so be conservative" and correctly degrades to the caller's local
// limiter. A spent daily quota means "we know the state exactly, and it is
// zero" — degrading there would pace requests against an account with nothing
// left, turning the mechanism built to prevent unlimited access into the thing
// granting it, while the limiter's own logs looked healthy.
//
// Callers must stop the pass rather than retry in-process. The momentum
// scanner's claim/lease checkpoint already does the right thing with this:
// record the failure against the symbol, end the round, resume after the window
// rolls.
var ErrDailyQuotaExhausted = errors.New("ratelimit: daily quota exhausted")

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

	// QuotaExhausted counts ErrDailyQuotaExhausted returns. Counted separately
	// from Degraded on purpose: degradation means coordination broke and we are
	// running on local rate, while quota exhaustion means coordination worked
	// perfectly and the answer was no. Conflating them would hide a spent
	// account inside a "limiter unhealthy" metric.
	QuotaExhausted uint64
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
	quotaOut atomic.Uint64

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
	if b.DailyLimit != nil && *b.DailyLimit < 1 {
		// A ceiling below one request can never grant anything, so every
		// acquisition would return ErrDailyQuotaExhausted and the caller would
		// look permanently quota-blocked with no way to tell why.
		return nil, fmt.Errorf("ratelimit: budget %q has DailyLimit %v; must be >= 1 or nil for no ceiling", b.Key, *b.DailyLimit)
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
INSERT INTO api_rate_budget (budget_key, tokens, refill_per_sec, burst, daily_limit, daily_reset_tz, updated_at)
VALUES ($1, $2, $2, $3, $4, $5, now())
ON CONFLICT (budget_key) DO UPDATE SET
    refill_per_sec = EXCLUDED.refill_per_sec,
    burst          = EXCLUDED.burst,
    daily_limit    = EXCLUDED.daily_limit,
    -- Reconciled on every startup, unlike the counters below: this is
    -- configuration, so a corrected boundary must take effect without a manual
    -- UPDATE. Changing it does NOT reset daily_used, so a mid-window correction
    -- cannot be used to hand back a spent allowance.
    daily_reset_tz = EXCLUDED.daily_reset_tz,
    -- tokens and updated_at are deliberately NOT reset: a worker restarting
    -- must not refill the shared bucket, or a rolling restart would hand out a
    -- free burst per process. That is the Redis-TTL bug this design avoids.
    --
    -- daily_used and daily_window_start are likewise untouched, for the same
    -- reason: a restart must not hand back a spent daily quota.
    tokens         = LEAST(api_rate_budget.tokens, EXCLUDED.burst)`

// EnsureBudget creates or reconciles the budget row. Idempotent, so every worker
// can call it at startup; whichever runs first creates it and the rest confirm
// the rate and burst.
//
// The initial token count is one second's worth rather than a full burst, so a
// cold start does not immediately spend the entire allowance.
func (s *Shared) EnsureBudget(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, ensureBudgetSQL,
		s.budget.Key, s.budget.RefillPerSec, s.budget.Burst, s.budget.DailyLimit,
		s.budget.resetTZ())
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
    updated_at = now(),
    -- Roll the daily window and consume in the same statement, so the counter
    -- can never be reset by one caller while another is mid-acquisition.
    daily_used = CASE
        WHEN daily_window_start IS DISTINCT FROM (now() AT TIME ZONE daily_reset_tz)::date THEN 1
        ELSE daily_used + 1
    END,
    daily_window_start = (now() AT TIME ZONE daily_reset_tz)::date
WHERE budget_key = $1
  AND LEAST(burst, tokens + refill_per_sec * GREATEST(0, EXTRACT(EPOCH FROM (now() - updated_at)))) >= 1
  AND (
       daily_limit IS NULL
       -- A window that has rolled is fresh regardless of the stored counter.
    OR daily_window_start IS DISTINCT FROM (now() AT TIME ZONE daily_reset_tz)::date
    OR daily_used < daily_limit
  )
RETURNING tokens`

// denialSQL explains a denial: was it the rate, or the daily ceiling?
//
// Only issued on the denied path, where the caller is about to sleep anyway, and
// the distinction is essential — one means "wait a moment", the other means
// "come back tomorrow", and no amount of waiting inside Wait can express the
// second.
const denialSQL = `
SELECT
    (daily_limit IS NOT NULL
     AND daily_window_start = (now() AT TIME ZONE daily_reset_tz)::date
     AND daily_used >= daily_limit) AS daily_exhausted,
    GREATEST(0, (1 - LEAST(burst, tokens + refill_per_sec * GREATEST(0, EXTRACT(EPOCH FROM (now() - updated_at))))) / refill_per_sec) AS wait_secs
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
		if errors.Is(err, ErrDailyQuotaExhausted) {
			// NOT degradation, and deliberately NOT routed through the fallback.
			// Coordination worked; the answer is that the account has nothing
			// left today. Pacing against a spent quota would convert a hard
			// ceiling into a wall of 429s — or, on a paid tier, into overage
			// charges — using the very mechanism meant to prevent it.
			s.quotaOut.Add(1)
			s.warn("shared daily quota exhausted; stopping rather than falling back to the local limiter", err)
			return err
		}
		if err != nil {
			// Coordination failed: we do not know the state, so be conservative
			// and degrade to the local limiter for this request only — never
			// unlimited, never a permanent switch.
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

	var dailyExhausted bool
	var secs float64
	err = s.pool.QueryRow(actx, denialSQL, s.budget.Key).Scan(&dailyExhausted, &secs)
	switch {
	case err == nil:
		if dailyExhausted {
			// Wrapped so the budget key appears in logs, while errors.Is still
			// matches the sentinel.
			return false, 0, fmt.Errorf("%w (budget %q)", ErrDailyQuotaExhausted, s.budget.Key)
		}
		return false, time.Duration(secs * float64(time.Second)), nil
	case errors.Is(err, pgx.ErrNoRows):
		return false, 0, fmt.Errorf("ratelimit: budget %q missing; call EnsureBudget at startup", s.budget.Key)
	default:
		return false, 0, err
	}
}

const syncDailyUsageSQL = `
UPDATE api_rate_budget SET
    daily_used         = $2,
    daily_window_start = (now() AT TIME ZONE daily_reset_tz)::date,
    updated_at         = now()
WHERE budget_key = $1`

// SyncDailyUsage overwrites the local daily counter with the provider's own
// reported consumption.
//
// Our counter and the provider's can drift: a request that consumed a credit
// upstream but failed locally is counted by them and not by us, and the
// UTC-midnight window boundary is an assumption rather than a verified fact.
// Reconciling against the authoritative number turns both into detectable
// conditions instead of silent ones.
//
// Twelve Data exposes this directly via GET /api_usage (daily_usage,
// plan_daily_limit). Call it periodically; until it has run across a window
// rollover, keep the configured DailyLimit padded below the documented figure.
func (s *Shared) SyncDailyUsage(ctx context.Context, used float64) error {
	if _, err := s.pool.Exec(ctx, syncDailyUsageSQL, s.budget.Key, used); err != nil {
		return fmt.Errorf("sync daily usage for %q: %w", s.budget.Key, err)
	}
	return nil
}

// DailyState reports the current window's consumption, for reconciliation and
// for reporting how much of the day's budget a pass has left.
type DailyState struct {
	Used        float64
	Limit       *float64
	WindowStart *time.Time
}

func (s *Shared) DailyState(ctx context.Context) (DailyState, error) {
	const q = `
SELECT daily_used, daily_limit, daily_window_start
FROM api_rate_budget WHERE budget_key = $1`
	var d DailyState
	err := s.pool.QueryRow(ctx, q, s.budget.Key).Scan(&d.Used, &d.Limit, &d.WindowStart)
	if err != nil {
		return d, fmt.Errorf("daily state for %q: %w", s.budget.Key, err)
	}
	return d, nil
}

// Stats returns a snapshot of coordination health.
func (s *Shared) Stats() Stats {
	return Stats{
		Granted:        s.granted.Load(),
		Throttle:       s.throttle.Load(),
		Degraded:       s.degraded.Load(),
		QuotaExhausted: s.quotaOut.Load(),
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

// resetTZ returns the configured daily-window timezone, defaulting to UTC.
//
// Defaulting to UTC rather than to EST is deliberate: a budget with no daily
// limit does not use this column at all, and silently giving every budget a
// non-UTC boundary would make the setting invisible where it matters. Providers
// with a documented non-UTC reset set it explicitly in SharedTiingo and
// SharedTwelveData.
func (b Budget) resetTZ() string {
	if b.DailyResetTZ == "" {
		return "UTC"
	}
	return b.DailyResetTZ
}
