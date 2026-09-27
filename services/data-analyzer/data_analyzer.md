# Data Analyzer Layer

The `data-analyzer` service processes raw data stored by `data-ingestion` workers and derives **signals** — scored, classified, and structured outputs ready for the `analyst-bot` to consume. It never calls any external API. All inputs come from TimescaleDB.

It ships several analysis binaries in one Docker image:

| Service | Binary | Reads from | Writes to |
|---|---|---|---|
| `technical-analysis` | `cmd/technical-analysis` | `equity_ohlcv`, `crypto_ohlcv`, `macro_fred` | `technical_indicators` |
| `fundamental-analysis` | `cmd/fundamental-analysis` | `equity_fundamentals` (raw metrics) | `equity_fundamentals` (derived period) |
| `macro-analysis` | `cmd/macro-analysis` | `macro_fred`, `equity_ohlcv` | `macro_derived` (`source = macro_analysis`) |
| `market-operations` | `cmd/market-operations` | `macro_fred` (VIXCLS) | `macro_derived` (`source = market_operations`, metric `mo_reference_snapshot`) |

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

Requires at least 4 10-Q filings with revenue and the income line. For a symbol with filings but fewer than 4, a nil row with `"status": "insufficient_data"` (no `direction`) is written, so an earlier trend does not stay the latest row.

Any other derived metric a run cannot compute (an input missing from the newest filing, FCF ≤ 0, no USD market cap) is superseded the same way before correlations run: a nil row `{"status": "not_computable", "superseded_ts": …}` with no `tier` (`supersedeUnwritten`).

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

## `macro-analysis` worker (FRED + market cycle + derived signals)

The third binary in the same Docker image as technical/fundamental analysis. Reads **`macro_fred`**, **`equity_ohlcv`** (for `MARKET_CYCLE_SYMBOL`), writes **`macro_derived`** with `source = macro_analysis`.

| Output (examples) | Meaning |
|---|---|
| `mp_*`, `gc_*`, `inf_*`, `gg_*` | Monetary, growth, inflation, global stance/regime metrics |
| `mc_market_cycle` | Benchmark drawdown + 200DMA + composite phase |
| `mc_macro_correlation` | Cross-metric regime label (Macro Correlations panel) |
| **`aa_reference_snapshot`** | **Additional analysis** (`additional_analysis_reference.html`): 60d rolling **ρ** benchmark × **DGS10**, **DCOILWTICO**, **VIXCLS**; static **month** + **presidential** calendars; **`reference_modules`** maps each HTML tab to `live_*` / `needs_data` / `not_automated` |

**Package layout:** `internal/marketcycle`, `internal/macrocorr`, `internal/additional` (intermarket + calendar helpers).

**Env (additional slice):** `ADDITIONAL_ANALYSIS_ENABLE`, `ADDITIONAL_ANALYSIS_CORR_WINDOW` (default 60), `ADDITIONAL_ANALYSIS_MIN_CORR_OBS` (default 40), `ADDITIONAL_ANALYSIS_MAX_BARS` (default 180). Benchmark symbol/interval follow **`MARKET_CYCLE_*`**.

**Not in v1:** options flow, GEX, dark pool, alt data, pairs trading — those remain reference-only in the HTML until separate ingestion exists.

---

## `market-operations` worker (Module 5 snapshot)

Separate binary from **`macro-analysis`**. Writes **`mo_reference_snapshot`** to **`macro_derived`** with **`source = market_operations`** (same table, different `source` for clarity).

| Field (payload) | Meaning |
|---|---|
| `global.vix`, `global.vix_regime`, `global.vix_label` | Latest **VIXCLS** from **`macro_fred`** + bands (`MARKET_OPS_VIX_*_MAX`) |
| `reference_modules` | Maps each tab of **`market_operations_reference.html`** to **`partial_live` / `needs_data` / `not_automated`** |
| `disclaimer` | Static reminder: context only |

**Per-symbol execution strip** (ATR% of price, volume vs median, flags) is **not** written by this worker — the **analyst-bot** computes it at read time from **`technical_indicators`** + OHLCV using **`BOT_MARKET_OPS_*`**.

**Env:** `MARKET_OPS_ENABLE`, `MARKET_OPS_POLL_INTERVAL` (default `1h`), `MARKET_OPS_STARTUP_DELAY_SECS`, `MARKET_OPS_VIX_LOW_MAX`, `MARKET_OPS_VIX_NORMAL_MAX`, `MARKET_OPS_VIX_ELEVATED_MAX`.

