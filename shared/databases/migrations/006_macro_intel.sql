-- Macro intelligence: calendars, geopolitical quant series, RSS macro news, LLM narrative scores.
-- Ingested by data-macro-intel (Go); narrative_scores optionally filled by analyst-bot (Python).

CREATE TABLE IF NOT EXISTS economic_calendar_events (
    event_ts TIMESTAMPTZ NOT NULL,
    country TEXT NOT NULL DEFAULT '',
    event_name TEXT NOT NULL,
    impact TEXT,
    actual DOUBLE PRECISION,
    estimate DOUBLE PRECISION,
    previous DOUBLE PRECISION,
    unit TEXT,
    source TEXT NOT NULL,
    external_id TEXT NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload JSONB,
    PRIMARY KEY (source, external_id)
);
CREATE INDEX IF NOT EXISTS idx_economic_calendar_event_ts ON economic_calendar_events (event_ts DESC);

CREATE TABLE IF NOT EXISTS earnings_calendar_events (
    earnings_date DATE NOT NULL,
    symbol TEXT NOT NULL,
    hour TEXT,
    year INTEGER,
    quarter TEXT,
    eps_estimate DOUBLE PRECISION,
    eps_actual DOUBLE PRECISION,
    revenue_estimate DOUBLE PRECISION,
    revenue_actual DOUBLE PRECISION,
    source TEXT NOT NULL,
    external_id TEXT NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload JSONB,
    PRIMARY KEY (source, external_id)
);
CREATE INDEX IF NOT EXISTS idx_earnings_calendar_date ON earnings_calendar_events (earnings_date DESC);
CREATE INDEX IF NOT EXISTS idx_earnings_calendar_symbol ON earnings_calendar_events (symbol);

CREATE TABLE IF NOT EXISTS geopolitical_risk_monthly (
    month_ts DATE NOT NULL,
    gpr_total DOUBLE PRECISION,
    gpr_act DOUBLE PRECISION,
    gpr_threat DOUBLE PRECISION,
    source TEXT NOT NULL DEFAULT 'gpr_csv',
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload JSONB,
    PRIMARY KEY (month_ts, source)
);

CREATE TABLE IF NOT EXISTS gdelt_macro_daily (
    day_ts DATE NOT NULL,
    query_label TEXT NOT NULL,
    article_count INTEGER,
    avg_tone DOUBLE PRECISION,
    avg_goldstein DOUBLE PRECISION,
    source TEXT NOT NULL DEFAULT 'gdelt',
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload JSONB,
    PRIMARY KEY (day_ts, query_label)
);

CREATE TABLE IF NOT EXISTS narrative_scores (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    doc_kind TEXT NOT NULL,
    source_url TEXT,
    title TEXT,
    llm_score DOUBLE PRECISION,
    llm_summary TEXT,
    model TEXT,
    payload JSONB
);
CREATE INDEX IF NOT EXISTS idx_narrative_scores_created ON narrative_scores (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_narrative_scores_kind ON narrative_scores (doc_kind);
