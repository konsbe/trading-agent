-- Momentum scanner (Phase 1) — universe, features, scores, forward labels,
-- catalyst keyword matches, and sell-side exit tracking.
--
-- Spec: docs/MOMENTUM_SCANNER_PHASE1.md (§3 features, §4 scoring, §5 exits,
-- §6 labels, §7 schema). Written by data-universe (Go) and momentum-scanner
-- (Go); read by analyst-bot (Python).
--
-- Daily bars are NOT duplicated here — momentum features are computed from
-- equity_ohlcv rows with interval = '1Day' and source = 'yahoo' (§7).
--
-- Applies to existing databases as well as fresh initdb: every statement is
-- idempotent (migrations are only auto-run by Postgres on first init, so this
-- file must be applied manually to an already-initialised volume).
--
-- UNIT CONVENTIONS (deviating from these is a silent, invisible bug):
--   *_pct            percent points, e.g. 11.8 means +11.8%   (§3.3 multiplies by 100)
--   pct_of_52w_high  RATIO, not percent: 1.0 = at the 52-week high (§3.7)
--   rvol_20          ratio, volume[t] / mean(volume[t-20..t-1]) (§3.4)
--   vol_accel        ratio, mean(vol[t-2..t]) / mean(vol[t-7..t-3]) (§3.5)
--   market_cap       USD ABSOLUTE. Finnhub /stock/metric serves $ millions —
--                    the writer multiplies by 1e6 before storing (§3.2 gates
--                    are expressed in dollars: $300M–$10B).
--   shares_outstanding / float_shares_est
--                    SHARE COUNT ABSOLUTE. Finnhub serves millions — the writer
--                    multiplies by 1e6 (§3.9 "values are in millions — normalize";
--                    §4.2 float bands are 20M/50M/100M/300M shares).
--   dollar_volume    USD absolute, close * volume (§3.3)


-- ── universe_symbols ────────────────────────────────────────────────────────
-- Eligible US common-stock universe (§3.1). Regular table, refreshed weekly by
-- data-universe. Ineligible symbols are RETAINED with is_eligible = false and a
-- populated excluded_reason so the exclusion rules stay auditable (§10 step 2).
--
-- The backfill_* columns are the per-symbol checkpoint for the resumable 3-year
-- bar backfill (§8.1.3) — they are job state, not a §3 feature.
--   backfill_status: 'pending' | 'in_progress' | 'done' | 'failed'
CREATE TABLE IF NOT EXISTS universe_symbols (
    symbol                TEXT             NOT NULL,
    exchange              TEXT             NOT NULL,
    mic                   TEXT,                        -- Finnhub market identifier code; the reliable NASDAQ/NYSE/NYSE-American discriminator
    name                  TEXT,
    type                  TEXT,                        -- raw instrument type from the symbol list, pre-filter
    sector                TEXT,                        -- from equity_fundamentals (Finnhub/Alpha Vantage), weekly refresh
    industry              TEXT,
    shares_outstanding    DOUBLE PRECISION,            -- absolute share count (see UNIT CONVENTIONS)
    market_cap            DOUBLE PRECISION,            -- USD absolute (see UNIT CONVENTIONS)
    fundamentals_ts       TIMESTAMPTZ,                 -- when sector/shares/cap were last refreshed
    bar_count             INTEGER,                     -- daily bars present in equity_ohlcv; §3.1 requires >= 250
    first_bar_ts          TIMESTAMPTZ,
    last_bar_ts           TIMESTAMPTZ,
    is_eligible           BOOLEAN          NOT NULL DEFAULT false,
    excluded_reason       TEXT,                        -- null iff is_eligible; e.g. 'type_not_common_stock', 'exchange_not_allowed', 'ticker_suffix_excluded', 'insufficient_history'
    backfill_status       TEXT             NOT NULL DEFAULT 'pending',
    backfill_cursor_ts    TIMESTAMPTZ,                 -- oldest bar fetched so far; resume point
    backfill_attempts     INTEGER          NOT NULL DEFAULT 0,
    backfill_last_error   TEXT,
    backfill_completed_at TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, exchange)
);

