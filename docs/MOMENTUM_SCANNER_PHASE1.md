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

**Critical constraint: volume must be consolidated volume.** Every volume feature in this spec (RVOL, acceleration, dollar volume) is meaningless on single-venue volume. Yahoo provides consolidated volume; IEX-only does not. If bar source ever changes, re-verify this.

### 2.3 Scale and runtime budget

After filtering (§3.1) expect roughly **5,000–7,500 US common stocks**.

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
- Has at least 250 daily bars of history (needed for the 52-week window).

Store the result in `universe_symbols`. Refresh weekly.

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
| `min bars of history` | 250 | 250 |

Notes on the gate design:
- **The upper bound on `change_pct` is deliberate and central to the strategy.** The stated thesis is to enter at +8–15% on a confirmed move, not to chase something already up 50%. A stock up +60% today is not a Phase 1 candidate — it is already gone. Do not remove this bound.
- **`dollar_volume` is a required addition** not in the original filter list. Without it the penny bucket fills with illiquid names where a $50k order moves the price 20%, and RVOL is statistical noise. `dollar_volume = close × volume`.
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
high_52w        = max(high[t-251 .. t])      // 252 trading days including today
pct_of_52w_high = close[t] / high_52w        // 1.0 = at the high
new_52w_high    = close[t] > max(high[t-251 .. t-1])
```

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

**Reuse `equity_ohlcv` for daily bars** — do not create a parallel bar table. Add rows with `source = 'yahoo'` (already a documented source value) and `interval = '1Day'`.

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
🟩 ┃ 🔥 XYZ  +11.8%   Score 92/100
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
   ┃ accel 21 · rvol 19 · breakout 20 · catalyst 15 · float 10 · vwap 5 · 52w 4  (−2 rsi)
   ┃
   ┃ Daily bars · regular session only · EOD scan 2026-09-15
```

The footer must state **"regular session only"** and the scan date. Free-tier daily data is easy to misread as live, and a stale-looking number with no provenance is how a research tool turns into a bad decision.

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
| 3 | `data-universe`: resumable 3-year bar backfill | `equity_ohlcv` has `source='yahoo'` daily bars for the universe; job survives a kill and resumes |
| 4 | Feature engine (§3) + unit tests against hand-computed fixtures | Every formula tested; a no-lookahead test passes |
| 5 | Hard gates + bucketing | Candidate counts per day are sane (tens to low hundreds, not thousands) |
| 6 | Scorer (§4) with full sub-score persistence | `/score` output reconstructs the total exactly |
| 7 | **Backfill + labels + base-rate report (§6)** | Score-decile hit rates vs. base rate printed. **This is the go/no-go gate.** |
| 8 | Catalyst keyword classifier + `catalyst_events` | Matches stored with keyword and tier |
| 9 | `analyst-bot`: queries, job, formatter, 4 channels, 3 commands | Alerts post; unset channel IDs don't crash |
| 10 | Sell-side tracking (§5) | Tracked rows open and close with recorded reasons |

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
- Ask before deviating from any definition in §3 or §4 — these are the product, and a plausible-looking variation silently changes what the whole system measures.
