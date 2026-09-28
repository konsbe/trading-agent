# mfe-tracked

Tracked Positions remote micro frontend for trading-agent: scanner alerts
followed through the exit rules, as research instrumentation (not a portfolio).
Listed in the sidebar's **Tracking** group at `/tracked-positions`.

Backed by `momentum-api` `GET /api/v1/scanner/tracked?status=active|closed|all`
(read-only over `momentum_tracked`; see
`services/data-analyzer/internal/momentumapi/tracked.go`). The page reads
`status=all` once and splits it into **Active** / **Closed** tabs (counts from
`summary`), shows the chain freshness banner from `chain`, and re-reads when its
browser tab becomes visible again. See `src/features/TrackedPositions/README.md`.

| | |
|---|---|
| Module Federation name / scope | `mfe_tracked` |
| Exposed module | `./TrackedPositions` → `src/app/app-root.tsx` |
| Dev server | http://localhost:3009 (`remoteEntry.js` at `/remoteEntry.js`) |
| Shell remote | `shellSpog` → `shell_spog@http://localhost:3000/remoteEntry.js` (dev); resolved from `window.__APP_CONFIG__.shell_spog` in production |
| API | `momentum-api`, default http://localhost:8090 |

Same stack and conventions as `mfe-alarm-history`. UI primitives and theme tokens
come from `@trading-agent/shared-components`, compiled from source (webpack alias
to `src/mfe.ts`), so only this package's React copy is bundled. No third-party UI
library is used.

Hosted vs standalone: `app-root` (hosted) follows the shell's theme through the
user-data message bus and wraps the app in `HostModeProvider hosted`, so the page
omits the "Screener — not a forecast" pill, which spog's header already shows.
`bootstrap` (standalone) uses its own router and the OS theme.
`.tracked-page--hosted` is the only scroll container inside spog's grid cell.

## Configuration

API base URL (`src/config/api.config.ts`), resolved per request:

