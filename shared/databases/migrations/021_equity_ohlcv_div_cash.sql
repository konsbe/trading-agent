-- 021: capture Tiingo's divCash, and with it the second half of the
-- corporate-action signal.
--
-- WHY NOW
--
-- Two open problems close on this column.
--
-- 1. THE SELF-INFLICTED SEAM. Adjusted prices are rewritten BACKWARDS on every
--    split and every dividend. The daily incremental refresh fetches only a
--    short recent window, so the moment a symbol has a corporate action after
--    its backfill, its stored history sits on the OLD adjustment basis while
--    the newly fetched bars sit on the NEW one. Windowed features then read a
--    step that no market participant experienced.
--
--    That is precisely the defect that disqualified Twelve Data — except
--    self-inflicted, slowly, one symbol at a time, and invisible because each
--    individual refresh looks correct.
--
--    `split_factor` (migration 012) detects the split half. `div_cash` detects
--    the dividend half. Without both, a dividend-only adjustment would slip
--    through the re-fetch trigger and produce exactly the seam described.
--
-- 2. THE UNRESOLVED BAR-AUDIT QUESTION. Phase 1 §2.3 recorded ~840 flagged
--    adjustment seams with no corresponding reported split, and could not say
--    whether they were provider defects or detector over-flagging, because
--    "dividend adjustment" could not be ruled out — divCash was not captured.
--    It is now.
--
-- NULLABLE, and never defaulted to zero: NULL means "not captured" (bars
-- predating this migration, or a provider that does not report it), which is
-- distinguishable from a genuine 0.0 meaning "no dividend that session".
-- Defaulting to 0 would silently assert "no dividend" for every historical bar
-- and disarm the very trigger this column exists to arm.

ALTER TABLE equity_ohlcv
    ADD COLUMN IF NOT EXISTS div_cash DOUBLE PRECISION;

COMMENT ON COLUMN equity_ohlcv.div_cash IS
'Tiingo divCash for this session: the cash dividend with an ex-date on this bar,
0.0 on an ordinary day. Together with split_factor it is the complete
corporate-action signal, and the daily refresh re-fetches a symbol''s FULL
history whenever either fires — because both rewrite the adjusted series
backwards, and appending new-basis bars to old-basis history creates an
adjustment seam. NULL means not captured, which is NOT the same as 0.0.';
