-- 028: one-shot lockbox evaluation (docs/HEURISTIC_SIGNALS_PREREGISTRATION.md §5g).
--
-- The lockbox is spent once per hypothesis. heuristic_lockbox_runs has one row
-- per hypothesis, inserted before any lockbox label is computed, so a second
-- run fails on the primary key instead of quietly re-reading the lockbox.
--
-- heuristic_lockbox_h2_rows holds the labels for H2's lockbox rows only:
-- rsi_oversold lockbox episodes (grp 1) and lockbox comparison days on which
-- rsi_oversold did not fire (grp 0). No other hypothesis's lockbox outcome is
-- computed. Written by cmd/heuristics-lockbox; read by
-- scripts/heuristics_lockbox_h2.py.

CREATE TABLE IF NOT EXISTS heuristic_lockbox_runs (
    hypothesis   TEXT PRIMARY KEY,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ,
    status       TEXT NOT NULL DEFAULT 'running',
    git_commit   TEXT,
    rows_written INTEGER,
    rows_missing INTEGER
);

CREATE TABLE IF NOT EXISTS heuristic_lockbox_h2_rows (
    symbol           TEXT NOT NULL,
    t                DATE NOT NULL,
    grp              SMALLINT NOT NULL CHECK (grp IN (0, 1)),
    atr_pct          DOUBLE PRECISION,
    fwd_return_5s    DOUBLE PRECISION,
    fwd_return_10s   DOUBLE PRECISION,
    fwd_return_20s   DOUBLE PRECISION,
    fwd_abs_move_10s DOUBLE PRECISION,
    label_complete   BOOLEAN NOT NULL,
    PRIMARY KEY (symbol, t, grp)
);
