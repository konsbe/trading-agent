-- Golden fixture for momentum-scanner and momentum-tracker (scripts/momentum_golden.sh).
--
-- Every date is relative to :'D', the session under test, so the dump (which
-- prints dates as offsets from D) does not depend on the day it runs. Bars are
-- daily, 300 of them ending on D; the commands do not read the NYSE calendar.
--
--   ZGMKT   market-bucket gate pass (+15% on 5x volume, PIT cap ~$1.15B)
--   ZGPNY   penny-bucket gate pass (+20% on 6x volume), tier-A catalyst on D
--   ZGFLAT  no move: every gate fails
--   ZGSTALE market-sized move, but its only share filing is 20 months old
--   ZGOLD   tracked since D-10; drops 20% on D (an exit)
--   ZGRUN   tracked since D-3; still running on D (advanced)
--
-- With -v later=1 it also seeds what lands AFTER D, each chosen to change the
-- result if a point-in-time read of D leaked it: a D+1 bar for every symbol, a
-- ZGMKT share filing and a $50B market_cap on D+1, and a tier-A ZGMKT catalyst
-- on D+1.

INSERT INTO universe_symbols (symbol, exchange, is_eligible, backfill_status)
SELECT s, 'US', true, 'done'
FROM unnest(ARRAY['ZGMKT','ZGPNY','ZGFLAT','ZGSTALE','ZGOLD','ZGRUN']) s;

-- base close, base volume, D close, D volume
CREATE TEMP TABLE zg (symbol text, base float8, vol float8, d_close float8, d_vol float8);
INSERT INTO zg VALUES
  ('ZGMKT',   20.0, 1e6, 23.0, 5e6),
  ('ZGPNY',    1.0, 5e6,  1.2, 3e7),
  ('ZGFLAT',  15.0, 1e6, 15.0, 1e6),
  ('ZGSTALE', 30.0, 1e6, 34.5, 5e6),
  ('ZGOLD',   10.0, 2e6,  8.0, 2e6),
  ('ZGRUN',   12.0, 2e6, 12.6, 3e6);

INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source, raw_close, split_factor, div_cash)
SELECT (:'D'::date - n)::timestamptz, z.symbol, '1Day',
       c * 0.995, c * 1.01, c * 0.99, c, v, 'tiingo', c, 1, 0
FROM zg z
CROSS JOIN generate_series(0, 299) n
CROSS JOIN LATERAL (SELECT
    CASE WHEN n = 0 THEN z.d_close ELSE round((z.base * (1 + 0.02 * sin(n)))::numeric, 4)::float8 END AS c,
    CASE WHEN n = 0 THEN z.d_vol ELSE z.vol * (1 + 0.1 * cos(n)) END AS v) x;

INSERT INTO shares_outstanding_pit (symbol, filed_date, shares, cik, form) VALUES
  ('ZGMKT',   :'D'::date - 100, 5e7,   '0000000001', '10-Q'),
  ('ZGPNY',   :'D'::date - 50,  2e7,   '0000000002', '10-Q'),
  ('ZGFLAT',  :'D'::date - 60,  1e8,   '0000000003', '10-Q'),
  ('ZGSTALE', :'D'::date - 600, 5e7,   '0000000004', '10-K'),
  ('ZGOLD',   :'D'::date - 80,  4e7,   '0000000005', '10-Q'),
  ('ZGRUN',   :'D'::date - 80,  4e7,   '0000000006', '10-Q');

INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source)
SELECT (:'D'::date - 1)::timestamptz + interval '12 hours', symbol, 'ttm', metric, value, 'finnhub_metric'
FROM (VALUES
  ('ZGMKT', 'market_cap', 1.0e9), ('ZGMKT', 'shares_outstanding', 5e7),
  ('ZGPNY', 'market_cap', 2.5e7), ('ZGPNY', 'shares_outstanding', 2e7),
  ('ZGFLAT', 'market_cap', 1.5e9), ('ZGSTALE', 'market_cap', 1.7e9),
  ('ZGOLD', 'market_cap', 4e8), ('ZGRUN', 'market_cap', 5e8)) m(symbol, metric, value);

INSERT INTO catalyst_events (ts, symbol, source, headline_hash, matched_keyword, tier, headline)
VALUES ((:'D'::date)::timestamptz + interval '14 hours', 'ZGPNY', 'golden', 'h1', 'acquisition', 'A', 'ZGPNY to be acquired');

INSERT INTO momentum_tracked (symbol, alerted_ts, bucket, reference_price, score_at_alert,
                              resistance_20_at_alert, atr_14_at_alert, last_evaluated_ts,
                              highest_close_since, max_gain_pct, sessions_elapsed)
VALUES
  ('ZGOLD', (:'D'::date - 10)::timestamptz, 'market', 10.0, 40, 10.2, 0.3,
   (:'D'::date - 1)::timestamptz, 10.2, 2.0, 9),
  ('ZGRUN', (:'D'::date - 3)::timestamptz, 'market', 12.0, 45, 12.2, 0.35,
   (:'D'::date - 1)::timestamptz, 12.2, 1.6, 2);

\if :later
INSERT INTO equity_ohlcv (ts, symbol, interval, open, high, low, close, volume, source, raw_close, split_factor, div_cash)
SELECT (:'D'::date + 1)::timestamptz, symbol, '1Day', d_close, d_close * 1.5, d_close * 0.5, d_close * 1.3,
       d_vol * 3, 'tiingo', d_close * 1.3, 1, 0
FROM zg;
INSERT INTO shares_outstanding_pit (symbol, filed_date, shares, cik, form)
VALUES ('ZGMKT', :'D'::date + 1, 5e9, '0000000001', '8-K');
INSERT INTO equity_fundamentals (ts, symbol, period, metric, value, source) VALUES
  ((:'D'::date + 1)::timestamptz + interval '1 hour', 'ZGMKT', 'ttm', 'market_cap', 5e10, 'finnhub_metric'),
  ((:'D'::date + 1)::timestamptz + interval '1 hour', 'ZGMKT', 'ttm', 'shares_outstanding', 9e9, 'finnhub_metric');
INSERT INTO catalyst_events (ts, symbol, source, headline_hash, matched_keyword, tier, headline)
VALUES ((:'D'::date + 1)::timestamptz + interval '2 hours', 'ZGMKT', 'golden', 'h2', 'fda approval', 'A', 'ZGMKT wins approval');
\endif
