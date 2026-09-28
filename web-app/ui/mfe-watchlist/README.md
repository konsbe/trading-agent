# mfe-watchlist

Watchlist remote micro frontend for trading-agent. One container, three screens,
all under the sidebar's **Tracking** group:

- **Watchlist** (`/watchlist`): the shared watchlist with each symbol's current
  price data, symbol search for adding, and a Compute button per row.
- **Followed Symbols** (`/followed-symbols`): what the pipeline fetches and
  computes every day, with two separate searches (scanner universe, and all
  symbols incl. ETFs / ADRs / OTC / crypto), Follow, Compute and Unfollow.
- **Computed Symbols** (`/computed-symbols`): every symbol with an open
  computation reason, its state, and Stop computing for manual requests.

Backed by `momentum-api` (`docs/MOMENTUM_SCANNER_API.md` §2.5 and §2.5a).

Every table sorts from its headers and has a search box (shared `useTableView`,
`SortableHeader`, `TableSearch`; URL keys `watchlist_*`, `followed_*`,
`computed_*`). Defaults: Watchlist and Followed newest added first, Computed
most recently computed first ("not yet" last). Compute / State columns sort by
`COMPUTE_STATE_ORDER` (attention first). Hosted, stock and fund tickers open
Stock Detail through the shared `StockDetailLink` with "Back to Watchlist /
Followed Symbols / Computed Symbols"; crypto pairs are plain text.

| | |
|---|---|
| Module Federation name / scope | `mfe_watchlist` |
| Exposed modules | `./Watchlist` → `src/app/app-root.tsx`, `./FollowedSymbols` → `src/app/followed-root.tsx`, `./ComputedSymbols` → `src/app/computed-root.tsx` |
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
| `GET /api/v1/watchlist` | `fetchWatchlist` | `{owner, items: [{symbol, company_name, exchange, added_at, as_of, is_stale, close, change_pct, rvol_20, volume, data_source, sources, market_cap_note, …}]}`, newest first. `data_source` `daily_bars` = outside the scanner's universe, computed from the symbol's own daily bars (marked "from daily bars", never scored; a non-USD market cap stays null with its note) |
| `PUT /api/v1/watchlist/{symbol}` | `addToWatchlist` | Updated list (201 added / 200 already present); `404 unknown_symbol`, `400 invalid_symbol`. Also queues the symbol's data fetch (its compute state is re-read after the add) |
| `DELETE /api/v1/watchlist/{symbol}` | `removeFromWatchlist` | Updated list (idempotent) |
| `GET /api/v1/symbols?q=` | `searchSymbols` | `{query, results: [{symbol, company_name, exchange, is_eligible}]}` (≤20); `400 invalid_query` for blank or >40 chars |
| `GET /api/v1/followed-symbols` | `fetchFollowedSymbols` | `{items: [{symbol, name, asset_type, listing, news_alias, source, added_at}]}`, newest first |
| `PUT /api/v1/followed-symbols/{symbol}` | `followSymbol` | Updated list (201 / 200); `404 unknown_symbol`, `422 not_computable`, `400 invalid_symbol` |
| `DELETE /api/v1/followed-symbols/{symbol}` | `unfollowSymbol` | Updated list |
| `GET /api/v1/symbols/directory?q=` | `searchDirectory` | `{query, results: [{symbol, name, type, mic, asset_type, source, in_universe, followed}]}` (≤20); `400 invalid_query` |
| `GET /api/v1/computed-symbols` | `fetchComputedSymbols` | `{data_timeout_minutes, items: [{symbol, name, asset_type, reasons, manual_requested_at, queued_at, state, …, computed_at, last_error}]}`. `queued_at` = newest open manual or watchlist reason |
| `PUT /api/v1/computed-symbols/{symbol}` | `requestCompute` | 202 with the computed-symbols body; same 404 / 422 / 400 as follow. Idempotent: an open request is kept, not restarted |
| `DELETE /api/v1/computed-symbols/{symbol}` | `stopCompute` | Closes only the manual reason; 200 with the computed-symbols body |

All responses go through strict parsers (`src/api/watchlist/parsers.ts`,
`src/api/tracking/parsers.ts`, shared readers in `src/api/parse.ts`). A shape
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
  aborted. The server stays the authority on query length. `useDirectorySearch`
  is the same over `/symbols/directory`; both are `useQuerySearch` underneath.
- `useFollowedSymbols({onChange})` → `{items, isLoading, loadError, isFollowed, saving, errors, follow, unfollow, reload}`.
  Follow / unfollow wait for the server (a follow can be refused) and keep a
  failure per symbol with its action.
- `ComputeStatusProvider` / `useComputeStatus()` — one computed-symbols list per
  screen: `{items, dataTimeoutMinutes, isPolling, getItem, requesting, errors, compute, stop, refresh}`.
  It polls every 12 s only while a queued fetch (a Compute press or a watchlist
  addition) is `waiting_for_data` or `computing`, and never after unmount. The
  Watchlist screen reloads its list when one of its rows goes from waiting /
  computing to computed (`useReloadOnComputed`, no extra timer). `ComputeButton` (Watchlist rows,
  Followed rows, both search lists) and the Computed Symbols table read it;
  `ComputeState` renders a state in words (queued, computing, computed,
  data not arrived with the queue time and timeout, failed with its error);
  the time reads "requested" for a Compute press and "queued … (added to
  watchlist)" for a watchlist addition.

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

spog loads the three modules from `mfes.mfe_watchlist`, `mfes.mfe_watchlist_followed`
and `mfes.mfe_watchlist_computed` in `web-app/spog/public/config.json` (one scope,
`mfe_watchlist`). Standalone, `AppRouter` serves the same screens at `/`,
`/followed-symbols` and `/computed-symbols`. A running dev server must be restarted
after `exposes` in `webpack.config.js` changes.

## Layout

```
src/
  api/            fetch-client (GET/PUT/DELETE, ApiError {status, code}), parse, watchlist/…, tracking/{types, parsers, trackingApi}
  app/            app-root / followed-root / computed-root (exposed, hosted), bootstrap (standalone), wrapper
  common/         error copy, formatters and tracking labels, compute state order / Compute column, sort summary, webpack MF helper
  components/     PageLayout, ApiErrorState, ComputeButton, ComputeState, SymbolLink (Stock Detail link for stocks / funds), Tag
  config/         api.config.ts
  features/       Watchlist/…, FollowedSymbols/{SearchPanel, UniverseSearch, DirectorySearch, FollowedTable, …}, ComputedSymbols/ComputedTable
  hooks/          useApiResource, useQuerySearch, watchlist/{useWatchlist, useSymbolSearch}, tracking/{useFollowedSymbols, useDirectorySearch}
  pages/          WatchlistPage, FollowedSymbolsPage, ComputedSymbolsPage
  providers/      HostModeContext (hosted vs standalone), ComputeStatusContext, StockDetailOrigin ("Back to …" label per screen)
  styles/         tracking-table.css (Followed / Computed tables)
  router/         AppRouter (relative routes)
  types/          MF remote declarations, constants
```
