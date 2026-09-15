-- Per-symbol, per-sub-task fetch checkpoint for data-fundamental.
--
-- Spec: docs/MOMENTUM_SCANNER_PHASE1.md §8.4 (build-order step 3b).
--
-- Why this exists: data-fundamental's sub-tasks iterate a static symbol list and
-- re-fetch all of it on every tick. That is fine for the three configured
-- symbols. It is not fine once runMetrics is widened to the eligible universe,
-- where one pass is ~3.3 hours at the shared Finnhub rate — a multi-hour
-- rate-limited pass must survive a restart the same way the bar backfill does.
--
-- Why a SEPARATE table rather than reusing universe_symbols.backfill_*:
--
--   Would these two failures be distinguishable at 3 a.m.?
--
-- The bar backfill and the fundamentals fetch want identically-shaped state, and
-- that identical shape is exactly what would make them confusable. A fundamentals
-- failure surfacing in a column named `backfill_last_error` sends an on-call
-- reader to the wrong pipeline. One state table per job; the extra table costs a
-- migration and nothing else. See shared/schemas/SCHEMAS.md.
--
-- Why keyed by (symbol, task) when only one task is widened today: the key IS the
-- lease granularity, so a symbol whose metrics call succeeded and whose (future)
-- earnings call failed retries only earnings. Costs nothing now, and avoids a
-- migration when the Alpha Vantage quota wall (§3.13) is eventually lifted and
-- runOverview becomes a widening candidate.
--
-- Regular table, not a hypertable: current state, not time-series.

CREATE TABLE IF NOT EXISTS fundamental_fetch_state (
    symbol          TEXT             NOT NULL,

    -- Which data-fundamental sub-task this row tracks. 'metrics' is the only
    -- widened task in Phase 1; the others still iterate the static env list and
    -- have no rows here.
    task            TEXT             NOT NULL,

    -- 'pending' | 'in_progress' | 'done' | 'failed'
    status          TEXT             NOT NULL DEFAULT 'pending',

    -- Set when 'in_progress' is claimed. A claim older than the configured lease
    -- is reclaimable: without this, a worker killed mid-symbol would strand the
    -- row forever and the pass would silently finish incomplete.
    claimed_at      TIMESTAMPTZ,

    attempts        INTEGER          NOT NULL DEFAULT 0,
    last_error      TEXT,

    -- When the last attempt finished, successful or not.
    completed_at    TIMESTAMPTZ,

    -- When data was last actually stored. Distinct from completed_at on purpose:
    -- the weekly cadence is driven by "how long since this symbol's data was
    -- genuinely refreshed", so a run of failures must not look like freshness.
    last_success_ts TIMESTAMPTZ,

    updated_at      TIMESTAMPTZ      NOT NULL DEFAULT now(),

    PRIMARY KEY (symbol, task)
);

-- The claim query filters on (task, status) and orders by staleness.
CREATE INDEX IF NOT EXISTS ffs_claimable
    ON fundamental_fetch_state (task, status, last_success_ts NULLS FIRST);

CREATE INDEX IF NOT EXISTS ffs_symbol
    ON fundamental_fetch_state (symbol);

-- Cadence is emergent from the data rather than driven by a reset job: a row is
-- claimable when it is 'pending', when its last success is older than the refresh
-- interval, when it failed under the attempt cap, or when its claim has expired.
--
-- That has a useful property — a symbol newly added to the universe is 'pending'
-- and gets fetched on the next round, while symbols already refreshed this cycle
-- wait out their interval. No cycle boundary to coordinate, and nothing to reset.
