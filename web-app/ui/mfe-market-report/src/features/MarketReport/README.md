# MarketReport

The Daily Market Report screen. It shows the latest report from
`GET /api/v1/market-report/today`, which is generated every 6 hours. The report
is fetched once. Its freshness is the generation timestamp in the subtitle
(`ReportSubtitle`); there is no refresh and there are no timers. The only links
are news links, which open in a new tab. The only buttons are the collapse
toggles and the in-card disclosures.

| Component | Report field | Container |
|---|---|---|
| `ReportSubtitle` | `generated_at`, `report_date`, `is_stale` | page subtitle |
| `MarketOverview` + `ReadingCard` | `global.macro`, the four stances, `macro_correlations_regime`, `market_cycle_composite` | always visible; details sit behind in-card disclosures |
| `InstrumentGroups` + `InstrumentCard` | `instruments` (`fixed_list` / `watchlist`) | `CollapsibleCard` `report.tracked`, `report.watchlist` |
| `SeasonalitySection` | `seasonality`, `presidential_cycle`, `intermarket` | `CollapsibleCard` `report.seasonality` |
| `CalendarNewsSection` | `calendars.economic`, `earnings_coverage` + `earnings_calendar`, `news` | `CollapsibleCard` `report.calendar` |
| `CoverageNote` | `automation_status`, `data_gaps` | plain muted footer |

## Rules

- Payloads are passed through untyped. `utils/payload.ts` reads them
  defensively. `signalStatus` takes the first string among `regime`, `stance`,
  `status`, `label`, then any `*_regime` key, and returns null if there is none.
  A status is never invented.
- The only red/green is the day's change %, via `--color-price-up/down`, and
  zero stays neutral. Regimes, phases and statuses are plain text.
- Numbers go through `common/format/sign.ts`, so negatives use U+2212.
  Nulls render "—".
- Yields show a value and a date only. A priced instrument shows the market-cycle
  facts, or its `unavailable_reason` when they are missing. The basis and windows
  come from the row itself: crypto shows "00:00 UTC daily close", 365, 14/7.
- The copy for earnings coverage and data gaps comes from the API (`note`).
  Fallbacks exist only for a missing note.
