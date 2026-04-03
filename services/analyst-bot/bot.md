# Bot Lexicon — Symbols, Emojis & Signal Meanings

A reference for every emoji, colour, label, and abbreviation you see in the bot's output.

## Colour Coding

🟢 **Positive / bullish / strong** — good signal, favourable condition
🟡 **Neutral / average / fair** — no strong edge either way
🔴 **Negative / bearish / weak** — warning, unfavourable condition
⚪ **No data / not computed** — field exists but value couldn't be calculated

These circles appear on every scored field. Example: `🟢 strong`, `🔴 weak`, `⚪ —`

## Status & Confirmation Symbols

✅ **Confirmed / healthy / active** — signal has triggered or condition is met
❌ **Failed / offline / missing** — condition not met or service down
⚠️ **Warning / elevated risk** — not critical, but worth attention
😱 **Extreme fear** — VIX above 35, market in panic mode
💤 **Complacency** — VIX below 12, market is overconfident / too calm
`—` **No value** — indicator returned nothing meaningful for this bar

## Price Direction

📈 Price went **up** vs the previous bar
📉 Price went **down** vs the previous bar
↔️ Price is moving **sideways** — no clear direction

These appear on the price title and the trend field.

## Commands Reference

`/price symbol:AAPL asset_type:equity` — Latest OHLCV bar (open, high, low, close, volume)
`/signals symbol:AAPL asset_type:equity` — Fast one-embed snapshot of the most actionable signals
`/analyze symbol:AAPL asset_type:equity` — Full deep-dive: price → **Context vs benchmark** → **Market ops** (VIX regime + ATR% / volume vs median) → technical → Tier 1 fundamentals → 🏦 Tier 2 balance sheet → 🔍 Tier 3 deep context → news
`/marketops [symbol] [asset_type]` — **Module 5** snapshot: VIX regime + HTML automation coverage; optional symbol adds ATR% and volume-vs-median context. **Not** entry/exit prices. With **default `asset_type:equity`**, symbols in **`BOT_CRYPTO_SYMBOLS`** or ending in **`USDT`/`USDC`/`BUSD`/`PERP`** are treated as **crypto** so pairs like **BTCUSDT** get the right execution note and OHLCV table. **Typo:** the command is **`/marketops`**, not `/matketops`.
`/report` — Triggers the daily market report on demand (same as the 07:00 scheduled job)
`/dictionary` — Sends this glossary as paginated Discord embeds
`/status` — Bot health: DB ✅/❌, Redis ✅/❌, scheduler jobs, configured symbols; macro-intel table row counts; latest **`mc_market_cycle`** / **`mc_macro_correlation`** / **`aa_reference_snapshot`** (`source=macro_analysis`) and **`mo_reference_snapshot`** (`source=market_operations`) timestamps in **`macro_derived`**
`/ping` — Bot latency in milliseconds

`asset_type` dropdown: `equity` for stocks/ETFs, `crypto` for crypto pairs. Defaults to `equity`.

## Technical Analysis Fields

### RSI 14 (Relative Strength Index)

Measures momentum — how fast price is moving.

`< 30` → 🔴 **oversold** — Price has fallen too fast. Potential bounce candidate.
`30–70` → ✅ Normal range. No momentum extreme.
`> 70` → 🔴 **overbought** — Price has risen too fast. Potential pullback candidate.

Example: `RSI 14: 48.8 ✅` → neutral momentum.

### MACD (12/26/9)

Measures trend momentum shifts.

`hist -1.110` — Histogram value. Negative = bearish momentum, positive = bullish.
`🟢 bullish cross` — MACD line crossed above signal line → momentum turning up
`🔴 bearish cross` — MACD line crossed below signal line → momentum turning down

Cross labels only appear if currently firing. Otherwise just the histogram value.

### ADX 14 (Average Directional Index)

Measures how **strong** the current trend is (not the direction).

`< 20` — Weak or no trend. Market is ranging, signals are less reliable.
`20–25` — Trend developing.
`> 25` — Strong trend. Directional signals are more reliable.
`> 40` — Very strong trend.

Example: `ADX 14: 32.7` → moderately strong trend.

### Trend

Calculated from the slope of the EMA (Exponential Moving Average).

`📈 uptrend (slope +0.44%)` — EMA sloping upward, price consistently rising
`📉 downtrend (slope -0.44%)` — EMA sloping down, price consistently falling
`↔️ sideways (slope -0.05%)` — EMA flat, no clear direction
`— down` — Slope too shallow to classify confidently

The slope % is the EMA's rate of change per bar relative to current price.

### MA Cross (Moving Average Cross)

Compares the 50-period and 200-period Simple Moving Averages.

`🟢 Golden cross` — 50 SMA crossed **above** 200 SMA → long-term bullish signal
`🔴 Death cross` — 50 SMA crossed **below** 200 SMA → long-term bearish signal
`—` — No cross recently, MAs aligned without a recent crossover

### ATR 14 (Average True Range)

The average daily price swing in dollars over the last 14 bars.

Example: `ATR 14: 5.58` → AAPL typically moves ±$5.58 per day. Useful for stop-losses.

Higher ATR = more volatility. Lower ATR = tighter price action (often precedes a squeeze).

### BB Squeeze (Bollinger Band Squeeze)

`🔴 ACTIVE — breakout expected` — Bollinger Bands are now **inside** Keltner Channels. Volatility has compressed to an unusually tight range. A big move is building. Direction unknown until the breakout occurs.
`—` — No squeeze, bands are at normal width.

One of the highest-value alerts — it often precedes sharp directional moves.

### VIX Regime (Market Fear Index)

The VIX (CBOE Volatility Index) measures expected market volatility. Sourced from FRED (`VIXCLS`). Only shown for equity symbols.

`😱 extreme_fear` (VIX > 35) — Market is in panic. High risk, but also opportunity for contrarian longs.
`⚠️ elevated` (VIX 20–35) — Investors are hedging more than usual. Risk-off environment.
`✅ normal` (VIX 12–20) — Calm market. Typical conditions.
`💤 complacency` (VIX < 12) — Investors are overconfident. Vulnerable to a surprise shock.

Thresholds configurable via `TECHNICAL_VIX_FEAR_THRESHOLD`, `TECHNICAL_VIX_ELEVATED_THRESHOLD`, `TECHNICAL_VIX_COMPLACENCY_THRESHOLD`.

Example: `VIX Regime: ⚠️ elevated (VIX 25.2)`

**Daily symbol summaries:** the per-symbol **footer** uses this TA **`vix_regime`** (e.g. elevated when VIX is 20–35 with default thresholds). The **Market ops** line on the same embed uses **Module 5** bands (`BOT_MARKET_OPS_VIX_*`, default “normal” while VIX &lt; 25). So one print of VIX can show **normal** in Market ops and **elevated** in the footer — both are intentional, not a data bug.

### Pivots (Classic Pivot Points)

Key price levels derived from the prior day's OHLC. Used as support/resistance reference.

`PP` (Pivot Point) — Central level. Price often oscillates around it.
`R1` (Resistance 1) — First resistance above PP. Common intraday ceiling.
`S1` (Support 1) — First support below PP. Common intraday floor.

Example: `PP $252.12 | R1 $257.15 | S1 $248.77`
If price is at $254.81 — it's between PP and R1, closer to resistance.

### SMC — Smart Money Concepts

`FVGs: 2 active` (Fair Value Gaps) — Price moved so fast it left a gap. These gaps act like magnets — price tends to revisit them. "Active" = not yet filled.
`OBs: 7 active` (Order Blocks) — Last opposing candle before a strong impulse move. Marks where institutions entered. Price often returns to test these zones.
`Liq sweeps: 5` (Liquidity Sweeps) — Price briefly broke a prior high/low and reversed within the same bar. Institutions hunting retail stop-losses. Count = how many occurred in recent bars.

### Chart Patterns

`🔴 H&S ✅ confirmed` — **Head & Shoulders** (bearish reversal). 3 peaks. `✅ confirmed` = price already broke below the neckline → signal active.
`🔴 H&S (unconfirmed)` — Pattern forming but neckline not yet broken. Watch, don't act.
`🟢 Inv. H&S ✅ confirmed` — **Inverse H&S** (bullish reversal). 3 troughs. `✅ confirmed` = price broke above neckline → signal active.
`🟢 Inv. H&S (unconfirmed)` — Pattern forming, neckline not yet broken.
`△ ascending` — Higher lows + flat resistance. Typically bullish breakout setup.
`△ descending` — Lower highs + flat support. Typically bearish.
`△ symmetrical` — Highs and lows converging. Direction unclear until breakout.
`🟢 Bull flag` — Strong up-move followed by tight consolidation. Breakout typically upward.
`🔴 Bear flag` — Strong down-move followed by a tight bounce. Breakout typically downward.

## Fundamental Analysis Fields

### Composite Score & Tier

Overall fundamental rating, scored **–1.0 to +1.0** based on all sub-metrics.

`🟢 strong` (score > 0) — Multiple strong signals, solid fundamentals
`🟡 neutral` (score ~0) — Mixed signals, some good, some weak
`🔴 weak` (score < 0) — Multiple red flags in fundamentals

Example: `🟢 strong (0.80)` → high-conviction bullish fundamental rating.

### EPS Strength

Earnings Per Share trend — is the company making more or less profit per share over time?

`🟢 strong` (> 15% YoY) — EPS growing strongly
`🟡 neutral` (5–15% YoY) — Moderate EPS growth
`🔴 weak` (< 5% or negative) — EPS stagnating or declining

Configurable via `FUNDAMENTAL_EPS_GROWTH_STRONG` (default 15) and `FUNDAMENTAL_EPS_GROWTH_WEAK` (default 5).

### Revenue

Revenue growth trend — is the company growing its top-line sales?

`🟢 strong` (> 10% YoY) — Top-line sales growing well
`🟡 neutral` (2–10% YoY) — Moderate revenue growth
`🔴 weak` (< 2% or negative) — Revenue stagnating or shrinking

Configurable via `FUNDAMENTAL_REV_GROWTH_STRONG` (default 10) and `FUNDAMENTAL_REV_GROWTH_WEAK` (default 2).

### P/E vs 5Y (Price-to-Earnings vs 5-Year Mean)

Is the stock cheap or expensive relative to its **own history**?

`🟢 cheap_vs_history` — Current P/E is significantly below its own 5-year average
`🟡 fair_vs_history` — P/E is near its 5-year mean
`🔴 expensive_vs_history` — P/E is significantly above its own 5-year average
`⚪ expensive` — P/E is high with no 5-year baseline to compare against
`⚪ growth_fair` — P/E is high but justified by the growth rate
`🔴 loss_making` — Company has negative earnings, P/E is undefined

### FCF Yield (Free Cash Flow Yield)

How much free cash the company generates relative to its market cap.

`🟢 attractive` (≥ 5%) — Strong cash generation relative to price, room for buybacks/dividends
`🟡 fair` (2–5%) — Moderate free cash flow
`🔴 avoid` (< 2% or negative) — Very little or no free cash flow

Configurable via `FUNDAMENTAL_FCF_YIELD_ATTRACTIVE` (default 5) and `FUNDAMENTAL_FCF_YIELD_FAIR` (default 2).

### Gross Margin & Net Margin

