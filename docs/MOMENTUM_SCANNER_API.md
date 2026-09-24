# Momentum Scanner — API Service Spec

**Target repo:** `trading-agent`
**Status:** steps 1–6 built (2026-09-23) as `services/data-analyzer/cmd/momentum-api`.
Step 7 is blocked — see §7. Where this spec's original assumptions turned out
wrong against the live schema, the text below has been corrected and §7 records
what was verified.

> **⚠️ AUTH IS A HARD BLOCKER.** v1 ships with no authentication, bound to
> loopback / a trusted network only (§3). It must not be exposed beyond that
> until real auth exists.
**Serves:** `web-app/ui/mfe-scanner`'s "Today's Candidates" screen — two
endpoints, read-only, no scoring or gate logic of its own.

This document assumes the reader has the repo but not the conversation that
produced it. Read `docs/MOMENTUM_SCANNER_PHASE1.md` §7 (schema), §4 (scoring),
and §8.3 (embed field reference) before implementing — this spec's field list
is derived from those, and column names must be verified against
`shared/schemas/SCHEMAS.md` and the live schema, not assumed from memory.

---

## 0. What this service is, and is not

**Is:** a thin, read-only HTTP layer over `momentum_scores`, `momentum_features`,
and `universe_symbols` — the same tables `services/analyst-bot/db/queries/momentum.py`
already reads for the Discord bot. It computes nothing. It does not call the
gate logic, the scorer, or the feature engine. Those already ran, once, when
`momentum-scanner` did its daily pass; this service only serves what's already
stored.

**Is not:** a second scoring path, a write endpoint, a place for business logic
that doesn't already exist elsewhere, or a general-purpose data API. If the
frontend later needs something these endpoints don't provide, that's a new,
separately-designed endpoint — not a reason to widen this one's scope quietly.

**On `momentum_score_100`: it IS exposed, deliberately, not by omission.**
An earlier draft of this spec excluded the score entirely, mirroring the
Discord alert embed. That's been overridden — the score is shown, matching how
`/score` already handles it in Discord: visible, but never presented as
validated.

```
list view (GET /today)        -> score shown per row, de-emphasized,
                                  carries score_status = "unvalidated"
detail view (GET /today/{sym}) -> full breakdown: total, every sub-score,
                                  penalties, the odds-ratio evidence text
```

Every place the score appears, its unvalidated status travels with it in the
same payload — never a field the frontend has to remember to caveat itself.
See §2.2 and §2.4.

**Every column in the list view is sortable, the score column included.**
This is a deliberate design choice with a real tension worth stating plainly:
Phase 1's screener redesign fixed the *default* sort to RVOL specifically so
the list wouldn't read as a leaderboard. Making every column — including the
unvalidated score — interactively sortable partially reopens that. The
resolution isn't to refuse the feature; it's that the `score_status` marker
travels with the score into every sort order, so sorting by score still shows
"unvalidated" next to every value, in every position. The list can be
reordered by the user; it is never silently presented as pre-ranked. Default
sort on load is still `rvol_20 DESC`.

Because sorting is user-driven and can land on any column, **this endpoint
returns the full candidate set per bucket, not a server-side top-10 slice.**
Windowing ("show top 10, then 'and X more'") is a frontend display default
applied to the default RVOL-sorted view, not a property of the data — the
API can't decide the useful top-N once the user might re-sort by a different
column entirely. See §2.3.

---

## 1. Where it lives, and why

**New Go binary: `services/data-analyzer/cmd/momentum-api/main.go`.**

Reasons, not defaults:

- `data-analyzer` already owns the `momentum` package (features, gates, labels,
  score) and already has three CLI binaries reading these tables
  (`momentum-scanner`, `momentum-backtest`, `momentum-tracker`). A fourth `cmd/`
  binary that only *reads* is a smaller, more consistent addition than
  introducing Python/FastAPI into a repo whose Go side already owns this
  schema.
- No new language footprint for two small JSON endpoints.
- Follows the existing worker pattern from `data_ingestion.md`: own `cmd/`
  binary, env-var config, own Compose service, Dockerfile at the package root.

**This is a server, not a one-shot job** — the one deliberate departure from
the CLI-tool pattern the other `cmd/` binaries follow. It stays running,
listening on a port, rather than running once and exiting.

---

## 2. Endpoints

### 2.1 `GET /api/v1/scanner/today`

