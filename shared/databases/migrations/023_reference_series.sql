-- 023: reference series (market indices, volatility) kept OUT of the universe.
--
-- WHY A SEPARATE TABLE RATHER THAN equity_ohlcv + universe_symbols
--
-- Phase 2 §3.6's regime features need SPY and IWM 20-day returns and the VIX.
-- None of those is a candidate. SPY and IWM are ETFs, which §3.1 excludes by
-- instrument type, and the VIX is not a tradable security at all.
--
-- Putting them in equity_ohlcv and universe_symbols would make every
-- eligibility filter, every report scope and the scanner itself responsible
-- for remembering to exclude three symbols. That is precisely the shape of
-- mistake this project keeps finding: a rule enforced by everyone remembering
-- rather than by construction. The universe would also silently gain three
-- members, moving the denominator guard's expected count and every base rate
-- computed from it.
--
-- So they live here. Nothing joins this table to universe_symbols, and a test
-- asserts they never appear in the universe, in a report scope, or in the
-- scanner's input (see reference_series_isolation_test.go).
--
-- AMENDED 2026-09-25: SPY, IWM etc. MAY ALSO HAVE ROWS IN equity_ohlcv — deliberate and safe
--
-- The daily market report (data-technical -> macro-analysis's market cycle and
-- intermarket correlations, and the bot's price cards) reads its benchmark and
-- instrument bars from equity_ohlcv, and has since before this migration. So
-- data-technical writes SPY, IWM, QQQ, GLD, USO, ... there as source
-- 'yahoo_finance'. That does not reintroduce the risk above, which was never
-- "these symbols have bars in equity_ohlcv" but "they become scanner
-- candidates". They cannot, and the tests enforce every layer that could leak
-- them: not in universe_symbols, never eligible, absent from the report-scope
-- join and from the scanner's exact input query, and never stored under the
-- scanner's source ('tiingo'). The regime features still read THIS table.
-- If you find SPY in equity_ohlcv, that is expected; if you find it in
-- universe_symbols, or under source 'tiingo', a test has already failed.
--
-- AS-OF t-1
--
-- §3.6 requires regime inputs joined as of t-1, not t. Two reasons, and both
-- are correctness rather than caution:
--   * FRED series can publish after the close, so the value stamped t may not
--     have existed when a t-dated scan ran.
--   * An index return computed through t uses the same session the candidate's
--     own features come from, which lets the market's move on that day inform
--     a feature about that day.
-- The t-1 join is applied at read time; this table stores the raw dated
-- series, so the lag is visible in the query rather than baked into the data.

CREATE TABLE IF NOT EXISTS reference_series (
    series_id TEXT        NOT NULL,
    ts        DATE        NOT NULL,
    value     DOUBLE PRECISION NOT NULL,
    source    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (series_id, ts)
);

CREATE INDEX IF NOT EXISTS ref_series_lookup ON reference_series (series_id, ts DESC);

COMMENT ON TABLE reference_series IS
'Market-wide reference data for Phase 2 §3.6 regime features: index closes
(SPY, IWM) and volatility (VIXCLS). Never in universe_symbols — these are not
candidates, so the eligibility filter, reportscope and the scanner cannot
include them. (SPY/IWM may also have yahoo_finance bars in equity_ohlcv for the
daily market report — deliberate and safe; see the 2026-09-25 note in migration
023.) Joined as-of t-1 at read time.';

COMMENT ON COLUMN reference_series.series_id IS
'SPY / IWM for index closes, VIXCLS for volatility. Namespaced separately from
equity_ohlcv.symbol so a ticker collision is impossible by construction.';
