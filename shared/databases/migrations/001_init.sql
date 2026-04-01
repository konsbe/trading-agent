-- TimescaleDB: run after CREATE EXTENSION (see docker-compose health / manual)
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS crypto_ohlcv (
    ts TIMESTAMPTZ NOT NULL,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    interval TEXT NOT NULL,
    open DOUBLE PRECISION NOT NULL,
    high DOUBLE PRECISION NOT NULL,
    low DOUBLE PRECISION NOT NULL,
    close DOUBLE PRECISION NOT NULL,
    volume DOUBLE PRECISION NOT NULL,
    source TEXT NOT NULL,
    PRIMARY KEY (exchange, symbol, interval, ts, source)
);
SELECT public.create_hypertable('crypto_ohlcv', 'ts', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS crypto_global_metrics (
    ts TIMESTAMPTZ NOT NULL,
    provider TEXT NOT NULL DEFAULT 'coingecko',
    payload JSONB NOT NULL,
    PRIMARY KEY (provider, ts)
);
SELECT public.create_hypertable('crypto_global_metrics', 'ts', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS equity_ohlcv (
    ts TIMESTAMPTZ NOT NULL,
    symbol TEXT NOT NULL,
    interval TEXT NOT NULL,
    open DOUBLE PRECISION NOT NULL,
    high DOUBLE PRECISION NOT NULL,
    low DOUBLE PRECISION NOT NULL,
    close DOUBLE PRECISION NOT NULL,
    volume DOUBLE PRECISION NOT NULL,
    source TEXT NOT NULL,
    PRIMARY KEY (symbol, interval, ts, source)
);
SELECT public.create_hypertable('equity_ohlcv', 'ts', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS macro_fred (
    ts TIMESTAMPTZ NOT NULL,
    series_id TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (series_id, ts)
);
SELECT public.create_hypertable('macro_fred', 'ts', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS onchain_metrics (
    ts TIMESTAMPTZ NOT NULL,
    asset TEXT NOT NULL,
    metric TEXT NOT NULL,
    value DOUBLE PRECISION,
    payload JSONB,
    source TEXT NOT NULL,
    PRIMARY KEY (asset, metric, ts, source)
);
SELECT public.create_hypertable('onchain_metrics', 'ts', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS sentiment_snapshots (
    ts TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    symbol TEXT NOT NULL,
    score DOUBLE PRECISION,
    payload JSONB NOT NULL,
    PRIMARY KEY (source, symbol, ts)
);
SELECT public.create_hypertable('sentiment_snapshots', 'ts', if_not_exists => TRUE);

CREATE TABLE IF NOT EXISTS news_headlines (
    ts TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    symbol TEXT,
    headline TEXT NOT NULL,
    url TEXT,
    sentiment NUMERIC,
    payload JSONB,
    PRIMARY KEY (ts, source, headline)
);
SELECT public.create_hypertable('news_headlines', 'ts', if_not_exists => TRUE);
