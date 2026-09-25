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
| `MarketOverview` + `ReadingCard` + `SignalGroups` + `ToneIndicator` | `global.macro`, the four stances, `macro_correlations_regime`, `market_cycle_composite` | always visible; details sit behind in-card disclosures |
| `InstrumentGroups` + `InstrumentCard` | `instruments` (`fixed_list` / `watchlist`) | `CollapsibleCard` `report.tracked`, `report.watchlist` |
| `SeasonalitySection` | `seasonality`, `presidential_cycle`, `intermarket` | `CollapsibleCard` `report.seasonality` |
| `CalendarNewsSection` | `calendars.economic`, `earnings_coverage` + `earnings_calendar`, `news` | `CollapsibleCard` `report.calendar` |
| `CoverageNote` | `automation_status`, `data_gaps` | plain muted footer |

## Rules

- Payloads are passed through untyped. `utils/payload.ts` reads them
  defensively.
- **The UI never decides a classification or a colour.** The backend stores a
  `tone` (`constructive` / `neutral` / `stressed` / `no_data` / `display_only`)
  on every stance, signal and the correlations regime. `ToneIndicator`'s
  `TONE_INDICATORS` table is the entire mapping: tone → icon shape + accessible
  name + `--color-status-*` token (green / amber / red / gray). A null, missing,
  unknown or `display_only` tone renders no indicator. There is no label or
  threshold logic anywhere.
- Section 1 has five classification cards: Monetary Policy, Growth Cycle,
  Inflation, Global/Geopolitical Stress and Macro Correlations Regime. Each one
  shows the indicator, the stored label verbatim (`elevated_stress`, not
  reworded) and the score. A null section shows the gray no-data indicator and
  "no data". The disclosure lists the signals grouped by stored `payload.tier`
  (1, 2, 3), each group headed "Tier N · <tier_group>", with untiered signals
  last under "Other signals". Each row has its own tone, name, label
  (`payload.regime`, or `payload.margin_signal` for the PPI–CPI spread), value
  and as-of. `display_only` rows (treasury yields) show their `<n>y_pct` levels
  as plain numbers, with no indicator and no label. The correlations disclosure
  shows `payload.label` and `payload.flags` as plain text.
- Status colour appears only in Section 1. Beyond that, the only red/green is
  the day's change %, via `--color-price-up/down`, and zero stays neutral. The
  `guards` tests in `MarketReportPage.test.tsx` enforce both rules, in the DOM
  and in the CSS.
- Numbers go through `common/format/sign.ts`, so negatives use U+2212.
  Nulls render "—".
- Yields show a value and a date only. A priced instrument shows the market-cycle
  facts, or its `unavailable_reason` when they are missing. The basis and windows
  come from the row itself: crypto shows "00:00 UTC daily close", 365, 14/7.
- The copy for earnings coverage and data gaps comes from the API (`note`).
  Fallbacks exist only for a missing note.
