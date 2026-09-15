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
> Details and the consolidated-volume caveat in
> `services/data-ingestion/data_ingestion.md`.

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

- Initial backfill: 3 years of daily bars per symbol. At ~5 requests/second this is a multi-hour, run-once job. Make it resumable — checkpoint progress per symbol so a crash doesn't restart from zero.
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

Keyword lists live in configuration (a YAML/JSON file or env-loaded config), so they can be tuned without a rebuild.

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

### 4.1 A correction to the original weight table

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

**Relative volume (0–20)** — input `rvol_20`
| Band | Points |
|---|---|
| < 1.5 | 0 |
| 1.5 – 3.0 | 0 → 12 |
| 3.0 – 5.0 | 12 → 18 |
| 5.0 – 10.0 | 18 → 20 |
| > 10.0 | 20 |

**Breakout (0–20)** — input `breakout_state`
| Value | Points |
|---|---|
| `breakout_from_consolidation` | 20 |
| `breakout` | 14 |
| `approaching` | 8 |
| `none` | 0 |

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

**Near 52-week high (0–5)** — input `pct_of_52w_high`
| Band | Points |
|---|---|
| ≥ 1.00 (new high) | 5 |
| 0.95 – 1.00 | 4 |
| 0.90 – 0.95 | 2 |
| < 0.90 | 0 |

Since §3.7's window excludes today, `pct_of_52w_high` can exceed 1.0 and the top band is
reachable: `>= 1.00` *is* the new-52-week-high case. Values above 1.0 clamp to 5.

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

Alert threshold: post to Discord when `momentum_score_100 >= 60` (env-configurable, per bucket).

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

### 8.1 `data-universe` (Go)

`services/data-ingestion/cmd/data-universe/main.go`

Responsibilities:
1. **Weekly:** refresh `universe_symbols` from Finnhub US symbol list; apply §3.1 eligibility; record `excluded_reason` for audit.
2. **Weekly:** refresh `shares_outstanding`, `market_cap`, `sector`, `industry` from existing fundamentals ingestion.
3. **Once, resumable:** backfill 3 years of daily bars for every eligible symbol into `equity_ohlcv` via the existing Yahoo fetcher. Checkpoint per symbol.
4. **Daily after close:** incremental bar refresh for all eligible symbols.

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

## 11. Phase 2 and Phase 3 (context only — do not build)

**Phase 2 — the model.** Using `momentum_labels`, train per-threshold binary classifiers (or one ordinal model) on the §3 feature vector, with proper time-series cross-validation (train on earlier periods, test on later — never random splits on time-series data) and probability calibration. *Then* the multi-division display becomes meaningful, because "+200%: 85" can mean a calibrated 85% probability rather than an arbitrary number. Feature importance from this step is also what tells you which catalyst keywords and which of the currently-unscored features (sector strength, gap %, ATR%) deserve weight.

**Phase 3 — real-time.** Paid data tier, intraday bars and/or WebSocket, time-of-day-normalized RVOL (§3.4), intraday session VWAP (§3.8), halt detection, pre-market coverage, and the funnel architecture (universe → cheap filters → technical → news → alert). Verify per-exchange real-time coverage before designing for any non-US market: US equities/forex/crypto real-time is well covered by EODHD's WebSocket, but European real-time coverage at the same tier is unconfirmed and may require a different provider.

**Web UI / MFE.** Not in Phase 1 — Discord is the only Phase 1 interface. When it comes: shell (nav/auth/theme/event bus), one configurable `scanner-table` MFE handling both buckets with buy/sell tabs (rather than four near-identical MFEs), `stock-detail` (chart + score breakdown + catalyst + news), `alerts-feed`, `backtest-lab` (the Phase 2 surface — decile hit rates, feature importance), `watchlist`, `settings` (tune thresholds without redeploy).

---

## 12. Notes for the implementing agent

- **Do not invent data sources.** If a required field has no free source, leave it null and record why. Several fields in this spec are deliberately null in Phase 1 (§3.13).
- **Do not silently substitute defaults for missing inputs.** A null RVOL must stay null and fail the gate, never become 0 or 1.
- **Do not add the multi-division score.** One score, `momentum_score_100`. §4 is the complete scoring definition.
- **Do not reimplement indicators.** `services/data-analyzer/internal/compute/` has ATR, RSI, Donchian, VWAP, swings, support/resistance already.
- **Do not create a second Discord bot.** Extend `analyst-bot`.
- Every worker must be idempotent (upserts) and safe to re-run.
- **Do not share job-state columns between two jobs.** The test: *would these two failures be distinguishable at 3 a.m.?* Identical shape is what makes the confusion possible. A second small state table costs one migration; a fundamentals failure surfacing in a column named `backfill_last_error` costs an incident. Stated in full in `shared/schemas/SCHEMAS.md`.
- Ask before deviating from any definition in §3 or §4 — these are the product, and a plausible-looking variation silently changes what the whole system measures.
