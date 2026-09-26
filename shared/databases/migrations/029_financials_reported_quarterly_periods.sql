-- 029: relabel financials-reported 10-Q rows from "annual_<year>" to
-- "q_<period end>".
--
-- data-fundamental labelled every /stock/financials-reported report by a
-- "freq" field the report items do not have, so 10-Qs were stored as
-- annual_<year> next to that year's 10-K (INTC annual_2024 operating income:
-- 10-K -11,678 and 10-Q -1,069). The writer now labels a 10-Q q_YYYY-MM-DD
-- (period end), the shape finnhub_earnings already uses; this moves the rows
-- already stored.
--
-- Provenance: every metric row of one run of one report shares (symbol,
-- period, ts, source) with that report's report_raw row, whose payload has the
-- form and endDate. At the time of writing all 11,576 financials-reported rows
-- have such a report_raw row, and no fcf_reported row (the one metric written
-- conditionally) disagrees with its report's operating_cf_reported.
--
-- Rows are relabelled, never deleted; 10-K rows keep annual_<year>. Rows with
-- no report_raw, or a report_raw without a parseable endDate, are left alone.
-- Idempotent: a second run matches nothing (relabelled rows no longer start
-- with annual_). Rows whose target key already exists are skipped, not
-- overwritten.
--
-- Not recoverable here: within one run, all of a year's 10-Qs shared one key,
-- so each run kept only the last-written (the oldest of that year) and the
-- others were overwritten. The data-fundamental refetch restores them.

WITH tenq AS (
    SELECT symbol, period, ts, 'q_' || left(payload->>'endDate', 10) AS new_period
    FROM equity_fundamentals
    WHERE source = 'finnhub_financials_reported'
      AND metric = 'report_raw'
      AND period LIKE 'annual\_%'
      AND payload->>'form' LIKE '10-Q%'
      AND payload->>'endDate' ~ '^\d{4}-\d{2}-\d{2}'
)
UPDATE equity_fundamentals e
SET period = tenq.new_period
FROM tenq
WHERE e.source = 'finnhub_financials_reported'
  AND e.symbol = tenq.symbol
  AND e.period = tenq.period
  AND e.ts     = tenq.ts
  AND NOT EXISTS (
      SELECT 1 FROM equity_fundamentals x
      WHERE x.symbol = e.symbol AND x.period = tenq.new_period
        AND x.metric = e.metric AND x.source = e.source AND x.ts = e.ts);
