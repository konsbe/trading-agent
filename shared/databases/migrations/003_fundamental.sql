-- Fundamental data: financial statement metrics, key ratios, and earnings history.
-- Tall/narrow layout identical to technical_indicators: one row per (symbol, period, metric).
-- period examples: 'ttm', 'annual_2024', 'q_2024Q3', 'point_in_time'
-- source examples: 'finnhub_metric', 'finnhub_financials_reported', 'finnhub_earnings'
CREATE TABLE IF NOT EXISTS equity_fundamentals (
    ts          TIMESTAMPTZ      NOT NULL,
    symbol      TEXT             NOT NULL,
    period      TEXT             NOT NULL,
    metric      TEXT             NOT NULL,
    value       DOUBLE PRECISION,
    payload     JSONB,
    source      TEXT             NOT NULL,
    PRIMARY KEY (symbol, period, metric, source, ts)
);
SELECT create_hypertable('equity_fundamentals', 'ts', if_not_exists => TRUE);

-- Fast lookup: latest value for a symbol/period/metric combination.
CREATE INDEX IF NOT EXISTS ef_lookup
    ON equity_fundamentals (symbol, period, metric, ts DESC);
