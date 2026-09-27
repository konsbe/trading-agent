-- 030: followed symbols, the symbol directory, and computation tracking.
--
-- FOLLOWED SYMBOLS
--
-- followed_symbols replaces the .env symbol lists (ALPACA_DATA_SYMBOLS,
-- TECHNICAL_*_SYMBOLS, FUNDAMENTAL_SYMBOLS, FINNHUB_*_NEWS_SYMBOLS,
-- BOT_*_SYMBOLS, EQUITY_SYMBOLS_*) as the source of "symbols the user wants in
-- the full pipeline even when they are not candidates or watchlisted" —
-- including ETFs, crypto and foreign listings the scanner excludes. The .env
-- lists stay only as a fallback when this table is empty or unreachable, and
-- every fallback use is logged loudly by the service that falls back.
-- Report-layout lists (BOT_REPORT_*, MARKET_CYCLE_*, the market report's fixed
-- instruments) are not followed symbols and stay in .env / code.
--
--   asset_type  which pipelines run: equity (bars, technicals, Finnhub
--               fundamentals, statements when filed), etf (bars, technicals,
--               Finnhub metrics; no statements), crypto (Binance bars,
--               technicals; no fundamentals).
--   listing     us | foreign | crypto. Foreign listings (2222.SR, OXIG.L) get
--               bars from Yahoo and nothing from Finnhub's US endpoints.
--   news_alias  crypto only: the coin symbol news/sentiment feeds use
--               (RENDERUSDT → RNDR), since it is not derivable from the pair.
--
-- Whether a company has financial statements is NOT decided here: Finnhub
-- returns no filings for some US-listed common stocks too (TTE, GFS, TSM,
-- SHEL file 20-F). It is recorded per symbol from the fetch result in
-- symbol_data_status.

CREATE TABLE IF NOT EXISTS followed_symbols (
    symbol      TEXT         PRIMARY KEY,
    asset_type  TEXT         NOT NULL,
    listing     TEXT         NOT NULL,
    news_alias  TEXT         NULL,
    source      TEXT         NOT NULL DEFAULT 'user',
    added_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT followed_symbols_symbol_upper CHECK (symbol = upper(symbol) AND symbol <> ''),
    CONSTRAINT followed_symbols_asset_type CHECK (asset_type IN ('equity', 'etf', 'crypto')),
    CONSTRAINT followed_symbols_listing CHECK (listing IN ('us', 'foreign', 'crypto')),
    CONSTRAINT followed_symbols_crypto_listing CHECK ((asset_type = 'crypto') = (listing = 'crypto')),
    CONSTRAINT followed_symbols_source CHECK (source IN ('env_seed', 'user'))
);

-- Seed: the .env values on 2026-09-27 (every pipeline list held the same 26
-- equities and 8 crypto pairs). news_alias from FINNHUB_SYMBOLS_FOR_NEWS.
INSERT INTO followed_symbols (symbol, asset_type, listing, news_alias, source) VALUES
    ('XOM', 'equity', 'us', NULL, 'env_seed'),
    ('CVX', 'equity', 'us', NULL, 'env_seed'),
    ('SHEL', 'equity', 'us', NULL, 'env_seed'),
    ('BB', 'equity', 'us', NULL, 'env_seed'),
    ('TTE', 'equity', 'us', NULL, 'env_seed'),
    ('2222.SR', 'equity', 'foreign', NULL, 'env_seed'),
    ('TSM', 'equity', 'us', NULL, 'env_seed'),
    ('INTC', 'equity', 'us', NULL, 'env_seed'),
    ('GFS', 'equity', 'us', NULL, 'env_seed'),
    ('OXIG.L', 'equity', 'foreign', NULL, 'env_seed'),
    ('KEYS', 'equity', 'us', NULL, 'env_seed'),
    ('COHR', 'equity', 'us', NULL, 'env_seed'),
    ('IPGP', 'equity', 'us', NULL, 'env_seed'),
    ('AMZN', 'equity', 'us', NULL, 'env_seed'),
    ('MSFT', 'equity', 'us', NULL, 'env_seed'),
    ('NVDA', 'equity', 'us', NULL, 'env_seed'),
    ('GOOGL', 'equity', 'us', NULL, 'env_seed'),
    ('SPY', 'etf', 'us', NULL, 'env_seed'),
    ('QQQ', 'etf', 'us', NULL, 'env_seed'),
    ('IWM', 'etf', 'us', NULL, 'env_seed'),
    ('XLF', 'etf', 'us', NULL, 'env_seed'),
    ('EEM', 'etf', 'us', NULL, 'env_seed'),
    ('MCHI', 'etf', 'us', NULL, 'env_seed'),
    ('GLD', 'etf', 'us', NULL, 'env_seed'),
    ('COPX', 'etf', 'us', NULL, 'env_seed'),
    ('USO', 'etf', 'us', NULL, 'env_seed'),
    ('BTCUSDT', 'crypto', 'crypto', 'BTC', 'env_seed'),
    ('ETHUSDT', 'crypto', 'crypto', 'ETH', 'env_seed'),
    ('LINKUSDT', 'crypto', 'crypto', 'LINK', 'env_seed'),
    ('KSMUSDT', 'crypto', 'crypto', 'KSM', 'env_seed'),
    ('TAOUSDT', 'crypto', 'crypto', 'TAO', 'env_seed'),
    ('RENDERUSDT', 'crypto', 'crypto', 'RNDR', 'env_seed'),
    ('SOLUSDT', 'crypto', 'crypto', 'SOL', 'env_seed'),
    ('TIAUSDT', 'crypto', 'crypto', 'TIA', 'env_seed')
