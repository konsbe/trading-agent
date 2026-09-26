# mfe-education

Education remote micro frontend for trading-agent: the **Handbook** (how each part
of the app works), the **MasterClass** (standards-based trading concepts) and the
**Glossary** (terms extracted from both). Content spec:
`docs/EDUCATION_SECTION_CONTENT_SPEC.md`; content files: `shared/content/`.

One Module Federation container exposes three page modules. spog lists them as three
sidebar items in an "Education" group, from three `config.json` entries that share the
one remote.

## Pages

| Route | Page | What it shows |
|---|---|---|
| `/handbook` | `HandbookPage` | Narrative sections → entries with a jump-nav. Each section's `intro` renders first under its title. Caveat blocks render the served `text` verbatim in the same callout as mfe-scanner's research-score / heuristic caveats, always expanded. |
| `/masterclass` | `MasterClassPage` | Modules → entries with a jump-nav. Each entry shows its `summary` ("In short") first, always visible; the full explanation (`blocks`) sits below it in a `CollapsibleCard` (state remembered per entry, `education.masterclass.<entry-id>`). |
| `/glossary` | `GlossaryPage` | Search-first flat list, alphabetised. The search matches term and synonyms, case-insensitively. Each row shows the term, its synonyms, the one-line definition, and a link to `/<handbook\|masterclass>#<entry_id>`. |

- **Deep links:** `/handbook#<entry-id>` and `/masterclass#<entry-id>` (section / module ids work too) scroll to and focus the target once the content has loaded (`useHashScroll`). Jump-nav links are real `#<id>` anchors on the current route: they work from the keyboard, and the current one is marked `aria-current="location"`. The nav is sticky beside the content on wide screens and sits at the top of the page below 900px.
- **Blocks** (`ContentBlocks`): `paragraph`, `heading`, `list`, and (Handbook only) `caveat`. Text goes through `InlineText`, a small parser that supports `**bold**` and `*italic*` (italic may nest in bold). Markers must hug their text, so `3 * 4` stays literal. Everything else, HTML-looking text included, renders as literal React text; there is no `dangerouslySetInnerHTML`. Caveat text is not parsed at all.
- **States:** a skeleton while loading. On failure, `ApiErrorState` shows the error with Retry (sibling pattern). The Glossary's empty state uses `MFEDataWrapper` with the text "Glossary terms are added as Handbook and MasterClass sections are written"; that is the expected state today, because every `terms` array is empty. Searching with no matches shows "No terms match “…”.". All three pages use neutral colours only.
- Handbook sections are deliberately **not** collapsible, so a caveat can never be hidden. In MasterClass, only the full explanation collapses.

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

The types in `src/api/education/types.ts` match the live responses
(`services/data-analyzer/internal/momentumapi/education.go`; `docs/MOMENTUM_SCANNER_API.md`,
"Education content"). The parsers in `src/api/education/parsers.ts` are strict: unknown block
types, a caveat block without its resolved `text`, a caveat in MasterClass, or a
missing `summary` all fail with the JSON path. `getJson` surfaces any such failure as
`invalid_response`. Optional fields (`spec_ref`, `intro`, `notes`, `number`, term
`synonyms`) may be absent.

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
  api/            fetch-client (GET, ApiError {status, code}), education/{types, parsers, educationApi}
  app/            handbook-root, masterclass-root, glossary-root (exposed, hosted),
                  bootstrap (standalone), wrapper (ThemeProvider + host mode)
  common/         webpack MF helper, errors/errorMessages
  components/     PageLayout (pill when standalone), DocLayout (jump-nav + content),
                  ContentBlocks, InlineText (bold/italic parser), CaveatCallout,
                  ContentStatus, DocSkeleton, ApiErrorState
  config/         api.config.ts
  features/       Handbook (HandbookSection), MasterClass (MasterClassModule, MasterClassEntry),
                  Glossary (GlossarySearch, GlossaryList, utils: sort / filter / entry link)
  hooks/          useApiResource, education (useHandbook / useMasterClass / useGlossary), useHashScroll
  pages/          HandbookPage, MasterClassPage, GlossaryPage
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (standalone routes)
  styles/         education-document.css (section / module cards, entry rows)
  test-utils/     mockResponse, response fixtures, mockPage
  types/          MF remote declarations, constants
```
