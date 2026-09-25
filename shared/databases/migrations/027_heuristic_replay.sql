-- 027: heuristic-signals replay harness (docs/HEURISTIC_SIGNALS_PREREGISTRATION.md
-- §5 steps 3-4).
--
-- WRITER
--
--   services/data-analyzer/cmd/heuristics-replay  per symbol, in one
--   transaction: DELETE that symbol's rows from every table below, then COPY
--   the replayed rows. Re-running is idempotent; each row records the
--   harness_version that wrote it.
--
-- TABLES
--
--   heuristic_ep_<signal>      one table per signal type (§2: never pooled).
--                              Episode = first firing day for a symbol after
--                              >= 5 evaluated sessions without a firing of the
--                              same type. Identical column layout.
--   heuristic_comparison_days  deterministic sample of symbol-days (every 5th
--                              session per symbol) with a fired flag per
--                              signal type: the non-signal comparison group
--                              for every hypothesis is "rows where fired_X is
--                              false" in the same stratum. H5's baseline is
--                              fired_bb_squeeze = false.
--   heuristic_replay_runs      one manifest row per replay run.
--
-- LABELS (§1), entry = close[t]:
--   fwd_return_Ns   = (close[t+N]/close[t] - 1) * 100            N = 5, 10, 20
--   fwd_abs_move_Ns = max_{i=1..N} |close[t+i]/close[t] - 1| * 100
--   label_complete  = at least 20 sessions exist after t
--
-- LOCKBOX: phase2_lockbox_v2 (2025-03-28..2026-03-27, symbols outside
-- momentum_pilot_cohort). A row is in_lockbox when t, or any session of its
-- label window t+1..t+20, falls in the region, for a non-pilot symbol. Such
-- rows keep their signal columns and have EVERY label column NULL, including
-- label_complete; the CHECK below enforces it.
--
-- H9/H10 are pinned to 4/4 confluence only (buy_watch_c4, trim_watch_c4);
-- there are deliberately no any-confluence BUY_WATCH / TRIM_WATCH tables.

DO $$
DECLARE
    sig TEXT;
BEGIN
    FOREACH sig IN ARRAY ARRAY[
        'rsi_overbought', 'rsi_oversold', 'macd_bull_cross', 'macd_bear_cross',
        'bb_squeeze', 'bearish_pattern', 'bullish_pattern',
        'low_sweep_reclaim', 'high_sweep_reject', 'buy_watch_c4', 'trim_watch_c4'
    ] LOOP
        EXECUTE format($f$
            CREATE TABLE IF NOT EXISTS %I (
                symbol                   TEXT NOT NULL,
                t                        DATE NOT NULL,
                close_t                  DOUBLE PRECISION NOT NULL,

                -- signal context at the close of t (heuristics.ComputeAt)
                rsi14                    DOUBLE PRECISION,
                macd_hist                DOUBLE PRECISION,
                macd_bull_cross          BOOLEAN NOT NULL,
                macd_bear_cross          BOOLEAN NOT NULL,
                bb_squeeze               BOOLEAN,
                trend_direction          TEXT,
                bull_flag                BOOLEAN NOT NULL,
                bear_flag                BOOLEAN NOT NULL,
                hs_found                 BOOLEAN NOT NULL,
                hs_neckline_break        BOOLEAN NOT NULL,
                inv_hs_found             BOOLEAN NOT NULL,
                inv_hs_neckline_break    BOOLEAN NOT NULL,
                total_sweeps             INTEGER NOT NULL,
                low_sweep_on_bar         BOOLEAN NOT NULL,
                low_sweep_level          DOUBLE PRECISION,
                high_sweep_on_bar        BOOLEAN NOT NULL,
                high_sweep_level         DOUBLE PRECISION,
                last_sweep_kind          TEXT,
                last_sweep_bar_close     DOUBLE PRECISION,
                last_sweep_swept_level   DOUBLE PRECISION,
                last_bullish_ob          BOOLEAN NOT NULL,
                last_bearish_ob          BOOLEAN NOT NULL,
                vix                      DOUBLE PRECISION,
                vix_regime               TEXT,
                rule_action              TEXT,
                rule_confluence          INTEGER,
                sessions_since_prev_fire INTEGER,

                uptrend                  BOOLEAN,
                atr14                    DOUBLE PRECISION,
                atr_pct                  DOUBLE PRECISION,

                fwd_return_5s            DOUBLE PRECISION,
                fwd_return_10s           DOUBLE PRECISION,
                fwd_return_20s           DOUBLE PRECISION,
                fwd_abs_move_5s          DOUBLE PRECISION,
                fwd_abs_move_10s         DOUBLE PRECISION,
                fwd_abs_move_20s         DOUBLE PRECISION,
                label_complete           BOOLEAN,
                in_lockbox               BOOLEAN NOT NULL,

                harness_version          TEXT NOT NULL,
                computed_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
                PRIMARY KEY (symbol, t),
                CONSTRAINT %I CHECK (NOT in_lockbox OR (
                    fwd_return_5s IS NULL AND fwd_return_10s IS NULL AND fwd_return_20s IS NULL AND
                    fwd_abs_move_5s IS NULL AND fwd_abs_move_10s IS NULL AND fwd_abs_move_20s IS NULL AND
                    label_complete IS NULL))
            )$f$, 'heuristic_ep_' || sig, 'heuristic_ep_' || sig || '_lockbox_unlabelled');
        EXECUTE format('COMMENT ON TABLE %I IS %L', 'heuristic_ep_' || sig,
            'Episodes of heuristic signal ' || sig || ' (migration 027; one table per signal type, never pooled).');
    END LOOP;