What percentage of revenue the company keeps as profit.

**Gross Margin** = (Revenue − Cost of Goods) / Revenue
Example: `🟢 +47.33%` → Apple keeps 47¢ of every $1 before operating costs.

**Net Margin** = Net Income / Revenue
Example: `🟢 +27.04%` → Apple keeps 27¢ of every $1 after ALL expenses.

**Gross Margin tiers** (configurable via `FUNDAMENTAL_GROSS_MARGIN_MOAT` / `FUNDAMENTAL_GROSS_MARGIN_AVG`):
`🟢 strong_moat` (≥ 40%) — Exceptional pricing power, hard to compete with
`🟡 average` (20–40%) — Typical for most industries
`🔴 margin_pressure` (< 20%) — Thin margins, cost-sensitive business

**Net Margin tiers** (configurable via `FUNDAMENTAL_NET_MARGIN_STRONG` / `FUNDAMENTAL_NET_MARGIN_AVG`):
`🟢 strong_moat` (≥ 15%) — Highly profitable after all expenses
`🟡 average` (5–15%) — Acceptable profitability
`🔴 margin_pressure` (< 5%) — Low net profitability

**Trend arrow** (second emoji after the %):
`📈` — Margin expanding, company becoming more profitable
`➡️` — Margin stable
`📉` — Margin compressing, profitability eroding
`⚪` — Trend not yet computable (insufficient history)

Example: `🟢 +68.59% ⚪` → MSFT has 68.59% gross margin, trend not computed yet.

### PEG (Price/Earnings-to-Growth)

Adjusts the P/E ratio for the company's growth rate. PEG = P/E ÷ EPS growth rate.

`🟢 undervalued_growth` (PEG < 1) — Paying less than the growth rate justifies, value + growth
`🟡 fairly_valued_growth` (PEG 1–2) — Growth is priced in, but not excessively
`🔴 expensive_growth` (PEG > 2) — Paying a heavy premium even accounting for growth

Configurable via `FUNDAMENTAL_PEG_UNDERVALUED` (default 1) and `FUNDAMENTAL_PEG_FAIR` (default 2).

### Earnings Surprise

How much EPS beat or missed analyst consensus estimates, averaged over recent quarters.

`🟢 +3.33% (beat)` — Company consistently beats estimates
`🟡 inline` — Results roughly in line with expectations
`🔴 (miss)` — Company has been missing estimates

### TTM P/E

**Trailing Twelve Month Price-to-Earnings ratio.** How much investors pay per $1 of last 12 months' earnings.

Example: `TTM P/E: 31.6` → investors pay $31.60 for every $1 AAPL earns.

### Market Cap

Total market value (price × shares outstanding). Formatted from Finnhub's millions-unit.

`$3.73T` — Trillions (mega-cap: AAPL, MSFT)
`$140B` — Billions (large-cap)
`$8B` — Billions (mid-cap)

## Balance Sheet Analysis (Tier 2)

Shown as a separate **🏦 Balance Sheet** embed in `/analyze` for equity symbols. These metrics assess financial health, leverage, and capital efficiency. Not all fields appear for every symbol — they appear only once data has been computed by the analyzer.

The **Balance Sheet health score** in the embed title combines ROE, D/E, and Current Ratio into a single [-1 … +1] score.

`🟢 healthy` — Multiple strong balance-sheet signals
`🟡 neutral` — Mixed signals
`🔴 stressed` — Multiple red flags in leverage or liquidity

---

### ROE (Return on Equity)

How efficiently the company generates profit from shareholders' equity. Sustained ROE > 15% for 5+ years is the Buffett-style moat signal.

`🟢 excellent` > 15% — Strong moat. Company creates significant value per dollar of equity.
`🟡 adequate` 8–15% — Acceptable but not exceptional capital efficiency.
`🔴 destroying_value` < 8% — Capital allocation is eroding shareholder value.

Configurable via `FUNDAMENTAL_ROE_EXCELLENT` (default 15) and `FUNDAMENTAL_ROE_ADEQUATE` (default 8).

The `ROA` (Return on Assets) shown inline is informational: > 10% high efficiency, 5–10% moderate, < 5% low.

### ROIC (Return on Invested Capital)

ROIC measures how efficiently the company deploys **all** invested capital (equity + debt). It is a stricter test than ROE because it cannot be inflated by leverage. ROIC consistently above the cost of capital (typically ~10%) is the hallmark of a compounding machine.

**How it's computed (from XBRL SEC filings — free tier):**
`NOPAT = Operating Income × (1 − effective tax rate)` (annualised from latest quarter × 4)
`Invested Capital = Total Assets − Current Liabilities`
`ROIC = NOPAT / Invested Capital × 100`

Fallback: Finnhub `roic5Y` (5-year average) when XBRL data is unavailable.

`🟢 moat_quality` > 15% — Durable economic moat. Company earns well above its cost of capital.
`🟡 adequate_roic` 8–15% — Acceptable. Value creation present but not exceptional.
`🔴 low_roic` < 8% — Earning near or below cost of capital. Capital deployment is inefficient.

The `source` shown in parentheses is `xbrl_computed` (live quarterly data) or `finnhub_5y_avg` (historical average). XBRL is preferred as it reflects the most recent filing.

Example: `🟢 +24.1% (moat_quality)` — AAPL's NOPAT divided by invested capital = 24.1%, well above its ~10% cost of capital.

Configurable via `FUNDAMENTAL_ROIC_EXCELLENT` (default 15) and `FUNDAMENTAL_ROIC_ADEQUATE` (default 8).

### Debt/Equity (D/E Ratio)

How much debt finances the business versus equity. Rising interest rates make high-debt companies more vulnerable — each refinancing hits earnings harder.

`🟢 conservative` D/E < 1.0 — Low leverage. Strong financial position.
`🟡 manageable` D/E 1–2× — Acceptable. Monitor debt maturity schedule.
`🔴 high_leverage` D/E > 2× — Demands scrutiny. Industry context essential — utilities safely operate at 3–4×.

Configurable via `FUNDAMENTAL_DE_CONSERVATIVE` (default 1.0) and `FUNDAMENTAL_DE_MANAGEABLE` (default 2.0).

### Net Debt / EBITDA

Cleaner leverage metric than D/E — accounts for cash holdings. Proxy computed as: (Total Debt − Cash) ÷ (Operating Income × 4).

`🟢 net_cash` — Company holds more cash than total debt. Ultra-safe.
`🟢 conservative` < 2× — Low leverage relative to earnings power.
`🟡 manageable` 2–4× — Monitor, especially if interest rates are rising.
`🔴 high_risk` > 4× — Vulnerable in an economic slowdown or rate-rise environment.

Configurable via `FUNDAMENTAL_NET_DEBT_EBITDA_LOW` (default 2) and `FUNDAMENTAL_NET_DEBT_EBITDA_HIGH` (default 4).

### EV/EBITDA

Capital-structure neutral valuation — removes the effect of different debt levels, tax rates, and depreciation choices. **Always compare within sector.** Tech typically 20–30×, industrials 10–15×, utilities 8–12×.

`🟢 value_territory` < 10× — Potentially undervalued relative to earnings.
`🟡 fairly_valued` 10–20× — Standard valuation range for most industries.
`🔴 growth_premium_required` > 20× — Requires strong, sustained earnings growth to justify.

Configurable via `FUNDAMENTAL_EV_EBITDA_VALUE` (default 10) and `FUNDAMENTAL_EV_EBITDA_FAIR` (default 20).

### Current Ratio

Short-term liquidity — can the company pay its near-term obligations? Current Ratio = Current Assets ÷ Current Liabilities.

`🟢 safe` > 1.5 — Comfortable liquidity buffer.
`🟡 monitor` 1.0–1.5 — Adequate but watch closely if debt maturities are approaching.
`🔴 liquidity_risk` < 1.0 — Short-term liabilities exceed liquid assets. Not always fatal but demands explanation.

The `Quick` ratio shown inline is stricter — it excludes inventory. > 1.0 adequate, 0.7–1.0 monitor, < 0.7 risk.

Configurable via `FUNDAMENTAL_CURRENT_RATIO_SAFE` (default 1.5) and `FUNDAMENTAL_CURRENT_RATIO_MONITOR` (default 1.0).

### Price/Book (P/B)

Market price vs. net asset value. Most relevant for banks, insurers, and asset-heavy industries. **Tech companies with heavy intangibles make P/B less meaningful — use EV/EBITDA instead.**

`🟢 value_signal` P/B < 1.5 — Market values company near (or below) its book assets.
`⚪ fair` P/B 1.5–5× — Standard range for most industries.
`🔴 limited_safety_margin` P/B > 5× — Little asset-backed downside protection.

Configurable via `FUNDAMENTAL_PB_VALUE` (default 1.5) and `FUNDAMENTAL_PB_EXPENSIVE` (default 5.0).

### Dividend Yield

Annual dividend as % of share price. Shown only if the company pays a dividend.

`🟢 sustainable_income` Yield 2–6%, Payout < 60% — Generous income with room for maintenance.
`🟡 moderate_yield` 2–6%, payout not assessed — Moderate income.
`🟡 verify_payout` Yield > 6% — High yield; verify payout ratio before investing.
`🔴 cut_risk` Payout > 80% — Dividend may be cut if earnings dip slightly.
`⚪ no_dividend` < 2% or no dividend — Growth company or dividend suspended.

Configurable via `FUNDAMENTAL_DIVIDEND_YIELD_MIN/HIGH` and `FUNDAMENTAL_PAYOUT_RATIO_SAFE/DANGER`.

### CapEx Intensity

Capital expenditure as % of revenue. Asset-light businesses (SaaS, brands) keep CapEx < 5% and convert most of their earnings into free cash. Capital-intensive industries (semiconductors, airlines, mining) must constantly reinvest.

`🟢 asset_light` < 5% of revenue — High FCF conversion potential.
`🟡 moderate_intensity` 5–20% — Typical for manufacturing, consumer.
`🔴 capital_intensive` > 20% — Heavy reinvestment required; FCF constrained.

Configurable via `FUNDAMENTAL_CAPEX_INTENSITY_LOW` (default 5) and `FUNDAMENTAL_CAPEX_INTENSITY_HIGH` (default 20).

---

## Deep Context (Tier 3)

Shown as a **🔍 Deep Context** embed in `/analyze` for equity symbols. These metrics provide important context for decision-making but require more interpretation than Tier 1/2 signals. They appear only once XBRL data has been ingested and the analyzer has run a full cycle.

> DCF values are directional sanity checks — never treat them as precise targets.

---

### Share Count Trend (Rank 13)

Is the company shrinking or growing its share count over time? Net buybacks boost EPS per share without improving underlying earnings.

`🟢 buyback` — Share count declining > 2%/yr. Active buyback program returns cash to shareholders.
`🟡 flat` — Share count stable ±2%/yr. Neutral.
`🔴 dilution_risk` — Share count growing > 3%/yr. Company is issuing stock (acquisitions, SBC, capital raises).

Configurable via `FUNDAMENTAL_SHARE_DECLINE_BUYBACK` (default 2) and `FUNDAMENTAL_SHARE_GROWTH_DILUTION` (default 3).

### DCF Margin of Safety (Rank 14)

