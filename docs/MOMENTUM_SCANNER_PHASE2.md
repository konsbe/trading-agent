# Momentum Scanner — Phase 2 Implementation Spec (The Model)

**Target repo:** `trading-agent`
**Status:** specification. Nothing in §5-§9 is built yet.
**Entry gate CLEARED 2026-09-21** — the Phase 1 post-upgrade validation is
complete and documented in `docs/MOMENTUM_SCANNER_PHASE1.md` §10.1. Three
prerequisites named in §10 were built during it: migration 012 (raw close +
`split_factor`), migration 013 (the frozen pilot cohort) and migration 014
(`phase2_lockbox`, reserved and hashed — see §4.3). `rvol` replicated
out-of-sample, so per §2 Phase 2 proceeds as **the full plan**. Note §10.1.4:
v2's pooled OOS pass is bucket composition, so the frozen-v2 baseline in §5.3 is
weaker than its p-value suggests.
**Depends on:** Phase 1 complete (`docs/MOMENTUM_SCANNER_PHASE1.md`) **and** the
Phase 1 post-upgrade validation run on Tiingo Power (see §2).
**Scope:** replace the hand-weighted `momentum_score_100` with models that output
**calibrated probabilities**, validated on data no decision has touched; deliver
the multi-threshold display only where the data supports it; revise §5's exit
rules under the same protocol; serve everything in shadow mode first.

This document is written for a coding agent that has the repo but **not** the
conversation that produced it. Every rule is stated explicitly.

---

## 0. Read these first

| File | Why |
|---|---|
| `docs/MOMENTUM_SCANNER_PHASE1.md` | §3 feature definitions, §4.1 v2 evidence and its **in-sample warning**, §4.4 thresholds, §5 replay findings, §6 labels, §12 rules. **All Phase 1 §12 rules still apply.** |
| `docs/SUBSCRIPTIONS_PLANS.md` | Provider limits and the "how to evaluate a provider" checklist. Use it for every new source in §3. |
| `shared/schemas/SCHEMAS.md` | Migration rule (committed = never amend), and the "3 a.m." test for state columns |
| `services/data-analyzer/internal/momentum/` | Features, gates, labels, score. **Reuse these; do not reimplement them in Python** (see §12) |
| `services/data-analyzer/cmd/momentum-backtest`, `cmd/momentum-tracker -replay` | The evaluation and replay harnesses Phase 2 extends |
| `services/data-ingestion/internal/barquality` | The adjustment-seam detector. Run it on any new price data |
| `services/model-training/` | Empty today. It becomes Phase 2's home |

---

## 1. What Phase 2 delivers

Phase 1's scoring was revised in-sample three times on the same 218 candidates.
Each revision was reasonable, and together they are exactly how overfitting
happens. **Phase 2 exists to produce numbers that can be believed**, so the
validation protocol (§4) matters more than the models (§5).

1. **Data corrections** that stop the dataset from leaking the future or hiding
   failures (§3).
2. A **leak-audited training dataset** at the episode level.
3. A **validation protocol**: purged walk-forward, a one-shot lockbox, and
   pre-registration.
4. **Baselines and models** for `P(hit_k)`, with calibrated outputs.
5. The **multi-threshold display** (+100/+200/+300/+500/+1000) that was the
   original goal, shown **only for thresholds with enough data**.
6. **§5 v2 exit rules**, evaluated under the same protocol.
7. **Shadow-mode serving**: predictions stored and shown next to v2, while alerts
   keep coming from v2 until an explicit decision says otherwise.

### The expected outcome, stated up front

**Given the Phase 1 post-upgrade findings, "no edge on daily bars" is a
realistic outcome for Phase 2 — and if it is the result, it is a complete
deliverable, not a failure.**

This is written here, before any modelling, because the evidence already points
that way and an expectation set afterwards is worthless:

- §4.1 v2, the hand-weighted score, is **not confirmed** out-of-sample. Its
  pooled pass was bucket composition (Phase 1 §10.1.0).
- `rvol_20`, the one component that replicated and survived stratification,
  lands at **MH OR 1.19-1.215 — below this document's own §4.2.0 floor of
  1.25.** The single best signal found does not clear the acceptance bar.
- The largest measured effects on `hit_100` are restatements of volatility
  (§5.1.1), and volatility is also what produces the 25-39% median forward
  drawdowns.

So the realistic range of Phase 2 results runs from "a small edge in one bucket"
to "no demonstrated edge". **The second is a genuine finding about a real
strategy on real data, and it is worth more than a fitted model that looks
better.** §12's rule is the operational form of this: do not keep searching
feature combinations until something turns out significant.

What would make Phase 2 a failure is not a null result. It is a null result
dressed up as a positive one, or a positive one that nobody can check.

### Out of scope
Intraday and real-time data (Phase 3), the web UI / MFE, order execution, short
interest, ADRs/REITs, non-US markets, LLM-based catalyst classification, and
neural networks.

### Subscriptions
Phase 2 needs **no new paid services.** Tiingo Power covers bars and Tiingo News.
SEC EDGAR is free, and Finnhub's free tier covers the rest. Do not add a paid
source without going through `SUBSCRIPTIONS_PLANS.md` first.

---

## 2. Entry gate

Phase 2 starts only when the Phase 1 post-upgrade validation is **complete and
documented** in the Phase 1 spec. That means all of the following are done:

- a 10-year backfill of the full eligible universe;
- raw close and `splitFactor` captured;
- bar-audit run over the result;
- a full-universe fundamentals pass;
- the lockbox reserved and manifested (§4.3);
- the pre-registered out-of-sample (OOS) report on frozen §4.1 v2.

### 2.1 The final entry-gate measurement — rule committed BEFORE running it

**Written and committed before the measurement was executed.** Everything in
this project that went wrong went wrong by deciding after seeing a number, so
the rule is fixed first and the result is whatever it is.

> **Final entry-gate measurement:** `rvol` CMH stratified by **bucket x ATR
> tercile**, **episode level**, **gate v2 with the us-gaap fallback**, **lockbox
> excluded.**
>
> - **Positive direction AND p < 0.05 -> the full plan.**
> - **Otherwise -> feature research only.**
>
> **This is the last re-measure for routing. No variants after it.**

Why this specific test and no other:

- **bucket x ATR tercile**, because both have already been shown to manufacture
  a signal through composition — v2's score through bucket (§10.1.4 of Phase 1)
  and `rvol` through volatility (§10.1.5b). Stratifying on both is the only
  form of the test that has not yet produced a spurious positive.
- **episode level**, because consecutive gate days of one move are not
  independent (§3.1).
- **gate v2 with the fallback**, because the earlier v2 result bundled the leak
  correction with a coverage exclusion, and the fallback removes as much of the
  coverage confound as EDGAR allows (§3.2.3).
- **lockbox excluded**, as in every report.

#### RESULT (run 2026-09-22, after the rule above was committed)

**Route: FEATURE RESEARCH ONLY.**

Gate v2 with the us-gaap fallback, 4,294 symbols with point-in-time shares,
9,407 evaluable candidates, 6,936 episodes, base rate 9.90%, lockbox region and
population A excluded.

| The pre-registered test | Result |
|---|---|
| **CMH, bucket x ATR tercile, episode level** | **chi-square(1) = 0.00, p = 0.947** |
| MH common odds ratio | **0.991** |
| Required for the full plan | positive direction AND p < 0.05 |
| Verdict | **FAILS on both** |

Supporting detail, reported because the rule says report it either way:

| Stratum | n | high-rvol hit% | low-rvol hit% |
|---|---|---|---|
| market / atr high | 1,810 | 14.81% | **15.36%** |
| market / atr mid | 2,264 | 7.33% | 6.10% |
| market / atr low | 2,309 | 1.21% | 1.21% |
| penny / atr high | 501 | 42.00% | **46.61%** |
| penny / atr mid | 48 | 20.83% | 25.00% |

Within-bucket: market +0.19pp (p = 0.809), penny −0.31pp (p = 0.951).

**The coverage recovery did not change the answer.** CMH by bucket x ATR was
p = 0.897 without the fallback and p = 0.947 with it; the MH odds ratio moved
from 0.985 to 0.991. Both are indistinguishable from no effect. Taken with the
decomposition (Phase 1 §10.1.5c), which located `rvol`'s v1 separation inside
the leak-corrected group (+4.63pp) and found it negative in the largest
coverage group (−2.60pp), the two independent checks agree: **the v1 signal was
a lookahead artifact, not a coverage artifact.**

**The pooled number is still large and still meaningless.** +6.80pp,
p < 0.001, and the top ATR tercile alone gives +14.57pp — and within
bucket x ATR cells `rvol` points the *wrong way* in both high-volatility
strata. That is the third time in this project that a pooled statistic has
survived where a stratified one did not, and it is the reason §4.2.0 forbids
pooling as acceptance evidence.

**No further routing measurements will be run.** Per the rule, this was the
last one.

#### ROUTE CONFIRMED BY THE USER, 2026-09-22

**Feature research only.** Confirmed without amendment, on the pre-registered
rule, before any discussion of whether the result was convenient.

That is the entire value of writing the rule down first, and it is worth
recording what it bought: without this protocol the project would have shipped
a 0-100 score with p-values behind it that measured the backtest's own
selection. The score looked validated at every checkpoint until the candidate
set stopped containing information from the future.

**"No variants after it" is the part that matters.** Re-running with a
different gap, a different tercile count, a different bucket definition or a
different threshold until one clears p < 0.05 is the failure mode §12 exists to
prevent, and at this point it would be the third time the same mechanism had
produced a positive. If this test does not clear the bar, the answer is
"feature research only", and that is a result rather than a prompt to keep
looking.

What the two routes mean in practice:

| Route | Phase 2 becomes |
|---|---|
| full plan | §3-§9 as written: models, calibration, multi-threshold display, shadow serving |
| feature research only | §3, §4 and §5.3 baselines. **No multi-threshold display, no serving.** The scanner may still exist as an honest screener — "stocks up 8-15% on unusual volume that pass the liquidity and size gates" — with no claim that its ranking predicts which of them run. Record the honest reading: the "+8-15% momentum entry" thesis shows no demonstrated edge on daily bars. |

---

What Phase 2 does next depends on that report:

| Post-upgrade result | Phase 2 proceeds as |
|---|---|
| ~~`rvol` replicates OOS~~ | ~~The full plan~~ — **not the outcome; see §2.1** |
| **Nothing replicates** | **Feature research only**: §3, §4 and §5.3 baselines. No multi-threshold display, no serving. Record the honest reading: the "+8–15% momentum entry" thesis shows no demonstrated edge on daily bars. That is a result worth recording, not a failure to hide. |
| `breakout`/`high52w` inversion replicates | They stay zeroed in v2. They enter the model as ordinary features (the model learns the sign) and feed the §8 risk research |

### 2.2 Feature research round 1 — pre-registered, and bounded

**ONE ROUND.** The hypotheses below are the complete list. They are written
before any of them is tested, with their targets, features, tests and
effect-size bars fixed.

**Why the bound exists.** "Feature research" is the phase where this protocol
is most likely to fail, because nothing stops it becoming a search for
something significant — and at n ≈ 7,000 episodes, something always is. This
project has already produced three positives that dissolved under
stratification (the v2 score, `rvol`, and `rvol` again inside ATR terciles). A
fourth is available to anyone willing to keep looking.

#### The hypotheses

Each is tested **bucket-stratified** (per bucket and CMH, never pooled —
§4.2.0 Rule 1), at **episode level**, under **purged walk-forward** (§4.1),
with the **lockbox untouched**, and reported **with and without the 2020 fold**
(§6.1). Each must clear §4.2.0 Rule 2's effect-size bar, not merely p < 0.05.

