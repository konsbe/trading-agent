# Bot Lexicon — Symbols, Emojis & Signal Meanings

A reference for every emoji, colour, label, and abbreviation you see in the bot's Discord output.

---

## Colour Coding (circle emojis)

| Symbol | Meaning |
|--------|---------|
| 🟢 | **Positive / bullish / strong** — good signal, favourable condition |
| 🟡 | **Neutral / average / fair** — no strong edge either way |
| 🔴 | **Negative / bearish / weak** — warning, unfavourable condition |
| ⚪ | **No data / not computed** — field exists but value couldn't be calculated |

These circles appear on every scored field. Example: `🟢 strong`, `🔴 weak`, `⚪ —`

---

## Status & Confirmation Symbols

| Symbol | Meaning |
|--------|---------|
| ✅ | **Confirmed / healthy / active** — signal has triggered or condition is met |
| ❌ | **Failed / offline / missing** — condition not met or service down |
| ⚠️ | **Warning / elevated risk** — not critical, but worth attention |
| 😱 | **Extreme fear** — VIX above 35, market in panic mode |
| 💤 | **Complacency** — VIX below 12, market is overconfident / too calm |
| —  | **No value** — indicator returned nothing meaningful for this bar |

---

## Price Direction

| Symbol | Meaning |
|--------|---------|
| 📈 | Price went **up** vs the previous bar |
| 📉 | Price went **down** vs the previous bar |
| ↔️ | Price is moving **sideways** — no clear direction |

These appear on the price title and the trend field.

---

## Commands Reference

| Command | What it does |
|---------|-------------|
| `/price symbol:AAPL asset_type:equity` | Latest OHLCV bar for one symbol (price, open, high, low, volume) |
| `/signals symbol:AAPL asset_type:equity` | Fast one-embed snapshot of the most actionable signals |
| `/analyze symbol:AAPL asset_type:equity` | Full deep-dive: 4 panels — price, technical, fundamentals, news |
| `/report` | Triggers the daily market report on demand (same as the 07:00 scheduled job) |
| `/status` | Bot health — DB, Redis, scheduler, configured symbols |
| `/ping` | Bot latency in milliseconds |

`asset_type` dropdown: `equity` for stocks and ETFs, `crypto` for crypto pairs. Defaults to `equity`.

---

## Technical Analysis Fields

### RSI 14 (Relative Strength Index)

Measures momentum — how fast price is moving.

| Value | Label | Meaning |
|-------|-------|---------|
| < 30 | 🔴 oversold | Price has fallen too fast. Potential bounce candidate. |
| 30–70 | ✅ | Normal range. No momentum extreme. |
| > 70 | 🔴 overbought | Price has risen too fast. Potential pullback candidate. |

Example: `RSI 14: 48.8 ✅` → neutral momentum.

---

### MACD (12/26/9) — Moving Average Convergence Divergence

Measures trend momentum shifts.

| Field | Meaning |
|-------|---------|
| `hist -1.110` | Histogram value — negative = bearish momentum, positive = bullish |
| `🟢 bullish cross` | MACD line just crossed above signal line → momentum turning up |
| `🔴 bearish cross` | MACD line just crossed below signal line → momentum turning down |

Only the cross labels appear if currently firing. Otherwise just the histogram value.

---

### ADX 14 (Average Directional Index)

Measures how strong the current trend is (not the direction).

| Value | Meaning |
|-------|---------|
| < 20 | Weak or no trend — market is ranging, signals are less reliable |
| 20–25 | Trend developing |
| > 25 | Strong trend — directional signals are more reliable |
| > 40 | Very strong trend |

Example: `ADX 14: 32.7` → moderately strong trend.

---

### Trend

Calculated from the slope of the EMA (Exponential Moving Average).

| Display | Meaning |
|---------|---------|
| `📈 uptrend (slope +0.44%)` | Price is consistently moving up — EMA sloping upward |
| `📉 downtrend (slope -0.44%)` | Price is consistently falling — EMA sloping down |
| `↔️ sideways (slope -0.05%)` | EMA is flat — no clear direction |
| `— down` | Trend computed but label suppressed (slope too shallow to classify confidently) |

The slope % is the EMA's rate of change per bar relative to current price.

---

### MA Cross (Moving Average Cross)

Compares the 50-period and 200-period Simple Moving Averages.

