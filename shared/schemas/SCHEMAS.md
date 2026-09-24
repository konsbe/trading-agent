# Shared Schemas

JSON Schema 2020-12 documents describing the logical row shape of every TimescaleDB hypertable.
Each schema lives alongside the SQL migrations in `shared/databases/migrations/`.
The schemas are **documentation** — they are not enforced at the DB layer (Postgres/TimescaleDB does
not validate JSON Schema), but can be used by downstream consumers, code generators, or linters.

---

## Migration conventions

Migrations are numbered sequentially (`001_`…) and live in
`shared/databases/migrations/`. Every statement must be idempotent
(`CREATE TABLE IF NOT EXISTS`, `create_hypertable(..., if_not_exists => TRUE)`,
`CREATE INDEX IF NOT EXISTS`), because the directory is mounted at
`/docker-entrypoint-initdb.d` and therefore only auto-runs on a **fresh** volume —
an already-initialised database needs each new migration applied by hand.

### When a migration may be amended

> **A migration may be amended in place only until it has been applied to a
> non-scratch database. After that, every change is a new migration. No exceptions.**

"Scratch" means a throwaway container or a local database you are willing to drop.
Staging, production, and any shared development database are **not** scratch — even
one you rarely think about, because someone else's data is in it.

The reasoning matters more than the rule, because the rule is easy to misapply:
amending a migration that has only ever run against a container you deleted costs
nothing, since no database anywhere has the old version. Amending one that has run
somewhere persistent produces two databases that disagree about what `007` means,
and nothing detects it — the file looks right, the schema looks right, and they are
different. That failure is silent and arbitrarily delayed.

The trap is that the "has it shipped yet?" answer changes over time while the file
does not. Whoever next edits a migration will be making this judgment call months
later, without knowing whether it was deployed in the interim. So: if you cannot
personally confirm the migration has never left scratch, treat it as shipped and
write a new one. A redundant extra migration is free; a divergent schema is not.

### When reusing a state column is a mistake

Job state — `*_status`, `*_claimed_at`, `*_attempts`, `*_last_error` — is tempting
to share between two jobs that need the same shape. The test for whether that is
safe:

> **Would these two failures be distinguishable at 3 a.m.?**

If job A and job B write their status to the same column, then a failure in either
presents identically in the one place an on-call reader would look. The shape being
identical is exactly what makes the confusion possible, and it is cheapest to
notice while designing rather than during an incident.

Worked example: the momentum scanner's bar backfill and its fundamentals fetch both
want per-symbol status, claim timestamp, attempt count and error text. They
deliberately do **not** share `universe_symbols.backfill_*` — a fundamentals
failure appearing in a column named `backfill_last_error` would send a reader to
the wrong pipeline entirely. Separate tables, one per job (§8.4 of
`docs/MOMENTUM_SCANNER_PHASE1.md`).

