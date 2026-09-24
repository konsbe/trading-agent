# mfe-watchlist

Watchlist remote micro frontend for trading-agent: the shared watchlist with
each symbol's current price data, plus symbol search for adding symbols. Backed by
`momentum-api` (`docs/MOMENTUM_SCANNER_API.md` §2.5). Loaded by the spog shell
(`web-app/spog`) under `/watchlist`.

The screen is still a placeholder. The typed API layer and hooks below are done.

| | |
|---|---|
| Module Federation name / scope | `mfe_watchlist` |
| Exposed module | `./Watchlist` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3002 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-scanner`. UI primitives and theme tokens come from
`@trading-agent/shared-components`, compiled from source (webpack alias to
`src/mfe.ts`), so only this package's React copy is bundled. No third-party UI
library is used.

Hosted vs standalone: `app-root` (hosted) follows the shell's theme through the
user-data message bus and wraps the app in `HostModeProvider hosted`, so screens
omit the page title and the "Screener — not a forecast" pill, which spog's header
already shows. `bootstrap` (standalone) uses its own router and the OS theme.
`.watchlist-page--hosted` is the only scroll container inside spog's grid cell.

## momentum-api endpoints

| Endpoint | Function | Result |
|---|---|---|
| `GET /api/v1/watchlist` | `fetchWatchlist` | `{owner, items: [{symbol, company_name, exchange, added_at, as_of, is_stale, close, change_pct, rvol_20}]}`, newest first |
| `PUT /api/v1/watchlist/{symbol}` | `addToWatchlist` | Updated list (201 added / 200 already present); `404 unknown_symbol`, `400 invalid_symbol` |
| `DELETE /api/v1/watchlist/{symbol}` | `removeFromWatchlist` | Updated list (idempotent) |
| `GET /api/v1/symbols?q=` | `searchSymbols` | `{query, results: [{symbol, company_name, exchange, is_eligible}]}` (≤20); `400 invalid_query` for blank or >40 chars |

All responses go through strict parsers (`src/api/watchlist/parsers.ts`). A shape
mismatch (including an `as_of` that isn't `YYYY-MM-DD` or an unparseable
`added_at`) surfaces as `invalid_response` rather than rendering wrong data.
`change_pct` is a raw ratio (0.12 = +12%). Errors are `ApiError {status, code}`,
with the same codes and handling as `mfe-scanner` (`src/common/errors/errorMessages.ts`
holds the copy).

Hooks:

- `useWatchlist()` → `{items, owner, isLoading, error, saving, isWatched, add, remove, reload}`.
  `add(symbol, seed?)` and `remove(symbol)` update the list optimistically and resolve to
  the server's updated list, or to `null` on failure. On failure only that symbol
  is rolled back and `error` is set. Only the latest save's response is adopted,
  and a load that returns after a save is ignored. An optimistic row has no
  market data yet (`is_stale: true`).
- `useSymbolSearch(query, {debounceMs = 250})` → `{query, results, isLoading, error, retry}`.
  A blank query is idle and never hits the API, and a superseded request is
  aborted. The server stays the authority on query length.

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3002` standalone. Both must be listed in
`MOMENTUM_API_CORS_ORIGINS` for momentum-api (repo-root `.env`):

```
MOMENTUM_API_CORS_ORIGINS=http://localhost:3000,http://localhost:3001,http://localhost:3002
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
| `make run` | Dev server on :3002 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3002 (standalone)
make docker                                # trading-agent-mfe-watchlist:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-watchlist && make run    # :3002
# 3. the shell
cd web-app/spog && npm run start-dev:all   # :3000 → http://localhost:3000/watchlist
```

spog loads the remote from `mfes.mfe_watchlist` in `web-app/spog/public/config.json`.

## Layout

```
src/
  api/            fetch-client (GET/PUT/DELETE, ApiError {status, code}), watchlist/{types, parsers, watchlistApi}
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper (ThemeProvider + host mode)
  common/         error copy, webpack MF helper
  components/     PageLayout (pill when standalone), ApiErrorState
  config/         api.config.ts
  hooks/          useApiResource, watchlist/{useWatchlist, useSymbolSearch}
  pages/          WatchlistPage (placeholder)
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  types/          MF remote declarations, constants
```