END $$;

CREATE TABLE IF NOT EXISTS heuristic_comparison_days (
    symbol                    TEXT NOT NULL,
    t                         DATE NOT NULL,
    close_t                   DOUBLE PRECISION NOT NULL,

    fired_rsi_overbought      BOOLEAN NOT NULL,
    fired_rsi_oversold        BOOLEAN NOT NULL,
    fired_macd_bull_cross     BOOLEAN NOT NULL,
    fired_macd_bear_cross     BOOLEAN NOT NULL,
    fired_bb_squeeze          BOOLEAN NOT NULL,
    fired_bearish_pattern     BOOLEAN NOT NULL,
    fired_bullish_pattern     BOOLEAN NOT NULL,
    fired_low_sweep_reclaim   BOOLEAN NOT NULL,
    fired_high_sweep_reject   BOOLEAN NOT NULL,
    fired_buy_watch_c4        BOOLEAN NOT NULL,
    fired_trim_watch_c4       BOOLEAN NOT NULL,

    uptrend                   BOOLEAN,
    atr14                     DOUBLE PRECISION,
    atr_pct                   DOUBLE PRECISION,

    fwd_return_5s             DOUBLE PRECISION,
    fwd_return_10s            DOUBLE PRECISION,
    fwd_return_20s            DOUBLE PRECISION,
    fwd_abs_move_5s           DOUBLE PRECISION,
    fwd_abs_move_10s          DOUBLE PRECISION,
    fwd_abs_move_20s          DOUBLE PRECISION,
    label_complete            BOOLEAN,
    in_lockbox                BOOLEAN NOT NULL,

    harness_version           TEXT NOT NULL,
    computed_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (symbol, t),
    CONSTRAINT heuristic_comparison_days_lockbox_unlabelled CHECK (NOT in_lockbox OR (
        fwd_return_5s IS NULL AND fwd_return_10s IS NULL AND fwd_return_20s IS NULL AND
        fwd_abs_move_5s IS NULL AND fwd_abs_move_10s IS NULL AND fwd_abs_move_20s IS NULL AND
        label_complete IS NULL))
);

COMMENT ON TABLE heuristic_comparison_days IS
'Comparison sample for every heuristic hypothesis: session index i (0-based in the
symbol''s one-bar-per-session 1Day series) with i % 5 = 0, among evaluated sessions
(window >= 252 bars). fired_<signal> is whether that signal fired on t (not
whether an episode started). Non-signal days for signal X = rows with fired_X false.';

CREATE TABLE IF NOT EXISTS heuristic_replay_runs (
    run_id          BIGSERIAL PRIMARY KEY,
    harness_version TEXT NOT NULL,
    universe_rule   TEXT NOT NULL,
    symbol_count    INTEGER NOT NULL,
    symbols_done    INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL CHECK (status IN ('running', 'complete', 'failed', 'partial')),
    started_at      TIMESTAMPTZ NOT NULL,
    finished_at     TIMESTAMPTZ,
    git_commit      TEXT,
    git_dirty       BOOLEAN,
    params          JSONB NOT NULL,
    notes           TEXT
);

COMMENT ON TABLE heuristic_replay_runs IS
'Manifest of cmd/heuristics-replay runs: universe rule, symbol count, git commit and
the full parameter set (worker indicator params, episode gap, comparison sampling,
lockbox region) the rows were produced under.';
