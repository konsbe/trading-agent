# mfe-backtest-lab

Backtest Lab remote micro frontend for trading-agent. It presents the frozen
Phase 2 backtest report, which explains why the screener makes no predictive
claim. It is backed by `momentum-api` (`docs/MOMENTUM_SCANNER_API.md`, Backtest
Lab addendum) and loaded by the spog shell (`web-app/spog`) under `/backtest-lab`.

The screen renders the whole report read-only: headline, the entry-gate test, the v2
score finding, the rvol stratification funnel, the research-round table and the closing
statement. See `src/features/BacktestReport/README.md`.

| | |
|---|---|
| Module Federation name / scope | `mfe_backtest_lab` |
| Exposed module | `./BacktestLab` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3003 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-watchlist` / `mfe-scanner`. UI primitives and
theme tokens come from `@trading-agent/shared-components`, compiled from source
(webpack alias to `src/mfe.ts`), so only this package's React copy is bundled. No
third-party UI or charting library is used. The stratification funnel is drawn
with plain CSS bars.

Hosted vs standalone: `app-root` (hosted) follows the shell's theme through the
user-data message bus and wraps the app in `HostModeProvider hosted`, so screens
omit the page title and the "Screener — not a forecast" pill, which spog's header
already shows. `bootstrap` (standalone) uses its own router and the OS theme.
`.backtest-lab-page--hosted` is the only scroll container inside spog's grid cell.

## momentum-api endpoint

| Endpoint | Function | Hook |
|---|---|---|
| `GET /api/v1/backtest-lab/report` | `fetchBacktestReport` | `useBacktestReport()` → `{report, error, isLoading}` |

The body is `shared/content/backtest_lab_report.json`, served verbatim. That file
is the authoritative shape, and `src/api/backtestLab/types.ts` mirrors it. The
report is static, a dated artifact, so the hook fetches once per mount and exposes
no refresh or polling.

`parseBacktestReport` (`src/api/backtestLab/parsers.ts`) is strict. Every required
field must be present with the right type, `closed_date` must be `YYYY-MM-DD`, and
each `ci` must be `[lower, upper]` with lower ≤ upper. A mismatch surfaces as
`invalid_response` instead of rendering a wrong number. The optional fields are
omitted (never null) and stay absent in the parsed result:

- `rvol_stratification_funnel.steps[].excess_odds`, `.composition_share_pct` and `.verdict` (per step)
- `research_round_1.hypotheses[].verdict_note` (currently only hypothesis `b`)

`research_round_1.abandoned[]` is required and may be empty.

The tests import the shared JSON file directly (`src/test-utils/fixtures.ts`),
not a copy, so they always run against the served report. They:

- check the parse round-trips the file exactly
- reject the body with each required key removed, one at a time
- check the numbers the addendum §3 pins: p 0.947, MH OR 0.991, composition 97%, final funnel OR 0.991

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3003` standalone. Both must be listed in
`MOMENTUM_API_CORS_ORIGINS` for momentum-api (repo-root `.env`):

```
MOMENTUM_API_CORS_ORIGINS=http://localhost:3000,http://localhost:3001,http://localhost:3002,http://localhost:3003
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
| `make run` | Dev server on :3003 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3003 (standalone)
make docker                                # trading-agent-mfe-backtest-lab:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-backtest-lab && make run   # :3003
# 3. the shell
cd web-app/spog && npm run start-dev:all     # :3000 → http://localhost:3000/backtest-lab
```

spog loads the remote from `mfes.mfe_backtest_lab` in `web-app/spog/public/config.json`.

## Layout

```
src/
  api/            fetch-client (GET, ApiError {status, code}), backtestLab/{types, parsers, backtestLabApi}
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper (ThemeProvider + host mode)
  common/         error copy, webpack MF helper
  components/     PageLayout (pill when standalone), ApiErrorState, ReportCard (always-expanded card)
  config/         api.config.ts
  features/       BacktestReport (report sections, format + funnel-scale utils)
  hooks/          useApiResource, backtestLab/useBacktestReport
  pages/          BacktestLabPage (loading / error / report)
  styles/         backtest-global.css (muted, eyebrow, mono, sr-only)
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  test-utils/     fixtures (imports shared/content/backtest_lab_report.json)
  types/          MF remote declarations, constants
```
