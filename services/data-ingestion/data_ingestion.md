# Data Ingestion Layer

The `data-ingestion` service is a collection of independent Go workers, each responsible for fetching a specific category of market data from external APIs and persisting it to TimescaleDB. No computation or analysis happens here — raw data only. All analysis lives in `data-analyzer`.

Every worker is a separate binary (built via its own `cmd/` directory), deployed as its own Docker Compose service, and configured exclusively through environment variables.

---

## Architecture Overview

```
External APIs
     │
     ├── Binance REST/WS ──────────┐
     ├── CoinGecko ────────────────┤  data-crypto       → crypto_ohlcv
     │                             │                    → crypto_global_metrics
     │
     ├── Alpaca Data ──────────────┐
     ├── Finnhub Quote ────────────┤  data-equity       → equity_ohlcv
     ├── FRED ─────────────────────┘                    → macro_fred
     │
     ├── Yahoo Finance ────────────┐
     ├── Binance REST ─────────────┤  data-technical    → equity_ohlcv (daily bars)
     └── Alpaca Data (fallback) ───┘                    → crypto_ohlcv (daily bars)
     │
     ├── Glassnode ────────────────┐
     └── Etherscan ────────────────┘  data-onchain      → onchain_metrics
     │
     ├── LunarCrush ───────────────┐
     └── Finnhub (crypto news) ────┘  data-sentiment    → sentiment_snapshots
                                                        → news_headlines
     │
     ├── Finnhub calendars + news ─┐
     ├── GDELT doc API ────────────┤  data-macro-intel   → economic_calendar_events
     ├── GPR CSV URL ──────────────┤                     → earnings_calendar_events
     └── RSS macro feeds ──────────┘                     → geopolitical_risk_monthly
                                                         → gdelt_macro_daily
                                                         → news_headlines (rss_macro_*, finnhub_macro_general)
     │
     ├── Finnhub /stock/metric ────┐
     ├── Finnhub /financials-rep.──┤
     ├── Finnhub /earnings ────────┤  data-fundamental  → equity_fundamentals
     └── Alpha Vantage Overview ───┘
```

All tables are **TimescaleDB hypertables** — time-partitioned PostgreSQL tables optimised for append-heavy time-series data with fast range queries.

---

## Database Tables

| Table | Written by | Primary key |
|---|---|---|
| `crypto_ohlcv` | data-crypto, data-technical | `(exchange, symbol, interval, ts, source)` |
| `crypto_global_metrics` | data-crypto | `(provider, ts)` |
| `equity_ohlcv` | data-equity, data-technical | `(symbol, interval, ts, source)` |
| `macro_fred` | data-equity | `(series_id, ts)` |
| `onchain_metrics` | data-onchain | `(asset, metric, ts, source)` |
| `sentiment_snapshots` | data-sentiment | `(source, symbol, ts)` |
| `news_headlines` | data-sentiment, **data-macro-intel** | `(ts, source, headline)` |
| `economic_calendar_events` | data-macro-intel | `(source, external_id)` |
| `earnings_calendar_events` | data-macro-intel | `(source, external_id)` |
| `geopolitical_risk_monthly` | data-macro-intel | `(month_ts, source)` |
| `gdelt_macro_daily` | data-macro-intel | `(day_ts, query_label)` |
| `narrative_scores` | optional analyst-bot (FOMC LLM job) | `(id)` |
| `equity_fundamentals` | data-fundamental | `(symbol, period, metric, source, ts)` |

All writes use `ON CONFLICT DO UPDATE` (upsert) unless noted otherwise, so re-running workers is idempotent.

---

## Worker 1 — `data-crypto`

**Purpose:** Real-time crypto price data and global market metrics.

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **Binance REST** | `GET /api/v3/klines` | OHLCV candlestick bars (latest 200) | None (public) |
| **Binance WebSocket** | `wss://stream.binance.com` | Real-time kline stream (optional) | None (public) |
| **CoinGecko** | `GET /api/v3/global` | Global crypto market stats (total market cap, dominance, volume, active coins) | None (free) |