-- The scanner iterates the eligible set every day; the backfill job claims work
-- by status. Both are partial/leading-column indexes on that access pattern.
CREATE INDEX IF NOT EXISTS us_eligible
    ON universe_symbols (is_eligible, symbol);
CREATE INDEX IF NOT EXISTS us_backfill_status
    ON universe_symbols (backfill_status, symbol)
    WHERE is_eligible;
CREATE INDEX IF NOT EXISTS us_sector
    ON universe_symbols (sector)
    WHERE is_eligible;


-- ── momentum_features ───────────────────────────────────────────────────────
-- One row per symbol per completed trading day (§3). Hypertable on ts.
--
-- Written in both live mode (today only) and backfill mode (full history), so
-- every column must be computable from bars up to and including ts — NO
-- LOOKAHEAD (§6). Forward-looking quantities belong in momentum_labels.
--
-- close is denormalised onto this row deliberately: §4 penalties, §5 exits and
-- §6 labels all reference close[t], and a self-contained feature row keeps the
-- scorer and label jobs from re-joining equity_ohlcv. It is a copy of the bar,
-- not a parallel bar table (§7).
CREATE TABLE IF NOT EXISTS momentum_features (
    ts                    TIMESTAMPTZ      NOT NULL,   -- the bar date t (bar-open ts, matching equity_ohlcv)
    symbol                TEXT             NOT NULL,

    -- §3.3 price and change
    close                 DOUBLE PRECISION,
    volume                DOUBLE PRECISION,
    prior_close           DOUBLE PRECISION,
    change_pct            DOUBLE PRECISION,
    gap_pct               DOUBLE PRECISION,
    dollar_volume         DOUBLE PRECISION,
    atr_14                DOUBLE PRECISION,
    atr_pct               DOUBLE PRECISION,

    -- §3.4 relative volume. rvol_20 is NULL when avg_vol_20 is 0 or fewer than
    -- 20 prior bars exist — never 0 and never 1 (§3.4, §12).
    avg_vol_20            DOUBLE PRECISION,
    rvol_20               DOUBLE PRECISION,

    -- §3.5 volume acceleration
    vol_accel             DOUBLE PRECISION,

    -- §3.6 breakout geometry. breakout_state ∈
    -- ('breakout_from_consolidation','breakout','approaching','none');
    -- judged on close, never on an intrabar high.
    resistance_20         DOUBLE PRECISION,
    range_20              DOUBLE PRECISION,
    was_consolidating     BOOLEAN,
    breakout_state        TEXT,

    -- §3.7 52-week high proximity. The window EXCLUDES today, matching the
    -- same convention as avg_vol_20 and resistance_20, so pct_of_52w_high can
    -- exceed 1.0 and > 1.0 *is* a new 52-week high. There is deliberately no
    -- separate new_52w_high boolean — it would be redundant with this ratio.
    high_52w              DOUBLE PRECISION,
    pct_of_52w_high       DOUBLE PRECISION,

    -- §3.8 rolling 20-day VWAP (Phase 1 stand-in for intraday session VWAP;
    -- field names are stable so the Phase 3 swap is one place).
    vwap_20               DOUBLE PRECISION,
    above_vwap            BOOLEAN,
    vwap_dist_pct         DOUBLE PRECISION,

    -- §3.9 float. float_shares_est is shares outstanding used as an
    -- APPROXIMATION of public float; float_is_proxy is always true in Phase 1
    -- and exists so Phase 2 can discount the feature. Label it "Float (est)".
    float_shares_est      DOUBLE PRECISION,
    float_is_proxy        BOOLEAN          NOT NULL DEFAULT true,

    -- §3.10 RSI — penalty input only, never a positive score
    rsi_14                DOUBLE PRECISION,

    -- §3.11 catalyst. Populated only for gated candidates (news is fetched
    -- after gating), so NULL here means "not looked up", which is distinct from
    -- the 'none' tier meaning "looked up, no company news in window".
    catalyst_tier         TEXT,                        -- 'A' | 'B' | 'none' | NULL
    catalyst_headline     TEXT,                        -- headline backing the winning tier, for the embed
    catalyst_checked_at   TIMESTAMPTZ,

    -- §3.12 recorded but NOT scored in Phase 1
    sector_strength_pct   DOUBLE PRECISION,
    change_pct_5d         DOUBLE PRECISION,            -- input to sector_strength_pct; see SCHEMAS.md note

    -- §3.2 / §3.9 market cap. market_cap is Finnhub's value; market_cap_est is
    -- the documented fallback shares_outstanding * close[t], used for the gate
    -- ONLY when market_cap is null (Finnhub's micro-cap coverage is poor and a
    -- hard null would silently empty the penny bucket). market_cap_is_proxy
    -- mirrors float_is_proxy. Not a silent default: explicit formula, persisted
    -- flag, and 'market_cap_null' stays in gate_failures even when the estimate
    -- lets the symbol through, so the proxy's contribution stays measurable.
    market_cap            DOUBLE PRECISION,
    market_cap_est        DOUBLE PRECISION,
    market_cap_is_proxy   BOOLEAN          NOT NULL DEFAULT false,

    -- §3.2 gate outcome. gates_passed = candidate; a gate failure means
    -- EXCLUDED, not low-scored. gate_failures is populated even when passing
    -- (empty array) so "evaluated" is distinguishable from "not evaluated".
    bucket                TEXT,                        -- 'market' | 'penny' | NULL when price is outside both bands
    gates_passed          BOOLEAN          NOT NULL DEFAULT false,
    gate_failures         TEXT[]           NOT NULL DEFAULT '{}',

    -- §3.13 / §3.9 — deliberately NULL in Phase 1, present so the features can
    -- be added later without a migration. No free source exists for these.
    short_interest_pct    DOUBLE PRECISION,
    days_to_cover         DOUBLE PRECISION,
    is_halted             BOOLEAN,
    premarket_change_pct  DOUBLE PRECISION,

    computed_at           TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, ts)
);
SELECT create_hypertable('momentum_features', 'ts', if_not_exists => TRUE);