The corollary is the cheap part: a second small state table costs a migration and
nothing else. Diagnosability is worth more than the saved table.

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
| [watchlist_items](#watchlist_items) | `watchlist_items` | `024_watchlist.sql` | `momentum-api` (watchlist endpoints) |
| [momentum_chain_runs](#momentum_chain_runs) | `momentum_chain_runs` | `025_momentum_chain_runs.sql` | `momentum-scanner`, `momentum-tracker`, `momentum-daily` |
| [api_rate_budget](#api_rate_budget) | `api_rate_budget` | `008_api_rate_budget.sql` | every worker calling a shared-quota API |
| [fundamental_fetch_state](#fundamental_fetch_state) | `fundamental_fetch_state` | `009_fundamental_fetch_state.sql` | `data-fundamental` |

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
features are computed from `equity_ohlcv` rows with `interval = '1Day'` and the
`source` named by `UNIVERSE_BAR_SOURCE`, which must be the value the fetcher
actually writes. (The spec's §7 originally said `'yahoo'`, which matches zero
rows; `internal/fetch/yahoo` writes `'yahoo_finance'`.)

**Why not Alpaca:** every volume feature in the scanner (RVOL, volume
acceleration, dollar volume) requires **consolidated** volume. Alpaca's free tier
serves the IEX feed only, a single-venue fraction of consolidated volume, which
would make all of them wrong.

**Source roles.** `twelve_data` is the Phase 1 primary (split- and
dividend-adjusted with `adjust=all`, volume split-adjusted, ~56 min for 450
symbols). `tiingo` is equally correct but capped at 50 requests per clock hour,
so it is retained for cross-validation rather than as a feed. `yahoo_finance` is
unfit for §3. See `services/data-ingestion/data_ingestion.md`.

**Why an adjusted source and not `yahoo_finance`:** Tiingo's `adj*` fields are split *and*
dividend adjusted. Our Yahoo rows are not dividend adjusted — `internal/fetch/yahoo`
decodes `indicators.quote` and the string `adjclose` appears nowhere in the
package, so the adjustment is absent by construction. `data-analyzer`'s
`QueryEquityBars` therefore prefers `source = 'tiingo'` per timestamp when a
symbol has rows from both; preferring Yahoo made a doubly-covered symbol resolve
to the unadjusted series, which is worse than either source alone.

### `interval = 'quote_snapshot'` is not a bar

`equity_ohlcv` also carries Finnhub `/quote` snapshots written with
`interval = 'quote_snapshot'`, `source = 'finnhub_quote'`, and `volume = 0`.

These are **not** daily bars and nothing reading `interval = '1Day'` can pick
them up. They exist to break the pilot's stratification bootstrap: stratifying by
price needs a price for every symbol, but prices come from the bar backfill that
the stratified draw selects. Finnhub `/quote` supplies an approximate price for
the whole universe through a budget the repo already pays for, at zero cost to
the bar provider's quota. See §8.1.1 of the spec.

Because the snapshot is a live price and not an adjusted close, a symbol's
selection-time price bucket can disagree with its post-backfill bucket.
`LoadSubsetStats` reports that disagreement with examples rather than hiding it.

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

Eligibility (§3.1) requires **all** of: common stock, exchange in NASDAQ / NYSE / NYSE American, no non-common ticker suffix, and ≥ 252 daily bars of history. Ineligible symbols are **retained** with `is_eligible = false` and a populated `excluded_reason` so the exclusion rules stay auditable.

The bar-count rule is the one exception to "applied at symbol-list load": it is enforced as a §3.2 hard gate at scan time, because bars exist only after the backfill, and the backfill iterates the eligible set. `bar_count` here is the audit record, not the gate.

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
| `bar_count` | integer/null | no | Daily bars in `equity_ohlcv`; ≥ 252 required (§3.7's window reads `high[t-251]`) |
| `first_bar_ts` / `last_bar_ts` | datetime/null | no | |
| `is_eligible` | boolean | yes | |
| `excluded_reason` | string/null | no | Null iff eligible. `type_not_common_stock`, `exchange_not_allowed`, `ticker_suffix_excluded`, `insufficient_history` |
| `backfill_status` | string | yes | `pending` / `in_progress` / `done` / `failed` |
| `backfill_cursor_ts` | datetime/null | no | Oldest bar fetched so far — the resume point after a crash |
| `backfill_claimed_at` | datetime/null | no | When `in_progress` was set. Claims older than the lease are reclaimable, so a worker killed mid-symbol does not strand the row |
| `backfill_attempts` | integer | yes | |
| `backfill_last_error` | string/null | no | |
| `backfill_completed_at` | datetime/null | no | |
| `backfill_selected` | boolean | yes | Marks the pilot subset validated on a free tier before the full universe (migration `010`). **Also the de-facto enforcement of Tiingo's 500-unique-symbols/month allowance** |
| `updated_at` | datetime | yes | |

**`backfill_selected` is a quota mechanism, and nothing else enforces it.** Tiingo's
free allowance is 500 *unique symbols per month* — neither a rate nor a daily count,
and therefore not expressible in `api_rate_budget` at all. Re-touching an
already-counted symbol is free, so a stable subset refreshed daily stays inside the
allowance indefinitely, while widening it past ~450 silently spends the month.

No limiter will stop that. The guard has to be an explicit assertion on subset size
in the selection job, where it fails loudly and immediately rather than a month later
when Tiingo starts rejecting requests for reasons that look unrelated to the change
that caused them.

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
| `high_52w` | number/null | `max(high[t-251 .. t-1])`, 251 trading days **excluding** today |
| `pct_of_52w_high` | number/null | `close[t] / high_52w`. **Ratio**, 1.0 = at the prior high, **> 1.0 is a new 52-week high** |

The window excludes the current bar, the same convention as `avg_vol_20` and
`resistance_20`. There is deliberately **no** `new_52w_high` boolean: it would be
redundant with `pct_of_52w_high >= 1.0`.

An earlier revision defined the window over `[t-251 .. t]`, *including* today.
Because `close[t] ≤ high[t] ≤ high_52w`, that capped the ratio at 1.0 and made
§4.2's top band unreachable — every genuine new high scored 4 instead of 5.

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
| `market_cap` | number/null | USD absolute, at `t`, as reported by Finnhub |
| `market_cap_est` | number/null | `shares_outstanding * close[t]`. Documented fallback, used for the gate **only** when `market_cap` is null |
| `market_cap_is_proxy` | boolean | `true` when the gate was evaluated against the estimate. Mirrors `float_is_proxy` |

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
| bars of history | 252 | 252 |

The **upper bound on `change_pct` is central to the strategy**, not a safety
rail: the thesis is entering at +8–15 % on a confirmed move, so a stock up +60 %
today is not a candidate — it is already gone.

**Market cap falls back to `market_cap_est`.** Finnhub's free-tier micro-cap
coverage is poor, and failing the gate on a hard null would silently empty the
penny bucket — exactly the population that bucket exists to scan. When
`market_cap` is null the gate is evaluated against `shares_outstanding * close[t]`,
`market_cap_is_proxy` is set, and **`market_cap_null` stays in `gate_failures`
even though the symbol passed**, so the proxy's contribution stays measurable.
This is not a silent default: §12 forbids substitutions with no formula and no
flag, and this one has an explicit formula, a persisted flag and a retained
failure record. If `shares_outstanding` is also null the gate fails for real.

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

## watchlist_items

**File:** `watchlist_items.schema.json`
**Migration:** `024_watchlist.sql`

Symbols a user chose to follow, written by momentum-api's watchlist endpoints
(`PUT`/`DELETE /api/v1/watchlist/{symbol}`) and read by data-ingestion's
`intraday-bars` job. The only table momentum-api writes.

| Column | Type | Nullable | Notes |
|---|---|---|---|
| `id` | bigint | no | Surrogate key |
| `owner_sub` | text | **yes** | Identity-provider subject of the signed-in user. **NULL = the shared unauthenticated list** (no auth exists yet, so every row is NULL today). Never a placeholder string |
| `symbol` | text | no | Upper-case; CHECK enforced |
| `added_at` | timestamptz | no | Default `now()` |

Unique on `(COALESCE(owner_sub, ''), symbol)` — a plain UNIQUE would let NULL
owners duplicate a symbol.

## momentum_chain_runs

**File:** `momentum_chain_runs.schema.json`
**Migration:** `025_momentum_chain_runs.sql`

Durable progress of the daily chain, one row per NYSE session, so that neither
a restart nor a kill mid-run depends on a process remembering what finished.

| Column | Type | Nullable | Notes |
|---|---|---|---|
| `session` | date | no | Primary key. The scan's modal latest-bar date |
| `attempts` | integer | no | Chain attempts by momentum-daily; the retry limit survives restarts |
| `scanner_completed_at` | timestamptz | **yes** | Set by momentum-scanner **in the same transaction** as the session's features and scores. Non-NULL = the whole scan committed. analyst-bot alerts only on these sessions |
| `tracker_completed_at` | timestamptz | **yes** | Set by momentum-tracker after all rows are evaluated and opened. momentum-daily treats a session as done only when this is set |
| `gave_up_at` | timestamptz | **yes** | momentum-daily gave up (bars never landed, or attempts exhausted) |
| `last_error` | text | **yes** | Why the last attempt or give-up happened |
| `updated_at` | timestamptz | no | Default `now()` |

A give-up is final; to retry a session by hand, see the recovery SQL in
[`services/data-analyzer/data_analyzer.md`](../../services/data-analyzer/data_analyzer.md#momentum-daily--the-scheduled-chain) (`momentum-daily` section).

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

---

# Cross-worker infrastructure

## api_rate_budget

**File:** `api_rate_budget.schema.json`
**Source:** written by `internal/ratelimit` from every worker that calls a shared-quota API.
**Grain:** one row per upstream quota. Regular table — current state, not time-series.

### The problem

Every worker builds its own in-process token bucket. Each is correct alone and
wrong in aggregate: the five Go workers holding a Finnhub client each allow
~0.5 req/s, summing to ~2.5 req/s against a free-tier budget of 1 req/s.

This has been harmless only because observed demand is roughly 10 % of budget —
each worker polls a handful of symbols on a 60-second tick and none approaches its
own allowance. **It stops being harmless the moment one caller saturates its
allowance for hours**, which the momentum scanner's universe-wide fundamentals pass
does. The 429s then surface in whichever *other* worker happens to be running, so
symptom and cause land in different services.

### Why Postgres and not Redis

| | Postgres | Redis |
|---|---|---|
| New dependency for the four existing workers | none — they already hold a `pgxpool` | yes, and no Go client exists in this repo today |
| New partial-failure surface | none — these workers already treat Postgres-down as total outage | a second thing that can fail independently |
| Survives a rolling restart | yes, rows are durable | a naive fixed-window scheme resets, handing out a free burst per deploy |
| Contention cost | ~1 acquisition/second aggregate — noise | lower, but irrelevant at this volume |

The deploy-burst row is the subtle one: it would present as an occasional
unexplained 429 spike after deploys, which is very hard to connect to its cause.

### Columns

| Column | Type | Required | Notes |
|---|---|---|---|
| `budget_key` | string | yes | Names the **quota**, not the worker. `finnhub` is one budget shared by every caller using `FINNHUB_API_KEY`; callers passing different keys share nothing |
| `tokens` | number | yes | Current balance, fractional. Refilled **lazily at read time**, so there is no background job to schedule and nothing to drift |
| `refill_per_sec` | number | yes | Sustained rate. `1.0` for Finnhub free tier |
| `burst` | number | yes | Largest allowed spike. Must be ≥ 1 — a smaller burst could never satisfy a request, so every acquisition would silently degrade |
| `updated_at` | datetime | yes | Advanced on each successful acquisition; the refill baseline |
| `daily_limit` | number/null | no | Hard requests-per-day ceiling (migration `010`). **NULL = no ceiling**, preserving pre-010 behaviour for existing rows |
| `daily_used` | number | yes | Consumed in the current window. Only a *granted* acquisition consumes it — refusals do not |
| `daily_window_start` | date/null | no | UTC calendar day the counter covers. Rolled inside the acquisition statement, so no worker action or scheduled job is needed |

#### The daily ceiling is a second, separate mechanism

A rate limit and a daily quota are not two settings of one thing:

| | Mechanism | Caller's correct response |
|---|---|---|
| `tokens` / `refill_per_sec` / `burst` | continuous refill — paces | wait a moment, retry |
| `daily_*` | fixed window — stops | **come back tomorrow** |

They are kept as separate columns rather than merged because conflating them is
where the bugs live. A token bucket can always eventually grant a request; a spent
daily quota cannot until the window rolls, and `Wait` blocking for fourteen hours
is not a sleep.

Motivating case: Twelve Data's free tier is 8 requests/minute **and** 800
credits/day, where the daily cap binds first — 800 credits at 8/min is ~100 minutes
of work, after which the account is blocked. A limiter expressing only the rate
would pace happily into a wall.

> ### ⚠️ Quota exhaustion is not degradation, and must never use the fallback
>
> `internal/ratelimit` degrades to the caller's local in-process bucket whenever
> coordination fails, and its guarantee is *"never unlimited"*. That is right for
> its designed failure mode: Postgres briefly unreachable means **we do not know
> the state, so be conservative**.
>
> A spent daily quota is the opposite: **we know the state exactly, and it is
> zero.** Falling back there would pace requests against an account with nothing
> left — turning a hard ceiling into a wall of 429s, or on a paid tier into overage
> charges. The mechanism built to prevent unlimited access would be the thing
> granting it, and the limiter's own logs would look healthy throughout.
>
> So exhaustion returns **`ErrDailyQuotaExhausted`**, is never routed through the
> fallback, and is counted in a separate `QuotaExhausted` stat rather than hidden
> inside `Degraded`. Callers stop the pass; the momentum scanner's claim/lease
> checkpoint resumes it after the window rolls, with no new plumbing.
>
> `TestDailyQuota_ExhaustionIsNotDegradationAndNeverUsesFallback` pins this, and it
> is mutation-verified: routing exhaustion through the degradation path makes
> `Wait` return **`nil`** — i.e. "go ahead" — against a spent account, and the test
> fails on exactly that.

#### The UTC-midnight boundary is an assumption, so reconcile it

`daily_window_start` uses the UTC calendar day because that is how most providers
bill, but that has not been verified against any provider's actual reset. Getting it
wrong in the "more room than we thought" direction overruns the account.

Two mitigations, both in place:

- **`SyncDailyUsage`** overwrites the local counter with the provider's own reported consumption. Twelve Data exposes this at `GET /api_usage` (`daily_usage`, `plan_daily_limit`), so drift — including a wrong window boundary — becomes detectable instead of silent.
- **The configured limit is padded below the documented one** (750 against a documented 800) until reconciliation has run across a rollover. A spurious `ErrDailyQuotaExhausted` is handled gracefully by the checkpoint; an actual overrun is untested territory.

### Atomicity

Acquisition is a single `UPDATE` whose `WHERE` clause repeats the refill
expression, so the check and the deduction are one statement. Two callers cannot
both take the last token: the second blocks on the row lock taken by the first,
then re-evaluates the predicate against the committed row under `READ COMMITTED`
and returns zero rows if the balance has fallen below 1. No explicit `FOR UPDATE`,
no advisory lock.

### Degradation

If coordination fails — unreachable database, exceeded acquire deadline, or a
missing budget row — the limiter falls back to the caller's **existing in-process
bucket** for that request. Three properties, each covered by a test:

- **Never unlimited.** The fallback is the rate that shipped before this table existed.
- **Never blocks indefinitely.** Acquisition has a bounded deadline (250 ms default).
- **Per-request, not a latch.** The next call re-attempts coordination, so a brief blip does not strand a worker on local rate.

A `degraded` counter and a throttled warn log exist so that *"why are there 429s in
`data-sentiment`"* resolves to *"the shared limiter degraded"* rather than becoming
a fresh investigation.

---

## fundamental_fetch_state

**File:** `fundamental_fetch_state.schema.json`
**Source:** `data-fundamental`'s checkpointed universe metrics pass.
**Grain:** one row per `(symbol, task)`. Regular table — current state, not time-series.
**Spec:** `docs/MOMENTUM_SCANNER_PHASE1.md` §8.4 (build-order step 3b).

### Why it exists

`data-fundamental`'s sub-tasks iterate a static symbol list and re-fetch all of it
each tick. Fine for three configured symbols; not fine once `runMetrics` is widened
to the eligible universe, where one pass is **~3.3 hours** at the shared Finnhub
rate. A multi-hour rate-limited pass has to survive a restart the same way the bar
backfill does.

### Why a separate table rather than reusing `universe_symbols.backfill_*`

> Would these two failures be distinguishable at 3 a.m.?

The bar backfill and the fundamentals fetch want identically-shaped state, and that
identical shape is exactly what would make them confusable. A fundamentals failure
surfacing in a column named `backfill_last_error` sends an on-call reader to the
wrong pipeline. One state table per job — the extra table costs a migration and
nothing else. See the standing convention near the top of this document.

### Columns

| Column | Type | Required | Notes |
|---|---|---|---|
| `symbol` | string | yes | |
| `task` | string | yes | `metrics` is the only widened sub-task in Phase 1. Part of the key, so **the key is the lease granularity** |
| `status` | string | yes | `pending` / `in_progress` / `done` / `failed` |
| `claimed_at` | datetime/null | no | Set on claim; claims older than the lease are reclaimable, so a killed worker strands nothing |
| `attempts` | integer | yes | Incremented on failure only. Past the cap the row stops being claimed |
| `last_error` | string/null | no | Persisted so a stall is diagnosable from SQL alone |
| `completed_at` | datetime/null | no | Last attempt finished, successful or not |
| `last_success_ts` | datetime/null | no | Last time data was **actually stored** |
| `updated_at` | datetime | yes | |

### `last_success_ts` is the one to understand

It advances only on success, never on failure. Two consequences:

- **The weekly cadence is emergent, not scheduled.** A row is claimable when it is `pending`, when `last_success_ts` is older than the refresh interval, when its claim expired, or when it failed under the attempt cap. There is no cycle boundary to coordinate and nothing to reset — and a symbol newly added to the universe is `pending`, so it is fetched on the next round rather than waiting out a cycle.
- **A broken symbol cannot hide.** If failures advanced freshness, a persistently failing symbol would look current, drop out of the rotation, and make its own staleness invisible. Because they do not, it stays claimable until it exhausts its attempts and is then reported.

This is also why progress is reported as **`fresh`** rather than `done`: `done`
only says the last attempt worked, while `fresh` says the data is inside the refresh
window. `fresh` is the number step 5's candidate counts actually depend on.