| # | Hypothesis | Target | Features | Test | Bar |
|---|---|---|---|---|---|
| **(a)** | A path-aware label is less volatility-dominated than touch-anytime, so existing features may separate on it where they do not on `hit_100` | `fp_100_dd50` and `fp_100_atr` (§5.1.1) | the §5.2 set | CMH by bucket x ATR tercile | MH OR ≥ 1.25, CI lower bound > 1.05 |
| **(b)** | A shorter horizon is closer to what a daily-bar signal could plausibly carry | **`hit_20_10s`: +20% within 10 sessions** | the §5.2 set | same | same, plus ≥ 3.0pp within bucket |
| ~~**(c)**~~ | ~~Catalyst via Tiingo News~~ — **ABANDONED 2026-09-22, before the round ran.** Tiingo News is a rolling ~3-month archive against Finnhub's ~12 (§3.5.1), so it would reduce catalyst coverage rather than extend it. Reported as abandoned with the reason, per this section's reporting obligation. **No substitute hypothesis is added** — the round is the four that remain. | — | — | — | — |
| **(d)** | Relative sector strength conditions which movers follow through | as (c) | `sector_strength_pct` (§3.4) | same | same |
| **(e)** | The 2020 anomaly suggests regime conditions the whole base rate | as (c) | SPY/IWM 20-day returns, VIX, joined as-of t-1 (§3.6) | same, **and** the regime split reported explicitly | same |

**(b) is proposed here rather than left open.** +20% within 10 sessions: short
enough that a daily-bar signal is plausible, long enough to clear typical
noise, and it produces far more positives than `hit_100`, so the folds are
better powered. If it turns out to have too few positives per fold under §5.1's
25-minimum, that is reported and the threshold is NOT adjusted to fix it.

#### STOPPING RULE

> **One round. If no hypothesis clears §4.2.0's effect-size bar out-of-sample,
> research STOPS. The screener is the product, and this spec records that as
> the finished result.**
>
> **No round 2 without a new written justification from the user.** Not from
> the agent, and not "one more variant while we are here".

What "stops" means concretely: §5 (models), §7 (multi-threshold display) and
§9 (shadow serving) are not built. The spec is closed with the honest reading —
**the "+8-15% momentum entry" thesis shows no demonstrated edge on daily
bars** — and that entry stands as the deliverable.

**Reporting obligation.** Every hypothesis is reported whether it clears or
not, with `n`, effect size and confidence interval before any p-value, and
Benjamini-Hochberg applied across the family of five. A hypothesis abandoned
mid-round is reported as abandoned, with the reason.

---

---

### 2.3 Round 1 results — LIVE, 2026-09-22

Run: `python3 scripts/research_round1.py` against `.work/data/cand_round1.csv`
(gate v2, `-scope eligible`, 4,969 symbols, episode gap 5). **7,579 episodes**
after excluding the §4.3 lockbox region and the in-sample pilot window.
16 walk-forward folds x 126 sessions, used as a stratum alongside bucket and
ATR tercile. **The lockbox was not read.**

Base rates: `hit_100` 9.47%, `fp_100_dd50` 9.29%, `fp_100_atr` 6.79%,
`hit_20_10s` 11.80%.

#### The table

| # | Pre-registered bar | Best effect, MH OR [95% CI] | Without 2020 | Per bucket | Verdict |
|---|---|---|---|---|---|
| **(a)** `fp_100_dd50` | OR ≥ 1.25, CI lower > 1.05, **and novel vs `hit_100`** | 1.178 [0.992, 1.399] | 1.288 | market / penny both flat | **FAIL** |
| **(a)** `fp_100_atr` | same | 1.195 [0.986, 1.449] | 1.263 | same | **FAIL** |
| **(b)** `hit_20_10s` | OR ≥ 1.25, CI > 1.05, **≥ 3.0pp** | 1.616 [1.389, 1.880] | 1.526 | market +14.90pp, penny +0.82pp | **PASS**, mechanically |
| **(d)** sector strength | OR ≥ 1.25, CI > 1.05 | 1.014 [0.830, 1.239] | 0.956 | flat both | **FAIL** |
| **(d)** sector strength on `fp` | same | 1.013 [0.828, 1.238] | 0.947 | flat both | **FAIL** |
| **(e)** SPY 20-day | same | 0.870 [0.733, 1.032] | 0.859 | flat both | **FAIL** |
| **(e)** IWM 20-day | same | 1.059 [0.893, 1.255] | 1.032 | flat both | **FAIL** |
| **(e)** VIX | same | 1.071 [0.903, 1.269] | 1.105 | flat both | **FAIL** |
| **(e)** VIX on `fp` | same | 1.081 [0.911, 1.283] | 1.128 | flat both | **FAIL** |

#### (a) fails on its own written terms, and the control is why

(a) was not "some feature clears the bar on a path-aware label". It was that
features "may separate on it **where they do not on `hit_100`**". That clause
is part of the pre-registered text, so the control is part of the test.
Running the identical test against `hit_100`:

| feature | `hit_100` | `fp_100_dd50` | `fp_100_atr` |
|---|---|---|---|
| `atr_pct` | **1.752** | 1.726 | 1.766 |
| `vwap_dist_pct` | **1.353** | 1.341 | 1.217 |
| `range_20` | 1.172 | 1.178 | 1.195 |
| `rvol_20` | 1.014 | 1.010 | 0.940 |

Every feature that clears the bar on a path-aware label clears it **just as
well, or better, on `hit_100`**. Nothing is novel to the new labels. The
premise — that touch-anytime is volatility-dominated in a way first-passage
is not — is **refuted**: both labels are dominated to the same degree. The
labels are still correct and worth keeping (they are honest about path), but
they revealed nothing `hit_100` had been hiding.

#### (b) clears its written bar, and the reason it should not count

(b)'s pre-registered bar contains no comparison clause, so `atr_pct` at
1.616 [1.389, 1.880] with +14.90pp in the market bucket clears it as written.
Recorded as PASS because that is what the rule says.

Two facts sit against it, and both were visible only after the control:

1. **The passing feature is the stratification variable.** The test is "CMH by
   bucket x **ATR tercile**", and the feature that passes is `atr_pct`. The
   strata control coarse volatility and the test then asks whether finer
   volatility within a tercile still separates. It does — but that is a
   statement about volatility, not about the shorter horizon. **This is a flaw
   in the pre-registration**: §5.2's feature set contains the stratifier, and
   nobody noticed when the rule was written. It is the same class of error as
   the §10.1.0 pooling flaw — the rule was written carefully and was still
   wrong in a way only the data exposed.

2. **The shorter horizon did not help.** `atr_pct` scores 1.752 on `hit_100`
   and 1.616 on `hit_20_10s`. (b)'s claim was that a 10-session target is
   closer to what a daily-bar signal can carry. It is slightly **worse**.

What the effect actually is, in raw rates (LIVE, same run):

| bucket | ATR half | n | `hit_100` | `fp_100_dd50` | `hit_20_10s` |
|---|---|---|---|---|---|
| market | low | 3,504 | 1.60% | 1.60% | 2.45% |
| market | high | 3,504 | 11.93% | 11.82% | 17.35% |
| penny | low | 286 | 34.97% | 33.57% | 34.62% |
| penny | high | 285 | 50.53% | 48.42% | 35.44% |

High-ATR stocks reach +100% more often than low-ATR stocks, on every label
alike. That is close to a restatement of what ATR measures, and it is not
something a scanner can act on: "volatile stocks move more" does not tell you
which ones, or when. It is the third appearance of the same confound that
already killed the v2 score and `rvol` twice.

#### Reading

**All four hypotheses fail substantively.** (a) by its own comparison clause,
(d) and (e) flat at OR ≈ 1.0, and (b) only via the stratification variable
behaving exactly as it does on the label (b) was meant to improve on.

Sector strength and regime deserve a specific note: both were plausible and
both are **flat**, not merely short of the bar. Sector strength at 1.013 and
VIX at 1.071 are the cleanest negative results in this project — they were
tested on 7,579 episodes with the confounds controlled, and there is nothing
there.

---

## 3. Data corrections

Each correction changes what the dataset measures. Each one is therefore
**versioned, migrated, and reported with a before/after measurement.** None may
be applied silently. Steps 3.2–3.6 are independent of each other and may run in
parallel.

### 3.1 Episodes

Consecutive gate-passing days of the same move share almost the same forward
label. Treating them as independent inflates `n` and makes p-values look
stronger than they are. A model trained on them also overweights long-running
moves.

```
episode = the first gate pass for a symbol after >= MOMENTUM_EPISODE_GAP_SESSIONS
          sessions with no gate pass                       (default 5, matching
                                                            the alert cooldown)
features and labels are taken from the episode's FIRST day,
because that is the day an alert would fire
```

Store episodes in a `momentum_episodes` table, not a view, so the set is stable
and can be manifested. Report how results change with a gap of 3, 5 and 10.
**All training and all evaluation in Phase 2 happen at the episode level.**

#### 3.1.1 Built 2026-09-21 (P2-1) — and the correction barely moves anything

Migration 016, populated by `scripts/build_episodes.py` from the
`momentum-backtest -dump-full` extract. All three gaps are stored side by side,
keyed by `gap_sessions`, so the sensitivity can be compared rather than
overwritten. **Every query must filter on `gap_sessions`** or it will union
three overlapping definitions of the same events.

| gap | episodes | rows/episode | median `gate_days` | max | single-day episodes |
|---|---|---|---|---|---|
| 3 | 8,976 | 1.07 | 1.0 | 6 | **94.1%** |
| 5 | 8,861 | 1.08 | 1.0 | 7 | **93.0%** |
| 10 | 8,688 | 1.10 | 1.0 | 8 | **91.4%** |

Base rate by gap and bucket:

| gap | market | penny |
|---|---|---|
| 3 | 10.40% (n=8,194) | 33.12% (n=782) |
| 5 | 10.28% (n=8,092) | 32.51% (n=769) |
| 10 | 9.97% (n=7,936) | 32.31% (n=752) |

**The result: the gap choice is nearly irrelevant on this dataset, and so is the
episode correction itself.** From 9,571 candidate rows to 8,861 episodes at
gap 5 is a **7.4% reduction**, base rates move by under half a point across
gaps 3 to 10, and the longest run of consecutive gate days in ten years is 8.

The reason is that **91-94% of gate passes are already isolated single days.**
§3.4's RVOL requirement makes a second consecutive pass unlikely: once a volume
surge is in the 20-day window it raises the baseline the next day has to beat.
The clustering this section was written to correct is mostly not present.

Two consequences, and they point in opposite directions:

1. **Episode level stays the default**, exactly as this section requires. It is
   free, it is the correct unit of inference, and being nearly identical to row
   level here is not a reason to prefer the wrong one. Gap 5 remains the
   default, matching the alert cooldown.
2. **It explains none of the Phase 1 findings.** Anyone hoping the row/episode
   distinction accounted for v2's composition artifact or rvol's shrinkage
   should stop: the post-upgrade report's row and episode results were already
   nearly identical (7,460 rows vs 6,924 episodes on B+C), and this measurement
   says why. **Recorded as a negative result** — a correction that was worth
   making and changed nothing.

### 3.2 Point-in-time shares outstanding — the forward-contamination problem

Phase 1 applied **today's** `shares_outstanding` to every historical row, and
listed this as a "point-in-time limitation." For a model it is worse than a
limitation: **it is lookahead.**

Today's share count differs from the count on the day of the setup, so applying
it to a historical row uses information from after that row. A model can then
learn a relationship between current float and past outcomes that exists only
because the future leaked into the feature.

**The DIRECTION of that leak was assumed and has now been measured — and the
measurement did not match the assumption.** This section originally argued that
small caps which run issue shares *into* the run (ATM offerings, secondaries),
making a past runner's current share count systematically inflated. On a
157-candidate sample (§3.2.1):

- median `today / as-of-setup` shares is **~1.03-1.06x and effectively identical
  for runners and non-runners**;
- the fat tail belongs to penny **non-runners** (p90 **16.49x**, a quarter above
  2x).

So in this sample dilution tracks **failure**, not success: a failing microcap
finances itself by issuing shares repeatedly. **Recorded as unverified** — n = 14
runners is far too small to settle it, and it is stated as an open question
rather than a refutation.

**Either direction is a leak, and the justification does not depend on which.**
If dilution tracks failure, a current-float feature learns "large float means
failure" from future information instead of "large float means success". The
feature is contaminated either way; only the sign of the contamination changes.

**The justification for this work is the flip rate, not the mechanism:** 20.1%
of historical bar-days carry a materially different market cap than the gates
used, and 12.1% of studied candidates do. That is measured, not assumed.

