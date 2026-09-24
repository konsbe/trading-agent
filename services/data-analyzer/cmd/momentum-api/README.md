# momentum-api

Read-only HTTP API over the momentum scanner's stored output, for the web app's
"Today's Candidates" screen. Spec: [`docs/MOMENTUM_SCANNER_API.md`](../../../../docs/MOMENTUM_SCANNER_API.md).

It computes nothing, and writes nothing except the watchlist (§2.5 of the spec). Every value is what `momentum-scanner`
already persisted to `momentum_features`, `momentum_scores` and
`universe_symbols`. The read logic mirrors
`services/analyst-bot/db/queries/momentum.py`.

> **⚠️ No authentication.** There is no identity provider yet to validate tokens
> against, so none is implemented — deliberately, rather than shipping an auth
> path nobody can exercise. The server binds `127.0.0.1:8090` by default and
> Compose publishes it on `127.0.0.1` only. **Do not expose it beyond this
> machine or a trusted private network.** Real auth (validating the tokens the
> `spog` shell obtains from Keycloak) is a hard blocker before any wider
> exposure, and should be built when a Keycloak instance exists to test it
> against.

## Endpoints

| Route | Returns |
|-------|---------|
| `GET /healthz` | `200 {"status":"ok"}` when the DB answers, else `503` |
| `GET /api/v1/scanner/today` | Latest scan: header + every gate-passing candidate, both buckets, ordered `rvol_20 DESC` |
| `GET /api/v1/scanner/today/{symbol}` | One symbol's row for the latest scan (case-insensitive), including gate failures and the full score breakdown |
| `GET /api/v1/scanner/symbols/{symbol}/bars?range=1D\|5D\|1M\|6M\|1Y\|ALL` | Price history for the chart (5-minute bars for 1D/5D when stored, else daily with `fallback`) |
| `GET /api/v1/scanner/tracked?status=active\|closed\|all` | Tracked Positions (read-only view of `momentum_tracked`, with a per-rule `exit_reason_note` from the shared caveats file) |
| `GET /api/v1/watchlist` · `PUT` / `DELETE /api/v1/watchlist/{symbol}` | The watchlist — the service's only write path (table `watchlist_items`, migration 024). Unauthenticated list until auth exists |

| Condition | Response |
|-----------|----------|
| No scan has ever been stored | `503 {"error":"no_scan_available"}` |
| Database unreachable | `503 {"error":"database_unavailable"}` |
| Query failed against a healthy DB | `500 {"error":"internal_error"}` |
| Holiday calendar doesn't cover today | `500 {"error":"session_calendar_unavailable"}` |
| Symbol has no row for the latest scan | `404 {"error":"no_data_for_symbol"}` |
| Stale scan | Still `200`, with `scan.is_stale: true` |

Every candidate carries `score_status: "unvalidated"`. A missing score is `null`,
never `0`. The detail view returns both shared caveats: `evidence_note`
(`EVIDENCE_CAVEAT`) and `score.caveat` (`RESEARCH_SCORE_CAVEAT`).

## How the fields are sourced

- **`close`** is the features row's own close, the price as traded on the scan
  day. `equity_ohlcv.close` is back-adjusted by later dividends and splits, so it
  is not joined.
- **`scan.completed_at`** is the later of `max(computed_at)` and `max(scored_at)`
  for the scan date. Both reset on every upsert. There is no run-completion
  record, so a run that crashed midway still reports its last write time.
- **`scan.universe_eligible`** is `COUNT(*) WHERE is_eligible` (§3.1). Symbols
  the scanner skipped show up as `universe_scanned` being lower, not as a smaller
  denominator.
- **`scan.is_stale`** compares the scan date with the most recent NYSE session
  whose 16:00 New York close plus `MOMENTUM_API_SCAN_GRACE` has passed. Holidays
  come from a hard-coded list in `internal/momentumapi/calendar.go` (2024–2027).
  Extend it before it runs out: the service warns 60 days ahead, a test fails six
  months ahead, and past the end `/today` returns `session_calendar_unavailable`
  rather than guessing.
- **`score.attainable`** is 90 minus the weights of the components in
  `null_inputs` (75 while `catalyst_tier` is null). `score.status` and
  `score.model_version` come from `internal/momentum/score_status.go`.
- **Caveats** are read at startup from `shared/content/momentum_caveats.json`,
  the same file `analyst-bot` reads. A missing file stops the service; there is
  no built-in fallback text.

## Configuration

| Env var | Default | Meaning |
|---------|---------|---------|
| `DATABASE_URL` | — (required) | Postgres/TimescaleDB DSN |
| `MOMENTUM_API_ADDR` | `127.0.0.1:8090` | Listen address. Keep it on loopback or a private network |
| `MOMENTUM_API_CORS_ORIGINS` | `http://localhost:3000` | Comma-separated allowed origins (`*` allows any) |
| `MOMENTUM_API_CACHE_TTL` | `5m` | Response cache TTL, capped at 5m |
| `MOMENTUM_API_SCAN_GRACE` | `6h` | Time after the 16:00 NY close before a session's scan is expected |
| `MOMENTUM_CAVEATS_PATH` | `../../shared/content/momentum_caveats.json` | Shared caveats file (relative to the working directory) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Run, test, build

From `services/data-analyzer`:

```bash
make run-momentum-api                     # go build + run; loads .env, then the repo-root ../../.env
curl -s localhost:8090/api/v1/scanner/today | jq

make test                                 # unit tests
TEST_DATABASE_URL=postgres://... make test-integration
                                          # real-Postgres tests; fixtures live in a
                                          # transaction that is always rolled back
```

Docker, from the repo root:

```bash
make up-api     # docker compose --profile api up momentum-api  → 127.0.0.1:8090
make log-api
```

The image is the shared `services/data-analyzer` image (`/app/momentum-api`);
Compose mounts `shared/content` read-only for the caveats file.
