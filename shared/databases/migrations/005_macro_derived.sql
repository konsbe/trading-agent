-- macro_derived stores computed signals produced by the macro-analysis worker.
-- Each metric (mp_rate, mp_yield_curve, mp_stance, …) has one row per computation
-- run, keyed by (metric, source, ts). The analyst-bot always reads the latest row
-- per metric (ORDER BY ts DESC LIMIT 1).
--
-- Design mirrors equity_fundamentals (period = "derived") but without symbol/period
-- columns because macro signals are market-wide, not per-company.
--
-- source is always 'macro_analysis' — left as a column for forward-compat with
-- future alternative analysis pipelines (e.g. a Python-rewrite of the same worker).

CREATE TABLE IF NOT EXISTS macro_derived (
    ts      TIMESTAMPTZ       NOT NULL,
    metric  TEXT              NOT NULL,
    value   DOUBLE PRECISION,
    payload JSONB,
    source  TEXT              NOT NULL DEFAULT 'macro_analysis',
    UNIQUE (metric, source, ts)
);

SELECT create_hypertable('macro_derived', 'ts', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS macro_derived_metric_ts
    ON macro_derived (metric, ts DESC);