> ### Why this is P2's first build step (2026-09-21)
>
> **The coverage argument is withdrawn.** It was based on `shares_outstanding`
> coverage of 38.4%, which was a reporting-query bug; actual coverage is 98.1%
> (Phase 1 §10.1.2). Recorded because the priority decision was originally made
> on that number.
>
> **The real reason is larger than the float feature: the §3.2 gates select the
> candidate set itself using information from the future.**
>
> The gates test market cap. Market cap is today's value. It is applied to every
> historical row back to 2016. So membership of the candidate set — which
> historical setups the backtest ever looked at, and which bucket each landed in
> — was decided partly by what the company became:
>
> - A company worth **$1B in 2017** that has since grown to **$20B** is excluded
>   from the 2017 market bucket by today's cap, even though it satisfied the gate
>   on the day.
> - A company that has since **diluted or shrunk** may be included in historical
>   buckets it never qualified for.
>
> This is not a feature-level leak that can be quarantined by dropping a column
> from the model. **It conditions the sample.** Every result produced so far is
> computed on a candidate set chosen with a leaking variable — including the
> bucket-stratified tests in Phase 1 §10.1.4 and §10.1.5a, because the strata
> themselves are defined by today's market cap. Stratifying by a contaminated
> variable does not remove the contamination.
>
> The 98.1% coverage figure makes this **worse, not better**: the leak is
> universal rather than partial. Good coverage of a contaminated field means the
> contamination is applied everywhere.
>
> **Price-adjusted approximations do not fix it.** Adjusting today's share count
> backwards by the cumulative split factor corrects splits, and splits are not
> the problem. **Share issuance is the problem**, and issuance is precisely what
> a split adjustment cannot see. Small caps that run finance themselves into the
> run through ATM offerings and secondaries, so the error is systematic and
> points the same way for exactly the population the scanner targets.
>
> **First deliverable of this step is therefore a MEASUREMENT, not a pipeline:**
> how many historical gate decisions and bucket assignments flip when market cap
> is recomputed point-in-time. That number converts "known limitation" into a
> quantity, and it says how much of every result so far to discount. Build the
> full pipeline only after the flip rate is known.

**Fix: point-in-time shares from SEC EDGAR** (free, no key).

- Endpoint: the XBRL `companyfacts` API. Read `dei:EntityCommonStockSharesOutstanding`
  and take each fact's **`filed` date**.
- As-of join on `filed <= t`. **Never join on the period-end date.** The period
  end comes weeks before the filing, so joining on it is itself lookahead.
- Ticker-to-CIK mapping comes from SEC's company tickers file. That file covers
  **current** tickers only, so delisted names (§3.3) need a separate mapping.
- Requirements: a descriptive `User-Agent` header with contact details, and
  staying within SEC's fair-access rate (~10 requests/second).
- Verify **before building**: reachability from the dev network (remember the
  Yahoo IP block), coverage on a sample from the penny bucket, and how
  multi-class share structures are handled.
- New table: `shares_outstanding_pit` (`symbol`, `cik`, `filed_date`,
  `period_end`, `shares`, `form`, `source`).

#### 3.2.1 Verification and leak measurement (2026-09-21) — done BEFORE building

Steps (a) and (b) of the pre-build check. Harnesses: `scripts/edgar_verify.py`,
`scripts/measure_pit_leak.py`.

##### (a) Reachability — EDGAR works, but the User-Agent rule is a trap

**Reachable. The blocker was the contact domain, not the network or the rate.**

The first attempts returned HTTP 403 with bodies titled *"Request Rate Threshold
Exceeded"* and *"Your Request Originates from an Undeclared Automated Tool"* —
after about five requests. Both messages point at rate limiting or bot
detection. Neither was the cause:

| User-Agent | Result |
|---|---|
| `TradingAgentResearch <your-contact-email>` | **200** |
| `Trading Agent Research <your-contact-email>` | **200** |
| `TradingAgentResearch` (no email at all) | **200** |
| `TradingAgentResearch <a-noreply-github-address>` | **403** |

**SEC denylists `users.noreply.github.com` as a contact address.** The identical
UA with a real domain succeeds; a UA with no email at all succeeds. Record this
because the error text sends you to look at rate limits and IP blocks — the
egress here is a shared Zscaler proxy (a shared corporate proxy), which makes
an IP-level block the obvious and wrong hypothesis. **Use a real contact
domain.**

Pacing: the pipeline runs at ~3 req/s against SEC's documented ~10/s. Lower on
purpose, because the published rate is per-IP and this IP is shared with unknown
other users, so our share of it is not ours to assume.

##### (a) Coverage of `dei:EntityCommonStockSharesOutstanding`

Stratified sample of 100: 45 penny, 45 market, 10 known-delisted.

| Stratum | CIK resolved | with facts | no facts | avg facts | multi-class |
|---|---|---|---|---|---|
| penny | 45/45 | **34 (75.6%)** | 11 | 29.6 | 7 |
| market | 45/45 | **37 (82.2%)** | 8 | 46.6 | 7 |
| delisted | **0/10** | — | — | — | — |

- **Overall 71/90 (78.9%)** of current symbols expose the concept with `filed`
  dates. The 19 misses are mostly HTTP 404 on the concept (the company does not
  tag it) and one case where the concept exists with zero facts. So EDGAR is
  **not** a complete replacement for the Finnhub share count; it is a
  point-in-time source for about four fifths of the universe, and the rest need
  a documented fallback with the bias stated.
- **Multi-class structures are real: 14 of 71 (19.7%)** report several facts
  sharing one `filed` date. The leak measurement sums them, because the gate
  wants total common shares and taking one class understates the cap by the
  size of the others. This needs deciding explicitly in the pipeline, not
  inheriting from whichever fact happens to be first.
- **`company_tickers.json` contains no delisted names — 0 of 10 resolved**
  (SIVB, FRC, BBBY, YELL, RAD, WE, PRTY, TWTR, ATVI, VMW). Exactly as §3.3
  predicted. The file has 10,438 current tickers. Survivorship work needs a
  separate CIK mapping, and that is now a measured fact rather than an
  expectation.

##### (b) THE LEAK, MEASURED — one bar-day in five is in the wrong bucket

Recomputed `mcap_pit(t) = raw_close[t] × shares(filed <= t)` over 71 symbols and
**127,508 bar-days** with a raw close. As-of join on `filed`, never on period
end. A further 7,106 bar-days (5.3%) predate the symbol's first filing and are
reported as unmeasurable rather than assumed unchanged.

> **CORRECTION (2026-09-21): these tables measure the MARKET-CAP CLASS, not the
> bucket.** The §3.2 gate assigns the bucket from the close **price** (penny
> $0.30-$2.00, market >= $2.00) and then checks market cap against *that
> bucket's* band (penny <= $300M, market $300M-$10bn). A price is a price, so
> **the bucket does not move between gate v1 and v2 at all.** What moves is
> whether the cap passes the band check.
>
> The rows below therefore read as "the cap crossed a band edge", which is the
> *input* to the gate's cap check, not its verdict. The direction of the
> resulting flip depends on the price bucket: a cap moving from the $300M-$10bn
> range down below $300M makes a **market**-priced symbol fail and a
> **penny**-priced symbol pass. The two cannot be added together, and the
> original framing did exactly that.
>
> The gate-level consequence — candidate counts and pass/fail under v1 vs v2 —
> is measured through the Go gate itself in §3.2.2, because that is the only way
> to get the price bucket and the band check applied together and consistently
> with the live code. The magnitude below still stands as evidence that the leak
> is large: **one bar-day in five has a materially different market cap than the
> one the gates used.**

**Market-cap class over all evaluated bar-days**

| Today's cap (what was used) | Point-in-time | n | share |
|---|---|---|---|
| penny | penny | 40,172 | 31.51% |
| market | market | 39,440 | 30.93% |
| too_big | too_big | 22,318 | 17.50% |
| **market** | **penny** | **12,359** | **9.69%** |
| **penny** | **market** | **9,263** | **7.26%** |
| **too_big** | **market** | **3,196** | **2.51%** |
| **market** | **too_big** | **689** | **0.54%** |
| **penny** | **too_big** | **71** | **0.06%** |

**20.1% of historical bar-days change bucket.** And the direction is not a wash:
`too_big -> market` (2.51%) are days that today's cap **excluded from the
universe entirely** while the company was a legitimate market-bucket candidate
on the day. Those setups were never studied, and no amount of re-weighting
recovers them, because they are not in the dataset.

**Among candidates that actually passed the gates** (157 in this sample), again
as market-cap class rather than bucket:

| Today's cap | Point-in-time | n | share |
|---|---|---|---|
| market | market | 113 | 71.97% |
| penny | penny | 25 | 15.92% |
| **market** | **penny** | **15** | **9.55%** |
| **market** | **too_big** | **4** | **2.55%** |

**12.1% of studied candidates had a market cap in a different class than the
gates believed**, and 2.55% were above the $10bn ceiling on the day, so they
should not have been candidates at all.

**What this does and does not contaminate.** Being precise matters here, because
the first version of this section overstated it.

- **The stratification variable is NOT contaminated.** Buckets come from the
  close price, so the `penny` / `market` split used in Phase 1 §10.1.4 and
  §10.1.5a is computed from bar data with no lookahead. Those results —
  including the decomposition that exposed v2's composition artifact and the
  rvol CMH tests — are stratified on a clean variable. The earlier claim that
  they carried a "~12% stratum-assignment error" was **wrong** and is withdrawn.
- **The candidate set IS contaminated.** Which symbol-days passed §3.2 at all
  depended on today's market cap. So the *population* every result describes was
  selected with future information, even though the strata within it are sound.
  That is still a conditioning problem and still cannot be fixed by
  re-weighting.
- **Some setups are simply absent.** The cases above the ceiling on the day, and
  the `too_big -> market` cases that today's cap excluded, were never evaluated.
  No re-analysis recovers a row that is not in the dataset.

The practical reading: the existing results are **valid statements about a
biased sample**, not invalid statements. Gate v2 re-selects the sample; §3.2.2
reports how much it moves.

##### (b) Dilution — the expected mechanism did NOT appear

`today's shares / shares as of the setup`. Above 1 means shares were issued
after the setup, so today's cap overstates the cap on the day.

| Bucket | Group | n | median | p90 | share > 2x |
|---|---|---|---|---|---|
| penny | runners (hit +100%) | 5 | 1.05x | 1.95x | 0.0% |
| penny | non-runners | 20 | 1.06x | **16.49x** | **25.0%** |
| market | runners | 9 | 1.04x | 2.06x | 11.1% |
| market | non-runners | 123 | 1.03x | 1.35x | 6.5% |
| all | runners | 14 | 1.04x | 1.95x | 7.1% |
| all | non-runners | 143 | 1.03x | 1.66x | 9.1% |

**The hypothesis in §3.2 is that past runners diluted into the run, so their
share count today is systematically inflated. This sample does not show that.**
Median dilution is ~1.03-1.06x and effectively identical for runners and
non-runners, and the fat tail belongs to **penny non-runners** (p90 16.49x, a
quarter above 2x) — i.e. dilution here tracks *failure*, not success. The
plausible reading is the opposite of the one assumed: a failing microcap
finances itself by issuing shares repeatedly, so heavy dilution is the signature
of a company that did not run.

**This is badly underpowered and must not be over-read: n = 5 penny runners and
14 runners in total.** Recorded as a negative and as a genuine open question,
not as a refutation. It changes nothing about the fix — the 20.1% bucket flip
rate is the reason to do this work, and it stands on its own — but it does mean
the *mechanism* stated in §3.2 is an assumption rather than a measured fact, and
the ablation this section already requires should report it either way.

**What the flip rate does not tell us.** It measures how many gate decisions
change, not how much the hit rates change. The latter needs the full pipeline,
because it requires re-running the candidate selection and the labels over a
point-in-time universe, which is the build step this measurement was meant to
inform.

**Point-in-time market cap** is `raw_close[t] × shares_pit(filed <= t)`. **Both
factors must be unadjusted.** Multiplying an *adjusted* price by an
*unadjusted* share count is wrong by the cumulative split factor, which is
10–100× for reverse-split penny names. That is why the post-upgrade backfill
captures raw close and `splitFactor`. If it did not, re-fetch them before this
step.

**Until 3.2 lands:**
- `float_shares_est` and `market_cap_est` are **excluded from model features**.
  They stay in the gates, with the bias documented.