### Tables written

#### `crypto_ohlcv`
One row per candlestick bar.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 22:00:00+00` |
| `exchange` | TEXT | `binance` |
| `symbol` | TEXT | `BTCUSDT` |
| `interval` | TEXT | `1h` |
| `open/high/low/close` | DOUBLE PRECISION | `68278.0` |
| `volume` | DOUBLE PRECISION | `518.89945` |
| `source` | TEXT | `binance_rest` or `binance_ws` |

#### `crypto_global_metrics`
One row per poll tick — the full CoinGecko `/global` response.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 22:48:00+00` |
| `provider` | TEXT | `coingecko` |
| `payload` | JSONB | `{"total_market_cap": {...}, "market_cap_percentage": {...}, ...}` |

### Configurable environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `BINANCE_SYMBOLS` | `BTCUSDT,ETHUSDT` | Comma-separated Binance trading pairs |
| `BINANCE_INTERVAL` | `1h` | Kline interval (`1m`, `5m`, `1h`, `4h`, `1d`, etc.) |
| `BINANCE_ENABLE_WS` | `false` | Set `true` to stream klines via WebSocket in addition to REST |
| `DATA_CRYPTO_BINANCE_REST_POLL_INTERVAL` | `60s` | How often to poll Binance REST |
| `DATA_CRYPTO_COINGECKO_POLL_INTERVAL` | `60s` | How often to poll CoinGecko global |
| `COINGECKO_POLL_GLOBAL` | `true` | Set `false` to disable the CoinGecko global fetch |
| `DATA_POLL_INTERVAL` | `60s` | Global fallback poll interval (used when per-source interval is unset) |

---

## Worker 2 — `data-equity`

**Purpose:** Equity price bars (intraday), real-time quotes, and macroeconomic time-series from the Federal Reserve.

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **Alpaca Data** | `GET /v2/stocks/{sym}/bars` | Historical hourly OHLCV bars | `APCA_API_KEY_ID` + `APCA_API_SECRET_KEY` |
| **Finnhub** | `GET /quote` | Latest bid/ask/price snapshot | `FINNHUB_API_KEY` |
| **FRED** | `GET /fred/series/observations` | Federal Reserve economic series (DGS10, VIXCLS, DEXUSEU, etc.) | `FRED_API_KEY` |

### Tables written

#### `equity_ohlcv`
One row per bar. Alpaca fetches proper hourly OHLCV; Finnhub quotes are stored with `interval=quote_snapshot` and `volume=0` (price-only snapshots).

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 22:00:00+00` |
| `symbol` | TEXT | `AAPL` |
| `interval` | TEXT | `1Hour` or `quote_snapshot` |
| `open/high/low/close` | DOUBLE PRECISION | `253.79` |
| `volume` | DOUBLE PRECISION | `0` (quote) or actual (bar) |
| `source` | TEXT | `alpaca` or `finnhub_quote` |

#### `macro_fred`
One row per series observation. Full history is fetched and upserted on each poll.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-30 00:00:00+00` |
| `series_id` | TEXT | `VIXCLS` |
| `value` | DOUBLE PRECISION | `30.61` |

Default FRED series: `DGS10` (10-year Treasury yield), `VIXCLS` (VIX closing level), `DEXUSEU` (USD/EUR exchange rate).

### Configurable environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `APCA_API_KEY_ID` | — | Alpaca API key (required for hourly bars) |
| `APCA_API_SECRET_KEY` | — | Alpaca API secret |
| `APCA_API_BASE_URL` | `https://paper-api.alpaca.markets` | Use live URL for production |
| `ALPACA_DATA_SYMBOLS` | `SPY,QQQ` | Comma-separated equity symbols |
| `FINNHUB_API_KEY` | — | Finnhub API key (for quote snapshots) |
| `FRED_API_KEY` | — | FRED API key (required for macro data) |
| `FRED_SERIES_IDS` | `DGS10,VIXCLS` | Comma-separated FRED series IDs |
| `DATA_EQUITY_ALPACA_POLL_INTERVAL` | `60s` | Alpaca bars poll interval |
| `DATA_EQUITY_FINNHUB_POLL_INTERVAL` | `60s` | Finnhub quote poll interval |
| `DATA_EQUITY_FRED_POLL_INTERVAL` | `60s` | FRED series refresh interval |
| `DATA_POLL_INTERVAL` | `60s` | Global fallback interval |