Simplified 5-year Discounted Cash Flow model. Computes the intrinsic value of the business from FCF growth projections and compares it to the current market cap.

**Value shown**: `price = X% of intrinsic` — where 100% means priced exactly at intrinsic value.

`🟢 strong_margin_of_safety` Price < 70% of DCF — 30%+ discount to intrinsic value. Strong buy signal.
`🟡 fairly_valued` Price 70–110% of DCF — trading near intrinsic value.
`🔴 downside_risk` Price > 110% of DCF — trading above intrinsic value estimate.

**Model assumptions** (all configurable):
- FCF growth rate: min(EPS 5Y growth, Revenue 5Y growth), capped at `FUNDAMENTAL_DCF_MAX_GROWTH_PCT` (default 20%)
- WACC (discount rate): `FUNDAMENTAL_DCF_WACC_PCT` (default 10%)
- Terminal growth: `FUNDAMENTAL_DCF_TERMINAL_GROWTH_PCT` (default 3%)
- Explicit stage: `FUNDAMENTAL_DCF_GROWTH_YEARS` (default 5 years)

⚠️ A 1% change in WACC or growth rate can shift the output by 30–50%. Use alongside multiples-based valuation.

### Interest Coverage (Rank 15)

Can the company comfortably pay interest on its debt? = EBIT ÷ Annual Interest Expense.

`🟢 very_safe` > 5× — Operating earnings cover interest payments 5+ times over.
`🟡 adequate` 2–5× — Serviceable but monitor if rates rise or earnings dip.
`🔴 high_risk` < 2× — Interest consumes >50% of operating earnings. Very dangerous in an economic slowdown.

Configurable via `FUNDAMENTAL_INTEREST_COVERAGE_SAFE` (default 5) and `FUNDAMENTAL_INTEREST_COVERAGE_ADEQUATE` (default 2).

### Asset Turnover (Rank 16)

Revenue generated per dollar of total assets. Higher = more capital-efficient business.

`Asset Turnover: 1.23×` — Company generates $1.23 of revenue for every $1 of assets it holds.
`Inventory X×/yr` — How many times inventory is sold and restocked per year (shown inline when available). Slowing inventory turnover is an early-warning signal for consumer and industrial companies.

**No absolute thresholds** — compare over time and to sector peers. Asset-heavy businesses (steel, airlines) naturally score lower than asset-light businesses (software, consumer brands).

### Analyst Target Price (Rank 17)

Consensus analyst target price vs. current close price. Source: Alpha Vantage.

`🟢 bullish_consensus` Upside > 15% — Analysts collectively expect significant appreciation.
`🟡 neutral` Upside −5% to +15% — Analysts expect modest or no price change.
`🔴 bearish_consensus` Upside < −5% — Analysts expect the stock to decline from here.

`+23.5% upside (target $320.00)` — Stock is at $259, analysts target $320 → 23.5% upside.

Configurable via `FUNDAMENTAL_ANALYST_UPSIDE_BULLISH` (default 15) and `FUNDAMENTAL_ANALYST_DOWNSIDE_BEARISH` (default -5).

### Analyst Rec Trend (New — Rank 17 extended)

Month-over-month change in the net analyst buy score, computed from Finnhub `/stock/recommendation` (free tier). Each month Finnhub provides a count of analysts with `strongBuy`, `buy`, `hold`, `sell`, `strongSell` ratings. The **net score** = `(strongBuy + buy) − (strongSell + sell)`. The **trend delta** = net score this month minus net score last month.

`🟢 upgrading` Delta > 5 — More analysts moved to bullish ratings vs last month. Positive revision momentum.
`🟡 neutral` Delta −5 to +5 — Consensus is stable month-over-month.
`🔴 downgrading` Delta < −5 — More analysts moved to bearish ratings vs last month. Negative revision momentum.

The **net score** shown in parentheses is the absolute current level (e.g. `net score 44` = 44 more bullish analysts than bearish).

Example: `🟢 +8 net delta — upgrading (net score 44)` — 8 more analysts upgraded to buy vs last month; 44 net bullish analysts in total.

Configurable via `FUNDAMENTAL_ANALYST_REC_UPGRADE_DELTA` (default 5) and `FUNDAMENTAL_ANALYST_REC_DOWNGRADE_DELTA` (default -5).

### Goodwill & Intangibles % (Rank 18)

Goodwill + intangible assets as a percentage of total assets. Goodwill arises when a company acquires another for more than book value. A goodwill impairment charge signals an acquisition that failed to deliver expected returns.

`🟢 low_risk` < 20% of assets — Minimal acquisition risk.
`🟡 monitor` 20–40% — Track acquisition discipline carefully.
`🔴 impairment_risk` > 40% — Significant portion of assets are intangible; impairment write-down risk is elevated.

Configurable via `FUNDAMENTAL_GOODWILL_LOW_PCT` (default 20) and `FUNDAMENTAL_GOODWILL_HIGH_PCT` (default 40).

### Price/Sales Ratio (Rank 19)

Market Cap ÷ TTM Revenue. Most useful when earnings are zero or negative (early-stage growth companies). **Always compare within sector** — SaaS companies typically command 5–15×; industrials > 3× is expensive.

`🟢 value` P/S < 5× — Cheap relative to revenue (assuming eventual margin normalisation).
`🟡 fairly_valued` P/S 5–10× — Standard range for mature growth companies.
`🟡 growth_premium_required` P/S 10–15× — Requires sustained >20% revenue growth to justify.
`🔴 speculative` P/S > 15× — Very high risk; small revenue growth disappointment can cause large price declines.

Configurable via `FUNDAMENTAL_PS_VALUE` (default 5), `FUNDAMENTAL_PS_FAIR` (default 10), and `FUNDAMENTAL_PS_SPECULATIVE` (default 15).

### FCF Conversion Rate (New — T3.9)

FCF Conversion = FCF / Net Income. This ratio reveals **earnings quality** — how much of accounting profit actually materialises as real cash. A ratio > 1.0 is common because non-cash depreciation adds back to operating income. A ratio < 0.7 is a red flag: it means reported earnings are significantly ahead of actual cash generation (aggressive accruals, deferred costs, or large working-capital buildup).

Source: `fcf_reported` and `net_income_reported` from XBRL SEC filings (both in millions).

`🟢 high_quality_cash` ≥ 1.0× — FCF equals or exceeds net income. Earnings are fully cash-backed or better.
`🟡 moderate` 0.7–1.0× — Most earnings convert to cash. Acceptable.
`🔴 accrual_concern` < 0.7× — Significant gap between accounting profits and real cash. Investigate working-capital trends, revenue recognition, or CapEx treatment.

Example: `🟢 1.23× (high_quality_cash)` — AAPL generates $1.23 in free cash for every $1 of net income reported.

Configurable via `FUNDAMENTAL_FCF_CONVERSION_HIGH` (default 1.0) and `FUNDAMENTAL_FCF_CONVERSION_LOW` (default 0.7).

---

## Alert Types

Alerts post to `#alerts` automatically every 5 minutes (configurable). Each has a severity level.

`rsi_oversold` ⚠️ — RSI < 30. Price has fallen sharply, potential bounce.
`rsi_overbought` ⚠️ — RSI > 70. Price has risen sharply, potential pullback.
`bb_squeeze` ℹ️ — BB inside Keltner. Volatility coiling, breakout incoming.
`vix_elevated` ⚠️ — VIX > 25. Market fear rising, risk-off environment.
`fa_tier_flip` ⚠️ — Composite tier changed (e.g. neutral → weak).
`liquidity_sweep` ℹ️ — Sweep detected. Institutions hunted stop-losses, potential directional move.

**Cooldown**: Same alert for same symbol won't repeat for 4 hours (configurable via `BOT_ALERT_COOLDOWN_SECS`).

## Macro Fields (Daily Report Header)

`VIX: 25.2` — FRED `VIXCLS`. Market fear index. See VIX Regime section above.
`10Y: 4.35%` — FRED `DGS10`. 10-year US Treasury yield. Rising = tighter financial conditions.
`EUR/USD: 1.1520` — FRED `DEXUSEU`. Euro vs US Dollar exchange rate.

---

## Qualitative Signals

Qualitative signals appear in the `🧠 Qualitative Signals` embed after the Tier 3 deep-context block. These are **structural proxies** computed from real data — they do not require reading 10-K prose or earnings call transcripts. An LLM layer (planned for a future release) will add richer text-based analysis.

### Moat Proxy (Tier 1 — Competitive Moat)

A moat proxy is computed from three structural inputs — no text analysis required.

**Scoring (1 point each):**
Current gross margin ≥ 40% → pricing power signal.
Gross margin standard deviation < `QUAL_MOAT_STABLE_STD_PP` (default 5pp) across 8 quarters → stability signal.
ROE ≥ 15% → sustained profitability signal.

`🏰 strong_moat_proxy` 3/3 — All three signals pass. Strong structural evidence of a durable competitive position.
`🟡 moderate_moat_proxy` 2/3 — Partial evidence. Monitor for erosion.
`🔴 weak_moat_proxy` 0–1/3 — No structural moat detected.

The `GM avg` shows the mean gross margin across history; `σ` shows the standard deviation in percentage points. A low σ means margins are stable, not just high.

Configurable via `QUAL_MOAT_STABLE_STD_PP` (default 5) and `QUAL_MOAT_STABILITY_QUARTERS` (default 8).

**Note:** This proxy can only detect structural evidence of a moat. It cannot assess brand strength, patent pipelines, or network effects — those require LLM analysis of 10-K filings.

### Insider Activity (Tier 1 — Management Quality)

Reads SEC Form 4 filings ingested from Finnhub `/stock/insider-transactions`. Tracks open-market purchases (`P`) by corporate insiders (executives, directors, major shareholders).

**Why it matters:** Insiders sell for many reasons (taxes, diversification, planned liquidations). But insiders **buy** for only one reason — they believe the stock is undervalued. Cluster buying (multiple distinct insiders buying within a short window) is one of the highest-conviction bullish signals available.

`🟢 cluster_buy` 3+ distinct insiders purchased open-market shares within the lookback window. High-conviction bullish.
`🟡 single_buy` 1–2 insiders purchased. Mildly bullish — could be individual conviction or routine.
`🔴 cluster_sell` 3+ distinct insiders sold shares. Informational — see note below.
`🟡 neutral` No significant insider activity in the lookback window.

The **buyer count** and **seller count** show how many distinct insiders transacted.

**Note:** Cluster selling is less informative than cluster buying. Directors routinely sell for estate planning, tax purposes, and 10b5-1 plans. Flag it as context, not a primary signal.

Configurable via `QUAL_INSIDER_CLUSTER_WINDOW_DAYS` (default 90) and `QUAL_INSIDER_CLUSTER_MIN_BUYERS` (default 3).

### News Sentiment (Tier 2 — Media Narrative)

7-day and 30-day rolling average sentiment scores computed from Alpha Vantage `NEWS_SENTIMENT` API. Each article is scored from −1.0 (Bearish) to +1.0 (Bullish). The per-ticker score is used when available, falling back to the overall article sentiment.

