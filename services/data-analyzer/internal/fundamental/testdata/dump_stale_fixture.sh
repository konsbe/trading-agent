#!/usr/bin/env bash
# Regenerate stale_fixture.json from the live DB (read-only SELECTs).
#
#   services/data-analyzer/internal/fundamental/testdata/dump_stale_fixture.sh
#
# Raw input of stale_integration_test.go: the latest raw row per
# (source, period, metric) for COHR and INTC, and the derived rows that were
# still their latest on 2026-09-26 although the fixed code no longer writes
# them (COHR's interest coverage −5.94 from FY2023 operating income over FY2020
# interest; INTC's FCF conversion and DCF from FY2021 FCF). Symbols are renamed
# so they cannot collide with real data. Dumped before the fix was deployed:
# re-running it afterwards no longer finds those derived rows.
set -euo pipefail
cd "$(dirname "$0")"
docker exec ta-phase1 psql -U postgres -d trading -At -c "
SELECT json_build_object(
  'fundamentals', (SELECT json_agg(r ORDER BY r.symbol, r.source, r.period, r.metric) FROM (
      SELECT DISTINCT ON (symbol, source, period, metric)
             'ZZ' || symbol AS symbol, ts, period, metric, value,
             -- report_raw carries the whole filing; the analysis reads only its span.
             CASE WHEN metric = 'report_raw'
                  THEN jsonb_build_object('form', payload->'form', 'startDate', payload->'startDate', 'endDate', payload->'endDate')
                  ELSE payload END AS payload,
             source
      FROM equity_fundamentals
      WHERE symbol IN ('COHR', 'INTC') AND source <> 'fundamental_analysis'
      ORDER BY symbol, source, period, metric, ts DESC) r),
  'stale_derived', (SELECT json_agg(d ORDER BY d.symbol, d.metric) FROM (
      SELECT DISTINCT ON (symbol, metric)
             'ZZ' || symbol AS symbol, ts, period, metric, value, payload, source
      FROM equity_fundamentals
      WHERE source = 'fundamental_analysis'
        AND (symbol, metric) IN (('COHR', 't3_interest_coverage'),
                                 ('INTC', 't3_fcf_conversion'), ('INTC', 't3_dcf'))
      ORDER BY symbol, metric, ts DESC) d),
  'closes', (SELECT json_agg(c ORDER BY c.symbol) FROM (
      SELECT DISTINCT ON (symbol) 'ZZ' || symbol AS symbol, ts, close
      FROM equity_ohlcv
      WHERE symbol IN ('COHR', 'INTC') AND interval = '1Day'
      ORDER BY symbol, ts DESC) c)
)" > stale_fixture.json
echo "wrote $(wc -c < stale_fixture.json) bytes to stale_fixture.json"