No query parameters in v1. Always returns the most recent complete scan, both
buckets, full candidate lists. (A `?date=` parameter for historical browsing
is a plausible future need — out of scope here; don't build it speculatively.)

#### Response shape

```json
{
  "scan": {
    "date": "2026-09-23",
    "completed_at": "2026-09-23T21:04:12Z",
    "universe_scanned": 4971,
    "universe_eligible": 4975,
    "is_stale": false
  },
  "buckets": {
    "market": {
      "total_candidates": 14,
      "candidates": [ /* see 2.2, full list, default order rvol_20 DESC */ ]
    },
    "penny": {
      "total_candidates": 3,
      "candidates": [ /* see 2.2 */ ]
    }
  }
}
```

- **`scan.date`** — the trading day the scan covers, `YYYY-MM-DD`.
- **`scan.completed_at`** — *verified (§7)*: the later of
  `max(momentum_features.computed_at)` and `max(momentum_scores.scored_at)` for
  the scan date. Both columns exist and are reset to `now()` on every upsert, so
  no migration was needed. Not faked from `MAX(ts)`. Caveat: no run-completion
  record exists, so this is the time of the scan's *last write* — a run that
  crashed midway still reports one.
- **`scan.universe_scanned`** — `COUNT(DISTINCT symbol)` from
  `momentum_features` where `ts = scan.date`.
- **`scan.universe_eligible`** — `COUNT(*)` from `universe_symbols` where
  `is_eligible`.
- **`scan.is_stale`** — `true` if `scan.date` is not the most recent completed
  trading session (e.g. the daily refresh hasn't run yet, or failed). This is
  what the frontend uses to decide between the normal header and the
  "Couldn't load today's results — showing yesterday's scan" error-state
  copy from the mfe-scanner design. Compute it by comparing `scan.date`
  against the expected most recent session: the latest NYSE trading day whose
  16:00 New York close plus a grace period (`MOMENTUM_API_SCAN_GRACE`, default
  6h) has passed. *Corrected (§7)*: `internal/additional/calendar.go` is a
  seasonality table, not a session calendar, and no holiday calendar existed.
  Deriving sessions from ingested bars was rejected as circular (it cannot see
  a day whose data never arrived), so NYSE holidays are a hard-coded, tested
  list in `internal/momentumapi/calendar.go`.

### 2.2 Candidate object (list view)

```json
{
  "symbol": "NEXR",
  "exchange": "NASDAQ",
  "company_name": "Nexien Inc",
  "bucket": "penny",
  "close": 1.64,
  "change_pct": 15.5,
  "rvol_20": 6.74,
  "dollar_volume": 4553306,
  "rsi_14": 36.0,
  "breakout_state": "none",
  "pct_of_52w_high": 0.0019,
  "catalyst_tier": null,
  "momentum_score_100": 68,
  "score_attainable": 75,
  "score_status": "unvalidated"
}
```

Field-by-field, with the schema question flagged where one exists:

| Field | Source | Note |
|---|---|---|
| `symbol`, `exchange` | `universe_symbols` | join on symbol |
| `company_name` | `universe_symbols.name` | per Phase 1 §7's schema table |
| `bucket` | `momentum_features.bucket` | `"market"` or `"penny"`, never omitted |
| `close` | `momentum_features.close` | *Verified (§7)*: stored on the features row as the price traded on the scan day. Do **not** join `equity_ohlcv` — its `close` is back-adjusted by later dividends/splits and already drifts (KVUE, FRO). |
| `change_pct` | `momentum_features.change_pct` | |
| `rvol_20` | `momentum_features.rvol_20` | default sort key on load (§2.3) |
| `dollar_volume` | `momentum_features.dollar_volume` | |
| `rsi_14` | `momentum_features.rsi_14` | |
| `breakout_state` | `momentum_features.breakout_state` | pass through as-is: `"none"` / `"approaching"` / `"breakout"` / `"breakout_from_consolidation"`. **Plain string, not a display label** — labeling ("Breakout: none") is a frontend concern, per the mfe-scanner design's "plain text, not a badge" requirement. |
| `pct_of_52w_high` | `momentum_features.pct_of_52w_high` | raw ratio (e.g. `0.0019`), frontend formats as a percentage — do not pre-format server-side |
| `catalyst_tier` | `momentum_features.catalyst_tier` | `"A"` / `"B"` / `"none"` / `null`. **`null` is the expected/common case** (§3.11's finding: catalyst is unresolved and often absent) — the frontend must render nothing rather than a placeholder when this is `null`, not treat it as missing data |
| `market_cap` | `momentum_features.market_cap` | reported market cap, `null` when the gate used an estimate or had none. Added to the list 2026-09-24 at the product owner's request |
| `market_cap_est` | `momentum_features.market_cap_est` | §3.9's shares-outstanding × close estimate, set only when the reported value was missing |
| `market_cap_is_proxy` | `momentum_features.market_cap_is_proxy` | `true` when the gate used `market_cap_est`. The frontend must visibly mark an estimate (never render it as if reported) |
| `momentum_score_100` | `momentum_scores.momentum_score_100` | **integer or `null`.** Null when the symbol has no score row yet (e.g. fundamentals pass hasn't populated market cap — see Phase 1 §10.1.0a). The frontend must render "—", never `0`, for a null score — a real `0` and a missing score are different facts. |
| `score_attainable` | derived from `momentum_scores.null_inputs` | **integer or `null`**, `null` exactly when `momentum_score_100` is. The row's own ceiling, same derivation as the detail view's `score.attainable` (90 allocated minus the weights of this row's null inputs; 75 while `catalyst_tier` is null). Added 2026-09-24 so the list shows "53/75", never a bare "53" that reads as "out of 100". Varies per row once catalyst data differs. |
| `score_status` | constant | always the literal string `"unvalidated"` in the current build. Not computed per-row — it's a build-level fact about the whole scoring system (Phase 1 §10.1.0's ruling), not a per-symbol property. Kept as a field rather than hardcoded in the frontend so a future, actually-validated model version has exactly one place to change it (see §2.4's `model_version`). |

**Still excluded from the list view, and why:** sub-scores, penalties,
`null_inputs`, `vol_accel`, `above_vwap`, `atr_pct`, `gate_failures`,
`float_shares_est`. (`market_cap` was here until 2026-09-24; it is now served
with its provenance fields above.) Not because they're hidden on principle —
the score itself no longer is — but because the list view's job is the
at-a-glance screener row, and the full breakdown belongs in the detail view
(§2.4) where there's room to show it with its actual evidence attached rather
than as a bare number with no context.

### 2.3 Query logic (list view)

Per bucket (`market`, `penny`):

```sql
SELECT mf.*, ms.momentum_score_100, u.name, u.exchange
FROM momentum_features mf
JOIN universe_symbols u ON u.symbol = mf.symbol
LEFT JOIN momentum_scores ms
  ON ms.symbol = mf.symbol AND ms.ts = mf.ts
WHERE mf.ts = $scan_date
  AND mf.bucket = $bucket
  AND mf.gates_passed = true
ORDER BY mf.rvol_20 DESC NULLS LAST
```

- **Only `gates_passed = true` rows.** This endpoint serves candidates, not
  the full daily feature table — `momentum_features` has one row per *symbol*
  per day (most of which fail the gates), and none of those belong in this
  response.
- **`LEFT JOIN` on scores, not an inner join.** A gate-passing row can still
  lack a score row (§2.2's null-score case) — don't let a missing score
  silently drop a real candidate from the list.
- **No `LIMIT`.** The full gate-passing set per bucket is returned; the
  frontend applies the "top 10 + and X more" windowing to whichever sort
  order is currently active, client-side. This is safe at the data volumes
  involved — even the worst historical day (2024-11-06, 157 candidates) is a
  small payload, nowhere near needing pagination.
- **`ORDER BY rvol_20 DESC` is the query's default order**, matching the
  list's default display order — but the frontend is free to re-sort the
  already-fetched set by any column without a new request. Don't add
  server-side sort parameters for this; there's no data-volume reason to,
  and it avoids a second thing (API sort params) having to stay in sync with
  the frontend's column list.

### 2.4 `GET /api/v1/scanner/today/{symbol}`

Detail view — the full score breakdown, matching `/score` in Discord.
`{symbol}` is case-insensitive, matched against `universe_symbols.symbol`.

#### Response shape

```json
{
  "symbol": "NEXR",
  "exchange": "NASDAQ",
  "company_name": "Nexien Inc",
  "bucket": "penny",
  "as_of": "2026-09-23",
  "gates_passed": true,
  "gate_failures": [],
  "evidence_note": "<EVIDENCE_CAVEAT, from shared/content/momentum_caveats.json>",
  "score": {
    "total": 68,
    "attainable": 75,
    "allocated": 90,
    "status": "unvalidated",
    "model_version": "v2",
    "sub_scores": {
      "rvol": 32.7,
      "vol_accel": 25.0,
      "catalyst": 0.0,
      "float": 10.0,
      "vwap": 0.0,
      "breakout": 0.0,
      "high52w": 0.0
    },
    "penalties": [],
    "penalty_total": 0,
    "null_inputs": ["catalyst_tier"],
    "caveat": "<RESEARCH_SCORE_CAVEAT, from shared/content/momentum_caveats.json>"
  }
}
```

- **`score.attainable`** — the practical ceiling, so a bare `total: 68` doesn't
  read as "68 out of 100". Derived per row as `allocated` (90) minus the weights
  of the components in `null_inputs`: 75 while `catalyst_tier` is null, as
  Phase 1 §4.4 states, and correct automatically once catalyst data arrives.
- **`score.penalties`** — *corrected (§7)*: a list of penalty reason names, as
  stored (`jsonb` array); the points are in `score.penalty_total`.
- **`score` is `null`** for a symbol that failed its gates (it has no score row).
- **Caveats — two, not one** (*corrected, §7*). The bot has two
  differently-scoped constants: `EVIDENCE_CAVEAT` (screener framing, every
  surface) and `RESEARCH_SCORE_CAVEAT` (wherever the score is shown). Both now
  live in **one** file, `shared/content/momentum_caveats.json`, read by the bot
  and this service; neither holds its own copy. The detail view returns
  `evidence_note` (top level, `EVIDENCE_CAVEAT`) and `score.caveat`
  (`RESEARCH_SCORE_CAVEAT`). This spec's original example text claimed rvol had
  a measured out-of-sample effect (MH OR 1.19–1.215); that was superseded on
  2026-09-22 and is no longer served anywhere.
- **`gate_failures`** included here (unlike the list view) because the
  detail view is exactly the place someone lands after wondering "why isn't
  this symbol showing up on the list" — `/score` already provides this in
  Discord for the same reason.
- If the symbol has no row for the current scan date at all (never computed,
  outside the universe, etc.) — `404`, body `{"error": "no_data_for_symbol"}`.

**Extended 2026-09-24 for the Stitch detail design** (all values as stored — the
API still computes nothing):

- **`facts`** — the stored features row: `close` (as traded), `prior_close`,
  `change_pct`, `change_abs` (close − prior_close), `gap_pct`, `volume`,
  `avg_volume_20`, `dollar_volume`, `rvol_20`, `vol_accel`, `atr_pct`, `rsi_14`,
  `high_52w`, `pct_of_52w_high` (ratio close ÷ 52w high; > 1 is a new high),
  `resistance_20`, `breakout_state`, `was_consolidating`, `vwap_20`, `above_vwap`,
  `vwap_dist_pct`, `float_shares_est` + `float_is_proxy`, `market_cap`,
  `market_cap_est`, `market_cap_is_proxy`, `catalyst_tier`, `catalyst_headline`
  (null until catalyst ingestion writes it), `computed_at`. No bid/ask data
  exists, so the design's "spread" is not served.
- **`gates`** — `{passed_count, total, checks[], unmapped_failures?}`. One check
  per §3.2 gate (`price`, `history`, `change_pct`, `rvol_20`, `dollar_volume`,
  `market_cap`) with `value`, the bucket's `min`/`max` from the scanner's gate
  configuration (null = unbounded), `passed`, and the stored `failures` that
  belong to it. **Pass/fail is read from `gate_failures`, never re-evaluated.**
  `market_cap_null` is provenance (proxy market cap), not a failure; it sets
  `value_is_proxy`. Any stored code no check claims is listed in
  `unmapped_failures` so nothing is hidden.
- **`score.weights`** — each component's maximum under `model_version` (v2:
  rvol 35, vol_accel 25, catalyst 15, float 10, vwap 5, breakout 0, high52w 0),
  so the UI can show "32.5 / 35 pts".
- **`score.penalty_rules`** — every §4.3 penalty `{code, points, applied}`, so a
  rule that did not fire is shown as checked-and-zero rather than absent.

### 2.5 Watchlist — `GET/PUT/DELETE /api/v1/watchlist[/{symbol}]`

The one **write path**, designed separately from the read-only scanner routes
(decision 2026-09-24). Stored in `watchlist_items` (migration 024).

| Route | Result |
|---|---|
| `GET /api/v1/watchlist` | `200 {"owner": "unauthenticated", "items": [{symbol, company_name, exchange, added_at}]}` newest first |
| `PUT /api/v1/watchlist/{symbol}` | Idempotent add. `201` when added, `200` when already present; body is the updated list. `404 unknown_symbol` if not in `universe_symbols`; `400 invalid_symbol` |
| `DELETE /api/v1/watchlist/{symbol}` | Idempotent remove. `200` with the updated list |

- **Owner.** A signed-in user's items are keyed by their identity-provider
  subject; without auth every request uses the shared **unauthenticated** list
  (`owner_sub` NULL). There is no auth yet, so all rows are unauthenticated
  today; `ownerFromRequest` in `internal/momentumapi/watchlist.go` is the single
  place that will read the verified subject.
- Never cached. Same no-auth, loopback-only constraint as the rest of the
  service (§3) — anyone who can reach the port can edit the unauthenticated list.

### 2.6 Price history — `GET /api/v1/scanner/symbols/{symbol}/bars?range=...`

For the detail chart. `range` ∈ `1D`, `5D`, `1M`, `6M`, `1Y`, `ALL` (required).

```json
{"symbol": "VGZ", "range": "5D", "interval": "5Min", "fallback": null, "adjusted": false,
 "bars": [{"time": 1790170200, "open": 2.51, "high": 2.55, "low": 2.5, "close": 2.53, "volume": 120400}]}
```

- `1D` / `5D`: stored **5-minute regular-session bars** (Yahoo, written by
  data-ingestion's `intraday-bars` job for candidates and watchlist symbols),
  for the latest 1 / 5 sessions that have them. A symbol with none falls back to
  its last 1 / 5 **daily** bars with `fallback: "no_intraday_data"` — never a
  single daily bar presented as an intraday chart.
- `1M` / `6M` / `1Y` / `ALL`: daily bars, split- and dividend-adjusted
  (`adjusted: true`), anchored on the symbol's latest daily bar so a stale
  symbol still shows a full window. One bar per session (`bar_source_rank`).
- `time` is unix seconds (UTC). Cached 5 minutes per symbol + range.
- Errors: `400 invalid_range` / `invalid_symbol`, `404 no_data_for_symbol`.

---

## 3. Non-functional requirements

- **Auth — v1 has none; this is a HARD BLOCKER before wider exposure.**
  Flagged at step 1: no Keycloak instance runs, there is no `auth/`
  deployment, and the shared HTTP client sends no token. Decision
  (2026-09-23): ship without auth rather than write Keycloak-shaped code
  (bearer validation, JWKS, issuer config) that cannot be exercised until an
  IdP exists. Scoped, not open-ended: the server binds `127.0.0.1` by default
  and Compose publishes it on `127.0.0.1` only. **TODO (blocker):** implement
  validation of the tokens `spog` obtains from Keycloak once Keycloak is
  deployed and testable, before exposing this service beyond this machine or
  a trusted private network.
- **CORS:** allow the origin(s) `spog`/`mfe-scanner` are served from in dev
  and prod. Configurable via env var, not hardcoded.
- **Caching:** this is once-a-day data. A 5-minute in-memory cache on both
  handlers (keyed on nothing for `/today`, on `symbol` for the detail route)
  is reasonable and cheap — avoids hitting Postgres on every page load/refresh
  without adding a cache dependency. Don't cache longer than that; a stale
  cache hiding a corrected re-run would be exactly the kind of
  silent-wrongness bug this project has spent two months eliminating
  elsewhere.
- **Write path: watchlist only.** The scanner endpoints never issue
  `INSERT`/`UPDATE`. The watchlist (§2.5) is the one explicitly-designed write
  path, added 2026-09-24; it touches only `watchlist_items`.
- **Health check:** `GET /healthz` returning 200 once the DB pool is up —
  standard for the container/orchestration layer, matches the pattern other
  services in this repo already use.

---

## 4. Error responses

| Condition | Response |
|---|---|
| No scan has ever completed (fresh install) | `503`, body `{"error": "no_scan_available"}` — frontend shows the error state, not the empty state; these are different things |
| Scan ran but zero candidates in a bucket | `200`, `candidates: []`, `total_candidates: 0` — this is the **empty state**, not an error (median day has ~3 candidates total; zero is normal) |
| DB unreachable | `503`, body `{"error": "database_unavailable"}` |
| Query fails against a reachable DB | `500`, body `{"error": "internal_error"}` — kept distinct so a SQL bug never reads as an outage |
| Holiday calendar doesn't cover today | `500`, body `{"error": "session_calendar_unavailable"}` — extend the list in `calendar.go` |
| `scan.is_stale = true` | Still `200` — staleness is signaled in the payload, not via HTTP status, since the data is valid, just from an earlier day |
| Detail view, unknown/no-data symbol | `404`, body `{"error": "no_data_for_symbol"}` |

The frontend's error-state copy ("Today's scan hasn't completed yet" /
"Couldn't load today's results — showing yesterday's scan") maps to
`no_scan_available` and `is_stale: true` respectively — **not** to any
real-time/connection-drop framing, since none applies here.

---

## 5. Testing

- Unit tests for the query layer against a real Postgres (same pattern as
  the rest of `data-analyzer`'s `store` package — no mocked SQL).
- A fixture with a known scan: some `market` rows, some `penny`, one bucket
  with more than 10 candidates (to confirm the full set is returned
  uncapped), one bucket with 0 (empty case), one row with `catalyst_tier =
  NULL`, one row with **no `momentum_scores` row at all** (confirms the
  `LEFT JOIN` doesn't drop it and `momentum_score_100` comes back `null`).
- Assert **every** candidate in the list response carries `score_status:
  "unvalidated"` — a test that fails if any row is missing the field, since
  the whole point of exposing the score is that this marker can never be
  silently dropped.
- Assert the detail endpoint's `evidence_note` matches the shared
  `EVIDENCE_CAVEAT` constant byte-for-byte, with a test that fails if the two
  ever diverge (this is what actually enforces the "one shared constant"
  rule, not just a comment saying it should be one).
- Assert `is_stale` computes correctly against a mocked "today" one session
  after the last stored scan date, and correctly against a holiday gap
  (reuse the session-calendar test fixtures if they exist).

---

## 6. Build order

| Step | Deliverable |
|---|---|
| 1 | Verify the `close` field and `completed_at` timestamp questions from §2.1/§2.2 against the live schema. Locate (or create) the shared `EVIDENCE_CAVEAT` source both this service and the bot can read. Do not proceed on assumptions. |
| 2 | `cmd/momentum-api/main.go` — HTTP server, `/healthz`, DB pool, config via env vars following the existing `.env.example` convention |
| 3 | `GET /api/v1/scanner/today` — query layer + handler, per §2.1-§2.3 |
| 4 | `GET /api/v1/scanner/today/{symbol}` — detail handler, per §2.4 |
| 5 | Tests per §5 |
| 6 | Dockerfile + Compose service entry, following the existing per-service pattern |
| 7 | Point `mfe-scanner`'s fetch layer at both endpoints; confirm end-to-end with a real scan day, including a live click-through from list row to detail view — **blocked (§7)** |

Report back after step 1 if either schema question, or the `EVIDENCE_CAVEAT`
location, changes the field list — better to adjust this spec than build
against a wrong assumption.

---

## 7. Step 1 findings (verified against the live DB, 2026-09-23)

| Question | Finding | Effect |
|---|---|---|
| `close` | `momentum_features.close` exists and equals the traded (raw) close on all 450 rows; `equity_ohlcv.close` has since been dividend-adjusted on 2 of them | Read from the features row; no join |
| `completed_at` | `momentum_features.computed_at` and `momentum_scores.scored_at` exist, both reset on upsert | No migration; later of the two. No run-completion record exists |
| `ts` | `timestamptz` at 00:00 UTC of the trading day; the scanner writes each symbol at *its own* last bar | `scan.date` = `max(ts)` over `momentum_features` (not scores — a zero-candidate day has no score rows) |
| Scores vs gates | Score rows exist only for gate-passers (0 exceptions) | `LEFT JOIN` kept; `momentum_score_100` column is `NOT NULL`, so a null score always means "no row" |
| `penalties` | `jsonb` array of reason names; the column *default* is the empty object `{}` | Served as a list plus `penalty_total`; `{}` read as none |
| Session calendar | `calendar.go` is seasonality; no holiday calendar existed | Hard-coded NYSE list 2024–2027, cross-checked in tests against the observance rules |
| `EVIDENCE_CAVEAT` | Lived only in the Python bot; two constants; spec text stale | Moved to `shared/content/momentum_caveats.json`; bot and API both read it |
| `universe_eligible` | 4,975 eligible; the scanner skips 2 with `data_unavailable_reason` | Kept as `COUNT(*) WHERE is_eligible` = 4,975 (§3.1's canonical figure) |

**State of the data at build time:** one persisted scan (2026-09-17; 450
symbols; one candidate, NEXR) while bars exist through 2026-09-21 for ~4,956
symbols — the scanner has not run since the backfill completed, so `/today`
currently reports `universe_scanned: 450` and `is_stale: true`.

**Step 7 blockers:** `web-app/ui/mfe-scanner` does not exist (only the `spog`
shell and `shared-components`), and there is no auth (see §3).


# Momentum Scanner — API Service Spec, Addendum: Tracked Positions

**Extends:** `docs/MOMENTUM_SCANNER_API.md` / `services/data-analyzer/cmd/momentum-api`.
**Status:** steps 1–3 built (2026-09-24): `GET /api/v1/scanner/tracked` is live.
Step 4 (`mfe-tracked`) is deliberately deferred until real tracked rows have
accumulated — see §6 for the step-1 findings, the decisions, and the UI brief.
**Serves:** `web-app/ui/mfe-tracked`'s "Tracked Positions" screen.

Same service, one new endpoint. Read `docs/MOMENTUM_SCANNER_PHASE1.md` §5
(sell-side logic) and §10.1.9 (the exit replay findings) before implementing —
this endpoint surfaces data whose *interpretation* the project has already
corrected once (`breakout_failed` firing on session 1, `timeout` never
firing). The API must not repeat that mistake in a different form by
presenting exit reasons as more meaningful than they've been shown to be.

---

## 0. What this endpoint is, and is not

**Is:** a read-only view of `momentum_tracked` — which symbols were alerted,
whether they're still `active` or have `closed`, and why. This is the exact
data `/tracked` already returns in Discord; this endpoint doesn't compute
anything `/tracked` doesn't already have.

**Is not:** a live position tracker, a P&L dashboard, or anything implying
these are real trades. §5 is explicit that `momentum_tracked` is *research
instrumentation* — free labeling data for evaluating exit rules — not a
portfolio. The UI language must not drift toward "positions" in the trading
sense (no "P&L," no "your holdings").

**The exit-rule caveat travels with this data, same pattern as the score.**
Phase 1 §10.1.9 found `breakout_failed` closes 42% of positions on session 1
at a median peak of 0.00% — it's a same-day-stop bug wearing an exit-reason
label, not a validated signal. `timeout` has never fired once across 1,545
replayed exits. An exit reason shown without that context would mislead a
reader into thinking `breakout_failed` means "the breakout genuinely failed."
It usually means "the rule fired immediately for structural reasons." See
§2.2's `exit_reason_note`.

---

## 1. Endpoint

```
GET /api/v1/scanner/tracked
```

### Query parameters

| Param | Default | Values |
|---|---|---|
| `status` | `active` | `active`, `closed`, `all` |

No date range in v1 — `closed` returns everything ever closed. If that
becomes large enough to matter, paginate then; don't build it speculatively.

### Response shape

```json
{
  "summary": {
    "active_count": 3,
    "closed_count": 41
  },
  "tracked": [
    {
      "symbol": "NEXR",
      "exchange": "NASDAQ",
      "company_name": "Nexien Inc",
      "bucket": "penny",
      "status": "active",
      "alerted_date": "2026-09-17",
      "sessions_elapsed": 4,
      "reference_price": 1.64,
      "current_price": 1.71,
      "unrealized_pct": 4.27,
      "max_gain_pct": 9.15,
      "exit_reason": null,
      "exit_reason_note": null,
      "exit_pct": null,
      "closed_date": null
    }
  ]
}
```

### Field sourcing — **verify every one against the live schema before implementing**

Phase 1 §5/§7 describe `momentum_tracked`'s *purpose* but the spec's own
schema table only lists it as "(symbol, alerted_ts), sell-side state from §5"
— it does not enumerate every column. §7's build history for `momentum-api`
found real gaps between what the spec assumed and what the live schema
actually had (`close`, `completed_at`, the `penalties` default) — treat this
endpoint the same way, not as a rubber-stamp.

| Field | Likely source | Verify |
|---|---|---|
| `symbol`, `exchange`, `company_name`, `bucket` | `universe_symbols`, join | same pattern as the candidates endpoint |
| `status` | `momentum_tracked.status` | confirm the exact stored values (`'active'`/`'closed'` per §5's prose — confirm case and exact strings) |
| `alerted_date` | `momentum_tracked.alerted_ts` (primary key) | |
| `reference_price` | `momentum_tracked.reference_price` | |
| `sessions_elapsed` | **STORED** (`momentum_tracked.sessions_elapsed`, verified §6) — served as stored, with `last_evaluated_date` and `evaluation_behind`. The original assumption follows: | Phase 1 §5's `timeout` condition needs "sessions elapsed since alert" — check whether it's persisted or must be computed from `alerted_date` to the latest scan date via the same trading-session calendar `momentum-api` already has (`internal/momentumapi/calendar.go`). Reuse that calendar; don't write a second one. |
| `current_price` | **needs a join** | `momentum_tracked` stores the reference price at alert time, not a live price. Join `momentum_features.close` (or `equity_ohlcv`, per the candidates endpoint's finding that `momentum_features.close` is the correct raw-close source, not the adjustment-drifted `equity_ohlcv.close`) for `ts = latest scan date`. If the symbol has since delisted or stopped scanning, `current_price` may be null — handle that explicitly, don't error. |
| `unrealized_pct` | derived | `(current_price / reference_price - 1) * 100`, only for `status = 'active'`; `null` for closed rows (they have `exit_pct` instead) |
| `max_gain_pct` | `momentum_tracked.max_gain_pct` | per §5, updated while active |
| `exit_reason` | `momentum_tracked.exit_reason` (or similar column name — verify) | one of `breakout_failed`, `lost_vwap`, `momentum_stalled`, `stop_atr`, `timeout`, or `null` while active |
| `exit_pct` | `momentum_tracked.exit_pct` | `null` while active |
| `closed_date` | `momentum_tracked.exit_ts` (verified §6: no `closed_date` column) | the date `status` flipped to `closed` |

### `exit_reason_note` — required whenever `exit_reason` is non-null

This is the mechanism that carries §10.1.9's findings into the UI, the same
way `score_status` carries the score's unvalidated status. **Source these
strings from the same shared caveats file** (`shared/content/momentum_caveats.json`)
as `EVIDENCE_CAVEAT`/`RESEARCH_SCORE_CAVEAT` — add new keys there, don't
hardcode a third copy of caution language in a third place.

| `exit_reason` | `exit_reason_note` (suggested text, finalize against the shared file) |
|---|---|
| `breakout_failed` | "This rule has been measured to fire immediately in most cases — a median 0.00% peak before exit — and is known to be overly sensitive, not a confirmed breakout failure." |
| `stop_atr` | "This condition is frequently pre-empted by faster-firing rules; its priority ordering is unvalidated." |
| `timeout` | (situational — this note only applies if a `timeout` exit is ever actually observed, which Phase 1's replay found never happens in 1,545 cases; if one appears, it's itself notable and worth a distinct note, not the generic one) |
| `lost_vwap`, `momentum_stalled` | Neutral factual description only — these two didn't get flagged as broken in §10.1.9, so don't manufacture a caveat that isn't backed by a finding. |

**Do not write one generic "exit rules are unvalidated" note and apply it
uniformly.** §10.1.9 found specific, different things about specific rules —
`breakout_failed`'s problem is sensitivity, `stop_atr`'s is ordering,
`timeout`'s is that it's never fired at all. A blanket caveat would flatten
findings the project spent real effort distinguishing.

---

## 2. Query logic

```sql
SELECT mt.*, u.name, u.exchange, mf.close AS current_close, mf.bucket
FROM momentum_tracked mt
JOIN universe_symbols u ON u.symbol = mt.symbol
LEFT JOIN momentum_features mf
  ON mf.symbol = mt.symbol AND mf.ts = $latest_scan_date
WHERE mt.status = $status_filter  -- or no filter if status=all
ORDER BY mt.alerted_ts DESC
```

- **`LEFT JOIN` on `momentum_features`**, not inner — a tracked symbol that's
  stopped appearing in scans (delisted, data gap) shouldn't disappear from
  this list; it should show `current_price: null` and let the frontend
  render that honestly rather than silently dropping a real tracked row.
- **Default sort: most recently alerted first.** No ranking implication here
  since there's no score involved — just recency, which is what "what's
  currently being tracked" naturally wants.

---

## 3. Error responses

| Condition | Response |
|---|---|
| No tracked rows exist at all (fresh install, nothing ever alerted) | `200`, `tracked: []`, `summary: {active_count: 0, closed_count: 0}` — empty, not an error; this is the expected state before the scanner has ever fired an alert |
| DB unreachable | `503`, `{"error": "database_unavailable"}` (same as the candidates endpoint) |
| Invalid `status` param | `400`, `{"error": "invalid_status_param"}` |

---

## 4. Testing

- Fixture with at least one `active` row with a `current_price` join hit, one
  `active` row where the join misses (symbol stopped scanning — confirms
  `current_price: null` rather than a dropped row), and one `closed` row per
  exit reason that has ever actually occurred in the real data
  (`breakout_failed`, `lost_vwap`, `momentum_stalled`, `stop_atr` — per
  §10.1.9, not `timeout`, since it's never fired; don't fabricate a fixture
  for a case that's never real).
- Assert every non-null `exit_reason` row carries a non-empty
  `exit_reason_note`, and that the note text matches the shared caveats file
  byte-for-byte — same enforcement pattern as the `EVIDENCE_CAVEAT` test on
  the candidates endpoint.
- Assert `unrealized_pct` is `null` on closed rows and `exit_pct` is `null`
  on active rows — these two fields are mutually exclusive by status, and a
  test should pin that rather than leave it to convention.

---

## 5. Build order

| Step | Deliverable |
|---|---|
| 1 | Verify every field in §1's table against the live `momentum_tracked` schema. Confirm `sessions_elapsed` is computed, not stored, and reuse the existing calendar package. Add the exit-reason note strings to `shared/content/momentum_caveats.json`. |
| 2 | `GET /api/v1/scanner/tracked` — query + handler |
| 3 | Tests per §4 |
| 4 | Point `mfe-tracked`'s fetch layer at it |

Report back after step 1 if the schema differs materially from this spec's
assumptions — same rule as the candidates endpoint's build order.

---

## 6. Step 1 findings (verified against the live DB, 2026-09-24) and decisions

| Question | Finding | Decision |
|---|---|---|
| Data | `momentum_tracked` was **empty**: `momentum-tracker` had never run live. §10.1.9's 1,545 exits come from `-replay`, which is in-memory and writes nothing | Ran the tracker; 7 real active rows now exist (the 2026-09-21 candidates) |
| What opens a row | The tracker opened rows on the **retired score thresholds** (65 market / 72 penny); the bot's screener alerts fire on every gate pass, so "tracked" ≠ "alerted" | New `-open-mode` flag, default **`gates`** (every gate pass). `-open-mode=score` keeps §10.1.9's replay reproducible |
| `status` values | `'active'` / `'closed'`, lowercase | as spec |
| `sessions_elapsed` | **Stored**, maintained by the tracker — the count the exit rules evaluated | Serve the stored value plus `last_evaluated_date` and `evaluation_behind` (true when an active row has not been evaluated through the latest scan), not a calendar recomputation |
| `closed_date` | No such column | From `exit_ts` |
| `current_price` | `momentum_features.close` at the latest scan date (as traded, same source as `reference_price`) | As spec, active rows only, with `current_price_date`. analyst-bot's `/tracked` still reads the adjusted `equity_ohlcv` close and can differ slightly after a dividend |
| Exit reasons | Go constants: `breakout_failed`, `lost_vwap`, `momentum_stalled`, `stop_atr`, `timeout` | Notes for all five in `shared/content/momentum_caveats.json` → `exit_reason_notes`; momentum-api refuses to start if any is missing. Flagged rules carry their §10.1.9 finding; `lost_vwap` / `momentum_stalled` are plain definitions |
| Join to `universe_symbols` | Spec shows an inner JOIN | LEFT JOIN, same reasoning as the candidates endpoint: a row must never disappear |

**Served fields beyond §1's example:** `last_evaluated_date`, `evaluation_behind`,
`current_price_date` (active), `exit_price` (closed). `current_price` and
`unrealized_pct` are null on closed rows; `exit_*` and `closed_date` are null on
active rows (pinned by tests).

**Daily order for real data:** bar ingestion → `momentum-scanner` →
`momentum-tracker` (evaluates exits on the new session, then opens that
session's gate passes). The scanner and tracker were never scheduled before
2026-09-24; `momentum-daily` (data-analyzer) now runs them in that order once
the session's bars have landed, and the analyst-bot alerts only for a scan of
the session that just closed (it used to re-alert whatever scan was latest). Until a session after the alert date has been ingested
and tracked, active rows show `sessions_elapsed: 0` and `unrealized_pct: 0`.

### 6.1 UI brief for step 4 — `mfe-tracked` (deferred; build after real rows accumulate)

Recorded from the product owner, 2026-09-24. **Build only once the full daily
chain — bars (data-universe) → `momentum-scanner` → `momentum-tracker` — has
completed live at least once** and this endpoint returns real, evolving data.
Scheduling scanner + tracker is not enough on its own: on 2026-09-24 both were
scheduled, but the bars worker had skipped 2026-09-22 and -23, so the chain was
wired up but stalled.

This is **research instrumentation, not a portfolio or trading dashboard** — the
system has never executed a trade. No "P&L", "your holdings", or portfolio-app
framing; no live/streaming language; no "performance"/"returns" framing beyond
the literal percentages.

- **Header:** the standard app header, unchanged (the permanent "Screener — not
  a forecast" badge, theme toggle, bell, avatar); title "Tracked Positions"; two
  tabs, **Active** (default) and **Closed**, with counts ("Active (3)",
  "Closed (41)") from `summary`.
- **Active tab table:** Symbol / Exchange / Bucket, Alerted Date, Sessions
  Elapsed, Reference Price, Current Price, Unrealized %. **Unrealized % is the
  only red/green** (`--color-price-up` / `--color-price-down`) — a price fact,
  not a signal. Everything else neutral text, same weight, no urgency icons.
- **Closed tab table:** the same base columns plus **Exit Reason** — plain text
  (e.g. `breakout_failed`), **not** a coloured badge, same rule as Breakout State
  on the candidates screen — and **Exit %** (red/green as above).
- **Exit reason note:** clicking a row's Exit Reason expands an inline panel
  showing `exit_reason_note`. Important — never a tooltip that is easy to miss,
  never collapsed away. Each note is specific to its rule; never apply one
  blanket warning. `lost_vwap` and `momentum_stalled` notes are plain
  definitions — show them plainly and don't add a caveat.
- **Empty states** (existing component): Active, zero rows → "Nothing currently
  tracked"; Closed, zero rows → "No closed positions yet". Neutral.
- **Themes:** both dark and light from the existing token set.
- Also surface `evaluation_behind` honestly (e.g. a neutral note that sessions
  elapsed is as of `last_evaluated_date`) rather than hiding a lagging count.
- **System-level freshness banner** (added 2026-09-24): one banner above the
  tabs, never per row, when the daily chain itself is behind. This is distinct
  from `evaluation_behind`: that flag explains **one** lagging row while the
  rest are current (e.g. a symbol missing a bar); the banner explains the case
  where **every** row is stale at once because nothing upstream ran. That case
  is real — a bars-worker outage stalls the whole chain — and per-row flags
  alone show it as N identical row notes with no cause. Two variants, both
  neutral (informational styling, not an error colour or alarm icon — no data
  is wrong, it is old):
  - **No fresh scan:** "No new scan for N trading sessions — the latest is
    {last_scan_date}. Tracked figures below are as of that date." Shown when
    `sessions_behind ≥ 1`.
  - **Scan fresh, tracker behind:** "The {date} scan exists but tracking has
    not been updated since {last_tracked_session}." Shown when the scan is
    current but the tracker did not complete for it.

  Count in **trading sessions** (the NYSE calendar the API already has), not
  calendar days, so weekends and holidays never raise it. When the banner is
  showing, keep the per-row notes (they stay true) but they need no further
  emphasis — the banner already states the cause.

  **API addition this needs** (none exists yet — `/scanner/tracked` returns
  only `summary` and rows): a `chain` object on that response, computed
  server-side from the same calendar and scan-grace rule as `scan.is_stale`:
  `expected_session` (latest session whose scan should exist by now),
  `last_scan_date` (max `momentum_features.ts`), `last_tracked_session` (latest
  session the tracker evaluated), `sessions_behind` (trading sessions from
  `last_scan_date` to `expected_session`), and `tracker_behind` (bool). Build it
  with the MFE, not before; the UI must not derive staleness from row dates.