---

## Worker 3 — `data-technical`

**Purpose:** Backfills and refreshes **daily and weekly OHLCV bars** for both equities and crypto. This is the historical data substrate that `data-analyzer/technical-analysis` reads to compute all indicators.

> Note: This worker does **not** compute any technical indicators. That logic lives in `data-analyzer/cmd/technical-analysis`.

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **Yahoo Finance** | `query1.finance.yahoo.com` | Daily / weekly / monthly OHLCV bars (primary, free, no key needed) | None |
| **Alpaca Data** | `GET /v2/stocks/{sym}/bars` | Same bars as fallback when Yahoo returns 0 results | `APCA_API_KEY_ID` + secret |
| **Binance REST** | `GET /api/v3/klines` | Daily crypto bars (`1d` interval, up to 1000 per call) | None (public) |

**Startup behaviour:** On first run, `data-technical` **backfills** each symbol × interval combination up to `TECHNICAL_BACKFILL_BARS` (default 500) before starting the periodic poll. This ensures indicators have sufficient history on day one.

### Tables written

Same tables as the other price workers:

- **`equity_ohlcv`** — daily/weekly equity bars with `source=yahoo` or `source=alpaca`
- **`crypto_ohlcv`** — daily crypto bars with `source=binance_rest`

### Configurable environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `APCA_API_KEY_ID` | — | Alpaca key (used as Yahoo fallback only) |
| `APCA_API_SECRET_KEY` | — | Alpaca secret |
| `TECHNICAL_EQUITY_SYMBOLS` | falls back to `ALPACA_DATA_SYMBOLS`, then `AAPL,MSFT,SPY` | Equity symbols to backfill |
| `TECHNICAL_EQUITY_INTERVALS` | `1Day` | Comma-separated equity intervals (`1Day`, `1Week`) |
| `TECHNICAL_CRYPTO_SYMBOLS` | falls back to `BINANCE_SYMBOLS`, then `BTCUSDT,ETHUSDT` | Crypto pairs to backfill |
| `TECHNICAL_CRYPTO_INTERVALS` | `1d` | Comma-separated crypto intervals (`1d`, `1w`) |
| `TECHNICAL_BACKFILL_BARS` | `500` | Target history depth per symbol × interval |
| `DATA_TECHNICAL_POLL_INTERVAL` | `6h` | How often to refresh the latest bars after initial backfill |
| `DATA_POLL_INTERVAL` | `60s` | Global fallback interval |

---

## Worker 4 — `data-onchain`

**Purpose:** On-chain blockchain metrics for BTC and ETH network health.

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **Glassnode** | `GET /v1/metrics/...` | BTC/ETH active address counts (24h), and more with paid tier | `GLASSNODE_API_KEY` |
| **Etherscan** | `GET /api?module=stats&action=ethsupply` | Current total ETH circulating supply | `ETHERSCAN_API_KEY` |

> **Glassnode free tier** only gives access to limited metrics. The worker fetches `addresses/active_count` for BTC and ETH. Additional metrics (SOPR, MVRV, NVT, exchange flows) require a paid plan — documented with `TODO` comments in the code.

### Table written

