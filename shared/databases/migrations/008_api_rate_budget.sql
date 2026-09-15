-- Shared API rate budgets, coordinated across processes.
--
-- Problem this solves: every worker that talks to a third-party API builds its
-- own in-process token bucket, so N workers sharing one API key permit N times
-- the intended rate. For Finnhub's free tier (60 req/min = 1 req/s) the five
-- Go workers holding a client each allow ~0.5 req/s, summing to ~2.5 req/s —
-- 250% of budget.
--
-- That has been harmless so far only because observed demand is ~10% of budget:
-- every worker polls a handful of symbols on a 60-second tick and none comes
-- near its own allowance. It stops being harmless the moment one caller
-- saturates its allowance for hours (the momentum scanner's universe-wide
-- fundamentals pass), because the 429s then surface in whichever *other* worker
-- happens to be running — symptom and cause in different services.
--
-- Postgres rather than Redis deliberately: every one of these workers already
-- holds a pgxpool and already treats "Postgres down" as total outage, so this
-- adds no new partial-failure surface. Redis would add a second independently
-- failing dependency to four services that currently have none. A Redis
-- fixed-window scheme also resets on every rolling restart, handing out a free
-- burst per deploy — an occasional unexplained 429 spike that is very hard to
-- trace back. Rows here survive restarts.
--
-- Contention is ~1 acquisition/second aggregate. Trivial for Postgres.
--
-- Regular table, not a hypertable: this is current state, not time-series.

CREATE TABLE IF NOT EXISTS api_rate_budget (
    -- One row per upstream quota. Keyed by the QUOTA, not by the worker —
    -- 'finnhub' is one budget shared by every caller using FINNHUB_API_KEY.
    budget_key     TEXT             NOT NULL,

    -- Current token count. Fractional, and refilled lazily at read time from
    -- refill_per_sec * (now() - updated_at) rather than by a background job, so
    -- there is nothing to schedule and nothing to drift.
    tokens         DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- Sustained rate. 1.0 for Finnhub free tier (60 requests/minute).
    refill_per_sec DOUBLE PRECISION NOT NULL,

    -- Maximum tokens that can accumulate, i.e. the largest allowed burst.
    burst          DOUBLE PRECISION NOT NULL,

    -- Advanced on every successful acquisition; the lazy-refill baseline.
    updated_at     TIMESTAMPTZ      NOT NULL DEFAULT now(),

    PRIMARY KEY (budget_key)
);

-- Acquisition is a single UPDATE whose WHERE clause repeats the refill
-- expression, so granting and deducting happen in one atomic statement:
--
--   UPDATE api_rate_budget SET
--       tokens     = LEAST(burst, tokens + refill_per_sec * elapsed) - 1,
--       updated_at = now()
--   WHERE budget_key = $1
--     AND LEAST(burst, tokens + refill_per_sec * elapsed) >= 1
--   RETURNING tokens;
--
-- Two concurrent callers cannot both be granted the last token: the second
-- blocks on the row lock taken by the first, and under READ COMMITTED it then
-- re-evaluates the WHERE clause against the committed row. If the balance has
-- dropped below 1 the predicate fails and it returns zero rows. No explicit
-- FOR UPDATE and no advisory lock are needed.
