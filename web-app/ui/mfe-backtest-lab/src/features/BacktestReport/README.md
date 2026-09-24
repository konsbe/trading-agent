# BacktestReport

The Backtest Lab screen: the frozen Phase 2 report (`BacktestLabReport`) rendered
top to bottom by `ReportView`. Nothing here is live, re-runnable or linked to a
symbol. The only controls are the three section collapse toggles and the
hypothesis `b` note disclosure.

| Component | Report field | Card |
|---|---|---|
| `ReportHeadline` | `report.headline` (verbatim) | plain block |
| `EntryGateSection` | `entry_gate` | `ReportCard`, always expanded |
| `V2ScoreSection` | `v2_score_finding` | `CollapsibleCard`, `backtest.v2` |
| `StratificationFunnel` | `rvol_stratification_funnel` | `CollapsibleCard`, `backtest.funnel` |
| `ResearchRoundSection` | `research_round_1` + `sample_size` | `CollapsibleCard`, `backtest.round1` |
| `ClosingSection` | `closing_statement` (verbatim) | `ReportCard`, always expanded |

## Rules

- No red/green and no pass/fail styling. Verdicts are plain text, with the same
  class, colour and weight on every row. Emphasis comes from layout and weight.
- Numbers use the authored precision (`utils/format.ts` `PRECISION`). JSON drops
  trailing zeros, so `0.00` and `9.90` are formatted back to their fixed digits.
- Funnel bars are sized on one linear axis from 0 to `funnelScaleMax`, which is the
  largest OR rounded up to 0.2. The dashed line is at OR 1.0. The bars are
  `aria-hidden`, and each step's label, OR and details are real text.
- Research-round rows are tested and abandoned hypotheses merged in id order.
  Abandoned rows get effect `—` and verdict "Abandoned before testing".
  `verdict_note` sits behind a "Note" disclosure (`aria-expanded`) that opens a row
  beneath. The open state lives above the card, so it survives a collapse.
