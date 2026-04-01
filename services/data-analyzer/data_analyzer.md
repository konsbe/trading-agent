# Data Analyzer Layer

The `data-analyzer` service processes raw data stored by `data-ingestion` workers and derives **signals** — scored, classified, and structured outputs ready for the `analyst-bot` to consume. It never calls any external API. All inputs come from TimescaleDB.

It contains two independent analysis services:

| Service | Binary | Reads from | Writes to |
|---|---|---|---|
| `technical-analysis` | `cmd/technical-analysis` | `equity_ohlcv`, `crypto_ohlcv`, `macro_fred` | `technical_indicators` |
| `fundamental-analysis` | `cmd/fundamental-analysis` | `equity_fundamentals` (raw metrics) | `equity_fundamentals` (derived period) |

---

## Architecture Overview

```
data-ingestion workers
        │
        ├── equity_ohlcv / crypto_ohlcv (OHLCV bars)
        ├── macro_fred (VIXCLS, DGS10, FX rates)
        └── equity_fundamentals (raw TTM ratios + XBRL financials)
                │
                ▼
     data-analyzer (no external API calls)
        │
        ├── technical-analysis ──→ technical_indicators
        │     (49+ indicators per symbol × interval)
        │
        └── fundamental-analysis ──→ equity_fundamentals (period = "derived")
              (11 scored signals + 3 margin trend signals)
```

Both services start with a configurable **startup delay** to let `data-ingestion` workers complete their initial backfill before the first computation run.

---

## Service 1 — `technical-analysis`

### What it does

Reads OHLCV bars from `equity_ohlcv` and `crypto_ohlcv`, computes every enabled technical indicator group in sequence, and writes results to `technical_indicators`. It calls **zero external APIs** — all computation is pure math over the bars already in the database.

### Data inputs

| Table read | What it needs | Required by |
|---|---|---|
| `equity_ohlcv` | Daily bars (interval `1Day`) for equity symbols | All indicators |
| `crypto_ohlcv` | Daily bars (interval `1d`) for crypto symbols | All indicators |
| `equity_ohlcv` | Weekly bars (interval `1Week`) | Weekly pivot points |
| `crypto_ohlcv` | Weekly bars (interval `1w`) | Weekly pivot points |
| `macro_fred` | Latest `VIXCLS` value | VIX regime classification |

Ingestion workers that must run first: **`data-technical`** (daily bar backfill), **`data-equity`** (FRED / VIXCLS data).

### Table written: `technical_indicators`

One row per `(symbol, exchange, interval, indicator)` per computation run. Writes are upserts — re-running the same bars produces identical rows and does not duplicate data.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | Last bar's close timestamp |
| `symbol` | TEXT | `AAPL`, `BTCUSDT` |
| `exchange` | TEXT | `equity`, `binance` |
| `interval` | TEXT | `1Day`, `1d` |
| `indicator` | TEXT | `rsi_14`, `macd_12_26_9` |
| `value` | DOUBLE PRECISION | Primary numeric value for the indicator |
| `payload` | JSONB | Full detail (sub-values, thresholds, breakdowns) |

### Indicators computed

Each group can be independently toggled on/off via `TECHNICAL_ENABLE_*` env vars.

#### Price & Structure

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `sma_20`, `sma_50`, `sma_100`, `sma_200` | Simple Moving Averages | — |
| `ema_9`, `ema_21`, `ema_50`, `ema_200` | Exponential Moving Averages | — |
| `ma_ribbon` | SMA ribbon compression score + golden/death cross detection | `bull_stack`, `bear_stack`, `golden_cross`, `death_cross`, `compression` |
| `trend` | Linear regression slope over `TECHNICAL_TREND_LOOKBACK` bars | `direction`, `slope_pct`, `r2`, `higher_highs`, `higher_lows` |
| `sr_levels` | Swing-based support & resistance zones | `support[]`, `resistance[]`, `touches[]` |
| `trendline_break_sw5_p3` | Break of a linear regression trendline drawn through last N swing pivots | `resistance_break`, `support_break` |
| `fib_retrace_sw5` | Fibonacci retracement + extension levels from last impulse swing | `levels` (0, 0.236, 0.382, 0.5, 0.618, 0.786, 1), `extensions` (1.272, 1.618), `direction`, `nearest_level` |
| `chart_pattern_hints` | Double-top / double-bottom candidates via swing clustering | `double_top_candidate`, `double_bottom_candidate` |
| `pivots_prior_bar` | Pivot levels computed from the prior bar | `classic` (PP, R1–R3, S1–S3), `camarilla` (R1–R4, S1–S4), `woodie` (PP, R1–R2, S1–S2) |
| `pivots_weekly` | Same pivot types from prior completed weekly bar | Same as above |
| `pivots_monthly` | Same pivot types from prior completed monthly bar | Same as above (disabled by default) |
| `candle_patterns` | Single and multi-bar candlestick pattern detection (last N bars) | `patterns[]` (e.g. `doji`, `hammer`, `engulfing`), bar OHLCV |
| `hs_pattern_sw5` | Head & Shoulders + Inverse H&S detection via swing pivots | `hs_found`, `hs_head`, `hs_neckline`, `hs_neckline_break`, `inv_hs_found`, symmetry % |
| `triangle_sw3` | Ascending / Descending / Symmetrical triangle via linear regression on swings | `kind`, `high_slope_pct`, `low_slope_pct`, `apex_bars_away`, `breakout` |
| `flag_pole5_len10` | Bull/bear flag & pennant detection (impulse pole + consolidation) | `bull_flag`, `bear_flag`, `pole_pct`, `max_retracement_pct` |
| `elliott_context_hint` | Swing high/low pivot count as an Elliott Wave leg estimate hint | `swing_highs`, `swing_lows`, `leg_estimate`. **Not full wave labelling** — human discretion required |
| `gann_regression_lb60` | Linear regression slope expressed as a Gann angle in degrees | `slope_degrees`, `slope_per_bar`. Geometric scaling not applied — illustrative only |

