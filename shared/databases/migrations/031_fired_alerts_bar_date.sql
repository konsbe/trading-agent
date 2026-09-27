-- 031: fired_alerts.bar_date — each alert is an onset on one bar, stored at
-- most once.
--
-- Until this migration the analyst bot re-posted a condition every cooldown
-- (4 h) while it stayed true, so fired_alerts holds many rows per condition.
-- From now on the bot posts only onsets (technical-analysis alert_onsets: the
-- condition started on that bar after at least 5 sessions without it), and
-- bar_date names the bar. The unique index makes a second row for the same
-- symbol, alert type and bar impossible whatever the code does; the bot claims
-- the row before posting and skips the post when the claim conflicts.
--
-- Old rows keep bar_date NULL (they were re-posts, not onsets); the CHECK is
-- NOT VALID so it binds every new or updated row without rewriting them.
-- Idempotent.

ALTER TABLE fired_alerts ADD COLUMN IF NOT EXISTS bar_date DATE;

CREATE UNIQUE INDEX IF NOT EXISTS fired_alerts_event_key
    ON fired_alerts (symbol, alert_type, bar_date);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fired_alerts_bar_date_required') THEN
        ALTER TABLE fired_alerts ADD CONSTRAINT fired_alerts_bar_date_required
            CHECK (bar_date IS NOT NULL) NOT VALID;
    END IF;
END $$;