- Run an **ablation**: train with and without the current-shares features and
  report the difference. Expect the current-shares version to look more
  predictive. The size of that gap is a measure of the leak.

**Once 3.2 lands:** gate v2 uses point-in-time market cap. Report how many
historical rows change bucket or gate status. This is a change to Phase 1 §3.2,
so it gets its own version and its own entry in that spec.

### 3.2.2 Gate v1 vs v2 on the full universe — 42% of the sample changes

Both runs through the Go gate itself (`momentum-backtest -gate-version 1|2`),
full eligible universe, 10 years, identical code except the market-cap
definition.

| | gate v1 (today's cap) | gate v2 (point-in-time) |
|---|---|---|
| symbols scored | 4,462 | 4,462 |
| §3.2 gate passes | 10,671 | 10,072 |
| evaluable candidates | **9,571** | **9,119** |
| bucket mix | market 91.1% / penny 8.9% | market 88.0% / **penny 12.0%** |
| base rate | 13.21% | 12.76% |

**The net change of −452 candidates is the least interesting number here.**

| | rows | share |
|---|---|---|
| in BOTH gate versions | 6,839 | |
| **only v1** — studied, but should not have been | **2,732** | **28.5% of v1** |
| **only v2** — eligible on the day, never studied | **2,280** | **25.0% of v2** |
| **total churn** | **5,012** | **42.3% of the union** |

**The net figure understates the churn by 11x.** Better than a quarter of
everything Phase 1 studied was not eligible on the day, and a quarter of what
was eligible had never been looked at. The two errors nearly cancel in the
totals and do not cancel at all in the sample.

Rejection-reason shifts corroborate the mechanism: `market_cap_above_max` falls
from 1,700,375 to 1,112,971 symbol-days — companies that are large today were
smaller then — and the new `market_cap_pit_unavailable` accounts for 1,040,397.

### 3.2.3 Who EDGAR does not cover, and whether excluding them biases the sample

Full backfill: **151,416 point-in-time rows across 4,076 symbols**, 630 with
multi-class structures. Coverage of the 4,952 eligible symbols attempted:

| Status | n | share |
|---|---|---|
| facts found | **4,053** | **81.8%** |
| HTTP 404 (concept not tagged) | 796 | 16.1% |
| concept present, no facts | 88 | 1.8% |
| no CIK in `company_tickers.json` | 11 | 0.2% |
| transport error | 4 | 0.1% |

**They differ systematically, and the dominant axis is listing age, not domicile.**

| | covered (4,053) | NOT covered (899) |
|---|---|---|
| mean bars of history | 1,870 | **1,114** |
| listed after 2023-09 | 16.2% | **48.7%** |
| priced under $2.00 | 16.4% | 12.3% |
| market cap under $300M | 38.2% | 31.9% |

**Uncovered symbols are three times more likely to be recent listings.**
Excluding them therefore biases the gate-v2 sample **against recent listings**,
which is the opposite direction from the survivorship bias in §3.3 and does not
offset it. Recorded rather than corrected.

On a 25-symbol sample of the 404s, annual forms were **13 x 10-K, 5 x 20-F, 1 x
6-K**, the rest registration-only. So **foreign private issuers are
over-represented (~24% against a few percent of the universe) but are NOT the
main cause** — most uncovered names are ordinary domestic 10-K filers, including
large established ones.

**A fallback concept would recover much of this.** `dei:EntityCommonStockSharesOutstanding`
is a cover-page tag that not every filer uses. Spot-checking three uncovered
large filers:

| Concept | resolves |
|---|---|
| `dei:EntityCommonStockSharesOutstanding` | 2/3 |
| **`us-gaap:CommonStockSharesOutstanding`** | **3/3** |
| `us-gaap:CommonStockSharesIssued` | 2/3 |

**Not implemented, because it is a measurement change, not a bug fix.** The
`us-gaap` concept is a balance-sheet figure at period end and is reported per
class, so it is a *different* quantity from the cover-page count and would need
its own as-of handling. Flagged as the highest-value next step for coverage, to
be decided explicitly rather than folded in.

### 3.2.3a The us-gaap fallback, and what stays structurally uncoverable

**Fallback applied 2026-09-21**, only where `dei:EntityCommonStockSharesOutstanding`
is absent for the symbol (enforced by `--only-missing`, so it can never
overwrite a primary-concept row).

| | symbols | coverage |
|---|---|---|
| via `dei` (primary) | 4,076 | 81.9% |
| via `us-gaap` (fallback) | **+218** | |
| **total** | **4,294** | **86.3%** |

Coverage gain by listing age — the fallback helps recent listings
proportionally more, which is the direction the bias needed:

| Group | eligible | before | after |
|---|---|---|---|
| listed after 2023-09 | 1,100 | 60.2% | **66.4%** |
| listed before | 3,871 | 88.2% | **92.0%** |

**The fallback is a staler measurement, and the data says by how much.** As-of
lag (`filed_date - period_end`):

| Concept | mean lag | median lag |
|---|---|---|
| `dei:EntityCommonStockSharesOutstanding` | 12 days | **6 days** |
| `us-gaap:CommonStockSharesOutstanding` | 91 days | **43 days** |

That is expected — the us-gaap figure describes the period end, the dei figure a
date near the filing — and it is why `concept` is stored per row (migration
019). Neither is lookahead: both are joined as-of on `filed_date`. But a
fallback row's share count is a month and a half older than a primary row filed
the same day, and any result that moves when the fallback is enabled must be
checked against this column first.

#### Post-IPO months are structurally uncoverable from EDGAR — a known bias

**A newly listed company has no XBRL filing until its first periodic report,
which arrives weeks to months after the IPO. For that window there is no
point-in-time share count at any price, from any concept.** Gate v2 rejects
those symbol-days with `market_cap_pit_unavailable`, and no fallback fixes it:
the data does not exist yet.

**This is a known bias toward excluding new listings**, and it is recorded
rather than corrected. It is not the same as the coverage gap the fallback
addresses — that was filers who never used the cover-page tag; this is filers
who have not yet filed at all.

Two things bound how much it matters here, and they should be read together:

1. **Recent listings barely reach the candidate set anyway.** §3.1's 252-bar
   history minimum plus the 120-session label horizon mean a symbol listed after
   2023-09 cannot produce a complete-label candidate until roughly 2025-03.
   Measured: symbols listed after 2023-09 supply **173 of 9,571 gate-v1
   candidate rows (1.81%)**, and **95% of those fall inside the lockbox window**
   — leaving about **9 rows** in the evaluable set. So for the current
   evaluation the exclusion is close to inert.
2. **That will not stay true.** The bound above is an artifact of a 10-year
   backfill whose recent end is still maturing. As the history rolls forward,
   post-IPO months become a growing share of the candidate set, and this bias
   grows with them. Phase 3 and any live deployment inherit it directly, since
   the live scanner faces exactly the names with no filing history.

### 3.2.4 Multi-class handling — the decision, recorded

**Classes are SUMMED, priced at the traded class's raw close, and the symbol is
flagged `multi_class = true`** (630 of 4,076 symbols, 15.5%).

Summing all classes and multiplying by one class's price is the standard
market-cap approximation, and it is an approximation: non-traded or
differently-voting classes need not be worth the traded class's price. The
alternative — using only the traded class — understates the cap by the size of
the others, which is worse and is wrong in a consistent direction.

The flag exists so the choice is auditable and so §3.2's sensitivity check can
re-run excluding these symbols, rather than the decision being invisible inside
a number.

### 3.3 Survivorship

Today's universe excludes every company that delisted, and delisted companies
are disproportionately failures. Phase 1 could only *describe* this bias. Phase 2
tries to *measure* it.

#### 3.3.-1 The magnitude, stated plainly

Survivorship has been described in this project as "a known limitation" for
long enough that the size of it should be on the page.

LIVE, from `supported_tickers.csv`:

| | count |
|---|---|
| US common stocks **delisted within the backtest window** (endDate >= 2016-09-21) | **11,340** |
| live eligible universe actually studied | **4,975** |
| implied real universe over the window | ~16,315 |
| **share of it we studied** | **~30%** |

**Across the backtest window, the studied universe is a minority of the real
one, and the ~70% that is missing consists, by construction, of companies that
died.** Delistings run 900–1,800 a year through the window; 2023 alone
contributes 1,768.

One qualification, so the number is not over-read: the 11,340 are US common
stocks by Tiingo's classification and have not been through §3.1's eligibility
filters (price floor, liquidity, exchange). Some would have been excluded
anyway. But §3.1 excludes on liquidity and price, which failing companies fail
*more* often, so filtering would remove them unevenly and the direction of the
bias is unchanged.

##### What this implies for the `rvol` result

Worth stating because the two findings are usually discussed separately.

The measured base rate — ~10% of candidates reaching +100% — is computed on a
sample that **omits most of the failures**. A survivor-only sample inflates hit
rates: the companies that went to zero are not in the denominator. So the true
base rate over the real universe is lower than anything reported here, by an
unmeasured amount.

**And `rvol`'s edge already vanished inside that inflated sample** (MH OR 0.991,
p = 0.947, Phase 1 §10.1.5b). The most plausible reading is therefore that the
true picture is **no better than "no edge"**, and quite possibly worse: a
signal that cannot separate outcomes among survivors has no obvious reason to
separate them once the failures are restored.

**That reading is not established, and two things could in principle overturn
it.** The bias is unmeasured (§3.3.1: unmeasurable with this provider
combination), and its *interaction* with `rvol` is unknown — it is conceivable,
though there is no evidence for it, that high-RVOL moves in companies that
later delisted behaved differently in a way that would restore separation.
Recorded as the plausible reading rather than a conclusion, and it is the
reading Phase 2 §1 already assumes when it says "no edge on daily bars" is a
realistic outcome.

---

#### 3.3.0 Acceptance rule for the delisted route — written 2026-09-22, BEFORE the numbers

Committed in advance, for the same reason §2.1's routing rule was: the
temptation here is to accept whatever coverage exists and apply a correction
with it, and that temptation is strongest after seeing an encouraging number.

**A new bias is already known to be in play.** Tiingo silently stops serving
back history for some symbols — verified on two currently-listed names, `HWH`
(0 bars for an 11-year window) and `JCSE` (1 bar), both of which we still hold
hundreds of stored bars for. If the provider drops history for *listed* names,
it will certainly drop it for delisted ones, and **the dead companies it kept
may differ systematically from the ones it dropped.** A survivorship
correction built on the kept subset would then be a second selection bias
layered on the first, presented as a fix.

> **Acceptance rule.**
>
> 1. **Usable** — delisted names resolve, a representative share return full
>    history, AND the served/unserved groups do **not** differ materially on
>    listing age, delisting year, last price, size or bucket. Then §3.3's
>    base-rate comparison proceeds.
> 2. **Unmeasurable** — coverage is partial AND the served group differs
>    materially from the unserved one. Then **record survivorship as
>    unmeasurable with this provider, and apply NO partial correction.**
>
> "Materially" is judged on the reported distributions, not on a p-value: with
> thousands of delisted names almost any difference is significant, and the
> question is whether the served subset is *representative*, not whether the
> difference is detectable.

**Outcome 2 is an acceptable result, not a failure.** It is strictly better
than a correction computed on a biased subset, because a wrong correction is
worse than a documented gap: the gap stays visible in every later report,
while the correction silently propagates into base rates that then look
trustworthy. §12's "no edge is a valid deliverable" applies to data corrections
as much as to models.

**What is reported either way** (verification only — build nothing yet):

| | Question |
|---|---|
| (a) | how many delisted names exist in `supported_tickers` |
| (b) | how many return full history on Power |
| (c) | how the served differ from the unserved: listing age, delisting year, last price/size, bucket, and delisting reason if obtainable |
| (d) | ticker-reuse cases found |
| (e) | CIK route coverage for delisted names |

---

#### 3.3.1 Verification RESULT (2026-09-22) — the blocker is the CIK route, not Tiingo

LIVE. Sources: `supported_tickers.csv` (108,778 rows),
`scripts/p23_delisted_verify.py` (stratified sample, seed 20260922),
SEC `browse-edgar`.