-- Daily scan: "today's candidates, best first" and per-symbol history for /score.
CREATE INDEX IF NOT EXISTS mf_candidates
    ON momentum_features (ts DESC, bucket)
    WHERE gates_passed;
CREATE INDEX IF NOT EXISTS mf_symbol_ts
    ON momentum_features (symbol, ts DESC);
-- §3.12 sector median is computed per (ts, sector) via a join on universe_symbols.
CREATE INDEX IF NOT EXISTS mf_ts_change5d
    ON momentum_features (ts DESC)
    WHERE change_pct_5d IS NOT NULL;


-- ── momentum_scores ────────────────────────────────────────────────────────
-- §4. One row per scored symbol per day. Only gated candidates are scored.
--
-- Every sub-score is its own column and the pre-penalty subtotal is stored, so
-- /score can reconstruct the total exactly and Phase 2 can use the components
-- independently (§4.4). A score with no visible breakdown is undebuggable.
--
-- Sub-score maxima (§4.1, total 100): accel 25, rvol 20, breakout 20,
-- catalyst 15, float 10, vwap 5, 52w 5.
CREATE TABLE IF NOT EXISTS momentum_scores (
    ts                        TIMESTAMPTZ      NOT NULL,
    symbol                    TEXT             NOT NULL,
    bucket                    TEXT             NOT NULL,   -- 'market' | 'penny'

    momentum_score_100        INTEGER          NOT NULL,   -- final, clamped to [0,100]

    score_vol_accel           DOUBLE PRECISION,            -- 0–25
    score_rvol                DOUBLE PRECISION,            -- 0–20
    score_breakout            DOUBLE PRECISION,            -- 0–20
    score_catalyst            DOUBLE PRECISION,            -- 0–15
    score_float               DOUBLE PRECISION,            -- 0–10
    score_vwap                DOUBLE PRECISION,            -- 0–5
    score_52w                 DOUBLE PRECISION,            -- 0–5

    subtotal_before_penalties DOUBLE PRECISION,            -- sum of the seven sub-scores
    penalty_total             DOUBLE PRECISION,            -- negative or zero
    -- §4.3, e.g. {"rsi_exhausted": -5, "already_extended": -10, "volume_decaying": -5}
    -- Only applied penalties appear as keys.
    penalties                 JSONB            NOT NULL DEFAULT '{}'::jsonb,

    -- Which §4.2 inputs were NULL and therefore scored 0 for their component.
    -- A null input scores zero — it is never imputed (§4.2, §12).
    null_inputs               TEXT[]           NOT NULL DEFAULT '{}',

    scored_at                 TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, ts)
);
SELECT create_hypertable('momentum_scores', 'ts', if_not_exists => TRUE);