ON CONFLICT (symbol) DO NOTHING;

-- SYMBOL DIRECTORY
--
-- The full provider directories, for adding symbols outside the scanner's
-- universe (the "all symbols" search). data-universe already fetches
-- Finnhub's whole US list (~30k rows) weekly but stored only the 9k rows on
-- NASDAQ / NYSE / NYSE American in universe_symbols; NYSE Arca ETFs (SPY,
-- GLD…), OTC and foreign MICs were discarded. It now also writes every row
-- here, plus Binance's spot pairs. universe_symbols is unchanged.
--   source      finnhub_us | binance_spot
--   asset_type  equity | etf | crypto | other, derived from the provider type
--   type        the provider's own type (Common Stock, ETP, ADR, …)

CREATE TABLE IF NOT EXISTS symbol_directory (
    source          TEXT         NOT NULL,
    symbol          TEXT         NOT NULL,
    display_symbol  TEXT         NULL,
    name            TEXT         NULL,
    type            TEXT         NULL,
    mic             TEXT         NULL,
    currency        TEXT         NULL,
    asset_type      TEXT         NOT NULL,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (source, symbol),
    CONSTRAINT symbol_directory_source CHECK (source IN ('finnhub_us', 'binance_spot')),
    CONSTRAINT symbol_directory_asset_type CHECK (asset_type IN ('equity', 'etf', 'crypto', 'other'))
);

CREATE INDEX IF NOT EXISTS symbol_directory_symbol_prefix ON symbol_directory (symbol text_pattern_ops);
CREATE INDEX IF NOT EXISTS symbol_directory_name_lower ON symbol_directory (lower(name) text_pattern_ops);

-- COMPUTATION INTEREST
--
-- Why a symbol is refreshed. One row per period a reason held; a reason is
-- closed (active_until set), never deleted. The refresh pass covers symbols
-- with at least one open reason. When every reason closes, refreshing stops
-- but the symbol's computed rows stay: candidate history is needed for later
-- evaluation (survivorship bias) and Tracked Positions follows symbols after
-- they stop being candidates.
--   reason  watchlist | candidate | followed — reconciled from their sources
--           each session; manual — opened by "Compute", closed by "Stop
--           computing".

CREATE TABLE IF NOT EXISTS computation_interest (
    id            BIGSERIAL    PRIMARY KEY,
    symbol        TEXT         NOT NULL,
    asset_type    TEXT         NOT NULL,
    reason        TEXT         NOT NULL,
    active_from   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    active_until  TIMESTAMPTZ  NULL,
    CONSTRAINT computation_interest_symbol_upper CHECK (symbol = upper(symbol) AND symbol <> ''),
    CONSTRAINT computation_interest_asset_type CHECK (asset_type IN ('equity', 'etf', 'crypto')),
    CONSTRAINT computation_interest_reason CHECK (reason IN ('watchlist', 'candidate', 'followed', 'manual')),
    CONSTRAINT computation_interest_period CHECK (active_until IS NULL OR active_until >= active_from)
);

-- At most one open row per (symbol, reason).
CREATE UNIQUE INDEX IF NOT EXISTS computation_interest_open
    ON computation_interest (symbol, reason) WHERE active_until IS NULL;
CREATE INDEX IF NOT EXISTS computation_interest_symbol ON computation_interest (symbol, active_from DESC);

-- SYMBOL DATA STATUS
--
-- Per-symbol fetch / compute bookkeeping, written by the workers:
--   bars_fetched_at / fundamentals_fetched_at   last successful fetch
--   statements_status  available | none_returned | not_applicable (with
--                      statements_reason, e.g. "Finnhub returned no 10-K/10-Q
--                      filings", "ETF", "crypto", "foreign listing")
--   computed_at        last technical + fundamental computation
--   last_error         the last fetch/compute error, for the computed-symbols view

CREATE TABLE IF NOT EXISTS symbol_data_status (
    symbol                   TEXT         PRIMARY KEY,
    bars_fetched_at          TIMESTAMPTZ  NULL,
    fundamentals_fetched_at  TIMESTAMPTZ  NULL,
    statements_status        TEXT         NULL,
    statements_reason        TEXT         NULL,
    statements_checked_at    TIMESTAMPTZ  NULL,
    computed_at              TIMESTAMPTZ  NULL,
    last_error               TEXT         NULL,
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT symbol_data_status_symbol_upper CHECK (symbol = upper(symbol) AND symbol <> ''),
    CONSTRAINT symbol_data_status_statements CHECK (
        statements_status IS NULL OR statements_status IN ('available', 'none_returned', 'not_applicable'))
);

-- The daily computation pass (reconcile the reasons above, then compute every
-- symbol with an open reason) runs once per session after the chain
-- completes. Its marker is its own column: a failing pass is retried and never
-- marks the session's chain unclean.
ALTER TABLE momentum_chain_runs ADD COLUMN IF NOT EXISTS computation_pass_at TIMESTAMPTZ NULL;
