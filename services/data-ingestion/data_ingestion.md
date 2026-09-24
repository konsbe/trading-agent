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
     ├── Finnhub /stock/symbol ────┐  data-universe     → universe_symbols
     └── equity_fundamentals (read)┘
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
| `universe_symbols` | **data-universe** | `(symbol, exchange)` |

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

- **`equity_ohlcv`** — daily/weekly equity bars with `source=yahoo_finance` or `source=alpaca`
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

#### Universe-wide metrics pass (momentum scanner step 3b)

`runMetrics` can be widened from the static `FUNDAMENTAL_SYMBOLS` list to the
eligible momentum universe. **Only this one sub-task is widened** — `market_cap`
and `shares_outstanding`, the two fields the scanner's §3.2 gate and §3.9 proxy
need, both come from `/stock/metric`. The other seven sub-tasks are untouched.

Sector is *not* obtainable this way: it comes from Alpha Vantage, whose free tier
is **25 requests per day** — a 200+ day pass for the universe. It stays null in
Phase 1 at zero cost to the score, because `sector_strength_pct` is recorded but
not scored. See `docs/MOMENTUM_SCANNER_PHASE1.md` §3.13.

Three behaviours worth knowing:

- **Union, never replacement.** `env+universe` iterates `FUNDAMENTAL_SYMBOLS ∪ (universe_symbols WHERE is_eligible)`. The configured list contains `SPY`, an ETF that §3.1 excludes from the universe, so replacing the list would silently drop the most visible symbol in the daily report. An unreachable or empty universe degrades to the configured list — never to an empty pass.
- **Checkpointed per symbol.** One pass is ~3.3 hours at the shared Finnhub rate, so work is claimed in batches against `fundamental_fetch_state` and survives a restart. The static and checkpointed paths are mutually exclusive; the 24-hour ticker is skipped entirely when checkpointing is on, so the symbol list is never walked twice.
- **Cadence is emergent.** A symbol is re-fetched when its *last success* is older than the refresh interval, so there is no cycle to reset and a newly-listed symbol is picked up on the next round rather than waiting one out.

The widened pass runs **sequentially** on purpose. The Finnhub budget is shared
across all five workers (migration `008`), so parallelism would not raise
throughput — it would only deepen the queue in front of the shared limiter and
starve the other workers' short, latency-sensitive polls.

| Variable | Default | Description |
|---|---|---|
| `FUNDAMENTAL_METRICS_SYMBOL_SOURCE` | `env` | `env` or `env+universe`. Opt-in, because this worker has live consumers |
| `FUNDAMENTAL_METRICS_UNIVERSE_CHECKPOINTED` | `true` | Route the widened pass through `fundamental_fetch_state` |
| `FUNDAMENTAL_METRICS_UNIVERSE_POLL_INTERVAL` | `168h` | The widened pass's **own** cadence — deliberately not the 24h metrics ticker |
| `FUNDAMENTAL_METRICS_UNIVERSE_REFRESH_INTERVAL` | `168h` | How stale a symbol's last success may be before it is claimable |
| `FUNDAMENTAL_METRICS_UNIVERSE_BATCH_SIZE` | `250` | Symbols claimed per round |
| `FUNDAMENTAL_METRICS_UNIVERSE_CLAIM_LEASE` | `30m` | A claim older than this is treated as abandoned |
| `FUNDAMENTAL_METRICS_UNIVERSE_MAX_ATTEMPTS` | `3` | Past this a symbol is reported, not retried |
| `FUNDAMENTAL_METRICS_UNIVERSE_IDLE_INTERVAL` | `1h` | Wait before re-checking once drained |

> **Watch `fresh`, not `done`.** The batch log reports both. `done` only means the
> last attempt worked; `fresh` means the data is inside the refresh window, and it
> is `fresh` that the scanner's candidate counts depend on. The worker warns when
> fewer than half the universe is fresh, because until that clears, §3.2's gate
> excludes the remainder and §6's base rate is not meaningful.

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

## Worker 8 — `data-universe`

**Purpose:** maintain `universe_symbols`, the eligible US common-stock universe the
momentum scanner operates on. Spec: `docs/MOMENTUM_SCANNER_PHASE1.md` §3.1 and §8.1.

**Migration:** `shared/databases/migrations/007_momentum.sql` (apply manually on an
already-initialised volume — migrations only auto-run on first `initdb`).

> Migration numbering, idempotency, and the rule for **when a migration may be
> amended in place versus superseded by a new one**, are documented once in
> `shared/schemas/SCHEMAS.md`. The short version: amend only while the migration has
> never been applied outside a scratch database.

This worker performs **no computation and no scoring**. It answers one question:
which tickers are in scope, and why is everything else out?

### Four passes

| Pass | Cadence | What it does |
|---|---|---|
| `runSymbols` | weekly | Fetch the exchange symbol directory, apply §3.1 eligibility, upsert **every** decision — exclusions included, with a reason |
| `runFundamentals` | weekly | Copy sector, industry, shares outstanding and market cap from `equity_fundamentals` onto the universe rows |
| `runBackfillRound` | looping | Claim a batch of symbols and backfill 3 years of daily bars (§8.1.3). Runs back-to-back while work remains, then idles |
| `runDailyBars` | daily | Short-window incremental refresh for every backfilled symbol (§8.1.4) |

Each pass is independently switchable, and all four are safe to interrupt.

### The backfill is resumable, not merely restartable

A symbol is claimed (`in_progress` + `backfill_claimed_at`) **before** its fetch and
only reaches `done` **after** its bars are committed. So a kill at any point leaves
the row reclaimable, and no symbol is ever recorded complete without its data.

The piece that makes this work is the **claim lease**. If the process dies
mid-symbol, the row stays `in_progress` forever; a claim older than
`UNIVERSE_BACKFILL_CLAIM_LEASE` is therefore treated as abandoned and re-claimed.
Without it a crash would silently finish the backfill incomplete — which looks
exactly like a universe with thin history rather than like a bug.

Claim priority is `pending` → abandoned `in_progress` → `failed` (under
`max_attempts`), so a first pass covers the universe once before spending rate
budget on known-flaky symbols. `FOR UPDATE SKIP LOCKED` means two workers would
split the work rather than duplicate it.

Three outcomes per symbol, deliberately distinguished:

| Outcome | Recorded as | Why |
|---|---|---|
| Bars fetched | `done`, `backfill_cursor_ts` = **oldest stored bar** | The cursor reflects the data, not the request window — a recent IPO gets its true first bar, not "three years ago" |
| Yahoo returns an empty series | `done`, `backfill_last_error = 'no_bars_returned'` | A delisting or bad ticker. Not a failure, and retrying it every round would waste the rate budget the rest of the universe needs |
| Request failed | `failed`, attempts incremented, error text persisted | Diagnosable from SQL alone rather than by correlating logs |

### APIs used

| API | Endpoint | What it provides | Auth |
|---|---|---|---|
| **Finnhub** | `GET /stock/symbol?exchange=US` | The entire US listing (~25–30k rows, all instrument types) in **one** request | `FINNHUB_API_KEY` |
| **Yahoo Finance** | `GET /v8/finance/chart/{sym}?period1=&period2=` | Daily OHLCV over an **explicit date window** | None |

