# Pre-registration — Classical TA Heuristic Signals

**Status:** committed before any hypothesis is tested. No result exists yet.
**Scope:** validates the signals in `docs/MOMENTUM_SCANNER_API_STOCK_ANALYSIS.md`
§2.4 (`heuristic_signals`) — RSI bands, MACD crosses, BB squeeze, chart
patterns, liquidity-sweep conditions, and the `BUY_WATCH`/`TRIM_WATCH`
composite. This is a separate validation track from the momentum score;
it does not reuse momentum's labels or hypotheses, only its protocol
(purged walk-forward, stratification, pre-registered effect-size floor,
one bounded round).

**Universe:** the full eligible backfill universe (all symbols in
`equity_ohlcv` with 10-year history, plus crypto symbols in their own
stratum), **not** scoped to any watchlist or candidate list. A live
watchlist is a *serving* scope, decided later and separately; it is the
wrong population for a backtest, which needs breadth to have any power.

**Lockbox:** reuses the existing region-based lockbox
(`phase2_lockbox_v2`, 2025-03-28–2026-03-27, symbols outside
`momentum_pilot_cohort`) rather than carving a second one. One canonical
never-touched region for the whole project is simpler than proliferating
lockboxes, and nothing about this validation has looked at that window.

---

## 1. Why this needs its own labels