#### Momentum & Oscillators

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `rsi_14` | RSI over last 14 bars (Wilder smoothing) | — |
| `rsi_divergence_rsi14_sw5` | Regular bullish/bearish RSI divergence | `kind`, `price_lo_1/2`, `rsi_lo_1/2` |
| `rsi_hidden_rsi14_sw5` | Hidden (continuation) RSI divergence | `kind`, `require_trend_gate` |
| `macd_12_26_9` | MACD line, signal, histogram | `macd_line`, `signal_line`, `histogram`, `bullish_cross_line_signal`, `hist_bull_zero_cross` |
| `stoch_slow_14_3_3` | Slow Stochastic %K and %D | `k`, `d`, `raw_k` |
| `cci_20` | Commodity Channel Index | — |
| `roc_12` | Rate of Change (12 bars) | — |
| `williams_r_14` | Williams %R | — |
| `parabolic_sar_s0.02_m0.2` | Parabolic SAR stop-and-reverse | `sar`, `bullish`, `trend` (+1 or -1) |

#### Trend Strength

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `adx_14` | Average Directional Index (Wilder) | `adx`, `plus_di`, `minus_di`, `dx` |
| `atr_14` | Average True Range (Wilder) | — |
| `ichimoku_9_26_52` | Full Ichimoku cloud (Tenkan, Kijun, Senkou A/B, Chikou) | `close_vs_cloud` (`above_cloud`, `below_cloud`, `in_cloud`) |

#### Volume Indicators

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `vol_sma_20` | 20-bar volume simple moving average | — |
| `rel_vol` | Relative volume vs 20-bar average | — |
| `obv` | On-Balance Volume (cumulative) | `last_bar_delta` |
| `ad_line` | Accumulation/Distribution Line (cumulative) | `cumulative` |
| `mfi_14` | Money Flow Index | — |
| `cmf_21` | Chaikin Money Flow | — |
| `vwap_rolling_20` | Rolling VWAP over last 20 bars | `bars`, `mode` |
| `vol_profile_proxy_b48_typical_price` | Volume-at-price histogram proxy (48 bins, typical price method). **Not true tick-level VPVR** — each bar's full volume is assigned to one bin | `poc_price`, `value_area_low/high`, `bins[]` |

#### Volatility

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `bb_20_2` | Bollinger Bands (20-period, 2σ) | `upper`, `middle`, `lower`, `pct_b`, `bandwidth` |
| `keltner_e20_a10_m2` | Keltner Channels (EMA 20, ATR 10, mult 2) | `upper`, `middle`, `lower`, `outside_upper/lower` |
| `bb_squeeze` | Bollinger Squeeze signal: BB bands fully inside Keltner → low-volatility coil before expansion | `squeeze` (bool), `bb_lower/upper`, `keltner_lower/upper` |
| `donchian_20` | Donchian Channel (20-bar price range) | `upper`, `lower`, `middle` |
| `vix_regime` | VIX classification read from `macro_fred.VIXCLS` | `vix`, `regime` (`extreme_fear`, `elevated`, `normal`, `complacency`) |