The bar fetch uses `period1`/`period2` Unix bounds rather than Yahoo's `range`
parameter. That is not a style choice: `range` tops out at `2y` for daily bars, so
the pre-existing `FetchBars` **cannot** satisfy the three-year backfill. The
explicit-window path (`FetchBarsRange`) has no such cap and makes the requested
window auditable. Both paths share one decoder, so the scanner's bars are filtered
identically to the ones `data-technical` already stores.

Retry policy: `429` and `5xx` are retried with exponential backoff, honouring
`Retry-After` when the server sends it. **`404` is not retried** — a delisted or
misspelled ticker will never succeed, and on a 6,000-symbol pass that wasted budget
is the difference between finishing and not.

`runFundamentals` makes **no API calls at all**. §8.1.2 specifies refreshing those
fields "from existing fundamentals ingestion", so it reads `equity_fundamentals`.
That is also the only affordable option: `/stock/metric` is rate-limited to one
request per two seconds, which is over four hours for a 7,500-symbol universe.

### Eligibility rules (§3.1)

Applied by `internal/universe`, which is pure and unit-tested — the rules are the
product, and a wrong exclusion silently shrinks the scannable universe.

| Rule | Excluded reason | Notes |
|---|---|---|
| Instrument type must be common stock | `type_not_common_stock` | Strict reading. ETFs, ETNs, closed-end funds, mutual funds, warrants, rights, units and preferred shares are all out. ADRs and REITs are **also** out by default — widen via `UNIVERSE_ALLOWED_TYPES` if you want them |
| Venue must be NASDAQ / NYSE / NYSE American | `exchange_not_allowed` | Matched on **MIC**, the only reliable discriminator. `ARCX` (NYSE Arca, predominantly ETFs) and every OTC tier are excluded — §3.1 drops OTC/pink sheets in Phase 1 |
| No non-common ticker suffix | `ticker_suffix_excluded` | Matched after a `.` or `-`. Warrants, units, rights and preferred series. Share-class letters (`BRK.B`) stay eligible, and bare tickers like `U` (Unity) and `R` (Ryder) are not mistaken for units or rights |
| Symbol well-formed | `symbol_malformed` | Blank or containing whitespace/slash |

Rules are evaluated in that order and the **first** failure is the recorded reason,
so `excluded_reason` is deterministic.

**The 252-bar history minimum is deliberately not applied here.** Bars only exist
because the backfill ran over the eligible set, so gating eligibility on bar count
would be circular and would leave the universe permanently empty. `bar_count`,
`first_bar_ts` and `last_bar_ts` are refreshed on every pass for auditability, and
the minimum is enforced as a hard gate at scan time, where §3.2 also lists it.

### Table written

#### `universe_symbols`
Regular table (current state, not time-series). Ineligible symbols are **retained**
so the filter is auditable. Full column reference: `shared/schemas/SCHEMAS.md`.

Two write-safety properties worth knowing:

- The symbol upsert runs in **one transaction**, so the daily scan never reads a
  half-refreshed universe — it sees either the previous complete set or the new one.
- An empty fetch is treated as a provider fault and leaves the existing universe
  untouched, rather than emptying it.
- `runFundamentals` uses `COALESCE`, so a provider returning null for one field
  preserves the previous value instead of erasing it. `fundamentals_ts` still
  advances, so staleness stays visible.

### Unit conversion

`equity_fundamentals` stores `market_cap` in **$ millions** and
`shares_outstanding` in **millions**. `universe_symbols` and the §3.2 gates are in
**absolute** dollars and shares, so both are multiplied by `1e6` on read. Getting
this wrong is invisible — the numbers still look plausible — so it is documented at
the top of the migration, in the schemas, and here.

### Sanity checks the worker logs

§2.3 expects roughly **5,000–7,500** eligible US common stocks after filtering. The
worker warns when the count falls below 3,000 or exceeds 12,000, because either
means the type or MIC allowlist is wrong and everything downstream is affected.

It also reports fundamentals coverage and warns when symbols have **neither**
market cap nor shares outstanding — those fail the §3.2 market-cap gate and cannot
be rescued by the §3.9 `market_cap_est` proxy either. Coverage is bounded by
whatever `FUNDAMENTAL_SYMBOLS` was set to, which is typically far narrower than the
universe; the warning exists so that shows up as a number now rather than as an
unexplained empty penny bucket later.

### Configurable environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://...` | TimescaleDB connection string |
| `LOG_LEVEL` | `info` | Log verbosity |
| `FINNHUB_API_KEY` | — | Required for the symbol list; missing key disables that pass with a warning, never a crash |
| `UNIVERSE_ENABLE_SYMBOLS` | `true` | Enable the symbol-list pass |
| `UNIVERSE_ENABLE_FUNDAMENTALS` | `true` | Enable the sector/shares/cap pass |
| `UNIVERSE_EXCHANGE` | `US` | Finnhub `/stock/symbol` exchange code |
| `UNIVERSE_SYMBOLS_POLL_INTERVAL` | `168h` | Symbol-list refresh cadence |
| `UNIVERSE_FUNDAMENTALS_POLL_INTERVAL` | `168h` | Fundamentals refresh cadence |
| `UNIVERSE_STARTUP_DELAY_SECS` | `30` | Settling delay before the first pass |
| `UNIVERSE_ALLOWED_TYPES` | `Common Stock` | Instrument-type allowlist; blank falls back to the default |
| `UNIVERSE_ALLOWED_MICS` | `XNAS,XNGS,XNMS,XNCM,XNYS,XASE` | Venue allowlist |
| `UNIVERSE_EXCLUDED_SUFFIXES` | `W,WS,WT,U,UN,R,RT` | Non-common share-class suffixes |
| `UNIVERSE_ALLOW_EMPTY_MIC` | `false` | Admit records with a blank MIC |
| `UNIVERSE_MIN_BARS_HISTORY` | `252` | §3.7's 52-week window reads `high[t-251]`, so 252 bars are needed for a complete window. Recorded/reported, not used for eligibility |
| `UNIVERSE_BAR_INTERVAL` | `1Day` | Which `equity_ohlcv` rows count as daily bars |
| `UNIVERSE_BAR_SOURCE` | `tiingo` | Bar source. **Not Alpaca** — its free tier is IEX-only volume, a single-venue fraction of consolidated volume, which makes every volume feature in §3 wrong. Note the value is `yahoo_finance`, not `yahoo`: that is what `internal/fetch/yahoo` writes and what `data-analyzer` reads, and the spec's §7 originally named a value that matches zero rows |
| `UNIVERSE_ENABLE_BACKFILL` | `true` | Enable the 3-year bar backfill |
| `UNIVERSE_ENABLE_DAILY_BARS` | `true` | Enable the daily incremental refresh |
| `UNIVERSE_BACKFILL_YEARS` | `3` | History depth |
| `UNIVERSE_BACKFILL_BATCH_SIZE` | `200` | Symbols claimed per round |
| `UNIVERSE_BACKFILL_CONCURRENCY` | `4` | Parallel in-flight fetches; the rate limiter is shared, so this is pipelining, not throughput |
| `UNIVERSE_BACKFILL_CLAIM_LEASE` | `15m` | How long an `in_progress` claim is honoured before it is treated as abandoned |
| `UNIVERSE_BACKFILL_MAX_ATTEMPTS` | `3` | Retries per symbol across runs |
| `UNIVERSE_BACKFILL_IDLE_INTERVAL` | `1h` | Wait before re-checking for work once drained |
| `UNIVERSE_YAHOO_REQUESTS_PER_SEC` | `2.0` | §2.3 budgets ~5/s, but the endpoint is unofficial and §2.2 says to be polite. Above ~5/s expect sustained 429s |
| `UNIVERSE_YAHOO_BURST` | `1` | Keeps request spacing even |
| `UNIVERSE_YAHOO_TIMEOUT` | `30s` | Per-request timeout |
| `UNIVERSE_YAHOO_MAX_RETRIES` | `3` | Retries after the first attempt, transient statuses only |
| `UNIVERSE_YAHOO_BACKOFF_BASE` / `_MAX` | `2s` / `60s` | Exponential backoff bounds; `Retry-After` overrides when longer |
| `UNIVERSE_DAILY_BARS_AT` | `18:30` | New York time the weekday refresh runs. `interval` = legacy plain ticker |
| `UNIVERSE_DAILY_BARS_CATCHUP_SHARE` | `0.95` | At startup, refresh immediately if fewer than this share of backfilled symbols have the latest due session |
| `UNIVERSE_DAILY_BARS_INTERVAL` | `24h` | Legacy cadence, only used with `UNIVERSE_DAILY_BARS_AT=interval`. It used to be the only schedule: the first pass came 24h after startup, so a worker restarted more often than daily never refreshed (2026-09-22/23 were missed that way) |
| `UNIVERSE_DAILY_BARS_LOOKBACK_DAYS` | `7` | Window per symbol — wider than a day to repair missed runs and late corrections |

