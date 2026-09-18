-- 011: give each rate budget its own daily-window boundary.
--
-- Migration 010 added daily_limit / daily_used / daily_window_start, and the
-- limiter rolled that window at UTC midnight for every provider. No provider we
-- use resets at UTC midnight.
--
-- Tiingo's own spec: "daily requests (reset at midnight EST)". With a UTC
-- boundary our counter reads fresh from 00:00 UTC while Tiingo's does not reset
-- until 05:00 UTC, so for five hours a day the daily ceiling was not real — we
-- could authorise a second full allowance on top of what the provider had
-- already counted.
--
-- Not biting yet (330 of 1,000 used on the pilot backfill), which is exactly why
-- it is worth fixing now: the same class as the source='yahoo' typo and the
-- setseed() sampling bug, cheap now and expensive to rediscover after it has
-- caused a real problem.
--
-- Amended by a NEW migration rather than by editing 010, per SCHEMAS.md: once a
-- migration is committed it is immutable.

ALTER TABLE api_rate_budget
    ADD COLUMN IF NOT EXISTS daily_reset_tz TEXT NOT NULL DEFAULT 'UTC';

COMMENT ON COLUMN api_rate_budget.daily_reset_tz IS
'Timezone whose midnight ends the daily window, e.g. UTC or EST. Postgres
resolves EST as a FIXED UTC-5 offset, which is what a provider documenting
"midnight EST" means; America/New_York would instead follow DST and reset an
hour early each summer. Resetting LATER than the provider is safe (we merely
under-use the allowance); resetting EARLIER over-spends it, so an unknown
provider reset should use the latest plausible boundary rather than UTC.';

-- Tiingo: documented as midnight EST.
UPDATE api_rate_budget SET daily_reset_tz = 'EST' WHERE budget_key = 'tiingo';

-- Twelve Data: reset time is NOT documented and has not been measured here.
-- EST is chosen deliberately as the conservative guess — if Twelve Data actually
-- resets at UTC midnight we simply wait five extra hours before reusing the
-- allowance, whereas the reverse error would overspend it.
UPDATE api_rate_budget SET daily_reset_tz = 'EST' WHERE budget_key = 'twelve_data';

-- Finnhub carries no daily ceiling, so the column is inert for it and stays UTC.
