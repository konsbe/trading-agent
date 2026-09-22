# Momentum Scanner — Phase 1 Implementation Spec

**Target repo:** `trading-agent`
**Status:** specification for implementation. Nothing in this document is built yet.
**Scope:** Phase 1 only — daily (end-of-day) scanner on free data sources, Discord alerts, and the labeled dataset that Phase 2 will train on. **No intraday, no real-time, no web UI in Phase 1.**

This document is written to be handed to a coding agent (Cursor/Opus) together with the repo. It assumes the reader has the repo but **not** the conversation that produced this spec, so every definition is stated explicitly.

---

## 0. Read these repo files first

The scanner is an extension of an existing, working system. Do not rebuild infrastructure that exists.

| File | Why |
|---|---|
| `services/data-ingestion/data_ingestion.md` | Worker pattern to copy: one binary per `cmd/`, env-var config, upsert stores |
| `shared/schemas/SCHEMAS.md` | Table/schema documentation conventions; new tables must be documented here |
| `shared/databases/migrations/` | Migration numbering — new migrations continue from `007_` |
| `README.md` (root) | Discord output reference: embed conventions, colours, `—` semantics, batching |
| `services/analyst-bot/` | The bot being extended: `reports/builder.py`, `notifier/discord/formatter.py`, `scheduler/jobs/`, `db/queries/` |
| `services/data-analyzer/internal/compute/` | Existing indicator library — `donchian.go`, `sr.go`, `swings.go`, `vwap.go`, `volume.go`, `atr.go` |

**Known repo issues to be aware of (pre-existing, not caused by this work):**
- `services/analyst-bot/analyst_bot.md` is an empty file (0 bytes). It contains no guidance.
- `services/analyst-bot/notifier/discord/discord.md` is stale — it predates the `#actions` channel and documents a `BOT_WEEKLY_DIGEST_CRON` that does not exist in code. Treat root `README.md` as authoritative for bot behaviour.
- `technical-analysis` writes trend direction as `up`/`down`/`sideways`, but the bot's `_trend_emoji` lookup expects `uptrend`/`downtrend`/`sideways`, so trending symbols render without an arrow. Unrelated to this spec; fix separately if convenient.

---

## 1. What Phase 1 delivers

1. A **universe loader** that pulls the full US common-stock list and keeps daily OHLCV bars for all of it.
2. A **feature engine** that computes, per symbol per day, an unambiguous set of momentum features.
3. A **single momentum score** (0–100) measuring *setup quality*, not predicted magnitude.
4. **Discord alerts** into four channels (penny buy/sell, market buy/sell), posted once per day after the close.
5. A **labeled dataset** (`momentum_labels`) recording what actually happened after each historical setup — this is the deliverable Phase 2 depends on.

### Explicitly out of scope for Phase 1
Intraday/real-time scanning, WebSocket feeds, pre-market and after-hours data, time-of-day-normalized RVOL, halt detection, short-interest data, European exchanges, any ML model, any web/MFE frontend, the `+200%/+300%/+500%/+1000%` score divisions.

### The one scoring decision that is locked
**There is exactly one score: `momentum_score_100`, an integer 0–100.** It answers *"how closely does this setup resemble the profile of a stock that went on to run?"* It does **not** predict a percentage gain.

The multi-division idea (+100/+200/+300/+500/+1000) is deferred to Phase 2, where each threshold becomes a **calibrated probability** from a classifier trained on `momentum_labels`. Phase 1 still *records* the `hit_200`/`hit_300`/`hit_500`/`hit_1000` labels so Phase 2 has training targets — but it never displays or computes a score for them. Do not build the 5-division display in Phase 1.

---

## 2. Data sources — free tier

### 2.1 Why not EODHD in Phase 1

EODHD's bulk endpoints are the right long-term answer (one call returns an entire exchange in 5–10 seconds) but its **free plan allows 20 API calls per day and restricts several data types**, and bulk access should be assumed paid-only until verified. Paid plans (100k calls/day, 1,000 req/min cap) are the Phase 3 target. Also note per-call cost weighting when budgeting later: intraday/technical/news endpoints cost 5 calls each, fundamentals/options cost 10.

**Phase 1 uses free sources that are already integrated in this repo.**

### 2.2 The Phase 1 free stack

| Need | Source | Status in repo | Notes |
|---|---|---|---|
| US symbol list | Finnhub `/stock/symbol?exchange=US` | `internal/fetch/finnhub/client.go` exists | Free tier. Alternative/cross-check: NASDAQ Trader public symbol directory files. |
| Daily OHLCV bars | Yahoo Finance | `internal/fetch/yahoo/bars.go` exists | **Free, no key, consolidated volume, multi-year history.** Primary bar source for Phase 1. Unofficial endpoint — rate-limit politely and treat as best-effort. |
| Company news (catalyst) | Finnhub company news | `internal/fetch/finnhub/client.go` exists | Free tier, 60 req/min. |
| Shares outstanding, sector, market cap | Finnhub `/stock/metric` | Already ingested into `equity_fundamentals` | `shares_outstanding` and `market_cap` metrics already exist there. |
| Cross-check bars | Alpaca | `internal/fetch/alpacadata/bars.go` exists | ⚠️ **Do not use as primary.** Alpaca's free tier serves the IEX feed only, whose volume is a single-venue fraction of consolidated volume. Volume-based features computed on IEX volume are unreliable. |

> **Status 2026-09-15 — the primary bar source is unverified in practice.** Yahoo's
> chart API returns a blanket `429` from the development network (an IP-reputation
> block, not rate limiting — Finnhub works fine from the same host, so egress is
> not the issue). Two fallbacks were evaluated and neither is viable as-is:
> Finnhub's own `/stock/candle` is **403 on the free tier** (a plan restriction
> that would fail from any network), and Stooq's free CSV endpoint returns a
> JavaScript proof-of-work challenge with an HTTP 200 status. The decisive
> outstanding test is re-running Yahoo from a network with different egress, which
> would distinguish "Yahoo is gone" from "Yahoo is unusable from that office".
>
> Separately confirmed: **Tiingo, Polygon.io, Twelve Data and EODHD are all
> reachable from the blocked network**, returning auth rejections rather than IP
> blocks. Providers metering by API key do not share a rate budget with everyone
> behind the same egress, so that class of source sidesteps this failure mode
> entirely and is the more durable fix even if Yahoo recovers. None is yet verified
> for free-tier daily bars, history depth, or — the requirement that actually
> matters — **consolidated volume**. Use
> `services/data-ingestion/scripts/verify-bar-source.sh` to settle that before
> naming a replacement here.
>
> **Measured 2026-09-16 — all four serve consolidated volume; they separate on
> history depth and quota.** Tiingo returns the full 751 daily bars over 3 years
> with no observed throttle and is the **leading replacement candidate**. Twelve
> Data matches on history but hard-caps at 8 requests/minute (≥ 10 hours per
> universe pass). Polygon's free tier stops at **2 years** and EODHD's at **1
> year**, both short of §8.1.3's 3-year requirement. One gap remains before an
> adapter is written: free tiers often meter *unique symbols per month*, which a
> 12-symbol probe cannot reveal — confirm Tiingo's account limits first, because a
> monthly cap fails silently partway through a 5,000-symbol pass. Full table in
> `services/data-ingestion/data_ingestion.md`.

> **Adjustment convention is a second, separate correctness requirement.** §3's
> opening line asks for split/dividend-adjusted bars, and the two implemented bar
> paths disagree: the Tiingo adapter reads Tiingo's `adj*` fields (split and
> dividend adjusted, including `adjVolume`), while the Yahoo adapter reads
> `indicators.quote` and never decodes `indicators.adjclose`, so its bars contain
> **no dividend adjustment** — established by code inspection, not inference.
> `equity_ohlcv` therefore holds rows on two conventions distinguished only by
> `source`, which is a trap for anyone querying across them. See
> `services/data-ingestion/data_ingestion.md`.
>
> Note `adjVolume` matters as much as the prices: raw volume is not rescaled
> across a split, so a 20-day average spanning one mixes two share bases and
> §3.4's RVOL reads the discontinuity as a genuine volume surge.

**Critical constraint: volume must be consolidated volume.** Every volume feature in this spec (RVOL, acceleration, dollar volume) is meaningless on single-venue volume. Yahoo provides consolidated volume; IEX-only does not. If bar source ever changes, re-verify this.

### 2.3 Scale and runtime budget

After filtering (§3.1) expect roughly **5,000–7,500 US common stocks**.

> **Measured 2026-09-15: 4,978**, from a live 31,051-record Finnhub US directory.
> Marginally below the estimate, and explained rather than anomalous — the
> directory contains 2,223 ADRs and 422 REITs which §3.1's strict common-stock
> reading excludes, and admitting ADRs alone would land inside the stated band. So
> the estimate above probably assumed a looser type filter. Treat ~5,000 as the
> Phase 1 baseline. Breakdown and method in
> `services/data-ingestion/data_ingestion.md`.

- Initial backfill: ~~3 years~~ **10 years** of daily bars per symbol — see the
  superseding note below. Resumable, checkpointed per symbol.

> #### The 3-year requirement is superseded (2026-09-21)
>
> **10 years, `UNIVERSE_BACKFILL_YEARS=10`.** Three years was chosen under the
> free tier, where 500 unique symbols per month was the binding constraint and
> history depth was not the thing worth spending on. The Tiingo Power upgrade
> removed that constraint, and two reasons now argue for the longer window:
>
> 1. **Labels need room.** Phase 2 validates with purged walk-forward folds and a
>    120-session label horizon. Three years of bars leaves roughly 1.5 years of
>    candidates whose labels have matured — the horizon eats the tail, and the
>    purge removes more around every fold boundary. That is not enough to cut
>    into honest folds.
> 2. **One regime is not evidence.** 2023-09 to 2026-09 is a single market
>    environment. A rule fitted inside one regime cannot be distinguished from a
>    rule that merely describes it. Ten years spans the 2016-19 expansion, the
>    2020 crash and recovery, the 2021 small-cap mania, the 2022 drawdown, and
>    2023-26.
>
> **Measured cost, not estimated.** A 20-symbol probe put the full backfill at
> **2.25 GB**, or 5.6% of Power's 40 GB/month allowance (2.95 GB at the
> pessimistic 75th percentile). Completed run: **4,975 symbols, 8,622,252 bars,
> 2016-09-21 to 2026-09-18.**
>
> #### Survivorship bias gets WORSE with the longer window, not better
>
> This has to be stated plainly, because a ten-year history reads like strictly
> more evidence and it is not.
>
> The universe is today's eligible symbols. Every company that delisted over the
> last ten years — bankruptcies, failed micro-caps, deregistrations — is absent,
> and those are disproportionately the failures. The bias is therefore a function
> of how far back the window reaches: a candidate from 2017 is drawn from a
> population already filtered by nine years of survival, while one from 2025 has
> been filtered by a few months. **Extending 3 years to 10 does not add 7 clean
> years; it adds 7 increasingly optimistic ones.**
>
> Nothing in this phase corrects it. Delisted-symbol coverage is Phase 2 §3.3,
> and until that lands the bias is unmeasured rather than removed. What this
> phase does instead is make it **visible**: hit rates are reported by calendar
> year in the post-upgrade validation section, so the drift is on the page rather
> than buried in a pooled average. If older years look better, that is the
> expected signature of the bias and not a finding about those years.
>
> #### Bar quality over the widened data
>
> `bar-audit` over the full result: **548 of 4,971 symbols carry at least one
> suspected adjustment seam (11.0%)**, against the pilot's ~8%.
>
> The increase is the **window, not the universe**. Re-running the audit over the
> same 450 pilot symbols across the new 10-year history gives **11.6%** — worse
> than the full universe's 11.0%. The metric is "has at least one seam ever", so
> it rises with observation length by construction. The ~4,500 newly-added
> symbols are, if anything, slightly cleaner than the pilot draw.
>
> **A new check the raw-close capture makes possible.** With `split_factor` now
> stored per bar, each flagged seam can be asked whether Tiingo itself reports a
> corporate action on that session. Of 1,003 flagged seams, only **15** coincide
> with a reported split — **21** if the window is widened to ±2 sessions, so an
> off-by-one is not the explanation. Sub-penny rounding (Tiingo rounds close to 4
> decimals, so a genuine sub-cent price oscillates between 0.0000 and 0.0001)
> accounts for 147 more, across just 6 symbols.
>
> That leaves ~840 flagged seams above the rounding floor with no reported
> corporate action. **This does not establish that they are provider defects.**
> `splitFactor` covers splits only; a dividend adjustment also produces a seam
> while leaving the factor at 1.0, and `divCash` was not captured. The more likely
> reading is that the detector over-flags genuine micro-cap volatility, which is
> the conservative failure direction for a data-quality check. Recorded as
> unresolved, and capturing `divCash` would close it.
- Daily incremental refresh: one recent-bars request per symbol, ~25–40 minutes at the same rate. Runs after US close.
- Fundamentals (shares outstanding, sector, market cap): refresh **weekly**, not daily. These do not move intraday and Finnhub's free quota is the binding constraint.
- News: fetch only for symbols that passed the hard gates (§3.2) — typically tens to low hundreds per day, not the whole universe.

Make the request rate, concurrency, and retry/backoff configurable via env vars. Reuse the existing `internal/httpclient` and the token-bucket rate limiter pattern already in the repo.

---

## 3. Filter and feature definitions

All features are computed on **daily bars, regular-session, split/dividend-adjusted, US exchanges only**. `t` = the most recent completed trading day. Bar fields: `open`, `high`, `low`, `close`, `volume`.

Because Phase 1 is daily-only, several definitions simplify. Where a Phase 3 intraday definition will differ, it is flagged.

### 3.1 Universe eligibility (applied once, at symbol-list load)

Include a symbol only if **all** hold:
- Instrument type is **common stock**. Exclude ETFs, ETNs, closed-end funds, mutual funds, warrants, rights, units, preferred shares, and SPAC warrant/unit classes.
- Exchange is NASDAQ, NYSE, or NYSE American (exclude OTC/pink sheets in Phase 1 — data quality is poor and the free sources cover them badly).
- Ticker contains no suffix indicating a non-common class (e.g. trailing `.W`, `.U`, `.R`, `-WT`, `-UN`, `-P`). Normalize and document the exclusion rules in code.
- Has at least **252** daily bars of history. This is the §3.7 52-week window's true requirement: that window reads `high[t-251]`, so a complete window needs 252 bars including today. An earlier revision said 250, which left a two-bar gap where "eligible" meant "window usually full" rather than "window full".

Store the result in `universe_symbols`. Refresh weekly.

> **Where the bar-count check actually runs.** The first three rules are applied at
> symbol-list load. The bar-count minimum is **not** — it is enforced as a hard gate
> in §3.2, at scan time, and `universe_symbols.bar_count` records the current depth
> for audit.
>
> This is not an omission. Checking bar history at symbol-list load is circular: bars
> exist only because the §8.1.3 backfill ran, and that backfill iterates the eligible
> set. Gating eligibility on bar count would leave the universe permanently empty on
> a fresh install. §3.2 already lists the same minimum as a hard gate, which is the
> non-circular place for it.

**ADRs and REITs are excluded in Phase 1**, even though both are arguably common
equity and ADRs in particular can run hard. The reason is measurement, not merit:
their fundamentals coverage on free sources is patchier than domestic common stock,
and adding a second asset class with its own data-quality profile on top of an
already-degraded fundamentals pipeline would make §6's base rate harder to read.
Revisit in Phase 3, once the core universe's numbers are trustworthy. The
instrument-type allowlist is configurable precisely so this is a one-line change
later rather than a code edit.

### 3.2 Hard gates (the candidate filter)

> #### VERSIONED 2026-09-21: gate v2 uses point-in-time market cap
>
> **v1 (original):** market cap is TODAY's value, applied to every historical
> row. **Retained only as an ablation** (`-gate-version 1`), because it is the
> definition every Phase 1 result was computed under and the size of the change
> has to stay measurable.
>
> **v2 (default for all new work):** market cap is
> `raw_close[t] x shares_outstanding(filed <= t)`, from SEC EDGAR
> (`shares_outstanding_pit`, migration 017). Both factors unadjusted.
>
> **Why:** v1 selects the candidate set using information from after the setup.
> A company worth $1bn in 2017 and $20bn today was judged by the $20bn, so it
> failed the market band on a day it would have passed; one that has since
> diluted was admitted to days it never qualified for. Measured on a 71-symbol
> sample, **20.1% of historical bar-days carry a materially different market cap
> than the gates used** (Phase 2 §3.2.1).
>
> **What v2 does NOT change: the bucket.** Buckets are assigned from the close
> PRICE (penny $0.30-$2.00, market >= $2.00); market cap is a band check *within*
> the price-determined bucket. So v2 changes whether a symbol-day PASSES, not
> which bucket it is judged in.
>
> **New rejection reason, `market_cap_pit_unavailable`.** A symbol-day with no
> SEC filing on or before `t` FAILS the gate. There is deliberately **no
> fallback to today's value**: a fallback would restore the lookahead precisely
> on the oldest rows, where today's share count is furthest from the truth and
> where nobody can check it. An unmeasurable day is excluded and counted, so the
> exclusion shows up in the candidate totals instead of hiding inside a
> plausible number.
>
> **v2 also has no §3.9 proxy path.** The proxy is today's share count times
> close, which is the same leak in another form.
>
> Coverage is incomplete by design: EDGAR supplies the concept for roughly four
> fifths of the universe (Phase 2 §3.2.1), and the remainder are excluded rather
> than approximated. Who they are, and whether excluding them biases the sample,
> is characterised in Phase 2 §3.2.3.