**Package layout:** `internal/marketops`.

---

## `momentum-scanner` — §8.2 daily scan

Computes §3 features for every eligible symbol's latest bar, applies the §3.2
gates, scores gate-passers (§4) and writes `momentum_features` /
`momentum_scores` plus the `momentum_chain_runs` marker in one transaction.
`momentum-daily` runs it with no arguments.
`go run ./cmd/momentum-scanner` (or `-dry-run` to compute and print without
writing).

| Flag | Env fallback | Default | Meaning |
|---|---|---|---|
| `-gate-version` | `MOMENTUM_GATE_VERSION` | `2` | §3.2 market cap: `2` = point-in-time (`raw_close[t] × shares_outstanding_pit` filed ≤ t, both unadjusted); `1` = today's Finnhub cap + §3.9 estimate |
| `-pit-max-age-months` | `MOMENTUM_PIT_MAX_FILING_AGE_MONTHS` | `15` (`DefaultGateConfig`) | v2 only: a share count filed more than N months before the session counts as unavailable → `market_cap_pit_unavailable`; `0` = no limit |
| `-source` / `-interval` | — | `tiingo` / `1Day` | bars to read |
| `-dry-run` | — | off | compute and report, write nothing |
| `-explain` | — | — | comma-separated symbols: print their gate verdict and market-cap input (PIT value, filed date, `STALE`) |

An explicit flag wins over the env var; a non-integer env value is fatal. The
gate version and age limit are printed at startup and in the run summary.

**Live gate history.** Until 2026-09-26 the scanner ran **v1** (it never set
`GateConfig.Version`, whose zero value behaves as v1). Since then it runs v2
with the 15-month filing-age limit — see Phase 1 §3.2's v2 box for why the
limit exists (JAGX priced on a 2018 share count). Under v2 the stored
`momentum_features.market_cap` is the point-in-time value the gate used (null
when unavailable or stale) and `market_cap_est` is null, since v2 has no proxy.
The point-in-time share series is loaded by `store.LoadSharesPIT`, shared with
`momentum-backtest` (whose `-pit-max-age-months` defaults to `0`, the setting
the published Phase 2 numbers were computed under).

## `momentum-tracker` — §5 exit tracking