`🟢 positive` Average sentiment > 0.15 — Recent news flow is predominantly positive.
`🟡 neutral` Average sentiment −0.15 to +0.15 — Mixed or flat news coverage.
`🔴 negative` Average sentiment < −0.15 — Recent news flow is predominantly negative.
`⚪ insufficient_data` No news articles with sentiment scores in the window. Enable `FUNDAMENTAL_ENABLE_NEWS_SENTIMENT=true` and wait for the first poll cycle.

**Reading the trend:** If 7-day sentiment is significantly worse than 30-day, sentiment is deteriorating. If 7-day is significantly better, it's improving.

Example: `🟢 7d: +0.28 | 30d: +0.18` — Recent week is more bullish than the trailing month; momentum improving.

Configurable via `QUAL_SENTIMENT_POSITIVE_THRESHOLD` (default 0.15) and `QUAL_SENTIMENT_NEGATIVE_THRESHOLD` (default −0.15).

### R&D Intensity (Tier 2 — Innovation Trajectory)

R&D expense as a percentage of quarterly revenue, both from XBRL SEC filings. Signals whether a company is investing in its future or harvesting its current position.

`🟢 investing_in_future` R&D ≥ `QUAL_RD_HEALTHY_PCT`% of revenue (default 10%) — Company is actively building future products.
`🟡 moderate` R&D ≥ `QUAL_RD_MODERATE_PCT`% of revenue (default 3%) — Moderate R&D investment.
`🔴 harvesting` R&D < `QUAL_RD_MODERATE_PCT`% of revenue — Company is milking existing products with little reinvestment.

**Sector context:** Thresholds vary significantly by industry. Tune for your watchlist:
Tech/Software: healthy 10–20%, warning < 5%.
Pharma/Biotech: healthy 15–25%, critical for pipeline sustainability.
Industrials/Consumer: healthy 2–5%, > 8% is exceptional.
ETFs/Banks/REITs: R&D is not applicable — expect no data.

Configurable via `QUAL_RD_HEALTHY_PCT` (default 10) and `QUAL_RD_MODERATE_PCT` (default 3).

---

## Correlation Signals

Correlations are displayed in a **🔗 Correlations** embed below the Qualitative embed. They only appear when at least one interesting pattern is detected (a fired master signal or a cluster scoring below "mixed_positive").

Cross-metric divergence is more valuable than any single metric in isolation. When two metrics that should move together diverge, it often precedes a price move by 5–10 trading days.

### Cluster Health

Four clusters assess coherence within related metric groups. Each cluster scores −1 (severe divergence) to +1 (fully aligned).

| Tier | Score range | Meaning |
|---|---|---|
| `🟢 healthy` | ≥ 0.5 | Metrics in this cluster are aligned — no divergence detected |
| `🟡 mixed positive` | 0 – 0.5 | Mostly aligned with minor inconsistencies |
| `🟠 mixed negative` | −0.5 – 0 | Some divergence detected — watch carefully |
| `🔴 alert` | < −0.5 | Multiple divergences in this cluster — high risk signal |

**Cluster definitions:**

- **Earnings Quality** — EPS/FCF alignment, revenue vs EPS coherence, gross vs net margin trends, revenue growth vs pricing power
- **Valuation vs Quality** — P/E vs earnings growth rate, P/E vs ROIC, FCF yield vs dividend yield (coverage), P/B vs ROE
- **Leverage & Liquidity** — Net Debt/EBITDA vs interest coverage, current ratio vs FCF conversion, D/E vs net margin, goodwill vs FCF conversion
- **Operational** — ROIC vs revenue growth (dilutive growth detection), gross margin trend as demand proxy, CapEx intensity vs FCF yield

### Master Divergence Signals (★ = Highest Conviction)

These five patterns have the highest historical predictive value. Each fires when ≥ N simultaneous conditions are met.

#### ★ Bullish Convergence 🟢🟢
All five factors pointing in the same direction — rarest and most reliable bullish signal.

Conditions checked (need ≥ `CORR_BULLISH_CONVERGENCE_MIN_CONDITIONS`, default 3 of 5):
1. Low P/E (below `FUNDAMENTAL_PE_ABS_GROWTH` threshold)
2. High ROIC (moat_quality tier)
3. FCF healthy (high_quality_cash conversion OR attractive FCF yield)
4. Conservative leverage (D/E below `FUNDAMENTAL_DE_CONSERVATIVE`)
5. Insider buying (cluster_buy or single_buy from Form 4 data)

**Score shown:** e.g. `★ Bullish Convergence (4/5 conditions)` — 4 of 5 align.

#### ★ Hidden Value 🟢
Earnings held down by non-cash charges while real cash generation is strong. Market prices on EPS; you buy on FCF.

Fires when ≥ 2 of: EPS stagnant/neutral + FCF conversion high quality + FCF yield attractive.

#### ★ Deterioration Warning 🔴
Earnings are being manufactured through accrual accounting — customers are buying but not paying, or revenue is recognised before cash is collected.

Fires when ≥ 2 of: EPS strong + FCF accrual concern + receivables growing faster than revenue (ratio > `CORR_RECEIVABLES_GROWTH_MULTIPLIER`, default 1.1×).

**Red flag:** When this fires alongside a strong consensus earnings beat, investigate accounts receivable growth and capitalised expenses.

#### ★ Value Trap 🔴
Cheap for a reason — the business is structurally deteriorating. Value investors attracted by the low P/E get trapped as earnings keep declining.

Fires when ≥ `CORR_VALUE_TRAP_MIN_CONDITIONS` (default 3) of 4: low P/E + low/adequate ROIC + elevated leverage + declining revenue.

#### ★ Leverage Cycle Warning 🔴🔴
Four leverage and liquidity metrics simultaneously deteriorating — financial distress trajectory. In a rising rate environment this combination can move to a credit event within 2–4 quarters.

Fires when ≥ `CORR_LEVERAGE_CYCLE_MIN_CONDITIONS` (default 3) of 4: Net Debt/EBITDA high risk + interest coverage high risk + FCF poor conversion + current ratio liquidity risk.

### Net Signal

The overall correlation verdict, combining all five master signals:

| Display | Meaning |
|---|---|
| `🟢🟢 strongly_bullish` | 2+ bullish master signals, 0 bearish |
| `🟢 bullish` | 1 bullish master signal, 0 bearish |
| `⚪ neutral` | Signals balanced or none fired |
| `🔴 bearish` | 1 bearish master signal, 0 bullish |
| `🔴🔴 strongly_bearish` | 2+ bearish master signals fired |

### Configurable Variables

| Variable | Default | Effect |
|---|---|---|
| `CORR_BULLISH_CONVERGENCE_MIN_CONDITIONS` | 3 | Min conditions (of 5) for Bullish Convergence to fire |
| `CORR_VALUE_TRAP_MIN_CONDITIONS` | 3 | Min conditions (of 4) for Value Trap to fire |
| `CORR_LEVERAGE_CYCLE_MIN_CONDITIONS` | 3 | Min conditions (of 4) for Leverage Cycle to fire |
| `CORR_RECEIVABLES_GROWTH_MULTIPLIER` | 1.1 | AR/Revenue growth ratio threshold for Deterioration Warning |

---

## Macro Analysis — Monetary Policy

The `🏦 Monetary Policy` embed appears **once per daily report**, immediately after the header line (VIX / 10Y / EUR/USD). It shows the current monetary environment classified into regimes — the single biggest macro driver of asset prices.

Data source: **FRED (Federal Reserve Economic Data)**. Computed by the `macro-analysis` worker, stored in `macro_derived`. Updates every 6 hours (configurable).

---

### Overall Stance

The composite verdict across all monetary policy signals.

| Display | Score | Meaning |
|---|---|---|
| `🟢 accommodative (+X.XX)` | > +0.4 | Policy is supportive of risk assets. Rates falling or low, credit benign, yield curve healthy. Bullish for growth equities. |
| `🟡 neutral (+X.XX)` | ±0.4 | Mixed signals. No clear tailwind or headwind from monetary conditions. |
| `🔴 restrictive (-X.XX)` | < -0.4 | Policy is a headwind. Rates high/rising, credit stressed, or yield curve inverted. Bearish for growth and duration. |

Score range: **−1.0** (maximum restrictive) to **+1.0** (maximum accommodative).

Configurable: `MACRO_MP_ACCOMMODATIVE_SCORE` (default 0.4) · `MACRO_MP_RESTRICTIVE_SCORE` (default -0.4)

---

### Tier 1 — Monetary Policy Signals

#### 🏛️ Policy Rate (FEDFUNDS)
The effective federal funds rate — the primary lever the Fed uses to control inflation and growth.

| Display | Regime | Meaning |
|---|---|---|
| `🔴 hiking` | YoY change > +25bps | Rate is rising — tightening credit, compressing multiples. Value > growth. Banks outperform. |
| `🟡 neutral` | YoY change ±25bps | Rate stable — no directional tailwind or headwind from rate cycle. |
| `🟢 cutting` | YoY change < -25bps | Rate is falling — accommodative for risk assets. Growth > value. Buy duration. |

The YoY change in basis points is shown in parentheses: e.g. `(-125bps YoY)`.

Source: `FRED FEDFUNDS` (monthly, %)

> **TODO [LLM]**: FOMC statement hawkish/dovish scoring (−5 to +5) will be added with the LLM layer.
> **TODO [PAID]**: CME FedWatch implied rate probabilities require CME Group API subscription.
> **TODO [FUTURE]**: ECB (`ECBDFR`), BoE (`UKBANKRATE`), BoJ rates via FRED.

---

#### 📐 Yield Curve (2s10s · T10Y2Y / 3m10y · T10Y3M)
The spread between long-term and short-term Treasury yields. One of the most reliable recession predictors in finance — historically accurate with a 12–18 month lag.

| Display | Spread | Regime | Meaning |
|---|---|---|---|
| `🟢 steep` | > +1.0pp | Growth expansion phase | Lenders earn more from long loans → credit flows freely. Banks earn well. Pro-growth. |
| `🟡 normal` | 0 – +1.0pp | Healthy cycle | No warning signal. Normal credit environment. |
| `🟠 flat` | −0.5pp – 0 | Caution | Curve approaching inversion. Tightening cycle mature. Watch closely. |
| `🔴 inverted` | < −0.5pp | Recession warning | Short rates > long rates — borrowing is unprofitable, credit contracts. Has preceded every US recession since 1970. |
| `🔴🔴 re_steepening` | Rising from inverted low | **Recession arriving** | The most dangerous signal. Was inverted, now steepening. Historically means the recession has begun, not ended. |

The 3-month/10-year spread (`T10Y3M`) is the most statistically robust variant and shown alongside 2s10s for confirmation.

Source: `FRED T10Y2Y` + `FRED T10Y3M` (daily, percentage points)

Configurable: `MACRO_YC_STEEP_THRESHOLD` · `MACRO_YC_FLAT_THRESHOLD` · `MACRO_YC_INVERTED_THRESHOLD` · `MACRO_YC_RESTEEPENING_BPS` · `MACRO_YC_LOOKBACK_DAYS`

---

#### 💹 Real Rate — TIPS 10Y (DFII10)
The 10-Year Treasury Inflation-Protected Securities yield — the cleanest measure of the true real cost of borrowing. The single most important variable for gold and growth stock pricing.

