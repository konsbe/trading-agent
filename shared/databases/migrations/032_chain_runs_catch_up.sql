-- 032: mark momentum chain runs that were caught up after their window.
--
-- A session whose chain did not run unattended in its window (the machine
-- slept over the close, bars landed after the give-up) can be caught up later:
-- by hand, or by momentum-daily's catch-up pass. Such a run must stay visible
-- as one: it never counts as a clean unattended session for the gate
-- (Tracked Positions addendum §6.1), and the analyst bot does not post alerts
-- from it, because it only posts for the session that just closed.
--
-- WRITERS
--
--   momentum-daily    sets catch_up = true when its catch-up pass runs a
--                     session; an operator does the same for a by-hand run.
--
-- READERS
--
--   analyst-bot       latest_scan_date ignores catch_up rows.
--   momentum-api      a catch_up row is never a clean session.
--
-- This supersedes 025's "a give-up is final": a session given up for missing
-- bars is now deferred to the catch-up; only exhausted attempts are final.
--
-- Additive: a column with a default that no running service reads or writes,
-- so it may be applied before the code that uses it (migrations README).

ALTER TABLE momentum_chain_runs
    ADD COLUMN IF NOT EXISTS catch_up BOOLEAN NOT NULL DEFAULT false;