#### Advanced & Smart Money Concepts (SMC)

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `fvg_min0.1_lb50` | Fair Value Gaps: 3-candle imbalance where `candle[n-2].high < candle[n].low` (bullish) or `candle[n-2].low > candle[n].high` (bearish) | `active_count`, `total_count`, `last_bullish/bearish` (gap range + bar index) |
| `order_blocks_sw3_imp1.5` | Order Blocks: last opposing candle before a minimum 1.5% impulse move | `active_count`, `last_bullish_ob`, `last_bearish_ob` (OHLC + bar index) |
| `liquidity_sweep_sw3` | Liquidity Sweeps: price wicks through a prior swing high/low and closes back inside | `total_sweeps`, `high_sweeps`, `low_sweeps`, `last_sweep` (swept level, bar OHLC) |
| `market_structure_sw5` | BOS (Break of Structure) and CHoCH (Change of Character) | `bullish_bos`, `bearish_bos`, `choch_up`, `choch_down`, swing levels |

#### Cross-Asset & Multi-Timeframe

| Indicator name in DB | What it computes | Key payload fields |
|---|---|---|
| `rs_vs_spy` | Relative strength ratio vs benchmark (e.g. SPY for equity) | `ratio`, `ratio_change_pct_1`, `outperformance_1`, `aligned_bars` |
| `mtf_confluence` | Trend agreement across multiple timeframes | `confluence_score` (0–1), `layers[]` per secondary interval |

### How a computation run works

1. For each `symbol × interval` combination, query the last `TECHNICAL_COMPUTE_LOOKBACK` bars from `equity_ohlcv` or `crypto_ohlcv`
2. Extract `closes[]`, `highs[]`, `lows[]`, `volumes[]` slices from bars
3. Run each enabled indicator group in sequence — all pure in-memory math, no API calls
4. Anchor `ts` to the **last bar's close timestamp** — so re-running against the same data is fully idempotent
5. Upsert each result to `technical_indicators` via `ON CONFLICT DO UPDATE`

### Startup behaviour

On first launch, waits `ANALYZER_STARTUP_DELAY_SECS` (default 60s) for `data-technical` to finish its OHLCV backfill, then runs a full computation pass immediately. After that, polls on `DATA_TECHNICAL_POLL_INTERVAL` (default 6h).

### Environment variables

#### General

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `ANALYZER_STARTUP_DELAY_SECS` | `60` | Wait time on startup before first computation run |
| `DATA_TECHNICAL_POLL_INTERVAL` | `6h` | How often to recompute all indicators |
| `TECHNICAL_COMPUTE_LOOKBACK` | `500` | Number of bars to query per symbol × interval |

#### Symbols & intervals

| Variable | Default | Description |
|---|---|---|
| `TECHNICAL_EQUITY_SYMBOLS` | `AAPL,MSFT,SPY` | Equity symbols to compute indicators for |
| `TECHNICAL_EQUITY_INTERVALS` | `1Day` | Equity bar intervals (comma-separated) |
| `TECHNICAL_CRYPTO_SYMBOLS` | `BTCUSDT,ETHUSDT` | Crypto pairs |
| `TECHNICAL_CRYPTO_INTERVALS` | `1d` | Crypto bar intervals |

#### Indicator parameters