1. Hosted: `window.__APP_CONFIG__.shell_spog.config.momentumApiUrl` (spog's `public/config.json`)
2. Otherwise: build-time `MOMENTUM_API_URL` env var, default `http://localhost:8090`

Browser requests come from the page origin: `http://localhost:3000` when hosted,
`http://localhost:3009` standalone. Both must be listed in
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
| `make run` | Dev server on :3009 |
| `make docker` | Build the nginx image (context `web-app/`) |

```bash
nvm use 22
make build
make test
make run                                   # http://localhost:3009 (standalone)
make docker                                # trading-agent-mfe-tracked:latest
MOMENTUM_API_URL=http://api:8090 make build
```

## Run hosted in spog

```bash
# 1. momentum-api on :8090 (see services/data-analyzer/cmd/momentum-api/README.md)
# 2. this MFE
cd web-app/ui/mfe-tracked && make run         # :3009
# 3. the shell
cd web-app/spog && npm run start-dev:all      # :3000 → http://localhost:3000/tracked-positions
```

spog loads `./TrackedPositions` from `mfes.mfe_tracked` in
`web-app/spog/public/config.json`. Standalone, `AppRouter` serves the page at `/`.
A running dev server must be restarted after `exposes` in `webpack.config.js` changes.

## Layout

```
src/
  api/            fetch-client (GET/PUT/DELETE, ApiError {status, code}), strict readers,
                  tracked/ (types, parsers, fetchTracked)
  app/            app-root (exposed, hosted), bootstrap (standalone), wrapper
  common/         error copy, date formatter, webpack MF helper
  components/     PageLayout, ApiErrorState
  config/         api.config.ts (base URL, TRACKED_ENDPOINTS), routes.ts (Stock Detail link)
  features/       TrackedPositions (screen, banner, tabs, tables, hooks, utils)
  hooks/          useApiResource, useRefreshOnVisible
  pages/          TrackedPositionsPage
  styles/         tracked-table.css (shared table look)
  providers/      HostModeContext (hosted vs standalone)
  router/         AppRouter (relative routes)
  types/          MF remote declarations, constants
```
UI brief -- Tracked Positions (mfe-tracked), per Tracked Positions
addendum §6.1, with today's decisions folded in.

This is research instrumentation, not a portfolio or trading dashboard
-- the system has never executed a trade. No "P&L", "your holdings" or
portfolio-app framing; no live/streaming language; no "performance" or
"returns" framing beyond the literal percentages.

PLACEMENT
- Route /tracked-positions, 5th item in the Tracking sidebar group.
  Delete the Coming Soon placeholder.
- Port 3009; add it to MOMENTUM_API_CORS_ORIGINS in .env and
  .env.example; recreate momentum-api.

HEADER
- Standard app header, unchanged (permanent "Screener — not a forecast"
  badge, theme toggle, bell, avatar).
- Title "Tracked Positions".
- Two tabs: Active (default) and Closed, with counts from summary,
  e.g. "Active (20)", "Closed (16)".

SYSTEM-LEVEL FRESHNESS BANNER (above the tabs, never per row)
From the new `chain` object on GET /api/v1/scanner/tracked:
expected_session, last_scan_date, last_tracked_session,
sessions_behind, tracker_behind.
- sessions_behind >= 1: "No new scan for N trading sessions — the
  latest is {last_scan_date}. Tracked figures below are as of that
  date."
- Otherwise, tracker_behind: "The {last_scan_date} scan exists but
  tracking has not been updated since {last_tracked_session}."
- "No new scan" takes precedence.
- tracker_behind is true only when that session's momentum_chain_runs
  row has scanner_completed_at more than 15 minutes ago and no
  tracker_completed_at, or has gave_up_at set (then immediately).
- Counted in NYSE trading sessions, never calendar days.
- Neutral informational styling: no error colour, no alarm icon. No
  data is wrong, it is old.
- The UI never derives staleness from row dates.

ACTIVE TAB
Columns: Symbol / Exchange / Bucket, Alerted Date, Sessions Elapsed,
Reference Price, Current Price, Unrealized %.
- Unrealized % is the only red/green (--color-price-up /
  --color-price-down). Everything else neutral, same weight, no
  urgency icons.
- Never-evaluated rows (last_evaluated_date null; the API then also
  serves sessions_elapsed and unrealized_pct as null): Sessions Elapsed
  reads "Not yet evaluated", Unrealized % reads "—" (never 0 or
  0.00%), with a neutral note that the next tracker run evaluates them.
- evaluation_behind true on a row: a neutral note that sessions elapsed
  is as of last_evaluated_date. When the banner is showing, keep these
  notes without extra emphasis.
- Null current price (symbol stopped scanning): "—", row kept.

CLOSED TAB
Columns: Symbol / Exchange / Bucket, Alerted Date, Closed Date,
Sessions Elapsed, Reference Price, Exit Reason, Exit %.
- Closed Date is closed_date, formatted like Alerted Date ("—" if
  null). Exit price is not shown.
- Exit Reason as plain text (e.g. breakout_failed), not a coloured
  badge.
- Exit % red/green, as above.
- Clicking Exit Reason expands an inline panel with exit_reason_note.
  Never a tooltip, never collapsed away by default once opened. Each
  note is rule-specific; lost_vwap and momentum_stalled are plain
  definitions with no caveat added.

EMPTY STATES (existing empty-state component)
- Active, zero rows: "Nothing currently tracked".
- Closed, zero rows: "No closed positions yet".

GENERAL
- "—" for every missing value, never 0.
- Dark and light themes from the existing token set.
- Refresh on tab focus; no push or WebSockets.
- Handbook entry for the page: what it tracks, active vs closed,
  never-evaluated rows, the banner, and each exit reason with its note.

TESTS
- Never-evaluated row renders "Not yet evaluated" and "—".
- Banner: shows for sessions_behind >= 1; shows for tracker_behind;
  "No new scan" wins when both; hidden when neither.
- Unrealized % null on closed rows, exit fields null on active rows.
- exit_reason_note present for every non-null exit_reason.

CONSTRAINTS
- Touch only momentum-api and the new MFE. No changes to momentum-daily,
  momentum-scanner or momentum-tracker (frozen until the gate clears).
- Verify live inside spog in both themes with real rows, including the
  never-evaluated rows and the closed notes.
- Commit in logical pieces with explicit paths. Don't push.
- Finish before 23:00 Greek time. Not signed off until the
  clean-session gate clears (Thursday 1 Oct at the earliest).