Daily: evaluates exits on every active `momentum_tracked` row, then opens rows
for the session's candidates. `-open-mode=gates` (default, since 2026-09-24)
opens on every §3.2 gate pass — the same criterion the bot's screener alerts
use, so tracked = alerted. `-open-mode=score` restores the retired 65/72 score
thresholds (Phase 1 §10.1.9's replay figures used it). `-replay` is in-memory and
writes nothing. Run after `momentum-scanner`:
`go run ./cmd/momentum-tracker` (or `-dry-run`). Each active row is evaluated on
**every** bar after its `last_evaluated_ts` (or alert bar), oldest first,
stopping at the first exit — the same fold `-replay` does — so a run that
covers several sessions (a missed day, a bars catch-up) counts each one.
Re-running for a session already evaluated is a no-op, so a retried chain
cannot double-count. (Until 2026-09-24 only the latest bar was evaluated; the
first multi-session catch-up merged 09-22 into 09-23, and those 7 rows were
reset to their opening state and re-tracked.)

## `momentum-daily` — the scheduled chain

Long-running daemon (Compose service `momentum-daily`, `analyzer` profile;
locally `make run-momentum-daily`, or `ARGS=-once` for a single pass). For each
NYSE session (same holiday calendar as momentum-api) it:

1. waits `MOMENTUM_DAILY_GRACE` (2h) after the close, then polls every
   `MOMENTUM_DAILY_POLL` (15m) until the session's `1Day` bars from
   `MOMENTUM_DAILY_BAR_SOURCE` (`tiingo`) cover `MOMENTUM_DAILY_MIN_COVERAGE`
   (95%) of scannable universe symbols;
2. runs `momentum-scanner`, then `momentum-tracker` (binaries from
   `MOMENTUM_DAILY_BIN_DIR`, default: next to its own executable), up to
   `MOMENTUM_DAILY_MAX_ATTEMPTS` (3) times;
3. gives up on the session after `MOMENTUM_DAILY_GIVE_UP_AFTER` (14h) and logs it.

Progress is **durable**, in `momentum_chain_runs` (migration 025), never in
memory — a restart or a kill mid-run cannot make a session look finished:

- `momentum-scanner` writes the whole scan **and** `scanner_completed_at` in one
  transaction. Killed part-way, Postgres rolls it all back: no rows, no marker.
  (Verified live on 2026-09-24 with `kill -9` inside the transaction.) A symbol
  that no longer passes on a re-run loses its old score for that bar.
- `momentum-tracker` sets `tracker_completed_at` only after every write
  landed; any write failure exits non-zero with no marker. Its row writes are
  each one statement and idempotent, so a partial run is finished by re-running.
- `momentum-daily` counts `attempts` **before** each run (a crash still uses
  one up, so a step that always dies stops after 3) and records `gave_up_at` /
  `last_error`. A session is done only when `tracker_completed_at` is set. A
  committed scan is not redone (it may already have been alerted).
- analyst-bot's freshness gate reads `scanner_completed_at`, not the feature
  rows, so a partial scan can never be alerted.

A give-up is final. To retry a session by hand, clear it first:
`UPDATE momentum_chain_runs SET gave_up_at = NULL, attempts = 0 WHERE session = 'YYYY-MM-DD';`

In Compose it runs next to `data-universe`, both `restart: unless-stopped`,
against the live `ta-phase1` database.

It does **not** ingest bars — data-ingestion's `data-universe` worker must be
running — and it does **not** backfill missed sessions: a session skipped while
it was down stays unscanned unless the scanner and tracker are run by hand.
The analyst-bot alerts only once the scan for the session that just closed
exists, so the bot's alert time follows this chain, not a fixed clock.

## Integration tests: live database, rolled-back fixtures — enforced

`make test-integration` runs against the populated database on purpose
(`TEST_DATABASE_URL` defaults to `DATABASE_URL`); the rule that makes that safe
is that fixtures live in a transaction that is always rolled back. Since
2026-09-24 it is enforced rather than conventional: every test reaches Postgres
through `internal/testdb`, whose `Pool` opens sessions with
`default_transaction_read_only = on` and whose `Tx` is the only read-write path
and always rolls back. A write outside `Tx` now fails with a read-only error.
(The `equity_ohlcv` tests used to insert and delete committed rows on the pool
against live data; they now use `Tx`.)

## `momentum-api` server (read-only scanner API)

Long-running HTTP server (not a one-shot job) that serves the momentum scanner's
stored output to the web app. No auth — loopback / trusted network only. See
[`cmd/momentum-api/README.md`](cmd/momentum-api/README.md) and
`docs/MOMENTUM_SCANNER_API.md`.

**Deploy rule: rebuild it whenever `internal/fundamental`, `internal/technical` or the heuristics code changes.** Its on-demand analysis runs those packages in-process and writes the same rows the workers write, so a container that is only restarted keeps the old binary and can overwrite freshly corrected rows with old logic. This happened on 2026-09-26: after the latest-period fix, an un-rebuilt momentum-api recomputed INTC and MSFT with the old code and put back the wrong ROIC / D/E / FCF yield until it was rebuilt (`docker compose --profile api up -d --build momentum-api`) and the worker re-ran.

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
| **Limiter degradation count (data-ingestion)** | The shared rate limiter counts degradations (falls back to its local bucket) in each process's memory only; it resets on restart and appears only in a throttled warn log. The Data Source page therefore serves `degraded_count_24h: null`. Logged 2026-09-24 | Persist degradation events (a small table or a counter column on `api_rate_budget` with a timestamp) so momentum-api can count the last 24h |
| **Momentum history depth (bar count)** | The §3.1 history gate (`≥ 252` daily bars) is evaluated by `momentum-scanner` but the count is not stored, so the detail view's History gate can only show the bound ("History ≥ 252 bars"), never the symbol's actual depth. Logged 2026-09-24 | Add a bar-count column to `momentum_features` (new migration), write it from the scanner, serve it as the History gate's `value` in `GET /scanner/today/{symbol}`; the UI then shows it like the other gate values. Useful as an audit fact (how close a symbol is to the minimum) |
| **FIXED 2026-09-25 (container recreated after the Finnhub key rotation; 2,582 rows for 15 symbols landed, tiers computed) — Insider activity (SEC Form 4) never landed in `insider_transactions`** | The table has 0 rows, so `qual_insider_signal` is `insufficient_data` for every symbol (since `448ce50`; before that it claimed "neutral"). The pipeline exists and runs — data-fundamental's Finnhub insider-transactions fetch, `FUNDAMENTAL_ENABLE_INSIDER_TRANSACTIONS=true`, 24h poll — and the fetches succeed; **every write fails**. Cause (verified 2026-09-25): config drift. The running `infra-data-fundamental-1` container still has `DATABASE_URL=…@timescaledb:5432`, the retired in-compose database (`infra-timescaledb-1`, exited 2026-04). `infra/docker-compose.yml` has since moved every service to `*live-db-url` (`host.docker.internal:55442`), but this container was only ever restarted, never recreated, so it kept its old environment. Every DB write since 2026-04-24 fails with `lookup timescaledb … no such host` (155,162 failures up to July, and still failing on 2026-09-25 after the restart — including ~10,000 fundamentals upserts, not just insider rows). The four stopped containers data-crypto, data-onchain, market-operations and data-sentiment carry the same stale URL. Separately, its logs held the Finnhub token in request URLs (fixed in `e19b5bd`; the token must be rotated). Deferred 2026-09-25 as its own scope decision | Recreate the container from the current compose file (`docker compose --profile ingestion up -d --force-recreate data-fundamental`), which fixes the URL; confirm insider and fundamentals rows land; check the Finnhub budget for the watchlist. Recreate or remove the four stopped containers before they are ever started again. Consider a startup check that fails loudly when the DB host does not resolve, rather than logging per-row errors for months |
| **EDGAR share series stop updating for some companies that still file** | The live scanner (gate v2, 15-month max filing age since 2026-09-26) rejects a symbol whose latest `shares_outstanding_pit` filing is older than 15 months. Some of those are data gaps, not companies that stopped filing: FFAI's latest row is 29.5M shares filed 2021-08-13 under CIK 0001805521 (the pre-merger SPAC's count), though the company still files. Rejecting is correct given the data — the stored count is wrong after the merger and later reverse splits — but it excludes a live, filing company. 337 of 5,000 eligible symbols had a latest filing older than 15 months on 2026-09-26; how many are gaps like FFAI is unmeasured. Logged 2026-09-26 | Sample the stale set and classify: stopped filing vs. still filing but missing from our series (ticker/CIK change after a SPAC merger, per-class reporting, a concept the ingestion doesn't read). Fix the ingestion for the second group; the age limit then stops excluding them automatically |
| **Recreating data-universe after the close delays that session's bars by ~2.5 h** | `cmd/data-universe/main.go` runs its startup work sequentially — symbol refresh, then a full 5,000-symbol quote-pricing pass on the shared Finnhub budget (~2.5 h at 1 req/s), then subset selection — and only after that creates the daily-bars timer. A restart or recreate after the US close therefore holds that session's Tiingo bars back until pricing finishes, while `momentum-daily` waits at 0% coverage against its 14 h give-up. Happened 2026-09-25 (recreate at 22:07 UTC for the key rotation), which put that session at risk of giving up. Logged 2026-09-26 | Arm the daily-bars timer (with its catch-up run) before the startup pricing pass, or run pricing in its own goroutine so it cannot block bar ingestion; test that a post-close start fetches bars first |
| **Four stopped containers still point at the retired `timescaledb` host** | `infra-data-crypto-1`, `infra-data-onchain-1`, `infra-market-operations-1` and `infra-data-sentiment-1` (all exited since ~2026-04) carry `DATABASE_URL=…@timescaledb:5432` — the same stale-environment cause as data-fundamental above: created before compose moved to `*live-db-url`, never recreated. Starting any of them as-is would log per-row DB errors indefinitely rather than fail. data-sentiment's old logs also contain the pre-rotation Finnhub token (dead after the 2026-09-25 rotation). Deliberately left stopped 2026-09-25: each is its own decision, not a batch | Decide per service whether it should run. For any that should, recreate from the current compose file (`--force-recreate`) and verify rows land; for any that should not, `docker rm` it so a stray `docker start` cannot revive the stale URL |
| **Quarterly financials and earnings only for the watchlist** | `finnhub_financials_reported` / `finnhub_earnings` are fetched for the 26 watchlist symbols only, so on-demand Stock Detail analysis for any other symbol has null margin trends, ROIC, D/E, net debt/EBITDA, moat proxy, R&D intensity and earnings surprise. Deliberately not fetched on demand (2026-09-25): a speculative per-symbol Finnhub cost for secondary data | Revisit only if real usage shows these are needed for non-watchlist symbols; then an on-demand fetch in data-fundamental, budgeted like the other Finnhub calls |
| **FIXED `4f60b83` — fundamental-analysis divided by `market_cap` as if it were in millions (it is in USD)** | Since 2026-09-18 data-fundamental stores `market_cap` in absolute USD (`mulM`, per `shared/schemas/SCHEMAS.md`); rows before that date are raw millions (3,224 of 13,204 `finnhub_metric` rows). `internal/fundamental` still assumes millions ("both … are in millions") in three places, so each result is 1e6 too small or large: `fcf_yield` (≈ line 271–282; MSFT 1.53e-6 instead of 1.53%), `t3_dcf` `market_cap_vs_dcf_pct` (≈ 1145–1190; MSFT 275,492,292% instead of ≈ 275%), `t3_ps_ratio` (≈ 1337; MSFT 21,793,168 instead of ≈ 21.8). Knock-on: `fcf_yield_tier` → `composite_score`, and `corr_valuation_quality` (reads `fcf_yield`, ≈ 1878). Tiers are wrong in general, even where they happen to coincide for MSFT. `t2_ev_ebitda` is unaffected (uses Finnhub's `ev_to_ebitda`). Went unnoticed because the worker was stopped from 2026-04-16 to 2026-09-25. Logged 2026-09-25 | Divide `market_cap` by 1e6 once where it is read (or compare in USD throughout), fix the three comments, add a units test pinning MSFT-like inputs to ≈ 1.5% / ≈ 275% / ≈ 21.8. Re-run the worker; the golden fixture for derived output must be regenerated deliberately in the same commit. Until fixed, Stock Detail must not present these three values as fact |
| **FIXED `b1c3cc9` (+ `555a5a7`, `ea2b921`, migration 029 `ecdb740`) — fundamental-analysis read every multi-period metric from its OLDEST period, and financials-reported stored 10-Qs as annual** | Two causes. (1) `QueryLatestMetrics` returns one row per (metric, period) in ascending label order and every pass kept the first value per metric, so INTC's `operating_income_reported` resolved to FY2021's 19,456 and was then ×4 "annualised" as if quarterly: `t2_roic` 50.55% `moat_quality` while net margin was −19.8%. Same for every XBRL input (D/E, net debt/EBITDA, FCF yield fallback, capex intensity, DCF, P/S, goodwill, inventory turnover, R&D intensity) and for `earnings_surprise_avg` once more than 4 quarters exist. (2) data-fundamental's `financialPeriodLabel` keyed on a `freq` field report items don't have, so every 10-Q was stored as `annual_<year>` beside the 10-K (INTC `annual_2024` operating income: 10-K −11,678 and 10-Q −1,069), and a year's 10-Qs in one run overwrote each other (48 of 104 10-Qs kept per run). Also found: Finnhub's 10-Q figures are fiscal-YTD (INTC Q3 2023 revenue 38,822 = nine months), so ×4 was wrong even for a true 10-Q. Logged and fixed 2026-09-26 | Fix: metrics resolve by period end; filing figures go through one filing per 10-K/10-Q (span from `report_raw`): flows use the newest 10-K as filed (no ×4; a 10-Q only when no 10-K exists, scaled by 365.25/days covered), balance sheet the newest filing; margin trends / moat read 10-Qs only, AR-vs-revenue the two newest 10-Ks. Ingestion labels 10-Qs `q_<period end>`; migration 029 relabelled 6,145 stored rows (none deleted). Live 2026-09-26 before → after: INTC ROIC 50.55% moat → −0.97% low, D/E 0.351 → 0.386, net debt/EBITDA 0.25 conservative → `negative_ebitda`, FCF yield 1.71% → −0.75%; MSFT ROIC 100.7% → 22.2%, FCF yield 1.53% → 1.95% (FY2025 FCF 71,611), capex intensity 49.1% → 22.9%, P/S 21.8 → 13.0; NVDA ROIC 98.9% → 63.4%; XOM ROIC 33.4% → 7.9%, FCF yield 5.36% → 3.51%, capex intensity 17.5% → 8.5% |
| **FIXED 2026-09-26 — correlations read two derived metrics under the wrong key, so four checks and one master-signal condition never ran** | `scoreCorrelations` reads every input's `payload.tier`. `fcf_eps_divergence` stores its class under `quality`, with values (`warning_eps_growing_fcf_low`, `high_quality_earnings`) that the reader did not match either (it compared against `accruals_concern` / `eps_backed_by_fcf`, which nothing writes); the three margin trends store `direction`. So check 1a (EPS vs FCF), 1c (gross vs net trend), 1d (revenue growth vs gross trend), 4b (gross trend as demand proxy) and the deterioration warning's FCF condition never ran for any symbol. Live MSFT: `fcf_eps_divergence` warning stored, earnings quality ran 1 check and read "healthy"; INTC: three trends "compressing", none reached a cluster | Read `quality` / `direction` by name with the values the producers write (tests: `TestFCFEPSDivergenceReachesCorrelations`, `TestMarginTrendsReachCorrelations_INTC`; golden regenerated). Every other value the correlations compare was checked against its producer and matches. Live 2026-09-26 before → after: MSFT earnings quality healthy (1 check) → mixed positive (4), deterioration warning fires (EPS strong + the FCF/EPS warning), net signal neutral → bearish; COHR and GOOGL the same warning fires; INTC earnings quality 0 (1 check) → −0.33 (3), operational −0.5 (2) → −0.67 (3); IPGP and NVDA similar. The FCF/EPS check tests FCF *yield* (FCF ÷ market cap) < 2% with EPS growth > 10%, which a high price alone satisfies. Resolved 2026-09-27 (`57e986e`): toward the deterioration warning it counts only with cash weak against earnings too (FCF conversion < `FUNDAMENTAL_FCF_CONVERSION_LOW`, or negative FCF); MSFT (conversion 0.70) and COHR (3.9) no longer fire it, GOOGL (0.55) still does. The Earnings Quality comparison itself is unchanged and still reads the yield |
| **FIXED 2026-09-26 — Derived rows go stale when a metric stops being computable** | Each pass writes a derived metric only when its inputs allow; when they no longer do, nothing is written and the previous row stays the latest one readers see. After the period fix (`b1c3cc9`): INTC `t3_dcf` 452% and `t3_fcf_conversion` 0.57 (FY2021 FCF; FY2025 FCF is negative, so neither is computed now) and NVDA `t3_interest_coverage` 42.5 (FY2026 10-K has no interest expense row) are still the 2026-09-25 values. `b1c3cc9` writes explicit nil rows only for margin trends, `qual_moat_proxy` and `t2_net_debt_ebitda`. Logged 2026-09-26 | Fixed once for all passes: after the scoring passes and before correlations, `supersedeUnwritten` writes a nil row `{"status": "not_computable", "superseded_ts": …}` (no `tier`) for every derived metric the run did not write, so every reader sees it as absent. Tests pin the live rows (`stale_integration_test.go`, fixture dumped from live: COHR interest coverage −5.94, INTC FCF conversion / DCF from FY2021 FCF); golden seeds one stale row. Live 2026-09-26: 14 rows superseded, no others — interest coverage AMZN, COHR, KEYS, NVDA (no interest expense line in the newest 10-K); BB `fcf_yield`, `fcf_yield_tier`, `t3_dcf`, `t3_ps_ratio` (market cap nulled as non-USD since 20:17); INTC `t3_dcf`, `t3_fcf_conversion` (FY2025 FCF negative); IPGP `t2_leverage`, `t2_net_debt_ebitda` (no debt line in its 10-Qs), `t3_dcf`, `t3_fcf_conversion` (FCF negative). COHR's "accelerating financial distress" line, driven by the −5.94 row, is gone |
| **News sentiment has never been stored: Alpha Vantage's daily quota is spent by the Overview pass first** | data-fundamental uses one Alpha Vantage key (free tier: 25 requests/day) for two passes. On every container start it runs them in sequence — `runOverview` (one `OVERVIEW` call per watchlist symbol, 26; then weekly) before `runNewsSentiment` (one `NEWS_SENTIMENT` call per symbol, 26; then every 24 h) — so the Overview pass alone uses the whole day's quota and the news calls get the rate-limit reply. On 2026-09-26 the container's start pass logged 26 Overview and 25 news rate-limit replies (the one "stored" call stored 0 articles); Overview had landed for 15 symbols in the day's first start. The container was recreated several times on 2026-09-25/26, and each start repeats the Overview pass; before that it could not write at all (stale DB URL since 2026-04). `news_headlines` has no `alphavantage_sentiment` row, no row with a symbol and no row with a sentiment score, so `qual_news_sentiment_7d/30d` are `insufficient_data` and the Stock Detail news list is empty for every symbol. The other per-symbol news source, data-sentiment (Finnhub company news), has been exited since ~2026-04. Logged 2026-09-26 | Needs a scheduling decision, not a patch: how the 25/day budget is split between Overview (forward P/E, PEG, analyst target — weekly is enough) and news (useful daily), whether a start-up pass should spend quota at all when the last successful pass is recent, and whether news should come from Finnhub company news instead (data-sentiment, stopped). Until then the Handbook describes the news card and sentiment readings as empty |
| **Insider activity counts employee stock purchase plan buys as insider buying (TSM)** | Since 2026-03-18 (Holding Foreign Insiders Accountable Act) officers and directors of foreign private issuers file Form 4, so TSM has Form 4 rows from 2026-03-22. They are genuine Form 4s, but 184 of its 210 purchase rows are six monthly batches (2026-04-09 … 09-07) of 28–31 officers each at one price; the filing footnote reads "purchased by the administrator of the issuer's Employee Stock Purchase Plan … pursuant to terms predetermined by the issuer" (e.g. accession 0001046179-26-000656). `qual_insider_signal` counts each as a buyer, so TSM reads `cluster_buy` (31 buyers in 90 days) and meets Bullish convergence's insider condition although no insider chose to buy. The Handbook describes this; the card keeps "cluster buy". Deliberately not rushed 2026-09-27: a logic change that needs its own false-positive/negative review. Logged 2026-09-27 | Proposed heuristic, to be scrutinised before building: treat purchases as plan purchases when, for one symbol, **N or more distinct insiders** (sketch: ≥ 5) buy on the **same transaction date** at the **same price** (to the cent after the provider's currency translation), and exclude them from the buyer count (or classify the reading separately, e.g. `plan_purchases`). Risks to test against the stored history: a genuine coordinated buy by several directors after results at one market price would be dropped (false negative for real signals; check prices vs the day's range — plan prices are often an average or a discounted fixed price); a plan batch split across two dates or prices would slip through (false positive); small companies with few officers may never reach N. Better evidence than the pattern: Finnhub does not return the Form 4 footnotes, but EDGAR's Form 4 XML does (`footnote` text, ownership "I … By ESPP Trust", transaction code with footnote reference) — parsing ownership nature / footnotes at ingestion would classify plan purchases exactly instead of by heuristic. Whichever is built needs a test on TSM's real rows and on a US company with a genuine cluster buy |
| **Market-cycle composite: `bull_macro_aligned` and `bull_macro_divergent` can never be produced** | In `internal/marketcycle/composite.go` (bull branch, price above the 200-session average and not extended) the score starts at 0.35 and moves only by Growth (+0.12 expansion, +0.02 slowdown, −0.25 contraction), Inflation hot (−0.08) and Policy (+0.05 accommodative, −0.06 restrictive): its range is −0.04 … 0.52. `bull_macro_aligned` needs > 0.55 and `bull_macro_divergent` < −0.35, so every bull phase reads `neutral_mixed`. `bull_macro_aligned` is the composite's only constructive code, so the Market Cycle card can never show a tick. Found by the Handbook Batch 5 QA, verified 2026-09-27; the Handbook states it. Logged 2026-09-27 | A design decision, not a bug fix: either the thresholds (e.g. aligned ≥ 0.45 = expansion with no hot inflation) or the increments are wrong, or the two codes should be removed. Decide what "macro-aligned bull" should mean, then test the full stance grid so every code is reachable (or deliberately not) |
| **Flows are the latest 10-K, not TTM** | Since `b1c3cc9`, ROIC, net debt/EBITDA, interest coverage, capex intensity, inventory turnover and the XBRL FCF/revenue fallbacks use the newest 10-K as filed, up to ~12 months old. Because 10-Q figures are fiscal-YTD, TTM is derivable: latest 10-K + latest 10-Q YTD − the prior year's 10-Q YTD at the same fiscal quarter (both now stored since `555a5a7`). Margin trends compare YTD margins as filed, not discrete quarters. Logged 2026-09-26 | Build TTM (and discrete quarters: YTD minus the previous YTD in the same fiscal year) per metric when the matching filings exist; fall back to the 10-K |