| Variable | Default | Description |
|---|---|---|
| `TECHNICAL_SMA_PERIODS` | `20,50,100,200` | Comma-separated SMA periods |
| `TECHNICAL_EMA_PERIODS` | `9,21,50,200` | Comma-separated EMA periods |
| `TECHNICAL_RSI_PERIOD` | `14` | RSI lookback period |
| `TECHNICAL_VOL_SMA_PERIOD` | `20` | Volume SMA period |
| `TECHNICAL_MACD_FAST` | `12` | MACD fast EMA |
| `TECHNICAL_MACD_SLOW` | `26` | MACD slow EMA |
| `TECHNICAL_MACD_SIGNAL` | `9` | MACD signal EMA |
| `TECHNICAL_BB_PERIOD` | `20` | Bollinger Bands period |
| `TECHNICAL_BB_STD` | `2` | Bollinger Bands standard deviation multiplier |
| `TECHNICAL_ATR_PERIOD` | `14` | ATR period |
| `TECHNICAL_ADX_PERIOD` | `14` | ADX period |
| `TECHNICAL_STOCH_K` | `14` | Stochastic %K period |
| `TECHNICAL_STOCH_D_SMOOTH` | `3` | Stochastic %D smoothing |
| `TECHNICAL_STOCH_D_SIGNAL` | `3` | Stochastic %D signal |
| `TECHNICAL_WILLIAMS_R_PERIOD` | `14` | Williams %R period |
| `TECHNICAL_CCI_PERIOD` | `20` | CCI period |
| `TECHNICAL_ROC_PERIOD` | `12` | ROC period |
| `TECHNICAL_MFI_PERIOD` | `14` | MFI period |
| `TECHNICAL_CMF_PERIOD` | `21` | Chaikin Money Flow period |
| `TECHNICAL_DONCHIAN_PERIOD` | `20` | Donchian Channel period |
| `TECHNICAL_PARABOLIC_STEP` | `0.02` | Parabolic SAR acceleration factor step |
| `TECHNICAL_PARABOLIC_MAX_AF` | `0.2` | Parabolic SAR maximum acceleration factor |
| `TECHNICAL_GANN_LOOKBACK` | `60` | Bars for Gann regression |
| `TECHNICAL_KELTNER_EMA` | `20` | Keltner EMA period |
| `TECHNICAL_KELTNER_ATR` | `10` | Keltner ATR period |
| `TECHNICAL_KELTNER_MULT` | `2` | Keltner multiplier |
| `TECHNICAL_ICHIMOKU_TENKAN` | `9` | Ichimoku Tenkan-sen period |
| `TECHNICAL_ICHIMOKU_KIJUN` | `26` | Ichimoku Kijun-sen period |
| `TECHNICAL_ICHIMOKU_SPAN_B` | `52` | Ichimoku Senkou Span B period |
| `TECHNICAL_ICHIMOKU_DISPLACE` | `0` | Ichimoku displacement (0 = use Kijun value) |
| `TECHNICAL_VWAP_MODE` | `rolling` | `rolling` or `session` |
| `TECHNICAL_VWAP_ROLLING_N` | `20` | Rolling VWAP bar count |
| `TECHNICAL_VWAP_USE_TYPICAL` | `true` | Use typical price `(H+L+C)/3` for VWAP weighting |
| `TECHNICAL_SR_SWING_STRENGTH` | `5` | Pivot swing strength for S/R detection |
| `TECHNICAL_SR_LEVELS` | `3` | Max S/R levels to store per side |
| `TECHNICAL_SR_CLUSTER_PCT` | `0.5` | Cluster tolerance % to merge nearby S/R levels |
| `TECHNICAL_TREND_LOOKBACK` | `60` | Bars for trend regression |
| `TECHNICAL_TRENDLINE_PIVOTS` | `3` | Pivot count for trendline break |
| `TECHNICAL_FIB_SWING_STRENGTH` | `0` | Fib pivot strength (0 = inherit SR strength) |
| `TECHNICAL_FIB_EXTENSIONS` | `true` | Include Fibonacci extension levels |
| `TECHNICAL_RSI_DIV_SWING_STRENGTH` | `0` | RSI divergence pivot strength (0 = inherit) |
| `TECHNICAL_RSI_HIDDEN_MIN_PIVOT_SEP` | `3` | Min bars between pivots for hidden divergence |
| `TECHNICAL_RSI_HIDDEN_REQUIRE_TREND` | `true` | Gate hidden divergence on trend direction |
| `TECHNICAL_RIBBON_PERIODS` | `10,20,50,200` | SMA periods for the MA ribbon |
| `TECHNICAL_MA_CROSS_FAST` | `50` | Fast SMA for golden/death cross |
| `TECHNICAL_MA_CROSS_SLOW` | `200` | Slow SMA for golden/death cross |
| `TECHNICAL_CHART_PATTERN_CLUSTER_PCT` | `0.5` | Max % spread to cluster double-top/bottom highs |
| `TECHNICAL_VOL_PROFILE_BINS` | `48` | Volume profile histogram bin count |
| `TECHNICAL_VOL_PROFILE_TYPICAL` | `true` | Use typical price for bin assignment |
| `TECHNICAL_VOL_PROFILE_VALUE_AREA_PCT` | `0.70` | Value area coverage target (70% of volume) |
| `TECHNICAL_CANDLE_WINDOW` | `3` | Last N bars scanned for candlestick patterns |
| `TECHNICAL_WEEKLY_PIVOT_LOOKBACK` | `10` | Weekly bars to query for weekly pivots |
| `TECHNICAL_MONTHLY_PIVOT_LOOKBACK` | `5` | Monthly bars to query for monthly pivots |
| `TECHNICAL_WEEKLY_PIVOT_EQUITY_INTERVAL` | `1Week` | Interval label for equity weekly bars |
| `TECHNICAL_WEEKLY_PIVOT_CRYPTO_INTERVAL` | `1w` | Interval label for crypto weekly bars |
| `TECHNICAL_MONTHLY_PIVOT_EQUITY_INTERVAL` | `1Month` | Interval label for equity monthly bars |
| `TECHNICAL_MONTHLY_PIVOT_CRYPTO_INTERVAL` | `1M` | Interval label for crypto monthly bars |

#### SMC parameters

