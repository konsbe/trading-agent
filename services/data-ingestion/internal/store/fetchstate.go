package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// data-fundamental sub-task names for fundamental_fetch_state.task.
//
// Only TaskMetrics is widened to the eligible universe in Phase 1 (spec §8.4);
// the other sub-tasks still iterate the static symbol list and have no rows in
// this table.
const (
	TaskMetrics = "metrics"
)

// Fetch-state status values. Deliberately the same vocabulary as the bar
// backfill's, in a separate table — see the 3 a.m. note in
// 009_fundamental_fetch_state.sql for why the shape is shared but the storage
// is not.
const (
	FetchPending    = "pending"
	FetchInProgress = "in_progress"
	FetchDone       = "done"
	FetchFailed     = "failed"
)

// SeedFundamentalFetchState creates missing (symbol, task) rows for every
// eligible universe symbol.
//
// Idempotent, and safe to call at the start of every round: newly-listed symbols
// get a 'pending' row and are picked up immediately, while existing rows keep
// their status, attempt count and success timestamp untouched.
// scope restricts which symbols are seeded. It has to match the scope used by
// ResolveMetricsSymbols: seeding the full eligible universe while the static
// pass iterates only the pilot draw would leave ~4,500 permanently-pending
// checkpoint rows, and the checkpointed pass would then work through all of them
// anyway — making the scope setting cosmetic rather than a real cost saving.
func SeedFundamentalFetchState(ctx context.Context, pool *pgxpool.Pool, task string, scope MetricsScope) (int64, error) {
	var q string
	switch scope {
	case ScopeSelected:
		q = `
INSERT INTO fundamental_fetch_state (symbol, task)
SELECT u.symbol, $1 FROM universe_symbols u WHERE u.is_eligible AND u.backfill_selected
ON CONFLICT (symbol, task) DO NOTHING`
	case ScopeEligible, "":
		q = `
INSERT INTO fundamental_fetch_state (symbol, task)
SELECT u.symbol, $1 FROM universe_symbols u WHERE u.is_eligible
ON CONFLICT (symbol, task) DO NOTHING`
	default:
		return 0, fmt.Errorf("seed fundamental fetch state (%s): unknown scope %q", task, scope)
	}
	ct, err := pool.Exec(ctx, q, task)
	if err != nil {
		return 0, fmt.Errorf("seed fundamental fetch state (%s): %w", task, err)
	}
	return ct.RowsAffected(), nil
}

// FetchClaim is one symbol leased for a fundamentals sub-task.
type FetchClaim struct {
	Symbol   string
	Task     string
	Attempts int
}