| Display | Real Rate | Regime | Meaning |
|---|---|---|---|
| `🟢 deeply_negative` | < −2% | Maximum risk-on | Real rates deep in negative territory. Capital floods into growth stocks, gold, real estate. 2020–21 bubble environment. |
| `🟡 balanced` | −2% to +2% | Normal equity environment | No extreme distortion. Asset prices governed by earnings, not rate manipulation. |
| `🔴 headwind` | > +2% | Growth and gold drag | High real yields make cash and bonds competitive vs equities. Compresses growth multiples. |

Breakeven inflation (10Y) is shown in parentheses: e.g. `(BE 10Y: 2.35%)`.

Source: `FRED DFII10` (daily, %)

Configurable: `MACRO_REAL_RATE_DEEPLY_NEGATIVE` (default −2.0%) · `MACRO_REAL_RATE_HEADWIND` (default +2.0%)

---

#### 🏦 Fed Balance Sheet (WALCL)
Total assets held by the Federal Reserve. QE = expanding (injecting liquidity). QT = contracting (withdrawing liquidity). Changes in pace signal policy shifts before formal announcements.

| Display | 4-week change | Regime | Meaning |
|---|---|---|---|
| `🟢 qe` | > +$100B / 4w | Quantitative Easing | Fed buying bonds, expanding money supply. Suppresses long yields, supports asset prices. |
| `🟡 neutral` | Within ±$100B / 4w | No active policy | Balance sheet stable. No incremental stimulus or tightening from this channel. |
| `🔴 qt` | < −$100B / 4w | Quantitative Tightening | Fed allowing bonds to roll off. Upward pressure on long yields. Reduces market liquidity. |

Displayed as `$X.XT  (+/-$XXB / 4w)`.

Source: `FRED WALCL` (weekly, millions USD — displayed in billions)

Configurable: `MACRO_BS_EXPAND_THRESHOLD_BN` (default 100) · `MACRO_BS_CONTRACT_THRESHOLD_BN` (default 100)

---

#### 📉 Credit Spreads (HY + IG OAS)
Option-adjusted spreads measure how much extra yield corporate bonds pay vs equivalent Treasuries. Credit stress reliably leads equity drawdowns by 4–8 weeks.

| Display | HY Spread | Regime | Meaning |
|---|---|---|---|
| `🟢 benign` | < 300bps | Normal credit environment | Markets confident. Corporate borrowing conditions healthy. No financial stress. |
| `🟠 elevated` | 300 – 600bps | Risk-off | Investors demanding more compensation for credit risk. Tightening financial conditions. Watch closely. |
| `🔴 crisis` | > 600bps | Severe financial stress | Credit markets seizing up. Borrowing costs spiking. Precedes broad equity selloffs. 2020 peak 1100bps, 2009 peak 1900bps. |

Displayed as `HY 280bps / IG 90bps`.

Source: `FRED BAMLH0A0HYM2` (HY) · `FRED BAMLC0A0CM` (IG) — daily, in % × 100 for bps display

> **NOTE**: `TEDRATE` (TED Spread) was discontinued by FRED in May 2023. HY OAS is the primary credit stress indicator.

Configurable: `MACRO_HY_ELEVATED_BPS` (default 300) · `MACRO_HY_CRISIS_BPS` (default 600)

---

### Tier 2 — Bond Market Signals

#### 📊 Breakeven Inflation (T10YIE / T5YIE)
The bond market's expectation for average inflation over the period (nominal yield − TIPS yield). When breakevenss rise sharply, the market expects the Fed to hike — a leading signal for rate-sensitive assets.

| Display | 10Y Breakeven | Regime | Meaning |
|---|---|---|---|
| `🟢 anchored` | < 2.5% | Fed comfortable | Inflation expectations stable. No forced policy action expected. |
| `🟡 rising` | 2.5% – 3.0% | Growing risk | Market starting to price in higher inflation. Watch for acceleration. |
| `🔴 unanchored` | > 3.0% | Fed must act | 2022 scenario. Expectations unmoored — Fed will hike aggressively until anchored again. |

Displayed as `10Y: 2.35% / 5Y: 2.20%`.

Source: `FRED T10YIE` + `FRED T5YIE` (daily, %)

