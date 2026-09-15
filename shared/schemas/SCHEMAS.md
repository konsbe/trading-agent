# Shared Schemas

JSON Schema 2020-12 documents describing the logical row shape of every TimescaleDB hypertable.
Each schema lives alongside the SQL migrations in `shared/databases/migrations/`.
The schemas are **documentation** — they are not enforced at the DB layer (Postgres/TimescaleDB does
not validate JSON Schema), but can be used by downstream consumers, code generators, or linters.

---

## Table of contents

| Schema file | DB table | Migration | Populated by |
|---|---|---|---|
| [crypto_ohlcv](#crypto_ohlcv) | `crypto_ohlcv` | `001_init.sql` | `data-crypto` |
| [crypto_global_metrics](#crypto_global_metrics) | `crypto_global_metrics` | `001_init.sql` | `data-crypto` |
| [equity_ohlcv](#equity_ohlcv) | `equity_ohlcv` | `001_init.sql` | `data-equity`, `data-technical` (backfill) |
| [macro_fred](#macro_fred) | `macro_fred` | `001_init.sql` | `data-equity` |
| [onchain_metrics](#onchain_metrics) | `onchain_metrics` | `001_init.sql` | `data-onchain` |
| [sentiment_snapshots](#sentiment_snapshots) | `sentiment_snapshots` | `001_init.sql` | `data-sentiment` |
| [news_headlines](#news_headlines) | `news_headlines` | `001_init.sql` | `data-sentiment` |
| [technical_indicators](#technical_indicators) | `technical_indicators` | `002_technical.sql` | `data-technical` |
| [equity_fundamentals](#equity_fundamentals) | `equity_fundamentals` | `003_fundamental.sql` | `data-fundamental` |
| [universe_symbols](#universe_symbols) | `universe_symbols` | `007_momentum.sql` | `data-universe` |
| [momentum_features](#momentum_features) | `momentum_features` | `007_momentum.sql` | `momentum-scanner` |
| [momentum_scores](#momentum_scores) | `momentum_scores` | `007_momentum.sql` | `momentum-scanner` |
| [momentum_labels](#momentum_labels) | `momentum_labels` | `007_momentum.sql` | `momentum-scanner` (backfill mode) |
| [catalyst_events](#catalyst_events) | `catalyst_events` | `007_momentum.sql` | `data-universe` (company news) |
| [momentum_tracked](#momentum_tracked) | `momentum_tracked` | `007_momentum.sql` | `momentum-scanner`, `analyst-bot` |

> Tables introduced by `004_qualitative.sql` (`insider_transactions`), `005_macro_derived.sql`
> (`macro_derived`) and `006_macro_intel.sql` (`economic_calendar_events`,
> `earnings_calendar_events`, `geopolitical_risk_monthly`, `gdelt_macro_daily`,
> `narrative_scores`) are not yet documented here. Pre-existing gap, tracked separately.

---

## crypto_ohlcv

**File:** `crypto_ohlcv.schema.json`  
**Source:** Binance REST kline endpoint (public, no auth required).  
**Grain:** one row per `(exchange, symbol, interval, ts, source)`.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Bar-open timestamp (UTC) |
| `exchange` | string | yes | e.g. `binance` |
| `symbol` | string | yes | e.g. `BTCUSDT` |
| `interval` | string | yes | Binance kline size: `1m`, `1h`, `1d`, … |
| `open` | number | yes | |
| `high` | number | yes | |
| `low` | number | yes | |
| `close` | number | yes | |
| `volume` | number | yes | Quote volume |
| `source` | string | yes | e.g. `binance_kline_rest` |

---

## crypto_global_metrics

**File:** `crypto_global_metrics.schema.json`  
**Source:** CoinGecko `/global` endpoint.  
**Grain:** one row per `(provider, ts)`.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Snapshot time |
| `provider` | string | yes | Default `coingecko` |
| `payload` | object | yes | Raw CoinGecko global JSON (market cap, dominance, etc.) |

---

## equity_ohlcv

**File:** `equity_ohlcv.schema.json`  
**Source:** Alpaca Data API (historical bars + live polling); Finnhub quote snapshot; Yahoo Finance (backfill via `data-technical`).  
**Grain:** one row per `(symbol, interval, ts, source)`.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Bar-open timestamp (UTC) |
| `symbol` | string | yes | e.g. `AAPL`, `SPY` |
| `interval` | string | yes | Alpaca format: `1Min`, `1Hour`, `1Day`, … |
| `open` | number | yes | |
| `high` | number | yes | |
| `low` | number | yes | |
| `close` | number | yes | |
| `volume` | number | yes | |
| `source` | string | yes | e.g. `alpaca`, `yahoo_finance`, `finnhub_quote` |

---

## macro_fred

**File:** `macro_fred.schema.json`  
**Source:** FRED (Federal Reserve Economic Data) via `data-equity`.  
**Grain:** one row per `(series_id, ts)`.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Observation date |
| `series_id` | string | yes | e.g. `DGS10`, `DEXUSEU`, `VIXCLS` |
| `value` | number | yes | Observation value |

---

## onchain_metrics

**File:** `onchain_metrics.schema.json`  
**Source:** Etherscan (ETH supply); Glassnode (optional, key-gated).  
**Grain:** one row per `(asset, metric, ts, source)`.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Snapshot time |
| `asset` | string | yes | e.g. `ETH`, `BTC` |
| `metric` | string | yes | e.g. `eth_supply` |
| `value` | number | no | Scalar metric value (null when only payload is available) |
| `payload` | object | no | Raw response JSON |
| `source` | string | yes | e.g. `etherscan`, `glassnode` |

---

## sentiment_snapshots

**File:** `sentiment_snapshots.schema.json`  
**Source:** LunarCrush (social sentiment) via `data-sentiment`.  
**Grain:** one row per `(source, symbol, ts)`.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Snapshot time |
| `source` | string | yes | e.g. `lunarcrush` |
| `symbol` | string | yes | e.g. `BTC`, `AAPL` |
| `score` | number | no | Normalised sentiment score (null if unavailable) |
| `payload` | object | yes | Raw API response JSON |

---

## news_headlines

**File:** `news_headlines.schema.json`  
**Source:** Finnhub crypto news endpoint via `data-sentiment`.  
**Grain:** one row per `(ts, id)` (auto-increment `id`).

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Publication timestamp |
| `id` | integer | no | Auto-assigned BIGSERIAL |
| `source` | string | yes | e.g. `finnhub_crypto` |
| `symbol` | string/null | no | Ticker if article is symbol-specific |
| `headline` | string | yes | Article headline |
| `url` | string/null | no | Article URL |
| `sentiment` | number/null | no | Pre-computed sentiment score (if provided by source) |
| `payload` | object | no | Raw headline JSON from the API |

---

## technical_indicators

**File:** `technical_indicators.schema.json`  
**Source:** `data-technical` (computes from OHLCV in `equity_ohlcv` / `crypto_ohlcv`).  
**Grain:** one row per `(symbol, exchange, interval, indicator, ts)`.

This table uses a **tall/narrow** layout: each computed number is its own row, identified by the
`indicator` column. Complex indicators store scalar + structured context in `payload`.

### Top-level columns

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Bar-close timestamp the indicator was anchored to |
| `symbol` | string | yes | e.g. `AAPL`, `BTCUSDT` |
| `exchange` | string | yes | `equity` or `binance` |
| `interval` | string | yes | e.g. `1Day`, `1d` |
| `indicator` | string | yes | Parameterised name — see table below |
| `value` | number/null | no | Primary scalar (null for payload-only rows) |
| `payload` | object/null | no | Structured context; shape depends on indicator |

### Indicator catalogue

| Indicator pattern | `value` meaning | Key `payload` fields |
|---|---|---|
| `sma_<N>` | SMA value | — |
| `ema_<N>` | EMA value | — |
| `rsi_<N>` | RSI (0–100) | — |
| `vol_sma_<N>` | Volume SMA | — |
| `rel_vol` | Current vol / vol SMA | — |
| `sr_levels` | Current price | `support[]`, `support_touches[]`, `resistance[]`, `resistance_touches[]`, `current_price` |
| `trend` | Slope % (OLS, last N bars) | `direction` (up/down/sideways), `slope_pct`, `r2`, `higher_highs`, `higher_lows` |
| `candle_patterns` | Pattern sentiment (−1/0/1) | `patterns[]` (names), `bar` (OHLCV) |
| `macd_<fast>_<slow>_<signal>` | Histogram | `macd_line`, `signal_line`, `histogram`, cross flags (`bullish_cross_line_signal`, `bearish_cross_line_signal`, `hist_bull_zero_cross`, `hist_bear_zero_cross`), prior-bar values |
| `obv` | Cumulative OBV | `obv`, `last_bar_delta` |
| `bb_<N>_<std>` | %B | `middle`, `upper`, `lower`, `bandwidth`, `pct_b`, `close` |
| `fib_retrace_sw<N>` | % distance to nearest Fib level | `direction`, `impulse_low/high`, `leg_size`, `levels{}`, `extensions{}`, `nearest_level`, `nearest_price` |
| `rsi_divergence_rsi<N>_sw<N>` | Divergence score (−1/0/1) | `kind` (none/bearish/bullish), `bearish_regular{}`, `bullish_regular{}` |
| `rsi_hidden_rsi<N>_sw<N>` | Hidden div score (−1/0/1) | `kind` (none/bearish_hidden/bullish_hidden), `bearish_hidden{}`, `bullish_hidden{}`, `min_pivot_sep`, `require_trend_gate` |
| `vol_profile_proxy_b<N>_<method>` | POC price | `bins[]` (price_low/high/volume), `poc_price`, `poc_bin`, `value_area_low/high`, `value_area_volume`, `histogram_total_volume` |
| `stoch_slow_<K>_<Ds>_<Dsig>` | %K | `k`, `d`, `raw_k` |
| `atr_<N>` | ATR (Wilder) | — |
| `ichimoku_<T>_<K>_<B>` | Current price | `tenkan`, `kijun`, `senkou_a`, `senkou_b`, `cloud_top`, `cloud_bottom`, `chikou_close`, `close_vs_cloud{}` |
| `ad_line` | Cumulative A/D | `cumulative` |
| `adx_<N>` | ADX | `adx`, `plus_di`, `minus_di`, `dx` |
| `pivots_prior_bar` | Classic PP | `reference_ts`, `classic{}` (PP/R1–R3/S1–S3), `camarilla{}`, `woodie{}` |
| `williams_r_<N>` | Williams %R (−100–0) | — |
| `vwap_rolling_<N>` | VWAP | `vwap`, `bars`, `mode` |
| `vwap_session_last_day` | VWAP | `vwap`, `utc_day`, `mode` |
| `ma_ribbon` | Ribbon compression | `periods[]`, `smas{}`, `bull_stack`, `bear_stack`, `compression`, `golden_cross`, `death_cross` |
| `chart_pattern_hints` | Score (−1/0/1) | `double_top_candidate`, `double_bottom_candidate`, `high1/2`, `low1/2`, `cluster_pct` |
| `cmf_<N>` | Chaikin Money Flow (−1–1) | — |
| `keltner_e<E>_a<A>_m<M>` | Middle EMA | `middle`, `upper`, `lower`, `close`, `outside_upper`, `outside_lower` |
| `donchian_<N>` | Midline | `upper`, `lower`, `middle`, `close` |
| `trendline_break_sw<N>_p<N>` | Break score (−1/0/1) | `resistance_break`, `support_break`, `high/low_line_at_end`, `prev_high/low_line` |
| `cci_<N>` | CCI | — |
| `roc_<N>` | Rate of change % | — |
| `parabolic_sar_s<step>_m<max>` | SAR price | `sar`, `bullish`, `trend` (1/−1) |
| `mfi_<N>` | Money Flow Index (0–100) | — |
| `market_structure_sw<N>` | Structure score (−1/0/1) | `bullish_bos`, `bearish_bos`, `choch_up`, `choch_down`, swing price levels |
| `elliott_context_hint` | Leg estimate count | `swing_highs`, `swing_lows`, `leg_estimate`, `note` |
| `gann_regression_lb<N>` | Slope degrees | `slope_per_bar`, `slope_degrees`, `one_to_one_delta`, `disclaimer` |
| `open_interest` | null | `available: false`, `reason` (data-gap note) |
| `rs_vs_<benchmark>` | Price ratio | `benchmark`, `ratio`, `ratio_change_pct_1`, `asset_roc_1`, `benchmark_roc_1`, `outperformance_1`, `aligned_bars` |
| `mtf_confluence` | Confluence score (0–1) | `primary_interval`, `primary_trend`, `layers[]`, `match_count`, `layer_count`, `confluence_score` |

---

## equity_fundamentals

**File:** `equity_fundamentals.schema.json`  
**Source:** Finnhub `/stock/metric`, `/stock/financials-reported`, `/stock/earnings` via `data-fundamental`.  
**Grain:** one row per `(symbol, period, metric, source, ts)`.  
**No new API key required** — uses the existing `FINNHUB_API_KEY`.

### Top-level columns

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Wall-clock fetch time |
| `symbol` | string | yes | e.g. `AAPL`, `MSFT` |
| `period` | string | yes | `ttm`, `annual_2024`, `q_2024-09-30` |
| `metric` | string | yes | Parameterised metric name — see table below |
| `value` | number/null | no | Primary scalar; null for raw payload-only rows |
| `payload` | object/null | no | Structured context; shape depends on metric |
| `source` | string | yes | `finnhub_metric`, `finnhub_financials_reported`, or `finnhub_earnings` |

### Metric catalogue — Tier 1 FA

**Source: `finnhub_metric` (period = `ttm`)**

| Metric | `value` meaning | Notes |
|---|---|---|
| `eps_ttm` | EPS trailing twelve months | Basic excl. extraordinary items |
| `eps_annual` | EPS most recent annual | |
| `eps_growth_3y` | 3-year EPS CAGR % | |
| `eps_growth_5y` | 5-year EPS CAGR % | |
| `eps_growth_ttm_yoy` | TTM EPS vs prior-year TTM % | |
| `eps_growth_quarterly_yoy` | Latest quarter EPS YoY % | |
| `revenue_ttm` | Total revenue TTM (absolute $) | |
| `revenue_per_share_ttm` | Revenue / diluted shares TTM | |
| `revenue_growth_3y` | 3-year revenue CAGR % | |
| `revenue_growth_5y` | 5-year revenue CAGR % | |
| `revenue_growth_ttm_yoy` | TTM revenue YoY % | |
| `revenue_growth_quarterly_yoy` | Latest quarter revenue YoY % | |
| `pe_ratio_ttm` | Price / TTM EPS | Trailing P/E |
| `pe_ratio_annual` | Price / annual EPS | |
| `pe_ratio_5y_avg` | 5-year normalised P/E average | |
| `pe_ratio_forward` | null | Forward P/E not on free tier; payload has note |
| `fcf_ttm` | Free cash flow TTM ($) | |
| `fcf_per_share_ttm` | FCF / diluted shares TTM | |
| `fcf_yield_1y` | FCF / market cap 1Y % | |
| `fcf_yield_5y` | FCF margin 5Y avg % | |
| `gross_margin_ttm` | Gross profit / revenue TTM % | |
| `gross_margin_annual` | Annual gross margin % | |
| `gross_margin_5y` | 5-year avg gross margin % | |
| `operating_margin_ttm` | Operating income / revenue TTM % | |
| `operating_margin_annual` | Annual operating margin % | |
| `net_margin_ttm` | Net income / revenue TTM % | |
| `net_margin_annual` | Annual net margin % | |
| `net_margin_5y` | 5-year avg net margin % | |
| `market_cap` | Market capitalisation ($M) | Used to derive FCF yield locally |
| `shares_outstanding` | Diluted shares outstanding (M) | |
| `metrics_raw` | null | Full `/stock/metric` JSON in payload |

**Source: `finnhub_financials_reported` (period = `q_YYYY-MM-DD` or `annual_YYYY`)**

| Metric | `value` meaning |
|---|---|
| `revenue_reported` | Top-line revenue from filing |
| `gross_profit_reported` | Gross profit from filing |
| `operating_income_reported` | Operating income/loss |
| `net_income_reported` | Net income/loss |
| `eps_diluted_reported` | Diluted EPS from filing |
| `eps_basic_reported` | Basic EPS from filing |
| `operating_cf_reported` | Cash from operations |
| `capex_reported` | Capital expenditures (absolute value) |
| `fcf_reported` | `operating_cf − abs(capex)`; payload has components |
| `total_assets_reported` | Balance sheet total assets |
| `total_liabilities_reported` | Balance sheet total liabilities |
| `total_equity_reported` | Shareholders' equity |
| `total_debt_reported` | Long-term debt |
| `cash_reported` | Cash & equivalents |
| `report_raw` | null | Full Finnhub report object in payload |

**Source: `finnhub_earnings` (period = `q_YYYY-MM-DD`)**

| Metric | `value` meaning | Notes |
|---|---|---|
| `eps_actual` | Reported EPS | |
| `eps_estimate` | Consensus estimate | |
| `eps_surprise_pct` | `(actual − estimate) / \|estimate\| × 100` | Payload has actual + estimate |
| `earnings_raw` | Reported EPS | Full Finnhub earnings item in payload |

---

# Momentum scanner (Phase 1)

The six tables below are introduced by `007_momentum.sql` and implement
`docs/MOMENTUM_SCANNER_PHASE1.md`. Daily bars are **not** duplicated — momentum
features are computed from `equity_ohlcv` rows with `interval = '1Day'` and
`source = 'yahoo'`.

**Why Yahoo and not Alpaca:** every volume feature in the scanner (RVOL, volume
acceleration, dollar volume) requires **consolidated** volume. Alpaca's free tier
serves the IEX feed only, a single-venue fraction of consolidated volume, which
would make all of them wrong. If the bar source ever changes, re-verify this.

### Unit conventions

Deviating from these is a silent, invisible bug — the numbers still look plausible.

| Convention | Applies to | Meaning |
|---|---|---|
| Percent points | every `*_pct` column | `11.8` means +11.8 % (§3.3 multiplies by 100) |
| **Ratio** | `pct_of_52w_high` | `1.0` = at the 52-week high. **Not** a percent — the Discord embed multiplies by 100 for display |
| Ratio | `rvol_20`, `vol_accel`, `range_20` | plain multiples |
| USD absolute | `market_cap`, `dollar_volume` | Finnhub serves market cap in **$ millions**; the writer multiplies by `1e6` because the §3.2 gates are stated in dollars ($300M–$10B) |
| Share count absolute | `shares_outstanding`, `float_shares_est` | Finnhub serves **millions**; the writer multiplies by `1e6` because the §4.2 float bands are in shares (20M/50M/100M/300M) |

### Null semantics

A null input is never replaced by a default. Specifically:

- `rvol_20` is **null** when `avg_vol_20` is 0 or fewer than 20 prior bars exist. It never becomes 0 or 1, and a null fails the hard gate.
- `catalyst_tier` null means *not looked up* (news is fetched only for gated candidates). The string `'none'` means *looked up, no company news in the window*. These are different states.
- `short_interest_pct`, `days_to_cover`, `is_halted`, `premarket_change_pct` are **always null in Phase 1**. No acceptable free source exists. The columns are present so the features can be added later without a migration.

---

## universe_symbols

**File:** `universe_symbols.schema.json`
**Source:** Finnhub `/stock/symbol?exchange=US`, plus sector/shares/market-cap from `equity_fundamentals`.
**Grain:** one row per `(symbol, exchange)`. Regular table (not a hypertable) — it is current state, not time-series.
**Refresh:** weekly.

Eligibility (§3.1) requires **all** of: common stock, exchange in NASDAQ / NYSE / NYSE American, no non-common ticker suffix, and ≥ 250 daily bars of history. Ineligible symbols are **retained** with `is_eligible = false` and a populated `excluded_reason` so the exclusion rules stay auditable.

| Column | Type | Required | Notes |
|---|---|---|---|
| `symbol` | string | yes | e.g. `AAPL` |
| `exchange` | string | yes | NASDAQ, NYSE, NYSE American. OTC/pink sheets excluded in Phase 1 — poor data quality and thin free-source coverage |
| `mic` | string/null | no | Market identifier code from the symbol list; the reliable exchange discriminator |
| `name` | string/null | no | |
| `type` | string/null | no | Raw instrument type, retained pre-filter for audit |
| `sector` | string/null | no | Also the grouping key for §3.12 sector strength |
| `industry` | string/null | no | |
| `shares_outstanding` | number/null | no | **Absolute share count** |
| `market_cap` | number/null | no | **USD absolute** |
| `fundamentals_ts` | datetime/null | no | When sector/shares/cap were last refreshed |
| `bar_count` | integer/null | no | Daily bars in `equity_ohlcv`; ≥ 250 required |
| `first_bar_ts` / `last_bar_ts` | datetime/null | no | |
| `is_eligible` | boolean | yes | |
| `excluded_reason` | string/null | no | Null iff eligible. `type_not_common_stock`, `exchange_not_allowed`, `ticker_suffix_excluded`, `insufficient_history` |
| `backfill_status` | string | yes | `pending` / `in_progress` / `done` / `failed` |
| `backfill_cursor_ts` | datetime/null | no | Oldest bar fetched so far — the resume point after a crash |
| `backfill_attempts` | integer | yes | |
| `backfill_last_error` | string/null | no | |
| `backfill_completed_at` | datetime/null | no | |
| `updated_at` | datetime | yes | |

The `backfill_*` columns are per-symbol checkpoint state for the resumable 3-year
bar backfill (§8.1.3). They are job state, not features — the backfill must
survive a kill and resume, and this is where it remembers how far it got.

---

## momentum_features

**File:** `momentum_features.schema.json`
**Source:** computed by `momentum-scanner` from `equity_ohlcv` daily bars, reusing `data-analyzer/internal/compute/` for ATR, RSI, Donchian, VWAP and swings.
**Grain:** one row per `(symbol, ts)` — one per symbol per completed trading day. Hypertable on `ts`.

**No lookahead.** Every column is computable from bars up to and including `ts`.
Forward-looking quantities live in `momentum_labels` — the split between the two
tables makes that boundary structural rather than a convention.

`close` and `volume` are denormalised onto the row deliberately: the §4 penalties,
§5 exit rules and §6 labels all reference `close[t]`, and a self-contained feature
row keeps the scorer and label jobs from re-joining `equity_ohlcv`. It is a copy
of the bar, not a parallel bar table.

### §3.3 price and change

| Column | Type | Notes |
|---|---|---|
| `close`, `volume` | number/null | `close[t]`, `volume[t]`. Consolidated volume only |
| `prior_close` | number/null | `close[t-1]` |
| `change_pct` | number/null | `(close[t]/close[t-1] - 1) * 100` |
| `gap_pct` | number/null | `(open[t]/close[t-1] - 1) * 100` |
| `dollar_volume` | number/null | `close[t] * volume[t]`. A required gate, not decoration — without it the penny bucket fills with names where a $50k order moves price 20 % and RVOL is noise |
| `atr_14` | number/null | Wilder ATR, 14 bars |
| `atr_pct` | number/null | `atr_14 / close[t] * 100` |

### §3.4 relative volume

| Column | Type | Notes |
|---|---|---|
| `avg_vol_20` | number/null | `mean(volume[t-20 .. t-1])` — **excludes today**. Including today deflates RVOL exactly when it matters most |
| `rvol_20` | number/null | `volume[t] / avg_vol_20`. Null when `avg_vol_20` is 0 or < 20 prior bars — never 0, never 1 |

### §3.5 volume acceleration

| Column | Type | Notes |
|---|---|---|
| `vol_accel` | number/null | `mean(volume[t-2..t]) / mean(volume[t-7..t-3])`. Null when the 8-bar span `t-7..t` is not fully available |

RVOL and acceleration are **different signals** and both are scored: RVOL asks
"is today unusual versus the last month", acceleration asks "is volume building
or already fading". `vol_accel < 1.0` with high RVOL means volume is decaying,
which §4.3 penalises rather than excludes.

### §3.6 breakout geometry

| Column | Type | Notes |
|---|---|---|
| `resistance_20` | number/null | `max(high[t-20 .. t-1])`, Donchian upper excluding today |
| `range_20` | number/null | `(max(high[t-20..t-1]) - min(low[t-20..t-1])) / close[t]` |
| `was_consolidating` | boolean/null | `range_20 < 0.25` |
| `breakout_state` | string/null | `breakout_from_consolidation` / `breakout` / `approaching` / `none` |

**Judged on the close, never on an intrabar high.** A wick above resistance that
closes back below is not a breakout — this is the main thing separating a real
breakout from a failed one.

### §3.7 52-week high proximity

| Column | Type | Notes |
|---|---|---|
| `high_52w` | number/null | `max(high[t-251 .. t])`, 252 trading days **including** today |
| `pct_of_52w_high` | number/null | `close[t] / high_52w`. **Ratio**, 1.0 = at the high |
| `new_52w_high` | boolean/null | `close[t] > max(high[t-251 .. t-1])` — window **excludes** today |

The two windows differ on purpose: you cannot be a "new high" relative to
yourself. Note the consequence — because `close[t] ≤ high[t] ≤ high_52w`,
`pct_of_52w_high` can never exceed 1.0.

### §3.8 VWAP

| Column | Type | Notes |
|---|---|---|
| `vwap_20` | number/null | Rolling 20-day VWAP, `sum(typical*volume)/sum(volume)` over `t-19..t` |
| `above_vwap` | boolean/null | `close[t] > vwap_20` |
| `vwap_dist_pct` | number/null | `(close[t]/vwap_20 - 1) * 100` |

Daily bars have no intraday VWAP. This is the Phase 1 stand-in for *intraday
session* VWAP; the field names are stable so the Phase 3 swap is a one-place
change.

### §3.9 float

| Column | Type | Notes |
|---|---|---|
| `float_shares_est` | number/null | **Absolute share count.** Shares outstanding used as an approximation of public float |
| `float_is_proxy` | boolean | Always `true` in Phase 1 |

Free sources do not provide true public float. Insider and locked-up shares are
not excluded, so this **overstates** float for recently-IPO'd and insider-heavy
companies. Label it `Float (est)` in any output — never `Float`.

### §3.10–3.12 remaining features

| Column | Type | Notes |
|---|---|---|
| `rsi_14` | number/null | Wilder RSI. **Penalty input only.** A momentum breakout is supposed to have elevated RSI; rewarding low RSI would select against the thesis |
| `catalyst_tier` | string/null | `A` / `B` / `none` / null. Null = not looked up; `none` = looked up, nothing found |
| `catalyst_headline` | string/null | Headline backing the winning tier, for the embed |
| `catalyst_checked_at` | datetime/null | |
| `sector_strength_pct` | number/null | `median(change_pct_5d)` across eligible symbols in the same sector. **Recorded, not scored** — there is no principled weight yet, and guessing one adds noise |
| `change_pct_5d` | number/null | `(close[t]/close[t-5] - 1) * 100`. Input to `sector_strength_pct`; not defined in §3.3 |
| `market_cap` | number/null | USD absolute, at `t` |

### §3.2 gate outcome

| Column | Type | Notes |
|---|---|---|
| `bucket` | string/null | `market` / `penny` / null. Assigned from `close[t]`; the two price bands are disjoint, so price alone decides the bucket and the remaining gates then apply within it |
| `gates_passed` | boolean | `true` = candidate |
| `gate_failures` | text[] | Empty array when passing, so "evaluated and passed" is distinguishable from "not evaluated" |

**A gate failure means excluded, not low-scored.** Scoring only ranks within the
candidate set.

| Gate | `market` | `penny` |
|---|---|---|
| `close` | ≥ $2.00 | $0.30 – $2.00 |
| `market_cap` | $300M – $10B | ≤ $300M |
| `change_pct` | +8 % to +25 % | +10 % to +40 % |
| `rvol_20` | ≥ 3.0 | ≥ 4.0 |
| `dollar_volume` | ≥ $5M | ≥ $2M |
| bars of history | 250 | 250 |

The **upper bound on `change_pct` is central to the strategy**, not a safety
rail: the thesis is entering at +8–15 % on a confirmed move, so a stock up +60 %
today is not a candidate — it is already gone.

### Phase-1-null columns

`short_interest_pct`, `days_to_cover`, `is_halted`, `premarket_change_pct` —
always null, present to avoid a later migration. See the null-semantics note above.

---

## momentum_scores

**File:** `momentum_scores.schema.json`
**Source:** computed by `momentum-scanner` from `momentum_features`.
**Grain:** one row per `(symbol, ts)`. Hypertable on `ts`. Only gated candidates are scored.

**There is exactly one score.** `momentum_score_100` is an integer 0–100 measuring
*setup quality* — "how closely does this setup resemble the profile of a stock
that went on to run?" It does **not** predict a percentage gain. The
+200/+300/+500/+1000 divisions are deferred to Phase 2, where each becomes a
calibrated probability from a classifier. Phase 1 records the `hit_*` labels but
never displays or computes a score for them.

| Column | Type | Max | Measures |
|---|---|---|---|
| `momentum_score_100` | integer | 100 | Final, clamped to `[0,100]` |
| `score_vol_accel` | number/null | 25 | Is volume **building** (`vol_accel`) |
| `score_rvol` | number/null | 20 | Is today **unusual vs the month** (`rvol_20`) |
| `score_breakout` | number/null | 20 | `breakout_state` |
| `score_catalyst` | number/null | 15 | `catalyst_tier` |
| `score_float` | number/null | 10 | `float_shares_est` |
| `score_vwap` | number/null | 5 | `above_vwap` |
| `score_52w` | number/null | 5 | 52-week high proximity |
| `subtotal_before_penalties` | number/null | 100 | Sum of the seven sub-scores |
| `penalty_total` | number/null | ≤ 0 | Sum of applied penalties |
| `penalties` | jsonb | | Applied penalties only, e.g. `{"already_extended": -10}` |
| `null_inputs` | text[] | | Features that were null and therefore scored 0 |

The original weight table listed both `Volume +25` and `Relative Volume +20`,
which double-counted the same quantity. The 25 now measures acceleration and the
20 measures RVOL — two genuinely different signals. **Do not reintroduce a plain
volume score alongside these.**

Sub-scores are all piecewise-linear, interpolated inside each band and clamped at
band edges. A null input scores **0** for its component and is recorded in
`null_inputs` — never imputed.

### Penalties (§4.3)

| Key | Condition | Points |
|---|---|---|
| `rsi_exhausted` | `rsi_14 > 85` | −5 |
| `already_extended` | `change_pct > 20` | −10 |
| `volume_decaying` | `vol_accel < 1.0` **and** `rvol_20 ≥ 3` | −5 |

`already_extended` enforces the core thesis inside the score itself. A same-day
+22 % move is later in the sequence than the intended entry.

Every sub-score is stored separately alongside the pre-penalty subtotal so
`/score` reconstructs the total exactly: a score with no visible breakdown is
undebuggable, and Phase 2 needs the components independently.

---

## momentum_labels

**File:** `momentum_labels.schema.json`
**Source:** computed by `momentum-scanner` in backfill mode.
**Grain:** one row per `(symbol, ts)` where `ts` is the **setup** date. Hypertable on `ts`.

This is the real Phase 1 deliverable — the dataset Phase 2 trains on — and the
part most likely to be skipped under time pressure.

| Column | Type | Notes |
|---|---|---|
| `horizon_days` | integer | `H`, forward window in trading days (default 120) |
| `bars_available` | integer | Forward bars actually present; `< H` ⇒ incomplete |
| `label_complete` | boolean | `false` for rows inside the last `H` sessions. **Incomplete rows must be excluded from any evaluation** |
| `entry_close` | number/null | `close[t]`, the gain denominator, copied for reproducibility |
| `fwd_max_close` | number/null | `max(close[t+1 .. t+H])` |
| `fwd_max_gain_pct` | number/null | `(fwd_max_close / close[t] - 1) * 100` |
| `days_to_peak` | integer/null | Offset of `fwd_max_close`, 1..H |
| `fwd_max_drawdown_pct` | number/null | Largest peak-to-trough decline in the window, negative |
| `hit_100` | boolean/null | `fwd_max_gain_pct ≥ 100` — the Phase 1 evaluation target |
| `hit_200` … `hit_1000` | boolean/null | Recorded as Phase 2 training targets only |

### Rules that keep this honest

- **No lookahead.** Features for day `t` use only bars ≤ `t`; labels use only bars after `t`. Leakage makes the whole exercise worthless and is easy to introduce accidentally — there is a test asserting it.
- **Survivorship bias is present and must be documented.** A universe list pulled today omits delisted tickers, which are disproportionately failures. Phase 1 cannot fix this on free data. Do not present the backtest as unbiased.
- **The base rate is the benchmark.** What fraction of *all* gated candidates hit +100 %? A score is only useful if high deciles beat that. If they don't separate, the §4 weights are wrong and must be revised before anything is built on top.

---

## catalyst_events

**File:** `catalyst_events.schema.json`
**Source:** Finnhub company news, fetched only for symbols that passed the hard gates, over a 48-hour window.
**Grain:** one row per `(symbol, ts, source, headline_hash, matched_keyword)`. Hypertable on `ts`.

**Every** keyword match is stored, not just the winning tier. Phase 2's most
valuable analysis is which specific keywords preceded real runners, and that
requires the raw matches retained.

Phase 1 uses a **keyword classifier, not an LLM** — the point is to discover which
keywords actually correlate with runners before paying for classification.
Keyword lists live in configuration, so they can be tuned without a rebuild.

| Column | Type | Required | Notes |
|---|---|---|---|
| `ts` | datetime | yes | Article publication time |
| `symbol` | string | yes | |
| `source` | string | yes | e.g. `finnhub_company_news` |
| `headline_hash` | string | yes | sha256 hex of the normalised headline (lowercased, whitespace collapsed, trimmed) |
| `matched_keyword` | string | yes | Part of the key — one headline matching several keywords yields several rows |
| `tier` | string | yes | `A` or `B`. The `none` tier is an *absence* of rows, never a stored row |
| `headline` | string | yes | |
| `url` | string/null | no | |
| `scan_ts` | datetime/null | no | The `momentum_features.ts` this lookup was performed for |
| `payload` | object/null | no | Raw news item |
| `ingested_at` | datetime | yes | |

---

## momentum_tracked

**File:** `momentum_tracked.schema.json`
**Source:** written by `momentum-scanner` (and read by `analyst-bot` for `/tracked`).
**Grain:** one row per `(symbol, alerted_ts)`. Regular table — current position state, not time-series.

The original brief specified sell channels but never defined a sell signal. Buy
and sell are **not symmetric**: a sell signal requires knowing what was
previously alerted. Phase 1 defines it as **exit tracking on prior alerts**, not
shorting.

| Column | Type | Required | Notes |
|---|---|---|---|
| `symbol` | string | yes | |
| `alerted_ts` | datetime | yes | The feature-row `ts` that triggered the buy alert |
| `bucket` | string | yes | `market` / `penny` |
| `status` | string | yes | `active` / `closed` |
| `reference_price` | number | yes | `close[t]` at alert; the `stop_atr` basis |
| `score_at_alert` | integer/null | no | |
| `resistance_20_at_alert` | number/null | no | The level it broke out over |
| `atr_14_at_alert` | number/null | no | |
| `last_evaluated_ts` | datetime/null | no | |
| `highest_close_since` | number/null | no | For the `timeout` rule's "no new high since alert" |
| `max_gain_pct` | number/null | no | Realised max favourable move while active |
| `low_rvol_streak` | integer | yes | Consecutive sessions with `rvol_20 < 1.5`; `momentum_stalled` fires at 3 |
| `sessions_elapsed` | integer | yes | Sessions since alert; `timeout` fires at 20 |
| `exit_reason` | string/null | no | See below |
| `exit_ts`, `exit_price`, `exit_pct` | | no | Populated on close |
| `created_at`, `updated_at` | datetime | yes | |

The `*_at_alert` columns are **frozen snapshots** of the levels the exit rules are
measured against. They must never be refreshed — `breakout_failed` compares
today's close to the resistance level as it stood at the alert.

### Exit reasons, evaluated in order — first match wins

| Reason | Condition |
|---|---|
| `breakout_failed` | `close[t] < resistance_20_at_alert` |
| `lost_vwap` | `close[t] < vwap_20` |
| `momentum_stalled` | `rvol_20 < 1.5` for 3 consecutive sessions |
| `stop_atr` | `close[t] < reference_price - 2 * atr_14_at_alert` |
| `timeout` | 20 sessions elapsed, no exit condition met, and no new high since alert |

Recording the realised outcome makes this free labelling data that doubles as a
live measure of whether the score is worth anything.

**These exit rules are a starting default, not a validated strategy.** They are
deliberately mechanical so the backtest can evaluate and replace them. Nothing
here constitutes trading advice; the score is a research output, not a
recommendation.
