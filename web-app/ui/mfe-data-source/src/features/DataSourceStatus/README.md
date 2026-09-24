# DataSourceStatus

The Data Source screen: current operational health from
`GET /api/v1/data-sources/status`. It is the one deliberately live-framed page.
It is checked on mount and when the user presses Refresh, and never on a timer.
It is read-only: there are no links and no retry or restart controls. The only
buttons are Refresh and the two collapse toggles.

| Component | Shows | Card |
|---|---|---|
| `CheckControls` | `LastChecked` ("Last checked: …", local time with seconds) and `RefreshButton` ("Checking…" and disabled while in flight) | page header |
| `OverallStatus` | `overall` + `overall_reasons` | always visible, not collapsible |
| `ProvidersSection` / `ProviderCard` | one card per `PROVIDER_DISPLAY_ORDER` key | `CollapsibleCard`, `datasource.providers` |
| `DailyChainSection` | sessions newest first, last-clean highlight, coverage note | `CollapsibleCard`, `datasource.chain` |
| `DatabaseUnavailablePanel` | the 503 (`kind: database_unavailable`) as the finding itself | full-width alert |
| `SectionUnavailable` | a section served as `"unavailable"` | inline |

## Colour rules

- `OverallStatus` is the only status-coloured element. It uses the kit's
  system-status tokens (`--color-status-ok` / `--color-status-warning`), never
  the price aliases, and it is shaped as a pill with a dot and an icon.
- A provider bar is primary. It switches to `--color-status-warning` only when
  `overall_reasons` names that provider's budget (`utils/status.ts`
  `isProviderOverBudget`).
- Chain statuses are plain text with no colour. `not_run` gets bold text and a
  glyph. `not_recorded` is muted.
- A provider without a daily limit (Finnhub) gets no bar and no percentage.
- A null `degraded_count_24h` reads "not tracked".
