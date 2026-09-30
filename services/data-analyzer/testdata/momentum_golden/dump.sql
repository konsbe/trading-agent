-- What a scanner + tracker run wrote, with dates as offsets from :'D' and
-- write timestamps (computed_at, scored_at, *_completed_at, ...) left out, so
-- two runs on different days or seconds compare byte for byte.
\pset format unaligned
\pset tuples_only on
\pset footer off

SELECT 'features ' || (ts::date - :'D'::date) || ' ' ||
       (to_jsonb(f) - 'ts' - 'computed_at' - 'catalyst_checked_at')::text
FROM momentum_features f ORDER BY symbol, ts;

SELECT 'scores ' || (ts::date - :'D'::date) || ' ' ||
       (to_jsonb(s) - 'ts' - 'scored_at')::text
FROM momentum_scores s ORDER BY symbol, ts;

SELECT 'tracked alerted=' || (alerted_ts::date - :'D'::date) ||
       ' evaluated=' || coalesce((last_evaluated_ts::date - :'D'::date)::text, '-') ||
       ' exit=' || coalesce((exit_ts::date - :'D'::date)::text, '-') || ' ' ||
       (to_jsonb(t) - 'alerted_ts' - 'last_evaluated_ts' - 'exit_ts' - 'created_at' - 'updated_at')::text
FROM momentum_tracked t ORDER BY symbol, alerted_ts;

SELECT 'chain ' || (session - :'D'::date) || ' attempts=' || attempts ||
       ' scanned=' || (scanner_completed_at IS NOT NULL) ||
       ' tracked=' || (tracker_completed_at IS NOT NULL) ||
       ' gave_up=' || (gave_up_at IS NOT NULL) || ' catch_up=' || catch_up
FROM momentum_chain_runs ORDER BY session;