| Display | Meaning |
|---------|---------|
| `🟢 Golden cross` | 50 SMA crossed **above** 200 SMA — long-term bullish signal |
| `🔴 Death cross` | 50 SMA crossed **below** 200 SMA — long-term bearish signal |
| `—` | No cross recently — MAs are aligned without a recent crossover |

---

### ATR 14 (Average True Range)

The average daily price swing in dollars over the last 14 bars.

Example: `ATR 14: 5.58` → AAPL typically moves ±$5.58 per day. Useful for setting stop-losses.

Higher ATR = more volatility. Lower ATR = tighter price action (often precedes a squeeze).

---

### BB Squeeze (Bollinger Band Squeeze)

| Display | Meaning |
|---------|---------|
| `🔴 ACTIVE — breakout expected` | Bollinger Bands (volatility envelope) are now **inside** Keltner Channels — volatility has compressed to an unusually tight range. A big move is building. Direction unknown until the breakout occurs. |
| `—` | No squeeze — bands are at normal width |

This is one of the highest-value alerts — it often precedes sharp directional moves.

---

### VIX Regime (Market Fear Index)

The VIX (CBOE Volatility Index) measures expected market volatility. Sourced from FRED (`VIXCLS`). Only shown for equity symbols.

| Display | VIX Range | Meaning |
|---------|-----------|---------|
| `😱 extreme_fear` | > 35 | Market is in panic. High risk, but also high opportunity for contrarian longs. |
| `⚠️ elevated` | 20–35 | Investors are hedging more than usual. Risk-off environment. |
| `✅ normal` | 12–20 | Calm market. Typical conditions. |
| `💤 complacency` | < 12 | Investors are overconfident. Market may be vulnerable to a surprise shock. |

Example: `VIX Regime: ⚠️ elevated (VIX 25.2)`

---

### Pivots (Classic Pivot Points)

Key price levels derived from the prior day's OHLC. Used as support/resistance reference.

| Label | Full name | Meaning |
|-------|-----------|---------|
| `PP` | Pivot Point | The central level. Acts as magnet — price often oscillates around it. |
| `R1` | Resistance 1 | First resistance above PP. Common intraday ceiling. |
| `S1` | Support 1 | First support below PP. Common intraday floor. |

Example: `PP $252.12 | R1 $257.15 | S1 $248.77`
If price is at $254.81 — it's between PP and R1, closer to resistance.

---

### SMC — Smart Money Concepts

Three signals derived from order flow analysis.

| Label | Full name | Meaning |
|-------|-----------|---------|
| `FVGs: 2 active` | Fair Value Gaps | Price moved so fast it left a gap in the chart. These gaps act like magnets — price tends to revisit them. "Active" means not yet filled. |
| `OBs: 7 active` | Order Blocks | The last opposing candle before a strong impulsive move. Marks where institutional buyers/sellers entered. Price often returns to test these zones. |
| `Liq sweeps: 5` | Liquidity Sweeps | Price briefly poked above a prior high (or below a prior low) and reversed within the same bar. Interpreted as institutions hunting retail stop-losses before moving in the intended direction. Count = how many occurred in recent bars. |

---

### Chart Patterns

| Display | Meaning | Actionable? |
|---------|---------|-------------|
| `🔴 H&S ✅ confirmed` | **Head & Shoulders** — bearish reversal. Price formed 3 peaks (left shoulder, head, right shoulder). `✅ confirmed` means price has **already broken below the neckline** → bearish signal is active. | Yes — price already broke down |
| `🔴 H&S (unconfirmed)` | Pattern is forming but neckline not yet broken — watch, don't act. | No — wait for confirmation |
| `🟢 Inv. H&S ✅ confirmed` | **Inverse Head & Shoulders** — bullish reversal. 3 troughs with the middle lowest. `✅ confirmed` means price broke above the neckline → bullish signal active. | Yes |
| `🟢 Inv. H&S (unconfirmed)` | Pattern forming, neckline not yet broken. | No — wait |
| `△ ascending` | Ascending triangle — higher lows + flat resistance. Typically bullish breakout setup. | Breakout direction shown if already broken |
| `△ descending` | Descending triangle — lower highs + flat support. Typically bearish. | |
| `△ symmetrical` | Symmetrical triangle — both highs and lows converging. Direction unclear until breakout. | |
| `🟢 Bull flag` | Strong up-move followed by a tight sideways/downward consolidation. Breakout is typically upward. | |
| `🔴 Bear flag` | Strong down-move followed by a tight bounce. Breakout is typically downward. | |

