# mfe-scanner

Momentum scanner remote micro frontend for trading-agent. Shows today's scan
candidates (market + penny buckets) and a per-symbol score breakdown, backed by
`momentum-api` (`docs/MOMENTUM_SCANNER_API.md`). Loaded by the spog shell
(`web-app/spog`) under `/candidates`.

| | |
|---|---|
| Module Federation name / scope | `mfe_scanner` |
| Exposed module | `./Scanner` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3001 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

UI primitives and theme tokens come from `@trading-agent/shared-components`
(`web-app/shared-components`), compiled from source (webpack alias to
`src/mfe.ts`, like spog aliases `src/shell.ts`), so no shared-components build
step is needed and only this package's React copy is bundled. No third-party UI
library is used; the one charting dependency is
[TradingView Lightweight Charts](https://github.com/tradingview/lightweight-charts)
v5 (Apache-2.0), approved for price charts (`docs/design-system/README.md` § Charts).

## Routes

Relative routes, so the same router works standalone (mounted at `/`) and
hosted (mounted under spog's `candidates/*`):

| Path | Page |
|------|------|
| index | `CandidatesPage` — both buckets as sortable tables (default RVOL desc, nulls last), top 10 + "and N more" per bucket, row/Enter/link to detail; Change % uses `--color-price-up`/`--color-price-down` (zero neutral) |
| `:symbol` | `CandidateDetailPage` — Stitch "Stock Detail & Score Breakdown": header, gates panel (value vs threshold per check), evidence note, primary facts matrix, price chart (1D 5D 1M 6M 1Y ALL), score breakdown (sub-metric evaluator, penalty rules, footer), shared-watchlist toggle |

Hosted vs standalone: `app-root` (hosted) wraps the app in `HostModeProvider hosted`;
`bootstrap` (standalone) does not. Hosted screens omit the page title and the
"Screener — not a forecast" pill because spog's header already shows both.
Hosted, spog never scrolls: `.scanner-page--hosted` is the only scroll
container. It is `position: relative`, so absolutely positioned descendants
(chart internals, visually hidden labels) scroll with it instead of stretching
the shell's document. Its height comes from spog's grid cell (spog's rows and
cell content use `min-height: 0`), so `overflow: auto` scrolls only when the
content is genuinely taller than the cell.

The list's Market/Penny buckets and the detail widgets (gates, primary facts,
price chart, score breakdown) use `CollapsibleCard` from
`@trading-agent/shared-components`: click the header or press Enter/Space to
collapse or expand. Header meta (the "Sorted by …" label, the "n/m met" badge,
the computed time, the chart's range tabs, the score status) stays visible and
is outside the toggle. Each card's state is kept in `sessionStorage`
(`ta-collapsible:scanner.list.<bucket>` / `scanner.detail.<widget>`), so a
collapsed card stays collapsed while clicking through candidates in the same
tab. Bucket sort order and "and N more" survive collapse/expand. Collapsing the
chart disposes it; expanding builds a new one at the visible size.

## momentum-api endpoints used

| Endpoint | Used by |
|---|---|
| `GET /api/v1/scanner/today` | `useScannerToday` → list |
| `GET /api/v1/scanner/today/{symbol}` (incl. `facts`, `gates`, `score.weights`, `score.penalty_rules`) | `useScannerSymbol` → detail |
| `GET /api/v1/scanner/symbols/{symbol}/bars?range=1D\|5D\|1M\|6M\|1Y\|ALL` | `usePriceBars` → price chart |
| `GET /api/v1/watchlist`, `PUT`/`DELETE /api/v1/watchlist/{symbol}` | `useWatchlist` → watchlist button (shared, unauthenticated list; optimistic with rollback) |

Specs: `docs/MOMENTUM_SCANNER_API.md` §2.1–§2.6. All responses go through strict
parsers in `src/api/scanner/parsers.ts`; a shape mismatch surfaces as
`invalid_response` rather than rendering wrong data.

## Price chart

`features/CandidateDetail/components/PriceChart`: candlesticks plus a volume
histogram on its own bottom scale. `chartAdapter.ts` is the only module that
imports `lightweight-charts`, so tests mock it (Jest maps the library to
`__mocks__/lightweight-charts.ts`; jsdom has no canvas). Colours are read at
runtime from `--color-price-up` / `--color-price-down` / `--color-outline-variant`
/ `--color-on-surface-variant` / `--color-outline` and re-applied when
`data-theme` / `style` changes on the nearest `[data-theme]` ancestor or
`<html>`. Intraday (5-minute) axes use New York time; daily bars use UTC dates.
The chart auto-sizes to its container and is disposed on unmount.

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests originate from the page origin — `http://localhost:3000` when
hosted, `http://localhost:3001` standalone — so both must be listed in
`MOMENTUM_API_CORS_ORIGINS` for momentum-api (repo-root `.env`).

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
| `make run` | Dev server on :3001 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3001 (standalone)
make docker                                # trading-agent-mfe-scanner:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-scanner && make run      # :3001
# 3. the shell
cd web-app/spog && npm run start-dev:all   # :3000 → http://localhost:3000/candidates
```

spog loads the remote from `mfes.mfe_scanner` in `web-app/spog/public/config.json`.

## Layout

```
src/
  api/            fetch-client (GET/PUT/DELETE, ApiError {status, code}), types + strict parsers + endpoint functions
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper (ThemeProvider)
  common/         formatters + code humanizer, error copy, webpack MF helper
  components/     PageLayout (pill when standalone), ApiErrorState, StatusNotice
  config/         api.config.ts
  features/       Candidates/{BucketSection, CandidatesTable, CandidatesSkeleton, StaleScanBanner, hooks, utils/sortCandidates}
                  CandidateDetail/{DetailHeader, GatesPanel, EvidenceNote, FactsMatrix, PriceChart, ScoreBreakdown, WatchlistButton, utils/describe}
  hooks/          useApiResource, scanner/{useScannerToday, useScannerSymbol, usePriceBars}, watchlist/useWatchlist
  pages/          CandidatesPage, CandidateDetailPage
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  styles/         scanner-global.css (card, link, code classes)
  types/          MF remote declarations, constants
```