#### `onchain_metrics`
Tall/narrow format — one row per metric measurement.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 22:50:31+00` |
| `asset` | TEXT | `ETH` |
| `metric` | TEXT | `eth_supply_etherscan` or `addresses_active_count` |
| `value` | DOUBLE PRECISION | `122373866.2178` |
| `payload` | JSONB | Raw API response object |
| `source` | TEXT | `etherscan` or `glassnode` |

### Configurable environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `GLASSNODE_API_KEY` | — | Glassnode API key (required; skipped if missing) |
| `ETHERSCAN_API_KEY` | — | Etherscan API key (required; skipped if missing) |
| `DATA_ONCHAIN_GLASSNODE_POLL_INTERVAL` | `60s` | Glassnode poll interval |
| `DATA_ONCHAIN_ETHERSCAN_POLL_INTERVAL` | `60s` | Etherscan poll interval |
| `DATA_POLL_INTERVAL` | `60s` | Global fallback interval |

---

## Worker 5 — `data-sentiment`

**Purpose:** Social sentiment scores and crypto news headlines.

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **LunarCrush** | `GET /public/coins/{sym}/v1` | Galaxy Score™ (0–100 composite social sentiment), social volume, social dominance | `LUNARCRUSH_API_KEY` |
| **Finnhub** | `GET /news?category=crypto` | Latest crypto news headlines with URL and publication timestamp | `FINNHUB_API_KEY` |

> **LunarCrush free tier** is rate-limited to ~10 req/min. Exceeding it returns HTTP 429, which is logged as a warning; the worker retries on the next tick.

### Tables written

#### `sentiment_snapshots`
One row per symbol per poll tick.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 22:45:00+00` |
| `source` | TEXT | `lunarcrush` |
| `symbol` | TEXT | `BTC` |
| `score` | DOUBLE PRECISION | `72.5` (Galaxy Score) |
| `payload` | JSONB | Full LunarCrush coin response |

