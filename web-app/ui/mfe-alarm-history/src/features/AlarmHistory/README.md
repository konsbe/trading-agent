# AlarmHistory feature

The Alarm History screen: alerts the analyst bot's alert scan posted to Discord
(`fired_alerts`, served by momentum-api `GET /api/v1/alerts`). The momentum
screener's alerts are not recorded there.

## Pieces

| Path | Role |
|------|------|
| `AlarmHistoryScreen.tsx` | Caveat, records and onsets notes, filters, and the alerts card (grouped or every alert) |
| `providers/TypeLabelsContext` | The response's `type_labels` for chips and tables (`useTypeLabel`) |
| `hooks/useAlarmHistory.ts` | Page state: filters, view (`?view=all`), each table's server-side sort + search (shared `useTableView`, URL keys `groups_*` / `alerts_*`), expanded groups, refresh token |
| `hooks/useAlarmFilters.ts` | Symbol (debounced), type / severity chips, local-day date range (default: last 7 days), all in the URL (`symbol`, `type`, `severity`, `from`, `to`; defaults omitted) |
| `hooks/usePagedAlerts.ts` | One list: keyset paging (`before` / `next_before`) with refresh merge for the default newest-first view; offset paging (`offset` / `next_offset`) under a pinned `until` for a sorted or searched view |
| `utils/alertColumns.ts` | Column definitions (key = API `sort` value; every column sortable), `serverSortQuery`, caption wording |
| `hooks/useAutoRefresh.ts` | Refresh token: every 60 s while the tab is visible, and on tab focus |
| `utils/paging.ts` | `appendPage` / `mergeFirstPage` for raw rows and groups |
| `components/GroupedAlertsTable` | One row per symbol + type; a group with repeats expands to `GroupAlerts` |
| `components/GroupAlerts` | An expanded group's alerts (raw, same range/severity filters and search, newest first), 5 per page |
| `components/AlertsTable` | Individual alerts (every-alert view and group expansions) |
| `components/AlertSymbol` | Symbol; hosted, links to Stock Detail `#classical-signals` ("Back to Alarm History") when the shared `isStockDetailEligible` allows it (not crypto, VIX or `vix_elevated`) |

## Behaviour

- Dates: the From / To boxes are local calendar days. The API gets `since` =
  local midnight of From and `until` = local midnight of the day after To, as
  RFC3339 with the offset of that day (`common/dates/localDays.ts`), so DST
  changes are exact. An empty box leaves that side open.
- Any filter, view, sort or search change restarts paging from the first page
  and closes expanded groups. Everything but expanded groups is in the URL, so
  "Back to Alarm History" restores the view.
- Sorting and search are server-side (`sort` / `dir` / `q`): header clicks and
  the search box (debounced 300 ms) only change the request. The default sort
  (fired, desc) sends neither and keeps the id cursor. A sorted or searched
  view pins `until` to its first load's time (the earlier of that and the
  range's end) on every page, pages with `offset`, and shows "Load more" /
  "Showing the first N". A refresh starts a new view: page 1, newly pinned,
  replacing the list. 400s (`invalid_sort`, `invalid_query`, …) show in plain
  words in place of the list.
- Refresh (default view) re-reads only the first page (of the active list and of each
  expanded group) and merges it: new rows and updated groups replace the head;
  rows loaded with "Load older" stay, with their cursor. If more than a page of
  new alerts arrived since the last check, the list is replaced and paging
  starts again.
- The caveat is the API's `caveat`, rendered verbatim.
- Alert type labels come only from the API's `type_labels` (sourced from
  `shared/content/alert_messages.json`); a type without a label shows its id
  as-is. The MFE keeps no label copy of its own.
- Onsets note: with `onsets_since` set, it dates the switch from re-posting an
  ongoing condition every few hours to onset-only alerts; while it is null it
  says onset-only alerts start with the next release.
- An alert with a `bar_date` (an onset) shows "bar Sep 26" under its fired time
  in the every-alert list and in an expanded group; alerts from before the
  switch show nothing extra.
- A body without `type_labels`, `onsets_since` or `bar_date` (an API from before
  them) still parses: no labels, null onsets, no bar dates.
