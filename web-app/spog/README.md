# SPOG – Trading Agent shell

Host (shell) micro frontend for the trading-agent web app. It owns the app
chrome (header, sidebar, auth dialogs), routing, Keycloak auth, the shared Redux
store and the message bus, and loads remote MFEs at runtime from `config.json`
via webpack Module Federation (`shell_spog@http://localhost:3000/remoteEntry.js`).

UI primitives and theme tokens come from `@trading-agent/shared-components`
(`../shared-components`). No third-party UI library is used.

## Routes and sidebar

Routes and sidebar entries are generated from `public/config.json`
(`src/common/navigation`). Every enabled entry under `mfes` with a
`router_path` becomes a route (`<router_path>/*` → the MFE's `module`) and a
sidebar link:

| Field | Meaning |
|-------|---------|
| `router_path` | Route URL, e.g. `/candidates` |
| `nav_group` | Sidebar group label (default `Other`) |
| `nav_order` | Group order, ascending; `0` pins the group to the sidebar bottom; missing = after numbered groups |
| `nav_sub_order` | Item order inside the group, ascending (ties by label) |
| `nav_icon` | Icon name, see `src/components/Sidebar/navIcons.ts`; unknown names show a fallback dot |

Adding a page, group or item only needs a `config.json` edit. `/` redirects to
the first item of the first (non-bottom) group.

Not-yet-implemented pages (`/stock-detail`, `/alarm-history`,
`/tracked-positions`, `/settings`) are text placeholders defined in
`src/common/navigation/navigation.ts`, listed last under "Coming Soon". A
placeholder disappears as soon as a config MFE claims its path. `/404` and
`/unauthorized` are error pages.

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