| Variable | Default | Description |
|---|---|---|
| `TECHNICAL_FVG_MIN_GAP_PCT` | `0.1` | Minimum gap size (%) to qualify as an FVG |
| `TECHNICAL_FVG_LOOKBACK` | `50` | Bars to search for FVGs |
| `TECHNICAL_OB_SWING_STRENGTH` | `3` | Swing strength for Order Block detection |
| `TECHNICAL_OB_IMPULSE_MIN_PCT` | `1.5` | Minimum impulse move % after an Order Block |
| `TECHNICAL_OB_LOOKBACK` | `100` | Bars to search for Order Blocks |
| `TECHNICAL_LIQUIDITY_SWING_STRENGTH` | `3` | Swing strength for Liquidity Sweep detection |
| `TECHNICAL_LIQUIDITY_LOOKBACK` | `50` | Bars to search for Liquidity Sweeps |

#### VIX regime thresholds

| Variable | Default | Description |
|---|---|---|
| `TECHNICAL_VIX_FEAR_THRESHOLD` | `35` | VIX above this = `extreme_fear` regime |
| `TECHNICAL_VIX_ELEVATED_THRESHOLD` | `20` | VIX above this = `elevated` regime |
| `TECHNICAL_VIX_COMPLACENCY_THRESHOLD` | `12` | VIX below this = `complacency` regime |

#### Chart pattern parameters

| Variable | Default | Description |
|---|---|---|
| `TECHNICAL_HS_SWING_STRENGTH` | `5` | Pivot strength for H&S detection |
| `TECHNICAL_HS_TOLERANCE_PCT` | `15` | Max shoulder height asymmetry % |
| `TECHNICAL_HS_LOOKBACK` | `100` | Bars to scan for H&S |
| `TECHNICAL_TRIANGLE_SWING_STRENGTH` | `3` | Pivot strength for triangle detection |
| `TECHNICAL_TRIANGLE_MIN_PIVOTS` | `3` | Minimum pivots per trendline for triangles |
| `TECHNICAL_TRIANGLE_FLAT_THRESHOLD_PCT` | `0.05` | Slope below this % = "flat" trendline |
| `TECHNICAL_TRIANGLE_LOOKBACK` | `100` | Bars to scan for triangles |
| `TECHNICAL_FLAG_POLE_PCT` | `5.0` | Minimum pole move % to qualify as a flag |
| `TECHNICAL_FLAG_MAX_RETRACEMENT_PCT` | `50.0` | Max retracement % of the pole in the flag body |
| `TECHNICAL_FLAG_POLE_LEN` | `5` | Number of bars for the flag pole |
| `TECHNICAL_FLAG_LEN` | `10` | Number of bars for the flag body |

#### Relative strength & multi-timeframe

| Variable | Default | Description |
|---|---|---|
| `TECHNICAL_RS_BENCHMARK_EQUITY` | — | Equity benchmark symbol for relative strength (e.g. `SPY`) |
| `TECHNICAL_RS_BENCHMARK_CRYPTO` | — | Crypto benchmark symbol (e.g. `BTCUSDT`) |
| `TECHNICAL_RS_MIN_ALIGNED_BARS` | `30` | Minimum timestamp-aligned bars required |
| `TECHNICAL_MTF_EQUITY_INTERVALS` | — | Secondary equity intervals for MTF confluence |
| `TECHNICAL_MTF_CRYPTO_INTERVALS` | — | Secondary crypto intervals for MTF confluence |

#### Feature toggles (all default `true` unless noted)