**(a) Delisted names exist, in quantity.** 36,547 US common stocks priced in
USD; **12,554 with an `endDate` before 2026-09-01**. Delistings per year run
~500–1,800 across 2014–2026. So the list itself is not the constraint.

**(b) Tiingo serves most of them.** Stratified sample of 154 across 23
delisting years: **133 served (86.4%)**, 21 not (13.6%, HTTP 200 with an empty
array). Served names return their full listed span — e.g. `KNDL` 3,500 bars
1997–2011, `SRSL` 3,997 bars 1996–2012. This is *better* coverage than the
HWH/JCSE cases suggested.

**(c) The served look broadly representative — on the attributes that can be
compared.**

| | served (133) | unserved (21) |
|---|---|---|
| median years listed | 4 | 4 |
| median delisting year | 2017 | **2013** |
| NASDAQ / NYSE / PINK | 42.1% / 36.8% / 20.3% | 42.9% / 42.9% / 14.3% |

No exchange category differs by more than 15pp. Listing age is identical. The
one real skew is age of delisting: unserved names are older, which is the
expected direction and modest at n = 21.

**The important caveat is a measurement limit, not a result.** Last price,
market cap and bucket **cannot be compared**, because they are only observable
for names Tiingo serves. The very attributes that matter most for §3.2
bucketing are unobservable in the group we would need them for. So (c) is
answered for listing age, delisting year and exchange, and is *unanswerable*
for size — and that gap cannot be closed with this provider.

**(d) Ticker reuse is real and unresolvable from this file.** 924 tickers carry
more than one listing row; **468 have strictly disjoint date ranges**, i.e. a
different company reusing the symbol. Example: `ADCT` is NYSE 2001-01-02 to
2010-12-20 (ADC Telecommunications) and again NYSE 2020-05-15 to 2026-09-21
(ADC Therapeutics) — unrelated companies, one ticker.

`supported_tickers` columns are `ticker, exchange, assetType, priceCurrency,
startDate, endDate`. **There is no CIK, no CUSIP, no permaticker — no stable
company identifier of any kind.** Reused tickers can only be split on the date
ranges, exactly as §3.3 anticipated.

**(e) The CIK route for delisted names fails, and fails dangerously.**

`company_tickers.json` and `company_tickers_exchange.json` are current-listing
only (confirmed earlier: 0 of 10 known-delisted names resolved).
`cik-lookup-data.txt` has 1,060,036 entries including former names, but it maps
**company NAME to CIK** — and `supported_tickers` supplies no company name, so
it cannot bridge.

Testing `browse-edgar` by ticker on six delisted names: three returned no CIK,
two were rate-limited (HTTP 503), and one resolved —

```
EM   ->  CIK 0001834253  "Smart Share Global Ltd"
```

`EM` delisted in **2012**. Smart Share Global is the company holding that
ticker **now**. **A ticker→CIK lookup for a delisted name returns whoever owns
the symbol today**, silently and with a 200. That is the ticker-reuse hazard
reappearing inside the identifier route meant to resolve it — the worst
possible place for it, because the result looks like a successful lookup.

#### Verdict against the §3.3.0 acceptance rule

**The rule's two outcomes were framed around Tiingo coverage, and Tiingo
coverage passes. The route fails for a different reason, upstream of it.**

Gate v2 (Phase 1 §3.2) requires point-in-time market cap, which requires
`shares_outstanding_pit`, which requires **a CIK**. Delisted names cannot be
resolved to a CIK reliably or safely. So a delisted name added to the universe
would fail gate v2 with `market_cap_pit_unavailable` on every historical bar —
it could never become a candidate, and the base-rate comparison §3.3 exists to
run would compare the live universe against an empty set.

Therefore, and per the rule's spirit:

> **Survivorship is UNMEASURABLE with this provider combination, and no partial
> correction is applied.**

Recorded with the specific reason, because it is narrower than "we could not
get the data" and may be fixable later:

- **Prices for delisted names: available** (86.4%, representative on observable
  attributes). Not the blocker.
- **Identity for delisted names: unavailable.** No stable ID in
  `supported_tickers`, no ticker→CIK route that is correct for dead tickers,
  and a lookup that returns the wrong company without erroring.

**What would unblock it** — recorded so the next attempt starts here rather
than repeating this: a source mapping historical ticker + date range to a
permanent identifier. Candidates are SEC's financial-statement data sets
(quarterly ZIPs carrying CIK with period coverage), or a paid reference dataset
with permatickers. Both are out of Phase 2's "no new paid services" scope for
the second; the first is free and untested.

**Consequence for every base rate already reported:** unchanged, and still
uncorrected for survivorship. §2.3's instruction stands — hit rates continue to
be reported by calendar year so the bias stays visible, since it cannot be
removed.

---

Tiingo publishes a daily ticker list (`supported_tickers`) with start and end
dates. Verify, in this order:

1. Delisted common stocks appear in the list.
2. Tiingo serves price history for them on the Power plan.
3. **Ticker reuse** is handled. A ticker recycled by a new company must never be
   stitched onto the old company's history. Find out what stable identifier
   Tiingo exposes. If it exposes none, split the series at the listing
   discontinuity and confirm the split with bar-audit.

If all three verify:
- Add delisted common stocks with `listed_at` / `delisted_at` columns.
- Evaluate eligibility **as of `t`**: the stock was listed and met §3.1 on that
  date. Finnhub's metadata does not cover delisted names, so use Tiingo's
  asset-type and exchange fields.
- When a delisting falls inside the label window, compute the label from the
  bars that exist and set `label_truncated_by_delisting`. **Do not drop these
  rows.** Dropping them brings the bias straight back.
- Report the base rate with and without delisted names, per bucket. The size of
  the drop is Phase 1's survivorship bias, finally measured.

If verification fails: record the bias as unmeasured, and keep reporting hit
rates by calendar year. Older years should look more inflated.

### 3.4 Sector

`/stock/profile2` is **already called** in the fundamentals metrics claim (for
`shareOutstanding`), so reading `finnhubIndustry` from the same response is
effectively free. Phase 1 §3.13's cost argument no longer applies. This makes
`sector_strength_pct` (Phase 1 §3.12) computable, and it enters the model as a
candidate feature.

Note that `finnhubIndustry` is the **current** classification. That is a mild
point-in-time issue, far less serious than the share-count problem in 3.2.

#### 3.4.1 P2-4 RESULT (2026-09-22) — sector captured, at zero extra cost

LIVE, `equity_fundamentals` where `metric='sector_profile'`,
`source='finnhub_profile2'`.

| | symbols | share |
|---|---|---|
| eligible universe | 4,975 | |
| profile row returned | 4,575 | 92.0% |
| **usable industry** | **4,303** | **86.5%** |
| explicit `N/A` from Finnhub | 272 | 5.5% |
| no profile row at all | 400 | 8.0% |

Per bucket:

| Bucket | symbols | usable | share | distinct industries |
|---|---|---|---|---|
| market | 3,056 | 2,749 | **90.0%** | 45 |
| penny | 1,809 | 1,536 | **84.9%** | 44 |

**Cost: zero additional requests.** `finnhubIndustry` comes from the same
`/stock/profile2` response already fetched for `shareOutstanding`, which is
what §3.4 meant by the sector data being effectively free and why Phase 1
§3.13's cost objection lapses.

Three properties of this field that constrain how it may be used:

1. **It is a CURRENT classification, not point-in-time.** A company
   reclassified since the setup carries today's label on every historical row.
   That is a mild version of the §3.2 problem — mild because industry changes
   are rare and, unlike share count, are not systematically related to whether
   the stock ran. Recorded, not corrected.
2. **Finnhub exposes ONE level**, not a sector/industry pair. The payload
   carries the same value under both keys with a note saying so, rather than
   inventing a hierarchy.
3. **Missing is stored as missing.** 272 explicit `N/A` and 400 absent rows
   are distinguishable from each other and from a real industry. §5.2 requires
   an explicit indicator column and forbids imputation; `sector_strength_pct`
   must therefore be null for those 672 symbols, never a universe average.

An ordering trap worth recording: the capture had to be written BEFORE the
`shareOutstanding` early-returns in the same function. That path returns early
when the profile has no share count and when `/stock/metric` already supplied
one — both common — so capturing the industry afterwards would have collected
it only for the minority needing §3.9's fallback.

### 3.5 Catalyst history via Tiingo News

Finnhub's free news reaches back only ~12 months, so catalyst could be evaluated
on just 80 of 218 Phase 1 candidates. Tiingo News is included in Power.

#### 3.5.1 Verification RESULT (2026-09-22) — the depth condition FAILS

LIVE, `scripts/p25_news_verify.py` plus direct probes.

**§3.5's build condition is "if the history is deeper than Finnhub's". It is
not. It is roughly a quarter as deep.**

**(1) History depth: a rolling ~3-month window, for every symbol.**

| Symbol | articles in archive | span |
|---|---|---|
| CHCI | 144 (ends at offset 150) | 2026-06-24 .. 2026-09-22 |
| WKHS | 38 | 2026-07-02 .. 2026-09-21 |
| VRME | 15 | 2026-07-31 .. 2026-09-21 |
| AAPL | archive ends between offset 5,000 and 10,000 | deepest reached 2026-07-10 |

Size makes no difference — AAPL bottoms out at the same ~2.5-3 months as a
micro-cap. Finnhub's free news reaches ~12 months, so Tiingo News is
**shallower than the source it was meant to replace**.

**`startDate` / `endDate` are silently ignored on this endpoint.** Requesting
2017-01-01..2017-03-31 returns 100 articles dated 2026-06-23..2026-06-24 — zero
inside the window, HTTP 200, no error. Depth has to be probed by `offset`
paging instead, which is how the numbers above were obtained. Recorded because
a backfill written against the documented date parameters would appear to work
and would silently store recent articles under historical dates — a
lookahead-shaped defect with no error to catch it.

**(2) Ticker tagging on micro-caps is decent**, which makes the depth result
the more frustrating. Sample of 25 penny-bucket symbols: **24/25 (96%) have at
least one article**, 569 articles total, and the queried symbol is the
first-listed ticker on **73%** of them. Tagging is not the obstacle.

The 27% where the symbol is not first-listed are a real caveat rather than
noise: an AAPL-tagged article in the sample was about Peloton's treadmills.
Any keyword classifier would need `tickers[0]` or a relevance filter, not bare
membership.

**(3) Timestamp precision is sufficient.** `publishedDate` is second-precision
UTC (`2026-09-22T11:00:01Z`), with a separate microsecond `crawlDate`. The
§3.5 as-of cutoff — compare against the scan time on day `t` — is expressible
exactly. Note `crawlDate` runs 2-8 minutes after `publishedDate`, so the cutoff
must use **publishedDate**; using crawlDate would discard articles that were
public before the scan.

**(4) Ticker reuse: not cleared, and not clearable here.** 466 of the 468
reused tickers have their current listing starting inside the news window. Of 7
with articles, **none** had an article predating the current listing — but the
archive is only ~3 months deep, so essentially every article postdates every
listing start. **This test cannot fail given the depth**, and its passing is
therefore no evidence. Recorded as untested rather than clean.

**(5) The as-of cutoff** is implementable identically for history and live,
since only `publishedDate` is needed — but with a 3-month archive there is no
history to apply it to.

#### Verdict

**Do not build the §3.5 backfill.** The condition it was gated on is not met:
Tiingo News is ~3 months against Finnhub's ~12, so it would *reduce* catalyst
coverage, not extend it.

Consequences, recorded rather than worked around:

- **§3.5 is not built.** The keyword-family storage and `news_coverage`
  indicator it describes remain unimplemented.
- **Research round 1 hypothesis (c) — "catalyst via Tiingo News" — cannot run
  as written** (§2.2). Phase 1 already found catalyst underpowered at n = 18
  with Finnhub's 12 months; 3 months would be worse. Per §2.2's reporting
  obligation this is reported as **abandoned, with the reason**, not silently
  dropped, and it does **not** license a substitute hypothesis — the round is
  the four that remain.
- Catalyst stays where Phase 1 left it: **unresolved and underpowered**, with
  no route to resolving it from the sources currently paid for.

---

Verify first: how far back the history goes, how well micro-caps are tagged by
ticker, and timestamp precision. If the history is deeper than Finnhub's:

