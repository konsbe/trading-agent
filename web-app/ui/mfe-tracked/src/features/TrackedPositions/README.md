# TrackedPositions feature

The Tracked Positions screen (`/tracked-positions`): every scanner gate-pass
alert followed through the exit rules. Research instrumentation, not a
portfolio — no P&L, holdings, performance or live framing anywhere in the copy.
Spec: `docs/MOMENTUM_SCANNER_API.md`, Tracked Positions addendum §6.1.

## Data

`useTrackedPositions` fetches `GET /api/v1/scanner/tracked?status=all` once and
splits the rows by `status`, so switching tabs never refetches. Tab counts come
from `summary`. The list re-reads when the browser tab becomes visible again
(`useRefreshOnVisible`); a failed re-read keeps the rows and shows a neutral
line instead of the error state.

## Pieces

| Piece | What it does |
|---|---|
| `FreshnessBanner` | One neutral note above the tabs from `chain` only (`utils/freshness.ts`): "No new scan for N trading sessions …" when `sessions_behind ≥ 1`, else "The {date} scan exists but tracking has not been updated since {date}." when `tracker_behind`. Never derived from row dates. |
| `TrackedTabs` | WAI-ARIA tabs, "Active (n)" / "Closed (n)"; arrow keys, Home, End. The selection lives in `?tab=closed` (`useTrackedTab`). |
| `TrackedPanel` | The tab's `CollapsibleCard` (`tracked.positions.active` / `.closed`), the "not yet evaluated" count note, a `TableSearch`, the shared `useTableView` (URL keys `active_*` / `closed_*`, default most recent alert first) and the empty states "Nothing currently tracked" / "No closed positions yet". |
| `TrackedTable` | Every header sorts (`SortableHeader`; "Not yet evaluated" / "—" last both ways). Active: Symbol / Exchange / Bucket, Alerted Date, Sessions Elapsed, Reference Price, Current Price, Unrealized %. Closed: Symbol / Exchange / Bucket, Alerted Date, Closed Date, Sessions Elapsed, Reference Price, Exit Reason, Exit % (no exit price). |
| `SessionsCell` | "Not yet evaluated" when `last_evaluated_date` or `sessions_elapsed` is null (`isNotYetEvaluated`); "as of {date}" under the count when `evaluation_behind`. |
| `SymbolCell` | Ticker (hosted: the shared `StockDetailLink`, "Back to Tracked Positions" with the tab, sort and search), company · exchange · bucket. |
| `TrackedSkeleton` | Loading placeholder with the page's layout. |

## Rules the tests pin

- Only Unrealized % and Exit % are toned, via the kit's `ChangeCell`
  (`--color-price-up` / `--color-price-down`; zero and null stay neutral).
- A row never evaluated (the API serves null `last_evaluated_date`,
  `sessions_elapsed` and `unrealized_pct`) shows "Not yet evaluated" and "—"
  for Unrealized %, never 0 or 0.0%.
- Exit Reason is plain text in a button (`aria-expanded`, `aria-controls`);
  it opens `exit_reason_note` in a row directly below, which stays open until
  the user closes it (state held by the screen in `useOpenNotes`, so it
  survives a collapsed card or a tab switch). Never a tooltip.
- Rows are keyed by symbol + alert date: a symbol can be tracked twice.
