# mfe-education

Education remote micro frontend for trading-agent: the **Handbook** (how each part
of the app works), the **MasterClass** (standards-based trading concepts) and the
**Glossary** (terms extracted from both). Content spec:
`docs/EDUCATION_SECTION_CONTENT_SPEC.md`; content files: `shared/content/`.

One Module Federation container exposes three page modules. spog lists them as three
sidebar items in an "Education" group, from three `config.json` entries that share the
one remote.

**Status: scaffold.** The three pages are placeholders, and the API client is a
typed stub for endpoints that momentum-api does not serve yet.

| | |
|---|---|
| Module Federation name / scope | `mfe_education` |
| Exposed modules | `./Handbook` → `src/app/handbook-root.tsx`, `./MasterClass` → `src/app/masterclass-root.tsx`, `./Glossary` → `src/app/glossary-root.tsx` |
| spog config keys | `mfe_education_handbook` (`/handbook`), `mfe_education_masterclass` (`/masterclass`), `mfe_education_glossary` (`/glossary`) |
| Dev server | http://localhost:3006 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-market-report` / `mfe-backtest-lab`. UI primitives
and theme tokens come from `@trading-agent/shared-components`, compiled from source
(webpack alias to `src/mfe.ts`), so only this package's React copy is bundled.

Hosted vs standalone: each exposed root follows the shell's theme through the
user-data message bus and wraps its page in `HostModeProvider hosted`, so pages omit
the title and the "Screener — not a forecast" pill, which spog's header already shows.
`bootstrap` (standalone) uses its own router (`/handbook`, `/masterclass`, `/glossary`;
`/` opens the Handbook) and the OS theme.

## momentum-api endpoints

| Endpoint | Function |
|---|---|
| `GET /api/v1/education/handbook` | `fetchHandbook` |
| `GET /api/v1/education/masterclass` | `fetchMasterClass` |
| `GET /api/v1/education/glossary` | `fetchGlossary` |

Types in `src/api/education/types.ts` are provisional, modelled on
`shared/content/handbook.json` / `masterclass.json`. The parsers only check for a JSON
object with the top-level array (`sections`, `modules`, `terms`); a mismatch surfaces
as `invalid_response`. Tighten both once the Go handler exists.

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3006` standalone. Both must be in `MOMENTUM_API_CORS_ORIGINS`
for momentum-api (repo-root `.env`); `:3006` is already listed.

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
| `make run` | Dev server on :3006 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3006 (standalone)
make docker                                # trading-agent-mfe-education:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. this MFE
cd web-app/ui/mfe-education && make run   # :3006
# 2. the shell
cd web-app/spog && npm run start-dev:all  # :3000 → http://localhost:3000/handbook
```

## Layout

```
src/
  api/            fetch-client (GET, ApiError {status, code}), education/{types, educationApi}
  app/            handbook-root, masterclass-root, glossary-root (exposed, hosted),
                  bootstrap (standalone), wrapper (ThemeProvider + host mode)
  common/         webpack MF helper
  components/     PageLayout (pill when standalone)
  config/         api.config.ts
  pages/          HandbookPage, MasterClassPage, GlossaryPage (placeholders)
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (standalone routes)
  test-utils/     mockResponse
  types/          MF remote declarations, constants
```
