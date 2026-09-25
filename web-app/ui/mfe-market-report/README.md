# mfe-market-report

Daily Market Report remote micro frontend for trading-agent. It shows the
market-wide macro sections (monetary policy, growth, inflation, geopolitical,
correlations, seasonality, news) and the per-instrument section (a fixed list
plus the watchlist: price, yield and market-cycle phase). It is backed by
`momentum-api` and loaded by the spog shell (`web-app/spog`) under
`/market-report`, listed in the sidebar as "Daily Market Report" right after
"Today's Candidates".

The screen shows the market overview, the tracked and watchlist instruments, seasonality,
the calendar and news, and a data-coverage note. See `src/features/MarketReport/README.md`.

| | |
|---|---|
| Module Federation name / scope | `mfe_market_report` |
| Exposed module | `./MarketReport` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3005 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-data-source` / `mfe-backtest-lab`. UI primitives
and theme tokens come from `@trading-agent/shared-components`, compiled from source
(webpack alias to `src/mfe.ts`), so only this package's React copy is bundled. No
third-party UI or charting library is used.

Hosted vs standalone: `app-root` (hosted) follows the shell's theme through the
user-data message bus and wraps the app in `HostModeProvider hosted`, so screens
omit the page title and the "Screener — not a forecast" pill, which spog's header
already shows. `bootstrap` (standalone) uses its own router and the OS theme.
`.market-report-page--hosted` is the only scroll container inside spog's grid cell.

## momentum-api endpoint

| Endpoint | Function | Hook |
|---|---|---|
| `GET /api/v1/market-report/today` | `fetchMarketReport` | `useMarketReport()` → `{report, error, isLoading}` |

The authoritative shape is `services/data-analyzer/internal/momentumapi/market_report.go`.
The example in `docs/MOMENTUM_SCANNER_API_DAILY_MARKET_REPORT.md` §2.1 predates it.
Types are in `src/api/marketReport/types.ts`.

**What the parser (`parseMarketReport`) checks.** It is strict on everything the
report schema defines:

- keys and types, and the `type` / `source` / earnings-coverage `status` enums
- `YYYY-MM-DD` dates and RFC 3339 timestamps
- `report_date` and `generated_at` are either both null or both set
- omitted keys (`price`, `yield`, `unavailable_reason`, `note`) are either absent or typed, never null
- a `treasury_yield` never carries `price` or `market_cycle`, and a non-yield never carries `yield`
- a missing price, yield or market cycle always has an `unavailable_reason`

The pipeline's payloads are passed through without being typed further:

- `stance`, `signals.*.payload` and `payload`: any JSON
- `seasonality`, `presidential_cycle`, `intermarket`, `gpr`, `gdelt` and the calendar/earnings rows: any object
- `automation_status`: each module's `hint` / `status` must be strings, and every status is kept as-is

The `market_cycle` values are copied from the pipeline, so each is nullable.

A mismatch surfaces as `invalid_response`.

**Errors.** `error` is a `MarketReportError {kind, status, code, message}`. `kind` is
`database_unavailable` (the 503), `network`, `invalid_response`, `server` or
`unexpected`.

**No refresh.** The hook fetches once per mount, with no polling, timers or refresh.
The report is generated every 6 h, and `generated_at` / `is_stale` state its age. A
report older than two runs, or no report yet (`report_date: null`), is `is_stale: true`.

Tests use `src/test-utils/market-report-today.live.json`, captured from the live
endpoint. Variants cover:

- no report yet
- an instrument with no data
- a yield without observations
- crypto and watchlist instruments
- a stale report and a session-open bar
- raw payload shapes
- rejections of every schema-defined field and invariant

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3005` standalone. Both must be in `MOMENTUM_API_CORS_ORIGINS`
for momentum-api (repo-root `.env`); `:3005` is already listed.

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
| `make run` | Dev server on :3005 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3005 (standalone)
make docker                                # trading-agent-mfe-market-report:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-market-report && make run   # :3005
# 3. the shell
cd web-app/spog && npm run start-dev:all      # :3000 → http://localhost:3000/market-report
```

spog loads the remote from `mfes.mfe_market_report` in `web-app/spog/public/config.json`.

## Layout

```
src/
  api/            fetch-client (GET, ApiError {status, code}), marketReport/{types, parsers, errors, marketReportApi}
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper (ThemeProvider + host mode)
  common/         error copy, format/sign.ts (U+2212 minus, identical to mfe-scanner), webpack MF helper
  components/     PageLayout (pill when standalone), ApiErrorState
  features/       MarketReport (report sections, format / humanize / payload utils)
  config/         api.config.ts
  hooks/          marketReport/useMarketReport
  pages/          MarketReportPage (skeleton / error / report)
  styles/         market-report-global.css (muted, mono, card, tile, list)
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  test-utils/     fixtures + market-report-today.live.json
  types/          MF remote declarations, constants
```
