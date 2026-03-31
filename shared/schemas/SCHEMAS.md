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