> **TODO [PAID]**: 5Y5Y forward inflation swap (the Fed's preferred long-run anchor) requires Bloomberg terminal or ICE Data subscription.

Configurable: `MACRO_BREAKEVEN_RISING_PCT` (default 2.5%) · `MACRO_BREAKEVEN_UNANCHORED_PCT` (default 3.0%)

---

#### 📈 Treasury Yields (2Y / 10Y / 30Y)
Benchmark rates for all global asset valuations. Every +100bps in the 10Y compresses equity fair value by ~10–15% via the discount rate effect.

Displayed as `2Y: 4.85% | 10Y: 4.30% | 30Y: 4.50%`.

Source: `FRED DGS2` · `FRED DGS10` · `FRED DGS30` (daily, %)

> **TODO [FUTURE]**: Equity Risk Premium = (1/P·E × 100) − 10Y. Requires S&P composite P/E from fundamental-analysis derived table (cross-service future link).

---

#### 💰 M2 Money Supply (M2SL)
The broad money stock. M2 YoY growth leads inflation by 12–24 months — one of the most powerful long-lead macro indicators. M2 contracted for the first time since the 1930s in 2022–23.

| Display | YoY Growth | Regime | Meaning |
|---|---|---|---|
| `🔴 inflationary` | > +15% | Surge — inflation incoming | Excess money creation. Inflation will arrive 12–24 months later. 2020: +27% preceded 2021–22 inflation spike. |
| `🟢 normal` | +4% – +15% | Healthy growth | Money supply expanding at a normal pace. No inflation or deflation concern. |
| `🟡 slow` | 0% – +4% | Below-normal growth | Growth slower than trend. Mild disinflationary signal. |
| `🟢 deflationary` | < 0% | Rare contraction | Money supply shrinking — strong disinflationary force. Eventually forces Fed to cut. Bullish for bonds. |

Displayed as `+2.1% YoY  (M2: $21,500B)`.

Source: `FRED M2SL` (monthly, billions USD)

> **TODO [FUTURE]**: M2 Velocity (`M2V`, quarterly) adds monetarist context — low signal frequency limits usefulness in daily reports.

Configurable: `MACRO_M2_INFLATIONARY_PCT` (default 15%) · `MACRO_M2_NORMAL_MIN_PCT` (default 4%)

---

### Composite Score Weights

The overall stance score is a weighted average of all individual signals:

| Signal | Weight | Most Impactful When |
|---|---|---|
| Rate regime | 2.0× | Fed actively hiking or cutting |
| Yield curve | 2.0× | Inverted or re-steepening |
| Credit spread | 2.0× | Elevated or crisis |
| Real rate | 1.5× | Deeply negative or headwind |
| Balance sheet | 1.0× | QE or QT actively running |
| Breakeven inflation | 1.0× | Unanchored |
| M2 supply | 0.5× | Extreme contraction or surge |

---

### Future Macro Panels (Not Yet Implemented)

These panels are planned but require data sources not yet available in the free tier:

| Panel | Status | Blocker |
|---|---|---|
| **FOMC Statement Scoring** | TODO — LLM layer | Requires NLP model to score hawkish/dovish language |
| **CME FedWatch probabilities** | TODO — paid API | Requires CME Group API subscription |
| **Growth Panel** (GDP, PMI, jobless claims) | TODO — future | Data exists in FRED but analysis worker not yet built |
| **Inflation Panel** (CPI, Core PCE, PPI) | TODO — future | Data exists in FRED but analysis worker not yet built |
| **Equity Risk Premium** | TODO — future | Requires linking S&P composite P/E from fundamental-analysis |
| **Global CBs** (ECB, BoE, BoJ) | TODO — future | Series IDs available in FRED; worker expansion needed |
| **China PMI** (Caixin) | TODO — paid/scrape | No free FRED equivalent |
| **5Y5Y Forward Inflation Swap** | TODO — paid | Bloomberg terminal or ICE Data required |
| **SOFR-OIS Spread** (TED replacement) | TODO — future | FRED SOFR + DFF available; computation not yet built |

---

## Macro Analysis — Growth Cycle

The Growth Cycle embed appears in `/report` immediately after the Monetary Policy embed.  
It is a **market-wide analysis** — not per-symbol, not per-asset. It describes the current state of the **real economy** (production, employment, consumer spending, business investment).

Understanding the phase of the growth cycle is critical for asset allocation:
- **Expansion**: cyclical equities outperform; high-yield credit tightens; commodities rally.
- **Slowdown**: defensives and quality outperform; duration extends; vol picks up.
- **Contraction**: cash and bonds outperform; credit spreads widen; earnings fall.

---

### Composite Score

| Display | Score | Meaning |
|---------|-------|---------|
| `🟢 Expansion (score)` | > +0.4 | Multiple leading indicators pointing to economic growth |
| `🟡 Slowdown (score)` | -0.4 to +0.4 | Mixed signals — economy neither accelerating nor collapsing |
| `🔴 Contraction (score)` | < -0.4 | Multiple indicators flagging economic deterioration or recession |
| `⚪ Insufficient Data` | — | FRED data not yet populated; restart after `data-equity` runs |

Score range: +1.0 = maximum expansion, -1.0 = maximum contraction.  
Configurable via `GROWTH_EXPANSION_SCORE` (default 0.4) and `GROWTH_CONTRACTION_SCORE` (default -0.4).

---

### Tier 1 — Leading Indicators

Leading indicators move **before** the economy — they provide 2–12 months of advance notice.

#### ISM Manufacturing PMI (NAPM)
Source: FRED `NAPM` (monthly, 0–100 index)  
Above 50 = expansion; below 50 = contraction. The gold standard for manufacturing cycle timing.

| Display | PMI Value | Meaning |
|---------|-----------|---------|
| 🟢 `strong_expansion` | ≥ 55 | Factories running hot — new orders and production accelerating |
| 🟢 `expansion` | 50–55 | Moderate growth — above the breakeven level |
| 🟡 `slowing` | 45–50 | Below breakeven — growth decelerating, watch closely |
| 🔴 `contraction` | 40–45 | Manufacturing contracting — orders and output falling |
| 🔴 `severe_contraction` | < 40 | Deep recession conditions in manufacturing |

3-month trend `improving` / `stable` / `deteriorating` is also shown when available.

Configurable: `GROWTH_PMI_STRONG` (55), `GROWTH_PMI_EXPANSION` (50), `GROWTH_PMI_SLOW` (45), `GROWTH_PMI_SEVERE` (40)

> **TODO [PAID]**: S&P Global (Markit) PMI provides monthly data with sector breakdown. ISM Services PMI requires ISM membership or paid feed.

---

#### Conference Board LEI (USSLIND)
Source: FRED `USSLIND` (monthly, index level)  
The Conference Board's composite of 10 leading indicators. Shows where the economy is heading 6–12 months ahead.

| Display | 6-Month Rate | Meaning |
|---------|-------------|---------|
| 🟢 `expanding` | > 0% | Composite leading indicator is rising — growth expected |
| 🟡 `slowing` | 0% to -3% | Deceleration — watch other signals carefully |
| 🔴 `recession_risk` | < -3% | Broad weakness across components |
| 🔴 `rule_of_three_decline` | 3+ consecutive monthly drops | Classic recession warning — historically highly reliable |

Configurable: `GROWTH_LEI_EXPANSION_RATE` (0.0), `GROWTH_LEI_RECESSION_RATE` (-3.0)

---

#### Initial Jobless Claims (ICSA + CCSA)
Source: FRED `ICSA` (weekly), `CCSA` (weekly)  
People filing for unemployment benefits for the first time. A real-time, high-frequency labour market signal. 4-week moving average used to reduce single-week noise.

| Display | 4-Week MA | Meaning |
|---------|-----------|---------|
| 🟢 `tight_labor` | < 225K | Layoffs extremely low — employers holding on to workers |
| 🟡 `normal` | 225K–300K | Healthy labour market — typical mid-cycle range |
| 🟡 `normalizing` | 300K–500K | Layoffs rising — labour market cooling |
| 🔴 `crisis` | > 500K | Mass layoff event — 2020 pandemic peaked at 6.8M |

`CCSA` (continuing claims) shows how long the unemployed remain out of work.

Configurable: `GROWTH_CLAIMS_TIGHT` (225000), `GROWTH_CLAIMS_NORMALIZING` (300000), `GROWTH_CLAIMS_CRISIS` (500000)

---

#### Housing Starts + Building Permits (HOUST / PERMIT)
Source: FRED `HOUST`, `PERMIT` (monthly, annualised thousands of units)  
Housing is 15–18% of GDP (including related services). When housing turns, the broader economy typically follows in 6–12 months — permits lead starts by 1–2 months.

| Display | Starts (ann.) | Meaning |
|---------|---------------|---------|
| 🟢 `strong` | ≥ 1,500K | Housing boom — builders active, mortgage demand high |
| 🟡 `moderate` | 800K–1,500K | Normal cycle range |
| 🔴 `weak` | < 800K | Severe housing contraction (2009 low was 478K) |

Configurable: `GROWTH_HOUSING_STRONG` (1500), `GROWTH_HOUSING_WEAK` (800)

---

### Tier 2 — Coincident Indicators

Coincident indicators move **with** the economy — they confirm what is happening now.

#### Real GDP (GDPC1)
Source: FRED `GDPC1` (quarterly, billions of chained 2012 dollars)  
The broadest measure of economic output. Annualised quarter-on-quarter growth is computed from the level data. This signal is always 1–3 months stale — supplement with LEI and claims for timeliness.

| Display | Annualised QoQ | Meaning |
|---------|----------------|---------|
| 🟢 `strong` | > 3% | Economy firing on all cylinders |
| 🟡 `moderate` | 1–3% | Normal expansion range |
| 🟡 `stall_speed` | 0–1% | Growth dangerously close to zero — small shock could tip to recession |
| 🔴 `recession` | < 0% | Economy shrinking — two consecutive quarters = technical recession |

Configurable: `GROWTH_GDP_STRONG` (3.0), `GROWTH_GDP_STALL` (1.0)

---

#### Employment — Payrolls + Unemployment + AHE + Sahm Rule
Sources: `PAYEMS` (monthly net jobs, thousands), `UNRATE` (%), `CES0500000003` (avg hourly earnings %), `SAHMREALTIME`

The Sahm Rule overrides all other employment signals when triggered.

| Display | Signal | Meaning |
|---------|--------|---------|
| 🔴 `recession_confirmed` | Sahm ≥ 0.5pp | Unemployment has risen enough above its 12-month low to historically confirm an ongoing recession. Acts as override. |
| 🟢 `strong` | Payrolls ≥ +200K/mo | Labour market booming — consistent with late expansion |
| 🟡 `moderate` | +75K to +200K/mo | Healthy but not overheating |
| 🟡 `slowing` | 0 to +75K/mo | Labour market barely growing |
| 🔴 `contraction` | < 0 | Net job losses — recession signal |

Average Hourly Earnings (`AHE`) and `CCSA` are shown as supplementary context.

Configurable: `GROWTH_NFP_STRONG` (200), `GROWTH_NFP_MODERATE` (75), `GROWTH_SAHM_THRESHOLD` (0.5)

> **TODO [PAID]**: ADP National Employment Report — no free API.

---

#### Real Retail Sales YoY — RRSFS
Source: FRED `RRSFS` (monthly, millions of chained 2012 dollars)  
Inflation-adjusted consumer spending. Removes price noise to show actual volume growth. `RSAFS` (nominal) shown for context.

| Display | YoY % | Meaning |
|---------|--------|---------|
| 🟢 `healthy` | > 3% | Consumer spending well above inflation — expansion driver |
| 🟡 `slowing` | 0–3% | Growth decelerating — consumer cautious |
| 🔴 `contraction` | < 0% | Real spending declining — consumer recession signal |

Configurable: `GROWTH_RETAIL_HEALTHY` (3.0)

---

### Tier 3 — Lagging / Sentiment Indicators

Lagging indicators confirm trends **after** they are established. Sentiment is often a contrarian signal at extremes.

#### Michigan Consumer Sentiment (UMCSENT)
Source: FRED `UMCSENT` (monthly, 0–200 index)  
Survey-based measure of household optimism. Most useful as a **contrarian signal at extremes**.

| Display | Index | Meaning |
|---------|-------|---------|
| 🟢 `near_bottom` | < 60 | Extreme pessimism — historically near market lows. Contrarian bullish. |
| 🟡 `pessimistic` | 60–80 | Below-average confidence — consumers cautious |
| 🟡 `normal` | 80–100 | Normal confidence range |
| 🔴 `complacency` | > 100 | Extreme optimism — historically near market peaks. Contrarian bearish. |

Configurable: `GROWTH_UMICH_BOTTOM` (60), `GROWTH_UMICH_COMPLACENCY` (100)

---

#### Core Capex — New Orders, Nondefense Capital Goods Ex-Aircraft (NEWORDER)
Source: FRED `NEWORDER` (monthly, millions, seasonally adjusted)  
The cleanest proxy for business capital expenditure plans. Excludes defense (government) and aircraft (lumpy Boeing orders). 3-month rolling change vs prior 3 months is used to reduce monthly noise.

| Display | 3-Month Trend | Meaning |
|---------|---------------|---------|
| 🟢 `expanding` | > +3% | Businesses investing in equipment — confidence in future growth |
| 🟡 `stable` | 0–3% | Neutral business investment |
| 🟡 `slowing` | -3% to 0% | Capex easing — caution setting in |
| 🔴 `warning` | < -3% | Businesses pulling back investment — recession risk rising |

`DGORDER` (Total Durable Goods Orders) shown in payload for broader context.

Configurable: `GROWTH_CAPEX_EXPANSION` (3.0), `GROWTH_CAPEX_WARNING` (-3.0)

---

### Composite Score Weights

| Signal | Tier | Weight |
|--------|------|--------|
| ISM Manufacturing PMI | Tier 1 | 0.15 |
| Conference Board LEI | Tier 1 | 0.12 |
| Initial Jobless Claims | Tier 1 | 0.08 |
| Housing Starts | Tier 1 | 0.08 |
| Real GDP | Tier 2 | 0.14 |
| Nonfarm Payrolls + Sahm | Tier 2 | 0.14 |
| Real Retail Sales | Tier 2 | 0.10 |
| Michigan Sentiment | Tier 3 | 0.05 |
| Core Capex | Tier 3 | 0.05 |

Total weight when all signals available: 0.91 (0.09 reserved for future paid PMI signals).

---

### What is NOT yet implemented (Future TODOs)

| What | Why Blocked |
|------|-------------|
| **S&P Global (Markit) PMI** | Paid subscription only — would improve leading indicator quality |
| **ISM Services PMI** | Requires ISM membership or paid feed |
| **China Caixin PMI** | Paid |
| **Eurozone / UK / Japan PMI** | Paid |
| **GDPNow (Atlanta Fed)** | No public API — requires web scraping |
| **ADP Employment Report** | No free API |
| **Conference Board LEI sub-components** | Raw component data is paid; only `USSLIND` composite is free on FRED |

---

## Macro Analysis — Inflation & Prices

The Inflation & Prices embed appears in `/report` after the Growth Cycle embed.  
It is a **market-wide analysis** — not per-symbol, not per-asset.

Inflation drives the most important macro trades:
- **Hot inflation** → Fed stays hawkish → bonds sell off → growth/tech underperforms → defensives, energy, commodities outperform
- **Deflationary** → demand collapse → Fed forced to cut → bonds rally → gold and cash outperform
- **Moderate (goldilocks)** → Fed comfortable → risk assets thrive

---

### Composite Score

| Display | Score | Meaning |
|---------|-------|---------|
| `🔴 Hot (score)` | > +0.4 | Multiple inflation signals elevated — Fed hawkish, bonds under pressure |
| `🟡 Moderate (score)` | -0.4 to +0.4 | Inflation within manageable range — mixed conditions |
| `🔵 Deflationary (score)` | < -0.4 | Deflation risk — demand collapse, bonds rally, Fed forced to act |
| `⚪ Insufficient Data` | — | FRED data not yet populated |

Score range: +1.0 = maximum inflation pressure, -1.0 = deflation risk.  
Configurable: `INFLATION_HOT_SCORE` (default 0.4), `INFLATION_DEFLATION_SCORE` (default -0.4)

---

### Tier 1 — Core Inflation Measures

#### Core PCE — The Fed's Actual Target (PCEPILFE)
Source: FRED `PCEPILFE` (monthly, index level; YoY % computed)  
Weight in composite: **0.20 (highest)** — when Powell says "inflation," he means Core PCE.  
Core PCE runs ~0.3–0.5pp **below** Core CPI due to chain-weighting (consumers substitute cheaper alternatives).

| Display | YoY % | Meaning |
|---------|--------|---------|
| 🟢 `at_target` | < 2.2% | Fed comfortable — neutral to dovish policy bias |
| 🟡 `hawkish_bias` | 2.2–3.0% | Above target — Fed stays restrictive; rate cuts unlikely |
| 🔴 `aggressive_tightening` | > 3.0% | Persistent overshot — Fed must hike or hold rates significantly longer |
| 🟢 `below_target` | < 1.8% | Potential undershoot — mild dovish pressure |

Fed target context: Core PCE persistently above 2.5% = **Fed will not cut rates regardless of other economic data.**

Configurable: `INFLATION_CORE_PCE_AT_TARGET` (2.2), `INFLATION_CORE_PCE_HAWKISH` (3.0)

---

#### Headline CPI (CPIAUCSL) + Core CPI (CPILFESL)
Sources: FRED `CPIAUCSL`, `CPILFESL` (monthly, index level; YoY % computed)  
Weight of CPI in composite: **0.20** | Core CPI: **0.10**

| Display | CPI YoY % | Meaning |
|---------|-----------|---------|
| 🟢 `goldilocks` | 1.5–2.5% | Ideal — Fed comfortable, equities thrive |
| 🟡 `rising` | 2.5–4.0% | Tightening bias — watch for Fed communication shift |
| 🔴 `above_target` | 4.0–5.0% | Significant overshoot — market expects sustained restriction |
| 🔴 `hot` | > 5.0% | Aggressive hiking cycle (2022: peaked at 9.1%) |
| 🟢 `below_target` | 0–1.5% | Below Fed comfort zone |
| 🔴 `deflation_risk` | < 0% | Demand collapse — recession signal |

CPI day surprise vs consensus = one of the highest-impact single data releases for bonds and equities.

Configurable: `INFLATION_CPI_GOLDILOCKS_MAX` (2.5), `INFLATION_CPI_ABOVE_TARGET` (4.0), `INFLATION_CPI_HOT` (5.0)

---

#### Shelter CPI (CUSR0000SAH1) — 35% of Headline CPI
Source: FRED `CUSR0000SAH1` (monthly, YoY % computed)  
Shelter (Owner's Equivalent Rent) has an **18-month lag** to actual market rents. This makes it the longest-lasting structural inflation driver in a rate cycle.

| Display | YoY % | Meaning |
|---------|--------|---------|
| 🟢 `normalizing` | < 2.5% | Shelter disinflation is materialising — CPI will follow down |
| 🟡 `moderating` | 2.5–3.5% | Slowing but still elevated — more time needed |
| 🟡 `elevated` | 3.5–5.0% | Still sticky — CPI will remain elevated despite goods disinflation |
| 🔴 `hot` | > 5.0% | Rent surge — structural inflation driver at peak |

*(18m lag)* notation shown — when market rents peaked in 2022, shelter CPI peaked in 2023.

> **TODO [LLM]**: Compare shelter CPI to real-time rental indices (Zillow, Apartments.com) to estimate the forward path of OER and predict when shelter disinflation arrives.

---

### Pipeline & Energy — Tier 2

#### PPI Final Demand (PPIFID) + PPI-CPI Spread
Source: FRED `PPIFID` (monthly, YoY % computed), `PPIACO` (all commodities)  
Weight in composite: **0.10**  
PPI typically **leads CPI by 3–6 months** — falling PPI = disinflation arriving.

| Display | PPI YoY % | Meaning |
|---------|-----------|---------|
| 🟢 `deflationary` | < 0% | Goods deflation — CPI will follow down in 3–6 months |
| 🟢 `stable` | 0–2% | Producer prices stable |
| 🟡 `moderate` | 2–4% | Moderate pipeline pressure |
| 🔴 `elevated` | 4–8% | Above-target producer inflation — CPI to follow |
| 🔴 `surge` | > 8% | Severe producer inflation (Ukraine war 2022: +11.2%) |

**PPI-CPI Spread (corporate margin signal):**

| Spread | Signal | Meaning |
|--------|--------|---------|
| > +3pp | 🔴 `margin_pressure` | Producers absorbing more than they pass through → watch for earnings misses from manufacturers |
| -3 to +3pp | 🟡 `neutral` | Normal pass-through |
| < -3pp | 🟢 `margin_expansion` | Producers benefit — input costs falling faster than output prices |

Configurable: `INFLATION_PPI_SURGE` (8.0), `INFLATION_PPI_ELEVATED` (4.0), `INFLATION_PPI_CPI_SPREAD_WARNING` (3.0)

---

#### WTI Crude Oil (DCOILWTICO) + Brent (DCOILBRENTEU)
Source: FRED `DCOILWTICO`, `DCOILBRENTEU` (daily, $/barrel — latest observation)  
Weight in composite: **0.15**  
A **$10/barrel move shifts US headline CPI by ~0.3–0.4pp.**

| Display | WTI $/barrel | Meaning |
|---------|--------------|---------|
| 🔴 `inflationary_risk` | > $100 | Demand destruction risk; consumer headwind; energy outperforms |
| 🟡 `elevated` | $80–100 | Inflationary pressure building; energy sector profitable |
| 🟢 `goldilocks` | $60–80 | Affordable energy; economy + margins healthy |
| 🟢 `low` | $50–60 | Disinflationary; energy sector under pressure |
| 🔴 `energy_sector_stress` | < $50 | Severe energy deflation; E&P sector stress; deflationary signal |

**Brent-WTI spread**: Brent trades above WTI due to transportation costs. A widening spread > $5 indicates elevated geopolitical risk premium.

> **TODO [SCRAPE]**: EIA weekly petroleum inventories — no FRED equivalent; EIA.gov data requires scraping.  
> **TODO [PAID]**: CME WTI futures curve (contango/backwardation) — requires CME API.

Configurable: `INFLATION_WTI_GOLDILOCKS_MIN` (60), `INFLATION_WTI_GOLDILOCKS_MAX` (80), `INFLATION_WTI_INFLATIONARY` (100), `INFLATION_WTI_STRESS` (50)

---

### Wages & Commodities — Tier 3

#### Wages — AHE (CES0500000003) + ECI (ECIALLCIV)
Sources: FRED `CES0500000003` (monthly), `ECIALLCIV` (quarterly)  
Weight in composite: **0.15**  
Wages are **60–70% of service sector costs**. Wage growth above 3.5% in a 2% inflation target regime = wage-price spiral risk.

AHE (Average Hourly Earnings) = high frequency but volatile.  
ECI (Employment Cost Index) = quarterly, smoother, the Fed's preferred wage measure.  
**ECI YoY %** in the bot is **latest quarter vs same quarter one year ago** (four quarterly observations back in `macro_fred`), not a 12-row lookback (which would mis-read quarterly data).

| Display | AHE YoY % | Meaning |
|---------|-----------|---------|
| 🟢 `soft` | < 2% | Wage growth too low — deflationary pressure on services |
| 🟢 `target_consistent` | 2–3.5% | Consistent with 2% inflation target — neutral Fed |
| 🟡 `above_target` | 3.5–4.5% | Services inflation sticky — Fed cautious on cuts |
| 🔴 `elevated` | 4.5–5% | Above-target wage pressure — hawkish Fed stance reinforced |
| 🔴 `spiral_risk` | > 5% | Wage-price spiral risk — the 2022–2023 challenge |

Configurable: `INFLATION_WAGE_TARGET_MAX` (3.5), `INFLATION_WAGE_ELEVATED` (4.5), `INFLATION_WAGE_SPIRAL` (5.0)

---

#### Copper (PCOPPUSDM) — Global Industrial Demand Proxy
Source: FRED `PCOPPUSDM` (monthly, $/metric ton; YoY % computed)  
Weight in composite: **0.10**  
Copper is used in virtually every industrial process. **China = 55% of global demand** → copper is a real-time barometer of Chinese and global industrial activity.

| Display | YoY % | Meaning |
|---------|--------|---------|
| 🟢 `global_expansion` | > +10% | Strong global industrial demand; cyclicals outperform |
| 🟢 `stable` | 0–10% | Neutral global growth signal |
| 🟡 `slowing` | -10–0% | Global demand decelerating |
| 🔴 `global_contraction` | < -10% | Global industrial contraction; defensives outperform |

> **TODO [PAID]**: Iron ore price — no free FRED equivalent; LME data is paid. AUD/USD (`DEXUSAL`) is a free liquid proxy for copper / iron ore exposure.  
> **TODO [PAID]**: CME copper futures curve for contango/backwardation signal.

Configurable: `INFLATION_COPPER_EXPANSION_YOY` (10.0), `INFLATION_COPPER_CONTRACTION_YOY` (-10.0)

---

### Composite Score Weights

| Signal | Source | Weight |
|--------|--------|--------|
| Core PCE (Fed target) | `PCEPILFE` | 0.20 |
| Headline CPI | `CPIAUCSL` | 0.20 |
| Wages (AHE) | `CES0500000003` | 0.15 |
| WTI Crude Oil | `DCOILWTICO` | 0.15 |
| Core CPI | `CPILFESL` | 0.10 |
| PPI Final Demand | `PPIFID` | 0.10 |
| Copper | `PCOPPUSDM` | 0.10 |

Shelter CPI and Brent are stored for context but not scored (shelter has a structural lag; Brent is a regional crude variant).  
ECI is scored implicitly through the wage signal.

---

### What is NOT yet implemented (Future TODOs)

| What | Why Blocked |
|------|-------------|
| **Iron ore price** | No free FRED equivalent — LME data is paid |
| **EIA weekly oil inventory** | EIA.gov has data but no FRED series; web scraping needed |
| **CPI surprise vs consensus** | Bloomberg/Refinitiv consensus estimates are paid |
| **Shelter CPI lag adjustment** | Requires LLM estimation vs real-time rent indices (Zillow etc.) |
| **CME WTI/copper futures curves** | Contango/backwardation signals require CME API |
| **AUD/USD as iron ore proxy** | `DEXUSAL` on FRED — low-effort future addition |

---

## Macro Analysis — Global & Geopolitical

Embed order in `/report`: after **Inflation & Prices**. Same as other macro panels: **one global snapshot**, not per symbol.

Uses **FRED only** today: `DTWEXBGS`, `DEXJPUS`, `CHNGDPNQDSMEI`, `FYFSD`, `GDP`. Computed by `macro-analysis` → `macro_derived` (`gg_*` metrics).

    ### Macro intelligence embed (after Global & Geopolitical)

Separate Discord embed **Macro intel · calendars · geo · headlines** — data from `data-macro-intel` and optional LLM rows:

| Source | Tables / notes |
|--------|----------------|
| Economic calendar | `economic_calendar_events` (Finnhub; tier may block) |
| Earnings calendar | `earnings_calendar_events` for symbols on your **equity watchlist** |
| GPR | `geopolitical_risk_monthly` from `GPR_CSV_URL` |
| GDELT | `gdelt_macro_daily` (aggregate tone for `MACRO_INTEL_GDELT_QUERY`) |
| Macro headlines | `news_headlines` where `source` is `rss_macro_*` or `finnhub_macro_general` |
| FOMC narrative | `narrative_scores` (`doc_kind=fomc_statement`) from optional **OpenAI** job |

Configure the job with `BOT_FOMC_NARRATIVE_ENABLE=true`, `OPENAI_API_KEY`, `FOMC_STATEMENT_URL` (HTML page), and `BOT_FOMC_NARRATIVE_CRON` (cron, UTC). See root `.env.example`.

Use Discord **`/status`** (ephemeral) to see **row counts** for macro-intel tables (`economic_calendar_events`, `gdelt_macro_daily`, etc.) — useful when a section is empty (Finnhub tier limits, missing `GPR_CSV_URL`, or worker not rebuilt).

### Market cycle embed (after Global & Geopolitical)

**Metric:** `mc_market_cycle` from **macro-analysis** (reads `equity_ohlcv` for **SPY** by default, blends **gc/mp/inf/gg** stances).

| Field | Meaning |
|--------|---------|
| **Composite phase** | Rule-based headline: e.g. `bull_macro_aligned`, `late_cycle_stretched`, `correction_risk`, `bear_structural`, `crash_panic` |
| **Score** | −1 (stress) … +1 (constructive) — not a trade signal, a compact regime index |
| **Price phase** | From drawdown vs ~252d peak high: pullback / correction / bear / crash velocity / bull / `below_sma` |
| **vs 200DMA** | % above/below simple 200-day close average |
| **Crash velocity flag** | True if drop vs 10d high or 5-bar return exceeds configured thresholds (`MARKET_CYCLE_*`) |

Implements the **live** slice of `macro_analysis_reference.html` **Market Cycles** (drawdown bands + 200DMA row). Historical episode **tables** in that HTML stay reference-only unless you add a static dataset later.

If **`mc_market_cycle`** is missing from the DB, the bot still shows a grey **“Market cycle — data missing”** card with fix steps (rebuild `macro-analysis`, ingest SPY daily bars, env).

### Macro correlation regime embed (after Market cycle)

**Storage:** same hypertable **`macro_derived`**, **`source = macro_analysis`** — **no new migration**.  
**Metric:** **`mc_macro_correlation`**, written by **`macro-analysis`** after **`analyzeMarketCycles`** when **`MARKET_MACRO_CORR_ENABLE=true`** (default).

The worker reads the **latest payloads** for stances/regimes already upserted in that run (`gc_stance`, `mp_stance`, `inf_stance`, `gg_stance`, `mp_yield_curve`, `mp_real_rate`, `mp_credit_spread`, `gc_gdp`, `inf_oil`, `gg_broad_dollar`, `gg_usdjpy`) and maps them to one **regime** bucket, a numeric **score** (−1 stress … +1 constructive), a short **label**, and **flags** (e.g. curve, credit, USD). Logic lives in **`services/data-analyzer/internal/macrocorr`** — it is a **compact regime summary**, not a full replication of every narrative cell in **`macro_analysis_reference.html`** **Macro Correlations** (that HTML panel remains the conceptual reference).

| Regime (examples) | Rough meaning |
|-------------------|---------------|
| `recession_pipeline` | Inverted curve + stressed credit + weak GDP/growth |
| `stagflation_risk` | Hot inflation stance + soft growth |
| `rising_inflation_tight_policy` | Hot inflation + restrictive policy + flat/inverted curve |
| `global_liquidity_stress` | Elevated global stress + USD or JPY stress |
| `goldilocks_light` / `disinflation_soft_landing` | Constructive mixes when spreads contained |
| `deflation_risk` / `neutral_mixed` | Deflationary stance or no single dominant story |

**Daily report:** Discord embed **Macro correlations** (after **Market cycle**). **Missing data** → grey card with rebuild/env hints.

### Additional analysis embed (after Macro correlations)

**Storage:** same **`macro_derived`** / **`macro_analysis`** source — **no new table**.  
**Metric:** **`aa_reference_snapshot`** — end of **`macro-analysis`** when **`ADDITIONAL_ANALYSIS_ENABLE=true`** (default).

**v1 scope** (from **`additional_analysis_reference.html`**; most tabs remain reference-only until new data feeds exist):

| Block | What it is |
|--------|------------|
| **Bond–equity (60d)** | Pearson **ρ** of benchmark **daily log returns** vs **Δ FRED `DGS10`** (forward-filled). Regimes: `deflationary_hedge` / `inflationary_positive` / `transition_neutral`. |
| **Oil–equity (60d)** | Same vs **Δ `DCOILWTICO`** (WTI). Regimes: `procyclical` / `decoupled` / `neutral_mixed`. |
| **VIX–equity (60d)** | Same vs **Δ `VIXCLS`**. Regimes: `typical_fear_greed` / `unusual_positive` / `compressed_link`. |
| **Month seasonality** | **Static almanac** per calendar month (tie-breaker only). |
| **Presidential cycle** | **Year 1–4** of the US election cycle + short narrative. |
| **HTML coverage** | Every tab from **`additional_analysis_reference.html`** is listed with **`live_*` / `needs_data` / `not_automated`** — honest scope (sentiment, flow, alt data, events, pairs are **not** computed yet). |

**Daily report:** embed **Additional analysis · intermarket & calendars** (+ **HTML coverage** field). **Missing row** → grey card (rebuild **macro-analysis**; **`macro_fred`** needs **DGS10**, **DCOILWTICO**, **VIXCLS** per `.env.example` **FRED_SERIES_IDS**).

### `/analyze` context strip (after price)

**Symbol reports** show the **price** embed first, then a compact **Context vs \<benchmark\>** embed when any of: benchmark **composite/price** phase or drawdown (`mc_market_cycle`), **macro correlation** regime (`mc_macro_correlation`), a one-line **additional context** summary (`aa_reference_snapshot`: bond–equity ρ, month bias, cycle year), or (equities only) **20-session excess return vs benchmark** is available.

- **Benchmark symbol** matches **`MARKET_CYCLE_SYMBOL`** on the bot (`market_cycle_symbol` in config, default **SPY**) so it stays aligned with the index used for **`mc_market_cycle`**.
- **Crypto** symbols get benchmark + macro regime only (no vs-benchmark RS from `equity_ohlcv`).
- Cached **`/analyze`** responses include **`analyze_context`** and **`market_ops`** in the Redis payload (see `ReportBuilder._serialise_symbol_report`).

### Market ops embed (after context strip)

When **`BOT_MARKET_OPS_ENABLE`** is true, **`/analyze`** adds **⚙️ Market ops — \<symbol\>**: **VIX** level and regime use **`macro_fred`** **`VIXCLS`** when present; if FRED has no row, the bot falls back to the same **`technical_indicators.vix_regime`** value as the TA embed (**`MARKET_CYCLE_SYMBOL`**, e.g. **SPY**) and says so in the label — fix the root cause by running **data-equity** FRED. **`mo_reference_snapshot`** still supplies **`reference_modules`**. Also **ATR%** and **volume vs median** over **`BOT_MARKET_OPS_VOLUME_LOOKBACK`** bars. Flags **`atr_pct_elevated`** / **`volume_vs_median_elevated`** use **`BOT_MARKET_OPS_ATR_PCT_ELEVATED`** and **`BOT_MARKET_OPS_VOLUME_RATIO_ELEVATED`**. This is **execution and noise context** — not positioning (COT), not buy/sell levels.

Discord **`/status`** lists **latest `ts`** for **`mc_market_cycle`**, **`mc_macro_correlation`**, **`aa_reference_snapshot`**, and **`mo_reference_snapshot`** under **Macro derived (latest ts)** when the DB queries succeed.

---

### Composite `gg_stance`

| Display | Score (approx.) | Meaning |
|---------|------------------|---------|
| `🔴 Elevated stress` | ≥ `GLOBAL_STRESS_ELEVATED_SCORE` (0.35) | Strong USD, carry unwind, weak China, and/or high deficit — tight global financial conditions |
| `🟡 Moderate` | between benign and elevated | Mixed signals |
| `🟢 Benign` | ≤ `GLOBAL_STRESS_BENIGN_SCORE` (-0.15) | Relatively supportive backdrop for risk / EM / commodities |
| `⚪ Insufficient Data` | — | Not enough FRED series backfilled yet |

Score range: **-1.0** (benign) … **+1.0** (elevated stress). Weights: broad dollar 0.28, USD/JPY 0.28, China GDP 0.24, fiscal 0.20.

---

### Tier 1 — FX & carry

#### Broad USD — `DTWEXBGS`

FRED **Trade Weighted U.S. Dollar Index: Broad, Goods** (weekly). **Not the ICE DXY** (different basket/weights); same macro role: strong USD tightens global liquidity.

| Regime | Index (defaults) | Meaning |
|--------|------------------|---------|
| `dollar_weak_risk_on` | < 95 | Softer USD — tailwind for commodities / EM FX |
| `supportive_equities` | 95–100 | Neutral-to-supportive for US equities |
| `neutral` | 100–105 | Transition zone |
| `em_commodity_headwind` | 105–110 | Headwind for commodities & EM |
| `major_global_stress` | ≥ 110 | Extreme USD strength — EM stress risk |

Configurable: `GLOBAL_DOLLAR_SUPPORTIVE_MAX`, `GLOBAL_DOLLAR_NEUTRAL_MAX`, `GLOBAL_DOLLAR_HEADWIND_MAX`, `GLOBAL_DOLLAR_STRESS_MIN`.

#### USD/JPY — `DEXJPUS`

Daily spot; **~20 observation** (~1 month) **% change**. **Negative %** = JPY strengthening vs USD → **yen carry unwind** risk (reference: 5% / 10% thresholds).

| Regime | Condition | Meaning |
|--------|-----------|---------|
| `carry_intact` | shallow drawdown | Baseline |
| `early_carry_unwind` | 20d % ≤ `GLOBAL_USDJPY_EARLY_UNWIND_PCT` (-5%) | Early warning |
| `systemic_carry_unwind` | 20d % ≤ `GLOBAL_USDJPY_SYSTEMIC_UNWIND_PCT` (-10%) | Severe deleveraging risk |

Configurable: `GLOBAL_USDJPY_LOOKBACK_OBS` (22), early/systemic % thresholds.

> **TODO [PAID]**: Options-implied JPY vol overlay.  
> **TODO [FUTURE]**: ECB / BoE rates vs Fed (`ECBDFR`, `UKBRBASE`) for policy divergence.

---

### Tier 2 — China & US fiscal

#### China GDP YoY — `CHNGDPNQDSMEI`

OECD **quarterly** China GDP; YoY vs same quarter prior year. **Low frequency** — not a PMI substitute.

| Regime | YoY % (defaults) | Stress contribution |
|--------|------------------|---------------------|
| `expansion` | ≥ 6% | Lowers composite stress |
| `stable` | 5–6% | Neutral |
| `slowing` | 3–5% | Raises stress |
| `contraction_risk` | < 3% | Raises stress |

Configurable: `GLOBAL_CHINA_GDP_CONTRACT`, `GLOBAL_CHINA_GDP_STABLE`, `GLOBAL_CHINA_GDP_EXPANSION`.

> **TODO [PAID]**: NBS official PMI, Caixin PMI — not on free FRED.  
> **TODO [SCRAPE]**: PBOC RRR/MLF, NPC/fiscal stimulus headlines.

#### US fiscal — `FYFSD` + `GDP`

- **`FYFSD`**: federal surplus (+) / deficit (-), **millions of USD**, **fiscal year** (annual, slow updates).
- **`GDP`**: US nominal GDP **billions**, quarterly **SAAR** — latest observation used as **annualized nominal GDP** denominator.

Deficit % of GDP ≈ `|FYFSD| / 1000 / GDP × 100` (indicative; FY vs calendar timing differs).

| Regime | Deficit % GDP | Meaning |
|--------|----------------|--------|
| `manageable` | ≤ 3% | Reference: manageable peacetime |
| `elevated_supply_risk` | 3–6% | More Treasury supply / term premium risk |
| `high_deficit_stress` | > 6% | Elevated peacetime concern |

Configurable: `GLOBAL_FISCAL_MANAGEABLE_PCT`, `GLOBAL_FISCAL_ELEVATED_PCT`.

> **TODO [SCRAPE]**: CBO long-run outlook narrative.  
> **TODO [LLM]**: Tariff / trade-war sector overlays (USTR) — not time-series in DB.

---

### Not implemented (see code `TODO`)

| Item | Blocker |
|------|---------|
| ICE DXY real-time | Paid / different vendor |
| GPR index, GDELT | Separate ingestion (file/API) |
| CFTC COT | Weekly bulk + parser + storage |
| EM stress (EMBI+), Caixin PMI | Paid or scrape |
| USTR / WTO tariff tracker | Scrape or manual |

---

## ETF / SPY Note

SPY is an ETF (Exchange-Traded Fund) that tracks the S&P 500 index. ETFs have no individual earnings, P/E ratio, or margins — all fundamental fields will show `⚪ —`. Only price and technical signals apply.
