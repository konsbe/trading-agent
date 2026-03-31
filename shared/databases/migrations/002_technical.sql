CREATE TABLE IF NOT EXISTS technical_indicators (
    ts          TIMESTAMPTZ      NOT NULL,
    symbol      TEXT             NOT NULL,
    exchange    TEXT             NOT NULL DEFAULT 'equity',
    interval    TEXT             NOT NULL,
    indicator   TEXT             NOT NULL,
    value       DOUBLE PRECISION,
    payload     JSONB,
    PRIMARY KEY (symbol, exchange, interval, indicator, ts)
);
SELECT create_hypertable('technical_indicators', 'ts', if_not_exists => TRUE);

-- Fast lookups: latest value for a symbol/interval/indicator combination.
CREATE INDEX IF NOT EXISTS ti_lookup
    ON technical_indicators (symbol, exchange, interval, indicator, ts DESC);