- Backfill news for **episode starts only**, not every row.
- Cache raw headlines exactly as `catalyst-backfill -cache` does today.
- **As-of cutoff:** use only articles published before the daily scan time on
  day `t`, and apply the same cutoff to historical rows. An article at 16:05 ET
  is visible to a 17:00 ET scan; an article at 09:00 ET on `t+1` is not.
- Keep the keyword classifier, but store **per-keyword-family indicators**, not
  only the A/B/none tier. Phase 1 found that the assumed tier ordering is
  unsupported.
- Add an explicit `news_coverage` indicator, so that "we had no news data" is
  never read as "there was no news."

### 3.6 Market regime features

Regime inputs such as the SPY and IWM 20-day returns and the VIX level are
already ingested (the equity tables and `macro_fred`). Join them **as of `t-1`**
to allow for publication lag; FRED series can post after the close.

---

## 4. Validation protocol

This section decides whether anything else in Phase 2 can be believed. It is
designed so that Phase 1's repeated in-sample revisions cannot happen again.

### 4.1 Purged walk-forward

- **Expanding** training window. Test windows of **126 sessions**, stepping
  forward 126 sessions at a time.
- **Purge:** remove from training every episode whose label window
  `[t+1, t+120]` overlaps the test window. Without this, a training label
  computed from bars inside the test period leaks test information into
  training.
- There is no training data after a test window, so no post-test embargo is
  needed. This is stated here so that nobody "fixes" it by adding shuffled folds.
- **Never** use random splits, shuffled k-fold, or random splits by symbol.
  Symbols on the same dates share one market regime, so a random symbol split
  still leaks.
- Tune hyperparameters with an **inner** walk-forward inside each training
  window only.
- Report positive counts per fold. A fold with fewer than 25 positives for a
  given threshold is excluded from that threshold's evaluation, and the
  exclusion is shown in the report.

### 4.2 Pre-registration

#### 4.2.0 Two rules that override everything else in this section

Both come from the post-upgrade validation, where the pre-registration itself —
not the analysis — was what failed. Phase 1 §10.1.0 records the ruling.

##### Rule 1: every test is BUCKET-STRATIFIED. Pooling across buckets is forbidden.

**A test pooled across the penny and market buckets may never be used as
acceptance evidence.** Not as a headline, not as a tiebreaker, not "for
comparison with Phase 1" if a decision rests on it.

This is not a preference. The post-upgrade report's criterion 1 was a pooled
B+C test, score v2 passed it at p = 0.017, and the entire effect was bucket
composition: v2 separated in **neither** bucket (penny p = 0.891, market
p = 0.645), while its top third held 9.2% penny names against the bottom
third's 1.6%. Penny names hit at 35% and market names at 9%, so shifting the
mix produced 1.97pp of the 2.04pp observed — **97%** — and left 0.07pp of
genuine within-bucket ranking on 6,924 episodes.

A pooled test cannot distinguish "ranks candidates well" from "picks more
candidates from the higher-base-rate stratum". Since the score is **computed and
thresholded per bucket**, the pooled quantity does not even correspond to how
the product behaves.

Acceptable forms:

1. **Per bucket, reported separately.** Always required. If a result holds in
   one bucket only, it ships for that bucket only (§6.3).
2. **Cochran-Mantel-Haenszel**, stratified by bucket, for a single pooled-power
   statistic. CMH combines evidence *across* strata while never comparing
   *between* them, which is exactly the property the naive pooled test lacks.
   Report the CMH statistic, its p-value, **and the Mantel-Haenszel common odds
   ratio**.
3. **Stratify by bucket x volatility tercile** where n allows. Volatility is a
   second composition axis and it is entangled with bucket — in the post-upgrade
   data 329 of 362 penny episodes sat in the top `atr_pct` tercile, so "control
   for bucket" is not the same as "control for volatility".

Always report the **crude and stratified estimates side by side**. The gap is
the size of the confound, and it is informative in its own right: `rvol_20`'s
excess odds fell from 0.529 crude to 0.192 after bucket and volatility, so 64%
of the apparent effect was composition even for a signal that genuinely
survived.

A worked application is Phase 1 §10.1.5a; the harness is
`scripts/rvol_stratification_check.py`. Note that every split there is computed
**within** its stratum — splitting globally and then tallying per stratum
reintroduces the confound being tested.

##### Rule 2: pre-register a MINIMUM EFFECT SIZE, not just p < 0.05.

**Every acceptance criterion must state a minimum effect size, and a result
that clears p < 0.05 but misses the effect-size floor is NOT accepted.**

At the post-upgrade sample size all seven v2 components cleared p < 0.05,
including three that amount to "volatile, small, beaten-down stocks move more".
A p-value answers "is this distinguishable from zero", and at n ~ 7,000 almost
everything is. It does not answer "is this large enough to act on", which is
the only question that matters for a threshold that decides which ~10% of
candidates to alert on.

Defaults, finalised in the pre-registration:

| Quantity | Floor |
|---|---|
| Hit-rate difference at matched alert volume | **>= 3.0pp** absolute, **within each bucket** |
| Relative improvement over the best baseline | **>= 1.25x** |
| MH common odds ratio (stratified) | **>= 1.25**, with a bootstrap 90% CI lower bound **> 1.05** |

State `n`, the point estimate, **and a confidence interval** before any
p-value. A confidence interval reports the effect size and the uncertainty
together, which is what the floor is checked against.

For calibration this floor is inverted: ECE has a **maximum** (§6.2), because
there the good result is a small number.

#### 4.2.1 Pre-registered criteria for the Phase 1 post-upgrade OOS report

**Written before the report was run, and before any of its numbers were seen.**
That ordering is the entire point: Phase 1's score was revised three times
against the same 218 candidates, each revision defensible on its own, and the
result was still in-sample. Criteria added after seeing an outcome are not
criteria, they are a description of the outcome.

Nothing below was added, removed or reworded after the first number appeared.
Amendments, if any, must be new dated entries stating the reason, with results
reported under both the original and the amended plan.

| # | Claim | Criterion for "confirmed" |
|---|---|---|
| 1 | **v2 confirmed out-of-sample** | B+C, **episode level**, top-third hit rate > bottom-third, **p < 0.05** |
| 2 | **`rvol` replicates** | Same direction as Phase 1 (higher RVOL -> higher hit rate), **p < 0.05**, episode level |
| 3 | **`breakout`/`high52w` inversion replicates** | Negative direction, **p < 0.05** |
| 4 | **Any other component** | "Resolved" only at **p < 0.05**. Otherwise it stays **unresolved**, whatever its point estimate |

Criterion 4 is the one that costs something. A component with a large,
encouraging point estimate and p = 0.2 is unresolved, and must be written down
as unresolved. Phase 1's catalyst tier is the precedent: an apparently ordered
set of lift numbers over n = 18 that meant nothing.

**Multiplicity.** Seven components are tested per population. Benjamini-Hochberg
is applied across that family, and both the raw and BH-adjusted verdicts are
reported. Seven independent tests at p < 0.05 produce at least one false
positive about 30% of the time, which is roughly the rate at which a
"discovery" would be manufactured by running this report alone.

**Populations, never pooled.** A (pilot symbols, 2023-09 onward) is in-sample
and is shown for reference only. B (new symbols, the original window) and C (all
symbols, before 2023-09) are out-of-sample. **B+C is the headline.** The lockbox
is excluded from all of them.

**Episode level is authoritative.** An episode is a symbol's first gate pass
after >= 5 sessions with none. Consecutive gate-passing days of one move share
nearly the same forward label, so row-level `n` counts the same event many times
and makes every p-value look stronger than the evidence. Row level is reported
too, only because it is what Phase 1 reported and the two must be comparable.

#### 4.2.2 Pre-registration for the models


Before the **first** model fit, commit `docs/phase2_preregistration.md`. It must
state:

- the feature list and model families;
- the hyperparameter grids;
- the metrics and the §6 acceptance criteria;
- the baselines;
- the method for enforcing cross-threshold consistency (§5.5).

Amendments are allowed only as new dated entries with a stated reason. Results
must then be reported under **both** the original and the amended plan. Git
history is the audit trail.

### 4.3 The lockbox

Reserved during the post-upgrade validation, **before any evaluation of the
widened dataset was run.**

```
date range:         2025-03-28 .. 2026-03-27
symbols:            1,037  (excludes the 450 pilot symbols)
episodes / rows:    1,626 / 1,783
manifest table:     phase2_lockbox (symbol, ts, bucket)
content hash:       104cce0836a12af00242b31de3df5b53175febf8b5cf1024e40f74792feb495c
                    (sha256 over the sorted "symbol,ts" member keys)
reserved on:        2026-09-21
migration:          014_phase2_lockbox.sql
reserved by:        scripts/reserve_phase2_lockbox.py
```

**Definition as applied:** complete labels, AND date > 2025-03-27 (the most
recent 12 months of complete-label dates, which end 2026-03-27), AND symbol not
in `momentum_pilot_cohort`.

Notes on the reservation:

- The script reads only `symbol`, `date` and `bucket` from the candidate
  extract. It never opens an outcome column, so the boundary cannot have been
  placed with any knowledge of what is inside it.
- It refuses to run if `phase2_lockbox` is already populated. Re-reserving
  would move the boundary of the only genuinely unseen data after reports had
  been written against the old one, which is the failure it exists to prevent.
- It refuses to run if `momentum_pilot_cohort` is empty, because without the
  in-sample list the lockbox would silently include symbols score v2 was fitted
  on.
- The **complete-label dates end 2026-03-27**, not at today's date. The 120-session
  label horizon means the most recent ~6 months of candidates have no mature
  label yet, so "the most recent 12 months" is measured from the last date where
  an outcome is actually known.

#### REDEFINED AS A REGION, 2026-09-21 (migration 018)

**The lockbox is now a REGION, not a row list.**

```
region_key:      phase2_lockbox_v2
date range:      2025-03-28 .. 2026-03-27     (unchanged)
symbols:         every symbol NOT in momentum_pilot_cohort   (unchanged)
membership:      re-derived under whatever gate version is in force
definition hash: a3ee7b3b375b53410fa1...  (over the DEFINITION, not over rows)

SUPERSEDES
row list:        phase2_lockbox, 1,783 rows / 1,037 symbols
old hash:        104cce0836a12af00242b31de3df5b53175febf8b5cf1024e40f74792feb495c
```

**Why.** The row list was reserved under gate v1. Gate v2 changes which
symbol-days pass §3.2 — 42.3% of the candidate union changes membership
(§3.2.2) — so the old list is neither a superset nor a subset of the candidates
in its own window. Keeping it gave two bad options: exclude exactly those 1,783
rows and let new gate-v2 candidates inside the window leak into training, or
re-derive the rows and call it the same lockbox, leaving the hash describing
nothing.

**Why this is legitimate here and would not be later.** The guarantee a lockbox
provides is "no decision has been informed by these outcomes". That holds for
the **region**, not merely for the row list: the whole window was excluded
wholesale from every report produced to date, and no outcome inside it has been
inspected. The boundaries are unchanged; only the unit of membership moved.

**This is a one-time correction, available only because the window has never
been looked at.** Once an outcome inside it has been seen, no redefinition is
legitimate and rule 4 above stands without exception.

The definition hash is over the region's **definition** rather than its member
rows, deliberately: hashing derived rows would reintroduce the brittleness this
change removes.

**Verifying the hash.** Run `scripts/verify_cohort_hashes.py`, which also
checks the pilot cohort hash and asserts the two sets are disjoint. The
canonical form is pinned there because it has to be exact: sha256 over
`symbol,ts` lines, sorted, joined by `\n`, **with no trailing newline**, `ts`
as `YYYY-MM-DD`. The first hand-verification of this hash failed purely because
`psql COPY TO STDOUT` appends a trailing newline — the hash was right and the
check was wrong, which is the least useful way for an integrity check to fail,
since it casts doubt on good data. Use the script, not an ad-hoc pipeline.

Current status: **verified 2026-09-21** — 1,783 rows, 1,037 symbols, hash
matches, 0 rows overlapping the in-sample pilot.

**Uses of `--final-lockbox-evaluation` so far: none.**

Rules:

1. Every training and evaluation query goes through **one** dataset function or
   view that excludes lockbox rows. The only exception is when the explicit
   `--final-lockbox-evaluation` flag is passed. Each use of that flag is logged
   into this section.
