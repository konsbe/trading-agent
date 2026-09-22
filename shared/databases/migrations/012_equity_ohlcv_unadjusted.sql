-- 012: capture Tiingo's UNADJUSTED close and splitFactor alongside the adjusted
-- series, for Phase 2 §3.2 (point-in-time market cap).
--
-- WHY THIS EXISTS
--
-- Phase 2 §3.2 computes point-in-time market cap as
--     raw_close[t] x shares_outstanding(filed <= t)
-- and both factors must be UNADJUSTED. Multiplying an adjusted price by an
-- unadjusted share count is wrong by the cumulative split factor, which for
-- reverse-split penny names is 10-100x. That is not a rounding error; it moves a
-- symbol between the penny and market buckets, which is exactly the failure mode
-- the Phase 1 market-cap unit bug already demonstrated once.
--
-- WHY NOW, DURING THE POST-UPGRADE BACKFILL
--
-- The 10-year full-universe backfill re-fetches every bar anyway. Tiingo returns
-- close, adjClose and splitFactor in the SAME response, so capturing them costs
-- zero extra requests and zero extra bandwidth. Adding these columns later would
-- mean a second full pass over ~4,975 symbols to collect data we already had in
-- hand and threw away.
--
-- WHAT DOES NOT CHANGE
--
-- Every existing feature keeps reading the ADJUSTED fields. The `close` column
-- retains exactly its current meaning (split/dividend-adjusted), so no feature,
-- gate, score or label shifts because of this migration. These columns are
-- additive and read by nothing yet. That is deliberate: this step is the
-- post-upgrade validation run, which is a measurement step, and a column nobody
-- reads cannot change a measurement.
--
-- WHY NULLABLE
--
-- The 291,793 bars already in the table were fetched before these columns
-- existed and cannot be retro-filled without re-requesting them. NULL therefore
-- means "not captured", which is honest, and is distinguishable from a genuine
-- value. Phase 1 §12's rule against silent imputation applies: nothing may
-- default raw_close to close, because for a split-adjusted history those two
-- differ precisely where it matters most.

ALTER TABLE equity_ohlcv
    ADD COLUMN IF NOT EXISTS raw_close    DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS split_factor DOUBLE PRECISION;

COMMENT ON COLUMN equity_ohlcv.raw_close IS
'UNADJUSTED close as printed on the day, from Tiingo''s `close` field. The
`close` column on this table is the ADJUSTED series (Tiingo `adjClose`) and
remains the one every feature reads. This column exists only for Phase 2 §3.2''s
point-in-time market cap, which must multiply an unadjusted price by an
unadjusted share count. NULL means the bar predates migration 012 and was never
captured -- it does NOT mean the raw close equals the adjusted close, and must
never be defaulted to it.';

COMMENT ON COLUMN equity_ohlcv.split_factor IS
'Tiingo `splitFactor` for this session: 1.0 on an ordinary day, 2.0 on a 2:1
split, 0.1 on a 1:10 reverse split. Stored per-bar as reported, NOT cumulative;
a cumulative factor must be derived by multiplying forward from the date of
interest. Used by Phase 2 §3.2 to reconcile the raw and adjusted series, and as
a cross-check on the barquality adjustment-seam detector: a seam WITHOUT a
corresponding split factor is a provider defect rather than a corporate action.
NULL means not captured (bar predates migration 012).';
