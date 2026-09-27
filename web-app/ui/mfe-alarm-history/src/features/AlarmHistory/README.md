# AlarmHistory feature

The Alarm History screen: alerts the analyst bot's alert scan posted to Discord
(`fired_alerts`, served by momentum-api `GET /api/v1/alerts`). The momentum
screener's alerts are not recorded there.

## Pieces

| Path | Role |
|------|------|
| `AlarmHistoryScreen.tsx` | Caveat + records note, filters, and the alerts card (grouped or every alert) |
| `hooks/useAlarmHistory.ts` | Page state: filters, view, expanded groups, refresh token |
| `hooks/useAlarmFilters.ts` | Symbol (debounced), type / severity chips, local-day date range (default: last 7 days) |
| `hooks/usePagedAlerts.ts` | One list with keyset paging (`before` / `next_before`), "Load older", refresh merge |
| `hooks/useAutoRefresh.ts` | Refresh token: every 60 s while the tab is visible, and on tab focus |
| `utils/paging.ts` | `appendPage` / `mergeFirstPage` for raw rows and groups |
| `components/GroupedAlertsTable` | One row per symbol + type; a group with repeats expands to `GroupAlerts` |
| `components/GroupAlerts` | An expanded group's alerts (raw, same range/severity filters), 5 per page |
| `components/AlertsTable` | Individual alerts (every-alert view and group expansions) |
| `components/AlertSymbol` | Symbol; links to Stock Detail `#classical-signals` for equity, non-market-wide alerts when hosted |

## Behaviour

- Dates: the From / To boxes are local calendar days. The API gets `since` =
  local midnight of From and `until` = local midnight of the day after To, as
  RFC3339 with the offset of that day (`common/dates/localDays.ts`), so DST
  changes are exact. An empty box leaves that side open.
- Any filter or view change restarts paging from the first page and closes
  expanded groups.
- Refresh re-reads only the first page (of the active list and of each
  expanded group) and merges it: new rows and updated groups replace the head;
  rows loaded with "Load older" stay, with their cursor. If more than a page of
  new alerts arrived since the last check, the list is replaced and paging
  starts again.
- The caveat is the API's `caveat`, rendered verbatim.
