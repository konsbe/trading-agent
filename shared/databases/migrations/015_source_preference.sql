-- 015: centralise "which source wins" for latest-row queries.
--
-- THE BUG THIS EXISTS TO PREVENT
--
-- Several tables are written by more than one provider, and some of those
-- providers collide on the same logical key AND the same timestamp:
--
--   equity_fundamentals   4,912 colliding (symbol, metric, period, ts) groups
--                         across 5 sources
--   equity_ohlcv          7 colliding (symbol, interval, ts) groups, 3 sources
--   catalyst_events       3 colliding (symbol, ts) groups, 4 sources
--
-- A `SELECT DISTINCT ON (symbol) ... ORDER BY symbol, ts DESC` over such a
-- table is NOT deterministic. When two rows tie on ts, Postgres may return
-- either, and "either" includes a row whose value is NULL.
--
-- That is not hypothetical. `/stock/metric` does not report a share count, so
-- its `shares_outstanding` row carries NULL by design, while `/stock/profile2`
-- carries the number -- both at the same timestamp. The universe loader's
-- latest-row query picked the NULL one for **2,951 of 4,975 eligible symbols
-- (59%)**, silently discarding a share count that was present in the table.
-- Nothing errored, and `universe_symbols.shares_outstanding` simply read NULL.
--
-- The same defect first surfaced in a coverage report, where it made
-- shares_outstanding look 38.4% covered when it is 98.1%. Finding it twice, in
-- unrelated code, is what makes it a class rather than an incident: any
-- latest-row query over a multi-source table has it unless the tie is broken
-- explicitly.
--
-- WHY A DATABASE FUNCTION RATHER THAN A CONVENTION
--
-- The preference has to hold in Go (two services), Python (the bot), and
-- ad-hoc SQL. Written as a comment it drifts; written as a helper in one
-- language the other two do not see it. As a SQL function it is one reviewable
-- definition that every caller shares, and changing the policy is a migration
-- rather than a search-and-replace.
--
-- IMMUTABLE so the planner can inline it; STRICT is deliberately NOT used,
-- because a NULL source must rank last rather than make the whole call NULL.

CREATE OR REPLACE FUNCTION fundamental_source_rank(src TEXT)
RETURNS INTEGER
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE src
        -- Finnhub's dedicated endpoints first: each is authoritative for the
        -- fields it actually reports.
        WHEN 'finnhub_profile2'            THEN 100  -- shareOutstanding, sector
        WHEN 'finnhub_metric'              THEN 90   -- marketCapitalization, TTM ratios
        WHEN 'finnhub_financials_reported' THEN 80   -- as-reported statements
        WHEN 'finnhub_earnings'            THEN 70
        -- Alpha Vantage last: it is the fallback overview source, and its
        -- rate limit means its rows are often the stalest present.
        WHEN 'alphavantage_overview'       THEN 10
        ELSE 0
    END;
$$;

COMMENT ON FUNCTION fundamental_source_rank(TEXT) IS
'Preference order for equity_fundamentals rows that tie on (symbol, metric,
period, ts). HIGHER wins. Use as a tiebreaker in every DISTINCT ON / latest-row
query over that table, AFTER a NULL-last clause:

  ORDER BY symbol, metric, ts DESC, (value IS NULL), fundamental_source_rank(source) DESC

The NULL-last clause comes FIRST and is not optional. Ranking sources alone is
not enough: finnhub_metric outranks alphavantage_overview, but its
shares_outstanding value is NULL by design, so a source-only ordering would
still choose NULL over a real number from a lower-ranked source. Prefer any
value to no value, then prefer the better source.';

CREATE OR REPLACE FUNCTION bar_source_rank(src TEXT)
RETURNS INTEGER
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT CASE src
        -- Tiingo is the Phase 1 primary: split AND dividend adjusted with one
        -- consistent factor per series (verified), and the only source that
        -- populates raw_close / split_factor.
        WHEN 'tiingo'        THEN 100
        WHEN 'yahoo_finance' THEN 50
        -- finnhub_quote is a single live quote, not a historical bar series.
        -- It must never outrank a real bar.
        WHEN 'finnhub_quote' THEN 10
        -- Twelve Data is deliberately last: its volume adjustment alternates
        -- between raw and adjusted within one response, which the barquality
        -- detector flagged as corporate-action fabrication.
        WHEN 'twelve_data'   THEN 5
        ELSE 0
    END;
$$;

COMMENT ON FUNCTION bar_source_rank(TEXT) IS
'Preference order for equity_ohlcv rows that tie on (symbol, interval, ts).
HIGHER wins. Matches the ranking QueryEquityBars already applied inline, which
was the one latest-row query in the repo that broke the tie explicitly; this
makes that behaviour the shared default rather than a local habit.';
