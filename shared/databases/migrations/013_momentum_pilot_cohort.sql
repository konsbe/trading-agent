-- 013: freeze the identity of the 450-symbol Phase 1 pilot cohort.
--
-- WHY THIS TABLE EXISTS AT ALL
--
-- The 450 pilot symbols are IN-SAMPLE for §4 v2. Its weights were revised three
-- times against the 218 candidates those symbols produced, so any statistic
-- computed over them measures the fit, not the edge. Every report from the
-- post-upgrade validation onward has to separate them from genuinely new data,
-- which requires knowing exactly which 450 they were.
--
-- WHY backfill_selected CANNOT BE THAT RECORD
--
-- Until now the cohort was identifiable only as `universe_symbols.backfill_selected`.
-- That flag is the backfill job's WORKING SET, and the selection routine opens
-- with:
--
--     UPDATE universe_symbols SET backfill_selected = false WHERE backfill_selected;
--
-- Widening the pilot to the full universe means selecting all ~4,975 symbols,
-- which sets the flag on every one of them. At that moment the in-sample cohort
-- becomes unrecoverable -- not corrupted in a way that shows up, but silently
-- merged into the out-of-sample population, which would make the OOS report
-- read better than the truth. There is no backup of a boolean.
--
-- So the cohort is copied into a table of its own BEFORE the universe is
-- widened. The flag keeps its original meaning (what the backfill is working
-- on) and this table carries the archival one (who was in-sample), instead of
-- one column trying to be both.
--
-- CONTENT HASH
--
-- Stored so a later report can prove it used the same 450 that Phase 1 fitted
-- on. Phase 1's whole problem was that nobody could tell in-sample from
-- out-of-sample after the fact; a hash makes that checkable rather than
-- remembered.

CREATE TABLE IF NOT EXISTS momentum_pilot_cohort (
    symbol      TEXT PRIMARY KEY,
    exchange    TEXT        NOT NULL,
    reserved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    note        TEXT
);

COMMENT ON TABLE momentum_pilot_cohort IS
'The 450 symbols of the Phase 1 pilot, frozen 2026-09-21 before the universe was
widened to ~4,975. These are IN-SAMPLE for §4 v2: its weights were revised three
times against the 218 candidates they produced. Population A in the post-upgrade
report is defined by membership here. Excluding them is what makes populations B
and C out-of-sample, so this table is a correctness dependency of every OOS
claim, not bookkeeping.';

COMMENT ON COLUMN momentum_pilot_cohort.symbol IS
'Symbol as carried in universe_symbols, i.e. the Finnhub dot form (BRK.B), not
Tiingo''s hyphen form (BRK-B). Joins against universe_symbols and momentum_*
tables are therefore direct; only the Tiingo request path translates.';

-- Populated from the live flag in the same migration that creates the table, so
-- there is no window in which the table exists but the cohort has moved on.
INSERT INTO momentum_pilot_cohort (symbol, exchange, note)
SELECT symbol, exchange, 'phase 1 pilot; free-tier 500-unique-symbol cap'
FROM universe_symbols
WHERE backfill_selected
ON CONFLICT (symbol) DO NOTHING;

-- Content hash of the frozen set, recorded where it cannot drift from the rows.
CREATE TABLE IF NOT EXISTS momentum_cohort_manifest (
    cohort_key   TEXT PRIMARY KEY,
    symbol_count INTEGER     NOT NULL,
    content_hash TEXT        NOT NULL,
    reserved_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    definition   TEXT        NOT NULL
);

COMMENT ON TABLE momentum_cohort_manifest IS
'Content hashes for frozen row sets (the pilot cohort, the Phase 2 lockbox).
The hash is over the sorted member keys, so a set that gains or loses a member
produces a different hash and any report claiming to use the original can be
shown not to. Phase 2 §4.3 requires this for the lockbox; the pilot cohort gets
the same treatment because the same argument applies to it.';

INSERT INTO momentum_cohort_manifest (cohort_key, symbol_count, content_hash, definition)
SELECT 'phase1_pilot_450',
       count(*),
       md5(string_agg(symbol, ',' ORDER BY symbol)),
       'universe_symbols.backfill_selected = true as of 2026-09-21, immediately '
       'before TIINGO_MAX_SELECTED_SYMBOLS was raised from 450 and the universe '
       'widened. In-sample for momentum score v2.'
FROM momentum_pilot_cohort
ON CONFLICT (cohort_key) DO NOTHING;
