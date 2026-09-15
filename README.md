# trading-agent — Discord Output Reference

This document explains **everything the bot can ever post to Discord**: which channel it lands in, what triggers it, which process produced the numbers, and exactly how it is rendered in chat.

For credentials and channel setup, see [`services/analyst-bot/notifier/discord/discord.md`](services/analyst-bot/notifier/discord/discord.md).
For the meaning of individual labels and thresholds, see [`services/analyst-bot/bot.md`](services/analyst-bot/bot.md) (the same file the `/dictionary` command renders in-chat).

---

## Table of contents

1. [The four channels at a glance](#1-the-four-channels-at-a-glance)
2. [How data gets to Discord](#2-how-data-gets-to-discord)
3. [Rendering mechanics shared by every message](#3-rendering-mechanics-shared-by-every-message) — including [asset class changes the output](#asset-class-changes-the-output) and [how raw values become display text](#how-raw-values-become-display-text)
4. [`#daily-report`](#4-daily-report)
5. [`#alerts`](#5-alerts)
6. [`#actions`](#6-actions)
7. [`#commands`](#7-commands)
8. [Configuration reference](#8-configuration-reference)
9. [Reading an empty or grey card](#9-reading-an-empty-or-grey-card)

---

## 1. The four channels at a glance

| Channel | Who posts | Trigger | Message shape |
|---|---|---|---|
| `#daily-report` | Scheduler (`DailyReportJob`) | Cron `BOT_DAILY_REPORT_CRON` (default 07:00 UTC) **and** every bot startup | A burst of messages carrying 10–40 embeds: macro panels, then one card per symbol |
| `#alerts` | Scheduler (`AlertScanJob`) | Interval `BOT_ALERT_SCAN_INTERVAL` (default 300 s) | One small embed per breached threshold |
| `#actions` | Actions engine, called by `AlertScanJob` | Immediately after each alert posts, if the rule yields a directed action | One embed per actionable alert, with a confluence score and next step |
| `#commands` | The bot, replying to you | Slash commands (`/price`, `/analyze`, …) | Replies threaded to your invocation; some ephemeral |

Channels are wired purely by ID, from `.env`:

```env
DISCORD_DAILY_REPORT_CHANNEL_ID=...
DISCORD_ALERTS_CHANNEL_ID=...
DISCORD_ACTIONS_CHANNEL_ID=...
DISCORD_COMMANDS_CHANNEL_ID=...   # informational only — slash commands work anywhere
```

If an ID is unset, that channel silently produces nothing. The bot logs a warning for `#daily-report` and a debug line for `#alerts` / `#actions`, then returns. It never crashes on a missing channel.

Slash commands are **not** restricted to `#commands` by the code — `DISCORD_COMMANDS_CHANNEL_ID` is read into config but never used for gating. Restrict them through **Server Settings → Integrations** if you want them confined.

---

## 2. How data gets to Discord

The bot never calls a market API. It reads Postgres/TimescaleDB (plus Redis for caching and alert dedup), so **every number in every embed was written earlier by an upstream worker**. When a field renders as `—`, the corresponding upstream table row is missing.

```
External APIs                Ingestion (Go)              Analysis (Go)                    analyst-bot (Python)
─────────────                ──────────────              ─────────────                    ────────────────────
Alpaca, Yahoo        ──►  data-equity, data-technical ──► equity_ohlcv     ─┐
Binance, CoinGecko   ──►  data-crypto                 ──► crypto_ohlcv     ─┤
FRED                 ──►  data-equity                 ──► macro_fred       ─┼─► technical-analysis   ──► technical_indicators   ─┐
Finnhub, AlphaVantage──►  data-fundamental            ──► equity_fundamentals ─► fundamental-analysis ──► equity_fundamentals    ─┼─► ReportBuilder ──► formatter ──► discord.Embed
LunarCrush, Finnhub  ──►  data-sentiment              ──► sentiment_snapshots                             (period='derived')      │
RSS, GDELT, GPR      ──►  data-macro-intel            ──► news_headlines, *_calendar_events ─────────────────────────────────────┤
                                                          macro_fred       ───► macro-analysis      ──► macro_derived            ─┤
                                                          macro_fred+SPY   ───► market-operations   ──► macro_derived            ─┘
```

### Refresh cadences that determine how stale an embed can be

| Producer | Writes | Cadence | Env var |
|---|---|---|---|
| `data-equity` / `data-crypto` | `equity_ohlcv`, `crypto_ohlcv`, `macro_fred` | 60 s | `DATA_POLL_INTERVAL` |
| `data-technical` | daily/weekly OHLCV gap-fill | 6 h | `DATA_TECHNICAL_POLL_INTERVAL` |
| `technical-analysis` | `technical_indicators` | 6 h | `DATA_TECHNICAL_POLL_INTERVAL` |
| `data-fundamental` | raw FA (`period='ttm'`, filings) | 24 h / 168 h | `DATA_FUNDAMENTAL_*_POLL_INTERVAL` |
| `fundamental-analysis` | derived FA (`period='derived'`) | 24 h | `DATA_FUNDAMENTAL_ANALYSIS_POLL_INTERVAL` |
| `macro-analysis` | `macro_derived` (`mp_*`, `gc_*`, `inf_*`, `gg_*`, `mc_*`, `aa_*`) | 6 h | `DATA_MACRO_ANALYSIS_POLL_INTERVAL` |
| `market-operations` | `macro_derived` (`mo_reference_snapshot`) | 1 h | `MARKET_OPS_POLL_INTERVAL` |
| `analyst-bot` alert scan | reads only | 5 min | `BOT_ALERT_SCAN_INTERVAL` |

This is why the actions embed footer always says `FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min` — the alert scan runs 72× more often than the indicators it reads, so repeat scans see identical data until the analyzer runs again. Redis cooldowns (below) are what stop the same alert firing 72 times.

### The three code layers

Every message follows the same path, defined in [`services/analyst-bot/reports/builder.py`](services/analyst-bot/reports/builder.py) and [`services/analyst-bot/notifier/discord/formatter.py`](services/analyst-bot/notifier/discord/formatter.py):

1. **`ReportBuilder`** runs the SQL and fills platform-neutral dataclasses (`PriceSnapshot`, `TechnicalSnapshot`, `FundamentalSnapshot`, `MacroSnapshot`, …). It is the only layer that knows the DB schema, and it swallows per-section exceptions — a failed query logs a warning and leaves that snapshot `None` rather than killing the report.
2. **`formatter`** converts those dataclasses into `discord.Embed` objects. All emoji, colour, and layout decisions live here.
3. **`DiscordNotifier`** (scheduled posts) or the slash-command handler (on-demand) sends the embeds to a channel.

Because formatting is isolated in step 2, a second platform (Telegram, etc.) reuses steps 1 and 3 unchanged.

---

## 3. Rendering mechanics shared by every message

### Everything is an embed

The bot posts **only** embeds, never plain prose, except for a handful of error strings and `/ping`. It therefore needs the **Embed Links** permission in every channel, or posts fail with `403 Forbidden`.

### Messages get split automatically

Discord caps a single message at **10 embeds** and **6000 total characters**. Both the notifier and the command handler run the same greedy packer (`_split_embed_batches` / `_split_into_batches`): embeds are appended to the current batch until adding one would breach either cap, then a new message starts.

In chat this means a full daily report or a wide `/analyze` arrives as **several consecutive messages**, not one. For slash commands, the first batch is the command reply and the rest arrive as follow-ups. Embed order is always preserved, so panels appear in the documented sequence regardless of how they were split.

### Colour is the fastest signal

| Colour | Hex | Used for |
|---|---|---|
| 🟩 Green | `0x2ECC71` | Positive price change, `strong` FA tier, accommodative policy, expansion, benign global |
| 🟥 Red | `0xE74C3C` | Negative price change, `weak` FA tier, restrictive policy, contraction, hot inflation |
| 🟨 Yellow | `0xF39C12` | `warning` alerts, slowdown, moderate stances, market cycle |
| 🟦 Blue | `0x3498DB` | `info` alerts, technical panel, macro correlations, dictionary, `/status` |
| 🟪 Purple | `0x9B59B6` | Fundamentals, market ops, macro intel, additional analysis |
| ⬜ Grey | `0x95A5A6` | Sentiment & news, neutral tiers, **"data missing" cards** |

### `—` means "no row in the database"

Every numeric helper (`_pct`, `_price`, `_num`, `_mktcap`) returns the em dash `—` for `None`. A field full of dashes is a data-availability problem upstream, not a bot bug. See [§9](#9-reading-an-empty-or-grey-card).

### Whole panels disappear when empty

Most macro and fundamental panels begin with a `has_data` guard and return `None` when every input is missing, so the embed is omitted entirely rather than rendered blank. Three panels are deliberately different — market cycle, macro correlations, and additional analysis return a **grey diagnostic card with remediation steps** instead of vanishing, because their absence usually means the `macro-analysis` worker is stale or disabled.

### Emoji are a lookup table, not decoration

`_tier_emoji` maps ~90 tier strings to 🟢/🟡/🔴/⚪; `_trend_emoji` maps trend direction to 📈/📉/↔️; `_regime_emoji` maps VIX regime to 😱/⚠️/✅/💤. Any string not in the table falls back to ⚪ or `—`, which is how a new upstream tier name shows up as an uncoloured label.

> **Known cosmetic gap:** `technical-analysis` writes trend direction as `up` / `down` / `sideways` ([`compute/trend.go`](services/data-analyzer/internal/compute/trend.go)), but `_trend_emoji` only has keys `uptrend` / `downtrend` / `sideways`. Trending symbols therefore render as `— up` or `— down` — correct text, missing arrow. Only `sideways` gets its ↔️. This is visible in every live card; see the [real cards](#three-real-cards-and-what-each-one-teaches) below.

### Asset class changes the output

The same command produces structurally different output for a US stock, an ETF, a crypto pair, or a foreign listing — because the fundamentals pipeline only produces meaningful rows for companies that file financials. This is the single biggest source of "why does my symbol look different".

| | US stock (AAPL, TSM) | ETF (SPY, XLF, EEM) | Crypto (BTCUSDT) | Foreign listing (2222.SR, OXIG.L) |
|---|---|---|---|---|
| Price, Technical panels | full | full | full | full |
| `vix_regime` **row in DB** | yes | yes | **yes** — written for crypto too | yes |
| VIX **shown** by bot | yes | yes | only in `/analyze` market-ops, relabelled | yes |
| Fundamentals panel | full | **empty placeholders** or absent | never | full *if* the data provider covers it, else absent |
| Balance Sheet / Deep Context / Qualitative / Correlations | when data exists | never | never | rarely |
| `FA:` chip on daily card | yes | no | no | only with coverage |
| `FA:` in actions Context strip | yes | **no** | **no** | only with coverage |
| `fa_tier_flip` alert possible | yes | no | no | only with coverage |
| RSI action (`BUY_WATCH`/`TRIM_WATCH`) | reachable | **unreachable** | **unreachable** | needs coverage |

Two consequences that look like bugs but are not:

- **An ETF can show a Fundamentals panel where every field is `⚪ —`.** `data-fundamental` calls the provider's metrics endpoint for every configured symbol and upserts rows even when the values come back null. The bot builds a snapshot whenever *any* row exists, so SPY gets a panel with no content and no composite tier. An ETF with no rows at all (XLF, EEM) correctly has no panel. Same command, two different-looking results, both correct.
- **The same ETF shows the empty panel in `/analyze` but no fundamentals at all on its daily card.** The two code paths have different emptiness rules: the `/analyze` panel renders a fixed set of fields with placeholders, while the daily card builds a chip string and adds the field only if at least one chip has a value. So `/analyze SPY` shows eight `⚪ —` rows and the SPY card in the daily report shows nothing — the underlying data is identical.
- **RSI alerts on ETFs and crypto never reach `#actions`.** The only two confluence checks that work are trend direction and FA tier, and these assets have no FA tier — so confluence caps at 1, below the minimum of 2. Their liquidity-sweep alerts *do* produce actions, because that rule never consults fundamentals.

### How raw values become display text

Upstream workers store machine-readable strings like `bull_fragile_global` and `hawkish_bias`. Different panels transform them differently, so **the same value can appear three ways in one report**:

| Transformation | Where | `hawkish_bias` renders as |
|---|---|---|
| Raw, wrapped in backticks | Monetary Policy rows | `` `hawkish_bias` `` |
| Underscores → spaces | Growth, Inflation, Global rows; Additional-analysis regimes | `hawkish bias` |
| Underscores → spaces, Title Case | Market cycle + Macro correlation **titles** | `Hawkish Bias` |
| Raw | Market cycle + Macro correlation **body** fields, and all correlation `flags` | `hawkish_bias` |

So when you search this document (or the code) for a label you saw in Discord, strip the formatting first: `stall speed` in a Growth row is `stall_speed` in the database, and a Market cycle title of `Bull Fragile Global` is `bull_fragile_global`.

Emoji are then applied by lookup. Growth and Inflation classify by **keyword set** (`GOOD` → 🟢, `WARN` → 🟡, `BAD` → 🔴), so a regime string that is not in any set renders ⚪ even though its text is correct. Monetary Policy and the tier fields use explicit per-value maps. **Any unmapped value falls through to ⚪ or `—`** — which is how a newly added upstream label shows up as an uncoloured row rather than an error.

### How this document diagrams an embed

Every mockup in this README uses the same notation, chosen to mirror what Discord actually draws:

```
🟦 ┃ Title line                          ← embed title
   ┃
   ┃ Description text, if any            ← embed description (italic in chat)
   ┃
   ┃ Field Name                          ← a full-width field (inline=False)
   ┃ field value, possibly multi-line
   ┃
   ┃ Inline A      Inline B      Inline C   ← inline fields, 3 per row
   ┃ value a       value b       value c
   ┃
   ┃ ⏱️ footer text                       ← embed footer (small, grey)
```

- The **coloured square at the top-left** is the vertical colour strip Discord paints down the embed's left edge. It is the fastest thing to read in a busy channel, so it is called out on every mockup.
- **`┃`** marks the embed boundary. Everything inside one `┃` block is a single embed; consecutive blocks are consecutive embeds.
- **Inline fields pack 3 to a row**, then wrap. This is Discord's layout, not something the bot controls — it is why the alert embed shows `Type / Exchange / Interval` across one row and `Value` alone on the next.
- Values shown are representative, not captured from a specific run. Anything rendered as `—` is a `None` from the database.
- `**bold**` in a value is literal Markdown the bot emits; Discord renders it bold inside embed fields.

---

## 4. `#daily-report`

### What triggers it

| Trigger | Mechanism | Symbol universe |
|---|---|---|
| Scheduled | `CronTrigger` from `BOT_DAILY_REPORT_CRON`, default `0 7 * * *` (07:00 UTC) | `BOT_EQUITY_SYMBOLS` + `BOT_CRYPTO_SYMBOLS` |
| Bot startup | `BOT_REPORT_ON_STARTUP=true` (default), fired `BOT_REPORT_ON_STARTUP_DELAY` seconds after `on_ready` (code default 30 s; `.env.example` sets 5) | same |
| `/report` | Slash command — posts to **the channel you ran it in**, not `#daily-report` | depends on `mode` |

The startup send means **every restart or redeploy drops a fresh full report** into the channel. Set `BOT_REPORT_ON_STARTUP=false` in development to avoid spam.

### The process

`DailyReportJob.run()` → `ReportBuilder.build_daily_report()`:

1. For each equity symbol, then each crypto symbol, build a full `SymbolReport` with **`use_cache=False`** — the daily report always hits the database, never Redis.
2. Build one `MacroSnapshot` by reading ~50 `macro_derived` metrics plus raw FRED series.
3. Build one `MacroIntelSnapshot`: next 72 h of economic events, next 14 days of earnings, latest GPR month, latest GDELT day, latest FOMC narrative, 8 macro-tagged headlines.
4. `formatter.daily_report_embeds()` assembles the embed list.
5. `DiscordNotifier.send_daily_report()` batches and sends it.

Each step is independently exception-guarded, so a broken symbol or a missing macro table degrades one card instead of losing the report.

### Every embed the report can contain, in order

| # | Embed | Colour | Appears when | Content |
|---|---|---|---|---|
| 1 | **📊 Daily Market Report — `<ts>`** | Blue | Always | Timestamp, plus a one-line `Macro` field: `VIX: 14.2 \| 10Y: 4.28% \| EUR/USD: 1.0845`. With `/report <mode>`, the mode tag is appended to the title (`… Report · ETFs — …`) |
| 2 | **🏦 Monetary Policy — `<stance>`** | Green/Grey/Red by stance | Any of `mp_stance`, `yield_curve_regime`, `mp_rate_regime`, `credit_regime` present | Tier 1 rows (policy rate, 2s10s curve, real rate, balance sheet, credit spreads) and Tier 2 rows (breakevens, treasury term structure, M2). Footer adds a plain-language warning when the stance is restrictive or accommodative |
| 3 | **📈 Growth Cycle — `<stance>` (`<score>`)** | Green/Yellow/Red | `gc_stance` or any of PMI/GDP/payrolls present | Tier 1 leading (ISM PMI, LEI, initial claims, housing), Tier 2 coincident (real GDP, payrolls + unemployment + Sahm, real retail), Tier 3 lagging (Michigan sentiment, core capex) |
| 4 | **🌡️ Inflation & Prices — `<stance>` (`<score>`)** | Red/Yellow/Blue | `inf_stance` or CPI/PCE/WTI present | Tier 1 core (Core PCE vs the Fed's 2 % target, CPI, shelter CPI with its 18-month lag note), Tier 2 pipeline (PPI + PPI−CPI margin spread, oil), Tier 3 (wages, copper) |
| 5 | **🌍 Global & Geopolitical — `<stance>` (`<score>`)** | Red/Yellow/Green | Any of broad dollar, USD/JPY, China GDP, US fiscal present | Tier 1 FX & carry (broad USD trade-weighted index — explicitly *not* ICE DXY, USD/JPY with 20-day change), Tier 2 (China GDP YoY, US deficit as % of GDP) |
| 6 | **📉 Market cycle — `<phase>`** | Yellow, or **Grey if missing** | Always renders | Composite phase + score (−1 stress … +1 constructive), interpretation, index mechanics (drawdown from peak, % vs 200 DMA, crash-velocity flag), and the blended `gc`/`mp`/`inf`/`gg` stances |
| 7 | **🔗 Macro correlations — `<regime>`** | Blue, or **Grey if missing** | Always renders | Cross-metric regime label, score, narrative read, and flags |
| 8 | **📚 Additional analysis** | Purple, or **Grey if missing** | Always renders | 60-day rolling bond–equity / oil–equity / VIX–equity correlations with regime labels, static month seasonality, presidential cycle year, and an "HTML coverage" list showing which reference modules are automated vs `needs_data` |
| 9 | **Macro intel · calendars · geo · headlines** | Purple | Any calendar row, GPR, GDELT, narrative, or headline exists | Next economic events (≤8) and earnings dates (≤10), GPR index + GDELT tone, the optional LLM-scored FOMC narrative, and ≤6 macro headlines |
| 10…N | **📈/📉 `SYMBOL`  `$PRICE`  (`±X%`)** | Green if FA `strong`, Red if `weak`, else Grey | One per symbol | See below |
| N+1 | **📈 FRED · latest observations** | Grey | Only via `/report commodities` or `/report macro_fred` | 20 series per embed, each as an inline field showing value + observation date |
| N+2 | **📌 Dashboard strip** | Blue | Only via `/report dashboard` | 11 fixed market rows (DXY, US10Y, US02Y, T10Y2Y, SPX, DAX, MCHI, USOIL, GOLD, HG1!, USDJPY) as a single description block |

Embeds 2–9 are built from `macro_derived`, written by the `macro-analysis` worker every 6 hours. Each stance is a weighted composite the worker computes from FRED series and stores with a `regime` label, so the bot only formats a decision that was already made upstream — it does not classify anything itself.

### The macro panels, rendered

Each panel below shows a **sample** (transcribed from live output) followed by the **complete value domain** for every slot in it, so you can interpret any future run rather than just this one.

**1 · Header** — the only embed guaranteed to appear. Its `Macro` field is raw FRED, not a computed stance, so it has no emoji or regime.

```
🟦 ┃ 📊 Daily Market Report — 2026-04-16 16:01 UTC
   ┃
   ┃ Macro
   ┃ VIX: 18.2 | 10Y: 4.26% | EUR/USD: 1.1723
```

| Slot | Possible values |
|---|---|
| Title tag | absent (scheduled / `standard` / `all`), or ` · ETFs`, ` · Commodities`, ` · FRED series`, ` · Crypto`, ` · Equities`, ` · Dashboard` |
| `VIX` | any float, 1 dp. Omitted if neither `macro_fred.VIXCLS` nor the benchmark TA fallback has a value |
| `10Y` | any float, 2 dp, `%` suffix. Omitted when `DGS10` is missing |
| `EUR/USD` | any float, 4 dp. Omitted when `DEXUSEU` is missing |
| Whole `Macro` field | **omitted entirely** if all three are missing, leaving a title-only embed |

**2 · Monetary Policy** — colour and title emoji follow `mp_stance`. This is the one panel that prints regimes **raw in backticks**, so you see `re_steepening` rather than `re steepening`.

```
🟩 ┃ 🏦 Monetary Policy — 🟢 accommodative (+0.55)
   ┃
   ┃ Monetary Policy — Tier 1
   ┃ 🟢 Policy Rate — 3.64%  `cutting` (-69bps YoY)
   ┃ 🟡 Yield Curve (2s10s) — +0.53pp  `normal`  3m10y: +0.58pp
   ┃ 🟡 Real Rate (TIPS 10Y) — +1.89%  `balanced`  (BE 10Y: 2.39%)
   ┃ 🟡 Balance Sheet — $6693.9B  `neutral`  (+48B / 4w)
   ┃ 🟢 Credit Spreads — HY 284bps / IG 81bps  `benign`
   ┃
   ┃ Bond Market — Tier 2
   ┃ 🟢 Breakeven Inflation — 10Y: 2.39% / 5Y: 2.61%  `anchored`
   ┃ 📊 Treasury Yields — 2Y: 3.76% | 10Y: 4.26% | 30Y: 4.87%
   ┃ 🟡 M2 Money Supply — +4.88% YoY  `normal`  (M2: $22667B)
   ┃
   ┃ Score: +1.0 = max accommodative | -1.0 = max restrictive · 🟢 Accommodative
   ┃ policy supports risk assets and growth equities.
```

| Row | Every possible regime | Emoji |
|---|---|---|
| **Policy Rate** | `cutting` / `neutral` / `hiking` | 🟢 / 🟡 / 🔴 |
| **Yield Curve** | `steep` 🟢 · `normal` 🟡 · `flat` 🟠 · `inverted` 🔴 · `re_steepening` 🔴🔴 | as listed |
| **Real Rate** | `deeply_negative` 🟢 · `balanced` 🟡 · `headwind` 🔴 | |
| **Balance Sheet** | `qe` 🟢 · `neutral` 🟡 · `qt` 🔴 | `neutral` is the common case when the Fed is neither expanding nor shrinking fast |
| **Credit Spreads** | `benign` 🟢 · `elevated` 🟠 · `crisis` 🔴 | |
| **Breakeven Inflation** | `anchored` 🟢 · `rising` 🟡 · `unanchored` 🔴 | |
| **Treasury Yields** | *no regime* — always 📊, a data row only | any/all of 2Y, 10Y, 30Y; row omitted if all three missing |
| **M2 Money Supply** | `deflationary` 🟢 · `normal` 🟡 · `slow` 🟡 · `inflationary` 🔴 | |
| **Title stance** | `accommodative` 🟢 · `neutral` 🟡 · `restrictive` 🔴 · `insufficient_data` ⚪ | |

**Score sign convention: `+1.0` = maximally accommodative, `−1.0` = maximally restrictive.** The sample's `+0.55` is therefore *easy* policy. This convention is **not shared** by the other three stance panels — see the warning after panel 5.

The footer's second clause is conditional: `⚠️ Restrictive policy is a headwind…` when restrictive, `🟢 Accommodative policy supports…` when accommodative, and nothing at all when neutral.

**3 · Growth Cycle** — three tiers ordered by how early they turn. This panel converts underscores to spaces, so `stall_speed` prints as `stall speed`.

```
🟨 ┃ 📈 Growth Cycle — 🟡 Slowdown (+0.27)
   ┃
   ┃ Leading Indicators — Tier 1
   ┃ ⚪ ISM PMI — — · no data
   ┃ 🟢 LEI — expanding (+9.1% 6m)
   ┃ 🟢 Initial Claims — 4w avg: 209,500 · CCSA 1,794,000 · tight labor
   ┃ 🟡 Housing — Starts 1,487K · Permits 1,386K · moderate
   ┃
   ┃ Coincident Indicators — Tier 2
   ┃ 🟡 Real GDP — +0.5% ann. · stall speed
   ┃ 🟡 Payrolls — +178K · UNRATE 4.3% · Sahm 0.20pp · moderate
   ┃ 🟡 Real Retail — +1.1% YoY · slowing
   ┃
   ┃ Lagging / Sentiment — Tier 3
   ┃ 🟢 Michigan Sentiment — 56.6 · near bottom
   ┃ 🟡 Core Capex — +1.6% 3m · stable
   ┃
   ┃ Growth Cycle · 8 signals · FRED free data · see bot.md for thresholds
```

| Row | Every possible regime (as displayed) |
|---|---|
| **ISM PMI** | `strong expansion` · `expansion` · `slowing` · `contraction` · `severe contraction` · `no data`. Optional trend suffix ` · improving` / ` · deteriorating` (suppressed when `stable`) |
| **LEI** | `expanding` · `slowing` · `recession risk` · `rule of three decline` · `stable` · `no data` |
| **Initial Claims** | `tight labor` · `normal` · `normalizing` · `crisis` · `no data` |
| **Housing** | `strong` · `moderate` · `weak` · `no data` |
| **Real GDP** | `strong` · `moderate` · `stall speed` · `recession` · `no data` |
| **Payrolls** | `strong` · `moderate` · `slowing` · `contraction` · `recession confirmed` · `no data`. A Sahm rule reading ≥ 0.5pp forces `recession confirmed` regardless of the payroll print |
| **Real Retail** | `healthy` · `slowing` · `contraction` · `no data` |
| **Michigan Sentiment** | `near bottom` · `pessimistic` · `normal` · `complacency` · `no data`. Row omitted when the series is absent |
| **Core Capex** | `expanding` · `stable` · `slowing` · `warning` · `no data`. Row omitted when absent |
| **Title stance** | `Expansion` 🟢 · `Slowdown` 🟡 · `Contraction` 🔴 · `Insufficient Data` ⚪ |

`⚪ ISM PMI — — · no data` in the sample is the **normal** state, not a fault: the ISM series is licensed and the free FRED tier does not carry it, so this row reads `no data` on most installs. Tier 3 is dropped entirely when neither Michigan sentiment nor capex has data.

Note `near bottom` is coloured 🟢 — consumer sentiment at a trough is treated as contrarian-positive. Emoji here come from keyword sets rather than per-row maps, so a few rows carry counter-intuitive colours.

**4 · Inflation & Prices** — Core PCE leads because it is the Fed's actual target.

```
🟥 ┃ 🌡️ Inflation & Prices — 🔴 Hot (+0.55)
   ┃
   ┃ Core Inflation — Tier 1
   ┃ 🟡 Core PCE — +2.97% · hawkish bias (Fed target 2.0%)
   ┃ 🟡 CPI — +3.32% · Core +2.67% · rising
   ┃ 🟡 Shelter CPI — +3.23% · moderating (18m lag)
   ┃
   ┃ Pipeline & Energy — Tier 2
   ┃ 🟡 PPI Final Demand — +4.02% · elevated · spread +0.7pp 🟡
   ┃ ⚪ PPI All Commodities — +6.03%
   ┃ 🔴 Oil — WTI $100.72 | Brent $123.28 | B-W +22.56 · inflationary risk
   ┃
   ┃ Wages & Commodities — Tier 3
   ┃ 🟡 Wages — AHE +3.52% · ECI +3.41% · above target
   ┃ 🟢 Copper — +28.69% YoY ($12,529.00/t) · global expansion
   ┃
   ┃ Inflation · 7 signals · FRED free data · see bot.md for thresholds
```

| Row | Every possible regime (as displayed) |
|---|---|
| **Core PCE** | `aggressive tightening` · `hawkish bias` · `at target` · `below target` · `no data` |
| **CPI** | `hot` · `above target` · `rising` · `goldilocks` · `below target` · `deflation risk` · `no data` |
| **Shelter CPI** | `hot` · `elevated` · `moderating` · `normalizing` · `no data`. Row omitted when absent |
| **PPI Final Demand** | `surge` · `elevated` · `moderate` · `stable` · `deflationary` · `no data` |
| **PPI margin suffix** | `margin_pressure` 🔴 · `margin_expansion` 🟢 · `neutral` 🟡. Appears only when CPI is also available |
| **PPI All Commodities** | *no regime* — always ⚪, a data row only |
| **Oil** | `inflationary risk` · `elevated` · `goldilocks` · `low` · `energy sector stress` · `no data`. Brent and the Brent−WTI spread are appended only when present |
| **Wages** | `spiral risk` · `elevated` · `above target` · `target consistent` · `soft` · `no data` |
| **Copper** | `global expansion` · `stable` · `slowing` · `global contraction` · `no data` |
| **Title stance** | `Hot` 🔴 · `Moderate` 🟡 · `Deflationary` 🔵 · `Insufficient Data` ⚪ |

The trailing emoji on the PPI row is a *second, independent* signal — the PPI−CPI spread's margin read — separate from the row's own regime emoji. Two emoji on one line is not a rendering error.

**5 · Global & Geopolitical** — the two italic disclaimers are deliberate: the dollar index is the FRED trade-weighted series, not ICE DXY, and China GDP is OECD quarterly, so it can be months stale.

```
🟥 ┃ 🌍 Global & Geopolitical — 🔴 Elevated stress (+0.43)
   ┃
   ┃ FX & carry — Tier 1
   ┃ 🔴 Broad USD (TWI) — 118.86 · major global stress (not ICE DXY)
   ┃ 🟢 USD/JPY — 159.22 · 20d +0.25% · carry intact
   ┃
   ┃ China & fiscal — Tier 2
   ┃ 🔴 China GDP YoY — +3.47% · slowing (OECD quarterly)
   ┃ 🟡 US fiscal — deficit 5.65% of GDP · elevated supply risk
   ┃
   ┃ Global · 4 signals · FRED · Market cycle + Macro intel embeds · see bot.md
```

| Row | Every possible regime (as displayed) | Emoji rule |
|---|---|---|
| **Broad USD (TWI)** | `major global stress` 🔴 · `em commodity headwind` 🟡 · `neutral` 🟡 · `supportive equities` 🟢 · `dollar weak risk on` 🟢 · `no data` | only `major global stress` is red |
| **USD/JPY** | `systemic carry unwind` 🔴 · `early carry unwind` 🟡 · `carry intact` 🟢 · `no data` | 20-day change suffix appears when available |
| **China GDP YoY** | `expansion` 🟢 · `stable` 🟡 · `slowing` 🔴 · `contraction risk` 🔴 · `no data` | |
| **US fiscal** | `high deficit stress` 🔴 · `elevated supply risk` 🟡 · `manageable` 🟢 · `no data` | |
| **Title stance** | `Benign` 🟢 · `Moderate` 🟡 · `Elevated stress` 🔴 · `Insufficient Data` ⚪ | |

When GDP is missing but the raw deficit exists, the fiscal row degrades to a data-only line: `⚪ US fiscal — FYFSD -1,832,000M USD (GDP missing for %GDP) · —`.

> ### ⚠️ The four stance scores do not share a sign convention
>
> Every panel prints `(±0.00)` next to its stance, which invites reading them as one scale. They are not.
>
> | Panel | `+1.0` means | `−1.0` means | Bands |
> |---|---|---|---|
> | 🏦 Monetary Policy | accommodative (**good for risk**) | restrictive | `> +0.4` / `> −0.4` / else |
> | 📈 Growth Cycle | expansion (**good**) | contraction | `≥ +0.4` / `≤ −0.4` / else |
> | 🌡️ Inflation | **hot (bad)** | deflationary | `≥ +0.4` / `≤ −0.4` / else |
> | 🌍 Global | **elevated stress (bad)** | benign | `≥ +0.35` / `≤ −0.15` / else |
>
> So the samples above — `+0.55` policy, `+0.27` growth, `+0.55` inflation, `+0.43` global — describe *easy policy, slowing growth, hot inflation, and global stress*. Positive is good for the first two and bad for the last two. Note also that Growth's `+0.27` lands in the **middle** band, which is why it reads `Slowdown` despite a positive score, and that Global's bands are asymmetric.

**6 · Market cycle** — the only symbol-aware macro panel: it reads `MARKET_CYCLE_SYMBOL` (default SPY) price action and blends it with the four stances above.

```
🟨 ┃ 📉 Market cycle — Bull Fragile Global
   ┃
   ┃ Composite
   ┃ bull_fragile_global · score 0.22 (−1 stress … +1 constructive)
   ┃
   ┃ Interpretation
   ┃ Strong price trend but global stress elevated — geopolitical/USD channel
   ┃ can snap leaders.
   ┃
   ┃ Index (reference HTML thresholds)
   ┃ Symbol SPY · price phase bull_extended
   ┃ Drawdown from —d peak: -0.05%
   ┃ vs 200DMA: 5.16% · SMA200 665.60 · close 699.94
   ┃ Crash velocity flag: no
   ┃
   ┃ Blended inputs
   ┃ Growth slowdown · Policy accommodative · Inflation hot · Global elevated_stress
   ┃
   ┃ mc_market_cycle · 320 daily bars · MARKET_CYCLE_* in .env
```

| Slot | Every possible value |
|---|---|
| **`composite_phase`** (title, Title-Cased; body, raw) | `crash_panic` · `bear_structural` · `correction_risk` · `pullback_healthy` · `late_cycle_stretched` · `bull_fragile_global` · `bull_overextended` · `trend_soft` · `bull_macro_aligned` · `bull_macro_divergent` · `neutral_mixed` · `insufficient_equity_data` |
| **`price_phase`** | `crash` · `bear` · `correction` · `pullback` · `bull_extended` · `bull` · `below_sma` · `insufficient_data` |
| **`Crash velocity flag`** | `no`, or `⚠️ yes (fast move vs recent highs)`. Triggering also forces `price_phase` to `crash` |
| **`Interpretation`** | free prose from the worker — not an enum. Omitted when the worker wrote no label |
| **`Blended inputs`** | the four stance strings **raw**, so `elevated_stress` keeps its underscore here while the Global panel above showed `elevated stress`. Each clause is omitted if that stance is absent |
| **Colour** | always 🟨 yellow, regardless of phase — do not read the strip on this panel |

`Drawdown from —d peak` in the sample is **expected, and it is informative**: `days_off_peak` is zero, meaning the lookback peak *is* the most recent bar — the index is sitting at a fresh high. The formatter renders falsy values as `—`, leaving a bare `d`. When the index has pulled back, this reads normally: `Drawdown from 48d peak: -3.59%`. So `—d` with a near-zero drawdown means "at the highs", not "data missing".

The **Blended inputs** row usually self-heals. The Go worker embeds the four stances at write time, so if it ran before FRED data landed they would read `insufficient_data`; the builder detects that and backfills from the live standalone metrics, which is why the row normally agrees with panels 2–5 above it. The backfill can only work when those standalone metrics exist, though — on a report where the macro worker has not completed a full pass, this row legitimately reads `Growth insufficient_data · Policy insufficient_data · Inflation insufficient_data · Global insufficient_data`.

The `Interpretation` field is composed, not looked up. A base sentence for the phase can gain conditional clauses, so the same `pullback_healthy` phase renders as *"Pullback (−3–10%) within uptrend — usually healthy if macro intact."* on one day and gains *"Inflation still hot — size dips smaller."* on another.

**6b · Market cycle, data missing** — the grey diagnostic variant. This replaces the panel rather than hiding it, because its absence is nearly always a stale worker:

```
⬜ ┃ 📉 Market cycle — data missing
   ┃
   ┃ The daily report did not load mc_market_cycle from macro_derived.
   ┃
   ┃ • Rebuild & restart macro-analysis (Docker image with market-cycle code).
   ┃ • Ensure SPY (or MARKET_CYCLE_SYMBOL) has ≥200 1Day rows in equity_ohlcv
   ┃   (add SPY to technical ingestion if needed).
   ┃ • Check MARKET_CYCLE_ENABLE on the worker (default true).
   ┃
   ┃ If you turned market cycle off on purpose, you can ignore this card.
```

Panels 7 and 8 have the same pattern — `🔗 Macro correlations — data missing` and `📚 Additional analysis — data missing`, each listing its own remediation steps.

**7 · Macro correlations** — a single cross-regime label rather than per-series rows.

```
🟦 ┃ 🔗 Macro correlations — Stagflation Risk
   ┃
   ┃ Regime
   ┃ stagflation_risk · score -0.58 (−1 stress … +1 constructive)
   ┃
   ┃ Read
   ┃ Inflation stance hot while growth is rolling over — stagflation-style
   ┃ pressure on risk assets.
   ┃
   ┃ Flags
   ┃ inflation_hot, growth_soft, usd_strong_em_headwind, global_stress,
   ┃ energy_price_pressure
   ┃
   ┃ mc_macro_correlation · MARKET_MACRO_CORR_ENABLE · see bot.md
```

| Slot | Every possible value |
|---|---|
| **Regime** (title Title-Cased, body raw) | `recession_pipeline` · `stagflation_risk` · `rising_inflation_tight_policy` · `global_liquidity_stress` · `deflation_risk` · `goldilocks_light` · `disinflation_soft_landing` · `neutral_mixed` |
| **Score** | **not continuous** — one fixed value per regime, spanning roughly `−0.82` to `+0.48`, with exactly `0.0` for `neutral_mixed`. Two runs in the same regime always show the same score |
| **Flags** | any subset of `real_rates_headwind`, `curve_flat_or_inverted`, `credit_stress`, `inflation_hot`, `growth_soft`, `usd_strong_em_headwind`, `global_stress`, `jpy_carry_stress`, `energy_price_pressure`, printed raw and comma-joined. The field is omitted when the set is empty |
| **Read** | free prose from the worker; omitted when no label was written |
| **Colour** | always 🟦 blue when present, 🩶 grey when the metric is missing |

Regimes are evaluated as a first-match chain, so a market that qualifies for several only ever reports the highest-priority one. `neutral_mixed` is the fall-through, not a positive assessment — and because it carries no flags, the panel shrinks to two fields:

```
🟦 ┃ 🔗 Macro correlations — Neutral Mixed
   ┃
   ┃ Regime
   ┃ neutral_mixed · score 0.00 (−1 stress … +1 constructive)
   ┃
   ┃ Read
   ┃ Macro inputs do not line up into a single high-conviction regime —
   ┃ treat as mixed / data-dependent.
   ┃
   ┃ mc_macro_correlation · MARKET_MACRO_CORR_ENABLE · see bot.md
```

**8 · Additional analysis** — intermarket correlations plus two static calendar tilts. The `HTML coverage` field is an honesty report: it lists which modules of the reference document are actually automated.

```
🟪 ┃ 📚 Additional analysis · intermarket & calendars
   ┃
   ┃ _Reference doc tabs: live vs needs_data is listed under HTML coverage below._
   ┃
   ┃ Bond–equity (60d rolling)
   ┃ ρ ≈ -0.321 · regime deflationary hedge (60 obs)
   ┃ Bonds hedging equities — classic risk-off diversification is working.
   ┃
   ┃ Oil–equity (60d · WTI)
   ┃ ρ ≈ -0.451 · decoupled
   ┃
   ┃ VIX–equity (60d)
   ┃ ρ ≈ -0.895 · typical fear greed
   ┃
   ┃ Month seasonality (almanac)
   ┃ April — strong_bull
   ┃ Historically one of the strongest months.
   ┃ Static reference tilt — tie-breaker only.
   ┃
   ┃ Presidential cycle
   ┃ Year 2 of 4 — midterm
   ┃ Year 2 — midterm year; often volatile, corrections common before midterms.
   ┃
   ┃ HTML coverage (automation status)
   ┃ • alternative_data: not_automated — Vendor datasets only — no ingestion.
   ┃ • event_driven: not_automated — M&A, spinoffs, index adds — would need
   ┃   SEC/index parsers + event tables.
   ┃ • factor: partial — Quality/value overlap fundamental-analysis; momentum/vol
   ┃   from technical-analysis OHLCV — no separate factor ETF pipeline yet.
   ┃ • flow_microstructure: needs_data — UOA, GEX, dark pool, max pain — paid
   ┃   options / FINRA OTC APIs not wired.
   ┃ • intermarket: partial_live — Live rolling ρ in aa_reference_snapshot (bond,
   ┃   oil, VIX vs benchmark). Other pairs in HTML are regime guides.
   ┃ • relative_value: not_automated — Pairs / sector ratios — configure ETF
   ┃   universe + second worker pass (future).
   ┃ • seasonality: live_static — Static month almanac + presidential cycle in
   ┃   payload — not backtested on your DB history.
   ┃ • sentiment: needs_data — PCR, IV skew, short interest, margin — need
   ┃   options/FINRA feeds (see HTML data sources).
   ┃
   ┃ aa_reference_snapshot · ADDITIONAL_ANALYSIS_* · additional_analysis_reference.html
```

| Slot | Every possible value |
|---|---|
| **Bond–equity regime** | `deflationary hedge` (ρ ≤ −0.25) · `inflationary positive` (ρ ≥ +0.25) · `transition neutral` · `insufficient data` |
| **Oil–equity regime** | `procyclical` (ρ ≥ +0.25) · `decoupled` (ρ ≤ −0.25) · `neutral mixed` · `insufficient data` |
| **VIX–equity regime** | `typical fear greed` (ρ ≤ −0.25) · `unusual positive` (ρ ≥ +0.15) · `compressed link` · `insufficient data` |
| **ρ and obs count** | computed floats — the ρ value is continuous, only the regime label is bucketed |
| **Month bias** | fixed per calendar month, never computed: Jan `mild_bull`, Feb `neutral`, Mar `mild_bull`, **Apr `strong_bull`**, May `neutral`, Jun `neutral`, Jul `mild_bull`, Aug `mild_bear`, Sep `weak_bear`, Oct `mild_bull`, Nov `strong_bull`, Dec `strong_bull` |
| **Presidential cycle label** | `post_election` · `midterm` · `pre_election` · `election`, with bias `moderate_constructive` · `choppy` · `strong_historical` · `positive_volatile` |
| **HTML coverage module keys** | exactly eight, always all present, alphabetical: `alternative_data`, `event_driven`, `factor`, `flow_microstructure`, `intermarket`, `relative_value`, `seasonality`, `sentiment` |
| **HTML coverage statuses** | `not_automated` · `partial` · `partial_live` · `live_static` · `needs_data`. There is **no plain `live` status** — even the automated modules are qualified |

Two things in this panel that look wrong but are not:

- **The description renders with literal underscores** — `_Reference doc tabs: live vs needs_data is listed…_`. The text is wrapped in `_…_` to italicise it, but `needs_data` contains an underscore that terminates the emphasis early, so Discord gives up and prints the markers. Cosmetic only.
- **Month and cycle biases never change within a month.** They are a hardcoded almanac table, not a backtest against your own database. The panel says so itself: *"Static reference tilt — tie-breaker only."*

Each correlation degrades independently, and **the field name changes when it does**. With data they read `Bond–equity (60d rolling)`, `Oil–equity (60d · WTI)`, `VIX–equity (60d)`; without it, all three become `… (60d rolling)` and the ρ line is replaced by a diagnostic:

```
🟪 ┃ 📚 Additional analysis · intermarket & calendars
   ┃
   ┃ Bond–equity (60d rolling)
   ┃ Insufficient overlapping SPY (or benchmark) bars and DGS10 — check ingestion.
   ┃
   ┃ Oil–equity (60d rolling)
   ┃ Insufficient DCOILWTICO or bars for this window.
   ┃
   ┃ VIX–equity (60d rolling)
   ┃ Insufficient VIXCLS or bars for this window.
   ┃
   ┃ Month seasonality (almanac)
   ┃ April — strong_bull
   ┃ …
```

Each diagnostic names the exact FRED series to check, and the seasonality and coverage fields below are unaffected — they need no market data at all, which is why this panel never disappears entirely.

**9 · Macro intel** — the only embed with no emoji in its title. Calendars, geopolitical tone, and headlines from `data-macro-intel`.

```
🟪 ┃ Macro intel · calendars · geo · headlines
   ┃
   ┃ Calendars (next window)
   ┃ • `09-12 12:30 UTC` US CPI m/m (high)
   ┃ • `09-12 14:00 UTC` US Michigan Sentiment Prelim (medium)
   ┃ • `09-15 08:00 UTC` EU Industrial Production (low)
   ┃ • MSFT 2026-09-16 Q1
   ┃ • GOOGL 2026-09-18 Q3
   ┃
   ┃ Geopolitical / news tone
   ┃ GDELT 2026-09-10 `macro_query_default` — n=120, avg tone 0.000
   ┃
   ┃ Narrative & macro headlines
   ┃ • [FH·general] Treasury yields ease after soft claims print — https://…
   ┃ • [FH·general] Rates Spark: as the dust settles — https://…
   ┃
   ┃ data-macro-intel · /status → macro table counts · Finnhub calendar may 403 on some tiers
```

| Slot | Every possible value |
|---|---|
| **Economic event line** | `` `MM-DD HH:MM UTC` `` + 2-letter country + event name + `(impact)`. Country is whatever the provider sends — `US`, `EU`, `GB`, `AU`, `NZ`, `PY`, and many more; it is **not** filtered to majors. Impact is `low` · `medium` · `high` |
| **Earnings line** | `**SYMBOL** YYYY-MM-DD Q<n>` where `n` is 1–4. Quarter suffix omitted when the provider gives no quarter |
| **Event counts** | economic events cap at 8, earnings at 10, headlines at 6 — each independently, and the whole field disappears when its list is empty |
| **`GPR` line** | present only when `geopolitical_risk_monthly` has a row. Monthly series, so the date is often weeks old. **Frequently absent**, as in the sample |
| **`GDELT` line** | present only when `gdelt_macro_daily` has a row. The backticked token is the configured query key (default `macro_query_default`); `avg tone` is a float that can legitimately be exactly `0.000` |
| **`FOMC narrative`** | present only when `BOT_FOMC_NARRATIVE_ENABLE=true` has produced a `narrative_scores` row. Off by default, so most installs never see this line |
| **Headline source tag** | first token of the source, abbreviated: `finnhub_*` → `FH`, `rss_*` → `RSS`, then `·` and the remainder — so `finnhub_macro_general` prints as `[FH·general]` |
| **Colour** | always 🟪 purple |

An empty `Geopolitical / news tone` section is the default state on a fresh install: both GPR and GDELT are optional collectors. The `Narrative & macro headlines` field is also subject to the 1024-character limit and is truncated mid-entry when the URLs are long, so a final line reading `• [FH·gene…` is the field running out of room rather than a malformed headline. The footer's 403 warning is there because Finnhub gates calendar endpoints by plan tier — empty calendars are usually a plan limitation, not a bug.

#### What a half-populated macro stack looks like

The panels are not all-or-nothing, and they do not fail as a group. Growth, Inflation and Global each return nothing at all when their stance is `insufficient_data` and no fallback series exist, so **those three embeds simply do not appear**. Monetary Policy has a looser guard — it renders as long as any raw FRED series is present — so it appears in a fully degraded form instead:

```
⬜ ┃ 🏦 Monetary Policy — ⚪ insufficient_data (+0.00)
   ┃
   ┃ Monetary Policy — Tier 1
   ┃ ⚪ Policy Rate — 3.64%  —
   ┃ ⚪ Yield Curve (2s10s) — —  —
   ┃ ⚪ Real Rate (TIPS 10Y) — +1.99%  —  (BE 10Y: 2.36%)
   ┃ ⚪ Balance Sheet — —  —
   ┃ ⚪ Credit Spreads — HY —  —
   ┃
   ┃ Bond Market — Tier 2
   ┃ ⚪ Breakeven Inflation — 10Y: 2.36% / 5Y: 2.60%  —
   ┃ 📊 Treasury Yields — 2Y: 3.84% | 10Y: 4.35% | 30Y: 4.91%
   ┃ ⚪ M2 Money Supply — —  —  (M2: $22667B)
   ┃
   ┃ Score: +1.0 = max accommodative | -1.0 = max restrictive
```

Every row still appears, because the row list is fixed — only the values and regimes are `—`. Note the mixed states within one panel: `Policy Rate` has a level but no regime, `Balance Sheet` has neither, `Credit Spreads` keeps its `HY ` prefix with nothing after it, and `Treasury Yields` is fully populated because it needs no classification. The `insufficient_data` stance also prints raw in the title rather than Title-Cased, unlike the other three panels.

A report in this state shows **Monetary Policy, Market cycle, Macro correlations, Additional analysis and Macro intel, but no Growth, Inflation or Global** — five macro embeds instead of eight. That is the signature of a `macro-analysis` worker that has not completed a full pass yet, not of a broken bot. `/status` will show the `macro_derived` row count.

**N+1 · FRED observations** — only via `/report commodities` or `/report macro_fred`. 20 inline fields per embed, so a 50-series request produces three embeds titled `…(cont.)`.

```
⬜ ┃ 📈 FRED · latest observations
   ┃
   ┃ DGS10          DGS2           DGS30
   ┃ 4.28           4.66           4.45
   ┃ `2026-09-10`   `2026-09-10`   `2026-09-10`
   ┃
   ┃ VIXCLS         FEDFUNDS       T10Y2Y
   ┃ 14.2           5.33           -0.38
   ┃ `2026-09-10`   `2026-08-31`   `2026-09-10`
```

The per-field date is the *observation* date, not the fetch date — this is how you spot a monthly series that has not updated in weeks.

**N+2 · Dashboard strip** — only via `/report dashboard`. A single description block, not fields, so it stays compact. Each row is label · hint on one line, value beneath.

```
🟦 ┃ 📌 Dashboard strip
   ┃
   ┃ DXY · FRED broad USD index (not ICE DXY)
   ┃ 121.45
   ┃
   ┃ US10Y · 10Y yield %
   ┃ 4.28%
   ┃
   ┃ T10Y2Y · 10Y−2Y spread (pp)
   ┃ -0.38 pp
   ┃
   ┃ SPX · SPY proxy for S&P 500
   ┃ 651.87
   ┃
   ┃ DAX · index / ADR if in OHLCV
   ┃ —
   ┃
   ┃ … 6 more rows (US02Y, MCHI, USOIL, GOLD, HG1!, USDJPY)
```

| Row | Source | Rendering |
|---|---|---|
| `DXY` | FRED `DTWEXBGS` | plain float |
| `US10Y`, `US02Y` | FRED `DGS10`, `DGS2` | 2 dp with `%` |
| `T10Y2Y` | FRED `T10Y2Y` | 2 dp with ` pp` |
| `SPX` | **SPY's share price**, not the index | plain float |
| `DAX` | OHLCV `^GDAXI` if ingested | usually `—` |
| `MCHI` | OHLCV `MCHI` | plain float |
| `USOIL` | FRED `DCOILWTICO` | plain float |
| `GOLD` | **GLD's share price**, not $/oz | often `—` |
| `HG1!` | FRED `PCOPPUSDM` — monthly copper, $/metric tonne | plain float |
| `USDJPY` | FRED `DEXJPUS` | 4 dp |

Exactly 11 rows, always all present — a missing series renders `—` rather than dropping the row, which is how you tell "not ingested" from "not in the list".

> **The row definitions are the one place this document can drift from a running bot.** They are a hardcoded tuple in `reports/builder.py`, so a deployed build compiled from a different commit will show different hints and sources. If your `GOLD` row reads `Gold LBMA` and renders `—` while your GLD symbol card shows a price, your bot is looking up a FRED gold series rather than the GLD share price this repo specifies — the deployment is ahead of or behind this source tree. Check the tuple in your running build before treating a `—` as an ingestion failure. The hints exist because several rows are proxies rather than the real instrument: reading `SPX` or `GOLD` as an index or a spot price would be wrong, and `HG1!` is a monthly macro series rather than the front-month future its name suggests.

### The per-symbol card

One compact embed per symbol, built by `_daily_report_symbol_summary_embed`. **Every slot on this card is optional except the title**, so two cards in the same report can look completely different without anything being wrong.

#### Every slot and its value domain

| Slot | Possible values | When it is absent |
|---|---|---|
| Title arrow | `📈` when the day's change is ≥ 0, `📉` when negative | never — but the price and change are dropped if there is no bar, leaving a bare symbol |
| Title price | `$` + close, with the precision chosen by **magnitude, not asset class**: `$74,785.90` at or above 1,000 (comma-grouped, 2 dp), `$4.17` between 1 and 1,000 (2 dp), and `$0.300800` below 1 (**6 dp**) | no OHLCV row |
| Colour strip | 🟩 green if FA tier `strong` · 🟥 red if `weak` · ⬜ grey for `neutral`, missing, or any asset with no FA | — |
| **Technical › RSI** | `RSI 0`–`RSI 100`, integer | RSI indicator missing or period env changed |
| **Technical › trend** | `— up` · `— down` · `↔️ sideways` | trend indicator missing |
| **Technical › squeeze** | `🔴 Squeeze` only | omitted whenever there is no squeeze — there is no "no squeeze" chip |
| **Technical › MACD** | `🟢 MACD cross` for a bullish cross, `🔴 MACD cross` for a bearish one — **identical text, only the dot differs** | omitted unless a cross fired on the latest bar |
| **Technical › H&S** | `🔴 H&S` or `🔴 H&S✅` | no pattern detected |
| **Technical › Inv H&S** | `🟢 Inv H&S` or `🟢 Inv H&S✅` | no pattern detected |
| **Fundamentals › FA** | `🟢 strong` · `🟡 neutral` · `🔴 weak`, plus the composite score in parentheses | no composite → whole chip dropped |
| **Fundamentals › P/E** | `cheap_vs_history` · `fair_vs_history` · `expensive_vs_history` · `loss_making`, or the absolute-P/E fallbacks `value` · `growth_fair` · `expensive` when no 5-year mean exists | no P/E tier |
| **Fundamentals › FCF** | `attractive` · `fair` · `avoid` | no FCF yield tier |
| **Fundamentals › BS** | `🟢 healthy` · `🔴 stressed` | omitted when the health tier is `neutral` or missing |
| **Whole Fundamentals field** | — | crypto always; ETFs almost always; foreign listings without provider coverage |
| **Sentiment (`source`)** | a float score; the field *name* carries the source | no `sentiment_snapshots` row |
| **Latest News** | newest headline, truncated to 100 characters | no `news_headlines` row |
| **Market ops** | `VIX regime: <regime> · ATR% <n> · Vol×median <n>` with any flags appended after a final `·` | no `mo_reference_snapshot` |
| **Market ops flags** | `atr_pct_elevated` when ATR% ≥ 3.0, `volume_vs_median_elevated` when the volume ratio ≥ 1.8, comma-joined | neither threshold crossed |
| **Footer** | `VIX: 😱 extreme_fear` · `⚠️ elevated` · `✅ normal` · `💤 complacency` | crypto, or no VIX row |

The `✅` suffix on a head-and-shoulders chip means the neckline already broke (confirmed), versus a detected-but-unconfirmed pattern without it.

The four `Fundamentals` chips are **independently optional**. A card can show `FA: neutral (0.08) | P/E: value` with nothing else, or `FA: weak (-0.50) | FCF: fair | BS: 🟢 healthy` with no P/E chip at all. Each chip needs its own metric, and a missing chip means that one metric was not computed — not that the others are unreliable.

#### Three real cards, and what each one teaches

A **green** card — FA composite tier is `strong`:

```
🟩 ┃ 📉 TSM  $349.31  (-1.26%)
   ┃
   ┃ Technical
   ┃ RSI 47 | — up | 🔴 H&S
   ┃
   ┃ Fundamentals
   ┃ 🟢 FA: strong (0.58) | P/E: expensive | BS: 🟢 healthy
   ┃
   ┃ Latest News
   ┃ TSMC's Monthly Sales Surge on Sustained AI Chip Demand
   ┃
   ┃ Market ops
   ┃ VIX regime: normal · ATR% 3.32 · Vol×median 1.14 · atr_pct_elevated
   ┃
   ┃ VIX: ✅ normal
```

A **grey** card — FA composite tier is `neutral`, which is neither `strong` nor `weak`:

```
⬜ ┃ 📉 CVX  $148.14  (-1.13%)
   ┃
   ┃ Technical
   ┃ RSI 37 | — up
   ┃
   ┃ Fundamentals
   ┃ 🟡 FA: neutral (-0.33) | P/E: expensive | BS: 🔴 stressed
   ┃
   ┃ Latest News
   ┃ Chevron Cuts Permian Spending as Crude Prices Slide
   ┃
   ┃ Market ops
   ┃ VIX regime: normal · ATR% 2.84 · Vol×median 0.91 · atr_pct_elevated
   ┃
   ┃ VIX: ✅ normal
```

A **foreign listing** — same pipeline, but the fundamentals provider has no coverage, so the entire Fundamentals field is gone:

```
⬜ ┃ 📉 2222.SR  $27.44  (-0.36%)
   ┃
   ┃ Technical
   ┃ RSI 57 | — up | 🔴 Squeeze | 🔴 H&S
   ┃
   ┃ Latest News
   ┃ Aramco Weighs Asset Sales to Shore Up Dividend
   ┃
   ┃ Market ops
   ┃ VIX regime: normal · ATR% 1.08 · Vol×median 0.64
   ┃
   ┃ VIX: ✅ normal
```

What these three demonstrate:

- **Colour tracks fundamentals, the title arrow tracks price.** TSM is *down* 1.26% on the day yet carries a green strip, because green means `strong` FA composite — not a good day. A red strip means `weak` fundamentals, not a down day.
- **A red `BS:` chip can sit under a neutral composite.** CVX shows `FA: neutral` with `BS: 🔴 stressed`: the balance-sheet health tier is a separate metric from the composite, and only appears at all when it is `healthy` or `stressed`. A card with no `BS:` chip has a *neutral* balance sheet, not an unknown one.
- **`— up` and `— down` are a display gap, not missing data.** The trend direction is present and correct; the emoji lookup uses the keys `uptrend`/`downtrend` while the analyzer writes `up`/`down`, so only `sideways` ever resolves (to `↔️`). Read the word, ignore the dash.
- **Chips are omitted, not zeroed.** 2222.SR shows four technical chips, CVX two. A short row means a quiet chart, because each chip only exists when its condition is true.
- **A missing Fundamentals field is normal for anything that is not a covered stock.** 2222.SR is a foreign listing; crypto and most ETFs look the same way. See [Asset class changes the output](#asset-class-changes-the-output).
- **Flags are appended inline to the Market ops line**, after the three fixed metrics, separated by the same `·`. `atr_pct_elevated` on TSM and CVX is a volatility flag, not an error.

### What it looks like in the channel

The report does not arrive as one message. Embeds are packed 10-at-a-time (or until 6000 characters), so a 5-equity + 2-crypto run lands as two or three consecutive posts:

```
  ┌─ message 1 ──────────────────────────────────────┐
  │ 🟦 📊 Daily Market Report — 2026-09-11 07:00 UTC  │
  │ 🟩 🏦 Monetary Policy — 🟢 accommodative (+0.55)   │
  │ 🟨 📈 Growth Cycle — 🟡 Slowdown (+0.27)           │
  │ 🟥 🌡️ Inflation & Prices — 🔴 Hot (+0.55)          │
  │ 🟥 🌍 Global & Geopolitical — 🔴 Elevated stress   │
  │ 🟨 📉 Market cycle — Bull Fragile Global          │
  │ 🟦 🔗 Macro correlations — Stagflation Risk       │
  │ 🟪 📚 Additional analysis · intermarket           │
  │ 🟪 Macro intel · calendars · geo · headlines      │
  │ 🟩 📈 NVDA  $198.87  (+1.20%)                     │
  └──────────────────────────────────────────────────┘
  ┌─ message 2 ──────────────────────────────────────┐
  │ ⬜ 📈 GOOGL  $337.12  (+1.26%)                    │
  │ ⬜ 📉 SPY    $651.87  (-0.79%)                    │
  │ 🟥 📉 INTC   $24.18   (-2.10%)                    │
  │ 🟩 📈 MSFT   $498.02  (+0.42%)                    │
  │ ⬜ 📈 BTCUSDT $71,204.50  (+2.81%)                 │
  │ ⬜ 📉 ETHUSDT $2,418.90   (-1.02%)                 │
  └──────────────────────────────────────────────────┘
```

Read the left column top-to-bottom and you get the whole morning in one pass: macro regime as a colour gradient, then a green/grey/red verdict per holding. Order is always deterministic — macro panels first, equities in config order, then crypto — so the card you care about sits in the same place every day.

---

## 5. `#alerts`

### What triggers it

`AlertScanJob.run()` fires every `BOT_ALERT_SCAN_INTERVAL` seconds (default 300). It calls `ReportBuilder.scan_alerts()`, which loops **every configured equity symbol, then every crypto symbol**, loads that symbol's latest `technical_indicators` row set, and tests six conditions.

### Dedup is what makes this channel usable

Each potential alert computes a Redis key. If the key exists, the alert is **skipped entirely** — not posted, not even constructed. If it does not exist, the alert is emitted and the key is set with TTL `BOT_ALERT_COOLDOWN_SECS` (default 14400 = 4 hours).

| Alert kind | Redis cooldown key | Scope of the cooldown |
|---|---|---|
| `rsi_oversold` | `alert:rsi_oversold:{symbol}:{interval}` | per symbol |
| `rsi_overbought` | `alert:rsi_overbought:{symbol}:{interval}` | per symbol |
| `bb_squeeze` | `alert:bb_squeeze:{symbol}:{interval}` | per symbol |
| `vix_elevated` | `alert:vix_elevated:{interval}` | **global** — no symbol in the key |
| `fa_tier_flip` | `alert:fa_tier_flip:{symbol}` | per symbol, no interval |
| `liquidity_sweep` | `alert:liq_sweep:{symbol}:{interval}` | per symbol |

Two consequences worth knowing:

- **VIX is announced once, not once per symbol.** The key omits the symbol, so the first equity that trips the VIX check claims the cooldown for all of them. The posted embed names that arbitrary symbol even though the condition is market-wide — which is why the VIX action rule explicitly says *"this is a sizing alert — not a buy/sell signal for a specific equity."*
- **If Redis is down, `cache.exists()` returns `False`** (it logs and degrades), so every alert re-fires on every scan. A channel flooding every 5 minutes is the signature of a dead Redis, not a market event.

### The six alert conditions

| Kind | Condition | Severity → colour | Message text |
|---|---|---|---|
| `rsi_oversold` | `rsi_14 < BOT_RSI_OVERSOLD` (30) | `warning` → 🟨 | `RSI 27.4 — oversold (<30.0)` |
| `rsi_overbought` | `rsi_14 > BOT_RSI_OVERBOUGHT` (70) | `warning` → 🟨 | `RSI 74.1 — overbought (>70.0)` |
| `bb_squeeze` | `bb_squeeze` value ≥ 1.0 (Bollinger Bands inside Keltner Channels) | `info` → 🟦 | `Bollinger Squeeze active — low-volatility coil, breakout expected` |
| `vix_elevated` | `vix_regime` value > `BOT_VIX_ALERT_THRESHOLD` (25), equities only | `warning` → 🟨 | `VIX 27.3 — regime: elevated` |
| `fa_tier_flip` | Cached tier (`fa_tier:{symbol}`, 24 h TTL) differs from current **and** the new tier is `strong` or `weak` | `warning` if `weak`, `info` if `strong` | `FA composite tier changed: neutral → weak` |
| `liquidity_sweep` | `liquidity_sweep_sw*` count > 0 | `info` → 🟦 | `Liquidity sweep detected (3 sweeps)` |

`fa_tier_flip` needs a **prior observation** to compare against, so it can never fire on the very first scan after a Redis flush — the first pass only seeds `fa_tier:{symbol}`. Flips into or out of `neutral` are also filtered out: only arrivals at `strong` or `weak` alert.

The `critical` severity (red, 🚨) exists in the formatter but **no condition in `scan_alerts` ever emits it**, so in practice this channel is only yellow and blue.

### The embed

Every alert uses one shape: title `{severity emoji} Alert — {symbol}`, the message as description, then four inline fields. Because Discord packs **three inline fields per row**, `Value` always wraps onto a second row on its own:

```
🟦 ┃ ℹ️ Alert — CVX
   ┃
   ┃ Liquidity sweep detected (3 sweeps)
   ┃
   ┃ Type              Exchange          Interval
   ┃ liquidity_sweep   equity            1Day
   ┃
   ┃ Value
   ┃ 3.00
```

That wrap is Discord's layout engine, not a formatting choice — the bot adds all four as `inline=True`.

| Slot | Every possible value |
|---|---|
| **Severity emoji + colour** | `⚠️` yellow for `warning`, `ℹ️` blue for `info`, `🚨` red for `critical`. **`critical` is never emitted** by the scanner, so `#alerts` is only ever yellow and blue |
| **`Type`** | `rsi_oversold` · `rsi_overbought` · `vix_elevated` · `bb_squeeze` · `liquidity_sweep` · `fa_tier_flip` |
| **`Exchange`** | `equity` or `binance` |
| **`Interval`** | `1Day` for equities and `1d` for crypto — two different strings for the same daily timeframe, from `BOT_EQUITY_INTERVAL` and `BOT_CRYPTO_INTERVAL`. They always track the `Exchange` field |
| **`Value`** | the numeric that triggered the alert, meaning different things per type: the RSI level, the VIX level, the sweep count, `1.00` for a squeeze, and the composite score for an FA flip |
| **Description** | free text composed per alert kind; the RSI alerts are the only ones that restate their threshold |

There is no fifth field and no footer — this embed is deliberately the simplest in the bot, because the interpretation lives in `#actions`.

Two small things that look like inconsistencies:

- **The description and the `Value` field disagree on precision.** An RSI alert reads `RSI 81.9 — overbought (>70.0)` in the description but `81.85` in `Value`. The description formats to 1 decimal and the field to 2; they are the same number. The threshold in the parentheses is also formatted to 1 decimal, so an integer setting of `70` prints as `70.0`.
- **A VIX above the threshold does not guarantee a `vix_elevated` alert in that scan.** With `BOT_VIX_ALERT_THRESHOLD=25` and a VIX of 25.8 you may still see a scan containing only sweeps and squeezes. That is the 4-hour cooldown working — the VIX alert fired on an earlier scan and its Redis key has not expired. Because its cooldown key omits the symbol, one arbitrary equity holds the alert for the whole window.

Conversely, if you ever see the **entire alert block repeat verbatim** a few minutes apart — same symbols, same sweep counts, same values — that is the signature of Redis being unreachable. `cache.exists()` degrades to `False`, every alert looks new, and the channel refloods on each scan.

### The description text, exhaustively

There are exactly six description templates. Everything that ever appears in the description of an alert embed is one of these, with the bracketed parts substituted:

| Type | Description template | `Value` holds | Severity |
|---|---|---|---|
| `rsi_oversold` | `RSI {rsi:.1f} — oversold (<{threshold})` | the RSI reading | `warning` 🟨 |
| `rsi_overbought` | `RSI {rsi:.1f} — overbought (>{threshold})` | the RSI reading | `warning` 🟨 |
| `bb_squeeze` | `Bollinger Squeeze active — low-volatility coil, breakout expected` — **fixed text, no substitution** | always `1.00` | `info` 🟦 |
| `vix_elevated` | `VIX {vix:.1f} — regime: {regime}` | the VIX level | `info` 🟦 |
| `fa_tier_flip` | `FA composite tier changed: {prev_tier} → {tier}` | the new composite score | `info` 🟦 |
| `liquidity_sweep` | `Liquidity sweep detected ({n} sweeps)` | the sweep count | `info` 🟦 |

The `{regime}` in a VIX alert draws from the technical-analysis vocabulary — `extreme_fear`, `elevated`, `normal`, `complacency` — while `{prev_tier} → {tier}` on an FA flip draws from `strong`, `neutral`, `weak`. Since only `strong` and `weak` are actionable destinations, a flip into `neutral` posts an alert but produces a suppressed `WATCH` in `#actions`.

Which of the six can fire depends on the asset:

| Type | Stocks | ETFs | Crypto |
|---|---|---|---|
| `rsi_oversold` / `rsi_overbought` | yes | yes | yes |
| `bb_squeeze` | yes | yes | yes |
| `liquidity_sweep` | yes | yes | yes |
| `vix_elevated` | **equity scan only** — and one arbitrary symbol claims it | same | never |
| `fa_tier_flip` | yes | never (no composite) | never |

### All six alerts, rendered

**`rsi_oversold`** and **`rsi_overbought`** — yellow, and the only alerts whose description restates the threshold that was crossed:

```
🟨 ┃ ⚠️ Alert — INTC
   ┃
   ┃ RSI 27.4 — oversold (<30.0)
   ┃
   ┃ Type              Exchange          Interval
   ┃ rsi_oversold      equity            1Day
   ┃
   ┃ Value
   ┃ 27.40
```

```
🟨 ┃ ⚠️ Alert — NVDA
   ┃
   ┃ RSI 74.1 — overbought (>70.0)
   ┃
   ┃ Type              Exchange          Interval
   ┃ rsi_overbought    equity            1Day
   ┃
   ┃ Value
   ┃ 74.10
```

**`bb_squeeze`** — blue/info. `Value` is always `1.00`: it is a boolean flag stored as a float, not a measurement of squeeze tightness.

```
🟦 ┃ ℹ️ Alert — SHEL
   ┃
   ┃ Bollinger Squeeze active — low-volatility coil, breakout expected
   ┃
   ┃ Type              Exchange          Interval
   ┃ bb_squeeze        equity            1Day
   ┃
   ┃ Value
   ┃ 1.00
```

**`vix_elevated`** — yellow. Note the symbol in the title: the condition is market-wide, but the embed is stamped with whichever equity happened to trip the check first, because the cooldown key omits the symbol. Treat the ticker here as incidental.

```
🟨 ┃ ⚠️ Alert — AAPL
   ┃
   ┃ VIX 27.3 — regime: elevated
   ┃
   ┃ Type              Exchange          Interval
   ┃ vix_elevated      equity            1Day
   ┃
   ┃ Value
   ┃ 27.30
```

**`fa_tier_flip`** — the **only alert with no `Value` field**, because a tier change has no scalar. Severity flips with direction: yellow into `weak`, blue into `strong`.

```
🟨 ┃ ⚠️ Alert — INTC
   ┃
   ┃ FA composite tier changed: neutral → weak
   ┃
   ┃ Type              Exchange          Interval
   ┃ fa_tier_flip      equity            1Day
```

```
🟦 ┃ ℹ️ Alert — MSFT
   ┃
   ┃ FA composite tier changed: neutral → strong
   ┃
   ┃ Type              Exchange          Interval
   ┃ fa_tier_flip      equity            1Day
```

**`liquidity_sweep`** — blue/info, `Value` carries the sweep count. Notably the description does **not** say which direction was swept; that only appears downstream in the `#actions` reasoning or via `/alert`.

```
🟦 ┃ ℹ️ Alert — SHEL
   ┃
   ┃ Liquidity sweep detected (3 sweeps)
   ┃
   ┃ Type              Exchange          Interval
   ┃ liquidity_sweep   equity            1Day
   ┃
   ┃ Value
   ┃ 3.00
```

### What a scan looks like in the channel

Alerts post one message per alert, in symbol order, all within the same second — so a scan that trips several conditions reads as a burst. Crypto alerts are distinguishable by `Exchange: binance`:

```
  9:25 PM  🟦 ℹ️ Alert — CVX     Liquidity sweep detected (3 sweeps)
  9:25 PM  🟦 ℹ️ Alert — SHEL    Bollinger Squeeze active — low-volatility coil…
  9:25 PM  🟦 ℹ️ Alert — SHEL    Liquidity sweep detected (3 sweeps)
  9:25 PM  🟨 ⚠️ Alert — INTC    RSI 27.4 — oversold (<30.0)
  9:25 PM  🟦 ℹ️ Alert — BTCUSDT Liquidity sweep detected (2 sweeps)
```

The same symbol can appear twice in one burst (SHEL above) because each condition holds its own cooldown key. Then the channel goes quiet: every key is now set for `BOT_ALERT_COOLDOWN_SECS`, so the next scan four minutes later posts nothing even though the indicators are unchanged. **Blue dominates in practice** — three of the six conditions are `info` severity, and `critical` red is never emitted at all.

---

## 6. `#actions`

`#alerts` tells you *what happened*. `#actions` tells you *what it might mean*, by cross-referencing the alert against trend, fundamentals, and the VIX regime.

### The process

Inside the same scan iteration, right after `notifier.send_alert()` succeeds, `AlertScanJob` calls `actions.engine.process_alert()` — but only if `BOT_ACTIONS_ENABLED=true` (default), a DB pool exists, and `DISCORD_ACTIONS_CHANNEL_ID` is set. The engine then:

1. Looks up the alert kind in `RULE_MAP`. Unmapped kinds return immediately (all six current kinds are mapped).
2. Reloads `technical_indicators` and derived `equity_fundamentals` for the symbol.
3. Reads `VIXCLS` from `macro_fred` and classifies it with **fixed thresholds**: `> 35` extreme_fear, `> 20` elevated, `< 12` complacency, else normal. If that read fails it falls back to the TA `vix_regime` payload.
4. For `fa_tier_flip`, injects the previous tier from the alert payload as `fa["_prev_tier"]`.
5. Calls the rule, which returns `(action, reasons, confluence)`.
6. **Suppression gate:** if the action is `WATCH` or `HOLD_WATCH`, the engine logs and returns. Nothing is posted.
7. Otherwise the formatter builds the embed and the notifier sends it.

So `#actions` is strictly quieter than `#alerts`: it only ever shows directed guidance.

### The rules and every action they can produce

| Alert kind | Action produced | Condition inside the rule | Confluence |
|---|---|---|---|
| `rsi_oversold` | `BUY_WATCH` 🟢 | confluence ≥ 2 | counted |
| | `WATCH` ⚪ | otherwise → **suppressed** | |
| `rsi_overbought` | `TRIM_WATCH` 🔴 | confluence ≥ 2 | counted |
| | `HOLD_WATCH` ⚪ | otherwise → **suppressed** | |
| `bb_squeeze` | `PREPARE_LONG` 🔵 | trend `up` **and** MACD bullish cross | counted |
| | `WATCH` ⚪ | otherwise → **suppressed** | |
| `vix_elevated` | `REDUCE_SIZE_WATCH_REVERSAL` 🟠 | regime is `extreme_fear` | hardcoded 2 |
| | `REDUCE_SIZE` 🟠 | any other elevated regime | hardcoded 2 |
| `fa_tier_flip` | `REVIEW_POSITION` 🟡 | new tier `weak` | hardcoded 1 |
| | `BUY_WATCH` 🟢 | new tier `strong` **and** trend `up` | hardcoded 1 |
| | `WATCH_ACCUMULATE` 🔵 | new tier `strong`, trend not up | hardcoded 1 |
| | `WATCH` ⚪ | any other tier → **suppressed** | |
| `liquidity_sweep` | `BUY_WATCH` 🟢 | low sweep, closed back **above** the swept level, and not (risk-off + non-uptrend) | starts at 1, +1 per confirmation |
| | `TRIM_WATCH` 🔴 | high sweep, closed back **below** the swept level | |
| | `WATCH` ⚪ | no close-back confirmation, undetermined direction, or risk-off downtrend → **suppressed** | |

`vix_elevated` is the only rule that **always** posts — its confluence is hardcoded to 2 on the reasoning that elevated volatility unconditionally warrants caution.

### Which actions you will realistically see

Several confluence checks reference indicator keys that `technical-analysis` does not actually write under those names, so they never contribute:

| Rule check | Key it looks up | Key actually written | Fires? |
|---|---|---|---|
| MACD cross / histogram | `macd` → payload `bullish_cross` | `macd_12_26_9` → payload `bullish_cross_line_signal` | ❌ never |
| RSI divergence | `rsi_divergence` → payload `divergence_type` = `bullish`/`bearish` | `rsi_divergence_rsi14_sw{n}` → payload `kind` = `bullish_regular`/`bearish_regular`/`none` | ❌ never — the name, the key, **and** the values all differ |
| Support/resistance proximity | `support_resistance` | `sr_levels` | ❌ never |
| Trend direction | `trend` → payload `direction` | `trend` → `direction` (`up`/`down`/`sideways`) | ✅ |
| Order blocks | prefix `order_block` | `order_blocks_sw{n}_imp{pct}` | ✅ |
| Liquidity sweeps | prefix `liquidity_sweep` | `liquidity_sweep_sw{n}` | ✅ |
| FA composite tier | `composite_score` → payload `tier` | same | ✅ (equities only) |

Practical effects:

- **`PREPARE_LONG` is currently unreachable.** It requires a MACD bullish cross, so `bb_squeeze` alerts always resolve to `WATCH` and never reach the channel.
- **RSI actions need two live signals.** Only trend and FA tier can contribute, so `BUY_WATCH` from `rsi_oversold` requires trend `up` **and** FA tier `strong`; `TRIM_WATCH` requires trend `down` **and** FA tier `weak`.
- **Crypto and ETF RSI alerts never produce an action.** With only trend and FA tier able to contribute, and no FA composite for these assets, confluence caps at 1 — below the minimum of 2. Their `liquidity_sweep` alerts *do* produce actions, because that rule scores off SMC structure and never consults fundamentals.
- **Liquidity-sweep actions are not trend-gated the way you would expect.** A `BUY_WATCH` can carry `Trend: DOWN` and a `TRIM_WATCH` can carry `Trend: UP`; see [the crypto example](#the-context-strip-drops-fa-for-anything-without-fundamentals).
- **`fa_tier_flip` posts with a "WATCH" title.** Its confluence is hardcoded to 1, below the default `BOT_ACTIONS_MIN_CONFLUENCE=2`, so the embed renders as `ℹ️ WATCH — SYMBOL` even though the action label is `REVIEW_POSITION` or `WATCH_ACCUMULATE`. The card still posts, because the suppression gate checks the *action label*, while the title checks the *confluence*.

Lowering `BOT_ACTIONS_MIN_CONFLUENCE` to 1 makes `fa_tier_flip` cards render as `⚡ ACTION`; it does not change which cards post.

### The embed

```
🟩 ┃ ⚡ ACTION — TTE
   ┃
   ┃ Alert: liquidity_sweep │ Action: 🟢 BUY_WATCH
   ┃
   ┃ Confluence Score
   ┃ 4/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ 📍 Low sweep (2 recent): stop-hunt below swing low detected
   ┃ ✅ Closed back above swept level — institutional accumulation pattern
   ┃ ✅ Bullish order block nearby — strong support confluence
   ┃ ✅ Uptrend intact — sweep aligns with trend continuation
   ┃ ℹ️ SMC sweep signals are most reliable on higher timeframes with FVG or
   ┃    OB confluence
   ┃
   ┃ 📋 Next step
   ┃ Run /analyze TTE for full analysis before opening a position.
   ┃
   ┃ Context
   ┃ RSI 55.9 │ Trend: UP │ FA: neutral │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

This is the maximum-confidence case: `4/4`, from a base of 1 plus close-back-above, a nearby bullish order block, and an intact uptrend. Note `FA: neutral` in the Context strip — the liquidity-sweep rule never consults fundamentals, so a `BUY_WATCH` here says nothing about the company's financials. The Context strip is informational only; it does not feed the score.

Anatomy:

- **Title** — `⚡ ACTION` when `confluence >= min_confluence` and the label is not a watch label, otherwise `ℹ️ WATCH`.
- **Colour** — per action label: green `BUY_WATCH`, red `TRIM_WATCH`, blue `PREPARE_LONG`/`WATCH_ACCUMULATE`, orange `REDUCE_SIZE*`, yellow `REVIEW_POSITION`, grey watch labels.
- **Confluence Score** — rendered as `{confluence}/{min_confluence + 2}`, so the default denominator is `4`. The denominator is a display convention, not a real maximum.
- **Reasoning** — the rule's ordered list. `✅` supports the action, `⚠️` is a caveat or bearish note, `ℹ️` is context, `📍` marks a detected SMC event, `👀` and `💡` appear in the extreme-fear VIX playbook.
- **📋 Next step** — a fixed per-label instruction with `{symbol}` substituted.
- **Context** — a pipe-separated strip of RSI, trend, FA tier, and VIX regime, each included only if available.
- **Footer** — the staleness reminder, always identical.

#### Every slot and its value domain

| Slot | Every possible value |
|---|---|
| **Title** | `⚡ ACTION — SYMBOL` or `ℹ️ WATCH — SYMBOL` |
| **`Alert:`** | `rsi_oversold` · `rsi_overbought` · `vix_elevated` · `bb_squeeze` · `liquidity_sweep` · `fa_tier_flip` — the six alert kinds that have rules |
| **`Action:`** | 🟢 `BUY_WATCH` · 🔴 `TRIM_WATCH` · 🔵 `PREPARE_LONG` · 🔵 `WATCH_ACCUMULATE` · 🟠 `REDUCE_SIZE` · 🟠 `REDUCE_SIZE_HEDGE` · 🟡 `REVIEW_POSITION` · ⚪ `WATCH` · ⚪ `HOLD_WATCH`. The last two are **filtered out before posting**, so you will never see them in `#actions` |
| **Confluence** | `0/4` through roughly `5/4`. The denominator is `BOT_ACTIONS_MIN_CONFLUENCE + 2`, a display convention rather than a ceiling — a score can exceed it |
| **Reasoning prefix** | `✅` supports · `⚠️` caveat or bearish · `ℹ️` context · `📍` detected SMC event · `👀` and `💡` in the extreme-fear VIX playbook |
| **Context › RSI** | a float, or the segment is dropped |
| **Context › Trend** | `UP` · `DOWN` · `SIDEWAYS` — uppercased here, unlike everywhere else in the bot |
| **Context › FA** | `strong` · `neutral` · `weak`, **or the whole segment is absent** |
| **Context › VIX** | `extreme_fear` · `elevated` · `normal` · `complacency` |
| **Colour** | tracks the action label, not the outcome |

#### The Context strip drops `FA:` for anything without fundamentals

Crypto pairs and ETFs have no FA composite, so their strip is three segments rather than four:

```
🟩 ┃ ⚡ ACTION — KSMUSDT
   ┃
   ┃ Alert: liquidity_sweep │ Action: 🟢 BUY_WATCH
   ┃
   ┃ Confluence Score
   ┃ 3/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ 📍 Low sweep (3 recent): stop-hunt below swing low detected
   ┃ ✅ Closed back above swept level — institutional accumulation pattern
   ┃ ✅ Bullish order block nearby — strong support confluence
   ┃ ℹ️ SMC sweep signals are most reliable on higher timeframes with FVG or
   ┃    OB confluence
   ┃
   ┃ 📋 Next step
   ┃ Run /analyze KSMUSDT for full analysis before opening a position.
   ┃
   ┃ Context
   ┃ RSI 60.1 │ Trend: DOWN │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

Two things this card settles:

- **A `BUY_WATCH` can carry `Trend: DOWN`.** The liquidity-sweep rule downgrades to a plain `WATCH` only when the market is *risk-off* (VIX elevated or worse) **and** the trend is not up. With VIX `normal`, a down-trending symbol that swept a low and closed back above it still earns `BUY_WATCH`. The mirror holds: a `TRIM_WATCH` can carry `Trend: UP`, since the high-sweep branch never checks trend at all.
- **The missing fourth confluence point is the trend line.** This card scores 3 — base 1, close-back-above, bullish order block — where an up-trending symbol would score 4. Crypto is not penalised for lacking fundamentals here, because this rule never consults them.

ETFs behave identically. `XLF`, `EEM` and `SPY` all produce sweep actions with a three-segment Context strip, for the same reason: no FA composite exists to print.

### Every action label, rendered

**`BUY_WATCH` from `rsi_oversold`** 🟩 — needs both live confluence signals, so the reasoning is always exactly these two plus the RSI line:

```
🟩 ┃ ⚡ ACTION — MSFT
   ┃
   ┃ Alert: rsi_oversold │ Action: 🟢 BUY_WATCH
   ┃
   ┃ Confluence Score
   ┃ 2/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ RSI 27.4
   ┃ ✅ Uptrend intact — pullback within trend
   ┃ ✅ FA composite: strong fundamentals
   ┃
   ┃ 📋 Next step
   ┃ Run /analyze MSFT for full analysis before opening a position.
   ┃
   ┃ Context
   ┃ RSI 27.4 │ Trend: UP │ FA: strong │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

If the VIX regime is elevated or worse, a fourth reasoning line appends — `⚠️ VIX regime: elevated — wider noise band, tighter sizing` — which does **not** change the score, only the advice.

**`TRIM_WATCH` from `rsi_overbought`** 🟥 — the mirror case, requiring trend `down` and FA tier `weak`:

```
🟥 ┃ ⚡ ACTION — INTC
   ┃
   ┃ Alert: rsi_overbought │ Action: 🔴 TRIM_WATCH
   ┃
   ┃ Confluence Score
   ┃ 2/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ RSI 74.1
   ┃ ⚠️ Downtrend active — overbought in weak trend
   ┃ ⚠️ FA composite: weak — confirms bearish tilt
   ┃
   ┃ 📋 Next step
   ┃ Consider reducing or closing position. Run /analyze INTC to confirm.
   ┃
   ┃ Context
   ┃ RSI 74.1 │ Trend: DOWN │ FA: weak │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

**`TRIM_WATCH` from `liquidity_sweep`** 🟥 — a high sweep that closed back below the swept level:

```
🟥 ┃ ⚡ ACTION — CVX
   ┃
   ┃ Alert: liquidity_sweep │ Action: 🔴 TRIM_WATCH
   ┃
   ┃ Confluence Score
   ┃ 3/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ 📍 High sweep (3 recent): stop-hunt above swing high detected
   ┃ ⚠️ Closed back below swept level — possible distribution / fakeout
   ┃ ⚠️ Bearish order block overhead — resistance confluence
   ┃ ℹ️ SMC sweep signals are most reliable on higher timeframes with FVG or
   ┃    OB confluence
   ┃
   ┃ 📋 Next step
   ┃ Consider reducing or closing position. Run /analyze CVX to confirm.
   ┃
   ┃ Context
   ┃ RSI 61.2 │ Trend: DOWN │ FA: neutral │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

**`REDUCE_SIZE` from `vix_elevated`** 🟠 — the sizing alert. Confluence is hardcoded to 2, so this always posts as `⚡ ACTION`. It names a symbol but explicitly disclaims it in the last reasoning line:

```
🟧 ┃ ⚡ ACTION — AAPL
   ┃
   ┃ Alert: vix_elevated │ Action: 🟠 REDUCE_SIZE
   ┃
   ┃ Confluence Score
   ┃ 2/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ 🟡 VIX 27.3 elevated — tighten stops, reduce new position sizes
   ┃ ⚠️ Do not open new trades without strong multi-signal confirmation
   ┃ ℹ️ This is a sizing alert — not a buy/sell signal for a specific equity
   ┃
   ┃ 📋 Next step
   ┃ Reduce all new position sizes. Do not chase moves in this VIX regime.
   ┃
   ┃ Context
   ┃ RSI 48.3 │ Trend: DOWN │ FA: neutral │ VIX: elevated
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

**`REDUCE_SIZE_WATCH_REVERSAL` from `vix_elevated`** 🟠 — same alert, but the engine classified VIX above 35 as `extreme_fear`. The reasoning switches to a contrarian playbook:

```
🟧 ┃ ⚡ ACTION — AAPL
   ┃
   ┃ Alert: vix_elevated │ Action: 🟠 REDUCE_SIZE_WATCH_REVERSAL
   ┃
   ┃ Confluence Score
   ┃ 2/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ 🔴 VIX extreme fear — reduce all position sizes by 50%
   ┃ 👀 Historically: extreme fear = opportunity for quality stocks
   ┃ 💡 Hold cash — do not buy all at once; scale in over multiple sessions
   ┃ ℹ️ This is a sizing alert — not a buy/sell signal for a specific equity
   ┃
   ┃ 📋 Next step
   ┃ Reduce all positions. Watch for quality-stock reversal opportunities on dips.
   ┃
   ┃ Context
   ┃ RSI 31.8 │ Trend: DOWN │ FA: strong │ VIX: extreme_fear
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

**`REVIEW_POSITION` from `fa_tier_flip`** 🟡 — the confluence-title mismatch in the wild. The card posts because `REVIEW_POSITION` is not a watch label, but its hardcoded confluence of 1 is below the default minimum of 2, so the title reads `ℹ️ WATCH` and the score says `watch-only`:

```
🟨 ┃ ℹ️ WATCH — INTC
   ┃
   ┃ Alert: fa_tier_flip │ Action: 🟡 REVIEW_POSITION
   ┃
   ┃ Confluence Score
   ┃ 1/4 (watch-only)
   ┃
   ┃ Reasoning
   ┃ 🔴 FA downgrade: neutral → weak
   ┃ ⚠️ If holding a position: review and consider reducing exposure
   ┃ ⚠️ Downtrend active — downgrade + bearish trend double warning
   ┃ ⏱️ FA data refreshes every 24h — medium-term context only, not intraday
   ┃ ℹ️ Run /analyze to see which component changed
   ┃
   ┃ 📋 Next step
   ┃ Review any existing position in INTC. FA score degraded — reassess thesis.
   ┃
   ┃ Context
   ┃ RSI 38.2 │ Trend: DOWN │ FA: weak │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

Set `BOT_ACTIONS_MIN_CONFLUENCE=1` and this exact card becomes `⚡ ACTION — INTC` with `1/3 (actionable)`. Nothing else changes.

**`WATCH_ACCUMULATE` from `fa_tier_flip`** 🟦 — an upgrade to `strong` without trend confirmation. Same title caveat:

```
🟦 ┃ ℹ️ WATCH — MSFT
   ┃
   ┃ Alert: fa_tier_flip │ Action: 🔵 WATCH_ACCUMULATE
   ┃
   ┃ Confluence Score
   ┃ 1/4 (watch-only)
   ┃
   ┃ Reasoning
   ┃ ✅ FA upgrade: neutral → strong
   ┃ ℹ️ Wait for trend confirmation before adding exposure
   ┃ ⏱️ FA data refreshes every 24h — medium-term context only, not intraday
   ┃ ℹ️ Run /analyze to see which component changed
   ┃
   ┃ 📋 Next step
   ┃ FA improved. Watch for a low-risk entry on the next pullback.
   ┃
   ┃ Context
   ┃ RSI 52.1 │ Trend: SIDEWAYS │ FA: strong │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

If the trend *is* `up`, this same alert instead yields `BUY_WATCH` with the extra line `✅ Uptrend confirmed — trend + FA upgrade combined bullish signal`.

**`PREPARE_LONG` from `bb_squeeze`** 🟦 — **what you would see if the MACD key mismatch were fixed.** It cannot currently appear, because the rule requires a MACD bullish cross it can never read:

```
🟦 ┃ ⚡ ACTION — SHEL          ← unreachable today
   ┃
   ┃ Alert: bb_squeeze │ Action: 🔵 PREPARE_LONG
   ┃
   ┃ Confluence Score
   ┃ 3/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ ✅ Uptrend active — squeeze likely bull breakout
   ┃ ✅ ADX 28.4 — strong trend behind the squeeze
   ┃ ✅ MACD bullish cross — momentum turning up
   ┃ ℹ️ Wait for a confirmed breakout candle before entering
   ┃
   ┃ 📋 Next step
   ┃ Wait for a confirmed breakout candle with volume before entering long.
   ┃
   ┃ Context
   ┃ RSI 58.0 │ Trend: UP │ FA: neutral │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min
```

In reality every `bb_squeeze` alert resolves to `WATCH` and is suppressed — which is why `#alerts` shows squeeze cards with no `#actions` counterpart.

### What it looks like in the channel

`#actions` is intentionally sparser than `#alerts`. A scan that posted five alerts typically posts one or two actions, and the colour strip alone tells you the direction:

```
  9:25 PM  🟩 ⚡ ACTION — TTE    liquidity_sweep │ 🟢 BUY_WATCH      4/4
  9:25 PM  🟥 ⚡ ACTION — CVX    liquidity_sweep │ 🔴 TRIM_WATCH     3/4
  9:25 PM  🟨 ℹ️ WATCH  — INTC   fa_tier_flip    │ 🟡 REVIEW_POSITION 1/4
           (SHEL bb_squeeze and BTCUSDT liquidity_sweep were suppressed)
```

Green means a possible entry, red a possible exit, orange a portfolio-wide sizing instruction, and yellow or blue a position review with no urgency. Nothing here is an order — every `Next step` routes you to `/analyze` first.

---

## 7. `#commands`

Ten slash commands across two cogs: [`bot/cogs/commands.py`](services/analyst-bot/bot/cogs/commands.py) (data) and [`bot/cogs/admin.py`](services/analyst-bot/bot/cogs/admin.py) (ops). They work in any channel the bot can see; `DISCORD_COMMANDS_CHANNEL_ID` is convention only.

### Behaviour common to all of them

- **Deferral.** Every data command calls `ctx.defer()` first, so Discord shows *"trading-analyst-bot is thinking…"* while the SQL runs. Without this, queries slower than 3 seconds would time out the interaction.
- **Caching.** `/price`, `/signals`, and `/analyze` all call `build_symbol_report(use_cache=True)`, which shares **one** Redis key: `analyze:{symbol}:{asset_type}`, TTL `BOT_CACHE_ANALYZE_TTL` (default 600 s). So a `/price AAPL` within 10 minutes of an `/analyze AAPL` is served from that cache. `BOT_CACHE_PRICE_TTL` is loaded into config and passed to the builder but never read — there is no separate price cache. `/report`, `/marketops`, `/alert`, and `/action` always bypass the cache.
- **Symbol handling.** Symbols are upper-cased and trimmed. `/alert`, `/action`, and `/marketops` auto-switch `asset_type` to `crypto` when the symbol is in `BOT_CRYPTO_SYMBOLS`; `/marketops` additionally infers crypto from a `USDT`/`USDC`/`BUSD`/`PERP` suffix. `/price`, `/signals`, and `/analyze` do **not** auto-detect — pass `asset_type: crypto` explicitly or you will get "no data".
- **Errors.** Any uncaught exception is intercepted by `on_application_command_error` and replied as an **ephemeral** `⚠️ An error occurred: \`<error>\``, visible only to you.

In chat, every data command therefore passes through three states:

```
  ┌─ 1. you invoke ──────────────────────────────────┐
  │ you  used /analyze                               │
  │      symbol: AAPL   asset_type: equity           │
  └──────────────────────────────────────────────────┘
  ┌─ 2. while the SQL runs (ctx.defer) ──────────────┐
  │ 🤖 trading-analyst-bot is thinking…              │
  └──────────────────────────────────────────────────┘
  ┌─ 3. the reply replaces it ───────────────────────┐
  │ 🟩 📈 AAPL  $189.45  (+1.24%)                     │
  │ 🟦 📐 Context vs SPY                              │
  │ …                                                │
  └──────────────────────────────────────────────────┘
```

Two commands are **ephemeral** — only the invoker sees them, and they vanish on reload, marked *Only you can see this*:

```
  ┌─ /status ────────────────────────────────────────┐
  │ 🟦 🤖 Bot Status                                  │
  │ …                                                │
  │ 🔒 Only you can see this message                 │
  └──────────────────────────────────────────────────┘
```

`/status` is ephemeral by design, and so is every error reply — which means a failing command leaves **no trace in the channel** for anyone else:

```
  ┌─ any command that raises ────────────────────────┐
  │ ⚠️ An error occurred: `connection was closed`     │
  │ 🔒 Only you can see this message                 │
  └──────────────────────────────────────────────────┘
```

Everything else (`/price`, `/signals`, `/analyze`, `/marketops`, `/report`, `/alert`, `/action`, `/dictionary`, `/ping`) is public, so running `/report` in a busy channel posts the full macro stack for everyone.

#### What each command gives you per asset class

The `asset_type` option offers only **`equity`** and **`crypto`**. There is no ETF value, and none is needed — ETFs live in the equity tables and must be queried as `equity`. The differences below are all downstream of which analysis tables have rows, not of any branch on asset class.

| Command | Stock | ETF | Crypto |
|---|---|---|---|
| `/price` | identical | identical | identical, except `Interval` is `1d` and `Source` is `binance` |
| `/signals` | up to 7 fields | up to 6 — no FA Composite | up to 6 — no FA Composite, **VIX still shown** |
| `/analyze` | up to 10 panels | 5–6: price, context, market ops, technical, sentiment, plus an all-`⚪` fundamentals panel if placeholder rows exist | 5: no fundamentals panel of any kind |
| `/marketops` | full | full | same, but the VIX field is relabelled *"US equities index — crypto risk backdrop"* |
| `/report` | `equity`, `standard`, `all` modes | `etfs`, `commodities` modes | `crypto` mode |
| `/alert` | all six kinds reachable | five — no `fa_tier_flip` | four — no `fa_tier_flip`, no `vix_elevated` |
| `/action` | all labels reachable except `PREPARE_LONG` | sweep actions only | sweep actions only |
| `/dictionary`, `/status`, `/ping` | asset-agnostic | asset-agnostic | asset-agnostic |

A foreign listing (`2222.SR`, `OXIG.L`) is queried as `equity` and behaves like a stock whose fundamentals provider has no coverage: full price and technical output, no fundamentals, no FA-driven actions. Whether it gets fundamentals is a data-coverage question, not a code path — see [asset class changes the output](#asset-class-changes-the-output).

### `/price <symbol> [asset_type]`

**Process:** `build_symbol_report` (cached) → reads the latest bar from `equity_ohlcv` or `crypto_ohlcv` at `BOT_EQUITY_INTERVAL`/`BOT_CRYPTO_INTERVAL`, plus the two most recent bars to compute `change_pct` against the prior close.

**Outcomes:**

| | Rendering |
|---|---|
| Success | One embed, **green if change ≥ 0, red if negative**. Title `📈 AAPL  $189.45  (+1.24%)`. Six inline fields: Open, High, Low, Volume (thousands-separated), Interval, Source. Footer `Bar timestamp: 2026-09-10 00:00 UTC` |
| No bar | `❌ No price data found for **AAPL**. Is the symbol correct and data-ingestion running?` |

Six inline fields means **exactly two rows of three** — the only embed in the bot that fills its rows evenly:

```
🟩 ┃ 📈 AAPL  $189.45  (+1.24%)
   ┃
   ┃ Open              High              Low
   ┃ $187.20           $190.10           $186.85
   ┃
   ┃ Volume            Interval          Source
   ┃ 54,218,900        1Day              alpaca
   ┃
   ┃ Bar timestamp: 2026-09-10 00:00 UTC
```

A down day flips both the title arrow and the strip to red:

```
🟥 ┃ 📉 INTC  $24.18  (-2.10%)
   ┃
   ┃ Open              High              Low
   ┃ $24.70           $24.81            $24.05
   ┃
   ┃ Volume            Interval          Source
   ┃ 38,441,200        1Day              alpaca
   ┃
   ┃ Bar timestamp: 2026-09-10 00:00 UTC
```

Crypto keeps precision on low-priced pairs, because prices format adaptively — `$1,234.56` above 1000, `$12.34` above 1, `$0.000123` below:

```
🟩 ┃ 📈 BTCUSDT  $71,204.50  (+2.81%)
   ┃
   ┃ Open              High              Low
   ┃ $69,260.10        $71,540.00        $69,102.30
   ┃
   ┃ Volume            Interval          Source
   ┃ 18,204            1d                binance
   ┃
   ┃ Bar timestamp: 2026-09-10 00:00 UTC
```

The failure case is plain text, not an embed:

```
❌ No price data found for **AAPL**. Is the symbol correct and data-ingestion running?
```

### `/signals <symbol> [asset_type]`

**Process:** the same cached `SymbolReport`, but the handler builds a **single** embed inline rather than calling the multi-panel formatter. This is the fast one-glance command.

**Outcomes:**

| | Rendering |
|---|---|
| Success | One blue embed `⚡ Signals — AAPL` with up to 7 inline fields, each added only if present: Price, RSI 14 (suffixed `🔴 OS` below 30 or `🔴 OB` above 70), Trend, VIX, BB Squeeze (only when active), MA Cross (`🟢 Golden` or `🔴 Death`, only when one occurred), FA Composite |
| Embed would be empty | `❌ No signal data found for **AAPL**.` |

A quiet chart yields five fields — two rows, the second partly empty:

```
🟦 ┃ ⚡ Signals — AAPL
   ┃
   ┃ Price             RSI 14            Trend
   ┃ $189.45 (+1.24%)  54.2              — up
   ┃
   ┃ VIX               FA Composite
   ┃ ✅ normal         🟢 strong (0.62)
```

An eventful one fills all seven, and the conditional fields appear:

```
🟦 ┃ ⚡ Signals — INTC
   ┃
   ┃ Price             RSI 14            Trend
   ┃ $24.18 (-2.10%)   27.4 🔴 OS        — down
   ┃
   ┃ VIX               BB Squeeze        MA Cross
   ┃ ⚠️ elevated       🔴 ACTIVE         🔴 Death
   ┃
   ┃ FA Composite
   ┃ 🔴 weak (-0.48)
```

Two asymmetries versus `/price` worth knowing:

- **The strip is always blue**, whatever the price did. This embed summarises *state*, not a move, so colour carries no directional meaning here — read the field emoji instead.
- **BB Squeeze and MA Cross only ever appear when something happened.** There is no "inactive" rendering: no squeeze field means no squeeze. Compare `/analyze`, which shows `BB Squeeze —` explicitly.

| Field | Every possible value | Present for |
|---|---|---|
| **Price** | `$` + close in the magnitude-based format, plus `(±n%)` when a prior bar exists | any asset with OHLCV |
| **RSI 14** | 1 decimal, suffixed ` 🔴 OS` below 30 or ` 🔴 OB` above 70, bare in between | any asset |
| **Trend** | `— up` · `— down` · `↔️ sideways` | any asset |
| **VIX** | `😱 extreme_fear` · `⚠️ elevated` · `✅ normal` · `💤 complacency` | **any asset, including crypto** |
| **BB Squeeze** | `🔴 ACTIVE` only | any asset, only while squeezing |
| **MA Cross** | `🟢 Golden` or `🔴 Death` | any asset, only on the crossing bar |
| **FA Composite** | `🟢 strong` · `🟡 neutral` · `🔴 weak`, with the score | covered stocks only |

Crypto returns the same embed minus only the FA Composite field — **the VIX field still appears**, because this command shows the field whenever a `vix_regime` row exists and the analyzer writes one for Binance pairs too:

```
🟦 ┃ ⚡ Signals — BTCUSDT
   ┃
   ┃ Price                 RSI 14            Trend
   ┃ $71,204.50 (+2.81%)   68.9              ↔️ sideways
   ┃
   ┃ VIX               BB Squeeze
   ┃ ✅ normal         🔴 ACTIVE
```

ETFs behave the same way: they have no FA Composite in practice, because the composite tier is null even when placeholder rows exist. Note also that the `asset_type` option offers only `equity` and `crypto` — **there is no ETF choice**, and ETFs must be queried as `equity`, which is the correct answer since they live in the equity tables.

And when nothing resolves:

```
❌ No signal data found for **AAPL**.
```

### `/analyze <symbol> [asset_type]`

The widest command. **Process:** cached `SymbolReport` → `formatter.symbol_report_embeds()`, which appends up to 10 panels in fixed order, skipping any whose data is absent.

| # | Embed | Colour | Appears when |
|---|---|---|---|
| 1 | **📈/📉 `SYMBOL` price** | Green/Red | Price row exists |
| 2 | **📐 Context vs SPY** | Blue | Benchmark cycle, macro-correlation regime, or 20-day relative strength available. Shows benchmark composite/price phase, drawdown from peak, macro-correlation regime + read, and the symbol's 20-day excess return vs the benchmark in percentage points |
| 3 | **⚙️ Market ops — `SYMBOL`** | Purple | `MARKET_OPS` enabled and any of VIX regime / ATR% / volume ratio present. Labels the VIX field *"US equities index — crypto risk backdrop"* for crypto, to make clear the index is not native to the asset |
| 4 | **📊 Technical Analysis — `SYMBOL` (`interval`)** | Blue | Any indicator row exists. RSI (annotated oversold/overbought/✅), MACD histogram + cross, ADX, Trend + slope, MA Cross, ATR, BB Squeeze, VIX regime, classic pivots (PP/R1/S1), SMC counts (FVGs, OBs, liquidity sweeps), and Patterns (H&S with confirmation state, triangles with breakout direction, bull/bear flags, up to 3 candle patterns) |
| 5 | **📋 Fundamentals — `SYMBOL` `<tier>` (`<score>`)** | Purple | **Any** fundamentals row exists — which includes ETFs whose rows are entirely null, producing a panel of `⚪ —` placeholders. EPS strength, revenue, P/E vs 5Y, FCF yield, gross & net margin (each with a trend arrow), PEG, earnings surprise, TTM P/E, market cap |
| 6 | **🏦 Balance Sheet — `SYMBOL` `<health>` (`<score>`)** | Purple | Any Tier 2 metric present. ROE/ROA, ROIC (5-year average), D/E, Net Debt/EBITDA, EV/EBITDA, current + quick ratio, P/B, dividend yield + sustainability, CapEx intensity |
| 7 | **🔍 Deep Context — `SYMBOL`** | Dark purple | Any Tier 3 metric present. Share count trend, DCF margin of safety (price as % of intrinsic), interest coverage, P/S, asset + inventory turnover, analyst target upside, goodwill share of assets, FCF conversion, analyst recommendation trend |
| 8 | **🧠 Qualitative Signals — `SYMBOL`** | Tier-driven | Moat proxy, non-neutral insider signal, non-`insufficient_data` news sentiment, or R&D tier present. Moat proxy with gross-margin mean and σ, 90-day insider buyer/seller counts, 7d vs 30d news sentiment, R&D as % of revenue |
| 9 | **🔗 Correlations — `SYMBOL` `<net signal>` (`<score>`)** | Green/Grey/Red | A master signal fired or any cluster tier exists. Cluster health for earnings quality / valuation vs quality / leverage & liquidity / operational; then **⚡ Master Signals Fired** — ★ Bullish Convergence, ★ Hidden Value, ★ Deterioration Warning, ★ Value Trap, ★ Leverage Cycle Warning, each with its plain-language condition; then ≤4 divergence warnings and ≤3 aligned signals, with `…and N more` when truncated |
| 10 | **💬 Sentiment & News — `SYMBOL`** | Grey | Sentiment or news exists. Sentiment score by source, then ≤5 headlines as `**[Sep 10]** [headline](url) 🟢`, where the dot is per-headline sentiment |

**Outcome if nothing resolves:** `❌ No data found for **AAPL**.`

#### The panel stack

A full equity `/analyze` is 8–10 embeds, which exceeds one message. The reply carries the first batch and a follow-up carries the rest:

```
  ┌─ reply ──────────────────────────────────────────┐
  │ 🟩 📈 AAPL  $189.45  (+1.24%)                     │
  │ 🟦 📐 Context vs SPY                              │
  │ 🟪 ⚙️ Market ops — AAPL                           │
  │ 🟦 📊 Technical Analysis — AAPL (1Day)            │
  │ 🟪 📋 Fundamentals — AAPL 🟢 strong (0.62)         │
  └──────────────────────────────────────────────────┘
  ┌─ follow-up ──────────────────────────────────────┐
  │ 🟪 🏦 Balance Sheet — AAPL 🟢 excellent (0.71)    │
  │ 🟪 🔍 Deep Context — AAPL                         │
  │ 🟩 🧠 Qualitative Signals — AAPL                  │
  │ 🟩 🔗 Correlations — AAPL 🟢 bullish (0.44)        │
  │ ⬜ 💬 Sentiment & News — AAPL                     │
  └──────────────────────────────────────────────────┘
```

Crypto is much shorter — panels 5–9 are equity-only, so you get price, context, market ops, technical, and sentiment in a single message.

#### Each panel, rendered

**2 · Context vs benchmark** — the macro backdrop, compressed. Relative strength is in *percentage points of excess return*, not a ratio.

```
🟦 ┃ 📐 Context vs SPY
   ┃
   ┃ Benchmark cycle
   ┃ Composite pullback_healthy
   ┃ Price pullback
   ┃ Drawdown from peak -3.13%
   ┃
   ┃ Macro correlation regime
   ┃ stagflation risk · -0.58
   ┃ Inflation stance hot while growth is rolling over — stagflation-style
   ┃ pressure on risk assets.
   ┃
   ┃ Additional context
   ┃ bond–equity 60d ρ≈-0.26 (deflationary_hedge) · oil-equity 60d ρ≈-0.35
   ┃ (decoupled) · VIX-equity 60d ρ≈-0.85 (typical_fear_greed) · April:
   ┃ strong_bull · election cycle yr2 (midterm)
   ┃
   ┃ mc_market_cycle · mc_macro_correlation · aa_reference_snapshot · equity_ohlcv
```

| Field | Possible values | When absent |
|---|---|---|
| **Composite** | the twelve `composite_phase` values listed under [Market cycle](#the-macro-panels-rendered), printed raw | no market-cycle row |
| **Price** | `crash` · `bear` · `correction` · `pullback` · `bull_extended` · `bull` · `below_sma` · `insufficient_data` | same |
| **Drawdown from peak** | negative percentage, 2 dp | same |
| **Macro correlation regime** | the eight regimes, underscores replaced by spaces, plus the fixed per-regime score and an optional prose read | `mc_macro_correlation` missing |
| **20d relative strength** | excess return in percentage points versus the benchmark | **omitted when the symbol *is* the benchmark** — as in the sample, where `/analyze SPY` compares SPY to itself |
| **Additional context** | any of three rolling correlations, the month bias, and the presidential-cycle year, joined by `·`. Correlation regimes here keep their underscores while the Macro-correlation field above strips them | `aa_reference_snapshot` missing |
| **Whole panel** | — | none of the above exists |

The sample is `/analyze SPY`, which is also the configured benchmark — a useful edge case, because it shows the panel silently dropping the relative-strength field rather than printing `0.00 pp`.

**3 · Market ops** — regime and execution context. The description disclaims it in italics, and for crypto the VIX field is relabelled to make clear the index is not native to the asset.

```
🟪 ┃ ⚙️ Market ops — AAPL
   ┃
   ┃ Regime + execution context — not entry/exit prices or advice.
   ┃
   ┃ VIX (US equities — risk backdrop)
   ┃ 14.2 · normal
   ┃ VIX 14.2 — typical range; baseline sizing rules.
   ┃
   ┃ This symbol (liquidity / noise)
   ┃ ATR% of price (≈equity bar): 1.420%
   ┃ Volume vs median (last 60 bars): 0.870×
   ┃
   ┃ Asset note
   ┃ US equities: RTH liquidity; gaps on news/earnings — see
   ┃ market_operations_reference.html.
   ┃
   ┃ mo_reference_snapshot · MARKET_OPS_* · market_operations_reference.html
```

When a threshold is crossed, a `Flags:` line joins the symbol block — `Flags: atr_pct_elevated, volume_vs_median_elevated`.

The **`This symbol` block disappears entirely** when the symbol has no OHLCV history, leaving only the VIX regime, the `Read` line, and the asset note. That is what you get from a typo or an unresolvable ticker — `/analyze TAO` rather than `TAOUSDT`, for instance, produces a panel with global context but nothing symbol-specific. The `Asset note` still says *"US equities: RTH liquidity…"* in that case, because `asset_type` defaults to `equity` and nothing downstream discovers that the symbol is not one.

**4 · Technical Analysis** — eight inline fields (three rows of 3 / 3 / 2) then full-width rows for pivots, SMC, and patterns. This is where the `—` placeholders are most visible, since the eight inline fields are shown whether or not they fired.

```
🟦 ┃ 📊 Technical Analysis — AAPL (1Day)
   ┃
   ┃ RSI 14            MACD (12/26/9)         ADX 14
   ┃ 54.2 ✅           hist 0.842 🟢 bullish  22.8
   ┃                   cross
   ┃
   ┃ Trend             MA Cross               ATR 14
   ┃ — up (slope       🟢 Golden cross        2.691
   ┃ +1.24%)
   ┃
   ┃ BB Squeeze        VIX Regime
   ┃ —                 ✅ normal (VIX 14.2)
   ┃
   ┃ Pivots
   ┃ PP $188.72 | R1 $191.05 | S1 $186.40
   ┃
   ┃ SMC
   ┃ FVGs: 3 active | OBs: 2 active | Liq sweeps: 1
   ┃
   ┃ Patterns
   ┃ 🟢 Inv. H&S ✅ confirmed | △ ascending → up | 🟢 Bull flag | hammer, doji
   ┃
```

| Field | Every possible value |
|---|---|
| **RSI 14** | a number plus a verdict: `🔴 oversold` below 30, `🔴 overbought` above 70, `✅` in between. Thresholds come from `BOT_RSI_OVERSOLD` / `BOT_RSI_OVERBOUGHT` |
| **MACD (12/26/9)** | `hist <n>` alone, or with `🟢 bullish cross` / `🔴 bearish cross` appended when a line/signal cross fired on the latest bar |
| **ADX 14** | a number, no interpretation |
| **Trend** | `— up` · `— down` · `↔️ sideways`, each with `(slope ±n%)`. The slope is percent change **per bar**, so daily values are small |
| **MA Cross** | `🟢 Golden cross` · `🔴 Death cross` · `—`. Both can never be true at once |
| **ATR 14** | a number in price units, 3 dp |
| **BB Squeeze** | `🔴 ACTIVE — breakout expected` or `—`. There is no "no squeeze" wording |
| **VIX Regime** | `😱 extreme_fear` (>35) · `⚠️ elevated` (>20) · `✅ normal` · `💤 complacency` (<12), each with the VIX level. **No `no data` value exists** — when the VIX series is missing the row simply is not written and the field shows `—` |
| **Pivots** | classic `PP`/`R1`/`S1` only. The worker also computes R2/R3/S2/S3 and Camarilla and Woodie sets, none of which are displayed |
| **SMC** | up to three counts — `FVGs: n active`, `OBs: n active`, `Liq sweeps: n` — each omitted individually when its indicator is missing |
| **Patterns › H&S** | `🔴 H&S` or `🔴 H&S ✅ confirmed`; `🟢 Inv. H&S` or `🟢 Inv. H&S ✅ confirmed` |
| **Patterns › triangle** | `△ ascending` · `△ descending` · `△ symmetrical`, optionally `→ up` or `→ down` on breakout |
| **Patterns › flag** | `🟢 Bull flag` · `🔴 Bear flag` |
| **Patterns › candles** | up to 3 of: `doji`, `hammer`, `shooting_star`, `pin_bar`, `bullish_engulfing`, `bearish_engulfing`, `inside_bar` |

Reading this panel:

- **`Pivots`, `SMC`, and `Patterns` vanish entirely** when empty, unlike the eight inline fields above them. A panel ending at VIX Regime means no pivot data and a featureless chart.
- **A VIX field appears on crypto too.** The analyzer writes a `vix_regime` row for every symbol it processes, including Binance pairs, so `/analyze BTCUSDT` shows US equity VIX here. Only the daily-report card footer suppresses it for crypto.
- **The field labels are hardcoded to the default periods.** `RSI 14`, `ADX 14`, `ATR 14` and `MACD (12/26/9)` are literal strings, and the bot looks up the matching indicator names. If you change `TECHNICAL_RSI_PERIOD`, `TECHNICAL_ADX_PERIOD`, `TECHNICAL_ATR_PERIOD` or any of the MACD periods in the analyzer's config, the worker writes `rsi_21` (say) while the bot still asks for `rsi_14`, and **the field silently renders `—` even though the data exists**. Leave those periods at their defaults unless you also change the bot.
- **`— up`** is the trend-emoji gap again.

**5 · Fundamentals (Tier 1)** — the headline FA verdict, with the composite tier and score in the title so it is readable while collapsed.

```
🟪 ┃ 📋 Fundamentals — AAPL  🟢 strong  (0.62)
   ┃
   ┃ EPS Strength      Revenue                P/E vs 5Y
   ┃ 🟢 strong         🟢 strong              🔴 expensive_vs_history
   ┃                                          (+18.40%)
   ┃
   ┃ FCF Yield         Gross Margin           Net Margin
   ┃ 🟢 +3.82%         🟢 +46.20% 📈          🟢 +25.10% ➡️
   ┃ (attractive)      (strong_moat)          (strong)
   ┃
   ┃ PEG               Earnings Surprise      TTM P/E
   ┃ 🟡 fairly_valued_ 🟢 +4.20% (beat)       31.4
   ┃ growth
   ┃
   ┃ Market Cap
   ┃ $2.94T
```

| Field | Every possible tier |
|---|---|
| Title composite | `strong` 🟢 · `neutral` 🟡 · `weak` 🔴 · absent ⚪, with score in `[−1, +1]` |
| **EPS Strength** | `strong` · `neutral` · `weak` |
| **Revenue** | `strong` · `neutral` · `weak` |
| **P/E vs 5Y** | `cheap_vs_history` · `fair_vs_history` · `expensive_vs_history` · `loss_making`. When no 5-year mean is available the worker falls back to absolute P/E and writes `value` · `growth_fair` · `expensive` instead — **the same field carries two different vocabularies** depending on history depth |
| **FCF Yield** | `attractive` · `fair` · `avoid` |
| **Gross Margin** | `strong_moat` · `average` · `margin_pressure` |
| **Net Margin** | `strong` · `average` · `pressure` — note this is *not* the same set as gross margin |
| **Margin trend arrow** | 📈 `expanding` · ➡️ `stable` · 📉 `compressing` |
| **PEG** | `undervalued_growth` · `fairly_valued_growth` · `expensive_growth` |
| **Earnings Surprise** | `beat` · `inline` · `miss` |
| **TTM P/E**, **Market Cap** | plain numbers, no tier |

The trailing emoji on the margin fields is a **trend arrow**, separate from the tier emoji: `🟢 +46.20% 📈` is a good margin that is still improving.

Two vocabularies are easy to confuse because both contain `growth_`-prefixed words. `growth_fair` on the **P/E** row is the absolute-P/E fallback; `fairly_valued_growth` on the **PEG** row is a different metric entirely; and `growth_premium_required` belongs to EV/EBITDA and P/S in the panels below.

**5b · Fundamentals on an ETF** — the same panel, with every slot empty. This is what `/analyze SPY` produces:

```
🟪 ┃ 📋 Fundamentals — SPY  ⚪  (—)
   ┃
   ┃ EPS Strength      Revenue                P/E vs 5Y
   ┃ ⚪ —              ⚪ —                   —
   ┃
   ┃ FCF Yield         Gross Margin           Net Margin
   ┃ ⚪ — (—)          ⚪ — ⚪                 ⚪ — ⚪
   ┃
   ┃ PEG               Earnings Surprise
   ┃ ⚪ —              ⚪ — (—)
```

This is **correct behaviour, not a failure**. The ingestion worker calls the provider's metrics endpoint for every configured symbol and writes rows even when the values come back null, so an ETF ends up with rows that contain nothing. The bot builds a snapshot whenever *any* row exists, so the panel renders with placeholders throughout, no composite tier, and no TTM P/E or Market Cap rows at all. An ETF with no rows whatsoever — XLF and EEM in current output — correctly gets **no panel**, which is why two ETFs in the same report can differ.

Note also that the Tier 2, Tier 3, Qualitative and Correlations panels are all absent here: each has its own has-data guard, and none of them tolerate an all-null snapshot.

**6 · Balance Sheet (Tier 2)** — omitted entirely when no Tier 2 metric exists. ROE and ROIC are full-width because they carry sub-values.

```
🟪 ┃ 🏦 Balance Sheet — AAPL  🟢 healthy  (0.71)
   ┃
   ┃ ROE (Return on Equity)
   ┃ 🟢 +147.20% (excellent)  ROA +28.40%
   ┃
   ┃ ROIC
   ┃ 🟢 +58.10% (moat_quality) · 5Y avg
   ┃
   ┃ Debt/Equity       Net Debt / EBITDA      EV/EBITDA
   ┃ 🟡 1.87× (        🟢 0.42× (             🔴 24.1× (growth_
   ┃ manageable)       conservative)          premium_required)
   ┃
   ┃ Current Ratio     Price/Book             Dividend Yield
   ┃ 🔴 0.87           🔴 48.20×              🟡 +0.51% —
   ┃ (liquidity_risk)  (limited_safety_…)     moderate_yield
   ┃ Quick 0.83
   ┃
   ┃ CapEx Intensity
   ┃ 🟢 +2.80% of revenue — asset_light
   ┃
   ┃ Balance sheet context · Compare D/E & EV/EBITDA within sector
```

| Field | Every possible tier |
|---|---|
| Title health | `healthy` 🟢 · `neutral` 🟡 · `stressed` 🔴 — the same tier that drives the `BS:` chip on daily cards |
| **ROE** | `excellent` · `adequate` · `destroying_value` |
| **ROA** (sub-value on the ROE row) | `high` · `moderate` · `low` |
| **ROIC** | `moat_quality` · `adequate_roic` · `low_roic` |
| **Debt/Equity** | `conservative` · `manageable` · `high_leverage` |
| **Net Debt / EBITDA** | `net_cash` · `conservative` · `manageable` · `high_risk` |
| **EV/EBITDA** | `value_territory` · `fairly_valued` · `growth_premium_required` |
| **Current Ratio** | `safe` · `monitor` · `liquidity_risk` |
| **Quick Ratio** | `adequate` · `monitor` · `low` |
| **Price/Book** | `value_signal` · `fair` · `limited_safety_margin` |
| **Dividend Yield** | `no_dividend` · `low_yield` · `moderate_yield` · `verify_payout` · `sustainable_income` · `cut_risk` · `monitor_payout` |
| **CapEx Intensity** | `asset_light` · `moderate_intensity` · `capital_intensive` |

Note that "healthy" in the title is computed from only three inputs — ROE, leverage and current ratio — so a green title can sit above a red Current Ratio row, as in the sample. The footer is a warning against reading any of these absolutely: a D/E that is alarming for a utility is unremarkable for a software company.

**7 · Deep Context (Tier 3)** — darker purple to distinguish it from Tier 2. Depends on Finnhub XBRL data, so it is the panel most often absent on a fresh install.

```
🟪 ┃ 🔍 Deep Context — AAPL
   ┃
   ┃ Share Count Trend    Interest Coverage
   ┃ 🟢 -2.4%/yr —        🟢 41.2× (very_safe)
   ┃ buyback
   ┃
   ┃ DCF Margin of Safety
   ┃ 🔴 price = 128% of intrinsic  (growth assume 8.0%)
   ┃
   ┃ Price/Sales          Asset Turnover
   ┃ 🔴 8.2×              1.09×  |  Inventory 38.4×/yr
   ┃ (growth_premium_re…)
   ┃
   ┃ Analyst Target
   ┃ 🟢 +9.80% upside  (target $208.00)
   ┃
   ┃ Goodwill/Intangibles  FCF Conversion
   ┃ 🟢 1.2% of assets —   🟢 1.18× (high_quality_cash)
   ┃ low_risk
   ┃
   ┃ Analyst Rec Trend
   ┃ 🟢 +3 net delta — upgrading  (net score 28)
   ┃
   ┃ Deep context · DCF is directional only — see bot.md for assumptions
```

| Field | Every possible tier |
|---|---|
| **Share Count Trend** | `buyback` · `flat` · `dilution_risk` |
| **Interest Coverage** | `very_safe` · `adequate` · `high_risk` |
| **DCF Margin of Safety** | `strong_margin_of_safety` · `fairly_valued` · `downside_risk` |
| **Price/Sales** | `value` · `fairly_valued` · `growth_premium_required` · `speculative` |
| **Asset Turnover** | `high` · `moderate` · `low`. The inventory turnover shown beside it has **no tier** |
| **Analyst Target** | `bullish_consensus` · `neutral` · `bearish_consensus` |
| **Goodwill/Intangibles** | `low_risk` · `monitor` · `impairment_risk` |
| **FCF Conversion** | `high_quality_cash` · `moderate` · `accrual_concern` |
| **Analyst Rec Trend** | `upgrading` · `neutral` · `downgrading` |

`price = 128% of intrinsic` means the market is paying a 28 % premium to the model's fair value. The footer's "directional only" caveat matters — the DCF uses a single assumed growth rate, shown inline.

**8 · Qualitative Signals** — structural proxies, all full-width. Colour comes from the moat proxy or insider signal, so this panel can be green while the fundamentals panel is red.

```
🟩 ┃ 🧠 Qualitative Signals — AAPL
   ┃
   ┃ Moat Proxy
   ┃ 🏰 strong moat proxy  GM avg 44.8%  σ 1.2pp
   ┃
   ┃ Insider Activity (90d)
   ┃ 🔴 cluster sell  4 seller(s)
   ┃
   ┃ News Sentiment
   ┃ 🟢 7d: +0.28 | 30d: +0.14
   ┃
   ┃ R&D Intensity
   ┃ 🟢 investing in future  7.8% of revenue
   ┃
   ┃ Qualitative · Structural proxies only — moat/insider/sentiment/R&D
```

| Field | Every possible tier | Shown when |
|---|---|---|
| **Moat Proxy** | `strong_moat_proxy` 🏰 · `moderate_moat_proxy` · `weak_moat_proxy` | tier present |
| **Insider Activity (90d)** | `cluster_buy` · `single_buy` · `neutral` · `cluster_sell` | **only when not `neutral`** — a missing row means no unusual activity, not no data |
| **News Sentiment** | `positive` · `neutral` · `negative` · `insufficient_data`, for each of the 7d and 30d windows | **only when not `insufficient_data`** |
| **Whole panel** | — | at least one of moat / non-neutral insider / usable sentiment / R&D exists |
| **R&D Intensity** | `investing_in_future` · `moderate` · `harvesting` | R&D reported |

The moat proxy is inferred from gross-margin *stability* — a high mean with a low σ implies pricing power — which is why it gets the 🏰 rather than a traffic light. The 7d-versus-30d sentiment split is there to show direction, not level.

**9 · Correlations** — cross-metric divergence. This is the highest-signal panel, because it flags cases where individual metrics look fine but disagree with each other.

```
🟩 ┃ 🔗 Correlations — AAPL  🟢 bullish  (0.44)
   ┃
   ┃ Cluster Health
   ┃ 🟢 Earnings Quality — healthy
   ┃ 🟠 Valuation vs Quality — mixed negative
   ┃ 🟢 Leverage & Liquidity — healthy
   ┃ 🟡 Operational — mixed positive
   ┃
   ┃ ⚡ Master Signals Fired
   ┃ 🟢 ★ Hidden Value — EPS stagnant but FCF conversion high + attractive
   ┃ FCF yield (market prices on EPS; real cash missed)
   ┃
   ┃ ⚠️ Divergence Warnings
   ┃ • P/E expensive vs history while revenue growth decelerates
   ┃ • Current ratio below 1.0 despite strong operating cash flow
   ┃ • Insider cluster selling into price strength
   ┃ _…and 2 more_
   ┃
   ┃ ✅ Aligned Signals
   ┃ • High ROIC confirmed by high FCF conversion
   ┃ • Buybacks funded from FCF, not debt
   ┃
   ┃ Correlations · Cross-metric divergence — see bot.md for signal definitions
```

| Field | Every possible value |
|---|---|
| Title net signal | `strongly_bullish` 🟢 · `bullish` 🟢 · `neutral` 🟡 · `bearish` 🔴 · `strongly_bearish` 🔴, with the summary score in `[−1, +1]` |
| **Each of the four clusters** | `healthy` (score ≥ 0.5) · `mixed_positive` (≥ 0) · `mixed_negative` (≥ −0.5) · `alert`, displayed with underscores replaced by spaces |
| **Cluster names** | always exactly these four: Earnings Quality, Valuation vs Quality, Leverage & Liquidity, Operational |
| **★ Master signals** | any subset of ★ Bullish Convergence · ★ Hidden Value · ★ Deterioration Warning · ★ Value Trap · ★ Leverage Cycle Warning. Each fires only when enough of its conditions are met (3, 2, 2, 3, 3 respectively) |
| **Warnings / Aligned signals** | drawn from a fixed catalogue of roughly twenty prose strings written by the worker, some with numbers interpolated. Warnings cap at 4, positives at 3, with `_…and N more_` appended |
| **Whole panel** | omitted when no signal fired and every cluster is healthy |

The ★ master signals are the conviction calls, and each renders with its full condition spelled out, so you never have to remember what fired.

One caveat worth knowing when a cluster looks quieter than expected: two of the correlation checks read payload keys that the upstream worker never writes. The FCF/EPS divergence check looks for a `tier` field on a metric that stores its verdict under `quality`, and the margin-trend checks look for `tier` where the worker writes `direction`. Those specific conditions therefore never contribute, which biases the Earnings Quality cluster slightly toward `healthy`.

**10 · Sentiment & News** — grey, always last. Headlines are hyperlinked when a URL exists.

```
⬜ ┃ 💬 Sentiment & News — AAPL
   ┃
   ┃ Sentiment (finnhub)
   ┃ 0.3
   ┃
   ┃ Latest Headlines
   ┃ [Sep 10] Apple unveils new iPhone lineup with on-device AI 🟢
   ┃ [Sep 10] Supplier checks point to softer Q4 builds 🔴
   ┃ [Sep 09] Services revenue hits record in latest quarter 🟢
   ┃ [Sep 09] EU regulator opens fresh App Store inquiry 🔴
   ┃ [Sep 08] Analyst raises target on AI upgrade cycle 🟢
```

| Slot | Possible values |
|---|---|
| **`Sentiment (source)`** | a float; the source in the field name is whatever `sentiment_snapshots.source` holds (`finnhub`, `gdelt`, …). **The whole field is dropped when there is no aggregate row** |
| **Headline dot** | 🟢 positive · 🔴 negative · nothing for neutral or scoreless |
| **Headline text** | truncated to **60 characters when it has a URL** (to leave room for the link markup) and **80 when it does not** — which is why linked headlines cut mid-word more often |
| **Whole panel** | omitted only when there is neither a sentiment row nor a single headline |

The two halves are independent. A panel with headlines but no `Sentiment` field — common on ETFs and index products — simply means no aggregate snapshot was collected for that symbol, not that sentiment was zero. Likewise, the per-headline dot is that article's own score and can disagree with the aggregate above it.

**One rendering bug shows up here with news-aggregator links.** The five headlines are joined and then truncated to fit Discord's 1024-character field limit. Hyperlinked headlines carry their full URL inside the markdown, and Google News RSS URLs are several hundred characters long, so the truncation can land in the middle of a link. Discord then cannot parse the markdown and prints it literally:

```
⬜ ┃ 💬 Sentiment & News — TAO
   ┃
   ┃ Latest Headlines
   ┃ [Apr 08] Markets shift back towards potential Fed rate cut this year
   ┃ [Apr 08] Saudi Arabia's oil pipeline bypassing Hormuz damaged in Iran
   ┃ [Apr 08] CEO shares a 'very dangerous' red flag in a boss—it makes em
   ┃ [Apr 08] [Iranian Oil Refining Company confirms attack on Lavan refine](
   ┃ https://news.google.com/rss/articles/CBMiwwFBVV95cUxPNmEyWW9lZkpSSkZrLXduc…
```

The first three render as clean clickable text; the fourth spills its raw markup and the fifth is gone. Nothing is wrong with the data — it is always the last headline in the field that breaks, and only when the preceding URLs were long.

And when the symbol has nothing at all:

```
❌ No data found for **AAPL**.
```

### `/marketops [symbol] [asset_type]`

**Process:** `build_market_ops_view()` reads `mo_reference_snapshot` from `macro_derived` (written hourly by `market-operations`), then **re-derives the VIX regime live**: it prefers `VIXCLS` from `macro_fred`, falls back to the benchmark's TA `vix_regime`, and classifies with the `BOT_MARKET_OPS_VIX_*` bands (12/20/35). Symbol is optional.

**Outcomes:**

| | Rendering |
|---|---|
| Enabled, no symbol | Purple embed `⚙️ Market operations`, description *"Module 5 reference context. Not buy/sell advice."* Shows snapshot timestamp, VIX regime + numeric value, a narrative "Read" line, and **HTML coverage (automation status)** — ≤12 lines showing which reference modules are automated |
| Enabled, with symbol | Same, title suffixed `— AAPL`, plus a `Symbol · AAPL` field with ATR%, volume vs median, flags, and an asset-specific execution note (RTH gaps for equities, 24/7 microstructure for crypto) |
| Disabled or load failed | Same embed with a single `Status` field: `Market ops disabled (BOT_MARKET_OPS_ENABLE=false) or failed to load.` |

Without a symbol — the global regime view. `HTML coverage` is the automation-status report for the reference document:

```
🟪 ┃ ⚙️ Market operations
   ┃
   ┃ Module 5 reference context. Not buy/sell advice.
   ┃
   ┃ Snapshot as of        VIX regime
   ┃ `2026-09-11T17:00Z`   14.2 · normal
   ┃
   ┃ Read
   ┃ VIX 14.2 — typical range; baseline sizing rules.
   ┃
   ┃ HTML coverage (automation status)
   ┃ • positioning: needs_data — COT, put/call, short interest not ingested.
   ┃ • volatility_regimes: partial_live — VIX regime live from macro_fred;
   ┃   term structure and skew not wired.
   ┃ • liquidity_flows: needs_data — L2 depth, spreads, dark pool prints
   ┃   require a paid feed.
   ┃ • risk_execution: partial_live — ATR% and volume-vs-median computed
   ┃   per symbol; slippage models not automated.
   ┃ • market_structure: needs_data — venue mix, auction imbalances,
   ┃   funding/OI for crypto not ingested.
   ┃
   ┃ Live VIX from macro_fred · /marketops · market_operations_reference.html
```

With a symbol, the execution strip is appended:

```
🟪 ┃ ⚙️ Market operations — BTCUSDT
   ┃
   ┃ Module 5 reference context. Not buy/sell advice.
   ┃
   ┃ Snapshot as of        VIX regime
   ┃ `2026-09-11T17:00Z`   14.2 · normal
   ┃
   ┃ Read
   ┃ VIX 14.2 — typical range; baseline sizing rules.
   ┃
   ┃ HTML coverage (automation status)
   ┃ • positioning: needs_data — COT, put/call, short interest not ingested.
   ┃ …
   ┃
   ┃ Symbol · BTCUSDT
   ┃ ATR%: 3.240%
   ┃ Vol vs median: 2.110×
   ┃ Flags: atr_pct_elevated, volume_vs_median_elevated
   ┃ Crypto: 24/7; funding/OI not wired — microstructure differs from US
   ┃ equity sessions.
   ┃
   ┃ Live VIX from macro_fred · /marketops · market_operations_reference.html
```

When disabled, the embed still renders — with a single status field instead of data:

```
🟪 ┃ ⚙️ Market operations
   ┃
   ┃ Module 5 reference context. Not buy/sell advice.
   ┃
   ┃ Status
   ┃ Market ops disabled (BOT_MARKET_OPS_ENABLE=false) or failed to load.
```

| Slot | Every possible value |
|---|---|
| **VIX regime** | `low` · `normal` · `elevated` · `stress` · `unknown`. Note this is the **market-operations** classifier, whose bands (12 / 20 / 35) produce different words from the technical-analysis one — the same VIX reading is `complacency`/`extreme_fear` in the Technical panel but `low`/`stress` here |
| **`Read` line** | a prose sizing note keyed to the regime; omitted when the regime is `unknown` |
| **HTML coverage module keys** | exactly five, always all present: `positioning`, `volatility_regimes`, `liquidity_flows`, `risk_execution`, `market_structure` |
| **HTML coverage statuses** | `not_automated` · `partial_live` · `needs_data` — a narrower set than the eight-module list in the Additional-analysis panel |
| **`Flags:`** | `atr_pct_elevated` (ATR% ≥ `BOT_MARKET_OPS_ATR_PCT_ELEVATED`, default 3.0) · `volume_vs_median_elevated` (ratio ≥ `BOT_MARKET_OPS_VOLUME_RATIO_ELEVATED`, default 1.8). Comma-joined; the line is omitted when neither fires. **These two flags are computed by the bot, not read from the snapshot** — changing the thresholds takes effect immediately, with no worker restart |
| **Asset note** | one of a small set of fixed strings keyed to asset type — a US-equity session note, a crypto 24/7 note, and so on |
| **Footer** | `Live VIX from macro_fred · …` or `VIX from TA benchmark / snapshot · …` |

The footer discloses the VIX provenance, so you can tell whether you are seeing a fresh reading or a fallback. In the fallback case the `Read` line also gains an inline italic note explaining that `macro_fred` has no `VIXCLS` and pointing at `data-equity`.

### `/report [mode]`

**Process:** `plan_daily_report(mode, cfg)` resolves the mode into a symbol universe plus optional add-ons, then runs the same `build_daily_report` + `daily_report_embeds` path as the scheduled job. **The full macro panel stack (embeds 2–9 of [§4](#4-daily-report)) is always included, in every mode** — modes only change the symbol list and the optional trailing panels.

Output goes to **the channel where you ran the command**, not `#daily-report`.

| Mode | Symbols | Extra panels | Mode tag in title |
|---|---|---|---|
| `standard` *(default)* | Full configured universe | — | none |
| `all` | Identical to `standard` | — | none |
| `etfs` | `BOT_REPORT_ETF_SYMBOLS` (SPY, QQQ, IWM, XLF, EEM, MCHI, …) | — | `ETFs` |
| `commodities` | `BOT_REPORT_COMMODITY_EQUITY_SYMBOLS` (GLD, COPX, USO) | FRED observations table | `Commodities` |
| `macro_fred` | none | FRED observations for ~50 series | `FRED series` |
| `crypto` | `BOT_REPORT_CRYPTO_SYMBOLS`, normalised to Binance pairs (`TIA` → `TIAUSDT`) | — | `Crypto` |
| `equity` | `BOT_REPORT_EQUITY_SYMBOLS`, cleaned (tickers with spaces or slashes dropped, `TSMC` → `TSM`) | — | `Equities` |
| `dashboard` | none | Dashboard strip | `Dashboard` |

`standard` and `all` are genuinely the same code path despite being separate choices.

The shape is identical to [§4](#4-daily-report) — what changes is the title tag and the tail. `/report macro_fred` is the clearest illustration: full macro panels, **zero symbol cards**, then a FRED table:

```
  ┌─ reply ──────────────────────────────────────────┐
  │ 🟦 📊 Daily Market Report · FRED series — 18:22   │
  │ 🟩 🏦 Monetary Policy — 🟢 accommodative (+0.55)   │
  │ 🟨 📈 Growth Cycle — 🟡 Slowdown (+0.27)           │
  │ 🟥 🌡️ Inflation & Prices — 🔴 Hot (+0.55)          │
  │ 🟥 🌍 Global & Geopolitical — 🔴 Elevated stress   │
  │ 🟨 📉 Market cycle — Bull Fragile Global          │
  │ 🟦 🔗 Macro correlations — Stagflation Risk       │
  │ 🟪 📚 Additional analysis · intermarket           │
  │ 🟪 Macro intel · calendars · geo · headlines      │
  │ ⬜ 📈 FRED · latest observations         (20 series)│
  └──────────────────────────────────────────────────┘
  ┌─ follow-up ──────────────────────────────────────┐
  │ ⬜ 📈 FRED · latest observations (cont.)  (20 series)│
  │ ⬜ 📈 FRED · latest observations (cont.)  (10 series)│
  └──────────────────────────────────────────────────┘
```

`/report dashboard` is the shortest — the same nine macro panels plus a single 📌 Dashboard strip and no symbol cards. `/report crypto` swaps every equity card for normalised Binance pairs, so `TIA` in config renders as a `TIAUSDT` card.

### `/alert <symbol> [asset_type]`

A **live threshold check that ignores the Redis cooldown** — use it to answer "would this alert fire right now?" without waiting out the 4-hour window.

**Process:** queries `technical_indicators` directly (no report builder, no cache) and evaluates four checks, sorting each into *triggered* or *within range*.

**Outcomes:**

| | Rendering |
|---|---|
| Any breach | Embed `🔔 Alert Check — AAPL`, **red** `0xFF4444`. A `⚠️ Triggered (N)` field lists breaches with thresholds, e.g. `🔴 \`rsi_oversold\` — RSI 14 = **27.4** (threshold < 30.0)`. A `✅ Within Range` field lists the rest |
| No breach | Same embed, **green** `0x00B050`, only the `✅ Within Range` field |
| No indicator rows | A multi-line help message explaining that `technical_indicators` is written by the `technical-analysis` worker, and to check that `data-technical` is running, the analyzer profile is up, and `BOT_EQUITY_INTERVAL` matches `TECHNICAL_EQUITY_INTERVALS` |

The red/green strip is a **whole-symbol verdict**: red if anything is breached, green if everything is in range. Unlike `#alerts`, breached and clear conditions appear side by side, each with its threshold:

```
🟥 ┃ 🔔 Alert Check — INTC
   ┃
   ┃ ⚠️  Triggered (2)
   ┃ 🔴 `rsi_oversold` — RSI 14 = 27.4 (threshold < 30.0)
   ┃ 🔴 `liquidity_sweep` — 3 sweeps detected — last: `low_sweep`
   ┃
   ┃ ✅ Within Range
   ┃ ✅ BB Squeeze inactive (value=0.00)
   ┃ ✅ VIX = 14.2  regime: `normal`
   ┃
   ┃ Live check — no cooldown applied. Scheduled alerts respect 4-hour cooldown.
```

All clear — green, and the `Triggered` field is absent rather than empty:

```
🟩 ┃ 🔔 Alert Check — AAPL
   ┃
   ┃ ✅ Within Range
   ┃ ✅ RSI 14 = 54.2 (normal range 30.0–70.0)
   ┃ ✅ BB Squeeze inactive (value=0.00)
   ┃ ✅ No liquidity sweeps detected
   ┃ ✅ VIX = 14.2  regime: `normal`
   ┃
   ┃ Live check — no cooldown applied. Scheduled alerts respect 4-hour cooldown.
```

Crypto drops the VIX line entirely, since that check is equity-only:

```
🟥 ┃ 🔔 Alert Check — BTCUSDT
   ┃
   ┃ ⚠️  Triggered (1)
   ┃ 🔴 `bb_squeeze` — Bollinger Squeeze ACTIVE (value=1.00)
   ┃
   ┃ ✅ Within Range
   ┃ ✅ RSI 14 = 68.9 (normal range 30.0–70.0)
   ┃ ✅ No liquidity sweeps detected
   ┃
   ┃ Live check — no cooldown applied. Scheduled alerts respect 4-hour cooldown.
```

And the no-data path, which is plain text and doubles as a setup checklist:

```
❌ No indicator data for **AAPL** (`equity` · `1Day`).

Indicators are read from Postgres table **technical_indicators**, written by the
**technical-analysis** worker from OHLCV bars.
• Ensure **data-technical** is running so **1Day** (or your
  `TECHNICAL_EQUITY_INTERVALS`) bars exist in **equity_ohlcv**.
• Ensure **technical-analysis** (compose profile **analyzer**) is running.
• Match **BOT_EQUITY_INTERVAL** to **TECHNICAL_EQUITY_INTERVALS** (e.g. both `1Day`).
```

Checks: RSI (oversold/overbought), BB squeeze, liquidity sweep (with the last sweep's kind — the only place that direction surfaces without going through `#actions`), and VIX for equities. **`fa_tier_flip` is not checked** — it depends on Redis state rather than a live threshold, so this command can never tell you about a pending tier flip.

### `/action <symbol> [asset_type]`

Runs the actions engine on demand. Unlike the scheduled path, it **shows you the suppressed watch-only results** so you can see what was evaluated and rejected.

**Process:** loads indicators, derived FA, and a live VIX classification, then tests four rule kinds against current indicator state — `rsi_oversold`, `rsi_overbought`, `bb_squeeze`, `liquidity_sweep`. Only rules whose precondition currently holds are evaluated.

**Outcomes:**

| | Rendering |
|---|---|
| Directed action(s) | The same `⚡ ACTION` embed as [§6](#6-actions) — the first as the reply, any others as follow-ups. If watch-only results also exist, a trailing plain-text message: `**Also evaluated (watch-only, not posted to #actions):**` followed by `⚪ \`kind\` → **WATCH** (confluence 1/2) — insufficient signals for directed action` |
| Only watch results | One grey embed `ℹ️ Action Check — AAPL` listing them, footer `All triggered rules produced WATCH/HOLD_WATCH — no directed action.` |
| No threshold breached | `ℹ️ **AAPL** — no alert thresholds are currently breached. No action to evaluate.` |
| Rule raised | That rule appears in the watch list as `⚠️ \`kind\` rule error: <exception>` and the others still run |
| No indicator rows | The same `technical_indicators` help message as `/alert` |

A directed action reproduces the `#actions` embed exactly, then appends the suppressed evaluations as a trailing plain-text message — the one place you can see *why* nothing reached `#actions`:

```
🟩 ┃ ⚡ ACTION — TTE
   ┃
   ┃ Alert: liquidity_sweep │ Action: 🟢 BUY_WATCH
   ┃
   ┃ Confluence Score
   ┃ 4/4 (actionable)
   ┃
   ┃ Reasoning
   ┃ 📍 Low sweep (2 recent): stop-hunt below swing low detected
   ┃ ✅ Closed back above swept level — institutional accumulation pattern
   ┃ ✅ Bullish order block nearby — strong support confluence
   ┃ ✅ Uptrend intact — sweep aligns with trend continuation
   ┃ ℹ️ SMC sweep signals are most reliable on higher timeframes with FVG or OB
   ┃    confluence
   ┃
   ┃ 📋 Next step
   ┃ Run /analyze TTE for full analysis before opening a position.
   ┃
   ┃ Context
   ┃ RSI 55.9 │ Trend: UP │ FA: neutral │ VIX: normal
   ┃
   ┃ ⏱️ FA data: refreshes every 24h | Technical indicators: every 6h | Alert scan: every 5min

**Also evaluated (watch-only, not posted to #actions):**
⚪ `bb_squeeze` → **WATCH** (confluence 2/2) — insufficient signals for directed action
```

That `bb_squeeze → WATCH` line is the MACD key mismatch made visible: confluence reached 2, but the rule still could not name a direction, so it never became an action.

When every triggered rule is watch-only, you get a single grey embed instead:

```
⬜ ┃ ℹ️  Action Check — SHEL
   ┃
   ┃ ⚪ `bb_squeeze` → **WATCH** (confluence 1/2) — insufficient signals for
   ┃ directed action
   ┃ ⚪ `liquidity_sweep` → **WATCH** (confluence 1/2) — insufficient signals
   ┃ for directed action
   ┃
   ┃ All triggered rules produced WATCH/HOLD_WATCH — no directed action.
```

> **Two things about these watch-only lines are misleading, and worth knowing before you trust the number.**
>
> **The denominator is different from the action card's.** Watch-only lines print `{confluence}/{min_confluence}` — so `/2` at default settings — while a posted action card prints `{confluence}/{min_confluence + 2}`, or `/4`. The same confluence of 3 therefore appears as `3/2` when suppressed and `3/4` when posted. Only the numerator is comparable between the two.
>
> **"Insufficient signals" is not always true.** The wording is hardcoded, but the suppression is decided by the action *label*, not the score. A liquidity sweep under a risk-off VIX with a non-uptrend resolves to `WATCH` no matter how much confluence it accumulated, so you can legitimately see `⚪ liquidity_sweep → WATCH (confluence 3/2) — insufficient signals for directed action` — three signals, above the minimum of two, still suppressed. Read it as "the rule declined to pick a direction", not "not enough evidence".
>
> That case is also the cleanest demonstration that **the VIX regime alone can flip an outcome.** The identical `liquidity_sweep` alert produces a green `BUY_WATCH` card when VIX is `normal` and a suppressed `WATCH` when VIX is `elevated`, with no change in the chart itself.

If a rule raises, it appears in the same list without stopping the others:

```
⬜ ┃ ℹ️  Action Check — SHEL
   ┃
   ┃ ⚠️  `liquidity_sweep` rule error: 'NoneType' object is not subscriptable
   ┃ ⚪ `bb_squeeze` → **WATCH** (confluence 1/2) — insufficient signals for
   ┃ directed action
   ┃
   ┃ All triggered rules produced WATCH/HOLD_WATCH — no directed action.
```

And the common quiet case — plain text, no embed:

```
ℹ️  **AAPL** — no alert thresholds are currently breached. No action to evaluate.
```

Two deliberate differences from the scheduled engine: `vix_elevated` and `fa_tier_flip` are **not** evaluated here (both need scan-time or Redis state), and nothing is written to `#actions`. So `/action` on a symbol whose FA tier just flipped reports "no thresholds breached" even though `#actions` has a card for it.

### `/dictionary`

**Process:** reads [`bot.md`](services/analyst-bot/bot.md) (~81 KB) from `/app/bot.md` in the container, splits it on top-level `##` headings, and emits one blue embed per section. Sections over 4000 characters are re-split at `###` boundaries and greedily regrouped, keeping every embed under Discord's 4096-character description cap. Horizontal rules are stripped.

**Outcomes:**

| | Rendering |
|---|---|
| Success | ~15–20 blue embeds, each titled with its section heading (Colour Coding, Technical Analysis Fields, Balance Sheet Analysis, Master Divergence Signals, …) and footed `📖 Bot Dictionary • 3/18`. Delivered as several consecutive messages |
| File missing | `❌ \`bot.md\` not found inside the container. The file should be at \`/app/bot.md\`.` |

Each embed is one `##` section of `bot.md`, rendered as a description rather than fields, with a page counter in the footer:

```
🟦 ┃ Colour Coding
   ┃
   ┃ 🟢 Green — bullish / healthy / attractive
   ┃ 🟡 Yellow — neutral / fair / monitor
   ┃ 🔴 Red — bearish / stressed / avoid
   ┃ ⚪ White — insufficient data or unmapped tier
   ┃
   ┃ 📖 Bot Dictionary  •  1/18

🟦 ┃ Technical Analysis Fields
   ┃
   ┃ **RSI 14 (Relative Strength Index)**
   ┃ Momentum oscillator, 0–100. Below 30 = oversold, above 70 = overbought.
   ┃ Computed by technical-analysis from close prices…
   ┃
   ┃ **MACD (12/26/9)**
   ┃ Trend-following momentum. The histogram is the value shown…
   ┃
   ┃ 📖 Bot Dictionary  •  5/18
```

Large sections are split at `###` boundaries and regrouped, which is why a single heading like *Technical Analysis Fields* can span several pages — the subsection titles are re-emitted as bold lines so context is never lost mid-page. The whole set arrives as two or three consecutive messages.

This is the in-chat lexicon for every emoji, tier, and threshold the other commands emit, and it is generated from the file rather than duplicated — editing `bot.md` changes the command output with no code change.

### `/status`

**Ephemeral — only you see it.** The operational health check.

**Process:** `SELECT 1` against Postgres, macro-intel row counts, latest `macro_derived` timestamps for `mc_market_cycle` / `mc_macro_correlation` / `aa_reference_snapshot` / `mo_reference_snapshot`, a Redis `PING`, and scheduler introspection.

**Rendering:** one blue embed `🤖 Bot Status`, visible only to you:

```
🟦 ┃ 🤖 Bot Status
   ┃
   ┃ Database          Redis             Scheduler
   ┃ ✅ Connected      ✅ Connected      ✅ Running (2 jobs)
   ┃
   ┃ Macro intel tables (rows)
   ┃ `economic_calendar_events`: 1284
   ┃ `earnings_calendar_events`: 412
   ┃ `geopolitical_risk_monthly`: 918
   ┃ `gdelt_macro_daily`: 64
   ┃ `news_headlines`: 28104
   ┃
   ┃ Macro derived (latest ts)
   ┃ `aa_reference_snapshot`: 2026-09-11 12:00:00+00
   ┃ `mc_macro_correlation`: 2026-09-11 12:00:00+00
   ┃ `mc_market_cycle`: 2026-09-11 12:00:00+00
   ┃ `mo_reference_snapshot` (market_operations): 2026-09-11 17:00:00+00
   ┃
   ┃ Equity Symbols
   ┃ XOM, CVX, SHEL, BB, TTE, 2222.SR, TSM, INTC, GFS, OXIG.L, KEYS, COHR, …
   ┃
   ┃ Crypto Symbols
   ┃ BTCUSDT, ETHUSDT, LINKUSDT, KSMUSDT, TAOUSDT, RENDERUSDT, SOLUSDT, TIAUSDT
   ┃
   ┃ Alert Scan Interval
   ┃ 300s
   ┃
   ┃ Scheduled Jobs
   ┃ `alert_scan` — next: 2026-09-11 18:27:00+00:00
   ┃ `daily_report` — next: 2026-09-12 07:00:00+00:00
   ┃
   ┃ Latency: 42.1ms
```

**The `Macro derived (latest ts)` block is the single most useful diagnostic in the bot.** If those timestamps are hours or days old, the grey "data missing" cards in `#daily-report` are explained — `macro-analysis` is stale or stopped. Compare `mo_reference_snapshot` (hourly) against the three `macro_analysis` metrics (6-hourly) to tell which worker is at fault.

Each sub-check degrades independently rather than failing the command:

```
🟦 ┃ 🤖 Bot Status
   ┃
   ┃ Database          Redis
   ┃ ✅ Connected      ❌ Error 111 connecting to redis:6379. Connection refused.
   ┃
   ┃ Macro intel tables
   ┃ ⚠️ relation "gdelt_macro_daily" does not exist
   ┃
   ┃ Macro derived (latest ts)
   ┃ ⚠️ relation "macro_derived" does not exist
   ┃ …
```

A red Redis here is the explanation for a flooding `#alerts` channel, and missing tables mean migrations have not been applied.

### `/ping`

No deferral, no DB access, no embed — the only plain-text, non-ephemeral reply in the bot:

```
🏓 Pong! Latency: **42.1ms**
```

This is the Discord gateway heartbeat, **not** query time. A healthy `/ping` alongside a slow `/analyze` points at the database, not the network.

---

## 8. Configuration reference

Everything below is read by [`services/analyst-bot/config.py`](services/analyst-bot/config.py) from the shared root `.env`. Unknown keys are ignored, so the same file serves all services.

> **`.env.example` is incomplete for the bot.** It ships the symbol universes, market-ops, FOMC, and startup-report keys, but **not** the Discord channel IDs, alert thresholds, cron/scan schedule, actions settings, or cache TTLs. Those fall back to the code defaults below unless you add them by hand — copy the block from [`discord.md`](services/analyst-bot/notifier/discord/discord.md). Note also that `.env.example` sets `BOT_REPORT_ON_STARTUP_DELAY=5` where the code default is `30`.

### Channels and identity

| Variable | Default | Effect on output |
|---|---|---|
| `DISCORD_BOT_TOKEN` | — | Required; the bot exits if unset |
| `DISCORD_GUILD_ID` | unset | Registers slash commands instantly on one server instead of waiting up to an hour for global propagation |
| `DISCORD_DAILY_REPORT_CHANNEL_ID` | unset | Unset ⇒ no scheduled or startup reports |
| `DISCORD_ALERTS_CHANNEL_ID` | unset | Unset ⇒ no alerts, which also means no actions |
| `DISCORD_ACTIONS_CHANNEL_ID` | unset | Unset ⇒ actions engine returns before evaluating |
| `DISCORD_COMMANDS_CHANNEL_ID` | unset | Read but never used — documentation only |

### Scheduling

| Variable | Default | Effect |
|---|---|---|
| `BOT_DAILY_REPORT_CRON` | `0 7 * * *` | UTC cron for the daily report. A malformed value logs an error and **skips the job**, leaving only the alert scan |
| `BOT_ALERT_SCAN_INTERVAL` | `300` | Seconds between scans; also the actions cadence |
| `BOT_REPORT_ON_STARTUP` | `true` | Posts a full report on every restart |
| `BOT_REPORT_ON_STARTUP_DELAY` | `30` (but `5` in `.env.example`) | Settling delay in seconds before that send |
| `BOT_FOMC_NARRATIVE_ENABLE` | `false` | Requires `OPENAI_API_KEY` **and** `FOMC_STATEMENT_URL`; scores a policy statement −1 (dovish) … +1 (hawkish) into `narrative_scores`, surfacing in the macro-intel embed |
| `BOT_FOMC_NARRATIVE_CRON` | `0 18 * * 3` | Wednesday 18:00 UTC |

There is no weekly digest. The older setup guide at `services/analyst-bot/notifier/discord/discord.md` documents a `BOT_WEEKLY_DIGEST_CRON`, but no such setting exists in the bot's config and no weekly job is registered with the scheduler — setting it has no effect. That file also predates the `#actions` channel and describes `/signals` as showing a MACD cross, which it no longer does; treat this README as the current reference.

### Alert thresholds and dedup

| Variable | Default | Effect |
|---|---|---|
| `BOT_RSI_OVERSOLD` / `BOT_RSI_OVERBOUGHT` | `30` / `70` | RSI trigger bands, echoed in the alert message text |
| `BOT_VIX_ALERT_THRESHOLD` | `25` | VIX alert level. Independent of the fixed 35/20/12 regime cuts the actions engine uses |
| `BOT_ALERT_COOLDOWN_SECS` | `14400` | Redis TTL per alert key — the single most effective noise control |

### Actions

| Variable | Default | Effect |
|---|---|---|
| `BOT_ACTIONS_ENABLED` | `true` | Master switch for `#actions` |
| `BOT_ACTIONS_MIN_CONFLUENCE` | `2` | Controls the `⚡ ACTION` vs `ℹ️ WATCH` title and the score denominator (`min + 2`). Does **not** control which cards post — the `WATCH`/`HOLD_WATCH` label gate does |
| `BOT_ACTIONS_COOLDOWN_SECS` | `14400` | Present in config but not currently enforced; actions inherit the alert cooldown |

### Symbols and intervals

| Variable | Default | Effect |
|---|---|---|
| `BOT_EQUITY_SYMBOLS` | empty | When empty, merges `EQUITY_SYMBOLS_STOCKS` + `_ETFS` + `_COMMODITY_ETFS` (deduped, case-insensitive); if those are empty too, falls back to `AAPL,MSFT,SPY` |
| `BOT_CRYPTO_SYMBOLS` | 8 Binance pairs | Also drives crypto auto-detection in `/alert`, `/action`, `/marketops` |
| `BOT_EQUITY_INTERVAL` / `BOT_CRYPTO_INTERVAL` | `1Day` / `1d` | **Must match `TECHNICAL_EQUITY_INTERVALS` / `TECHNICAL_CRYPTO_INTERVALS`**, or indicator lookups return nothing and every technical field shows `—` |
| `MARKET_CYCLE_SYMBOL` | `SPY` | Benchmark for the `/analyze` context panel and the VIX fallback. Shared with the `macro-analysis` worker |

### Caching and limits

| Variable | Default | Effect |
|---|---|---|
| `BOT_CACHE_ANALYZE_TTL` | `600` | Shared TTL for `/price`, `/signals`, `/analyze` |
| `BOT_CACHE_PRICE_TTL` | `300` | Loaded and passed to the builder but never read |
| `BOT_CACHE_DAILY_REPORT_TTL` | `3600` | Declared; the daily report path uses `use_cache=False` throughout |
| `BOT_NEWS_HEADLINES_LIMIT` | `5` | Headlines fetched per symbol; the formatter also caps display at 5 |
| `BOT_MARKET_OPS_ENABLE` | `true` | `false` ⇒ market-ops panels vanish from `/analyze` and the daily cards; `/marketops` shows its disabled status card |
| `BOT_MARKET_OPS_ATR_PCT_ELEVATED` | `3.0` | Sets the `atr_pct_elevated` flag |
| `BOT_MARKET_OPS_VOLUME_RATIO_ELEVATED` | `1.8` | Sets the `volume_vs_median_elevated` flag |
| `BOT_MARKET_OPS_VIX_LOW_MAX` / `_NORMAL_MAX` / `_ELEVATED_MAX` | `12` / `20` / `35` | Bands for the `/marketops` VIX label: low → normal → elevated → stress |

### Running it

```bash
make up          # TimescaleDB + Redis + ingestion + analyzer
make up-bot      # analyst-bot (profile: bot)
make log-bot     # follow bot logs
```

A healthy startup logs:

```
INFO  main           initialising DB pool
INFO  main           initialising Redis cache
INFO  scheduler      daily report scheduled: 0 7 * * *
INFO  scheduler      alert scan scheduled every 300s
INFO  bot.client     Bot ready — logged in as trading-analyst-bot#1234 (id=…)
INFO  bot.client     Slash commands synced — 10 registered: [...]
INFO  bot.client     sending startup daily report (delay=30s)
```

---

## 9. Reading an empty or grey card

Because the bot only formats what the database already holds, a sparse embed is a precise diagnostic. Work backwards from the symptom:

| What you see | What it means | Fix |
|---|---|---|
| Every technical field is `—` | No `technical_indicators` rows for that `(symbol, exchange, interval)` | Start `data-technical` + `technical-analysis`, and make `BOT_EQUITY_INTERVAL` match `TECHNICAL_EQUITY_INTERVALS` |
| Fundamentals panel absent on an equity | No `equity_fundamentals` rows with `period='derived'` | Start `data-fundamental`, then `fundamental-analysis`; allow a 24 h cycle |
| Tier 2 / Tier 3 / Qualitative panels absent | Those tiers need Finnhub XBRL data, which can take a full poll cycle to land | Wait one `DATA_FUNDAMENTAL_*_POLL_INTERVAL`; confirm `FINNHUB_API_KEY` |
| Grey **📉 Market cycle — data missing** | `mc_market_cycle` absent from `macro_derived` | Rebuild/restart `macro-analysis`; ensure the benchmark has ≥200 `1Day` bars in `equity_ohlcv`; check `MARKET_CYCLE_ENABLE` |
| Grey **🔗 Macro correlations — data missing** | `mc_macro_correlation` absent | Restart `macro-analysis`; set `MARKET_MACRO_CORR_ENABLE=true` |
| Grey **📚 Additional analysis — data missing** | `aa_reference_snapshot` absent | Restart `macro-analysis`; needs `macro_fred` DGS10, DCOILWTICO, VIXCLS |
| Monetary / Growth / Inflation / Global panels missing entirely | No `mp_*` / `gc_*` / `inf_*` / `gg_*` metrics at all | Check `FRED_API_KEY` and `FRED_SERIES_IDS`, then let `macro-analysis` run |
| Header VIX shows but market-ops VIX says "from TA benchmark" | `macro_fred` has no `VIXCLS`; both paths fell back to the TA indicator | Add `VIXCLS` to `FRED_SERIES_IDS` and run `data-equity` |
| Macro-intel calendars empty | `data-macro-intel` not running, or Finnhub returned `403` on your plan tier | Check `make log-services`; calendar endpoints are gated on some Finnhub tiers |
| `#alerts` floods every 5 minutes | Redis is unreachable, so `cache.exists()` degrades to `False` and no cooldown holds | Check the Redis connection in `make log-bot` |
| `#actions` is silent while `#alerts` is busy | Expected. Every rule resolved to `WATCH`/`HOLD_WATCH`, or `DISCORD_ACTIONS_CHANNEL_ID` is unset | Run `/action <symbol>` to see the suppressed evaluations and their confluence |
| Slash commands missing in Discord | Global registration takes up to an hour | Set `DISCORD_GUILD_ID` for instant per-server registration |
| `403 Forbidden` in logs on post | Missing **Send Messages** or **Embed Links** in that channel | Grant both — the bot posts embeds exclusively |
