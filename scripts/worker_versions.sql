-- Which build(s) are working the backfill queue?
--
-- Run this whenever a re-fetch or migration produces data that is populated
-- for "most but not all" rows. That pattern reads as a bug in the column, and
-- once it was two binaries instead:
--
--   a data-universe process launched the previous evening was still running on
--   the OLD build while a freshly compiled one worked the same
--   backfill_status='pending' queue. Both claimed rows atomically, neither
--   errored, and the only symptom was a new column populated for 64.5% of
--   bars. Rebuilding a file does not restart a running daemon.
--
-- MORE THAN ONE ROW IN THE FIRST RESULT MEANS MORE THAN ONE BUILD IS ACTIVE.
-- That is almost never intentional.
--
--   psql "$DATABASE_URL" -f scripts/worker_versions.sql

\echo ''
\echo '== builds that have claimed rows in the last 24h =='
SELECT COALESCE(backfill_worker_version, '(pre-migration-022, unstamped)') AS build,
       count(*)                                                           AS rows_claimed,
       min(updated_at)                                                    AS first_seen,
       max(updated_at)                                                    AS last_seen
FROM universe_symbols
-- updated_at, not backfill_claimed_at: the latter is cleared on completion,
-- so filtering on it shows only rows still in flight and would report "no
-- activity" moments after a successful run.
WHERE updated_at > now() - interval '24 hours'
  AND backfill_worker_version IS NOT NULL
GROUP BY 1
ORDER BY 4 DESC NULLS LAST;

\echo ''
\echo '== VERDICT =='
SELECT CASE
         WHEN count(DISTINCT backfill_worker_version) > 1
           THEN 'CONFLICT: ' || count(DISTINCT backfill_worker_version)
                || ' builds claimed rows in the last 24h. Check for a stale daemon: '
                || 'ps -eo pid,lstart,cmd | grep data-universe. '
                || 'Kill the older PID; a rebuild does not restart it.'
         WHEN count(DISTINCT backfill_worker_version) = 1
           THEN 'OK: one build (' || max(backfill_worker_version) || ') is working the queue.'
         ELSE 'No rows claimed in the last 24h.'
       END AS verdict
FROM universe_symbols
WHERE updated_at > now() - interval '24 hours'
  AND backfill_worker_version IS NOT NULL;

\echo ''
\echo '== symbols excluded as unrefreshable (migration 022) =='
SELECT symbol, data_unavailable_since::date AS since, data_unavailable_reason AS reason
FROM universe_symbols
WHERE data_unavailable_reason IS NOT NULL
ORDER BY symbol;
