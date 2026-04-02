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
`/analyze symbol:AAPL asset_type:equity` — Full deep-dive: price → technical → Tier 1 fundamentals → 🏦 Tier 2 balance sheet → 🔍 Tier 3 deep context → news
`/report` — Triggers the daily market report on demand (same as the 07:00 scheduled job)
`/dictionary` — Sends this glossary as paginated Discord embeds
`/status` — Bot health: DB ✅/❌, Redis ✅/❌, scheduler jobs, configured symbols
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

## ETF / SPY Note

SPY is an ETF (Exchange-Traded Fund) that tracks the S&P 500 index. ETFs have no individual earnings, P/E ratio, or margins — all fundamental fields will show `⚪ —`. Only price and technical signals apply.