2. A test asserts that lockbox rows never appear in a training dataset.
3. The lockbox is evaluated **exactly once**, on the final pre-registered
   model, and the result is reported whatever it is.
4. If the final model fails on the lockbox, it does not ship. **There is no
   second lockbox evaluation of a modified model.** A modified model must be
   tested on forward sessions that accrue after the reservation date.

### 4.4 Statistical hygiene

- State `n` (episodes and positives) before any rate.
- Inference is at the episode level only.
- Report **every** feature and threshold combination tested. When claiming
  significance within a family of tests, apply Benjamini–Hochberg.
- Log every run in MLflow, including negative and abandoned runs.
- For pooled OOS significance, use a **block bootstrap by calendar month**, so
  that the correlation between episodes in the same market regime is preserved.

---

## 5. Models

### 5.1 Targets and eligibility

The targets are Phase 1 §6's labels: `hit_100`, `hit_200`, `hit_300`,
`hit_500`, `hit_1000`. Each is **the peak close reaching +X% at any point
within 120 sessions.** It is not an exit price and not a target.

A threshold is modelled **only if** the pre-lockbox data contains at least **150
positive episodes** for it, and every evaluated test fold contains at least
**25**. The expectation is that `hit_100` qualifies, `hit_200` probably does,
and `hit_500`/`hit_1000` probably do not.

#### 5.1.1 First-passage targets — pre-registered, because the touch-anytime label rewards volatility

**Pre-registered secondary targets, added 2026-09-21:**

```
fp_100_dd50   : the close reaches +100% BEFORE it ever closes -50% from entry
fp_100_atr    : the close reaches +100% BEFORE it ever closes -2 x atr_14_at_entry
                from entry
```

Same 120-session horizon. If neither bound is touched within the horizon, the
episode is a **miss** for the first-passage label, so the three outcomes
(target first / stop first / neither) collapse to a binary in the conservative
direction.

**The confound these exist to address.** `hit_100` asks whether the peak close
*ever touched* +100%. Touch-anytime is structurally easiest for the wildest
stocks, and the post-upgrade report measured exactly that — three of the four
strongest "signals" are restatements of volatility:

| Component | Effect on `hit_100` | What it says |
|---|---|---|
| `atr_pct` above median | **+15.86pp** | realised volatility |
| `log(dollar_volume)` above median | **-6.09pp** | illiquid, small names |
| `pct_of_52w_high` above median | **-7.65pp** | already beaten down, so more room |

All three are the same statement: *a stock that moves a lot is more likely to
reach any distant level.* That is not an edge. It predicts the **magnitude** of
motion and says nothing about **direction**, and §4.1's drawdown finding already
established that these are the names that reach the level through the worst
drawdowns — median forward drawdown across the dataset runs 25-39% by year.

A first-passage label cancels most of that advantage, because the same
volatility that makes +100% reachable also makes -50% reachable, and the label
requires the *good* bound to arrive **first**. It is therefore much closer to a
tradable outcome: it encodes path, not just extremum.

**Rules.**

- `hit_100` stays the **primary** target, for continuity with Phase 1 and
  because it is what §7's display promises.
- `fp_100_dd50` and `fp_100_atr` are **reported alongside it for every model and
  every baseline.** Not optional, and not conditional on the primary result.
- **A model whose edge exists on `hit_100` but vanishes on both first-passage
  targets has most likely learned volatility.** Say so in the report, in those
  words, rather than leading with the `hit_100` number.
- The `-2 x atr_14` variant exists because a fixed -50% is itself
  volatility-dependent: it is a far tighter stop for a low-ATR name than a
  high-ATR one, so the fixed and scaled bounds fail in opposite directions and
  running both brackets the truth.
- Entry reference is `close[t]` of the episode's first day, matching §6's label
  convention, and both bounds are evaluated on **closes** — daily bars cannot
  resolve whether an intraday low preceded an intraday high, and pretending
  otherwise would be a lookahead of exactly the kind §12 forbids.
- Eligibility minimums (150 pre-lockbox positives, 25 per fold) apply
  independently; first-passage positives will be **rarer** than `hit_100`
  positives, so a threshold may qualify on one label and not the other. Report
  the counts.


An ineligible threshold is displayed as `—` together with its positive count.
**Never lower the bar to fill the display.**

### 5.2 Features

Read from `momentum_features`, which the Go feature engine computes. **Do not
recompute these in Python.**

- `rvol_20`, `vol_accel`, `change_pct`, `gap_pct`, `atr_pct`
- `log(dollar_volume)`, `rsi_14`, `vwap_dist_pct`, `above_vwap`
- `breakout_state` (one-hot), `pct_of_52w_high`, `range_20`, `bucket`

Plus, as they become available: `sector_strength_pct` (§3.4), catalyst keyword
families with `news_coverage` (§3.5), regime features (§3.6), and point-in-time
float and market cap (§3.2).

**Excluded:**
- anything derived from current share counts, until §3.2 lands;
- `momentum_score_100` itself, which is a baseline, not a feature.

**Missing values** get explicit indicator columns. Never impute silently (Phase 1
§12 still applies).

### 5.3 Baselines — every model must beat these

1. The base rate, per bucket.
2. **`rvol_20` alone**, used as a rank. If a model cannot beat one column at the
   same alert volume, it adds nothing. Note that rvol-alone out-separated the
   full v2 score out-of-sample (+5.68pp against +2.21pp pooled), so this is a
   real bar, not a formality.
3. **`atr_pct` alone**, used as a rank. **Added 2026-09-21 and non-negotiable.**
   Volatility was the single strongest correlate of `hit_100` in the
   post-upgrade report (+15.86pp on a median split, larger than every designed
   component), so *"just pick the most volatile stocks"* is the baseline a model
   most needs to beat. A model that cannot has learned nothing useful — it has
   rediscovered that wild stocks move.
   - Report it on **both** the `hit_100` and the first-passage targets (§5.1.1).
     The expectation is that it looks strong on `hit_100` and much weaker on
     first-passage; if a model's advantage over this baseline appears only under
     first-passage, that is evidence the model found something other than
     volatility, which is the point of carrying both.
4. The frozen §4.1 v2 score. **Note that v2 is NOT confirmed out-of-sample**
   (Phase 1 §10.1.0): its pooled pass was bucket composition, and it separates
   within neither bucket. It remains a baseline because it is what currently
   drives alerts, but beating it is a weak achievement and must not be reported
   as validation.

### 5.4 Model families, tried in this order

1. **L2-regularised logistic regression**, one per eligible threshold, on
   standardised features. It is interpretable and stable at small `n`, and it is
   the expected winner at this sample size.
2. **Gradient-boosted trees** (LightGBM): shallow trees, strong regularisation,
   early stopping on inner folds. Use this only if it is pre-registered **and**
   beats (1) on the primary metric across folds. Do **not** impose monotone
   sign constraints — Phase 1's `breakout` result showed that assumed signs can
   be backwards.

### 5.5 Cross-threshold consistency

Independent classifiers can output `P(hit_200) > P(hit_100)`, which is
impossible. Default fix: `p_k = min(p_k, p_(k-1))`. Report how often this clip
is active. Frequent clipping means the per-threshold models disagree
structurally, and that is itself a finding. A test asserts that final outputs
are monotone across thresholds.

### 5.6 Calibration

Fit calibration on **out-of-fold predictions from the inner walk-forward, within
the training window.** Never fit it on the test fold.

- Use isotonic calibration when there are ~1,000 or more calibration episodes;
  otherwise use sigmoid (Platt) calibration.
- Report reliability diagrams per fold, expected calibration error (ECE), and
  the Brier score.

### 5.7 Metrics

**Primary (pre-registered):** hit rate **at matched alert volume**. That means
the precision of the model's top slice, at the same alert rate the Phase 1 §4.4
thresholds produce (~the p90 per bucket), compared with baselines 2 and 3. This
is the metric that matches what the product actually does: choose which ~10% of
candidates to alert on.

**Secondary:** Brier score, log loss, PR-AUC (with rare positives, ROC-AUC
flatters), calibration error, per-bucket results, stability across folds, and
the **median `fwd_max_drawdown_pct` of the selected episodes**. Hit rate alone
would have missed Phase 1's drawdown finding.

---

## 6. Acceptance criteria

These are defaults. They are finalised in the pre-registration and not changed
afterwards. **§4.2.0's two rules bind here**: every criterion below is evaluated
bucket-stratified, never pooled across buckets, and each carries a minimum
effect size as well as a p-value.

For a model to ship **to shadow mode**, all of the following must hold,
out-of-sample, at the episode level, for each threshold:

1. The primary metric beats **both** `rvol`-alone and `atr_pct`-alone (§5.3) in
   **≥ 70% of folds**, **and** the improvement is significant at p < 0.05 by
   block bootstrap by month, **and** clears the §4.2.0 effect-size floor
   (≥ 3.0pp absolute within bucket, ≥ 1.25x relative). Significance without the
   floor is not acceptance.
2. Calibration: ECE ≤ 0.03, and a reliability slope within [0.8, 1.2].
3. Criteria 1 and 2 hold **per bucket**, reported per bucket, with a CMH
   statistic and MH common odds ratio for combined power. If they hold for only
   one bucket, the model ships for that bucket only. **A pooled-across-bucket
   result satisfies nothing** (§4.2.0 Rule 1).

### 6.1 The 2020 sensitivity requirement — mandatory, added 2026-09-21

**Every acceptance result must be reported twice: with and without the 2020
fold. A model that passes only because of 2020 does not ship.**

The post-upgrade data makes this unavoidable. Hit rate by calendar year, B+C
episode level:

| Year | episodes | hits | hit rate |
|---|---|---|---|
| 2017 | 183 | 19 | 10.38% |
| 2018 | 660 | 39 | 5.91% |
| 2019 | 699 | 38 | 5.44% |
| **2020** | **968** | **269** | **27.79%** |
| 2021 | 945 | 63 | 6.67% |
| 2022 | 775 | 52 | 6.71% |
| 2023 | 1,019 | 77 | 7.56% |
| 2024 | 1,378 | 147 | 10.67% |
| 2025 | 297 | 25 | 8.42% |

**2020 supplies 269 of 729 positives — 37% of every positive outcome — from 14%
of the episodes.** Its base rate is 3-5x every other year. Any statistic pooled
across the ten-year window is substantially a statement about the post-COVID
crash and recovery, and a model trained and tested on windows that both contain
2020 will look excellent while describing one quarter of one year.

Requirements:

1. Report the primary metric, per fold, **including and excluding the fold(s)
   overlapping calendar 2020.** Both numbers appear in every report; neither is
   a footnote.
2. The **≥ 70% of folds** criterion is computed on the **2020-excluded** fold
   set. A model that beats the baselines in most folds only when 2020 is one of
   them has not beaten them in most folds.
3. Report the share of positives each fold contributes. A fold contributing a
   disproportionate share of the positives is flagged in the report, so the next
   2020-like regime is caught without waiting for someone to notice.
4. **If the model passes with 2020 and fails without it, it does not ship**, and
   the result is recorded in this spec as a negative — the same way Phase 1
   recorded its negatives.

Note the lockbox helps here by construction: it covers **2025-03-28 to
2026-03-27** (§4.3) and therefore contains no 2020 data at all. A model that
survives the lockbox has cleared at least one genuinely 2020-free test. That is
a reason to trust the lockbox result more than the fold results, not a
substitute for requirement 2.
4. **Lockbox, evaluated once:** lift over **both** `rvol`-alone and
   `atr_pct`-alone above 1.0, with the lower bound of the bootstrap 90%
   confidence interval also above 1.0, evaluated per bucket. If the lockbox is
   too small to decide, the result is "undecided": the model may run in shadow
   mode but may **never** drive alerts on that basis.
5. The **first-passage targets** (§5.1.1) are reported alongside. An edge on
   `hit_100` that disappears on both first-passage labels is reported as
   probable volatility-learning, in those words.

If a model fails: v2 stays in place, and the result is documented in this spec
the same way Phase 1 documented its negative results.

---

## 7. The multi-threshold display

This is the original idea, in its honest form. It is built **only after** a model
passes §6.