-- /scanner: top N by score for a given day and bucket.
CREATE INDEX IF NOT EXISTS ms_rank
    ON momentum_scores (ts DESC, bucket, momentum_score_100 DESC);
CREATE INDEX IF NOT EXISTS ms_symbol_ts
    ON momentum_scores (symbol, ts DESC);
-- §6 decile analysis scans score x label across the whole backfill.
CREATE INDEX IF NOT EXISTS ms_score
    ON momentum_scores (momentum_score_100 DESC, ts);


-- ── momentum_labels ────────────────────────────────────────────────────────
-- §6. Forward outcomes for historical gated candidates. THE Phase 1 deliverable.
--
-- Features for day t use only bars <= t; labels use only bars > t. Both halves
-- live in separate tables to make that boundary structural rather than a
-- convention someone can accidentally break.
--
-- label_complete = false for rows inside the last H sessions (incomplete
-- window). Those rows MUST be excluded from any evaluation (§6).
--
-- hit_200/300/500/1000 are recorded for Phase 2 training targets only. Phase 1
-- never computes or displays a score for them (§1, §12).
CREATE TABLE IF NOT EXISTS momentum_labels (
    ts                    TIMESTAMPTZ      NOT NULL,   -- the setup date t
    symbol                TEXT             NOT NULL,

    horizon_days          INTEGER          NOT NULL,   -- H, trading days (default 120)
    bars_available        INTEGER          NOT NULL,   -- forward bars actually present; < H ⇒ label_complete = false
    label_complete        BOOLEAN          NOT NULL DEFAULT false,

    entry_close           DOUBLE PRECISION,            -- close[t], the denominator; copied for reproducibility
    fwd_max_close         DOUBLE PRECISION,            -- max(close[t+1 .. t+H])
    fwd_max_gain_pct      DOUBLE PRECISION,            -- (fwd_max_close / close[t] - 1) * 100
    days_to_peak          INTEGER,                     -- argmax offset, 1..H
    fwd_max_drawdown_pct  DOUBLE PRECISION,            -- max peak-to-trough decline within the window (negative)

    hit_100               BOOLEAN,                     -- fwd_max_gain_pct >= 100
    hit_200               BOOLEAN,
    hit_300               BOOLEAN,
    hit_500               BOOLEAN,
    hit_1000              BOOLEAN,

    labeled_at            TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, ts)
);
SELECT create_hypertable('momentum_labels', 'ts', if_not_exists => TRUE);

-- Base-rate and decile reporting joins scores to complete labels only.
CREATE INDEX IF NOT EXISTS ml_complete
    ON momentum_labels (ts DESC)
    WHERE label_complete;
CREATE INDEX IF NOT EXISTS ml_symbol_ts
    ON momentum_labels (symbol, ts DESC);


