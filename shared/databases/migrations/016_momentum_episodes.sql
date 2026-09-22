-- 016: momentum_episodes (Phase 2 §3.1).
--
-- WHY A TABLE AND NOT A VIEW
--
-- §3.1 requires the episode set to be STABLE and manifestable. A view would
-- recompute on every read, so the set could change silently whenever a bar
-- arrived or a gate input was corrected -- and a training set that moves
-- underneath a reported result cannot be audited. The spec is explicit: store
-- episodes in a table, not a view.
--
-- WHAT AN EPISODE IS
--
--   the first gate pass for a symbol after >= gap_sessions sessions with no
--   gate pass  (default 5, matching the alert cooldown)
--
-- Features and labels are taken from the episode's FIRST day, because that is
-- the day an alert would fire.
--
-- WHY IT MATTERS
--
-- Consecutive gate-passing days of one move share almost the same forward
-- label. Counting them as independent observations inflates n and makes every
-- p-value look stronger than the evidence supports. In the post-upgrade data
-- the row count is 7,460 and the episode count 6,924, so the inflation is
-- modest in aggregate -- but it is concentrated in exactly the long-running
-- moves a model would otherwise overweight.
--
-- GAP IS A COLUMN, NOT A CONSTANT
--
-- §3.1 requires results reported for gaps of 3, 5 and 10. Storing the gap per
-- row lets all three coexist and be compared, instead of one overwriting
-- another. Every query must therefore filter on gap_sessions; a query that
-- forgets will silently union three overlapping definitions, which is the same
-- class of mistake as the unscoped report harness.

CREATE TABLE IF NOT EXISTS momentum_episodes (
    symbol        TEXT    NOT NULL,
    episode_start DATE    NOT NULL,
    gap_sessions  INTEGER NOT NULL,

    -- Where the episode sits in the symbol's own bar series. Session-based
    -- rather than calendar-based because the gap is defined in TRADING
    -- sessions: a calendar gap would count weekends and halts as if the symbol
    -- had traded through them.
    start_bar_index INTEGER NOT NULL,

    -- How many gate-passing days the episode ran, and when it ended. Carried
    -- so "one move" can be described without re-deriving it, and so an episode
    -- that ran 30 sessions is distinguishable from one that ran 1.
    gate_days     INTEGER NOT NULL,
    episode_end   DATE    NOT NULL,

    bucket        TEXT,
    score_total   INTEGER,

    -- Label taken from the FIRST day, per §3.1.
    fwd_max_gain_pct     DOUBLE PRECISION,
    fwd_max_drawdown_pct DOUBLE PRECISION,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (symbol, episode_start, gap_sessions)
);

COMMENT ON TABLE momentum_episodes IS
'Phase 2 §3.1 episodes: the first gate pass for a symbol after >= gap_sessions
sessions without one. ALL Phase 2 training and evaluation happen at this level,
because consecutive gate days of a single move are not independent
observations. Rows exist for multiple gap_sessions values (3, 5, 10) so the
sensitivity required by §3.1 can be reported; EVERY query must filter on
gap_sessions or it will union overlapping definitions of the same events.';

COMMENT ON COLUMN momentum_episodes.start_bar_index IS
'Index of the episode''s first day within that symbol''s bar series. The gap is
measured in trading sessions, and only an index expresses that -- a date
difference would treat weekends, holidays and trading halts as elapsed
sessions, which would merge episodes that a session count keeps apart.';

COMMENT ON COLUMN momentum_episodes.gate_days IS
'Number of gate-passing days in this episode. 1 means a single-day pass. Large
values are the long-running moves that row-level inference overweights, so this
column is what makes that inflation measurable rather than asserted.';