```
🔥 XYZ · penny · +11.8%
Chance the close reaches the level below at any point in the next 120 sessions
  +100%   31%   (base rate 22%)
  +200%   12%   (base rate  7%)
  +300%    —    insufficient history (38 positive episodes)
  +500%    —    insufficient history (9 positive episodes)
  +1000%   —    insufficient history (2 positive episodes)
model v1 · validated OOS on 412 episodes · regular session, daily bars
```

Rules:

- Show **probabilities, not scores**, and always show the base rate next to
  each one.
- State the horizon and meaning explicitly: *peak close touching this level
  within 120 sessions — not a price target and not a prediction of where the
  stock ends.*
- An ineligible threshold shows `—` with its positive count. Never show a number
  the data cannot support.
- Show the model version and its OOS sample size.
- Update the Phase 1 `EVIDENCE_CAVEAT` to match. It remains one shared constant
  used by both output paths.

---

## 8. Exit rules — §5 v2 research

Use the same protocol as the models: candidate rules are chosen on training
folds, confirmed on test folds, and checked on the lockbox once.

**Pre-registered candidates**, taken from Phase 1's replay findings:

| Candidate | Addresses |
|---|---|
| `breakout_failed` with an ATR buffer: `close < resistance_20_at_alert − k·atr_14_at_alert`, `k ∈ {0.5, 1.0}` | The rule firing on session 1 at a median peak of 0.00% |
| `breakout_failed` requiring 2 consecutive closes below the level | Same |
| Move `stop_atr` ahead of `breakout_failed` | `stop_atr` is suppressed by ordering |
| A `timeout` horizon, **tested only after** the faster rules are loosened | `timeout` is unreachable: 0 of 43 positions reached 20 sessions |
| Exits using `breakout_state` / `pct_of_52w_high` as risk inputs | The drawdown finding in Phase 1 §4.1 |

**Naive baselines every exit rule must beat:** a fixed hold of N sessions
(N chosen on training folds), and holding to the full 120-session horizon. If
the rules do not beat a fixed hold, they add nothing.

**Metrics:** median exit %, median give-back (peak minus exit), the share of
positions exited before their peak, and session survival.

Extend `cmd/momentum-tracker -replay` to accept a **rule-set config**, so that
variants run without code edits. The live exit path does not change until a
variant passes the protocol. These are research outputs, not trading advice.

---

## 8.5 Screener mode — what the bot actually does now (implemented 2026-09-22)

The route is feature research only, so there is no model to serve. The scanner
ships as an **honest screener** instead, and the framing is the deliverable.

| Surface | Behaviour |
|---|---|
| `EVIDENCE_CAVEAT` | Rewritten: "SCREENER, not a forecast. These candidates meet the published gates on daily bars, regular session. No component of the ranking has shown predictive value out-of-sample. Nothing here is a forecast or trading advice." One shared constant, on every surface. |
| Alert trigger | `BOT_MOMENTUM_ALERT_MODE=screener` (default). Fires on a **§3.2 gate pass per bucket**, not a score threshold. `score` mode restores 65/72 and is not recommended. |
| Alert embed | **No score, no component breakdown.** Shows why it qualified plus measured inputs. The breakdown was the more misleading half: itemised points imply each component earned its weight. |
| `/score` | Leads with the screener facts; the score appears below, under "🔬 Research score — NOT VALIDATED" with its odds ratio. Never the headline. |
| `/scanner` | Sorted by `rvol_20`, labelled "Sorted by RVOL — descriptive, not predictive". No score in entries. |
| Sell channels | **Never posted to.** Env vars and routing retained; see below. |
| `momentum_tracked` | Keeps updating, for research. |

**Why no sell alerts.** §5's exit rules are unvalidated and one is demonstrably
broken: `breakout_failed` fires on session 1 at a median peak of 0.00% on **651
of 1,545** replayed positions (42%) — it closes positions that never moved. A
"sell" in a channel is a trading instruction, which is a stronger claim than a
screener alert and has less behind it. The channels stay mapped so the decision
is visible where someone would go to add posting, rather than looking like an
oversight.

**Expected volume**, measured over the 10-year history under gate v2 with the
fallback (9,407 candidates), with the 5-session cooldown applied:

| | mean/day | median | p90 | max |
|---|---|---|---|---|
| all | **4.49** | 3 | 9 | 157 |
| market | 3.99 | 3 | 8 | |
| penny | 0.51 | 1 | 3 | |

Roughly four to five a day, with a p90 of nine — a readable feed.

**The 157 outlier is 2024-11-06**, the session after the US election. An
earlier version of this section called it "a 2020 regime day", which was wrong:
2020's busiest day produced 41. The correction matters beyond tidiness —
attributing it to 2020 would fold a single market-wide event into the separate
§6.1 finding that 2020 supplies 37% of all positives, making that finding look
partly like a volume artefact when the two are unrelated.

Days like it are capped rather than dropped (§8.5 burst cap): 10 per channel
plus an overflow line, so a truncated day is never mistaken for a quiet one.

`BOT_MOMENTUM_SCAN_ENABLE` stays **false** until the user enables it.

---

## 9. Serving — shadow mode

- **Service:** `services/model-training` (Python), with a training CLI and a
  daily inference job.
- **MLflow:** add a tracking server to Compose. Use a separate `mlflow` schema in
  the existing Postgres/Timescale database for the backend store, and a local
  volume for artifacts. Registry stages are `none → shadow → alerting`. **Moving
  a model to `alerting` is the user's decision, never the agent's.**
- **Predictions table** (`momentum_predictions`, a hypertable), primary key
  `(symbol, ts, model_version, threshold_pct)`. Columns: `p_raw`,
  `p_calibrated`, `eligible`, `feature_hash`, `created_at`.
- **Inference** runs after the daily `momentum-scanner` run, on today's episode
  starts, reading `momentum_features`.
- **Training/serving parity test:** the inference feature vector for a
  historical date must equal that date's training-dataset row. This is Phase 2's
  version of Phase 1's "replay uses the same `EvaluateExit` as live."
- **Bot:** `/score` shows the v2 score and the model probabilities side by side.
  Alerts are selected by `MOMENTUM_ALERT_SOURCE=v2|model`, with default `v2`.
- **What shadow mode can and cannot show.** Outcome labels need 120 sessions to
  mature, so a shadow period of a few weeks **cannot** validate hit rates. It
  checks serving correctness: parity, prediction distributions against training,
  and feature drift (population stability index, with a warning above 0.2).
  Live outcome validation builds up over 6+ months. Say this plainly in the
  bot's `/score` output while the model is in shadow.

---

## 10. Schema

Migrations continue after the last number used by the Phase 1 post-upgrade step.
Follow the repo's migration rule (committed means never amended), and document
every table in `SCHEMAS.md` with a matching `*.schema.json`.

| Table / change | For |
|---|---|
| `phase2_lockbox` | §4.3 (created during post-upgrade validation) |
| raw close + `split_factor` on `equity_ohlcv` | §3.2 (created during post-upgrade validation) |
| `momentum_episodes` | §3.1 |
| `shares_outstanding_pit` | §3.2 |
| `universe_symbols`: `listed_at`, `delisted_at`, source identifier | §3.3 |
| `momentum_labels.label_truncated_by_delisting` | §3.3 |
| `momentum_predictions` | §9 |

---

## 11. Build order

| Step | Deliverable | Done when |
|---|---|---|
| P2-0 | Entry gate (§2) | Post-upgrade report documented; lockbox manifest committed and hashed |
| P2-1 | Episodes (§3.1) | Table populated; results reported for gaps 3/5/10 |
| P2-2 | Point-in-time shares (§3.2) | EDGAR reachability and coverage verified; table populated; gate-v2 change report; current-shares ablation reported |
| P2-3 | Survivorship (§3.3) | Tiingo delisted coverage verified (or recorded as unverifiable); base rate with vs without delisted names |
| P2-4 | Sector (§3.4) | `finnhubIndustry` stored; `sector_strength_pct` computed |
| P2-5 | Catalyst history (§3.5) | Tiingo News depth verified; episode-start backfill cached; keyword families stored |
| P2-6 | Dataset builder + leak tests | Lockbox-exclusion test, filed-date as-of test and news-cutoff test all pass |
| P2-7 | Pre-registration (§4.2) | Committed before any fit |
| P2-8 | Baselines through the walk-forward harness | All three baselines reported per fold |
| P2-9 | Logistic models (§5.4.1) + calibration + monotonicity | Per-threshold results against §6 |
| P2-10 | GBM (§5.4.2) | Only if pre-registered; result reported either way |
| P2-11 | Exit research (§8) | Candidate rules evaluated against the naive baselines |
| P2-12 | **Lockbox evaluation — once** | Result recorded in §4.3, whatever it is |
| P2-13 | Shadow serving (§9) | Predictions written daily; parity test passes; `/score` shows both |
| P2-14 | Decision point: alert source | **The user's decision.** The agent presents evidence and does not flip the switch |

**Why the data corrections come before any modelling:** every model fitted on
contaminated features would have to be redone. Worse, its results would already
have been looked at, and looking at results is what the lockbox exists to
prevent.

---

## 12. Notes for the implementing agent

Every Phase 1 §12 rule still applies. In addition:

- **Checkpoint commit before and after every step, and before any multi-file
  or scripted edit. Push at the end of each step.**

  Not hygiene — recovery. A scripted edit to
  `services/data-ingestion/cmd/data-fundamental/main.go` used an inverted
  string slice, so `old` was the empty string and `str.replace("", new)`
  inserted the replacement between **every character** of the file. It went
  from ~42 KB to 121 MB in one call, and no backup of that file existed
  anywhere on the machine.

  It was recovered only because the corruption happened to be uniform: the
  inserted text could be removed again to reconstruct the original byte for
  byte. **That was luck, not a plan.** A slice that had merely mangled part of
  the file, or a script that had written partial output, would have been
  unrecoverable.

  With a checkpoint commit it is `git checkout -- <file>`, one line, no
  cleverness required. The cost of a commit is a few seconds; the cost of not
  having one is however long it takes to rewrite work from memory, plus the
  risk of rewriting it subtly differently.

- **Label every example output `LIVE` or `FIXTURE`. No exceptions.**

  `LIVE` output must be accompanied by the query, command or script that
  produced it, so a reader can re-run it. `FIXTURE` output must say so in the
  same breath as the numbers, not in a footnote.

  This exists because of a real cost. A screener embed was rendered from
  `sample_row()` — a hardcoded test fixture — and presented as "the actual
  output". Two of its fields (`change_pct`, `dollar_volume`) had been copied
  from real data, so it read as a genuine bar, and the mismatch against an
  earlier live test looked exactly like a data-corruption bug: the same
  symbol-day with different RVOL, RSI and 52-week-high. It triggered a full
  investigation of adjusted-history handling before the fixture was identified.

  The investigation happened to find a genuine defect, which is luck and not a
  defence. A fixture presented as live data is indistinguishable from corrupted
  live data, and the reader has no way to tell them apart — so the labelling is
  the writer's job, every time.

- **Never** use random or shuffled splits, for any purpose.
- **Never** touch the lockbox without the explicit flag, and never evaluate it
  twice.
- **Never** recompute in Python a feature the Go engine already computes. Read
  `momentum_features`. A second implementation will quietly diverge from the
  first.
- **As-of joins use the date information became available** (a filing's `filed`
  date, an article's publication time), never the date the information is
  *about*.
- Report every run, including negative ones. State `n` before any rate.
- Tune only on inner folds. Test folds are for measuring, never for choosing.
- **Stop and ask before:** amending the pre-registration, changing a gate
  definition, changing `MOMENTUM_ALERT_SOURCE`, or touching the lockbox.
- **"No edge" is a valid deliverable.** Do not keep searching feature
  combinations until something turns out significant. That is precisely the
  failure mode this protocol exists to prevent.

---

## 13. After Phase 2

**Phase 3 (real-time)** stays as Phase 1 §11 describes: intraday bars and
WebSocket feeds, time-of-day-normalised RVOL, intraday VWAP, halt detection and
pre-market data. It is only worth building if Phase 2 shows an edge on daily
bars. A daily-bar edge that does not exist will not appear by looking at the
same stocks more often.

The **web UI / MFE** follows Phase 1 §11's layout. The `backtest-lab` view is
where this document's fold reports, calibration plots and lockbox result
eventually live.
