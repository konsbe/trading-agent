-- 019: record WHICH XBRL concept each point-in-time share count came from.
--
-- WHY
--
-- `dei:EntityCommonStockSharesOutstanding` is a COVER-PAGE tag and not every
-- filer uses it: 796 of 4,952 eligible symbols (16.1%) return 404 for it. Those
-- symbols are not random — 48.7% of the uncovered listed after 2023-09, against
-- 16.2% of the covered — so excluding them biases the sample against recent
-- listings, which is the same direction as, and additive to, the momentum
-- strategy's population of interest.
--
-- `us-gaap:CommonStockSharesOutstanding` resolves for many of them, so it is
-- used as a FALLBACK where the dei concept is absent.
--
-- WHY THE PROVENANCE COLUMN IS NOT OPTIONAL
--
-- The two concepts are not the same measurement:
--
--   dei:EntityCommonStockSharesOutstanding   cover page, as of a date near the
--                                            filing, one figure for the class
--   us-gaap:CommonStockSharesOutstanding     balance sheet, as of the PERIOD
--                                            END, reported per class
--
-- The us-gaap figure is therefore systematically STALER: it describes the
-- period end, which precedes the filing by weeks. Both are joined as-of on
-- `filed_date` so neither is lookahead, but a fallback row describes a share
-- count from further in the past than a dei row filed the same day. That
-- difference has to be visible in the data, not just in a commit message,
-- because it is a candidate explanation for any result that changes when the
-- fallback is switched on.

ALTER TABLE shares_outstanding_pit
    ADD COLUMN IF NOT EXISTS concept TEXT NOT NULL DEFAULT 'dei:EntityCommonStockSharesOutstanding';

COMMENT ON COLUMN shares_outstanding_pit.concept IS
'The XBRL concept this row came from. dei:EntityCommonStockSharesOutstanding is
the primary (cover page); us-gaap:CommonStockSharesOutstanding is the fallback,
used ONLY where the dei concept is absent for that symbol. The fallback is a
balance-sheet figure as of period_end, so it is systematically staler than a dei
row filed on the same date -- compare filed_date against period_end to see by
how much. Any result that moves when the fallback is enabled must be checked
against this column before it is attributed to anything else.';

-- As-of lag, derived rather than stored, so it cannot drift from its inputs.
CREATE OR REPLACE VIEW shares_outstanding_pit_lag AS
SELECT symbol,
       filed_date,
       period_end,
       concept,
       multi_class,
       shares,
       (filed_date - period_end) AS asof_lag_days
FROM shares_outstanding_pit;

COMMENT ON VIEW shares_outstanding_pit_lag IS
'shares_outstanding_pit with the as-of lag (filed_date - period_end) computed.
The lag is how far in the past the share count actually is on the day it
becomes usable. Expected to be materially larger for the us-gaap fallback than
for the dei concept, because the former describes the period end and the latter
a date near the filing.';