| Variable | Default |
|---|---|
| `TECHNICAL_ENABLE_MA` | `true` |
| `TECHNICAL_ENABLE_RSI` | `true` |
| `TECHNICAL_ENABLE_VOLUME` | `true` |
| `TECHNICAL_ENABLE_SR` | `true` |
| `TECHNICAL_ENABLE_TREND` | `true` |
| `TECHNICAL_ENABLE_CANDLES` | `true` |
| `TECHNICAL_ENABLE_MACD` | `true` |
| `TECHNICAL_ENABLE_OBV` | `true` |
| `TECHNICAL_ENABLE_BOLLINGER` | `true` |
| `TECHNICAL_ENABLE_FIB` | `true` |
| `TECHNICAL_ENABLE_RSI_DIVERGENCE` | `true` |
| `TECHNICAL_ENABLE_VOL_PROFILE_PROXY` | `true` |
| `TECHNICAL_ENABLE_RSI_HIDDEN` | `true` |
| `TECHNICAL_ENABLE_STOCHASTIC` | `true` |
| `TECHNICAL_ENABLE_ATR` | `true` |
| `TECHNICAL_ENABLE_ICHIMOKU` | `true` |
| `TECHNICAL_ENABLE_AD_LINE` | `true` |
| `TECHNICAL_ENABLE_ADX` | `true` |
| `TECHNICAL_ENABLE_PIVOTS` | `true` |
| `TECHNICAL_ENABLE_WILLIAMS_R` | `true` |
| `TECHNICAL_ENABLE_VWAP` | `true` |
| `TECHNICAL_ENABLE_MA_RIBBON` | `true` |
| `TECHNICAL_ENABLE_CHART_PATTERNS` | `true` |
| `TECHNICAL_ENABLE_CMF` | `true` |
| `TECHNICAL_ENABLE_KELTNER` | `true` |
| `TECHNICAL_ENABLE_BB_SQUEEZE` | `true` |
| `TECHNICAL_ENABLE_DONCHIAN` | `true` |
| `TECHNICAL_ENABLE_TRENDLINE_BREAK` | `true` |
| `TECHNICAL_ENABLE_CCI` | `true` |
| `TECHNICAL_ENABLE_ROC` | `true` |
| `TECHNICAL_ENABLE_PARABOLIC_SAR` | `true` |
| `TECHNICAL_ENABLE_MFI` | `true` |
| `TECHNICAL_ENABLE_MARKET_STRUCTURE` | `true` |
| `TECHNICAL_ENABLE_ELLIOTT_HINT` | `true` |
| `TECHNICAL_ENABLE_GANN_HINT` | `true` |
| `TECHNICAL_ENABLE_ORDER_BLOCKS` | `true` |
| `TECHNICAL_ENABLE_FVG` | `true` |
| `TECHNICAL_ENABLE_LIQUIDITY_SWEEP` | `true` |
| `TECHNICAL_ENABLE_VIX_REGIME` | `true` |
| `TECHNICAL_ENABLE_WEEKLY_PIVOTS` | `true` |
| `TECHNICAL_ENABLE_HS_PATTERN` | `true` |
| `TECHNICAL_ENABLE_TRIANGLE` | `true` |
| `TECHNICAL_ENABLE_FLAG` | `true` |
| `TECHNICAL_ENABLE_MONTHLY_PIVOTS` | **`false`** |
| `TECHNICAL_ENABLE_RS_BENCHMARK` | **`false`** |
| `TECHNICAL_ENABLE_MTF_CONFLUENCE` | **`false`** |
| `TECHNICAL_ENABLE_OPEN_INTEREST_INFO` | **`false`** |

---

## Service 2 — `fundamental-analysis`

### What it does

Reads raw fundamental metrics stored by `data-fundamental` (period = `ttm`, `q_*`, `annual_*`) and scores them against configurable thresholds to produce **Tier 1 FA signals** — qualitative tiers and a composite quality score. Results are written back into `equity_fundamentals` with `period = "derived"` and `source = "fundamental_analysis"`.

This service calls **zero external APIs**. All inputs come from `equity_fundamentals` rows already stored by `data-fundamental`.

### Data inputs

| Table read | Period filter | What it uses |
|---|---|---|
| `equity_fundamentals` | `ttm` | All latest TTM ratios (EPS, revenue growth, P/E, FCF, margins, PEG) |
| `equity_fundamentals` | `q_*` (quarterly) | `revenue_reported`, `gross_profit_reported`, `operating_income_reported`, `net_income_reported` — for 8-quarter margin trend analysis |
| `equity_fundamentals` | any | `eps_surprise_pct` — all quarterly entries for rolling earnings surprise average |

Ingestion service that must run first: **`data-fundamental`** (from `data-ingestion`).

### Table written: `equity_fundamentals`

Derived rows are written back into the same table with `period = "derived"` and `source = "fundamental_analysis"`. Every upsert is idempotent.

### Derived signals computed

Each run computes up to **14 derived metrics** per symbol:

#### Scoring signals (contribute to composite score)

| Metric name | What it measures | Score values | Tier labels |
|---|---|---|---|
| `eps_strength` | EPS YoY growth rate vs thresholds | `+1` strong, `0` neutral, `-1` weak | `strong`, `neutral`, `weak` |
| `revenue_strength` | Revenue TTM YoY growth rate | `+1` / `0` / `-1` | `strong`, `neutral`, `weak` |
| `pe_vs_5y_mean` | Trailing P/E deviation from own 5-year mean | `+1` cheap, `0` fair, `-1` expensive | `cheap_vs_history`, `fair_vs_history`, `expensive_vs_history` (or absolute fallback: `value`, `growth_fair`, `expensive`) |
| `fcf_yield_tier` | FCF ÷ Market Cap × 100 vs thresholds | `+1` attractive, `0` fair, `-1` avoid | `attractive`, `fair`, `avoid` |
| `gross_margin_tier` | Gross margin % as a moat indicator | `+1` strong moat, `0` average, `-1` pressure | `strong_moat`, `average`, `margin_pressure` |
| `net_margin_tier` | Net margin % profitability tier | `+1` / `0` / `-1` | `strong`, `average`, `pressure` |
| `peg_tier` | PEG ratio (P/E ÷ EPS growth) — growth-adjusted valuation | `+1` undervalued, `0` fair, `-0.5` expensive | `undervalued_growth`, `fairly_valued_growth`, `expensive_growth` |

