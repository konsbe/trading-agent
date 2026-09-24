# mfe-data-source

Data Source remote micro frontend for trading-agent: the current operational
health of the data pipeline (provider budgets and the daily scanner/tracker
chain), checked on demand. It is backed by `momentum-api`
(`docs/MOMENTUM_SCANNER_API.md`, Data Source addendum) and loaded by the spog
shell (`web-app/spog`) under `/data-source`.

The screen shows the overall status, the provider budgets and the daily chain, with a
manual Refresh and nothing on a timer. See `src/features/DataSourceStatus/README.md`.

| | |
|---|---|
| Module Federation name / scope | `mfe_data_source` |
| Exposed module | `./DataSource` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3004 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-watchlist` / `mfe-backtest-lab`. UI primitives
and theme tokens come from `@trading-agent/shared-components`, compiled from source
(webpack alias to `src/mfe.ts`), so only this package's React copy is bundled. No
third-party UI or charting library is used.

Hosted vs standalone: `app-root` (hosted) follows the shell's theme through the
user-data message bus and wraps the app in `HostModeProvider hosted`, so screens
omit the page title and the "Screener — not a forecast" pill, which spog's header
already shows. `bootstrap` (standalone) uses its own router and the OS theme.
`.data-source-page--hosted` is the only scroll container inside spog's grid cell.

## momentum-api endpoint

| Endpoint | Function | Hook |
|---|---|---|
| `GET /api/v1/data-sources/status` | `fetchDataSourceStatus` | `useDataSourceStatus()` → `{status, error, isLoading, isRefreshing, refresh}` |

The shape follows addendum §2.3 and `services/data-analyzer/internal/momentumapi/data_sources.go`.
Types are in `src/api/dataSources/types.ts`.

- `providers` and `daily_chain` are each either their object or the string
  `"unavailable"` (that section's query failed, and the other is still served).
  Narrow them with `isSectionUnavailable()`.
- `providers` is keyed by provider. JSON keys arrive sorted, so render in
  `PROVIDER_DISPLAY_ORDER` (`tiingo`, `finnhub`). A provider without a budget row
  is missing and named in `overall_reasons`.
- Finnhub has no daily cap, so its `daily_limit` and `daily_used_pct` are `null`.
  `theoretical_daily_capacity` is capacity, never a quota or a denominator.
  `degraded_count_24h` is always `null` today.
- Session `status` is one of `clean`, `completed_after_retry`, `pending`, `failed`,
  `not_run` or `not_recorded`. `note` is omitted unless the status needs one.

`parseDataSourceStatus` is strict:

- Unknown `overall` or session `status` values are rejected.
- A section may only be its object or exactly `"unavailable"`.
- Nullable fields must be present, whether they carry a value or `null`.
- `note` is either absent or a string.
- Dates must be `YYYY-MM-DD` and `checked_at` must be RFC 3339.
- Counts must be non-negative integers.

A mismatch surfaces as `invalid_response`.

**Errors.** `error` is a `DataSourceError {kind, status, code, message}`:

| `kind` | When |
|---|---|
| `database_unavailable` | `503 {"error":"database_unavailable"}`: momentum-api can't reach Postgres. On this page that is the operational problem itself, so render it prominently, not as a generic error |
| `network` | momentum-api unreachable |
| `invalid_response` | body didn't match the shape |
| `server` | any other HTTP error |
| `unexpected` | client-side bug |

**Refresh, never poll.** The hook fetches once on mount. `refresh()` re-fetches on
demand, and a call is ignored while a request is in flight. The previous `status`
stays visible while refreshing and after a failed refresh; its `checked_at` shows
its age. There are no timers, intervals, WebSockets or message-bus wiring (addendum §0).
The server caches for up to 60 s, so `checked_at` can be older than the click.

Tests use `src/test-utils/data-sources-status.live.json`, captured from the live
endpoint. Variants cover each section unavailable, both unavailable, the 503, one
row of every session status, finnhub's null limit, and rejections.

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3004` standalone. Both must be listed in
`MOMENTUM_API_CORS_ORIGINS` for momentum-api (repo-root `.env`):

```
MOMENTUM_API_CORS_ORIGINS=http://localhost:3000,http://localhost:3001,http://localhost:3002,http://localhost:3003,http://localhost:3004
```

## Prerequisites

- Node.js 22 (`nvm use 22`), npm 10
- Public npm registry (`https://registry.npmjs.org/`, set in `.npmrc`)
- Docker (for `make docker`)

## Commands

| Make target | What it does |
|-------------|--------------|
| `make build` | Install deps and production webpack build → `dist/` |
| `make test` | Jest unit tests |
| `make coverage` | Jest with coverage |
| `make typecheck` | `tsc --noEmit` |
| `make run` | Dev server on :3004 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3004 (standalone)
make docker                                # trading-agent-mfe-data-source:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-data-source && make run   # :3004
# 3. the shell
cd web-app/spog && npm run start-dev:all    # :3000 → http://localhost:3000/data-source
```

spog loads the remote from `mfes.mfe_data_source` in `web-app/spog/public/config.json`.

## Layout

```
src/
  api/            fetch-client (GET, ApiError {status, code}), dataSources/{types, parsers, errors, dataSourcesApi}
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper (ThemeProvider + host mode)
  common/         error copy, webpack MF helper
  components/     PageLayout (pill when standalone), ApiErrorState
  features/       DataSourceStatus (overall, providers, daily chain, DB-down panel, format + status utils)
  config/         api.config.ts
  hooks/          dataSources/useDataSourceStatus
  pages/          DataSourcePage (skeleton / errors / status)
  styles/         data-source-global.css (muted, mono, card, unavailable)
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  test-utils/     fixtures + data-sources-status.live.json
  types/          MF remote declarations, constants
```