Momentum's labels ask "does this reach +100% within 120 sessions" — a
magnitude question for an explosive setup. These heuristics make a
different, shorter-horizon, directional claim ("price moves favorably in
the next few sessions"). Testing them against momentum's label would be
a category mismatch. New labels, defined once here:

```
fwd_return_Ns   = (close[t+N] / close[t] - 1) * 100      for N in {5, 10, 20}
fwd_abs_move_Ns = max(|close[t+i]/close[t] - 1| * 100)   for i in 1..N
                  (largest absolute move within the window — for
                  magnitude-only claims like BB squeeze, which predicts
                  volatility expansion, not direction)
```

Entry reference is `close[t]` on the day the signal fired, matching
momentum's convention. No lookahead: labels use only bars after `t`,
same rule as every prior label definition in this project.

---

## 2. Episodes — one series per signal type, never pooled

```
For each signal type independently:
episode = the first occurrence of that signal for a symbol after
          >= SIGNAL_EPISODE_GAP_SESSIONS sessions with none (default 5,
          matching momentum's convention)
```

Each signal type gets its own episode table. **Do not pool different
signal types into one episode set** — this is the direct lesson from
`breakout_state`, where collapsing `breakout` and
`breakout_from_consolidation` into one field hid two opposite effects
(20.89% vs 6.42% hit rate). A "bear flag" episode and a "liquidity
sweep" episode are different claims and must be counted, and reported,
separately.

---

## 3. Hypotheses — components first, composite last, one bounded round

All of the following are tested in **this one round**, Benjamini-Hochberg
corrected across the full family (11 hypotheses). The composite signals
(H9, H10) are tested last, using the same episodes and folds as the
component hypotheses that feed them — this is not a "round 2," since
nothing here is decided after seeing an earlier result; the ordering is
just presentation (components, then what they're built from).

Every hypothesis: **bucket-stratified by asset type (equity/crypto) and
by ATR/volatility tercile within equities**, episode level, purged
walk-forward, CMH for pooled power, never accepted on a pooled-only
result (Rule 1, inherited unchanged from Phase 2 §4.2.0).

| # | Hypothesis | Label | Direction | Effect-size floor |
|---|---|---|---|---|
| H1 | RSI overbought (>70) predicts negative drift | `fwd_return_10s` | negative | MH OR ≥ 1.25, ≥ 3.0pp within stratum |
| H2 | RSI oversold (<30) predicts positive drift | `fwd_return_10s` | positive | same |
| H3 | MACD bullish cross predicts positive drift | `fwd_return_10s` | positive | same |
| H4 | MACD bearish cross predicts negative drift | `fwd_return_10s` | negative | same |
| H5 | BB squeeze active predicts a larger subsequent move (volatility expansion, no direction claimed) | `fwd_abs_move_10s` vs baseline | magnitude only | ≥ 1.25x relative lift over non-squeeze baseline |
| H6 | Confirmed bear flag / confirmed H&S predicts negative drift | `fwd_return_10s` | negative | MH OR ≥ 1.25, ≥ 3.0pp |
| H7 | Confirmed bull flag / confirmed inverse H&S predicts positive drift | `fwd_return_10s` | positive | same |
| H8a | Low liquidity sweep + close back above predicts positive drift | `fwd_return_10s` | positive | same |
| H8b | High liquidity sweep + close back below predicts negative drift | `fwd_return_10s` | negative | same |
| H9 | Composite `BUY_WATCH` (all 4 conditions) predicts positive drift beyond any single component | `fwd_return_10s` | positive | MH OR ≥ 1.25 **and** must exceed the best single component's OR from H1–H8, or it adds nothing over its inputs |
| H10 | Composite `TRIM_WATCH` (3-condition variant) predicts negative drift beyond any single component | `fwd_return_10s` | negative | same |

**"Uptrend intact" is not its own hypothesis.** It's a context filter
inside the composite, not an independent directional claim — treated as
a stratification variable when testing H8a/H8b/H9/H10, not tested alone.

**H9/H10's floor is stricter than the others on purpose.** A composite
built from four conditions is only worth keeping if it beats picking the
best single input — otherwise it's added complexity with no added
signal, the exact failure mode momentum's own v2 composite had (rvol
alone out-separated the full score, +5.68pp vs +2.21pp pooled). This
floor makes that comparison mandatory rather than optional.

**All 10 sessions of 5/10/20-day windows are reported**, not only the
one in the table — the 10-session column is the primary pre-registered
target; 5 and 20 are secondary context, same "report every horizon
tested" discipline as momentum's first-passage work.

---

## 4. Stopping rule

> **One round. If a hypothesis doesn't clear its floor, it's reported as
> not confirmed and left there. No round 2 for that signal type without
> a new written justification from the user — same rule as Phase 2 §2.2,
> applied per signal type, not just globally.**

A signal type reported as "not confirmed" is not removed from the
product — it can still render in `heuristic_signals` with its caveat,
exactly as it does today. This validation decides what the data actually
shows, not what ships. Nothing here gates the feature's existence, only
what's claimed about it.

---

## 5. Build order — the replay harness

Momentum's replay pattern (`-replay`, same `EvaluateExit` as live) is
the template: one code path, walked backward over stored history, so
replay and live results stay comparable.

| Step | Deliverable |
|---|---|
| 1 | **Verify before building.** Do `technical-analysis`'s indicator functions (RSI, MACD, ADX, BB squeeze, SMC pattern detection, the confluence-score logic) already accept an arbitrary historical window, or are they wired only to "compute on the latest bar"? This is the same question asked of `marketcycle` in the Daily Market Report addendum, and it may have the same answer either way — check, don't assume. |
| 2 | If needed: extract a pure batch-replay entry point, mirroring `ComputeAt(bars, i)` from the momentum feature engine — output must equal running the live function on `bars[:i+1]`, same no-lookahead test discipline. |
| 3 | New table(s): one episode table per signal type (§2), each with `fwd_return_5s/10s/20s`, `fwd_abs_move_10s`, `label_complete` (per momentum's convention — episodes inside the last 20 sessions are incomplete and excluded from evaluation). |
| 4 | Run the replay over the full universe, 10-year history, both asset-type strata. |
| 5 | Compute H1–H10 exactly as pre-registered here. Report every one, confirmed or not. |
| 6 | Lockbox evaluation — once, on whatever clears the round, same one-shot rule as Phase 2 §4.3. |

**Do not touch the lockbox before step 6, and only for hypotheses that
already cleared their floor on the non-lockbox data.** Same discipline,
same reason, as every other use of this lockbox.

---

## 6. What this does not do

- Does not change `heuristic_signals`' current live behavior. The
  `BUY_WATCH`/`TRIM_WATCH` labels and reasoning text stay exactly as
  built until this validation reports a result and a decision is made
  about what to do with it.
- Does not test every possible TA heuristic — only the ones currently
  live in the product (§0). A new pattern added later needs its own
  hypothesis, not silent inclusion in this round's results.
- Does not promise a positive result. Per the honest-reading precedent
  set by momentum's own closure: "these heuristics show no demonstrated
  edge on daily bars" is as valid and complete an outcome as any
  confirmation would be.