#### Informational signals (stored but not in composite score)

| Metric name | What it measures | Payload detail |
|---|---|---|
| `fcf_yield` | Raw FCF yield % (derived from `fcf_yield_1y` or `fcf_ttm ÷ market_cap`) | `fcf_yield_pct`, `source` (which formula was used) |
| `fcf_eps_divergence` | Red flag: high EPS growth + low FCF yield = suspect earnings quality | `quality` (`warning_eps_growing_fcf_low`, `high_quality_earnings`, `normal`) |
| `operating_margin_signal` | Operating margin % with tier label | `operating_margin_pct`, `tier` |
| `pe_compression` | Forward P/E vs trailing P/E change % | `direction` (`compressing`, `flat`, `expanding`), `trailing_pe`, `forward_pe` |
| `earnings_surprise_avg` | Rolling average EPS surprise % over last N quarters | `avg_surprise_pct`, `quarters_sampled`, `tier` (`beat`, `inline`, `miss`) |

#### Composite score

| Metric name | What it measures |
|---|---|
| `composite_score` | Mean of all scored components normalised to `[-1, +1]`. Tier: `strong` (≥ 0.5), `neutral`, `weak` (≤ -0.5). Payload includes all component names and the method version. |

#### Margin trend (8-quarter analysis)

| Metric name | What it measures |
|---|---|
| `gross_margin_trend_8q` | Compares gross margin in the most recent 2 quarters vs 2 oldest quarters in the series. `+1` expanding, `0` stable, `-1` compressing. |
| `operating_margin_trend_8q` | Same analysis for operating margin |
| `net_margin_trend_8q` | Same analysis for net margin |

**How margin trend is computed:**
```
margin(q) = income_line(q) ÷ revenue_reported(q) × 100
recent_mean = mean(margin[Q0], margin[Q1])
old_mean    = mean(margin[Qn-1], margin[Qn])
diff = recent_mean − old_mean

if diff > FUNDAMENTAL_MARGIN_TREND_STABLE_PP  → expanding
if diff < -FUNDAMENTAL_MARGIN_TREND_STABLE_PP → compressing
else                                           → stable
```

Requires at least 4 quarters of data. If fewer are available, the trend metrics are simply not written for that symbol.

### Startup behaviour

On first launch, waits `FUNDAMENTAL_STARTUP_DELAY_SECS` (default 30s) for `data-fundamental` to finish its first metrics fetch, then runs a full scoring pass immediately. After that, polls on `DATA_FUNDAMENTAL_ANALYSIS_POLL_INTERVAL` (default 24h).

### Environment variables

#### General

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `FUNDAMENTAL_STARTUP_DELAY_SECS` | `30` | Wait time on startup before first scoring run |
| `DATA_FUNDAMENTAL_ANALYSIS_POLL_INTERVAL` | `24h` | How often to re-score all symbols |
| `FUNDAMENTAL_SYMBOLS` | `AAPL,MSFT,SPY` | Equity symbols to score (falls back to `ALPACA_DATA_SYMBOLS`) |
| `FUNDAMENTAL_ANALYSIS_MIN_METRICS` | `5` | Minimum raw TTM metrics required to score a symbol |

#### EPS & revenue growth thresholds

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_EPS_GROWTH_STRONG` | `15` | EPS YoY growth > this % = "strong" |
| `FUNDAMENTAL_EPS_GROWTH_WEAK` | `5` | EPS YoY growth < this % = "weak" |
| `FUNDAMENTAL_REV_GROWTH_STRONG` | `10` | Revenue YoY growth > this % = "strong" |
| `FUNDAMENTAL_REV_GROWTH_WEAK` | `2` | Revenue YoY growth < this % = "weak" |

#### P/E evaluation thresholds

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_PE_5Y_CHEAP_PCT` | `15` | P/E below 5Y mean by this % = "cheap_vs_history" |
| `FUNDAMENTAL_PE_5Y_EXPENSIVE_PCT` | `15` | P/E above 5Y mean by this % = "expensive_vs_history" |
| `FUNDAMENTAL_PE_ABS_VALUE` | `15` | Fallback: P/E < this = "value" (when 5Y mean unavailable) |
| `FUNDAMENTAL_PE_ABS_GROWTH` | `25` | Fallback: P/E < this = "growth_fair" |
| `FUNDAMENTAL_PE_COMPRESSION_FLAT` | `5` | Forward P/E within ±this % of trailing = "flat" |