---

## Worker 9 — `intraday-bars`

Stores **5-minute regular-session bars** for the symbols someone is looking at:
the latest scan's gate-passing candidates plus every symbol on any watchlist
(`watchlist_items`). They feed the 1D and 5D ranges of the web app's detail chart
through momentum-api's bars endpoint. Longer ranges use the existing daily bars.

| | |
|---|---|
| Source | Yahoo Finance chart endpoint (`internal/fetch/yahoo`, `FetchBars` with `5Min`): free, no key, consolidated volume, `includePrePost=false` |
| Writes | `equity_ohlcv`, `interval = '5Min'`, `source = 'yahoo_finance'` — every daily reader filters `interval = '1Day'`, so the scanner never sees these rows |
| Scope | candidates + watchlist, capped by `INTRADAY_BARS_MAX_SYMBOLS` (candidates kept first) — never the whole universe, because Yahoo access depends on IP reputation |
| Run | `make run-intraday-bars` (loop) or `go run ./cmd/intraday-bars -once` |

| Env var | Default | Meaning |
|---|---|---|
| `INTRADAY_BARS_POLL_INTERVAL` | `30m` | Time between passes |
| `INTRADAY_BARS_LOOKBACK_DAYS` | `8` | Calendar days of bars kept per pass (≥ 5 sessions) |
| `INTRADAY_BARS_MAX_SYMBOLS` | `200` | Hard cap on symbols per pass |

Rows are upserted on `(symbol, interval, ts, source)` and never pruned; at ~78
bars per symbol per session this is small, but prune old `5Min` rows if the
watchlist grows large. A failed symbol is logged and skipped until the next pass.

## Shared API rate budgets

Five workers hold a Finnhub client: `data-equity`, `data-sentiment`,
`data-macro-intel`, `data-fundamental`, and `data-universe`. Each used to build its
own in-process token bucket at ~0.5 req/s, summing to **~2.5 req/s against a
free-tier budget of 1 req/s** (60 requests/minute).

That was harmless only because observed demand is roughly **10 % of budget** — each
worker polls a handful of symbols on a 60-second tick and none approaches its own
allowance. It stops being harmless the moment one caller saturates its allowance
for hours, which the momentum scanner's universe-wide fundamentals pass does. The
429s then surface in whichever *other* worker happens to be running, which is the
worst shape of bug: symptom and cause in different services.

All five now pace against one Postgres-backed budget (`api_rate_budget`, migration
`008`). Construction is one line per worker:

```go
fh := finnhub.NewWithLimiter(cfg.FinnhubKey, ratelimit.SharedFinnhub(ctx, pool, log))
```

`SharedFinnhub` returns `nil` when disabled or unbuildable, and
`NewWithLimiter(token, nil)` is exactly the old `New(token)` — so a worker that
cannot coordinate still starts and still runs at its previous rate.

**If coordination fails**, the limiter falls back to that same in-process bucket
for the affected request: never unlimited, never blocking indefinitely, and
per-request rather than a latch, so a brief blip does not strand a worker on local
rate. A `degraded` counter and a throttled warn log make the condition visible —
that log line is what turns *"why are there 429s in `data-sentiment`"* into *"the
shared limiter degraded"*.

Full rationale, including why Postgres rather than Redis, is in
`shared/schemas/SCHEMAS.md` under `api_rate_budget`.

### What to check after the first real universe-wide pass

Nothing in this design has yet run against live Finnhub. Coordination is proven
against Postgres and the fallback against an unreachable one, but the operational
question — do five workers under one budget stop producing 429s where they
previously would — is only answerable from a real pass. Two numbers, with
thresholds set in advance rather than eyeballed afterwards:

| Signal | Where | Threshold |
|---|---|---|
| `degraded` count across the five workers | the throttled `shared rate limiter unavailable` warn, and `Stats().Degraded` | **Any nonzero count is worth investigating.** A handful during a Postgres failover is fine; a *sustained* rate is not, because it means the workers are running uncoordinated while appearing healthy |
| Measured Finnhub ceiling | 429 responses while the shared budget is active and `degraded` is zero | If the real ceiling proves **meaningfully below 1.0 req/s**, fold the measured value back into `FINNHUB_RATE_PER_SEC` / `refill_per_sec`. Do **not** work around it by lowering individual workers' rates — that recreates the uncoordinated-aggregate problem this table exists to solve |

The second row is the one that is easy to get wrong under pressure: a per-worker
tweak makes the symptom go away locally and puts the budget back out of sync
globally. The shared budget is now the only place that number should live.