Hard gates produce the candidate set. **Scoring only ranks within the candidate set — a gate failure means excluded, not low-scored.** Every threshold below is an env-var default, not a constant.

Two buckets, differing only in thresholds:

| Gate | `market` bucket | `penny` bucket |
|---|---|---|
| `close` | ≥ $2.00 | $0.30 – $2.00 |
| `market_cap` | $300M – $10B | no lower bound; ≤ $300M |
| `change_pct` (§3.3) | +8% to +25% | +10% to +40% |
| `rvol_20` (§3.4) | ≥ 3.0 | ≥ 4.0 |
| `dollar_volume` (§3.3) | ≥ $5M | ≥ $2M |
| `min bars of history` | 252 | 252 |

Notes on the gate design:
- **The upper bound on `change_pct` is deliberate and central to the strategy.** The stated thesis is to enter at +8–15% on a confirmed move, not to chase something already up 50%. A stock up +60% today is not a Phase 1 candidate — it is already gone. Do not remove this bound.
- **`dollar_volume` is a required addition** not in the original filter list. Without it the penny bucket fills with illiquid names where a $50k order moves the price 20%, and RVOL is statistical noise. `dollar_volume = close × volume`.
- **`market_cap` falls back to `market_cap_est`** when Finnhub returns null — see §3.9. The gate is evaluated against the estimate, and `market_cap_null` stays recorded in `gate_failures` so the proxy's contribution stays measurable.
- Penny bucket has a **$0.30 floor**: sub-$0.30 names are dominated by tick artifacts and reverse-split noise.

### 3.3 Price and change features

```
prior_close      = close[t-1]
change_pct       = (close[t] / close[t-1] - 1) * 100
gap_pct          = (open[t] / close[t-1] - 1) * 100
dollar_volume    = close[t] * volume[t]
atr_14           = Wilder ATR over 14 bars          (reuse compute/atr.go)
atr_pct          = atr_14 / close[t] * 100
```

### 3.4 Relative volume — `rvol_20`

```
avg_vol_20 = mean(volume[t-20 .. t-1])       // 20 bars, EXCLUDING today
rvol_20    = volume[t] / avg_vol_20
```

Two rules that must not be violated:
- **The baseline excludes the current bar.** Including today deflates RVOL exactly when it matters most.
- If `avg_vol_20` is 0 or fewer than 20 prior bars exist, `rvol_20` is **null** and the symbol fails the gate. Never substitute 0 or 1.

> **Phase 3 note (do not implement now):** intraday RVOL must be *time-of-day normalized* — cumulative volume at minute-of-session `m` divided by the 20-day average cumulative volume at that same minute-of-session. Comparing a partial intraday volume against a full-day average makes every stock look explosive in the morning. This is the single most common scanner bug. Phase 1 avoids it entirely by only ever comparing complete daily bars.

### 3.5 Volume acceleration — `vol_accel`

RVOL answers "is today unusual versus the last month." Acceleration answers "is volume *building* or already fading." They are different signals and both are scored.

```
recent_window = mean(volume[t-2 .. t])       // 3 bars including today
prior_window  = mean(volume[t-7 .. t-3])     // the 5 bars before that
vol_accel     = recent_window / prior_window
```

Null if fewer than 8 prior bars. `vol_accel >= 1.5` is treated as genuinely accelerating; `< 1.0` means volume is decaying even if RVOL is high — this is the case the original spec wanted flagged, and §4 scores it near zero rather than excluding it.

### 3.6 Breakout — `breakout_state`

Geometry, not description. Reuse `compute/donchian.go` and `compute/swings.go`.

```
resistance_20 = max(high[t-20 .. t-1])       // Donchian upper, excluding today
range_20      = (max(high[t-20..t-1]) - min(low[t-20..t-1])) / close[t]
was_consolidating = range_20 < 0.25          // prior 20-bar range under 25% of price
```

`breakout_state` is one of:

| Value | Condition |
|---|---|
| `breakout_from_consolidation` | `close[t] > resistance_20` **and** `was_consolidating` |
| `breakout` | `close[t] > resistance_20` and not `was_consolidating` |
| `approaching` | `close[t] >= resistance_20 * 0.98` and not above it |
| `none` | otherwise |

**Confirmation rule: the breakout is judged on the `close`, never on an intrabar `high`.** A wick above resistance that closes back below is not a breakout. This matters because it is the main thing separating a real breakout from a failed one.

### 3.7 52-week high proximity — `pct_of_52w_high`

```
high_52w        = max(high[t-251 .. t-1])    // 251 trading days, EXCLUDING today
pct_of_52w_high = close[t] / high_52w        // 1.0 = at the prior high; > 1.0 = new 52-week high
```

**The window excludes the current bar**, matching the same convention already used by
`avg_vol_20` (§3.4) and `resistance_20` (§3.6). This is deliberate and load-bearing:

- An earlier revision defined `high_52w` over `[t-251 .. t]`, *including* today. Because
  `close[t] <= high[t] <= high_52w`, that made `pct_of_52w_high <= 1.0` always, so §4.2's top
  band (`>= 1.00`) was unreachable except in the knife-edge case of a stock closing exactly at
  its own session high on a 52-week-high day. Every genuine new high scored 4 instead of 5.
- With today excluded, `pct_of_52w_high > 1.0` **is** a new 52-week high, so the separate
  `new_52w_high` boolean of that earlier revision is redundant and has been **removed**. One
  field, no knife-edge, top band reachable.

> The window reads `high[t-251]`, so a complete `high_52w` needs **252** bars including
> today. §3.1's bar minimum is set to 252 for exactly this reason, so an eligible symbol
> always has a full window rather than an occasionally-short one. The minimum is an env
> var, but lowering it below 252 reintroduces silently truncated 52-week windows.

### 3.8 VWAP — `above_vwap`

Daily bars have no intraday VWAP. Phase 1 uses a **rolling 20-day VWAP** (the repo already computes `vwap_rolling_<N>` in `compute/vwap.go`):

```
typical[i] = (high[i] + low[i] + close[i]) / 3
vwap_20    = sum(typical[i] * volume[i], i = t-19..t) / sum(volume[i], i = t-19..t)
above_vwap = close[t] > vwap_20
vwap_dist_pct = (close[t] / vwap_20 - 1) * 100
```

> **Phase 3 note:** the original intent was *intraday session VWAP* (price above today's VWAP). That requires intraday bars and is a Phase 3 replacement for this feature. Keep the field name stable so the swap is a one-place change.

### 3.9 Float proxy — `float_shares_est`

Free sources do not provide true public float. Phase 1 uses **shares outstanding as a documented approximation**, already available in `equity_fundamentals` as the `shares_outstanding` metric (values are in millions — normalize).

```
float_shares_est = shares_outstanding        // APPROXIMATION, not true float
float_is_proxy   = true                      // always true in Phase 1
```

Insider and locked-up shares are not excluded, so this **overstates** float for recently-IPO'd and insider-heavy companies. Store `float_is_proxy` so Phase 2 can discount the feature's weight when the proxy is known to be poor. Do not present it to the user as "Float" without qualification — label it `Float (est)` in Discord output.

#### Market cap fallback — `market_cap_est`

Market cap comes from Finnhub's free `/stock/metric`, whose micro-cap coverage is poor. A null
market cap fails the §3.2 gate, which would silently empty the penny bucket — precisely the
population the penny bucket exists to scan. Phase 1 therefore uses the same documented-proxy
pattern as float above:

```
market_cap_est      = shares_outstanding * close[t]   // ONLY when Finnhub market_cap is null
market_cap_is_proxy = true                            // mirrors float_is_proxy
```

Gate on `market_cap_est` when the real value is null. **Keep `market_cap_null` in
`gate_failures` even when the estimate lets the symbol through**, so §10 step 5's candidate-count
sanity check can measure how often the proxy is doing the work.

This is not a silent substitution and does not violate §12. What §12 forbids is a substitution
with no formula and no flag — a bare `0` or `1`, or quietly dropping the gate. This fallback has
an explicit formula, a persisted `market_cap_is_proxy` flag, and a retained gate-failure record.
If `shares_outstanding` is *also* null, `market_cap_est` is null and the gate fails for real.

**Short interest / days-to-cover is not in Phase 1.** Small float plus high short interest is one of the more reliable ingredients in real 100%+ squeezes, but no free source covers it acceptably. Leave nullable columns `short_interest_pct` and `days_to_cover` in the schema so the feature can be added without a migration later.

### 3.10 RSI — a penalty, not a filter

RSI is deliberately **not** a positive scoring input. A momentum breakout is *supposed* to have elevated RSI; rewarding low RSI would select against the thesis. It is used only to penalize already-exhausted moves.

```
rsi_14 = Wilder RSI, 14 bars                 (existing indicator)
```
Applied as a penalty in §4.3.

### 3.11 Catalyst — `catalyst_tier`

The hardest feature, and the one most likely to be over-engineered. **Phase 1 uses a keyword classifier, not an LLM.** The point of Phase 1 is to discover which keywords actually correlate with runners; paying for LLM classification before knowing that is premature.

Fetch Finnhub company news for candidate symbols only, window = **last 48 hours**. Classify each headline into the highest matching tier:

| Tier | Score input | Example keyword sets (extend in config, not code) |
|---|---|---|
| `A` | strong | FDA approval / clearance / breakthrough designation, acquisition, merger, buyout, takeover, definitive agreement, contract award, government contract, phase 3 results, earnings beat, raised guidance, uplisting |
| `B` | weak | analyst upgrade, price target raised, partnership, collaboration, product launch, conference presentation, general company PR, any other company news |
| `none` | zero | no company news in window |

Store **every** matched event in `catalyst_events` with its headline, source, matched keyword, and tier — not just the winning tier. Phase 2's most valuable analysis is which specific keywords preceded real runners, and that requires the raw matches retained.

Keyword lists live in configuration (a YAML/JSON file or env-loaded config), so they can be tuned without a rebuild. Shipped at
`services/data-ingestion/config/catalyst_keywords.json`; the file **replaces** the
built-in table rather than extending it, and a missing or malformed file is an
error rather than a silent fallback to defaults.

#### Step 8 result — catalyst is UNVALIDATED (measured 2026-09-18)

Catalyst was the one scoring component never tested: null throughout every Step 7
run, worth 15 of the 90 allocated points. It has now been measured, and the
answer is that **it carries no detectable positive signal at this sample size.**

Evaluated against the same candidate set §4.1 v2 was derived from, restricted to
those inside the provider's news window — Finnhub's free company news reaches
back roughly 12 months (measured: a 2026-01 window returns 243 AAPL articles,
2025-09 returns 0), and §6's complete-label rule requires candidates be at least
120 sessions old, so the two constraints intersect to **80 of the 218**:

| Tier | n | hit% | lift |
|---|---|---|---|
| `A` | 9 | 11.11 | 0.56× |
| `B` | 59 | 22.03 | 1.10× |
| `none` | 10 | 20.00 | 1.00× |

Base rate on the subset: 16/80 = 20.00%. **Tier A underperforms tier B**, which
contradicts §4.2's assumed A > B > none ordering — but on n=9 carrying a single
hit, that is not a finding.

The `b_on_any_news` catch-all turns out to dominate the picture: only **18 of 78**
candidates matched a real keyword at all, so roughly 50 of the 59 tier-B rows are
there purely because coverage existed. Re-classifying with the catch-all off
(free, from the cached headlines) separates the two:

| Group | n | hit% |
|---|---|---|
| any keyword match (A or B) | 18 | 11.11 |
| no keyword match | 62 | 22.58 |

z=+1.07, **p=0.28 — not distinguishable from noise.** The direction is negative in
both configurations, which is at least consistent with the §4.1 v2 finding that
lateness markers score backwards, but 18 keyword-matched candidates carrying 2
hits cannot establish that.

**Consequences, stated plainly:**

- Catalyst's 15 points are **unvalidated**, the same status as `vol_accel`,
  `float` and `vwap` — not the proven component that §4.1 v2's reserved 10 points
  could be assigned to. The reserved capacity stays reserved.
- The **keyword vocabulary itself is a hypothesis**, not a validated instrument.
  This is exactly why §3.11 requires every individual match be stored rather than
  only the tier: which specific phrases precede runners remains measurable once
  the sample is larger.
- Raw headlines are cached (`catalyst-backfill -cache`), so vocabulary variants
  can be re-tested offline at zero provider cost. Tuning a list that needs ~80
  provider requests per iteration is not tuning.

### 3.12 Sector strength — `sector_strength_pct` (recorded, not scored)

Because the whole universe is in the database, this is cheap:

```
sector_strength_pct = median(change_pct_5d of all universe symbols in the same sector)
```

Computed daily, stored on the feature row, **not included in the Phase 1 score.** It is a Phase 2 candidate feature — there is no principled weight for it yet, and guessing one adds noise. Sector mapping comes from Finnhub fundamentals.

### 3.13 Deliberately absent in Phase 1

| Feature | Why absent |
|---|---|
| Halt status (LULD) | Not available on free/EOD data. Irrelevant for a once-daily post-close scan. Required for Phase 3 real-time. |
| Pre-market / after-hours change | Different data tier; and `change_pct`, `rvol`, and VWAP all mean different things across sessions. Phase 1 is **regular session only** — state this in the Discord footer so output is never misread. |
| Short interest | No acceptable free source (§3.9). |
| Intraday session VWAP | Needs intraday bars (§3.8). |
| **Sector / industry** — and therefore `sector_strength_pct` (§3.12) | Structurally unreachable on the free tier at universe scale, see below. |

#### Why sector is null in Phase 1

`sector` and `industry` reach `equity_fundamentals` from exactly one place:
`data-fundamental`'s `runOverview`, which calls **Alpha Vantage
`COMPANY_OVERVIEW`**. The Alpha Vantage free tier allows **25 requests per day**.
Against a 5,000–7,500 symbol universe that is a **200+ day** pass for a single
refresh. No cadence, concurrency or retry setting changes that — it is a quota
ceiling, not a throughput problem.

This costs **zero scoring accuracy**, which is why it is acceptable rather than
merely unavoidable: §3.12 already specifies `sector_strength_pct` as *recorded, not
scored* in Phase 1, precisely because there is no principled weight for it yet. A
null sector therefore removes a Phase 2 candidate feature from the dataset and
nothing from `momentum_score_100`.

**Phase 2 path, not something to chase now:** Finnhub `/stock/profile2` is on the
free tier and carries `finnhubIndustry`, so sector becomes reachable at roughly one
request per symbol — about 3.3 hours for the universe at the client's current rate.
Worth doing only once §3.12 has earned a weight from the labelled data. Adding it
in Phase 1 would buy an unscored column at the cost of another 6,000-symbol pass
against a Finnhub budget that is already oversubscribed (see §8.1.2).

---

## 4. Scoring — `momentum_score_100`

One integer, 0–100, computed only for symbols that passed the hard gates.

### 4.1 Weight table — versioned

Two revisions exist. **v1 is retained deliberately**, not as history but as the
thing v2 is evidence *against*: a reader six months from now needs to see what
changed and why, or v2 looks like an arbitrary retune.

#### v1 — the original correction (superseded)

The original brief listed both `Volume +25` and `Relative Volume +20`, which
double-counts the same underlying quantity. v1 resolved that, preserving a total
of 100:

| Component | v1 Weight | Measures |
|---|---|---|
| Volume acceleration | 25 | Is volume *building* (`vol_accel`) |
| Relative volume | 20 | Is today unusual vs. the month (`rvol_20`) |
| Breakout | 20 | `breakout_state` |
| Catalyst | 15 | `catalyst_tier` |
| Float (est) | 10 | `float_shares_est` |
| Above VWAP | 5 | `above_vwap` |
| Near 52-week high | 5 | `pct_of_52w_high` |
| **Total** | **100** | |

#### v2 — revised on Step 7 evidence (current)

| Component | v2 Weight | Δ | Basis |
|---|---|---|---|
| Relative volume | **35** | +15 | only component with measured positive signal |
| Volume acceleration | 25 | — | underpowered to distinguish; unchanged |
| Catalyst | 15 | — | never exercised (§3.11 is Step 8) |
| Float (est) | 10 | — | underpowered to distinguish; unchanged |
| Above VWAP | 5 | — | underpowered to distinguish; unchanged |
| Breakout | **0** | −20 | **measured inverted**, p=0.0008 |
| Near 52-week high | **0** | −5 | **measured inverted**, p=0.0089 |
| **Allocated** | **90** | | |
| **Reserved, unallocated** | **10** | | held pending §3.11 and re-validation at scale |
| **Capacity** | **100** | | `momentum_score_100` keeps its name and range |

Both zeroed fields are **still computed and stored**. `breakout_state` is read
elsewhere, both are useful review data, and zeroing a weight is reversible in a
way that deleting a feature is not. The sub-score functions derive their points
from the weight constants rather than hardcoding zeros, so the constant is the
single source of truth and restoring a weight restores v1's exact shape.

##### The evidence

Step 7, over the 450-symbol pilot: 180,460 symbol-days evaluated, 321 gate
passes, 102 excluded as label-incomplete per §6, leaving **218 evaluable
candidates** and a **15.60% base rate** (34/218 reached +100% within 120
sessions).

v1's total score did not merely fail to separate — it separated **backwards**:

```
bottom third (n=72): 25.00%    top third (n=72): 11.11%
```

Per-component, two-proportion z-tests, 109 candidates per half:

| Component | v1 weight | low-half hits | high-half hits | z | p | Reading |
|---|---|---|---|---|---|---|
| `rvol` | 20 | 10 | 24 | +2.61 | 0.0089 | **predictive** |
| `breakout` | 20 | 26 | 8 | −3.36 | 0.0008 | **inverted** |
| `high52w` | 5 | 24 | 10 | −2.61 | 0.0089 | **inverted** |
| `vol_accel` | 25 | 16 | 18 | +0.37 | 0.71 | underpowered |
| `float` | 10 | 16 | 18 | +0.37 | 0.71 | underpowered |
| `vwap` | 5 | 18 | 16 | −0.37 | 0.71 | underpowered |

That accounts for the inversion arithmetically: **20 points of real signal
against 25 points of inverted signal and 40 points of noise.**

##### The confound check

The bucket effect is stronger than the score effect (penny 32.08% vs market
10.30%), which is a textbook Simpson's paradox setup — a score correlating with
bucket membership would show separation, or inversion, without any of it being
about the score. **It is not the explanation.** The inversion persists inside the
market bucket alone:

| Market-bucket quintile | Score | n | hit% | median drawdown% |
|---|---|---|---|---|
| Q1 | 20–48 | 33 | 24.24 | 45.6 |
| Q2 | 50–59 | 33 | 6.06 | 36.3 |
| Q3 | 59–64 | 33 | 9.09 | 35.1 |
| Q4 | 64–70 | 33 | 6.06 | 28.0 |
| Q5 | 70–84 | 33 | 6.06 | 32.0 |

The penny bucket is U-shaped with 10–11 rows per quintile, which is noise.

##### The structural reading

This is not a tuning problem, it is a **contradiction in the design**.

§3.2 excludes stocks already up more than 20–25% because, in this spec's own
words, *"a stock up +60% today is not a Phase 1 candidate — it is already
gone."* §4.2 v1 then awarded 25 points for a fresh 52-week high and a confirmed
breakout, which is the geometric signature of precisely that lateness. **The gate
said don't chase; the score paid to chase.**

Among candidates that have *already cleared the gate*, being at a fresh high is a
lateness marker, not a quality marker: it does not indicate a clean setup, it
indicates the same setup further along its run. That both inverted components
carry the same theoretical sign — rather than one being a fluke — is what makes
this a confident finding rather than a noisy one.

##### Why rvol did not absorb all 25 freed points

`rvol` is the sole proven-positive component, but "proven" rests on a single
z-test at n=109 per half from one 450-symbol pilot. Real signal; not a number
worth staking half the score's weight on. 15 points go to `rvol`; **10 are
reserved and explicitly unallocated**, pending §3.11's catalyst tier (null
throughout the Step 7 run, so its 15 points were never exercised) and
re-validation at larger scale. Assigning that capacity requires evidence —
defaulting it into an existing component would be the unevidenced retune this
revision exists to avoid.