#### FCF thresholds

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_FCF_YIELD_ATTRACTIVE` | `5` | FCF yield ≥ this % = "attractive" |
| `FUNDAMENTAL_FCF_YIELD_FAIR` | `2` | FCF yield ≥ this % = "fair" |
| `FUNDAMENTAL_FCF_DIV_EPS_GROWTH` | `10` | EPS growth above this % triggers divergence check |
| `FUNDAMENTAL_FCF_DIV_YIELD_LOW` | `2` | FCF yield below this % = "warning_eps_growing_fcf_low" |
| `FUNDAMENTAL_FCF_DIV_YIELD_HIGH` | `5` | FCF yield above this % = "high_quality_earnings" |

#### Margin thresholds

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_GROSS_MARGIN_MOAT` | `40` | Gross margin ≥ this % = "strong_moat" |
| `FUNDAMENTAL_GROSS_MARGIN_AVG` | `20` | Gross margin ≥ this % = "average" |
| `FUNDAMENTAL_NET_MARGIN_STRONG` | `15` | Net margin ≥ this % = "strong" |
| `FUNDAMENTAL_NET_MARGIN_AVG` | `5` | Net margin ≥ this % = "average" |
| `FUNDAMENTAL_MARGIN_TREND_STABLE_PP` | `2` | Margin change within ±this percentage-points = "stable" |
| `FUNDAMENTAL_MARGIN_TREND_QUARTERS` | `8` | Quarters to include in margin trend window |

#### PEG & earnings surprise thresholds

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_PEG_UNDERVALUED` | `1` | PEG < this = "undervalued_growth" |
| `FUNDAMENTAL_PEG_FAIR` | `2` | PEG < this = "fairly_valued_growth" |
| `FUNDAMENTAL_SURPRISE_BEAT_PCT` | `2` | Avg surprise ≥ this % = "beat" |
| `FUNDAMENTAL_SURPRISE_MISS_PCT` | `2` | Avg surprise ≤ −this % = "miss" |
| `FUNDAMENTAL_SURPRISE_QUARTERS` | `4` | Max quarters to include in the rolling surprise average |

#### Composite score thresholds

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_COMPOSITE_STRONG` | `0.5` | Composite ≥ this = "strong" tier |
| `FUNDAMENTAL_COMPOSITE_WEAK` | `0.5` | Composite ≤ −this = "weak" tier |

---

## Data Flow Summary

```
data-ingestion workers
        │
        ├── data-technical ──→ equity_ohlcv (daily, weekly bars)
        │                   ──→ crypto_ohlcv (daily bars)
        │
        ├── data-equity ────→ macro_fred (VIXCLS for VIX regime)
        │
        └── data-fundamental → equity_fundamentals (TTM ratios, XBRL financials)
                │
                ▼
      data-analyzer (no external APIs)
                │
                ├── technical-analysis
                │     reads: equity_ohlcv, crypto_ohlcv, macro_fred
                │     writes: technical_indicators
                │       └── 49+ indicators per symbol × interval
                │             (scoring, structured payloads, regime labels)
                │
                └── fundamental-analysis
                      reads: equity_fundamentals (ttm + quarterly periods)
                      writes: equity_fundamentals (period = "derived")
                        └── 14 derived signals per symbol
                              (eps_strength, composite_score, margin_trend_8q, ...)
                │
                ▼
          analyst-bot (Python)
            reads both tables to build Discord reports
```

## Known Limitations & Future Work

| Area | Current state | Future plan |
|---|---|---|
| **True VPVR** | Volume profile uses daily bars — one volume per day assigned to one bin. Not real tick-level volume-at-price | Requires 1-minute bars or tick data in ingestion |
| **Anchored VWAP** | Not implemented — needs an event feed (earnings dates, key pivot dates) to know where to anchor | Add event date feed to data-ingestion |
| **Open Interest** | Not in OHLCV. Placeholder row stored with `available: false` | Add CME/CFTC feed or Glassnode (paid) to data-ingestion |
| **Implied Volatility** | Not implemented — needs options chain API (Polygon paid, CBOE) | Blocked by API access |
| **P/E 5-year mean** | Often NULL — Finnhub free tier returns it inconsistently | Falls back to absolute P/E bands |
| **FCF yield** | Often NULL — Finnhub free tier doesn't reliably expose `freeCashFlowYield1Y` | Falls back to `fcf_ttm ÷ market_cap` when raw FCF is available |
| **Elliott Wave** | Pivot-count hint only, not labeled waves | Full labelling requires human expertise; hint is for context only |
| **Gann angles** | Price/time scaling not applied — illustrative regression slope only | True geometric Gann requires chart-specific price/time normalisation |
| **Composite score** | Simple mean of tier scores | TODO: replace with Piotroski F-score or trained ML classifier in Python analyst-bot |
| **Python migration** | Both services are in Go with `TODO: migrate to Python` comments throughout | pandas + psycopg3 would simplify pivoting, ratio math, and trend analysis |