---

## Fundamental Analysis Fields

### Composite Score & Tier

The overall fundamental rating of the company, scored from **–1.0 to +1.0** based on all sub-metrics.

| Display | Score | Meaning |
|---------|-------|---------|
| `🟢 strong` | > 0 | Multiple strong signals — solid fundamentals |
| `🟡 neutral` | ~0 | Mixed signals — some good, some weak |
| `🔴 weak` | < 0 | Multiple red flags in fundamentals |

Example: `🟢 strong (0.80)` → high-conviction bullish fundamental rating.

---

### EPS Strength

Earnings Per Share trend — is the company making more or less profit per share over time?

| Display | Growth rate (YoY) | Meaning |
|---------|-------------------|---------|
| `🟢 strong` | > 15% | EPS growing strongly |
| `🟡 neutral` | 5–15% | Moderate EPS growth |
| `🔴 weak` | < 5% or negative | EPS stagnating or declining |

Configurable via `FUNDAMENTAL_EPS_GROWTH_STRONG` (default 15) and `FUNDAMENTAL_EPS_GROWTH_WEAK` (default 5).

---

### Revenue

Revenue growth trend — is the company growing its top-line sales?

| Display | Growth rate (YoY) | Meaning |
|---------|-------------------|---------|
| `🟢 strong` | > 10% | Top-line sales growing well |
| `🟡 neutral` | 2–10% | Moderate revenue growth |
| `🔴 weak` | < 2% or negative | Revenue stagnating or shrinking |

Configurable via `FUNDAMENTAL_REV_GROWTH_STRONG` (default 10) and `FUNDAMENTAL_REV_GROWTH_WEAK` (default 2).

---

### P/E vs 5Y (Price-to-Earnings vs 5-Year Mean)

Is the stock cheap or expensive relative to its **own history**?

| Display | Meaning |
|---------|---------|
| `🟢 cheap_vs_history` | Current P/E is significantly **below** its own 5-year average — historically cheap |
| `🟡 fair_vs_history` | P/E is near its 5-year mean |
| `🔴 expensive_vs_history` | P/E is significantly **above** its own 5-year average — historically expensive |
| `⚪ expensive` | P/E is high with no 5-year baseline to compare against |
| `⚪ growth_fair` | P/E is high but justified by the growth rate — not alarming |
| `🔴 loss_making` | Company has negative earnings — P/E is undefined |

---

### FCF Yield (Free Cash Flow Yield)

How much free cash the company generates relative to its market cap. Higher = more cash for dividends, buybacks, or reinvestment.

| Display | FCF Yield | Meaning |
|---------|-----------|---------|
| `🟢 attractive` | ≥ 5% | Company generates strong cash relative to its price — room for buybacks/dividends |
| `🟡 fair` | 2–5% | Moderate free cash flow |
| `🔴 avoid` | < 2% or negative | Very little or no free cash flow |

Configurable via `FUNDAMENTAL_FCF_YIELD_ATTRACTIVE` (default 5) and `FUNDAMENTAL_FCF_YIELD_FAIR` (default 2).

---

### Gross Margin & Net Margin

What percentage of revenue the company keeps as profit.

| Field | Formula | Example |
|-------|---------|---------|
| Gross Margin | (Revenue − Cost of Goods) / Revenue | `🟢 +47.33%` — Apple keeps 47¢ of every $1 in revenue before operating costs |
| Net Margin | Net Income / Revenue | `🟢 +27.04%` — Apple keeps 27¢ of every $1 after ALL expenses |

**Gross Margin tiers** (configurable via `FUNDAMENTAL_GROSS_MARGIN_MOAT` / `FUNDAMENTAL_GROSS_MARGIN_AVG`):

| Tier | Threshold | Meaning |
|------|-----------|---------|
| 🟢 `strong_moat` | ≥ 40% | Exceptional pricing power — hard to compete with |
| 🟡 `average` | 20–40% | Typical for most industries |
| 🔴 `margin_pressure` | < 20% | Thin margins — cost-sensitive business |