##### Why vol_accel, float and vwap were left alone

"Underpowered to detect" is not "shown to be useless." At 34 total hits these
three cannot be distinguished from noise in either direction, and cutting them
now would treat absence of evidence as evidence of absence. `vol_accel` in
particular is notable: it carries the largest single weight and §4.1 v1 was
specifically corrected to prioritise it, yet it shows nothing at this sample
size. That is a question for the next validation round, not a licence to cut it.

##### ⚠ This revision is IN-SAMPLE and awaits out-of-sample confirmation

The weights were revised using the same 218 candidates that revealed the
problem. Re-running the decile report against the revised weighting on those
same 218 candidates is a **sanity check that the logic is internally
consistent** — it would show improvement almost by construction, because the
components were selected on that data. **It is not evidence the revision
generalises.**

Real validation requires either the full-universe backfill or a fresh batch of
forward sessions that this exact 450-symbol set did not already inform. Until
one of those exists, v2 is a better-reasoned hypothesis than v1, not a validated
scoring model.

Standing limitations that apply to all of the above: survivorship bias is present
and unmeasured (the universe was pulled today, so delisted tickers — which are
disproportionately failures — are absent, making every hit rate optimistic);
`market_cap` and `shares_outstanding` are point-in-time today applied to every
historical row; and the pilot is 450 stratified symbols, not the full 4,975.

##### Drawdown: a Phase 2 candidate feature, deliberately NOT folded in

The score does not predict +100% hits, but it does predict **drawdown**,
monotonically across market-bucket quintiles (45.6% → 36.3% → 35.1% → 28.0%) and
across halves (43.4% vs 35.5%, a 7.9-point reduction for high scores). §6 tests
hit rate only, so this would have gone unnoticed.

It is recorded here and **kept separate from `momentum_score_100`**. A risk or
quality signal is a different thing from a hit-rate signal, and folding it into
the same number would make both harder to evaluate. It is a candidate for a
distinct signal later — possibly feeding §5's exit logic — and receives the same
treatment §3.12 gives `sector_strength_pct`: recorded, not scored.

**Where the drawdown signal actually lives — measured, not assumed.** Re-running
the report under v2 weights, the drawdown relationship **disappears**: 38.2% for
the low-score half against 39.2% for the high-score half, i.e. no advantage. The
monotonic 45.6% → 28.0% pattern was therefore being carried by `breakout` and
`high52w` — the very components zeroed for hit-rate inversion — and not by the
score as a whole.

That is a sharper and more useful finding than the original. `breakout_state` and
`pct_of_52w_high` are **inverted for predicting +100% moves and simultaneously
informative about drawdown**, which is coherent rather than contradictory: a
stock at a fresh high after a confirmed breakout is further along its run, so it
has less room left to gain and less distance to give back. Both readings describe
the same lateness.

The Phase 2 candidate is therefore **`breakout` and `high52w` as a risk/exit
signal**, not the total score. This is also why §4.1 v2 keeps both fields
computed: zeroing their scoring weight while preserving the features is exactly
what leaves that avenue open.

### 4.1.1 Original correction rationale (v1, for reference)

The original table listed both `Volume +25` and `Relative Volume +20`, which double-counts the same underlying quantity. Resolved as follows, preserving the total of 100:

| Component | Weight | Measures |
|---|---|---|
| Volume acceleration | 25 | Is volume *building* (`vol_accel`) |
| Relative volume | 20 | Is today unusual vs. the month (`rvol_20`) |
| Breakout | 20 | `breakout_state` |
| Catalyst | 15 | `catalyst_tier` |
| Float (est) | 10 | `float_shares_est` |
| Above VWAP | 5 | `above_vwap` |
| Near 52-week high | 5 | `pct_of_52w_high` |
| **Total** | **100** | |

This keeps the original intent ("volume increasing, not just high volume") and removes the overlap.

### 4.2 Sub-score formulas

All piecewise-linear and deterministic. Interpolate linearly inside each band; clamp at band edges. A null input scores **0** for that component (and the row records which components were null).

**Volume acceleration (0–25)** — input `vol_accel`
| Band | Points |
|---|---|
| < 1.0 | 0 |
| 1.0 – 1.5 | 0 → 10 |
| 1.5 – 2.5 | 10 → 20 |
| 2.5 – 4.0 | 20 → 25 |
| > 4.0 | 25 |

**Relative volume (0–35 in v2; 0–20 in v1)** — input `rvol_20`

v2 keeps v1's band **shape** and rescales it by 35/20 = 1.75. The evidence speaks
to rvol's weight, not to where its breakpoints belong, so changing the curve
would smuggle an unevidenced change in alongside an evidenced one.

| Band | v2 Points | (v1) |
|---|---|---|
| < 1.5 | 0 | 0 |
| 1.5 – 3.0 | 0 → 21 | 0 → 12 |
| 3.0 – 5.0 | 21 → 31.5 | 12 → 18 |
| 5.0 – 10.0 | 31.5 → 35 | 18 → 20 |
| > 10.0 | 35 | 20 |

**Breakout — ZEROED in v2** (was 0–20) — input `breakout_state`

Measured inverted, p=0.0008. `breakout_state` is **still computed and stored**;
it simply contributes no points. Implementation scales the v1 shape by
`WeightBreakout`, so restoring the weight restores these exact values.

| Value | v2 Points | (v1) | v1 fraction |
|---|---|---|---|
| `breakout_from_consolidation` | 0 | 20 | 1.0 |
| `breakout` | 0 | 14 | 0.7 |
| `approaching` | 0 | 8 | 0.4 |
| `none` | 0 | 0 | 0 |

**Catalyst (0–15)** — input `catalyst_tier`: `A` → 15, `B` → 8, `none` → 0.

**Float (0–10)** — input `float_shares_est`
| Band | Points |
|---|---|
| < 20M | 10 |
| 20M – 50M | 7 |
| 50M – 100M | 4 |
| 100M – 300M | 2 |
| > 300M | 0 |

**Above VWAP (0–5)** — `above_vwap` true → 5, false → 0.

**Near 52-week high — ZEROED in v2** (was 0–5) — input `pct_of_52w_high`

Measured inverted, p=0.0089. The ratio is **still computed and stored**. Note
that v1 rewarded the new-high case *most heavily*, and that is precisely the
signal that measured backwards: among already-gated candidates a fresh high marks
lateness, not quality.

| Band | v2 Points | (v1) | v1 fraction |
|---|---|---|---|
| ≥ 1.00 (new high) | 0 | 5 | 1.0 |
| 0.95 – 1.00 | 0 | 4 | 0.8 |
| 0.90 – 0.95 | 0 | 2 | 0.4 |
| < 0.90 | 0 | 0 | 0 |

§3.7's window excludes today, so `pct_of_52w_high` can exceed 1.0 and `>= 1.00`
*is* the new-52-week-high case. That remains true of the **feature**; in v2 the
band earns 0 points regardless.

### 4.3 Penalties

Applied after summing, then clamp to `[0, 100]`:

| Penalty | Condition | Points |
|---|---|---|
| Exhausted momentum | `rsi_14 > 85` | −5 |
| Already extended | `change_pct > 20` | −10 |
| Volume decaying | `vol_accel < 1.0` **and** `rvol_20 >= 3` | −5 |

The "already extended" penalty enforces the core thesis in the score itself: the system is meant to fire at +8–15% on a confirmed move, and a same-day +22% move is later in the sequence than the intended entry. The "volume decaying" penalty is the explicit case from the original brief — high volume that is *not* increasing should be visibly marked down rather than silently scored well.

### 4.4 Output contract

Persist, per symbol per day: the total, **every sub-score separately**, each penalty applied, and which inputs were null. A score with no visible breakdown is undebuggable, and Phase 2 needs the components independently.

This contract is what made §4.1 v2 possible. The per-component decomposition that
identified `breakout` and `high52w` as inverted could only be computed because
every sub-score was stored separately — a stored total alone would have shown
that the score failed and given no way to find out which part of it was wrong.
Zeroed components must keep being persisted for the same reason: their
relationship to the outcome still needs measuring at larger scale.

**The maximum attainable total is 90, not 100**, because 10 points are reserved
and unallocated (§4.1 v2). With `catalyst_tier` null until §3.11 ships in Step 8,
the practical ceiling today is **75**. Any alert threshold has to be read against
that ceiling rather than against 100.

#### Alert threshold — RECALIBRATED for v2 (2026-09-18)

`>= 60` was set against v1's weight table, where `breakout` and `high52w`
contributed up to 25 points. Under v2 those are zeroed, the allocated ceiling is
90, and the practical ceiling is 75 while `catalyst_tier` is null — **so 60 no
longer means what it meant, and carrying it over would silently change the alert
rate rather than preserve it.**

Measured against the 218 pilot candidates (base rate 15.60%):

| Threshold | fires on | hit% | lift | |
|---|---|---|---|---|
| ≥ 55 | 115 | 15.65 | 1.00× | no better than random |
| ≥ 60 | 75 (34.4%) | 18.67 | 1.20× | ← the carried-over value |
| ≥ 65 | 42 | 21.43 | 1.37× | |
| ≥ 69 (p90) | 23 | 26.09 | 1.67× | |
| ≥ 72 (p95) | 14 | 42.86 | 2.75× | |

v2 score distribution: min 26, p25 49, **median 55**, p75 63, p90 69, p95 72,
max 75.

Per bucket the picture is sharper, and it is why **the threshold must be
per-bucket rather than global**:

| Bucket | base rate | median | p90 | ≥60 lift | ≥p90 lift |
|---|---|---|---|---|---|
| `market` | 10.30% | 53 | 65 | 1.10× | **1.39×** (n=21) |
| `penny` | 32.08% | 62 | 72 | **0.91×** | **1.39×** (n=9) |

In the penny bucket a global 60 is **worse than useless**: penny's median score
is 62, so `>= 60` selects candidates that hit *less* often than the penny base
rate. A single global threshold does not mean the same thing in two buckets whose
score distributions differ by nine points and whose base rates differ threefold.

**Phase 1 defaults, each bucket's own 90th percentile:**

```
BOT_MOMENTUM_MIN_SCORE_MARKET=65
BOT_MOMENTUM_MIN_SCORE_PENNY=72
```

That is roughly 30 alerts across 18 months of pilot history — under two a month,
which is a scanner rather than a feed. **These percentiles are IN-SAMPLE**, drawn
from the same candidates §4.1 v2 was derived from; recalibrate when the universe
widens.

##### Running log: where the threshold sits relative to real candidates

Append one line per observed candidate. This exists so a future recalibration
reads *accumulated* observations rather than reconstructing them, and so no
single day's result gets mistaken for a pattern.

**Read the n before the numbers.** Every entry below is one symbol on one day.
The calibrated thresholds came from 218 candidates; a handful of live
observations cannot revise them and are not meant to.

| Date | Symbol | Bucket | Score | Threshold | Outcome | Note |
|---|---|---|---|---|---|---|
| 2026-09-17 | NEXR | penny | 68 | 72 | near miss, −4 | `rvol` contributed 32.7 of its 35; the one validated component did almost all the work |

**n=1. Nothing follows from this yet.** It is logged because it is the first real
candidate the calibrated thresholds were ever measured against, not because one
near miss says the threshold is wrong. A −4 gap is exactly what a 90th-percentile
cut is supposed to produce most of the time.

The question worth revisiting once there are perhaps 20–30 entries: whether
near-miss candidates go on to behave like the ones that cleared. That is
answerable from `momentum_features` — gate-passing rows are persisted regardless
of whether they cleared the alert threshold, which is what makes the comparison
possible without changing anything now.

> **The ~2-alerts-a-month figure is PILOT-SCALE, not a property of the approach.**
> It is measured over 450 stratified symbols of the 4,975 eligible — roughly 9% of
> the universe. Alert volume scales approximately with universe size, so the full
> universe at the same percentile thresholds would produce on the order of 15–20
> alerts a month, not two. Read as a rate per symbol-day rather than as an
> absolute: a future reader who sees "two a month" and concludes the strategy
> produces too few signals to be useful would be drawing the wrong conclusion from
> a subset measurement. The percentile thresholds themselves are what to carry
> forward; the absolute count is an artifact of pilot scope.

#### Diagnostic: rvol alone separates where the full score does not

Run because it is free and answers whether the unresolved components are merely
failing to add or are actively diluting. Same 218 candidates, ranked by one
component's sub-score instead of the total:

| Ranking by | bottom third | top third | lift | z | p |
|---|---|---|---|---|---|
| **v2 total** | 16.67% | 19.44% | 1.17× | +0.43 | 0.665 |
| **`rvol` alone** | 8.33% | 23.61% | **2.83×** | +2.50 | **0.012** |
| `vol_accel` alone | 16.67% | 22.22% | 1.33× | — | 0.400 |
| `float` alone | 16.67% | 13.89% | 0.83× | — | 0.643 |
| `vwap` alone | 20.83% | 13.89% | 0.67× | — | 0.271 |

Isolating `rvol` more than doubles the lift and crosses significance, while
isolating any of the other three does not — so this is not an artifact of
isolation itself. **The 40 points of unresolved components are substantially
diluting the 35 points of proven signal.**

That raises the urgency of resolving them, because they are costing separation
rather than merely not adding any. It does **not** justify adopting an
rvol-only score: that would be a third round of fitting on the same candidates
that identified rvol. The dilution mechanism — adding uncorrelated noise to a
signal reduces rank correlation — is more robust than the p-value attached to it,
and is the part worth carrying forward.

Alert threshold: post to Discord when a candidate clears its bucket's threshold
above (env-configurable, per bucket).

---

## 5. Sell-side logic — a gap in the original brief

The original brief specifies four channels including sell channels, but never defines a sell signal. Buy and sell are not symmetric: a sell signal requires knowing what was previously alerted. Phase 1 defines it as **exit tracking on prior alerts**, not shorting.

When a buy alert fires, insert a row in `momentum_tracked` with `status = 'active'`, the alert date, and `reference_price = close[t]`.

On each daily run, evaluate every `active` tracked symbol and emit a sell alert (then set `status = 'closed'` with the reason) on the **first** condition that matches:

| Exit reason | Condition |
|---|---|
| `breakout_failed` | `close[t] < resistance_20_at_alert` (the level it broke out over) |
| `lost_vwap` | `close[t] < vwap_20` |
| `momentum_stalled` | `rvol_20 < 1.5` for 3 consecutive sessions |
| `stop_atr` | `close[t] < reference_price - 2 * atr_14_at_alert` |
| `timeout` | 20 sessions elapsed with no exit condition met and no new high since alert |

Record the realized outcome (`max_gain_pct` while active, `exit_pct`) on the tracked row. This is free labeling data — it doubles as a live measure of whether the score is worth anything.

**These exit rules are a starting default, not a validated strategy.** They are deliberately mechanical so the backtest can evaluate and replace them. Nothing in this document constitutes trading advice, and the score is a research output, not a recommendation.

#### Step 10 replay — first measurement of §5 (2026-09-18)

`cmd/momentum-tracker -replay` walks the stored history with the same
`EvaluateExit` the live daily path uses, so replay and live results are
comparable rather than two implementations. It writes nothing. At the §4.4
thresholds over the 450-symbol pilot it produced **43 closed positions**:

| Exit reason | n | med exit% | med peak% | med sessions | gave back |
|---|---|---|---|---|---|
| `breakout_failed` | 19 | −6.19 | **0.00** | **1** | 6.19 pts |
| `lost_vwap` | 7 | −0.18 | 9.09 | 3 | 9.27 pts |
| `momentum_stalled` | 17 | +2.01 | 4.76 | 4 | 2.74 pts |
| `stop_atr` | 0 | — | — | — | never fired first |
| `timeout` | 0 | — | — | — | never true |

Overall: median exit −2.00%, median peak +1.87%. **Session survival: longest
position 9 sessions, 0 of 43 reached 20** — the figure that makes finding 2
interpretable.

**Two structural problems, both visible only because every matching condition is
recorded rather than just the acted-on one.**

**1. `breakout_failed` behaves as a same-day stop, not a breakout-failure rule.**
It fires on **median session 1** with a **median peak of 0.00%** — closing 19 of
43 positions before they ever traded above the alert price, at a median −6.19%.
The mechanism is arithmetic rather than mysterious: §3.6's `resistance_20`
excludes today, so a breakout candidate closes *above* it by construction, and
any next-session pullback through that level trips the exit. The rule as written
cannot distinguish "the breakout failed" from "the stock had one red day."

This is the rule §4.1 v2 already flagged for scrutiny, since it keys on the
geometry that measured inverted for entry. The replay says the concern was
warranted, though for a different reason than expected — the problem is the
rule's *sensitivity*, not the direction of its signal.

**2. Conditions 4 and 5 never fire — but for two DIFFERENT reasons, and the
difference decides what a fix would even look like.**

| Condition | fired first | also true | why it never fired |
|---|---|---|---|
| `breakout_failed` | 19 | 0 | |
| `lost_vwap` | 7 | 11 | outranked more often than it fires |
| `momentum_stalled` | 17 | 0 | |
| `stop_atr` | **0** | **3** | **suppressed by ORDERING** |
| `timeout` | **0** | **0** | **its trigger is never reached** |

These look like one finding in a summary table and are not. Anyone reordering
§5's priority list needs to know which is which:

- **`stop_atr` is an ordering artifact.** It became true three times and lost the
  priority contest every time. It is a working condition that never gets a turn,
  and **reordering would surface it immediately.**

- **`timeout` is not an ordering artifact, and reordering would not help.** It
  never became true at all. Measured session survival: **the longest-held
  position lasted 9 sessions, and 0 of 43 reached 20.** `timeout` requires 20
  sessions *and* no new high, so its trigger is simply never reached. Promoting
  it to first priority would change nothing. The only thing that would expose it
  is making the faster conditions less trigger-happy — which is finding 1.

So §5's ordering is doing more work than its thresholds for `stop_atr`, while for
`timeout` the ordering is irrelevant. A priority list whose 4th entry is
suppressed and whose 5th is unreachable is **three active rules plus two that
read as safety nets without functioning as one** — `timeout` in particular
appears to be a backstop against positions drifting indefinitely, and currently
cannot catch anything.

**None of this is a fix, and none of it is applied.** The rules are unchanged and
still exactly as §5 specifies.