-- ── catalyst_events ────────────────────────────────────────────────────────
-- §3.11. EVERY keyword match is stored, not just the winning tier: Phase 2's
-- most valuable analysis is which specific keywords preceded real runners, and
-- that requires the raw matches retained.
--
-- headline_hash = sha256 hex of the normalised headline (lowercased, collapsed
-- whitespace, trimmed). It exists because headlines are too long for a
-- practical key and arrive duplicated across wires.
--
-- One headline matching several keywords yields several rows; they differ by
-- matched_keyword, which is therefore part of the key.
CREATE TABLE IF NOT EXISTS catalyst_events (
    ts                TIMESTAMPTZ      NOT NULL,   -- article publication time
    symbol            TEXT             NOT NULL,
    source            TEXT             NOT NULL,   -- e.g. 'finnhub_company_news'
    headline_hash     TEXT             NOT NULL,   -- sha256 hex of normalised headline
    matched_keyword   TEXT             NOT NULL,
    tier              TEXT             NOT NULL,   -- 'A' | 'B'
    headline          TEXT             NOT NULL,
    url               TEXT,
    scan_ts           TIMESTAMPTZ,                 -- the feature-row ts this match was fetched for
    payload           JSONB,                       -- raw news item
    ingested_at       TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, ts, source, headline_hash, matched_keyword)
);
SELECT create_hypertable('catalyst_events', 'ts', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS ce_symbol_ts
    ON catalyst_events (symbol, ts DESC);
-- "which keywords preceded runners" — the Phase 2 question.
CREATE INDEX IF NOT EXISTS ce_keyword
    ON catalyst_events (matched_keyword, tier, ts DESC);


-- ── momentum_tracked ───────────────────────────────────────────────────────
-- §5. Sell-side state. A sell signal is exit tracking on a prior buy alert, not
-- a short: it requires knowing what was previously alerted, so buy and sell are
-- not symmetric.
--
-- The *_at_alert columns freeze the levels the exit rules are measured against
-- (§5 references resistance_20_at_alert and atr_14_at_alert explicitly). They
-- are snapshots, not live values, and must not be refreshed.
--
-- status:      'active' | 'closed'
-- exit_reason: 'breakout_failed' | 'lost_vwap' | 'momentum_stalled' | 'stop_atr' | 'timeout'
--              Evaluated in that order; the FIRST match wins.
--
-- These exit rules are a starting default, not a validated strategy (§5).
CREATE TABLE IF NOT EXISTS momentum_tracked (
    symbol                  TEXT             NOT NULL,
    alerted_ts              TIMESTAMPTZ      NOT NULL,   -- the feature-row ts that triggered the buy alert
    bucket                  TEXT             NOT NULL,
    status                  TEXT             NOT NULL DEFAULT 'active',

    reference_price         DOUBLE PRECISION NOT NULL,   -- close[t] at alert
    score_at_alert          INTEGER,
    resistance_20_at_alert  DOUBLE PRECISION,            -- the level it broke out over
    atr_14_at_alert         DOUBLE PRECISION,

    -- running state, updated each daily evaluation while active
    last_evaluated_ts       TIMESTAMPTZ,
    highest_close_since     DOUBLE PRECISION,            -- for the §5 timeout rule's "no new high since alert"
    max_gain_pct            DOUBLE PRECISION,            -- realised max favourable move while active
    low_rvol_streak         INTEGER          NOT NULL DEFAULT 0,  -- consecutive sessions with rvol_20 < 1.5 (momentum_stalled needs 3)
    sessions_elapsed        INTEGER          NOT NULL DEFAULT 0,  -- trading sessions since alert (timeout at 20)

    -- populated on close
    exit_reason             TEXT,
    exit_ts                 TIMESTAMPTZ,
    exit_price              DOUBLE PRECISION,
    exit_pct                DOUBLE PRECISION,

    created_at              TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ      NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, alerted_ts)
);

-- The daily sell pass reads every active row.
CREATE INDEX IF NOT EXISTS mt_active
    ON momentum_tracked (status, symbol)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS mt_symbol
    ON momentum_tracked (symbol, alerted_ts DESC);
