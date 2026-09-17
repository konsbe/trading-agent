-- Two independent additions, one commit:
--   1. api_rate_budget gains a DAILY ceiling alongside its per-second bucket.
--   2. universe_symbols gains backfill_selected, marking the pilot subset.
--
-- Spec: docs/MOMENTUM_SCANNER_PHASE1.md §2.2. Both exist to support running the
-- scanner on two bar providers with structurally different quotas.
--
-- Why a new migration rather than amending 008/007: those are committed. By the
-- letter of the rule in shared/schemas/SCHEMAS.md amendment would still be
-- allowed — neither has been applied outside a throwaway container — but
-- "committed" is the line, not "pushed", because that is the first point at
-- which a local file and someone else's checkout of the same commit can
-- silently disagree about what a migration means.


-- ── 1. api_rate_budget: a daily ceiling ────────────────────────────────────
--
-- A rate limit and a daily quota are DIFFERENT MECHANISMS and are kept as
-- separate columns rather than merged into one concept:
--
--   tokens / refill_per_sec / burst   continuous refill — smooth pacing
--   daily_* below                     fixed window — hard stop at a boundary
--
-- Conflating them is where the bugs live. A token bucket can always eventually
-- grant a request; a spent daily quota cannot grant one until tomorrow, and the
-- caller has to be told which situation it is in.
--
-- Motivating case: Twelve Data's free tier is 8 requests/minute AND 800
-- credits/day. The DAILY cap binds first — 800 credits at 8/min is ~100 minutes
-- of work, after which the account is blocked until the window rolls. A limiter
-- expressing only the per-minute rate would pace happily into a wall.
--
-- ⚠️ THE HAZARD THIS INTRODUCES, AND WHY THE CALLER CONTRACT CHANGED
--
-- internal/ratelimit degrades to the caller's local in-process bucket whenever
-- coordination fails, and its stated guarantee is "never unlimited". That is
-- correct for its designed failure mode (Postgres briefly unreachable: we do not
-- know the state, so be conservative).
--
-- A spent daily quota is the opposite situation: we know the state exactly, and
-- it is zero. Falling back to the local bucket there would pace requests at
-- 8/min against an account with nothing left — turning a hard ceiling into a
-- wall of 429s, or on a paid tier into overage charges. The mechanism built to
-- prevent unlimited access would be the thing granting it, while the limiter's
-- own logs looked healthy.
--
-- So quota exhaustion is reported as ErrDailyQuotaExhausted and is NEVER routed
-- through the fallback, and never counted as degradation. The caller stops the
-- pass; Step 3's claim/lease machinery resumes it after the window rolls.
--
-- daily_limit NULL means "no daily ceiling", so every pre-existing row (finnhub,
-- and tiingo when it is added) keeps exactly today's behaviour. No backfill of
-- existing data is required.
ALTER TABLE api_rate_budget
    ADD COLUMN IF NOT EXISTS daily_limit        DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS daily_used         DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS daily_window_start DATE;

COMMENT ON COLUMN api_rate_budget.daily_limit IS
    'Hard requests-per-day ceiling. NULL = no daily ceiling (the pre-existing behaviour). Exhaustion yields ErrDailyQuotaExhausted, which is never routed through the limiter''s local fallback.';
COMMENT ON COLUMN api_rate_budget.daily_used IS
    'Requests consumed in the current daily window. Reset when daily_window_start rolls.';
COMMENT ON COLUMN api_rate_budget.daily_window_start IS
    'UTC calendar day the counter covers. The UTC-midnight boundary is an ASSUMPTION about the provider''s reset, not a verified fact — reconcile against the provider''s own reported usage before trusting it, and keep the configured limit padded below the documented one until then.';


-- ── 2. universe_symbols: the pilot subset marker ───────────────────────────
--
-- backfill_selected marks the curated/sampled subset used to validate the
-- pipeline end to end on a free tier before committing to the full ~4,978-symbol
-- universe (see §2.2 and the Step 7a note).
--
-- This flag is ALSO Tiingo's quota mechanism, which is worth stating plainly
-- because nothing else enforces it. Tiingo's free allowance is 500 UNIQUE
-- SYMBOLS PER MONTH — neither a rate nor a daily count, and therefore not
-- expressible in api_rate_budget at all. Re-touching an already-counted symbol
-- is free, so a stable subset refreshed daily stays inside the allowance
-- indefinitely, while widening the subset past ~450 silently spends it.
--
-- Consequence: no limiter will stop someone from widening the pilot. The guard
-- has to be an explicit assertion on subset size in the selection job, where it
-- fails loudly and immediately, rather than a month later when Tiingo starts
-- rejecting requests for reasons that look unrelated to the change that caused
-- them.
ALTER TABLE universe_symbols
    ADD COLUMN IF NOT EXISTS backfill_selected BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN universe_symbols.backfill_selected IS
    'Marks the pilot subset validated on a free-tier provider before the full universe. Also the de-facto enforcement of Tiingo''s 500-unique-symbols/month allowance, which api_rate_budget cannot express — the size guard lives in the selection job.';

-- The pilot backfill and daily refresh both iterate the selected subset.
CREATE INDEX IF NOT EXISTS us_selected
    ON universe_symbols (backfill_selected, symbol)
    WHERE is_eligible AND backfill_selected;
