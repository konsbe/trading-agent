-- 025: durable progress of the daily momentum chain, one row per NYSE session.
--
-- momentum-daily used to remember "session done" in memory only. A restart
-- therefore forgot what had finished, and a kill mid-run could not be told
-- apart from a finished run. Each step now records its own completion here,
-- inside the same transaction as its writes where it has one, so a killed step
-- leaves no marker and is simply run again.
--
-- WRITERS
--
--   momentum-scanner  scanner_completed_at — in the SAME transaction as the
--                     session's momentum_features / momentum_scores rows. A
--                     marker therefore means the whole scan committed.
--   momentum-tracker  tracker_completed_at — after every row is evaluated and
--                     every candidate opened. Its per-row writes are idempotent
--                     (evaluation floor + ON CONFLICT DO NOTHING), so a kill
--                     before the marker is repaired by running it again.
--   momentum-daily    attempts, gave_up_at, last_error — so the retry limit
--                     and a give-up survive restarts.
--
-- READERS
--
--   momentum-daily    a session is done iff tracker_completed_at IS NOT NULL.
--   analyst-bot       alerts only for a session with scanner_completed_at, so a
--                     partial scan can never be alerted on.
--
-- A give-up is final. To retry a session by hand, use the recovery SQL in
-- services/data-analyzer/data_analyzer.md (momentum-daily section).

CREATE TABLE IF NOT EXISTS momentum_chain_runs (
    session               DATE         PRIMARY KEY,
    attempts              INTEGER      NOT NULL DEFAULT 0,
    scanner_completed_at  TIMESTAMPTZ  NULL,
    tracker_completed_at  TIMESTAMPTZ  NULL,
    gave_up_at            TIMESTAMPTZ  NULL,
    last_error            TEXT         NULL,
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);
