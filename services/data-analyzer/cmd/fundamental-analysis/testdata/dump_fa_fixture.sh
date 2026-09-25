#!/usr/bin/env bash
# Regenerate fa_fixture.json from the live DB (read-only SELECTs).
#
#   services/data-analyzer/cmd/fundamental-analysis/testdata/dump_fa_fixture.sh
#
# The fixture is the raw input of the golden test, not its expected output:
# the latest row per (source, period, metric) for a fully covered watchlist
# symbol (MSFT) and for a scanner candidate that only has finnhub_metric /
# finnhub_profile2 rows (WRBY), renamed so they cannot collide with real data.
# Regenerating it changes the golden output too; re-run the test with -update
# only when that is the point.
set -euo pipefail
cd "$(dirname "$0")"
docker exec ta-phase1 psql -U postgres -d trading -At -c "
SELECT json_build_object(
  'fundamentals', (SELECT json_agg(r ORDER BY r.symbol, r.source, r.period, r.metric) FROM (
      SELECT DISTINCT ON (symbol, source, period, metric)
             CASE symbol WHEN 'MSFT' THEN 'ZZFAFULL' ELSE 'ZZFATHIN' END AS symbol,
             ts, period, metric, value, payload, source
      FROM equity_fundamentals
      WHERE symbol IN ('MSFT', 'WRBY') AND source <> 'fundamental_analysis'
      ORDER BY symbol, source, period, metric, ts DESC) r),
  'closes', (SELECT json_agg(c ORDER BY c.symbol) FROM (
      SELECT DISTINCT ON (symbol)
             CASE symbol WHEN 'MSFT' THEN 'ZZFAFULL' ELSE 'ZZFATHIN' END AS symbol,
             ts, close
      FROM equity_ohlcv
      WHERE symbol IN ('MSFT', 'WRBY') AND interval = '1Day'
      ORDER BY symbol, ts DESC) c)
)" > fa_fixture.json
echo "wrote $(wc -c < fa_fixture.json) bytes to fa_fixture.json"
