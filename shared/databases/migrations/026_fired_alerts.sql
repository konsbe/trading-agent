-- 026: alerts the analyst-bot actually posted, one row per confirmed post.
--
-- Before this table, a fired alert existed only as a Discord message plus a
-- Redis cooldown flag. The flag was also set at detection time, before the post
-- was attempted, so a failed send burned the 4-hour cooldown with nothing posted
-- and nothing recorded.
--
-- WRITER
--
--   analyst-bot alert scan  one row after at least one notifier confirms the
--                           post, at the same point the Redis cooldown is set.
--                           Detected-but-unposted and cooldown-suppressed
--                           evaluations are never written: they were not fired
--                           from a user's point of view.
--
-- READERS
--
--   momentum-api  GET /api/v1/alerts and the candidates-list recent_alert join.
--
-- severity is the info | notice | warning scale from
-- docs/MOMENTUM_SCANNER_FULL_STOCK_ANALYSIS_API.md §2.3, not the Discord embed's
-- colour severity.
--
-- vix_elevated is market-wide; its row carries the symbol whose scan detected
-- it first, because that is the alert that was posted.

CREATE TABLE IF NOT EXISTS fired_alerts (
    id            BIGSERIAL PRIMARY KEY,
    symbol        TEXT NOT NULL,
    exchange_type TEXT NOT NULL CHECK (exchange_type IN ('equity', 'crypto')),
    alert_type    TEXT NOT NULL,
    interval      TEXT NOT NULL,
    value         NUMERIC,
    severity      TEXT NOT NULL CHECK (severity IN ('info', 'notice', 'warning')),
    message       TEXT NOT NULL,
    fired_at      TIMESTAMPTZ NOT NULL,
    UNIQUE (symbol, alert_type, interval, fired_at)
);

CREATE INDEX IF NOT EXISTS fired_alerts_symbol_fired_at_idx ON fired_alerts (symbol, fired_at DESC);
CREATE INDEX IF NOT EXISTS fired_alerts_fired_at_idx ON fired_alerts (fired_at DESC);
