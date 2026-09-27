# mfe-alarm-history

Alarm History remote micro frontend for trading-agent: the alerts the analyst
bot posted, newest first, grouped by symbol and type. Listed in the sidebar's
**Tracking** group at `/alarm-history`.

Backed by `momentum-api` `GET /api/v1/alerts` (read-only over `fired_alerts`;
see `services/data-analyzer/internal/momentumapi/alerts.go`). Page behaviour —
views, filters, local-day bounds, paging and the 60 s refresh — is described in
`src/features/AlarmHistory/README.md`.

| | |
|---|---|
| Module Federation name / scope | `mfe_alarm_history` |
| Exposed module | `./AlarmHistory` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3007 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-watchlist`. UI primitives and theme tokens come
from `@trading-agent/shared-components`, compiled from source (webpack alias to
`src/mfe.ts`), so only this package's React copy is bundled. No third-party UI
library is used.

Hosted vs standalone: `app-root` (hosted) follows the shell's theme through the
user-data message bus and wraps the app in `HostModeProvider hosted`, so the page
omits the "Screener — not a forecast" pill, which spog's header already shows.
`bootstrap` (standalone) uses its own router and the OS theme.
`.alarm-history-page--hosted` is the only scroll container inside spog's grid cell.

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3007` standalone. Both must be listed in
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
| `make run` | Dev server on :3007 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3007 (standalone)
make docker                                # trading-agent-mfe-alarm-history:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-alarm-history && make run   # :3007
# 3. the shell
cd web-app/spog && npm run start-dev:all      # :3000 → http://localhost:3000/alarm-history
```

spog loads `./AlarmHistory` from `mfes.mfe_alarm_history` in
`web-app/spog/public/config.json`. Standalone, `AppRouter` serves the page at `/`.
A running dev server must be restarted after `exposes` in `webpack.config.js` changes.

## Layout

```
src/
  api/            fetch-client (GET/PUT/DELETE, ApiError {status, code})
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper
  common/         error copy, webpack MF helper
  components/     PageLayout, ApiErrorState
  config/         api.config.ts
  pages/          AlarmHistoryPage
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  types/          MF remote declarations, constants
```