#### `news_headlines`
One row per unique article. Deduplicated on `(ts, source, headline)` — re-inserting the same article on the next poll tick is silently ignored.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 21:48:00+00` |
| `source` | TEXT | `finnhub_crypto` |
| `symbol` | TEXT | `null` (crypto category news) or `AAPL` |
| `headline` | TEXT | `"Texas Lt. Gov. lists crypto..."` |
| `url` | TEXT | Article URL |
| `sentiment` | NUMERIC | `null` (not yet scored) |
| `payload` | JSONB | Full Finnhub news item |

### Configurable environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `LUNARCRUSH_API_KEY` | — | LunarCrush API key (skipped if missing) |
| `FINNHUB_API_KEY` | — | Finnhub API key (shared with data-equity) |
| `FINNHUB_SYMBOLS_FOR_NEWS` | `BTC,ETH` | Symbols to fetch sentiment scores for via LunarCrush |
| `DATA_SENTIMENT_LUNARCRUSH_POLL_INTERVAL` | `60s` | LunarCrush poll interval |
| `DATA_SENTIMENT_FINNHUB_NEWS_POLL_INTERVAL` | `60s` | Finnhub news poll interval |
| `DATA_POLL_INTERVAL` | `60s` | Global fallback interval |

---

## Worker 6 — `data-fundamental`

**Purpose:** Equity fundamental data — financial statement metrics, valuation ratios, earnings history, and annual/quarterly XBRL financials from SEC filings.

This worker runs four independent sub-tasks on separate tickers:

| Sub-task | Source | What it fetches |
|---|---|---|
| `runMetrics` | Finnhub `/stock/metric` | TTM and annual ratios (EPS, P/E, margins, FCF, revenue growth) |
| `runFinancials` | Finnhub `/stock/financials-reported` | XBRL income statement, cash flow, balance sheet (quarterly + annual) |
| `runEarnings` | Finnhub `/stock/earnings` | Historical EPS actuals vs analyst estimates (earnings surprise) |
| `runOverview` | Alpha Vantage `COMPANY_OVERVIEW` | Forward P/E, PEG ratio, Beta, sector/industry, analyst target price |

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **Finnhub** | `GET /stock/metric?metric=all` | ~100 TTM/annual ratio fields (EPS, revenue, P/E, FCF yield, margins, market cap) | `FINNHUB_API_KEY` |
| **Finnhub** | `GET /stock/financials-reported` | XBRL-parsed SEC filings: income statement, cash flow, balance sheet per quarter/annual | `FINNHUB_API_KEY` |
| **Finnhub** | `GET /stock/earnings` | EPS actual vs estimate for last 4+ quarters | `FINNHUB_API_KEY` |
| **Alpha Vantage** | `FUNCTION=COMPANY_OVERVIEW` | Forward P/E, PEG ratio, Beta, sector, 52-week range, analyst target (free: 25 calls/day) | `ALPHA_VANTAGE_API_KEY` |

### Table written

#### `equity_fundamentals`
Tall/narrow format — one row per `(symbol, period, metric)`. This mirrors the design of `technical_indicators` so the analyzer can query any metric by name.

| Column | Type | Example |
|---|---|---|
| `ts` | TIMESTAMPTZ | `2026-03-31 22:39:31+00` |
| `symbol` | TEXT | `AAPL` |
| `period` | TEXT | `ttm`, `q_2024Q3`, `annual_2024`, `derived` |
| `metric` | TEXT | `pe_ratio_ttm`, `revenue_reported`, `composite_score` |
| `value` | DOUBLE PRECISION | `30.74` |
| `payload` | JSONB | Additional context (e.g. `{"tier": "strong", "score": 0.5}`) |
| `source` | TEXT | `finnhub_metric`, `finnhub_financials_reported`, `finnhub_earnings`, `alphavantage_overview`, `fundamental_analysis` |

**Period conventions:**

| Period | Description | Example |
|---|---|---|
| `ttm` | Trailing twelve months — latest snapshot ratios | `pe_ratio_ttm`, `eps_ttm` |
| `q_YYYYQN` | A specific fiscal quarter from a 10-Q filing | `q_2024Q3` |
| `annual_YYYY` | A full fiscal year from a 10-K filing | `annual_2024` |
| `derived` | Computed by `data-analyzer` (scores, tiers, signals) | `composite_score`, `margin_trend` |

**Metrics stored (TTM period):**

| Category | Metrics |
|---|---|
| EPS | `eps_ttm`, `eps_annual`, `eps_growth_3y`, `eps_growth_5y`, `eps_growth_ttm_yoy`, `eps_growth_quarterly_yoy` |
| Revenue | `revenue_ttm`, `revenue_per_share_ttm`, `revenue_growth_3y`, `revenue_growth_5y`, `revenue_growth_ttm_yoy`, `revenue_growth_quarterly_yoy` |
| P/E | `pe_ratio_ttm`, `pe_ratio_annual`, `pe_ratio_5y_avg`, `pe_ratio_forward`, `forward_pe` |
| FCF | `fcf_ttm`, `fcf_per_share_ttm`, `fcf_yield_1y`, `fcf_yield_5y` |
| Margins | `gross_margin_ttm/annual/5y`, `operating_margin_ttm/annual`, `net_margin_ttm/annual/5y` |
| Valuation | `peg_ratio`, `price_to_book`, `ev_to_ebitda`, `market_cap`, `shares_outstanding` |
| Alpha Vantage | `beta`, `analyst_target_price`, `dividend_yield`, `payout_ratio`, `week52_high/low`, `ma_50d`, `ma_200d`, `sector_profile` |

**Metrics stored (quarterly/annual periods from XBRL):**

`revenue_reported`, `gross_profit_reported`, `operating_income_reported`, `net_income_reported`, `eps_diluted_reported`, `eps_basic_reported`, `operating_cf_reported`, `capex_reported`, `fcf_reported`, `total_assets_reported`, `total_liabilities_reported`, `total_equity_reported`, `total_debt_reported`, `cash_reported`, `report_raw`

> The XBRL concept name search covers multiple fallback names to handle differences between 10-Q and 10-K filings (e.g. Apple 10-K uses `RevenueFromContractWithCustomerExcludingAssessedTax` instead of `Revenues`).

### Configurable environment variables

#### API keys

| Variable | Default | Description |
|---|---|---|
| `FINNHUB_API_KEY` | — | Finnhub key (required; all three Finnhub sub-tasks disabled if missing) |
| `ALPHA_VANTAGE_API_KEY` | — | Alpha Vantage key (optional; `runOverview` is skipped if missing) |

#### Symbols

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_SYMBOLS` | falls back to `ALPACA_DATA_SYMBOLS`, then `AAPL,MSFT,SPY` | Equity symbols to fetch fundamentals for |