// ClaimFundamentalFetchBatch leases up to `limit` symbols for a sub-task.
//
// Claimable rows, in priority order:
//
//  1. 'pending'     — never fetched
//  2. 'done' whose last_success_ts is older than `staleAfter` — the weekly cycle
//  3. 'in_progress' claimed longer ago than `lease` — abandoned by a killed worker
//  4. 'failed' under `maxAttempts`
//
// Cadence is emergent from case 2 rather than driven by a reset job, so there is
// no cycle boundary to coordinate and a mid-cycle addition is not delayed.
//
// Case 3 is what makes the pass resumable rather than merely restartable: a crash
// leaves rows in_progress, and without the lease they would never be retried and
// the pass would quietly finish incomplete.
//
// Ordering puts never-fetched and longest-stale symbols first, so an interrupted
// pass resumes where coverage is thinnest instead of re-walking from 'A'.
func ClaimFundamentalFetchBatch(
	ctx context.Context,
	pool *pgxpool.Pool,
	task string,
	limit int,
	staleAfter, lease time.Duration,
	maxAttempts int,
) ([]FetchClaim, error) {
	if limit <= 0 {
		return nil, nil
	}
	const q = `
WITH claimable AS (
    SELECT symbol, task
    FROM fundamental_fetch_state
    WHERE task = $1
      AND (
            status = 'pending'
         OR (status = 'done'        AND (last_success_ts IS NULL OR last_success_ts < now() - $3::interval))
         OR (status = 'in_progress' AND claimed_at < now() - $4::interval)
         OR (status = 'failed'      AND attempts < $5)
      )
    ORDER BY
        CASE status
            WHEN 'pending'     THEN 0
            WHEN 'in_progress' THEN 1
            WHEN 'done'        THEN 2
            WHEN 'failed'      THEN 3
            ELSE 4
        END,
        last_success_ts NULLS FIRST,
        attempts,
        symbol
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
UPDATE fundamental_fetch_state f SET
    status     = 'in_progress',
    claimed_at = now(),
    updated_at = now()
FROM claimable c
WHERE f.symbol = c.symbol AND f.task = c.task
RETURNING f.symbol, f.task, f.attempts`

	rows, err := pool.Query(ctx, q, task, limit, staleAfter, lease, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("claim fundamental fetch batch (%s): %w", task, err)
	}
	defer rows.Close()
	var out []FetchClaim
	for rows.Next() {
		var c FetchClaim
		if err := rows.Scan(&c.Symbol, &c.Task, &c.Attempts); err != nil {
			return nil, fmt.Errorf("scan fetch claim: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkFundamentalFetchDone records a successful fetch.
//
// last_success_ts advances here and only here, which is what drives the weekly
// cadence: a symbol is not considered fresh because it was attempted, only
// because it succeeded.
func MarkFundamentalFetchDone(ctx context.Context, pool *pgxpool.Pool, symbol, task string) error {
	const q = `
UPDATE fundamental_fetch_state SET
    status          = 'done',
    claimed_at      = NULL,
    completed_at    = now(),
    last_success_ts = now(),
    last_error      = NULL,
    updated_at      = now()
WHERE symbol = $1 AND task = $2`
	if _, err := pool.Exec(ctx, q, symbol, task); err != nil {
		return fmt.Errorf("mark fetch done %s/%s: %w", symbol, task, err)
	}
	return nil
}

// MarkFundamentalFetchFailed records a failed attempt.
//
// last_success_ts is deliberately left alone: a failure must not make a symbol
// look recently refreshed, or a persistently broken symbol would drop out of the
// claim rotation and its staleness would become invisible.
func MarkFundamentalFetchFailed(ctx context.Context, pool *pgxpool.Pool, symbol, task, reason string) error {
	if len(reason) > 500 {
		reason = reason[:500]
	}
	const q = `
UPDATE fundamental_fetch_state SET
    status       = 'failed',
    claimed_at   = NULL,
    completed_at = now(),
    attempts     = attempts + 1,
    last_error   = $3,
    updated_at   = now()
WHERE symbol = $1 AND task = $2`
	if _, err := pool.Exec(ctx, q, symbol, task, reason); err != nil {
		return fmt.Errorf("mark fetch failed %s/%s: %w", symbol, task, err)
	}
	return nil
}

// FetchProgress is the state of a sub-task across the eligible universe.
type FetchProgress struct {
	Total      int
	Pending    int
	InProgress int
	Done       int
	Failed     int

	// Exhausted counts failed rows past maxAttempts — no longer claimed, and the
	// ones needing human attention.
	Exhausted int

	// Fresh counts rows whose last success is inside the refresh interval. This
	// is the real coverage number: 'done' only means the last attempt worked,
	// while Fresh means the data is current.
	Fresh int
}

func LoadFundamentalFetchProgress(
	ctx context.Context,
	pool *pgxpool.Pool,
	task string,
	maxAttempts int,
	staleAfter time.Duration,
) (FetchProgress, error) {
	var p FetchProgress
	const q = `
SELECT count(*),
       count(*) FILTER (WHERE status = 'pending'),
       count(*) FILTER (WHERE status = 'in_progress'),
       count(*) FILTER (WHERE status = 'done'),
       count(*) FILTER (WHERE status = 'failed'),
       count(*) FILTER (WHERE status = 'failed' AND attempts >= $2),
       count(*) FILTER (WHERE last_success_ts IS NOT NULL AND last_success_ts >= now() - $3::interval)
FROM fundamental_fetch_state WHERE task = $1`
	err := pool.QueryRow(ctx, q, task, maxAttempts, staleAfter).Scan(
		&p.Total, &p.Pending, &p.InProgress, &p.Done, &p.Failed, &p.Exhausted, &p.Fresh)
	if err != nil {
		return p, fmt.Errorf("fundamental fetch progress (%s): %w", task, err)
	}
	return p, nil
}

// ResolveMetricsSymbols returns the union of a configured symbol list and the
// eligible universe (spec §8.4).
//
// UNION, NOT REPLACEMENT, and that is load-bearing. The configured list contains
// symbols that are structurally absent from universe_symbols — SPY is an ETF and
// §3.1 excludes ETFs — so replacing the list would silently drop the most visible
// symbol in the existing daily report. A structural change must not quietly
// degrade something that already works.
//
// On query failure the caller is expected to fall back to the configured list
// alone rather than fetching nothing; this function surfaces the error so that
// choice is explicit at the call site.
// MetricsScope selects which slice of the universe the fundamentals pass covers.
type MetricsScope string

const (
	// ScopeEligible is the full §3.1 eligible universe (~4,975 symbols).
	ScopeEligible MetricsScope = "eligible"

	// ScopeSelected restricts the universe half of the union to the pilot subset
	// (universe_symbols.backfill_selected).
	//
	// This is the Phase 1 default, and it is a cost decision rather than a
	// correctness one. Step 7 evaluates the 450-symbol draw and nothing else, so
	// resolving the full eligible universe would spend ~3.3 hours of Finnhub
	// budget on ~4,500 symbols that take no part in the evaluation. Scoped to the
	// draw the same pass is ~15 minutes, which is the difference between getting
	// a base-rate result today and getting one tomorrow.
	//
	// Widen to ScopeEligible only once the pilot has shown the scoring approach
	// is worth extending — the same sequencing §2.2 applies to paying for data.
	ScopeSelected MetricsScope = "selected"
)

// ResolveMetricsSymbols unions the configured symbol list with a slice of the
// universe, preserving the configured ordering.
//
// scope controls only the universe half. Configured symbols are ALWAYS included
// regardless of scope: they are existing consumers' watchlists, and silently
// dropping them because a pilot flag was set would break unrelated features —
// the union exists precisely so the momentum work cannot narrow what already
// works.
func ResolveMetricsSymbols(ctx context.Context, pool *pgxpool.Pool, configured []string, scope MetricsScope) ([]string, error) {
	seen := make(map[string]struct{}, len(configured)+4096)
	out := make([]string, 0, len(configured)+4096)

	// Configured symbols first, so existing consumers keep their ordering and are
	// refreshed before the long universe tail on any partial pass.
	for _, s := range configured {
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	// A scope value that is neither known constant is a configuration mistake and
	// must not silently fall back to the expensive branch.
	var q string
	switch scope {
	case ScopeSelected:
		q = `SELECT symbol FROM universe_symbols WHERE is_eligible AND backfill_selected ORDER BY symbol`
	case ScopeEligible, "":
		q = `SELECT symbol FROM universe_symbols WHERE is_eligible ORDER BY symbol`
	default:
		return out, fmt.Errorf("resolve metrics symbols: unknown scope %q (want %q or %q)",
			scope, ScopeEligible, ScopeSelected)
	}
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return out, fmt.Errorf("resolve metrics symbols: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return out, fmt.Errorf("scan metrics symbol: %w", err)
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// ErrNoFetchStateRow reports that a (symbol, task) row is absent, which means
// seeding has not run for it.
var ErrNoFetchStateRow = pgx.ErrNoRows