The same evidence-first rule applies to `UNIVERSE_YAHOO_REQUESTS_PER_SEC`
(default `2.0`, against §2.3's guessed ~5/s): tune it from observed
success/error/retry rates on the first backfill, not from the spec's number.

### Measured findings from the first verification attempt (2026-09-15)

Recorded as found, not after deciding what to do about them.

#### 1. Yahoo's chart API is unreachable from this network — Step 3 is blocked

Every request returns **`HTTP 429 "Too Many Requests"`**, a 19-byte plain-text
body, no `Retry-After`, `server: ATS` (Yahoo's CDN). It is identical on the first
request of a session and unaffected by the browser-like headers the client already
sends, so it is **not** per-request rate limiting — it is an IP-level block.

The likely cause is shared egress: this host sits behind a corporate proxy
(a Nokia PAC config is active and `infra/corp-ca.pem` is populated), so the entire
network appears to Yahoo as one address whose per-IP allowance is already spent by
other users. Lowering `UNIVERSE_YAHOO_REQUESTS_PER_SEC` cannot help — the limit is
not ours to spend. Running the workers in containers does not help either, since
they share the host's egress IP.

**Why this matters beyond one blocked step:** §2.2 designates Yahoo as the
*primary* bar source for Phase 1, specifically because it provides **consolidated**
volume, and every volume feature in §3 (RVOL, acceleration, dollar volume) is
meaningless without that. Alpaca's free tier is explicitly rejected in §2.2 for
being IEX-only. So this is not "swap the fetcher" — there is currently no verified
free source of consolidated daily volume reachable from this network, which is a
prerequisite for Steps 4 through 7 having meaningful inputs.

Options, none yet chosen: run the backfill from a network with its own egress
(the endpoint is unofficial and best-effort either way, per §2.2); obtain a data
source with an authenticated quota rather than an IP-shared one; or re-verify from
a different host before treating this as permanent.

**Narrowing evidence — the block is provider-specific, not egress-wide.** Three
providers tested from the same host in the same minutes:

| Provider | Result | Reading |
|---|---|---|
| Finnhub | `/quote`, `/stock/metric`, `/stock/symbol` all **200** (also from inside a container) | Outbound HTTPS to market-data APIs is not blocked |
| Stooq | **200**, but the body is a JavaScript proof-of-work browser challenge, not CSV | Reachable; refuses non-browser clients |
| Yahoo chart | **429** on every request, first one included | Blanket block |

So the corporate proxy is not preventing market-data traffic in general — Yahoo
alone refuses this egress address. That points at IP reputation rather than network
policy, and it means the decisive test is **re-running the same request from a
network with different egress** (phone hotspot, home connection). That test cannot
be run from this host; it needs someone on a different connection. Until it is run,
"Yahoo is unusable" is unproven — what is proven is "Yahoo is unusable *from here*".

#### Fallback bar sources evaluated — neither is viable as-is

| Candidate | Verdict |
|---|---|
| **Finnhub `/stock/candle`** | **403 `"You don't have access to this resource."`** Stock candles are behind a paid Finnhub tier. This is a plan restriction, not a network result — the same key succeeds on `/quote` and `/stock/metric` — so it would fail identically from clean egress. The appealing "reuse the client we already have" option is closed on the free tier. |
| **Stooq CSV** (`stooq.com/q/d/l/?s=aapl.us&i=d`) | **Returns HTTP 200 with an HTML+JS proof-of-work challenge**, not CSV. Unusable without executing JavaScript. Whether a cleaner egress IP avoids the challenge is untested. |

The Stooq result carries a trap worth recording: it answers **200** with an HTML
body. A client that checks only the status code would treat the challenge page as
success and parse it as CSV, producing zero or garbage bars rather than an error.
Any Stooq adapter must validate the content type and the header row, not the status.

Neither candidate has been confirmed to provide **consolidated** volume either,
which is the actual requirement §2.2 encodes — Yahoo was a means to it, not the end.
A fallback that quietly supplies single-venue volume reintroduces the exact problem
that disqualified Alpaca's free tier, and every §3 volume feature would be wrong
while looking fine. That check has to pass before any adapter is written.

#### API-key providers ARE reachable from this network — the fallback branch is open

Tested from the same proxied host that Yahoo blocks:

| Provider | Unauthenticated response | Reading |
|---|---|---|
| Tiingo | `403 {"detail":"Please supply a token"}` | Request reaches the API |
| Polygon.io | `401 {"error":"API Key was not…"}` | Reaches the API |
| Twelve Data | `401 apikey parameter …` | Reaches the API |
| EODHD | `401 Unauthenticated` | Reaches the API |

All four return **authentication** rejections, not IP blocks. That is the important
distinction: providers authenticating by **API key** meter per account, so they do
not share a rate budget with everyone else behind the same corporate egress — which
is precisely the failure mode that killed Yahoo here.

**Consequence for the decision.** Salvaging Yahoo is no longer the only path, and it
is arguably the weaker one: it depends on IP reputation staying favourable, which is
not something the project controls. An API-key provider is the more durable fix even
if the hotspot test succeeds. The hotspot test is still worth running — it is five
minutes and a clean result means zero new provider surface — but it is no longer
blocking.

**Not yet verified for any of the four:** whether their free tier includes daily
bars at all, their history depth, their rate limits at ~5,000 symbols, and above all
whether their volume is **consolidated**. No key is held for any of them
(`ALPHA_VANTAGE_API_KEY` is the only market-data key present, and its 25-requests-
per-day ceiling rules it out at universe scale regardless). Do not write any of them
into §2.2 as the replacement until that is measured — same rule that correctly
disqualified Finnhub candles and Stooq.

#### `scripts/verify-bar-source.sh` — run this rather than re-deriving the test

One command, answers both questions in §2.2's order, and works with no repo setup:

```
./services/data-ingestion/scripts/verify-bar-source.sh yahoo
./services/data-ingestion/scripts/verify-bar-source.sh tiingo  "$TIINGO_TOKEN"
./services/data-ingestion/scripts/verify-bar-source.sh all
```

It reports reachability **and** a consolidated-volume verdict, because the second
check is the one that is easy to skip and fatal to get wrong. The method needs no
reference provider: AAPL trades roughly 40–60 M shares a day consolidated, while a
single venue like IEX reports low single-digit millions — a 20-40× gap that a
magnitude test resolves unambiguously.

Verified against the known-bad case: run from this host it correctly reports Yahoo's
blanket 429 and explains that a rate setting cannot fix it.

#### Measured provider comparison (2026-09-16) — Tiingo is the leading candidate

Run with real keys from the proxied network, using the script above plus history-depth
and rate-limit probes. **All four serve consolidated volume**; they separate on
history depth and quota.

| Provider | Reachable | AAPL volume | Consolidated | 3-yr history | Measured rate limit | Verdict |
|---|---|---|---|---|---|---|
| **Tiingo** | ✅ 200 | 31,748,183 | ✅ | ✅ **751 bars**, 2023-09-18 → 2026-09-15 | 12/12 in 7 s, no throttle (~1.7 req/s) | **Leading candidate** |
| Twelve Data | ✅ 200 | 31,694,100 | ✅ | ✅ 751 bars | ❌ **8 requests/minute** hard cap | Fallback only |
| Polygon.io | ✅ 200 | 31,748,183 | ✅ | ❌ **501 bars — 2 years only** | not probed | Fails §8.1.3's 3-year requirement |
| EODHD | ✅ 200 | 31,694,100 | ✅ | ❌ **251 bars — 1 year only** | not probed | Out |
| Yahoo | ❌ 429 | — | (was the reason it was chosen) | — | IP-reputation block | Unusable from this network |

**Why the consolidated verdict is trustworthy rather than a heuristic.** Tiingo and
Polygon report the identical figure to the share (31,748,183), as do Twelve Data and
EODHD (31,694,100 — a different last-completed-session). Independent providers
agreeing exactly means both are reading the official consolidated tape. A
single-venue feed could not match, and IEX-scale volume for AAPL would be low
single-digit millions — roughly a 10-30× gap.

**Quota arithmetic at universe scale (~4,978 symbols):**

- **Tiingo** — no per-minute throttle observed at ~1.7 req/s, which would put a full
  pass near 50 minutes. Comfortable.
- **Twelve Data** — 8 req/min measured means **≥ 10.4 hours** per pass even before
  any daily cap, so it cannot be the primary.
- **Polygon** — 2 years of history would require lowering `UNIVERSE_BACKFILL_YEARS`
  from 3 to 2, which is a §2.3 change and needs a decision, not a config tweak.

**The one thing still unverified for Tiingo, and it matters:** free-tier plans
commonly meter *unique symbols per month* rather than requests per second. Twelve
symbols were consumed measuring the above, and probing further would burn the very
quota the backfill needs. Check the account dashboard or plan terms for a
unique-symbol or monthly cap **before** writing an adapter — a 5,000-symbol pass
against a 500-symbol monthly allowance fails on day one, and it fails silently
partway through rather than at the first request.

#### Twelve Data's real quota (2026-09-16) — no symbol cap, but a hard daily ceiling

`GET /api_usage?apikey=…` reports the plan directly, so this needed no guessing:

```
plan_category: basic      (free)
plan_limit: 8             requests per minute
plan_daily_limit: 800     requests per DAY
```

Two things follow, and the second is the disqualifying one:

- **There is no unique-symbol cap.** The quota is a pure request count that resets
  daily, so the 8/min rate is not the binding constraint — 800/day is. A one-time
  4,978-symbol backfill is therefore ~7 days of unattended running, which is
  tolerable for a one-off.
- **But §8.1.4's daily refresh needs one request per symbol per day.** At full
  universe scale that is 4,978 requests/day against a ceiling of 800 — **6× over**.
  No amount of patience fixes an ongoing requirement that exceeds a daily quota.

So Twelve Data's free tier can sustain a universe of roughly **800 symbols
indefinitely** (backfill inside a day, daily refresh comfortably within quota), and
cannot sustain the full universe at all. That number is worth noting because it lands
close to the size of a pilot subset, which makes it a viable *pilot* source rather
than a rejected one.

Tiingo showed no per-minute throttle but exposes **no usage endpoint** (`/api/test`
and `/api/usage` 404; `/account/usage` redirects to a login wall) and its pricing and
documentation pages are JS-rendered. Its unique-symbol allowance, and whether that
allowance resets monthly or is cumulative-ever, are **only visible from the account
dashboard**. That distinction decides whether the free tier offers one shot at N
symbols or a rolling N per month, so it has to be read before any full-scale run.

#### ⚠️ equity_ohlcv holds bars on TWO DIFFERENT adjustment conventions

**Do not aggregate or compare `equity_ohlcv` rows across `source` values without
handling this.** The two bar paths do not agree on what their prices mean:

| Source | Adjustment | Determined by |
|---|---|---|
| `tiingo` | split **and** dividend adjusted | reads `adjOpen/adjHigh/adjLow/adjClose/adjVolume` |
| `yahoo_finance` | **dividend adjustment definitely absent**; split adjustment unverified | reads `indicators.quote` only |

The Yahoo finding is from code inspection and is not a guess: `chartResponse` in
`internal/fetch/yahoo/bars.go` declares only `indicators.quote`, and the string
`adjclose` appears nowhere in the package. Yahoo returns dividend-adjusted closes
exclusively in `indicators.adjclose`, so a series that never decodes that field
cannot contain dividend adjustment. Whether Yahoo's `quote` array is
split-adjusted could not be checked — the endpoint returns a blanket 429 from
this network — so treat that half as unknown rather than as either answer.

§3's opening line requires bars that are "split/dividend-adjusted", so **the
Yahoo path does not satisfy §3 as written.** For a dividend-paying symbol its
prices drift from the adjusted series by the cumulative dividend, which shifts
every price-derived feature: §3.7's 52-week ratio, §3.6's resistance level, and
§3.3's `change_pct` across an ex-dividend date.

It is tempting to note that the two providers write different `source` values so
nothing mixes today. That is true and it is not a fix — it holds only for as long
as nobody queries `equity_ohlcv` across both values without knowing to treat them
differently, which is precisely the trap a future reader falls into. The
conditions are all in place for it: the scanner filters by `UNIVERSE_BAR_SOURCE`,
`data-technical` writes Yahoo bars for its own symbols independently, and
`data-analyzer`'s bar reader already *prefers* `source = 'yahoo_finance'` when
deduplicating across sources — so a symbol present under both would silently
resolve to the unadjusted series.

Options, none yet chosen: decode `indicators.adjclose` in the Yahoo adapter and
use it (small change, but it only fixes the dividend half and Yahoo is currently
unreachable anyway); confine the scanner to one source and assert it; or retire
the Yahoo path once a reachable provider is settled. Whichever is chosen, the
cross-source dedup preference in `data-analyzer` needs revisiting at the same
time.

#### Two bar providers, on purpose — do not "simplify" this to one

Phase 1 uses **Tiingo and Twelve Data together**, because their free tiers fail in
different directions and neither alone covers the job:

| | Tiingo | Twelve Data |
|---|---|---|
| Quota shape | **500 unique symbols / month**, resets monthly | **800 credits / day**, ~8/min, resets daily |
| Re-reading a known symbol | free (already counted this month) | costs a credit every time |
| 3-year daily history | ✅ 751 bars | ✅ 751 bars |
| Consolidated volume | ✅ | ✅ |
| Full ~4,978-symbol universe | ✗ — 10 months of monthly allowances | ✓ one-off, ~6–7 days |
| Sustained daily refresh at universe scale | ✗ | ✗ — needs 4,978/day against 800 |
| Sustained daily refresh at ~450 symbols | ✅ indefinitely | ✅ comfortably |

The split follows directly from those shapes:

- **Tiingo drives the pilot subset** (`backfill_selected = true`, ~450 symbols). Its
  monthly-unique-symbol model is *ideal* for a fixed subset refreshed daily — the
  same 450 symbols re-counted each month stay inside 500 forever — and it has no
  daily ceiling to pace around.
- **Twelve Data drives the full-universe backfill**, once the pilot has cleared
  §6's base-rate check. Its per-request model is the wrong shape for a small subset
  refreshed daily but the right shape for a one-off sweep of thousands of symbols.
- Neither can sustain a *daily refresh* of the full universe. That remains a paid-tier
  question and is deliberately not solved here.

**Why this is not over-engineering.** Using one provider for both jobs fails
concretely: Tiingo alone cannot reach 4,978 symbols inside its monthly allowance,
and Twelve Data alone burns 450 credits a day on the pilot's refresh — over half its
daily budget — for data Tiingo would re-serve for free. The two-provider split is
what makes the pilot and the full sweep both affordable on free tiers.

Sequencing is manual and gated: Twelve Data is **not** wired to run automatically.
The full-universe backfill is a deliberate go-ahead after §6's base rate is read, for
the same reason Step 3b was gated — spending days of quota backfilling 5,000 symbols
before knowing the score beats a base rate is backwards.

**Consequence for the Yahoo question.** The hotspot test is now optional rather than
blocking. Tiingo gives consolidated 3-year daily bars from the corporate network
with API-key metering, so it does not share a rate budget with everyone behind the
same egress — the failure mode that killed Yahoo here cannot recur. Salvaging Yahoo
would save an adapter; it is no longer the only path forward.

#### 2. Finnhub works — including the endpoint 3b depends on

Verified with the configured key: `/quote` → 200, `/stock/metric` → 200 with a
242 KB payload, `/stock/symbol?exchange=US` → 200 after one redirect. Confirmed
working from inside a container with the corporate CA mounted, which is the real
runtime path.

One trap worth recording: `/stock/symbol` answers **`302`** to a pre-signed
`static2.finnhub.io` URL rather than returning the directory inline. Go's
`http.Client` follows redirects by default so `StockSymbols` is unaffected, but a
`curl` without `-L` makes a working key look like a rejected one.

#### 3. The eligible universe is 4,978 — marginally below §2.3's estimate

Measured by running the real §3.1 rules over the live 31,051-record directory
(`TestRealDirectory_EligibleCountIsInTheExpectedRange`):

| | Count |
|---|---|
| Directory records | 31,051 |
| Excluded — venue not allowed | 22,033 |
| Excluded — type not common stock | 4,040 |
| **Eligible** | **4,978** |
| … NASDAQ | 3,193 |
| … NYSE | 1,558 |
| … NYSE American | 227 |

§2.3 expects 5,000–7,500. The shortfall is **explained, not an allowlist bug**: the
directory holds 2,223 ADRs and 422 REITs, which §3.1's strict common-stock reading
excludes. Admitting ADRs alone would land the count inside the stated band, which
suggests §2.3's estimate assumed a looser type filter. Keeping the exclusion is
still the right Phase 1 call (§3.1), and this is a good argument for
`UNIVERSE_ALLOWED_TYPES` having been made configurable rather than hardcoded.

Two smaller observations from the same run:

- Finnhub returns only **composite** MICs (`XNAS`, `XNYS`, `XASE`). The tier codes
  `XNGS`/`XNMS`/`XNCM` in the default allowlist are dead with this provider —
  harmless, and worth keeping for provider changes.
- `SkippedDuplicate` was **0**, so cross-tier duplication does not occur with this
  provider. The dedup in `BuildPlan` is defensive rather than load-bearing here,
  but it still guards the batch's primary key.

| Variable | Default | Description |
|---|---|---|
| `FINNHUB_SHARED_RATE_ENABLE` | `true` | Set `false` to revert a worker to its own in-process bucket |
| `FINNHUB_RATE_PER_SEC` | `1.0` | The **shared** sustained rate — free tier is 60/min |
| `FINNHUB_RATE_BURST` | `2.0` | Largest allowed spike; must be ≥ 1 |
| `SHARED_RATE_ACQUIRE_TIMEOUT` | `250ms` | Bound on one acquisition before degrading |
| `SHARED_RATE_MAX_SLEEP` | `5s` | Cap on a single enforced wait |
| `SHARED_RATE_WARN_EVERY` | `30s` | Degradation-warning throttle |

> Alpha Vantage is **not** coordinated here. Its free tier is 25 requests **per
> day**, a quota so small that no rate limiter helps — the constraint is handled by
> restricting which symbols reach `runOverview` at all. See
> `docs/MOMENTUM_SCANNER_PHASE1.md` §3.13.

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

data-universe    → universe_symbols (eligible US common stock, weekly)

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

---

## Cross-service runtime dependencies

Services here are separate containers, but a few share a file or a table in a
way that lets a change made for one service break another that was working.
Each entry names who depends on what, what failure looks like, and where the
wiring lives. Add to this list whenever a service starts requiring something it
does not own.

| Shared thing | Owner / writer | Required by | If it is missing |
|---|---|---|---|
| `shared/content/momentum_caveats.json` (`EVIDENCE_CAVEAT`, `RESEARCH_SCORE_CAVEAT`) | edited by hand; one source for both services | **analyst-bot** (`notifier/discord/momentum.py`) and **momentum-api** (`services/data-analyzer/cmd/momentum-api`) | Both refuse to start and log exactly which path was tried and how to fix it. There is deliberately no fallback text — a fallback would be a second source of the claim |
| `watchlist_items` (migration 024) | momentum-api (`PUT/DELETE /api/v1/watchlist/{symbol}`) | **intraday-bars** reads it to pick symbols | Not a failure: the job just fetches candidates only |
| `momentum_features`, `momentum_scores` | `momentum-scanner` (data-analyzer) | analyst-bot (`/scanner`, `/score`, alerts), momentum-api | Not a startup failure: both serve the last stored scan; momentum-api reports it via `scan.is_stale`. Evening alerts are skipped (logged) until a scan for the session that just closed exists |
| `equity_ohlcv` daily `tiingo` bars for the session | `data-universe` (this service) | **momentum-daily** (data-analyzer) waits for ≥95% coverage before running scanner → tracker | The chain waits, then gives up on that session after 14h and logs it; no scan, no tracker update, no bot alert that day |

**The caveats file is the one that can break a running service.** It was
introduced for momentum-api, but analyst-bot reads it too. Rebuilding or
redeploying the bot without the Compose wiring stops the bot at boot with
`analyst-bot cannot start: ...` naming the missing file or unset `MOMENTUM_CAVEATS_PATH` and the fix. The
wiring both services need, in `infra/docker-compose.yml`:

```yaml
environment:
  MOMENTUM_CAVEATS_PATH: /shared/content/momentum_caveats.json
volumes:
  - ../shared/content:/shared/content:ro
```

Neither Docker image bakes the file in (each build context is its own service
directory), so any deployment outside this Compose file must provide the same
mount. Running from a repo checkout needs nothing: both default to the
repo-relative path.

## Multi-source `equity_ohlcv`: the reader's preference is a correctness setting

Once more than one provider writes `equity_ohlcv`, "which source wins" stops
being a tie-break and becomes a correctness decision, because the providers do
not agree on what their prices mean:

| `source` | Split adjusted | Dividend adjusted | Volume |
|---|---|---|---|
| `tiingo` | yes (`adj*` fields) | **yes** | consolidated |
| `yahoo_finance` | unverified | **no** | consolidated |
| `alpaca` | n/a | n/a | IEX only — rejected by §2.2 |

The Yahoo row is not a guess about Yahoo's API. It is a fact about this
repository: `internal/fetch/yahoo` decodes `indicators.quote` and the string
`adjclose` appears nowhere in the package, so whatever dividend adjustment Yahoo
offers in `indicators.adjclose` is absent from our rows by construction.

`data-analyzer`'s `QueryEquityBars` therefore prefers `tiingo` first. It
previously preferred `yahoo_finance`, which meant a symbol covered by both
silently resolved to the unadjusted series — **strictly worse than either source
alone**, because the symbol looked fully covered while serving prices that drift
from the adjusted series by the cumulative dividend. That shifts every
price-derived feature across any ex-dividend date: 52-week ratios, resistance
levels, and `change_pct`.

Two properties of that preference are load-bearing and each has a test that
fails when it is broken:

- It is a **preference, not a filter**. A Yahoo-only symbol still returns its
  Yahoo bars; preferring Tiingo must not drop coverage.
- It applies **per timestamp, not per symbol**. A symbol whose history is partly
  Yahoo and partly Tiingo keeps every distinct date, rather than being truncated
  to the range Tiingo happens to cover.

## Integration tests truncate tables and must never see a real database

The destructive fixtures in `internal/store` call `clearUniverse`, which is a
wholesale `DELETE FROM universe_symbols`. `requireScratchDB` refuses to run
unless the connected database's **name** contains `test`.

This was added after `TEST_DATABASE_URL` was pointed at a populated database
during Phase 1 bring-up. The damage was silent in both directions: the real
universe was deleted, and ~1,000 fixture symbols (`MKT0000`, `PNY0042`) were
left behind marked `is_eligible`, where the next stratified pilot draw selected
them. A sample containing `MKT0042` still has a perfectly normal row count.

The guard checks the database name rather than row counts or symbol shapes
because neither of those can separate the two cases: `seedPriced` legitimately
creates 1,000 rows, and other tests in the package use bare tickers like `AAA`
and `AAPL`. It is deliberately not an opt-in env flag — a flag gets set once and
then forgotten, at which point it protects nothing. `clearUniverse` also now
registers `t.Cleanup` so fixtures cannot outlive the test that created them.

Run integration tests as:

```bash
TEST_DATABASE_URL="postgres://<user>:<password>@localhost:5432/trading_test?sslmode=disable" \
  go test -tags=integration ./...
```

---

## Bar providers: Tiingo is primary, Twelve Data is disqualified

> **Supersedes the section below.** Twelve Data was adopted as primary on
> throughput grounds and withdrawn the same day when the Tiingo cross-validation
> set exposed an adjustment defect. The original reasoning is kept underneath
> because the throughput argument was correct and is worth not re-deriving — it
> simply lost to a correctness problem.

| `source` | Role | Adjustment | 450 symbols |
|---|---|---|---|
| `tiingo` | **PRIMARY** | split + dividend, **one consistent factor per series (verified)** | ~9 h (50 req/clock-hour) |
| `twelve_data` | **disqualified** | alternates adjusted/unadjusted **within one response** | ~56 min |
| `yahoo_finance` | legacy, unfit for §3 | no dividend adjustment | n/a |

### The defect

`adjust=all` applies adjustment inconsistently, bar by bar, inside a single
response. ABTS across a 1-for-15 reverse split, alongside Tiingo for the same
window:

| date | Tiingo | Twelve Data | |
|---|---|---|---|
| 2025-02-26 | 6.3045 | 0.4200 | unadjusted |
| 2025-02-27 | 6.3150 | 0.4210 | unadjusted |
| 2025-02-28 | 6.2400 | **6.2400** | adjusted |
| 2025-03-03 | 6.2250 | **6.2250** | adjusted |
| 2025-03-04 | 5.2725 | 0.3520 | unadjusted again |
| 2025-03-10 | 3.7320 | **3.7320** | adjusted again |

Every individual bar is internally consistent, so nothing in the response looks
malformed. The damage is at the seams, which fabricate one-day moves of
**+1382%, -94%, +796%, +925%**.

Scope on the 450-symbol pilot: **12-17% of symbols**, and **41% of the penny
bucket** (37 of 90) versus 10.6% of the market bucket. The penny bucket is half
the pilot's purpose.

This is the worst possible failure for a momentum scanner, because a fabricated
+796% day is exactly the signal being hunted — every corrupted symbol sorts to
the top of the scan looking like a flawless breakout.

### Why a hybrid was rejected

"Use Twelve Data where it looks clean" is not available, because clean cannot be
established locally: **APAM, AQN and ARX disagree with Tiingo by 4.02%, 3.52% and
2.09% with no detectable discontinuity anywhere in their series.** Any threshold
low enough to catch those flags real penny-stock moves as well.

NVDA's forward 10:1 split adjusts correctly, which is why the split fixture
passed. The failures cluster on recent and reverse splits in micro-caps — the
population the pilot deliberately over-samples.

### What was kept from the episode

- **`internal/barquality`** — an adjustment-seam detector, with the real ABTS
  series as a permanent fixture. It finds this defect with ONE provider, which is
  the gap that let it through: the original discovery needed a second provider.
- **`cmd/bar-audit`** — runs that detector over stored rows and exits non-zero,
  so it can gate a pipeline. `go run ./cmd/bar-audit -source tiingo`.
- **The Twelve Data adapter**, tests and all, so the defect stays reproducible
  and a future fix can be verified in minutes.

`barquality` is a **screen, not a verdict**: measured false-positive rate is ~3%
of symbols on a correct source (2 of 64), because a real gap-up can land near a
round split factor. The `Signals` field supports triage —
`dollar_volume_continuous` is strong evidence of a seam, while
`overnight_gap_on_split_factor` alone with dollar volume above ~20x is usually a
real move.

### Tiingo's real limits (the earlier note was wrong)

The Tiingo limiter previously paced at 1.5 req/sec and called itself "politeness
only", reasoning that the monthly unique-symbol count was the only constraint a
rate limiter could not express. That spent the hourly allowance in under a minute
and failed 69 symbols, each burning 4 attempts against a refusal that could not
clear for the rest of the hour.

```
50 requests / hour    HARD, fixed-clock reset (measured: blocked 05:54 UTC,
                      recovered 06:01 UTC — not a rolling window)
1,000 requests / day  resets midnight EST — expressible, now enforced
500 unique symbols/mo the only part api_rate_budget cannot express
```

50/hour is **0.0139 req/sec**, and that is the true throughput: 450 symbols takes
~9 hours and no setting shortens it. A higher value only converts waiting into
429s.

---

## Bar providers: who is primary, who is a cross-check, and why

**Tiingo is not abandoned. It is demoted to validation.** There is a working,
fully tested Tiingo adapter sitting next to the one in use, and that is on
purpose — it is the instrument that verifies the primary, not dead code awaiting
deletion.

| `source` | Role | Split adj. | Dividend adj. | Adjusted volume | Throughput for 450 symbols |
|---|---|---|---|---|---|
| `twelve_data` | ~~PRIMARY~~ **disqualified, see above** | yes | yes, **only with `adjust=all`** | yes | **~56 min** (8 credits/min) |
| `tiingo` | ~~cross-validation~~ **PRIMARY** | yes | yes (`adj*` fields) | yes (`adjVolume`) | ~9 h (50 req/clock-hour) |
| `yahoo_finance` | legacy, unfit for §3 | unverified | **no** | no | n/a |

### Why Twelve Data became primary

Not quality — the two agree to **-0.0028% on close and +0.0000% on volume**.
Purely throughput and quota shape:

- **Tiingo Starter (free):** 50 requests per *clock hour* (fixed-clock reset,
  measured: blocked 05:54 UTC, recovered 06:01 UTC), 1,000/day, and **500 unique
  symbols/month**. 450 symbols is therefore a **~9-hour** job, and the
  unique-symbol meter raises an unanswerable question after a failed run — there
  is no account-usage endpoint on this tier, so "did those 136 retries burn
  allowance?" cannot be checked.
- **Twelve Data (free):** **8 credits/minute** (measured; the 429 names the
  count), 800/day, **no unique-symbol cap** for US equities. Same 450 symbols in
  **~56 minutes**, and nothing to reason about afterwards.

The full 450 were redrawn on Twelve Data rather than only the 386 that Tiingo had
not reached. A single-source pilot avoids putting two different adjustment
formulas into one dataset: 0.05% sounds negligible until it lands on a §3.2 price
threshold or a §3.6 breakout confirmation for a handful of symbols, at which
point "why did this one symbol gate differently" has the answer "half the rows
came from a different provider" — which is exactly the class of hidden
inconsistency this pipeline is built to avoid.

The 64 symbols Tiingo had already completed were **kept**, giving 64 names with
two independently adjusted series. That is a stronger correctness check on the
new adapter than any unit test, and it was a bonus rather than the reason.

### `adjust=all` is mandatory, and its absence is silent

Twelve Data's default output is split-adjusted but **not** dividend-adjusted. The
adapter always sends `adjust=all`, and a test pins it, because omitting it
produces a completely plausible series that reproduces the exact defect that
disqualified Yahoo. Evidence, against `ALL` and `AWR` over three years: the
default sits a constant **+6.32%** and **+7.52%** above the adjusted close — the
signature of cumulative dividends.

### Volume adjustment was verified, not inferred from the parameter name

A provider that rescales OHLC while leaving volume raw would silently corrupt
§3.4's RVOL and volume acceleration for every symbol that ever split. `adjust=all`
does not promise anything about volume, so it was measured against NVDA's 10:1
split of 2024-06-10, using Tiingo's raw/adjusted pair as ground truth for the
last pre-split session:

```
                        close        volume
Tiingo raw              1208.88      41,238,580
Tiingo fully adjusted    120.5447    412,385,800
Twelve Data default      120.888     412,386,000   <- volume already split-adjusted
Twelve Data adjust=all   120.5414    412,386,000   <- price now dividend-adjusted too
```

Volume is **10.0000x** Tiingo's raw figure, i.e. split-adjusted, in *both* modes.
`adjust=all` changes price only — and that is correct, not a gap: a dividend does
not change share count, so only splits may rescale volume. Both the hermetic
fixture and a live test assert these absolute numbers.

## Credentials must never reach an error string

`*url.Error` — what `http.Client.Do` returns for every transport failure —
embeds the **full request URL**. For a provider that authenticates by query
parameter, the API key is therefore inside the error text, and that text travels
wherever errors go. This was not hypothetical: a run wrote **23 Twelve Data API
keys in plaintext** into `universe_symbols.backfill_last_error`.

Three changes, in order of how much they help:

1. **Tiingo's token moved to the `Authorization: Token <token>` header**, which
   its own spec documents. The credential is no longer in the URL, so it cannot
   be in a `*url.Error` at all. This is the real fix.
2. **`barsource.RedactSecrets`** rewrites credential query values to `REDACTED`,
   and every wrapper around a transport error passes through it. Note those
   wrappers use `%s`, not `%w`: wrapping would keep the unredacted text
   reachable via `errors.Unwrap`.
3. Redaction preserves the symbol, endpoint and cause, so it does not trade a
   leak for an unreadable log. A test asserts both halves.

Worth stating because it is the easy mistake: the first draft of the redaction
test pasted the real leaked error verbatim and thereby committed the live key —
the same failure one layer up. **A fixture that needs a secret needs a fake one.**

> **The exposed Twelve Data key should be rotated.** It was written to the
> database and briefly existed in a working tree. Redaction prevents recurrence;
> it does not un-expose what already leaked.

## Bar-fetch HTTP knobs are provider-neutral

`UNIVERSE_BAR_TIMEOUT`, `UNIVERSE_BAR_MAX_RETRIES`, `UNIVERSE_BAR_BACKOFF_*` and
`UNIVERSE_BAR_BURST` apply to whichever provider `UNIVERSE_BAR_SOURCE` names. The
`UNIVERSE_YAHOO_*` spellings still work as fallbacks.

They were *only* called `UNIVERSE_YAHOO_*` while feeding all three adapters,
which is how Twelve Data inherited Yahoo's 30-second timeout and failed 12
symbols with `Client.Timeout exceeded while awaiting headers` — a three-year
daily response is ~100KB, and each failure spent 4 requests of the daily
allowance to produce nothing. The default is now 90s. The same misnaming made the
startup log report `bar_req_per_sec=2` while the Twelve Data limiter was pacing at
`0.125`; it now reports the provider's real rate.

## SEC EDGAR — the 403 that is not a rate limit or an IP block

Phase 2 §3.2 reads point-in-time share counts from SEC's XBRL API. Reachability
was verified from this network on 2026-09-21, and the way it fails is worth
knowing before you spend an hour on it.

**SEC denylists some contact domains in the User-Agent.** Measured:

| User-Agent | Result |
|---|---|
| `TradingAgentResearch <your-contact-email>` | **200** |
| `Trading Agent Research <your-contact-email>` | **200** |
| `TradingAgentResearch` (no email at all) | **200** |
| `TradingAgentResearch <a-noreply-github-address>` | **403** |

Only the `users.noreply.github.com` contact fails. The identical UA with a real
domain succeeds, and so does one with no email.

**The error text actively misdirects.** The 403 bodies are titled
*"SEC.gov | Request Rate Threshold Exceeded"* and *"Your Request Originates from
an Undeclared Automated Tool"* — after about five requests. Both point at rate
limiting or bot detection. Neither is the cause. This egress is a shared
Zscaler corporate proxy (a shared corporate proxy), which makes "our IP is
blocked" the obvious hypothesis and the wrong one.

**If EDGAR returns 403: check `SEC_EDGAR_USER_AGENT` first.** Use a real domain
you control. The value is an env var, not a literal, precisely because it is the
single setting that decides whether requests succeed.

**Rate.** SEC documents ~10 req/s fair access. `SEC_EDGAR_RATE_PER_SEC` defaults
to 3. Lower on purpose: the published rate is per IP, and on a shared proxy our
share of it is not ours to assume.

**A 403 mid-run stops the backfill.** `scripts/edgar_backfill.py` halts rather
than continuing, because a 403 is a request problem, not a coverage fact, and
recording thousands of them as "no EDGAR data" would turn our own error into a
permanent bias in the dataset.