#### Poll intervals

| Variable | Default | Description |
|---|---|---|
| `DATA_FUNDAMENTAL_METRICS_POLL_INTERVAL` | `24h` | How often to refresh TTM ratios from Finnhub `/stock/metric` |
| `DATA_FUNDAMENTAL_FINANCIALS_POLL_INTERVAL` | `168h` (7 days) | How often to re-fetch financial statements |
| `DATA_FUNDAMENTAL_EARNINGS_POLL_INTERVAL` | `24h` | How often to refresh earnings history |
| `DATA_FUNDAMENTAL_OVERVIEW_POLL_INTERVAL` | `168h` (7 days) | How often to call Alpha Vantage (free: 25/day limit) |

#### Feature toggles

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_ENABLE_METRICS` | `true` | Enable TTM ratio fetch (Finnhub `/stock/metric`) |
| `FUNDAMENTAL_ENABLE_FINANCIALS` | `true` | Enable XBRL financial statements fetch |
| `FUNDAMENTAL_ENABLE_EARNINGS` | `true` | Enable earnings history fetch |
| `FUNDAMENTAL_ENABLE_OVERVIEW` | `true` | Enable Alpha Vantage overview fetch |
| `FUNDAMENTAL_ENABLE_ANNUAL_FINANCIALS` | `true` | Also fetch annual 10-K reports alongside quarterly |

#### Financials depth

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_FINANCIALS_FREQ` | `quarterly` | Primary frequency for financials-reported (`quarterly` or `annual`) |
| `FUNDAMENTAL_FINANCIALS_LIMIT` | `8` | Max quarterly reports to store (8 = 2 years of history for margin trends) |
| `FUNDAMENTAL_ANNUAL_FINANCIALS_LIMIT` | `5` | Max annual 10-K reports to store |

#### Startup timing

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_STARTUP_DELAY_SECS` | `30` | Seconds to wait on startup before first fetch (prevents race conditions with `data-fundamental`) |

---

## Worker 7 — `data-macro-intel`

**Purpose:** Event-style macro context that is **not** on FRED: economic and earnings calendars, a user-supplied **GPR** CSV, **GDELT** article-tone aggregates for a boolean query, **RSS** macro headlines, and Finnhub **general** market news (stored as `news_headlines` with distinct `source` values so equity/crypto company news stays separate).

**Migration:** `shared/databases/migrations/006_macro_intel.sql` (apply on existing DBs, not only fresh `initdb`).

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `DATA_MACRO_INTEL_POLL_INTERVAL` | `DATA_POLL_INTERVAL` | Loop interval between full ingest passes |
| `FINNHUB_API_KEY` | — | Required for calendars + general news (same key as other Finnhub workers) |
| `MACRO_INTEL_ENABLE_ECONOMIC_CALENDAR` | `true` | `GET /calendar/economic` — **some Finnhub tiers return 403**; disable if needed |
| `MACRO_INTEL_ENABLE_EARNINGS_CALENDAR` | `true` | `GET /calendar/earnings` |
| `MACRO_INTEL_EARNINGS_SYMBOLS` | falls back to equity symbol envs | Comma list; empty = one unfiltered earnings request |
| `MACRO_INTEL_ENABLE_FINNHUB_GENERAL` | `true` | `GET /news?category=general` → `finnhub_macro_general` |
| `MACRO_INTEL_RSS_FEEDS` | — | Comma-separated RSS URLs |
| `MACRO_INTEL_RSS_MAX_ITEMS` | `15` | Max items stored per feed per pass |
| `GPR_CSV_URL` | — | HTTP(S) URL to GPR-style monthly CSV (optional) |
| `MACRO_INTEL_GDELT_ENABLE` | `true` | Query GDELT 2.1 doc API (no API key) |
| `MACRO_INTEL_GDELT_QUERY` | macro boolean query | Passed to GDELT `ArtList` |
| `MACRO_INTEL_GDELT_MAX_RECORDS` | `120` | Cap per request |
| `MACRO_INTEL_GDELT_LOOKBACK` | `168h` | Window for GDELT query |

The GDELT 2.1 doc API requires `STARTDATETIME` / `ENDDATETIME` in **`YYYYMMDDHHMMSS`** (14 digits). The worker uses that format; shorter values return a non-JSON error from GDELT.

**Qualitative LLM scores** (e.g. FOMC hawkish/dovish) are **not** written by this worker; they are optional `narrative_scores` rows from `analyst-bot` (see `services/analyst-bot/bot.md`).

---

## Shared Configuration

All workers inherit these base variables:

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/trading?sslmode=disable` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity for all workers |
| `DATA_POLL_INTERVAL` | `60s` | Global fallback poll interval; overrides per-source intervals when those are not set |

