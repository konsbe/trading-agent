# SPOG – Trading Agent shell

Host (shell) micro frontend for the trading-agent web app. It owns the app
chrome (header, sidebar, auth dialogs), routing, Keycloak auth, the shared Redux
store and the message bus, and loads remote MFEs at runtime from `config.json`
via webpack Module Federation (`shell_spog@http://localhost:3000/remoteEntry.js`).

UI primitives and theme tokens come from `@trading-agent/shared-components`
(`../shared-components`). No third-party UI library is used.

## Routes

| Path | Page |
|------|------|
| `/` | redirects to `/candidates` |
| `/candidates` | Candidates (placeholder) |
| `/stock-detail` | Stock Detail (placeholder) |
| `/backtest-lab` | Backtest Lab (placeholder) |
| `/alarm-history` | Alarm History (placeholder) |
| `/watchlist` | Watchlist (placeholder) |
| `/tracked-positions` | Tracked Positions (placeholder) |
| `/data-source` | Data Source (placeholder) |
| `/settings` | Settings (placeholder) |
| `/404`, `/unauthorized` | error pages |

Routes are declared in `src/router/AppRouter.tsx`; sidebar entries come from
`src/constants/routes.ts`.

## Prerequisites

- Node.js 22 (`nvm use 22`)
- Public npm registry (`https://registry.npmjs.org/`)

## Install

```bash
nvm use 22
npm install --legacy-peer-deps
```

## Run

```bash
npm run start-dev:all   # dev server on :3000, Keycloak bypassed
npm run start-dev       # dev server with Keycloak login
npm run start-app       # production-mode dev server
```

Runtime configuration lives in `public/config.json` (`shell_spog.config` holds the
banner name and the `keycloakUrl` / `keycloakRealm` / `keycloakClientId` settings;
`mfes` lists remote MFEs to load).

## Build & test

```bash
npm run build           # dist/
npm test
npm run test:coverage
```

## Docker

The image is built from the `web-app/` directory because the shell depends on
`file:../shared-components`:

```bash
make docker             # docker build -f Dockerfile -t trading-agent-spog:latest ..
docker run -p 3000:80 trading-agent-spog:latest
```

Mount your own `config.json` at `/usr/share/nginx/html/config.json` (set
`mainJsUrl` to `/bundle.js` or the deployed URL).

## Make targets

`build`, `test`, `coverage`, `run`, `dev`, `clean`, `docker`
