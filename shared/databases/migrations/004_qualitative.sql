-- Qualitative analysis supporting tables.
--
-- insider_transactions: stores SEC Form 4 insider buy/sell data from Finnhub.
--   transaction_code: P = open-market purchase, S = sale, A = award/grant, F = tax withholding, M = exercise
--   Signal use: cluster_buy (3+ distinct insiders buying in 90 days) is a high-conviction bullish signal.
--
-- Note: news_headlines.sentiment already exists from migration 001_init.sql.
--   Alpha Vantage NEWS_SENTIMENT populates that column with numeric scores (-1 to +1).

CREATE TABLE IF NOT EXISTS insider_transactions (
    ts               TIMESTAMPTZ NOT NULL,
    symbol           TEXT NOT NULL,
    insider_name     TEXT,
    transaction_code TEXT,          -- 'P' purchase, 'S' sale, 'A' award, 'F' tax, 'M' exercise
    shares           FLOAT,         -- number of shares transacted (positive)
    price_per_share  FLOAT,         -- transaction price per share
    filing_date      DATE,
    UNIQUE (symbol, ts, insider_name, transaction_code, shares)
);
SELECT create_hypertable('insider_transactions', 'ts', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS it_symbol_ts
    ON insider_transactions (symbol, ts DESC);
