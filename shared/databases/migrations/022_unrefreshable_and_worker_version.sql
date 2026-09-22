-- 022: two operational facts that were being carried in people's heads.
--
-- ============================================================================
-- 1. UNREFRESHABLE SYMBOLS
-- ============================================================================
--
-- Some symbols' back history simply stops being served. Measured examples:
--
--   HWH   Tiingo returns 0 bars for an 11-year window; we hold 678
--   JCSE  Tiingo returns 1 bar for an 11-year window; we hold 1,107
--
-- Their stored bars can never be re-fetched onto the current adjustment basis,
-- so they will fail the seam audit forever and their corporate-action columns
-- stay NULL. More importantly, a symbol the provider will not serve should not
-- be quietly SCORED: the feature engine would compute a 20-day RVOL and a
-- 252-bar high from a frozen remnant and emit a candidate that looks exactly
-- like a live one.
--
-- This is also survivorship acting at the PROVIDER layer rather than in our
-- own universe filter, which is a distinct mechanism from the one Phase 2 §3.3
-- is about, and it needs recording where §3.3's work will trip over it.
--
-- Nullable and reason-carrying rather than a boolean: "we do not serve this"
-- and "this delisted" and "this was renamed" have different consequences, and
-- a bare flag would flatten them.

ALTER TABLE universe_symbols
    ADD COLUMN IF NOT EXISTS data_unavailable_reason TEXT,
    ADD COLUMN IF NOT EXISTS data_unavailable_since  TIMESTAMPTZ;

COMMENT ON COLUMN universe_symbols.data_unavailable_reason IS
'Set when the bar provider no longer serves this symbol''s history, so its
stored bars are a frozen remnant that cannot be refreshed. Such symbols are
EXCLUDED from scanning: scoring a remnant produces a candidate indistinguishable
from a live one. NULL means the symbol is refreshable as normal. Carries a
reason rather than being a boolean because "provider returns nothing",
"delisted" and "renamed" have different downstream consequences.';

-- ============================================================================
-- 2. WORKER BUILD VERSION ON CLAIMED ROWS
-- ============================================================================
--
-- An hour was lost to this exact failure: a data-universe process launched the
-- previous evening was still running on the OLD binary while a freshly built
-- one worked the same `backfill_status='pending'` queue. Rebuilding the file
-- does not restart a running process. Both instances claimed rows atomically,
-- neither errored, and the only symptom was a column populated for 64.5% of
-- bars -- which reads as a bug in the column, not as two writers.
--
-- Stamping the claiming worker's build version onto the row makes that
-- diagnosable in one query instead of by noticing an odd percentage.
--
-- The version is a content hash of the running executable, computed at startup.
-- That works without git (this repo has no VCS metadata) and, unlike a
-- hand-maintained version string, cannot be forgotten on a rebuild.

ALTER TABLE universe_symbols
    ADD COLUMN IF NOT EXISTS backfill_worker_version TEXT;

COMMENT ON COLUMN universe_symbols.backfill_worker_version IS
'Build version of the worker that last CLAIMED this row: a content hash of the
running executable, computed at startup. Exists so that two binaries working
one queue is a visible fact rather than an inference from anomalous data. Query
for more than one distinct value among recently-claimed rows; see
scripts/worker_versions.sql.';

-- Record what is already known, so the audit and the scanner agree immediately.
UPDATE universe_symbols
   SET data_unavailable_reason = 'tiingo serves 0 bars for an 11-year window; '
                                 'stored history is a frozen remnant (verified 2026-09-22)',
       data_unavailable_since  = now()
 WHERE symbol = 'HWH' AND data_unavailable_reason IS NULL;

UPDATE universe_symbols
   SET data_unavailable_reason = 'tiingo serves 1 bar for an 11-year window; '
                                 'stored history is a frozen remnant (verified 2026-09-22)',
       data_unavailable_since  = now()
 WHERE symbol = 'JCSE' AND data_unavailable_reason IS NULL;