---

## API Rate Limits and Free Tier Constraints

| API | Free tier limit | Notes |
|---|---|---|
| **Binance REST** | 1200 weight/min | No auth needed; each kline request = 1 weight |
| **Binance WebSocket** | No limit stated | One stream per connection; reconnects automatically |
| **CoinGecko** | 10–30 calls/min | Shared pool; no key needed on public endpoints |
| **Alpaca Data** | Unlimited (paper) | Live data requires a funded account for some endpoints |
| **FRED** | 120 calls/min | Free with key; full historical data available |
| **Yahoo Finance** | Undocumented | Unofficial scrape endpoint; may throttle aggressively |
| **Finnhub** | 60 calls/min | Free tier; rate limiter built into client (1 req/2s) |
| **Alpha Vantage** | 25 calls/day | Client enforces 12s minimum gap between requests; poll weekly to stay within budget |
| **LunarCrush** | ~10 calls/min | Returns HTTP 429 when exceeded; worker retries on next tick |
| **Glassnode** | Very limited free | Only basic metrics (active addresses) on free plan |
| **Etherscan** | 5 calls/sec | Free with key; generous for ETH supply metric |

---

## Data Flow Summary

```
data-crypto      → crypto_ohlcv (1h bars)
                 → crypto_global_metrics (market dominance, volumes)

data-equity      → equity_ohlcv (1h bars + quote snapshots)
                 → macro_fred (DGS10 yield, VIX, FX rates — full history)

data-technical   → equity_ohlcv (daily bars, backfill 500 bars on startup)
                 → crypto_ohlcv (daily bars, backfill 500 bars on startup)

data-onchain     → onchain_metrics (ETH supply, BTC/ETH active addresses)

data-sentiment   → sentiment_snapshots (LunarCrush Galaxy Score per coin)
                 → news_headlines (Finnhub crypto news, deduplicated)

data-fundamental → equity_fundamentals (TTM ratios, quarterly/annual XBRL,
                                        earnings history, forward estimates)

data-macro-intel → economic_calendar_events, earnings_calendar_events,
                   geopolitical_risk_monthly, gdelt_macro_daily,
                   news_headlines (rss_macro_*, finnhub_macro_general)
```

**Downstream consumers:**
- `data-analyzer/technical-analysis` reads `crypto_ohlcv` + `equity_ohlcv` + `macro_fred` to compute all technical indicators → writes to `technical_indicators`
- `data-analyzer/fundamental-analysis` reads `equity_fundamentals` to score and tier each metric → writes derived rows back to `equity_fundamentals` (period=`derived`)
- `analyst-bot` (Python) reads all tables to generate Discord reports (including the **Macro intel** embed from macro-intel tables + optional `narrative_scores`)

**Limitations:**
- `SEC EDGAR API`. Finnhub's `/stock/financials-reported` endpoint is a pre-parsed wrapper over SEC EDGAR filings. Finnhub downloads the 10-Q and 10-K XBRL filings from EDGAR, parses the XBRL tags, normalises the concept names, and serves the result through their REST API. Your code in data-fundamental/main.go calls Finnhub — it never touches sec.gov directly.