**Net Margin tiers** (configurable via `FUNDAMENTAL_NET_MARGIN_STRONG` / `FUNDAMENTAL_NET_MARGIN_AVG`):

| Tier | Threshold | Meaning |
|------|-----------|---------|
| 🟢 `strong_moat` | ≥ 15% | Highly profitable after all expenses |
| 🟡 `average` | 5–15% | Acceptable profitability |
| 🔴 `margin_pressure` | < 5% | Low net profitability |

The second emoji after the % is the **trend**:

| Display | Meaning |
|---------|---------|
| `📈` | Margin expanding — company becoming more profitable |
| `➡️` | Margin stable |
| `📉` | Margin compressing — profitability eroding |
| `⚪` | Trend not yet computable (insufficient history) |

Example: `🟢 +68.59% ⚪` → MSFT has 68.59% gross margin, trend not computed yet.

---

### PEG (Price/Earnings-to-Growth)

Adjusts the P/E ratio for the company's growth rate. PEG = P/E ÷ EPS growth rate.

| Display | PEG range | Meaning |
|---------|-----------|---------|
| `🟢 undervalued_growth` | < 1 | You're paying less than the growth rate justifies — value + growth |
| `🟡 fairly_valued_growth` | 1–2 | Growth is priced in, but not excessively |
| `🔴 expensive_growth` | > 2 | Paying a heavy premium even accounting for growth rate |

Configurable via `FUNDAMENTAL_PEG_UNDERVALUED` (default 1) and `FUNDAMENTAL_PEG_FAIR` (default 2).

---

### Earnings Surprise

How much EPS beat or missed analyst consensus estimates, averaged over recent quarters.

| Display | Meaning |
|---------|---------|
| `🟢 +3.33% (beat)` | Company consistently beats estimates by 3.33% on average |
| `🟡 inline` | Results roughly in line with expectations |
| `🔴 (miss)` | Company has been missing estimates |

---

### TTM P/E

**Trailing Twelve Month Price-to-Earnings ratio.** How much investors pay per $1 of the last 12 months of earnings.

Example: `TTM P/E: 31.6` → investors pay $31.60 for every $1 AAPL earns. High but normal for large-cap tech.

---

### Market Cap

Total market value of the company (price × shares outstanding). Converted from Finnhub's millions-unit to human-readable format.

| Display | Scale |
|---------|-------|
| `$3.73T` | Trillions — mega-cap (AAPL, MSFT) |
| `$2.70T` | Trillions |
| `$140B` | Billions — large-cap |
| `$8B` | Billions — mid-cap |

---

## Alert Types

Alerts are posted to the `#alerts` channel automatically every 5 minutes (configurable). Each alert has a severity level.

| Alert | Severity | Trigger | Meaning |
|-------|----------|---------|---------|
| `rsi_oversold` | ⚠️ warning | RSI < 30 | Price has fallen sharply — potential bounce |
| `rsi_overbought` | ⚠️ warning | RSI > 70 | Price has risen sharply — potential pullback |
| `bb_squeeze` | ℹ️ info | BB inside Keltner | Volatility coiling — breakout incoming |
| `vix_elevated` | ⚠️ warning | VIX > 25 | Market fear rising — risk-off environment |
| `fa_tier_flip` | ⚠️ warning / ℹ️ info | Composite tier changed | Company's fundamental rating changed tier (e.g. neutral → weak) |
| `liquidity_sweep` | ℹ️ info | Sweep detected | Institutions hunted stop-losses — potential directional move |

**Cooldown**: The same alert for the same symbol won't repeat for 4 hours (configurable via `BOT_ALERT_COOLDOWN_SECS`).

---

## Macro Fields (Daily Report Header)

| Field | Source | Meaning |
|-------|--------|---------|
| `VIX: 25.2` | FRED `VIXCLS` | Market fear index — see VIX Regime table above |
| `10Y: 4.35%` | FRED `DGS10` | 10-year US Treasury yield — rising yield = tighter financial conditions |
| `EUR/USD: 1.1520` | FRED `DEXUSEU` | Euro vs US Dollar exchange rate |

---

## ETF / SPY Note

SPY is an ETF (Exchange-Traded Fund) that tracks the S&P 500 index. ETFs have no individual earnings, P/E ratio, or margins — all fundamental fields will show `⚪ —`. Only price and technical signals apply.