That restraint is deliberate, not incompleteness. Tuning exit rules on 43 closed
positions drawn from the same 450-symbol pilot that produced §4.1 v2's in-sample
weights would be a fourth round of fitting the same data — and this document has
already recorded three corrections that were only possible because the full
decomposition was stored instead of the outcome (§3.2's gate failures, §4.4's
sub-scores, §5's matched-condition list). Replacing a measured baseline with a
hand-tuned one would trade that position away for an improvement nobody could
verify.

What exists now is the baseline, the dataset, and a replay harness that can test
an alternative ordering or a less twitchy `breakout_failed` without touching the
live path. Standard caveats apply: 43 positions is small, survivorship bias is
present, and fundamentals are point-in-time today.

---

## 6. The labeled dataset — the real Phase 1 deliverable

This is what makes Phase 2 possible, and it is the part most likely to be skipped under time pressure. Do not skip it.

Run the feature engine and scorer **over historical bars**, not just today, producing one feature row per symbol per day for the full backfill period (3 years). Then, for every historical row that passed the hard gates, compute forward-looking labels:

```
fwd_max_close   = max(close[t+1 .. t+H])          // H = 120 trading days
fwd_max_gain_pct = (fwd_max_close / close[t] - 1) * 100
days_to_peak     = argmax offset of fwd_max_close
fwd_max_drawdown_pct = max peak-to-trough decline within the window
hit_100  = fwd_max_gain_pct >= 100
hit_200  = fwd_max_gain_pct >= 200
hit_300  = fwd_max_gain_pct >= 300
hit_500  = fwd_max_gain_pct >= 500
hit_1000 = fwd_max_gain_pct >= 1000
```

Rules that keep this honest:
- **No lookahead.** Features for day `t` may use only bars up to and including `t`. Labels use only bars after `t`. Any leakage makes the whole exercise worthless, and it is easy to introduce accidentally — write a test that asserts it.
- **Rows inside the last `H` sessions have incomplete labels.** Mark them `label_complete = false` and exclude them from any evaluation.
- **Survivorship bias is present and must be documented.** A universe list pulled today omits delisted tickers, which are disproportionately failures. Phase 1 cannot fully fix this on free data. Record the limitation in the results; do not present the backtest as unbiased.
- Compute the **base rate**: what fraction of *all* gated candidates hit +100%? A score is only useful if high scores beat that base rate. Report score-decile hit rates against it. If they don't separate, the scoring weights are wrong and should be revised before anything is built on top of them.

---

## 7. Database schema

New migration `007_momentum.sql`. Follow existing conventions: TimescaleDB hypertables for time-series, `ON CONFLICT DO UPDATE` upserts everywhere (workers must be idempotent), and document every table in `shared/schemas/SCHEMAS.md` with a matching `*.schema.json`.

**Reuse `equity_ohlcv` for daily bars** — do not create a parallel bar table. Add rows with `source = 'yahoo_finance'` and `interval = '1Day'`.

> An earlier revision of this section said `source = 'yahoo'`. That value does not exist
> anywhere in the repo: `internal/fetch/yahoo` writes `yahoo_finance`, `data-analyzer`'s
> bar reader prefers `source = 'yahoo_finance'` when deduplicating across sources, and
> `SCHEMAS.md` documents `yahoo_finance`. Filtering on `'yahoo'` returns **zero rows** —
> a failure that looks like missing data rather than a wrong predicate. The code is
> authoritative here; `data_ingestion.md` also carried the wrong short form and has been
> corrected.

| Table | Type | Primary key | Purpose |
|---|---|---|---|
| `universe_symbols` | regular table | `(symbol, exchange)` | Eligible universe + `name`, `type`, `sector`, `industry`, `shares_outstanding`, `market_cap`, `is_eligible`, `excluded_reason`, `updated_at` |
| `momentum_features` | hypertable on `ts` | `(symbol, ts)` | Every feature from §3, one row per symbol per day, plus `bucket`, `gates_passed` (bool), `gate_failures` (text[]) |
| `momentum_scores` | hypertable on `ts` | `(symbol, ts)` | `bucket`, `momentum_score_100`, all sub-scores as individual columns, `penalties` (jsonb), `null_inputs` (text[]) |
| `momentum_labels` | hypertable on `ts` | `(symbol, ts)` | Forward labels from §6, plus `label_complete` |
| `catalyst_events` | hypertable on `ts` | `(symbol, ts, source, headline_hash)` | Every keyword match: `headline`, `url`, `matched_keyword`, `tier` |
| `momentum_tracked` | regular table | `(symbol, alerted_ts)` | Sell-side state from §5 |

Nullable columns to include now for later use without migration: `short_interest_pct`, `days_to_cover`, `is_halted`, `premarket_change_pct`.

---

## 8. Services

Two new services, both following the existing `data-ingestion` worker pattern (own `cmd/` binary, own Compose service, env-var config only).

### 2.2.1 Bar provider decision (resolved 2026-09-17)

**RESOLVED AGAINST TWELVE DATA. Tiingo is the primary bar provider.**

Twelve Data was adopted for throughput and withdrawn the same day: its
`adjust=all` applies adjustment inconsistently bar-by-bar inside a single
response, fabricating one-day moves up to **+1382%** across **12-17% of symbols
and 41% of the penny bucket**. For a momentum scanner that is the worst possible
defect — a fabricated +796% day is precisely the signal §3 hunts, so every
corrupted symbol ranks top of scan. A hybrid was rejected because "clean" cannot
be established locally: APAM, AQN and ARX disagree with Tiingo by 2-4% with no
detectable discontinuity. Full evidence in `data_ingestion.md`; the detector and
the ABTS fixture live in `internal/barquality`.

Tiingo costs ~9 hours for 450 symbols at its hard 50 requests/clock-hour. That is
accepted: the pilot is a one-off, unattended, checkpointed job, and correctness of
the adjustment is the premise the whole scanner rests on.

The throughput comparison below is retained because it was accurate.

Both satisfy §3's split- and dividend-adjusted requirement and agree to
**-0.0028% on close and +0.0000% on volume**. The decision was throughput and
quota shape, measured rather than assumed:

| | Twelve Data (free) | Tiingo Starter (free) |
|---|---|---|
| Rate | 8 credits/min (measured) | 50 req / **clock hour** (fixed-clock reset, measured) |
| Daily | 800 | 1,000 |
| Unique symbols | **none** | 500/month |
| 450-symbol backfill | **~56 min** | **~9 h** |

Tiingo's unique-symbol meter also leaves an unanswerable question after a failed
run: the free tier exposes no account-usage endpoint, so whether ~136 rejected
requests consumed allowance cannot be checked. Twelve Data removes the question.

All 450 were redrawn on Twelve Data rather than only the 386 Tiingo had not
reached, so the pilot has one adjustment methodology throughout. A 0.05%
discrepancy between two "adjusted" series is negligible until it falls on a §3.2
price threshold or a §3.6 breakout confirmation, at which point a single symbol
gates differently for a reason no one would think to look for. The 64 symbols
Tiingo had completed are kept, giving 64 names with two independent adjusted
series as a correctness check on the new adapter.

**`adjust=all` is mandatory.** Twelve Data's default is split-adjusted but NOT
dividend-adjusted (a constant +6.32% / +7.52% above the adjusted close for ALL
and AWR over three years). The adapter always sends it and a test pins it.

**Volume adjustment was verified against a real split, not inferred.** Measured
on NVDA's 10:1 split of 2024-06-10, last pre-split session:

```
                        close        volume
Tiingo raw              1208.88      41,238,580
Tiingo fully adjusted    120.5447    412,385,800
Twelve Data adjust=all   120.5414    412,386,000   = 10.0000x raw
```

Volume is split-adjusted in both modes; `adjust=all` affects price only, which is
correct because dividends do not change share count. This was checked because a
provider rescaling OHLC but not volume would silently corrupt §3.4's RVOL for
every symbol that ever split — the same defect class as the Yahoo dividend gap.

### 8.1 `data-universe` (Go)

`services/data-ingestion/cmd/data-universe/main.go`

Responsibilities:
1. **Weekly:** refresh `universe_symbols` from Finnhub US symbol list; apply §3.1 eligibility; record `excluded_reason` for audit.
2. **Weekly:** refresh `shares_outstanding`, `market_cap`, `sector`, `industry` from existing fundamentals ingestion.
3. **Once, before the pilot subset is drawn:** fetch an approximate current price for every eligible symbol from Finnhub `/quote`, stored as `equity_ohlcv` rows with `interval='quote_snapshot'`, `source='finnhub_quote'`. See §8.1.1.
4. **Once, resumable:** backfill 3 years of split- and dividend-adjusted daily bars into `equity_ohlcv`, checkpointed per symbol. Bar provider is selected by `UNIVERSE_BAR_SOURCE`; an unrecognised value fails at startup rather than defaulting.
5. **Daily after close:** incremental bar refresh.

#### 8.1.1 The stratification bootstrap, and why pricing is a separate pass

Stratifying the pilot subset by §3.2's price buckets needs a price per symbol. Prices come from the bar backfill. The backfill only runs over the subset the stratification chooses. That is a genuine circular dependency, and it has to be broken by a price source that is not the bar provider.

It is **not** broken by drawing twice from the bar provider. Two candidate schemes were rejected:

| Rejected approach | Why |
|---|---|
| Backfill ~100 symbols, then re-stratify against their prices | Spends ~550 of Tiingo's 500-unique-symbols/month allowance across the two draws, and the bootstrap's 100 symbols and the final 450 come from *different random processes* — nothing guarantees the bucket boundaries learned from the first 100 generalise before the real selection is committed |
| Accept an unstratified first month | Trades away the penny bucket exactly when the base rate is being established; a 9 % sampling rate over a ~4 %-penny universe can plausibly return zero penny symbols, leaving half the gate logic unexercised |

The resolution is to price the universe from a source the repo already pays for. Finnhub `/quote` is already integrated, already paced through the shared Postgres budget in `api_rate_budget`, and costs **zero** bar-provider symbols. At the existing 1 req/sec Finnhub budget, pricing ~4,978 eligible symbols takes roughly **83 minutes**, once. The stratified draw then runs against real bucket membership for the whole universe, and hands the resulting set to the bar backfill as its one and only touch.

Beyond being cheaper, this is the more correct shape: one signal covering every symbol, one draw, and no bootstrap-versus-final consistency question to reason about.

**Caveat, accepted and monitored:** Finnhub `/quote` is a live snapshot, not the adjusted close the bar provider will later serve. A handful of borderline symbols (the $1.98-versus-$2.02 case) can therefore land in a different bucket once real bars arrive. This is accepted rather than engineered around — but it is not ignored: `LoadSubsetStats` compares each selected symbol's selection-time bucket against its post-backfill bucket and warns with examples, reported on the backfill's drained edge. A drifted symbol is scored against the bucket its adjusted close implies.

Symbols with no usable quote are left without a price rather than stored as zero; a zero close would bucket them as penny and quietly corrupt the stratification.

New fetcher work needed: a per-symbol company-news method on the existing Finnhub client (for §3.11), writing to `catalyst_events`. Called only for gated candidates.

### 8.2 `momentum-scanner` (Go)

New service `services/momentum-scanner/`, or a new `cmd/` under `data-analyzer` if you prefer to keep the analysis binaries together — `data-analyzer` already owns the indicator library this needs, which argues for putting it there.

Responsibilities:
1. **Daily after bar refresh:** compute §3 features for all eligible symbols → `momentum_features`.
2. Apply hard gates (§3.2), assign bucket, trigger news fetch for candidates.
3. Compute `momentum_score_100` (§4) → `momentum_scores`.
4. Evaluate sell conditions (§5) → update `momentum_tracked`.
5. **Backfill mode (CLI flag):** run steps 1–3 across the full history, then compute labels (§6) → `momentum_labels`. Plus a report command printing score-decile hit rates vs. base rate.

Reuse `compute/` for ATR, RSI, Donchian, VWAP, swings rather than reimplementing. Unit-test every feature formula in §3 against hand-computed fixtures — these formulas are the whole product, and a silent off-by-one in a window boundary is invisible in output but fatal to the results.

### 8.3 `analyst-bot` additions (Python)

**Extend the existing bot. Do not create a second bot.** The embed formatter, batching, notifier abstraction, scheduler, and Redis cooldown machinery all already work and are documented in the root `README.md`.

New files:
- `db/queries/momentum.py` — queries against `momentum_scores` / `momentum_features` / `momentum_tracked`, following the pattern of the existing `queries/technical.py`.
- `scheduler/jobs/momentum_scan.py` — `MomentumScanJob`, registered alongside the existing `AlertScanJob`.
- Formatter additions in `notifier/discord/formatter.py` for the momentum embed.

New channels (four new env vars alongside the existing four):
```
DISCORD_PENNY_BUY_CHANNEL_ID
DISCORD_PENNY_SELL_CHANNEL_ID
DISCORD_MARKET_BUY_CHANNEL_ID
DISCORD_MARKET_SELL_CHANNEL_ID
```
Follow the existing convention: an unset channel ID logs a warning and silently produces nothing — **it must never crash the bot.**

New slash commands — these are the "fetch dynamically" requirement, and they are the reason the bot needs a query change rather than a redesign. The existing commands only operate on the statically configured `BOT_EQUITY_SYMBOLS`/`BOT_CRYPTO_SYMBOLS` lists; these must query the scanner's result set instead:

| Command | Behaviour |
|---|---|
| `/scanner [bucket] [min_score]` | Top N by `momentum_score_100` from today's `momentum_scores`, independent of any configured watchlist |
| `/score TICKER` | Full score breakdown for any symbol in `universe_symbols`, not just configured ones — all sub-scores, gate pass/fail with reasons, null inputs |
| `/tracked` | Current `active` rows in `momentum_tracked` with unrealized move |

Reuse the existing Redis alert-cooldown pattern so a symbol that stays qualified for a week doesn't alert daily. Default cooldown: one alert per symbol per bucket per 5 sessions.

**Embed content** (follow root `README.md` conventions — embeds only, `—` for nulls, existing colour semantics):

```
🟩 ┃ 🔥 XYZ  +11.8%   Score 89/100
   ┃
   ┃ RVOL          Vol accel      Volume
   ┃ 7.4x          2.1x           4.2M
   ┃
   ┃ Breakout      52W high       VWAP
   ┃ consolidation 97%            +4.1%
   ┃
   ┃ Float (est)   Market cap     Catalyst
   ┃ 18M           $850M          A — FDA clearance
   ┃
   ┃ Score breakdown
   ┃ accel 21 · rvol 19 · breakout 20 · catalyst 15 · float 10 · vwap 5 · 52w 4  (−5 rsi)
   ┃
   ┃ Daily bars · regular session only · EOD scan 2026-09-15
```

The breakdown must reconcile: the seven sub-scores sum to 94, the single applied penalty is
**−5** (`rsi_14 > 85`, §4.3), and 94 − 5 = 89. §4.3 is the definition — an earlier revision of
this mockup showed `(−2 rsi)` totalling 92, which contradicted it. If a future mockup and §4.3
disagree again, §4.3 wins.

The footer must state **"regular session only"** and the scan date. Free-tier daily data is easy to misread as live, and a stale-looking number with no provenance is how a research tool turns into a bad decision.

---

### 8.4 `data-fundamental` widening (build-order step 3b)

Step 3b's design, recorded here because it changes a **shared worker with live
consumers** rather than adding a new one. `data-fundamental` currently feeds the
bot's existing daily-report output for its configured symbols, and that must not
regress.

#### Only one of eight sub-tasks needs widening

`data-fundamental` runs **eight** independent sub-tasks, each on its own ticker and
enable flag: `runMetrics`, `runFinancials`, `runEarnings`, `runOverview`,
`runRecommendations`, `runInsiderTransactions`, `runNewsSentiment`,
`runInstitutionalOwnership`.

The scanner needs exactly two fields, and both come from one of them:
`market_cap` and `shares_outstanding` are written by **`runMetrics`** from Finnhub
`/stock/metric`. Those are the inputs to §3.2's market-cap gate and §3.9's
`market_cap_est` proxy. The other seven are irrelevant to Phase 1 and are **not
touched**.

`sector` / `industry` are the exception, and they are unreachable rather than
merely expensive — see §3.13. They stay null in Phase 1 at zero cost to
`momentum_score_100`.

This is what collapses 3b from "widen a worker" to "widen one sub-task": about
3.3 hours per universe pass rather than 23, and one checkpoint rather than eight.

#### Symbol source: union, never replacement

A per-sub-task flag, `FUNDAMENTAL_METRICS_SYMBOL_SOURCE = env | env+universe`,
defaulting to `env`. The other seven sub-tasks keep reading the configured list
with no flag and no new code path.

When enabled, `runMetrics` iterates:

```
FUNDAMENTAL_SYMBOLS  ∪  (universe_symbols WHERE is_eligible)
```

**Union, not replacement, and this is load-bearing.** The configured list contains
`SPY`, an ETF that by construction is never in `universe_symbols` because §3.1
excludes ETFs. Replacement would silently drop the single most visible symbol in
the daily report — a structural change quietly degrading something that already
works, which is the same failure the float and market-cap proxies exist to avoid.
Union also means an empty or missing `universe_symbols` cannot regress anything.

If the universe query errors or returns zero rows, fall back to the env list and
warn. Never fail closed into fetching nothing.

#### Checkpoint state: a new table, keyed `(symbol, task)`

New table `fundamental_fetch_state` in migration `009`, primary key
`(symbol, task)`, with the same columns as step 3's bar backfill: `status`,
`claimed_at`, `attempts`, `last_error`, `completed_at`, `last_success_ts`.

Two rejected homes, and the reasons generalise:

- **Not on `universe_symbols`.** That is the scanner's table; `data-fundamental` is
  a shared worker whose other seven sub-tasks are not universe-scoped. Coupling a
  shared worker's job state to a feature-specific table points the dependency the
  wrong way.
- **Not reusing `universe_symbols.backfill_*`.** Those belong to the bar backfill.
  Sharing them would make a fundamentals failure indistinguishable from a bar
  failure in precisely the column someone would check during an incident. *"Would
  these two failures be distinguishable at 3 a.m.?"* is the test to apply when
  tempted to reuse a state column.

Per the migration rule in `shared/schemas/SCHEMAS.md`, this is a **new migration**,
not an amendment to `007` or `008`.

#### Leases are independent per `(symbol, task)`

The primary key *is* the lease granularity, so a symbol whose metric call
succeeded and whose earnings call failed retries only earnings — it never redoes
completed work.

Only one task row per symbol exists today, since only `runMetrics` is widened. The
per-task key is kept anyway: it costs nothing now and avoids a migration later,
which matters because the Alpha Vantage quota wall (§3.13) is the kind of
constraint a paid tier eventually removes, at which point `runOverview` becomes a
widening candidate too.

Claim semantics are **reused from step 3, not reinvented**: claim sets
`in_progress` plus `claimed_at`, claims older than a configurable lease are
reclaimable so a killed worker strands nothing, attempts are capped, and the error
text is persisted so a stall is diagnosable from SQL alone.

#### Cadence

`runMetrics` fires on a 24-hour ticker today. At ~3.3 hours per universe pass that
would be seven passes and ~23 hours of API time per week, against §2.3 and §8.1.2
which both specify **weekly**. The widened path therefore gets its own
`FUNDAMENTAL_METRICS_UNIVERSE_POLL_INTERVAL`, default `168h`, rather than
inheriting the 24-hour cadence — a cadence change to a shared worker should be a
stated decision, not an inherited default.

#### Prerequisite

The widened pass is the first workload in this repo to saturate a Finnhub
allowance continuously for hours, which is why the **shared rate limiter**
(migration `008`, `internal/ratelimit`) had to land first. Without it the 429s
would surface in whichever *other* Finnhub-calling worker happened to be running.
Full rationale in `shared/schemas/SCHEMAS.md` under `api_rate_budget`.

---

## 9. Configuration

Every threshold in §3.2, §4, and §5 is an env var with the documented value as default. Follow the existing `.env` conventions and the `SCANNER_` / `MOMENTUM_` prefix pattern.

⚠️ The repo's `.env.example` is already documented as incomplete for the bot (missing Discord channel IDs, alert thresholds, cron/scan schedule, actions settings, cache TTLs). **Add the new scanner variables to `.env.example` properly** rather than extending that gap — and while there, consider fixing the existing omissions.

Minimum set to expose: bucket price/cap/change/rvol/dollar-volume bounds, alert score threshold per bucket, alert cooldown sessions, scan cron, backfill years, request rate/concurrency, catalyst keyword file path, catalyst lookback hours, label horizon `H`, and all four channel IDs.

---

## 10. Build order

Each step should produce something verifiable before the next begins.

| Step | Deliverable | Done when |
|---|---|---|
| 1 | Migration `007_momentum.sql` + schema docs + `*.schema.json` | Tables exist; `SCHEMAS.md` updated |
| 2 | `data-universe`: symbol list + eligibility | `universe_symbols` populated, ~5–7.5k eligible, exclusions auditable |
| 3 | `data-universe`: resumable 3-year bar backfill **+ the §8.1.4 daily incremental refresh** | `equity_ohlcv` has `source='yahoo_finance'` daily bars for the universe; job survives a kill and resumes; the daily short-window refresh keeps them current. §8.1.4 is folded in here because §10 originally gave it no step of its own, and without it bars go stale the day after the backfill completes |
| **3b** | **`data-fundamental`: widen symbol source to the eligible universe** | **One full pass completed against the real universe; `universe_symbols.market_cap` / `shares_outstanding` / `sector` populated for the bulk of it. HARD PREREQUISITE FOR STEP 5 — see below.** |
| 4 | Feature engine (§3) + unit tests against hand-computed fixtures | Every formula tested; a no-lookahead test passes. **Delivered** as `services/data-analyzer/internal/momentum` — pure functions over bars, 25 tests. `ComputeAt(bars, i)` is the primitive so no-lookahead is *testable*: its output must equal `Compute(bars[:i+1])` for every `i`, which a forward index cannot satisfy. Mutation-verified — injecting a deliberate leak fails the test and names the leaking field |
| 5 | Hard gates + bucketing | Candidate counts per day are sane (tens to low hundreds, not thousands) |
| 6 | Scorer (§4) with full sub-score persistence | `/score` output reconstructs the total exactly |
| 7 | **Backfill + labels + base-rate report (§6)** | Score-decile hit rates vs. base rate printed. **This is the go/no-go gate.** |
| 8 | Catalyst keyword classifier + `catalyst_events` | Matches stored with keyword and tier |
| 9 | `analyst-bot`: queries, job, formatter, 4 channels, 3 commands | Alerts post; unset channel IDs don't crash |
| 10 | Sell-side tracking (§5) | Tracked rows open and close with recorded reasons |

### Step 3b — the fundamentals-coverage prerequisite

This step was not in the original build order. It exists because step 2 surfaced a
gap that makes steps 5 and 7 unreadable if left alone.

**The gap.** §8.1.2 says to refresh sector, shares outstanding and market cap "from
existing fundamentals ingestion", and `data-universe` does exactly that — it reads
`equity_fundamentals` and makes no API calls. But `data-fundamental` fetches only
the symbols in its static `FUNDAMENTAL_SYMBOLS` env var, which defaults to three
tickers. So market cap and sector are populated for a handful of symbols out of
several thousand.

**Why that is fatal rather than cosmetic.** `market_cap` is load-bearing in §3.2 for
*both* buckets — the market bucket needs $300M–$10B, the penny bucket needs
≤ $300M — so it decides bucket membership, not just ranking. With coverage near
zero, almost every symbol fails the market-cap gate, candidate counts collapse, and
§6's base rate is computed over a tiny non-representative slice. Step 5's
"candidate counts are sane" check would pass for entirely the wrong reason.

**The fix.** Point `data-fundamental`'s symbol source at `universe_symbols WHERE
is_eligible` instead of the static env list, behind a feature flag so existing
non-scanner use of that worker is unaffected. **The full design — which single
sub-task is widened, the union-not-replacement resolver, the
`fundamental_fetch_state` table shape, per-`(symbol, task)` leases, and the
cadence decision — is in §8.4.** Weekly cadence, which is what §2.3
and §8.1.2 already specify — the ~4-hour runtime at one request per two seconds is
not a new cost, it is the job the spec already assumed, at the scale that had not
been built yet. Reuse the same `backfill_status` / `backfill_cursor_ts` checkpoint
pattern as the step 3 bar backfill rather than inventing a second one: a
multi-hour rate-limited pass must survive a restart for the same reasons.

Rejected alternatives: a cap-free gate (market cap is not optional, per the above),
and a second metric-fetch pass inside `data-universe` (duplicates a worker that
already exists, which §0 and §12 both forbid).

**Sequencing.** Steps 3 and 3b are independent and may run in parallel. **Do not
start step 5 until both have completed a full pass against the real universe.** The
temptation is to treat 3b as "not blocking steps 3 or 4" — true, and irrelevant,
because step 5 is where it bites and step 7 is where it becomes unrecoverable.

**Step 7 is the decision point.** If high-score deciles do not beat the base rate for `hit_100`, the scoring weights in §4 are wrong. The correct response is to revise the weights (or the features) using the labeled data — not to proceed to Phase 2 or build a UI on top of a score with no demonstrated edge. This ordering exists specifically so that finding out is cheap.

---

## 10.1 Phase 1 — post-upgrade validation (2026-09-21)

Run after the Tiingo Power upgrade. **A measurement step, not a tuning step:**
no weight, threshold, gate or exit rule was changed. Everything below is
reported as measured, including the results that go against the design.

### 10.1.0-STATUS Final status: the scanner is a screener (2026-09-22)

**Phase 1 is complete. Its scoring model is not validated and will not be
used for ranking.**

The pre-registered routing measurement (Phase 2 §2.1) returned **feature
research only**: on a candidate set selected without lookahead, `rvol_20`'s
stratified odds ratio is **0.991 (p = 0.947)**, and no other component
separates either. The user confirmed the route on the rule, unamended.

What Phase 1 delivered, stated plainly:

- **A working screener.** The §3.2 gates are a real, reproducible filter:
  ~4.5 candidates/day across a 4,975-symbol universe that are up 8-15%
  (10-40% penny) on unusual volume and clear the liquidity and size bands.
  That ships.
- **No demonstrated ranking ability.** `momentum_score_100` is retained as a
  Phase 2 research baseline and is absent from every alert.
- **A measurement of why the earlier answer was wrong.** Gate v1 applied
  today's market cap to historical rows, which preferentially admitted
  companies that grew — and companies that grew are companies that ran. The
  leak-corrected group hit at 20.06% against 8.73% in the retained set, and
  `rvol`'s apparent edge lived there (§10.1.5c).

The bot runs in screener mode (Phase 2 §8.5). Scanning stays disabled until
enabled explicitly.

### 10.1.0-CA Corporate actions in the daily refresh — a seam we were about to create

Found 2026-09-22 while investigating an apparent NEXR feature discrepancy. The
discrepancy was not real (see below); the defect it prompted a look at was.

**The defect.** Adjusted prices are rewritten BACKWARDS on every split and every
dividend. `runDailyBars` fetched only `[now - lookback, now]` and upserted. So
the first corporate action after a symbol's backfill would leave its stored
history on the OLD adjustment basis and the newly written bars on the NEW one.
Every windowed feature spanning the boundary — 20-bar RVOL, 252-bar highs —
would then be computed across two price scales.

**This is the Twelve Data seam defect, self-inflicted.** That provider was
rejected in §2.2.1 for alternating adjustment bases within one response. This
would have produced the same corruption from our own refresh loop, one symbol
at a time, invisibly — because each individual refresh looks correct in
isolation and only the join is wrong.

**Nothing is currently corrupted.** The 10-year backfill ran as a single pass on
2026-09-21, so every bar is on one basis by construction, and the daily refresh
has not run since. NEXR — five reverse splits, most recently 2026-07-31 — shows
a cumulative adjustment factor of exactly 11.0000 before that split and 1.0000
after, with no unexplained step. The exposure was entirely forward-looking.

**The fix.** The action is the trigger: when any bar in the refresh window
carries `splitFactor != 1` or `divCash != 0`, the symbol's FULL history is
re-fetched onto one basis. Checked on both fields, because either alone leaves
the other's seams — dividends rewrite the series exactly as splits do, and they
are far more frequent. `div_cash` is now captured (migration 021), which also
closes §2.3's open question.

**A direct seam detector, now possible.** With `raw_close` stored,
`close / raw_close` is the cumulative adjustment factor, which must be constant
between corporate actions and step only at one. A step with no split and no
dividend has no legitimate mechanism. That is arithmetic rather than the
inference `barquality` makes from price shape, and it is what
`scripts/seam_audit.py` checks.

First run flagged **52,403 steps across 1,869 symbols** — and they are **not
seams**. They are dividend adjustments that the audit could not attribute
because `div_cash` is NULL on every pre-migration bar. Confirmed directly:
symbol A steps on 2016-09-30, 2016-12-29, 2017-03-31 (quarterly), and Tiingo
reports `divCash = 0.115` on 2016-09-30. A universe-wide re-fetch is populating
`div_cash` so the audit can discriminate; until it completes the audit
over-reports, which is the conservative direction.

#### A stale daemon silently wrote half the re-fetch on the old code path

Worth recording because the symptom was indistinguishable from a data bug.

After deploying `div_cash` capture and re-fetching the universe, the backfill
reported `done=4975` while only **64.5% of bars** carried `div_cash` — 1,773
symbols had none at all, despite clearly having been re-fetched (their newest
bar was later than the previous run's).

The source was correct, the binary was newer than the source, and Tiingo
returns `divCash: 0.0` for the affected symbols. Nothing about the code
explained it.

**Cause: a `data-universe` process launched the previous evening was still
running, 13.5 hours later, on the OLD binary.** Rebuilding `/tmp/data-universe`
replaced the file but not the running process, and both instances were claiming
symbols from the same `backfill_status='pending'` queue. The old one wrote bars
without `div_cash`; the new one wrote them with. The 64.5% was the race's split.

Three things generalise:

1. **A rebuilt binary does not restart a running daemon**, and this repo's
   workers are daemons with internal timers, not one-shot jobs. Any
   deploy-then-verify cycle has to kill the old process explicitly.
2. **A shared work queue makes two versions look like one inconsistent
   version.** Because claims are atomic, neither process errored and neither
   log showed anything unusual. The only visible artefact was a column that
   was populated for some rows and not others — which reads as a bug in the
   column.
3. **`done` meant "every symbol was claimed and written by someone"**, which
   is a weaker statement than it appears when more than one writer exists.

Repair: stale processes killed, symbols with any missing `div_cash` requeued,
re-fetched by a single verified instance. The seam audit runs only after that
completes, because running it against a half-old dataset would have produced
exactly the unattributed steps it is meant to detect — and they would have been
our own artefact, not a provider one.

#### Seam-audit acceptance criterion — set 2026-09-22, BEFORE the re-fetch finished

Written before the result, for the same reason the routing rule was.

> **Criterion.** After the `div_cash` re-fetch, every step in
> `close / raw_close` must be attributable to a `split_factor` or a `div_cash`
> recorded on that same session.
>
> **Tolerance: a step is REAL when the relative change exceeds 1e-3
> (0.1%); an attribution MATCHES when predicted and observed factors agree to
> within 1e-3 relative.**
>
> **Clean = ZERO unattributed steps, or a short list where each one is
> individually explained.** A count alone does not clear it.
>
> **Any unexplained step blocks P2-3 until it is understood.**

**Why 1e-3, both ways.** Two numbers set the floor and the ceiling:

- *Floor.* `close` and `raw_close` are stored as `double precision` but arrive
  from Tiingo already rounded to 4 decimal places. On a sub-dollar penny name
  a 4-dp rounding is worth up to ~1e-4 relative on each of the two values, so
  their ratio carries a few parts in 10,000 of pure quantisation noise. A
  tolerance at 1e-4 would flag rounding; 1e-3 sits an order of magnitude above
  it.
- *Ceiling.* The smallest real adjustment we need to catch is a modest
  quarterly dividend. Symbol A's 2016-09-30 dividend of $0.115 on a $47 close
  moved the factor from 0.9249 to 0.9272 — **2.5e-3 relative**. That is above
  1e-3, so the tolerance still catches the smallest economically meaningful
  action in this dataset, with roughly 2.5x margin.

Anything between those bounds is unresolvable with 4-dp inputs and is
deliberately not chased.

**Also reported:** how many full re-fetches the corporate-action trigger causes
per day on average, measured from recent history, so the ongoing daily cost of
the fix is a known number rather than an assumption.

#### Audit RESULT (2026-09-22) — criterion met, and it caught a real seam we made

Final state after the `div_cash` re-fetch. LIVE, from `scripts/seam_audit.py`:

```
bars: 9,205,082 across 4,971 symbols   raw_close 100%  split_factor 100%  div_cash 100%
steps flagged by the ratio test:   336
  spanning >=1 missed session:     332   EXPLAINED (action fell on an unstored session)
  UNATTRIBUTED:                      4
```

**The criterion permits a short individually-explained list. All four:**

| # | Step | Explanation |
|---|---|---|
| 1 | ADTN 2023-08-16 | Tiingo begins a dividend adjustment ~2 sessions BEFORE the ex-date. The step magnitude is exactly `1/(1 + 0.09/8.14)` — error **4e-7**. The dividend is real and recorded; only its start date is early. |
| 2 | BRN 2023-08-21 | Same, `1/(1 + 0.015/2.46)`, error **4e-7**. |
| 3-4 | JCSE 2024-12-10, 2026-01-29 | `div_cash` NULL. Tiingo now serves **1 bar total** for JCSE over an 11-year window, so its 1,107 stored bars are an unrefreshable remnant. `HWH` is the same, returning **0 bars** (`no_bars_returned`). |

**Zero seams of our own making remain.** 1 and 2 are a provider timing quirk with
the magnitude exactly right; 3 and 4 are symbols whose history the provider no
longer serves.

That last class is worth carrying into P2-3: **Tiingo silently stops serving
back history for some delisted names**, which is survivorship acting at the
provider layer rather than in our universe filter.

#### The audit caught a seam the REPAIR created

Not a hypothetical. The first clean-ish run left 13 flags all on **2016-09-22**,
the second bar of the window. Cause:

```
2026-09-21  backfill stores bars from 2016-09-21
2026-09-21  DTE, EPM, WHF go EX-DIVIDEND -> adjusted history rewritten backwards
2026-09-22  re-fetch uses now-10y = 2016-09-22 as its start
            -> the 2016-09-21 bar falls OUTSIDE the window, keeps the OLD basis
```

A rolling `now - N years` window advances daily, so **every re-run orphans the
previous run's oldest bar**. `refetchFullHistory` now anchors to the earliest
STORED bar instead, with a one-week buffer.

The audit found this within hours of the mechanism being introduced, on 17
symbols, which is the strongest available evidence that it works: it detected a
one-bar, 1%-scale inconsistency in 9.2M bars.

#### Ongoing cost of the corporate-action trigger

LIVE, last 250 sessions:

| | full re-fetches/day |
|---|---|
| mean | **27.1** |
| median | 21 |
| p90 | 60 |
| worst day | 110 |

At ~2,500 bars per 10-year re-fetch and 2 req/s, a median day adds ~21 requests
(one per symbol) to the ~4,975 of the daily refresh — **under 0.5%** of the
90,000/day budget. Even the worst day is 110 extra requests. The fix is
effectively free; what it costs is wall clock on the write path, not quota.

#### The NEXR "discrepancy" was a synthetic test fixture

Reported as: the same symbol-day showing RVOL 6.7x / RSI 36 / 52W 0.19% in the
live test and 4.2x / 68 / 41% in a later report, with the hypothesis that the
10-year re-backfill fetched a different adjusted history.

**It did not.** The second set of numbers came from `sample_row()`, the
hardcoded fixture in `test_momentum.py`, rendered while demonstrating the
screener embed. Only `change_pct` and `dollar_volume` in that fixture were
copied from real data, which is what made it look like the same bar.

Querying the stored series for that exact day returns RVOL **6.74** and 52W
high **0.22%** — reproducing the live test. The re-backfill changed nothing.

Recorded because the inference was correct given what was shown, and because
the lesson is about presentation rather than data: **rendering fixture output
as "the actual output" invites exactly this**, and it cost a real investigation
to rule out. The investigation was still worth it — it found the refresh
defect.

### 10.1.0 Rulings (recorded 2026-09-21, after review)

**1. §4.1 v2 is NOT CONFIRMED out-of-sample.**

The pooled B+C test returned p = 0.017 and that verdict is withdrawn. The
separation is a **bucket-composition artifact**: v2 separates in neither bucket
(penny p = 0.891, market p = 0.645), the top third holds six times the penny
share of the bottom third (9.2% against 1.6%), penny names hit at 35% against
market names' 9%, and 97% of the apparent effect is explained by that mix alone
— leaving 0.07pp within buckets on 6,924 episodes. Full decomposition in
§10.1.4.

**The pooled criterion was a flaw in the pre-registration itself, not in the
analysis.** Criterion 1 was written as a single pooled B+C test, and pooling
across buckets is precisely what allows composition to pose as ranking. The
analysis applied the criterion correctly and the criterion was wrong. This is
worth recording plainly because the pre-registration was supposed to be the
safeguard, and a safeguard that can be satisfied by an artifact is the more
dangerous kind of error — it converts a wrong answer into an authorised one.

Consequence for Phase 2, now binding in its §4.2: **every acceptance test must
be bucket-stratified**, reported per bucket and/or combined with a stratified
test such as Cochran-Mantel-Haenszel. **Pooled-across-bucket tests are forbidden
as acceptance evidence.**

**2. Three findings recorded. No weight changes.**

| Finding | Status |
|---|---|
| `rvol_20` replicated out-of-sample | **SUPERSEDED 2026-09-21.** Confirmed on the gate-v1 sample (§10.1.5a), but that sample was selected with today's market cap. On the gate-v2 sample it does **not** separate within either bucket and the MH odds ratio is 0.983 (§10.1.5b) |
| The `high52w` inversion replicated | **CONFIRMED** (-7.65pp, p < 0.001) |
| `breakout_state` is two opposite effects under one name | **RECORDED** as a design finding: plain `breakout` hits 20.89%, `breakout_from_consolidation` 6.42% |

Weights, thresholds, gates and exits are unchanged.
`BOT_MOMENTUM_SCAN_ENABLE` remains **false** — the 70/67 threshold agreement
(§10.1.8) governs alert *volume*, and volume calibration is not a reason to
alert on a score that does not rank within buckets.

**3. `rvol_20` is genuine but small — and falls BELOW Phase 2's own minimum
effect size.**

It survives stratification (CMH p = 0.017 by bucket, p = 0.037 by bucket x atr
tercile), so it is a real signal. But:

- **MH common odds ratio 1.215** (bucket) and **1.192** (bucket x atr tercile).
- **Phase 2 §4.2.0 Rule 2 sets the floor at MH OR >= 1.25.**
- **Therefore, under Phase 2's own binding rule, `rvol_20` is NOT a validated
  component.** It is a real effect that is too small to accept as evidence for
  shipping anything.
- ~64% of its pooled effect was composition, and what survives is concentrated
  in the top `atr_pct` tercile (+9.68pp there, +0.26pp in the bottom).

**The bar stays where it is.** Two reasons worth recording:

1. **It was set in the same batch as this test, and it cuts against the known
   result rather than admitting it.** An effect-size floor chosen after seeing
   that rvol lands at 1.19-1.215, and set at 1.15 so that rvol passes, would be
   the same in-sample fitting this whole protocol exists to prevent — only
   applied to the acceptance rule instead of the weights. The floor was written
   to exclude results of the size this project has actually been producing, and
   it does, including the one result we like.
2. Raising rvol to "validated" would license exactly the decision that has
   already been declined twice: staking alerts on a small effect concentrated in
   the names with the worst drawdowns (§4.1).

**No switch to an rvol-only score.** It is now out-of-sample supported and
stratification-robust, so it is defensible in principle, and it still fails the
effect-size floor. It remains a Phase 2 baseline (§5.3) and a model feature, not
an alerting rule.

**4. What this implies for Phase 2's expected outcome.**

The only signal found across the whole post-upgrade validation is small, below
the acceptance floor, and concentrated in the most volatile names — which are
also the names that crash hardest. **"No edge on daily bars" is now a realistic
Phase 2 outcome rather than a tail risk.** Phase 2 §1 records this, and §12's
rule stands: that is a complete deliverable, not a failure to be avoided by
searching harder.

### 10.1.0a Reconciliation: the two `shares_outstanding` statements (2026-09-21)

Two claims in these reports appeared to contradict each other:

1. "Coverage is 98.1%, and the pipeline always read the right value; only the
   report query was wrong." (§10.1.2)
2. "The production universe loader wrote NULL for 59% of symbols."
   (§10.1.10, the DISTINCT ON audit)

**Both are true. They describe different consumers, and only one of them feeds
scoring.** Establishing which is not a formality: if scoring had read the
NULL-laden column, then the float sub-score and the §3.9 `market_cap_est`
fallback were missing for most of the universe, and the float component's
"resolved" status would rest on mostly-absent data.

#### What the scoring path actually reads

**`equity_fundamentals`, directly, with a `value IS NOT NULL AND value > 0`
filter** — not `universe_symbols`. This holds for all four consumers:
`momentum-backtest` (Step 7 and the post-upgrade run), `momentum-scanner` (the
live daily path), `momentum-tracker`, and `momentum-dryrun`. Each calls its own
`loadMetric` and feeds the result into `ScoreInput.FloatSharesEst`.

Because the NOT NULL filter sits in the `WHERE` clause, `DISTINCT ON` chooses
among **non-null rows only**. The tie that broke the universe loader could not
arise: the NULL-by-design `finnhub_metric` row was excluded before the ordering
was applied. The missing `ORDER BY ... source` was a determinism gap in these
queries, not a correctness one, and it has been closed anyway (migration 015).

#### Coverage at scoring time, measured

| Quantity | Value |
|---|---|
| Candidate rows in the post-upgrade extract | 9,571 |
| Rows whose symbol **had** a usable share count | **9,567 (100.0%)** |
| Rows with no share count | **4 (0.0%)** |
| Distinct candidate symbols with a share count | **2,015 / 2,018 (99.9%)** |
| Rows with a real `market_cap` | **9,571 (100.0%)** |
| Rows that fell back to `market_cap_est` | **0** |

`market_cap_est` was used **zero times** among candidates, which follows from
the gate itself: a symbol with no market cap fails §3.2 with
`market_cap_unavailable` and never becomes a candidate.

#### The 59% figure: a real defect in a path with no readers

`LoadFundamentalsFromEquityFundamentals` populates
`universe_symbols.shares_outstanding` / `.market_cap`. Two facts about it:

1. **Nothing in the repository reads those two columns.** A search across both
   Go services and the Python bot returns no consumer.
2. **It never ran during this work.** `universe_symbols.shares_outstanding`,
   `.market_cap` and `.fundamentals_ts` are all **0 populated across all 4,975
   eligible symbols.**

So the 59% was measured by executing the loader's query by hand, not by
observing damaged data. It was a genuine latent defect — it would have written
NULL the moment the loader ran — in a write path whose output nothing consumes.
Worth fixing, and it changes no result.

#### Annotation of the affected Phase 1 results

Every result below is hereby recorded as **computed with 99.9% share-count
coverage among candidates**:

| Result | Status after reconciliation |
|---|---|
| `float` sub-score (§4.2 band scores) | Computed on real data for 9,567 of 9,571 candidate rows. **Not affected.** |
| `float` component's resolved/unresolved status | Not driven by missing data. See the caveat below on what it *is* driven by. |
| §3.9 `market_cap_est` fallback | **Never exercised** among candidates (0 rows). Its behaviour remains untested by this dataset. |
| §3.2 market-cap gate | 100% of candidates had a real market cap. |
| Step 7 (the original 218) and the post-upgrade run | Both read `equity_fundamentals`; neither touched `universe_symbols`. |

**One caveat that does apply to the float component.** `sub_float = 0` is
ambiguous in the extract: `floatPoints` returns 0 both for a missing estimate
and for a genuine float of 300M shares or more. 913 of 9,571 candidates score
0, and on the coverage measured above essentially all of them are **real
large-float names**, not gaps. Anyone reading the extract's `sub_float` column
as a coverage proxy will get 90.5% and be wrong by nine points; coverage has to
be measured against `equity_fundamentals`, as above. The component's status is
therefore limited by the **band design and by the point-in-time problem**
(§3.2), not by absent data.

---

### 10.1.1 What was done

| Step | Result |
|---|---|
| Tiingo Power verified | Token unchanged; **179 requests in 111s, zero 429s** (free tier refused at 51-74). $30/mo, purchased 2026-09-21. See `SUBSCRIPTIONS_PLANS.md`. |
| Subset constraint lifted | `TIINGO_MAX_SELECTED_SYMBOLS` 450 -> 6,000; assertion kept. Pilot cohort frozen in `momentum_pilot_cohort` (migration 013), hash `718ea0e2d6c9d67f63e1c8206ffcc298`. |
| History 3y -> 10y | **4,975 symbols, 8,622,252 bars, 2016-09-21 .. 2026-09-18.** Projected 2.25 GB, 5.6% of the 40 GB/month allowance. |
| Raw close + splitFactor | Migration 012. **100% of Tiingo bars carry both.** No feature reads them. |
| Bar-audit | 548/4,971 symbols flagged (**11.0%**) vs the pilot's ~8%. Decomposed in §2.3: the rise is the longer window, not the wider universe. |
| Fundamentals, full universe | `market_cap` **4,865/4,975 (97.8%)**; `shares_outstanding` **4,881/4,975 (98.1%)** (first reported as 38.4% — query bug, corrected in §10.1.2). |
| Lockbox reserved | 1,783 rows / 1,626 episodes / 1,037 symbols, 2025-03-28 .. 2026-03-27. Reserved **before** any evaluation. Phase 2 §4.3. |
| OOS report | 9,571 evaluable candidates (Phase 1 had 218). Populations A/B/C, row and episode level. |
| Exit replay | **1,545 positions** (Phase 1 had 43). First reported as 119 on a pilot-scoped run; corrected in §10.1.9. |
| Daily refresh | 4,975 req/day = **5.5%** of the configured daily budget. |

`BOT_MOMENTUM_SCAN_ENABLE` remains **false**.

### 10.1.2 Fundamentals coverage (step 3)

> **CORRECTED 2026-09-21.** This section first reported `shares_outstanding`
> coverage as **1,910 / 4,975 (38.4%)**. That figure was wrong, and the error was
> in the reporting query, not the data. See the correction note below.

| Field | Eligible symbols covered | Note |
|---|---|---|
| `market_cap` | 4,865 / 4,975 (**97.8%**) | from Finnhub `/stock/metric` |
| `shares_outstanding` | 4,881 / 4,975 (**98.1%**) | from `/stock/profile2` |
| relying on `market_cap_est` | **34** | no market cap, but shares available |
| no bucket assignable | **76** (1.5%) | neither field usable; never gated or scored |

Bucket split among symbols with a usable market cap: **1,800 penny / 3,065 market**.

#### The 38.4% figure was a query bug of exactly the kind this project keeps finding

Each symbol carries **two** `shares_outstanding` rows at the same timestamp,
because two Finnhub endpoints are written under one metric name:

```
A  2026-09-21 14:36:25  ttm  finnhub_metric     (NULL)
A  2026-09-21 14:36:25  ttm  finnhub_profile2   282430000
```

`/stock/metric` does not report a share count, so its row is NULL by design;
`/stock/profile2` carries the value. Both are legitimate rows — the primary key
is `(symbol, period, metric, source, ts)`.

The reporting query used `DISTINCT ON (symbol) ... ORDER BY symbol, ts DESC`
**without disambiguating `source`**. The two rows tie on `ts`, so Postgres was
free to return either, and it returned the NULL one for most symbols. Every
scoring consumer (`loadMetric`) filters `value IS NOT NULL` before taking the
latest, so **the scoring path was always reading the right value** — only the
coverage report was wrong. §10.1.0a reconciles this against the 59% figure from
the DISTINCT ON audit, which concerns a different consumer with no readers.

It surfaced because the rescoped exit replay printed metric coverage from the
rows it actually used (98.2%) next to a section claiming 38.4%. That is the
denominator guard of §10.1.10 earning its place on its first run: the
contradiction was visible because two independent paths were made to state their
denominators.

**What this changes.** The EDGAR work in Phase 2 §3.2 is **still necessary, but
not for this reason.** Its actual justification is lookahead: today's share count
applied to a 2017 row is information from the future, and small caps that run
issue shares into the run, so the contamination is systematic rather than random.
That argument is untouched. The *coverage* argument — "three fifths of the
universe has no share count" — was never true and is withdrawn. §3.9's float
proxy is likewise in better shape than reported.

### 10.1.3 The out-of-sample result on frozen v2

Populations, reported separately and never pooled (Phase 2 §4.2.1):

| Pop | Rows | Episodes | Symbols | Definition |
|---|---|---|---|---|
| A | 328 | 311 | 135 | pilot symbols, 2023-09 onward — **in-sample, reference only** |
| B | 2,065 | 1,923 | 1,109 | new symbols, 2023-09 onward — OOS |
| C | 5,395 | 5,001 | 1,411 | all symbols, before 2023-09 — OOS |
| **B+C** | **7,460** | **6,924** | **1,702** | **the headline OOS number** |

Lockbox rows (1,783) are excluded from all of them.

#### v2 top-vs-bottom third, episode level (the authoritative cut)

| Pop | top | bottom | delta | p | verdict |
|---|---|---|---|---|---|
| A (in-sample) | 17.48% | 11.65% | +5.83pp | 0.236 | not significant even in-sample |
| B | 12.79% | 9.52% | +3.28pp | 0.062 | **not significant** |
| C | 11.88% | 10.14% | +1.74pp | 0.109 | **not significant** |
| **B+C** | **12.22%** | **10.01%** | **+2.21pp** | **0.017** | meets criterion 1 |

Against the pre-registered criteria as written, criterion 1 is met. **The
verdict is withdrawn** — see §10.1.0 and the decomposition in §10.1.4. The
criterion was pooled, and a pooled test cannot separate ranking from
composition.

### 10.1.4 The finding that matters: v2's separation is bucket composition

v2 separates on B+C pooled (p = 0.017) and separates in **neither bucket**:

| Bucket | n (episodes) | base rate | top | bottom | p |
|---|---|---|---|---|---|
| penny | 362 | 35.08% | 34.17% | 33.33% | 0.891 |
| market | 6,562 | 9.17% | 9.42% | 9.83% | 0.645 |

A score that discriminates in no stratum but discriminates when the strata are
combined is not discriminating. It is sorting on stratum membership. The penny
bucket hits at 35% and the market bucket at 9%, so any score that puts more
penny names in its top third will show separation without ranking anything
within either group.

That is exactly what it does:

| Third | penny share | hit rate |
|---|---|---|
| bottom | 36 / 2,308 = **1.6%** | 10.01% |
| top | 212 / 2,308 = **9.2%** | 12.05% |

Applying the two base rates to those mixes:

```
bottom third   expected from mix alone  9.58%    observed 10.01%
top third      expected from mix alone 11.55%    observed 12.05%

separation predicted by composition alone   1.97pp
separation observed                         2.04pp
                                            ------
share of the effect explained by mix          97%
```

**97% of v2's apparent out-of-sample separation is bucket composition.** The
within-bucket residual is 0.07pp, on 6,924 episodes.

This is not a small caveat on a positive result; it inverts it. v2 is scored
separately per bucket precisely so that penny and market names are ranked
against their own kind, and the pooled statistic quietly undoes that. A
pre-registered criterion was met by a mechanism the criterion was not written to
detect — which is an argument for reporting per-stratum results always, not for
a better criterion.

**Reading:** v2 as a ranker is **NOT CONFIRMED out-of-sample** (ruling,
§10.1.0). The pooled test passed and the stratified tests — the ones that
correspond to how the score is actually used, since v2 is scored and thresholded
per bucket — did not.

### 10.1.5 rvol replicates; the inversion replicates

**`rvol_20` (criterion 2): CONFIRMED.** Same direction as Phase 1, at far
greater strength, and in both OOS populations independently:

| Pop | rvol-alone top | bottom | delta | p |
|---|---|---|---|---|
| A | 20.39% | 6.80% | +13.59pp | 0.004 |
| B | 15.44% | 7.96% | +7.49pp | <0.001 |
| C | 13.62% | 8.04% | +5.58pp | <0.001 |
| **B+C** | **13.91%** | **8.23%** | **+5.68pp** | **<0.001** |

Phase 1 saw p = 0.012 on 218 candidates and called it the one component with
evidence behind it. On 6,924 out-of-sample episodes it holds, in the same
direction, in every population separately. This is the clearest positive result
of the phase — and note that **rvol-alone separates more strongly than the full
v2 score does** (+5.68pp against +2.21pp).

**`high52w` inversion (criterion 3): CONFIRMED.** Episodes above the median
`pct_of_52w_high` hit at 6.70% against 14.36% below it, **-7.65pp, p < 0.001**.
Phase 1's finding that proximity to the 52-week high is *negatively* related to
a subsequent double is not noise. Zeroing it in v2 was right, and the sign is
now established well enough to use as a feature in Phase 2 with the model
learning the direction.

**`breakout`: the inversion is state-specific, and the pooled sign was hiding
that.**

| `breakout_state` | n | hit% |
|---|---|---|
| `breakout_from_consolidation` | 4,109 | **6.42%** |
| `none` | 1,618 | 15.57% |
| `breakout` | 876 | **20.89%** |
| `approaching` | 321 | 9.35% |

`breakout_from_consolidation` vs `none` is -9.15pp (p < 0.001), so the negative
direction replicates for that state. But a plain `breakout` is the **best**
state in the table at 20.89%, well above the 10.53% base rate. Phase 1 measured
"breakout geometry" as one inverted component; it is two effects with opposite
signs, and the dominant state (4,109 of 6,924 episodes) carries the negative
one.

That is a **new** finding and it is a design observation, not a tuning one: the
component collapses four states onto one axis, and no single weight can be right
for both `breakout` at 20.89% and `breakout_from_consolidation` at 6.42%.

### 10.1.5a Does `rvol_20` survive stratification? Yes — at a third of the size

Because v2's pooled pass turned out to be composition, `rvol_20` had to face the
same question. "Independently significant in B and C" separates by **time
period**, not by bucket, so it did not answer it.

Every rvol split below is computed **within** its stratum. Splitting globally
and then counting per stratum would reintroduce the exact confound being tested.
Population B+C, episode level, lockbox and population A excluded, n = 6,924.
Harness: `scripts/rvol_stratification_check.py`.

**Within each bucket**

| Bucket | n | base rate | rvol top tercile | bottom | delta | RR | p |
|---|---|---|---|---|---|---|---|
| market | 6,562 | 9.17% | 10.43% | 8.14% | **+2.29pp** | 1.28 | **0.009** |
| penny | 362 | 35.08% | 34.17% | 31.40% | +2.76pp | 1.09 | 0.648 |

**Within `atr_pct` terciles** (volatility held roughly constant)

| Tercile | n | base rate | rvol top | bottom | delta | RR | p |
|---|---|---|---|---|---|---|---|
| atr low | 2,309 | 1.34% | 1.30% | 1.04% | +0.26pp | 1.25 | 0.633 |
| atr mid | 2,308 | 7.76% | 8.71% | 6.23% | +2.48pp | 1.40 | 0.064 |
| atr high | 2,307 | 22.50% | 28.26% | 18.57% | **+9.68pp** | 1.52 | **<0.001** |

**Cochran-Mantel-Haenszel** — pools evidence across strata without ever
comparing across them, which is the property the pooled v2 test lacked:

| Stratification | strata | CMH chi-sq (1 df) | p | MH common OR |
|---|---|---|---|---|
| by bucket | 2 | 5.72 | **0.017** | **1.215** |
| by bucket x atr tercile | 5 | 4.35 | **0.037** | **1.192** |

**Verdict: `rvol_20` SURVIVES both stratified tests.** It is a genuine signal,
not a composition artifact. That is the answer to the question asked.

**But most of its pooled effect was composition too.** On the same median split
throughout, so the comparison is like-for-like:

```
crude, unstratified                       OR 1.529   excess odds 0.529
stratified by bucket                      OR 1.215   excess odds 0.215   (59% was bucket mix)
stratified by bucket x atr tercile        OR 1.192   excess odds 0.192   (64% was bucket + volatility)
```

**The surviving effect is 2.8x smaller than the pooled number advertised.** The
headline "+5.68pp, p < 0.001" becomes +2.29pp within the market bucket and an
odds ratio of 1.19 once volatility is also held constant.

Two qualifications that matter more than the p-values:

1. **The signal lives almost entirely in high-volatility names.** +9.68pp in the
   top atr tercile, +2.48pp in the middle (p = 0.064), +0.26pp in the bottom
   (p = 0.633). So the honest statement is *"among already-volatile names,
   higher relative volume helps"* — not *"rvol predicts doubles."* It is
   entangled with the volatility confound rather than independent of it.
2. **The penny bucket is unresolved, not negative.** n = 362 with a 35% base
   rate has little power; +2.76pp at p = 0.648 is an absence of evidence. Per
   criterion 4 it stays **unresolved**. Note also that penny and volatility are
   nearly collinear here — 329 of 362 penny episodes sit in the top atr tercile,
   and the penny/atr-low cell holds 2 episodes — so "bucket" and "volatility"
   are not cleanly separable strata in this data.

**Practical reading:** rvol is defensible as a signal in the market bucket and
unproven in the penny bucket, at roughly a third of the strength the pooled
report suggested. That is enough to build on in Phase 2 as a baseline and a
feature. It is not enough to justify switching the live score to rvol-only,
which would stake alerting on a 1.19 odds ratio concentrated in the names that
also crash hardest (§4.1's drawdown finding).

### 10.1.5b `rvol_20` does NOT survive on the gate-v2 candidate set (2026-09-21)

§10.1.5a concluded that `rvol_20` survives bucket and volatility stratification.
**That conclusion is withdrawn.** It was measured on a candidate set selected
with today's market cap, and on the corrected set it does not hold.

Descriptive re-measure, episode level, lockbox region excluded. Identical code,
identical statistics; the only change is which symbol-days passed §3.2.

| Test | gate v1 (as reported in §10.1.5a) | **gate v2 (corrected sample)** |
|---|---|---|
| within market bucket | +2.29pp, RR 1.28, **p = 0.009** | **+0.39pp, RR 1.06, p = 0.622** |
| within penny bucket | +2.76pp, p = 0.648 | **−0.32pp, p = 0.951** |
| CMH by bucket | χ² 5.72, **p = 0.017**, MH OR **1.215** | **χ² 0.02, p = 0.877, MH OR 0.983** |
| CMH by bucket × atr tercile | χ² 4.35, **p = 0.037**, MH OR **1.192** | **χ² 0.02, p = 0.897, MH OR 0.985** |

**The Mantel-Haenszel common odds ratio falls from 1.215 to 0.983.** Not reduced
— gone. 0.983 is indistinguishable from no effect, and slightly the wrong side
of 1.

#### The change is the gate, not the lockbox

Both changed in the same batch, so attribution was checked rather than assumed.
Re-running **gate v1 under the new region-based lockbox** reproduces §10.1.5a
exactly: +2.29pp, p = 0.009, CMH p = 0.017, MH OR 1.215 — identical to three
decimal places. The region and the old row list coincide for gate-v1
candidates, which is expected, since the row list *was* the gate-v1 candidates
in that window.

**So the entire change is attributable to gate v2.**

#### What survives, and what it turns out to be

The pooled result is still strong (+7.05pp, p < 0.001), and so is the
unstratified `atr_pct`-tercile split (+14.61pp in the top tercile, p < 0.001).
Crossing the two explains both:

| stratum | high-rvol hit% | low-rvol hit% |
|---|---|---|
| market / atr high | 14.42% | **15.22%** |
| penny / atr high | 41.70% | **46.56%** |
| market / atr mid | 7.20% | 5.92% |
| market / atr low | 1.25% | 1.07% |

**In the high-volatility cells — the ones carrying the pooled effect — rvol
points the WRONG WAY in both buckets.** The apparent "+14.61pp in the top atr
tercile" from §10.1.5a was itself a bucket-composition artifact: the top
`atr_pct` tercile mixes penny names (42% base rate) with market names (7%), and
sorting on rvol within that mixture sorts partly on bucket.

That is the same failure mode as v2's score, one level down, and it survived the
first stratified check because the *candidate set* was contaminated even though
the *strata* were clean.

#### Consequences

1. **`rvol_20` is not a validated component and is no longer even a replicated
   one** on the corrected sample. Phase 1 §10.1.0's ruling 3 said it was
   "genuine but small and below the effect-size floor"; the honest statement now
   is **unresolved at best, absent at worst**.
2. **The Phase 2 entry gate (§2 of that spec) must be re-read.** Its table says
   the full plan proceeds if `rvol` replicates out-of-sample, and the
   feature-research-only path applies if **nothing replicates**. On the
   gate-v2 sample, nothing does. That is a decision for the user, not the agent,
   and it is flagged rather than acted on.
3. **This is descriptive, not a new test.** No weight, threshold, gate band or
   exit rule changed. It re-runs the same pre-registered statistics on a
   candidate set that is no longer chosen with information from the future.
4. **Nothing here is evidence that the strategy is worse than believed.** It is
   evidence that the earlier measurement was made on the wrong sample. The
   corrected sample is smaller in overlap than either version's total (§3.2.2:
   42.3% churn), so this is the first measurement of this question on data
   selected without lookahead.

Base rates also move materially, which is worth noting before anything is
compared across versions: market 9.17% → **6.94%**, penny 35.08% → **42.20%**.
Results computed under different gate versions are not comparable and must never
be pooled.

### 10.1.5c Decomposing the gate change: the `rvol` signal was in the LEAK group

§10.1.5b showed `rvol` losing its separation under gate v2. Gate v2 bundled two
changes, so that result could not be read yet:

- **the leak correction** — rows whose point-in-time cap falls outside the
  bucket's band are rejected. The actual fix.
- **a coverage exclusion** — rows with no EDGAR share count are rejected too.
  Not random: 48.7% of uncovered symbols listed after 2023-09, against 16.2% of
  covered ones, and post-IPO months are exactly where momentum runs plausibly
  live.

If the signal sat in the leak group it was an artifact. If it sat in the
coverage groups, v2 was a coverage question, not a verdict. Decomposed
descriptively (`scripts/decompose_gate_change.py`), episode level, lockbox
region and population A excluded:

| Group | episodes | hit% | penny hit% | market hit% |
|---|---|---|---|---|
| in both v1 and v2 | 4,877 | 8.73% | 36.22% | 6.79% |
| **v1-only: PIT cap outside band [LEAK]** | **982** | **20.06%** | 22.22% | 20.04% |
| v1-only: no EDGAR series [COVERAGE] | 832 | 8.65% | 30.43% | 8.03% |
| v1-only: bar predates 1st filing [COVERAGE] | 233 | 14.59% | 14.29% | 14.60% |
| v2-only: eligible, never studied | 1,844 | 12.53% | 50.90% | 7.27% |

**The leak group hits at 20.06% against 8.73% in the retained set — 2.3x the
base rate.** That is the signature of lookahead doing exactly what it does:
today's large market cap identifies companies that *grew*, and companies that
grew are companies that ran. Gate v1 was preferentially admitting winners.

`rvol_20` within-bucket separation, computed **within each group**:

| Group | bucket | n | delta | p |
|---|---|---|---|---|
| in both v1 and v2 | market | 4,554 | +0.53pp | 0.560 |
| **v1-only [LEAK]** | market | 973 | **+4.63pp** | 0.140 |
| v1-only: no EDGAR series [COVERAGE] | market | 809 | **−2.60pp** | 0.266 |
| v1-only: pre-first-filing [COVERAGE] | market | 226 | +2.67pp | 0.631 |
| v2-only | market | 1,622 | −0.56pp | 0.732 |

**`rvol`'s positive separation is concentrated in the leak group (+4.63pp),
absent in the retained rows (+0.53pp), and NEGATIVE in the largest coverage
group (−2.60pp).** None reaches p < 0.05 individually — the groups are small
once split — but the pattern is unambiguous, and it is the opposite of the
coverage explanation.

#### The coverage confound turned out to be near-empty, for a reason nobody planned

Listing age by group was **0.0–0.2% in every group**. The hypothesis that the
coverage exclusion removes recent listings is true of *symbols* and false of
*candidates*:

- symbols listed after 2023-09: **1,100 of 4,971**
- candidate rows they supply: **173 of 9,571 (1.81%)**
- of those, inside the lockbox window: **164 (95%)** — leaving **~9 rows**

§3.1's 252-bar history minimum plus §6's 120-session label horizon mean a
symbol listed after 2023-09 cannot produce a complete-label candidate until
roughly 2025-03, which is where the lockbox begins. **Recent listings were
already absent from the evaluable set, for reasons that have nothing to do with
EDGAR.**

So the coverage exclusion could not have removed `rvol`'s signal, because the
names it removes were never in the measurement. Worth recording that this is
luck rather than design, and it expires: as the history rolls forward,
post-IPO months become a growing share of the candidate set (Phase 2 §3.2.3a).

### 10.1.6 Other components — resolved, with a caveat that applies to most of them

Median-split z-tests on B+C episodes, Benjamini-Hochberg applied across the
family of seven. All survive BH:

| Component | above median | below median | delta | p | direction |
|---|---|---|---|---|---|
| `atr_pct` | 18.46% | 2.60% | **+15.86pp** | <0.001 | positive |
| `high52w` | 6.70% | 14.36% | -7.65pp | <0.001 | **negative** |
| `log(dollar_volume)` | 7.48% | 13.58% | **-6.09pp** | <0.001 | **negative** |
| `rvol_20` | 12.51% | 8.55% | +3.96pp | <0.001 | positive |
| `vol_accel` | 12.39% | 8.67% | +3.73pp | <0.001 | positive |
| `rsi_14` | 9.13% | 11.93% | -2.80pp | <0.001 | **negative** |
| `vwap_dist_pct` | 11.32% | 9.73% | +1.59pp | 0.031 | positive |

Every component is now "resolved" under criterion 4, which after Phase 1's
sea of unresolved components looks like a breakthrough. It mostly is not, and
the reason should be recorded before anyone builds on these numbers.

**The three strongest results are close to tautologies.** The label is "did the
close double within 120 sessions". `atr_pct` is realised volatility, and a
volatile stock is mechanically more likely to reach any distant price level.
`log(dollar_volume)` being negative says smaller, less liquid names double more
often, which is the same statement. `high52w` being negative says a stock that
has already fallen a long way has more room to double, again the same statement.
**A +15.86pp result that says "volatile things move more" is not an edge**; it
predicts the magnitude of the move without saying anything about its direction,
and §4.1's drawdown finding already showed these are the names that get there
through the worst drawdowns.

`rvol_20` and `vol_accel` are the two that are *not* restatements of volatility
— they are relative-volume measures, and they are the ones Phase 1 identified.
They are also, tellingly, much smaller effects (+3.96pp, +3.73pp).

The honest ordering of confidence coming out of this phase:

1. `rvol_20` is real, replicated, and modest.
2. The `high52w` and `dollar_volume` effects are real and probably mechanical.
3. `breakout` is two opposite effects wearing one name.
4. v2 as a composite is not demonstrated (§10.1.4).

### 10.1.7 Hit rate by calendar year — and a regime that owns the dataset

B+C, episode level:

| Year | n | hits | hit% | median gain% | median drawdown% |
|---|---|---|---|---|---|
| 2017 | 183 | 19 | 10.38% | 24.68% | 25.21% |
| 2018 | 660 | 39 | 5.91% | 17.36% | 29.94% |
| 2019 | 699 | 38 | 5.44% | 14.92% | 37.79% |
| **2020** | **968** | **269** | **27.79%** | **57.13%** | 34.96% |
| 2021 | 945 | 63 | 6.67% | 18.75% | 37.65% |
| 2022 | 775 | 52 | 6.71% | 20.00% | 37.86% |
| 2023 | 1,019 | 77 | 7.56% | 21.86% | 30.97% |
| 2024 | 1,378 | 147 | 10.67% | 20.24% | 35.10% |
| 2025 | 297 | 25 | 8.42% | 15.78% | 38.64% |

Two things to take from this.

**2020 supplies 269 of 729 total hits — 37% of every positive outcome in the
dataset, from 14% of the episodes.** The base rate that year is 27.79% against
5-11% in every other year. Any statistic pooled across ten years is substantially
a statement about the post-COVID-crash recovery. This is the strongest argument
in the report for Phase 2's purged walk-forward folds: a model trained on data
containing 2020 and tested on data containing 2020 will look excellent and will
be describing one quarter of 2020.

**The survivorship signature is not visible here, which does not mean it is
absent.** §2.3 predicts older years should look inflated because the universe is
filtered by survival to today. The years do not trend that way — 2018 and 2019
are the *worst* in the table. The likely reason is that 2020 and the differing
episode counts swamp a monotonic drift, not that the bias is gone. It stays
recorded as **unmeasured**, and Phase 2 §3.3's delisted-symbol work is what
would measure it. Reporting by year was the commitment; the commitment is met
and the answer is inconclusive.

### 10.1.8 Per-bucket p90 thresholds (step 6) — REPORT ONLY

Recomputed on B+C. **The defaults are unchanged; this is a measurement.**

| Bucket | n (episodes) | current | p90 here | p95 | median |
|---|---|---|---|---|---|
| penny | 362 | **72** | 70.0 | 72.0 | 62.0 |
| market | 6,562 | **65** | 67.0 | 69.0 | 55.0 |

Row-level figures are identical to episode-level ones to one decimal.

Both current thresholds sit within 2 points of the recomputed p90, on a dataset
roughly 35x the one that set them. §4.4's thresholds were chosen to alert on
about the top decile per bucket, and on ten years of out-of-sample data they
still do. That is a genuine, if narrow, piece of good news: the *alert volume*
calibration held even though the *ranking* underneath it (§10.1.4) did not.

The NEXR observation logged in §4.4 (68/75 in the penny bucket, below the 72
threshold) is now joined by a population-level reading: a penny threshold of 70
would have admitted it. **No change is made here.**

### 10.1.9 §5 exit replay on the widened data (step 7) — no rule changes

> **CORRECTED 2026-09-21.** The first run of this section reported **119
> positions** and described them as "the widened data". `momentum-tracker`
> carried the same hardcoded `backfill_selected` join as `momentum-backtest`
> (§10.1.10), so it replayed the **450-symbol pilot** over ten years of history.
> The widening it saw was in time, not in symbols. Re-run under
> `-scope eligible` with the denominator guard active: **1,545 positions.** All
> figures below are the corrected full-universe run.

Denominators printed by the run itself: scope `eligible`, 4,971 symbols in scope,
4,971 loaded, 4,971 scored, 8,617,103 bars read.

**1,545 positions closed** (Phase 1: 43), 5 still open and excluded.

**Exit reasons**

| Reason | n | median exit% | median peak% | median sessions | gave back |
|---|---|---|---|---|---|
| `momentum_stalled` | 757 | +2.25% | 5.02% | 5 | 2.77 pts |
| `breakout_failed` | 651 | -5.03% | **0.00%** | **1** | 5.03 pts |
| `lost_vwap` | 106 | -3.23% | 5.24% | 3 | 8.47 pts |
| `stop_atr` | 31 | -8.06% | 0.00% | 2 | 8.06 pts |
| `timeout` | **0** | — | — | — | **never fired** |

Overall: median exit -1.20%, median peak +1.50%, median gave back 2.70 pts.

**Fired-first vs also-true**

| Condition | fired 1st | also true | note |
|---|---|---|---|
| `momentum_stalled` | 757 | 42 | |
| `breakout_failed` | 651 | 0 | |
| `lost_vwap` | 106 | **276** | outranked more often than it fires |
| `stop_atr` | **31** | **85** | outranked more often than it fires |
| `timeout` | 0 | 0 | never true |

**Session survival:** longest position **17 sessions**; **0 of 1,545 reached 20+**.

All three Phase 1 findings replicate, now on **36x** Phase 1's 43 positions:

1. **`breakout_failed` fires on session 1 at a median peak of 0.00%**, on 651 of
   1,545 exits. It is not exiting a trade that went wrong; it closes positions
   that never moved, immediately, at a median -5.03%. The wider data makes this
   worse, not better: the median exit fell from -4.18% to -5.03%.
2. **`timeout` is structurally unreachable, now beyond reasonable doubt.** The
   longest position across 1,545 positions and ten years ran **17 sessions**
   against a 20-session threshold. Phase 1 saw 9 of 43; the pilot-scoped run saw
   16 of 119. Three independent samples, the maximum creeping toward the
   threshold without reaching it, and zero fires. The faster rules always
   pre-empt it.
3. **`stop_atr` is suppressed by ordering**, now on decisive evidence: true
   **116** times, fired first **31**. Phase 1 saw 3 and 0.

The Phase 1 distinction holds exactly and is now well powered: **reordering
would surface `stop_atr` but would never surface `timeout`.** The first is an
ordering artefact — the condition is true 116 times and something faster wins 85
of them. The second is structural: no amount of reordering reaches a horizon the
data never attains. No rule was changed; these are Phase 2 §8's inputs.

### 10.1.10 Three harness bugs found, all the same bug — and the guard for it

The report harnesses hardcoded the pilot subset in **three** places across two
commands:

```
momentum-backtest  loadBars()    JOIN universe_symbols u ON ... AND u.backfill_selected
momentum-backtest  loadMetric()  JOIN universe_symbols u ON ... AND u.backfill_selected
momentum-tracker   loadBars()    JOIN universe_symbols u ON ... AND u.backfill_selected
momentum-tracker   loadMetric()  JOIN universe_symbols u ON ... AND u.backfill_selected
momentum-dryrun    loadMetric()  JOIN universe_symbols u ON ... AND u.backfill_selected
```

The tracker one was found only after the first version of §10.1.9 had already
been written and reported, which is the point: **the first two were found by
looking, and looking is not a control.**

Fixing only the first produced a run reporting **"symbols: 4,971"** that scored
450 of them: with bars widened but fundamentals still pilot-scoped, every
non-pilot symbol-day failed §3.2 with `market_cap_unavailable` and was counted as
an ordinary gate rejection. 9,571 candidates were reported as 739. **Nothing
errored.** A gate rejection is a normal outcome, so a report describing a
twentieth of the data looked exactly like a complete one.

This is the same lesson as the six integration bugs that closed Phase 1: the
scanner's daily path had never been run end-to-end, and here the backtest's wide
path had never been run wide. All of it was correct code whose *scope* was frozen
at the moment it was written.

#### The denominator guard (`internal/reportscope`)

Remembering to pass a scope is not a fix, so the scope is now checked rather
than trusted. Two mechanisms, both needed:

1. **One `Scope` value drives every query in a run.** A harness cannot widen its
   bars while leaving its fundamentals narrow, because both clauses come from
   the same value via `Scope.JoinOn(alias)`. This makes the original divergence
   unexpressible.
2. **The denominator is verified before any scoring.** The harness asks the
   database how many symbols the declared scope actually contains *with bars*,
   compares that against the set it loaded, and **exits non-zero** on any
   disagreement. Mechanism 1 cannot catch someone editing the SQL string
   directly; mechanism 2 can.

Equality is exact, deliberately. 450 against 4,971 is not a rounding
difference, and a guard that accepts "close enough" invites a later one that is.
A legitimate reason to differ belongs in `ExpectedBarSymbols` as a reviewable
rule, not in a fuzzy comparison.

Every report now prints denominators computed **from the rows it actually
handled**, so a reader can check the arithmetic without trusting the narration:

```
scope:                   eligible
symbols in scope:        4971 (verified against the database)
symbols loaded:          4971
symbols scored:          4971
bars read:               8617103
market_cap:            4865/4971 symbols (97.9%)
shares_outstanding:    4881/4971 symbols (98.2%)
```

Metric coverage is printed rather than asserted, because coverage genuinely
varies by metric. That printout earned itself on its first run: it contradicted
§10.1.2's claimed 38.4% share-count coverage and exposed a bug in that
section's query (see the correction there).

**Mutation-tested against the live database.** Re-introducing the original
hardcoded join verbatim and running `-scope eligible`:

```
SCOPE MISMATCH: declared scope "eligible" contains 4971 symbols with bars, but the harness loaded 450.
This is the silent-narrowing failure the denominator guard exists to catch:
a report that covers part of the data looks identical to one that covers all of it,
because the symbols it never saw simply never appear as candidates.
  4521 in scope but NOT loaded, e.g. A, AAC, AACI, AACO, AACP, AADX, AAL, AAME, ...
  -> usually a query still restricted to the pilot subset (backfill_selected).
exit=1
```

The guard also fails on the inverse error (a pilot-scoped report that loads the
whole universe) and on equal-cardinality-but-different-membership, since
counting alone would pass a scope that swapped one symbol for another. Unit
tests in `reportscope_test.go`.

There is a second-order point worth keeping. `backfill_selected` was serving as
both the backfill's working set **and** the identity of the in-sample cohort,
and the subset-selection job begins by clearing it for every row. Widening the
universe would have erased the only record of which 450 symbols score v2 was
fitted on — silently, and in a way that makes the OOS report look *better*,
since in-sample rows would have merged into population B. That is why the cohort
was copied into `momentum_pilot_cohort` with a content hash before anything else
ran (migration 013).

### 10.1.11 Daily refresh fits the budget (step 8)

Measured, not estimated: a single-session request averages **226 bytes** across
10 symbols.

| Dimension | Daily refresh, full universe | Budget | Used |
|---|---|---|---|
| requests | 4,975/day | 90,000 configured (100,000 provider) | **5.5%** |
| peak rate | 4,975/hour at 2 req/s | 10,000/hour | 49.8% |
| wall clock | ~41 min | after US close | — |
| bandwidth | 1.1 MB/day, 0.02 GB/month | 40 GB/month | **0.05%** |

The daily budget absorbs **18x** the current universe before it binds.
`BOT_MOMENTUM_SCAN_ENABLE` stays **false**.

### 10.1.12 Scorecard against the pre-registered criteria

Criteria were written into Phase 2 §4.2.1 **before** the report was run.

| # | Criterion | Result |
|---|---|---|
| 1 | v2 confirmed OOS: B+C episode top-third > bottom-third, p < 0.05 | **NOT CONFIRMED.** Met on the letter (p = 0.017), **verdict withdrawn** — 97% of the separation is bucket composition and v2 separates in neither bucket. **The criterion itself was flawed**, being pooled across buckets (§10.1.0, §10.1.4) |
| 2 | `rvol` replicates, same direction, p < 0.05, episode level | **CONFIRMED**, and it survives bucket and volatility stratification: CMH p = 0.017 by bucket, p = 0.037 by bucket x atr tercile. But 64% of the pooled effect was composition, the surviving effect is 2.8x smaller, and it is concentrated in high-volatility names (§10.1.5a) |
| 3 | `breakout`/`high52w` inversion replicates, negative, p < 0.05 | **CONFIRMED for `high52w`** (-7.65pp, p < 0.001) and for `breakout_from_consolidation` (-9.15pp). **Not for plain `breakout`**, which is the best state at 20.89% |
| 4 | Other components resolved only at p < 0.05 | All seven cleared p < 0.05, which at n = 6,924 is close to uninformative. Three of the strongest are volatility restatements (§10.1.6). **Phase 2 §4.2 now requires a pre-registered minimum effect size, not just a p-value** |

**Phase 2 entry gate (§2 of the Phase 2 spec):** `rvol` replicates
out-of-sample **and survives stratification**, so Phase 2 proceeds as **the full
plan**, not the feature-research-only path. The `breakout`/`high52w` row of that
table also applies: they stay zeroed in v2 and enter the model as ordinary
features whose sign the model learns.

#### What the pre-registration got wrong, for the next one

Criterion 1 was pooled. Criterion 4 was a bare p-value. Both were satisfiable by
artifacts:

- **Pooling let composition pose as ranking.** Phase 2 §4.2 now forbids
  pooled-across-bucket tests as acceptance evidence and requires per-bucket
  and/or CMH.
- **A bare p < 0.05 at n ~ 7,000 resolves almost anything.** All seven
  components "resolved", including three that restate "volatile things move
  more". Phase 2 §4.2 now requires a minimum effect size alongside the p-value.

Neither flaw was in the analysis. The pre-registration is the safeguard, and
these are the two ways this one could be passed without learning anything.

### 10.1.13 What this phase did NOT establish

Recorded because a list of confirmations reads as more than it is.

- **v2 is not a demonstrated ranker.** §10.1.4, and now a formal ruling
  (§10.1.0). The pooled pass is composition.
- **`rvol` in the penny bucket is unresolved**, not confirmed: n = 362,
  +2.76pp, p = 0.648. And its surviving effect overall is concentrated in
  high-volatility names, so it is entangled with the volatility confound rather
  than independent of it (§10.1.5a).
- **Survivorship bias is still unmeasured**, and the ten-year window makes it
  larger. Hit rates by year are reported and are inconclusive (§10.1.7).
- **Point-in-time fundamentals are still absent.** Today's market cap and share
  count were applied to every historical row, including 2016 ones. Phase 2 §3.2
  calls this lookahead rather than a limitation, and that argument stands on its
  own — **not** on share-count coverage, which is 98.1% and was misreported as
  38.4% (§10.1.2).
- **The bar-audit seam question is open.** ~840 flagged seams have no reported
  corporate action, but `divCash` was not captured, so provider defect and
  detector over-flagging cannot be separated (§2.3).
- **2020 dominates the outcomes** (37% of hits) and no result here is
  regime-controlled. That is Phase 2 §4.1's job.
- **The lockbox is untouched**, as it must be.
- **Nothing was tuned.** No weight, threshold, gate or exit rule changed.

---

## 11. Phase 2 and Phase 3 (context only — do not build)

**Phase 2 — the model.** Using `momentum_labels`, train per-threshold binary classifiers (or one ordinal model) on the §3 feature vector, with proper time-series cross-validation (train on earlier periods, test on later — never random splits on time-series data) and probability calibration. *Then* the multi-division display becomes meaningful, because "+200%: 85" can mean a calibrated 85% probability rather than an arbitrary number. Feature importance from this step is also what tells you which catalyst keywords and which of the currently-unscored features (sector strength, gap %, ATR%) deserve weight.

**Phase 3 — real-time.** Paid data tier, intraday bars and/or WebSocket, time-of-day-normalized RVOL (§3.4), intraday session VWAP (§3.8), halt detection, pre-market coverage, and the funnel architecture (universe → cheap filters → technical → news → alert). Verify per-exchange real-time coverage before designing for any non-US market: US equities/forex/crypto real-time is well covered by EODHD's WebSocket, but European real-time coverage at the same tier is unconfirmed and may require a different provider.

**Web UI / MFE.** Not in Phase 1 — Discord is the only Phase 1 interface. When it comes: shell (nav/auth/theme/event bus), one configurable `scanner-table` MFE handling both buckets with buy/sell tabs (rather than four near-identical MFEs), `stock-detail` (chart + score breakdown + catalyst + news), `alerts-feed`, `backtest-lab` (the Phase 2 surface — decile hit rates, feature importance), `watchlist`, `settings` (tune thresholds without redeploy).

---

## 12. Notes for the implementing agent

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

- **Do not invent data sources.** If a required field has no free source, leave it null and record why. Several fields in this spec are deliberately null in Phase 1 (§3.13).
- **Do not silently substitute defaults for missing inputs.** A null RVOL must stay null and fail the gate, never become 0 or 1.
- **Do not add the multi-division score.** One score, `momentum_score_100`. §4 is the complete scoring definition.
- **Do not reimplement indicators.** `services/data-analyzer/internal/compute/` has ATR, RSI, Donchian, VWAP, swings, support/resistance already.
- **Do not create a second Discord bot.** Extend `analyst-bot`.
- Every worker must be idempotent (upserts) and safe to re-run.
- **Do not share job-state columns between two jobs.** The test: *would these two failures be distinguishable at 3 a.m.?* Identical shape is what makes the confusion possible. A second small state table costs one migration; a fundamentals failure surfacing in a column named `backfill_last_error` costs an incident. Stated in full in `shared/schemas/SCHEMAS.md`.
- Ask before deviating from any definition in §3 or §4 — these are the product, and a plausible-looking variation silently changes what the whole system measures.
