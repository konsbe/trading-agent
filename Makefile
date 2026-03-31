ROOT    := $(abspath .)
INFRA   := $(ROOT)/infra
INGEST  := $(ROOT)/services/data-ingestion
ANALYZER := $(ROOT)/services/data-analyzer

.PHONY: help tidy build-ingestion build-analyzer db-up db-down up down deploy ensure-env \
	docker-build-timescaledb restart clean psql \
	db-crypto-ohlcv db-crypto-global db-equity-ohlcv db-macro-fred db-onchain db-sentiment db-news db-tables \
	db-technical db-technical-symbol db-fundamental db-fundamental-symbol \
	log-docker-compose log-services log-analyzer \
	log-technical log-fundamental log-technical-analysis log-fundamental-analysis

help:
	@echo "━━━ Stack management ━━━"
	@echo "  make up            Full stack: DB + Redis + all ingestion + analyzer workers (--build)"
	@echo "  make up-ingestion  Ingestion workers only (DB must be running)"
	@echo "  make up-analyzer   Analyzer workers only (DB + ingestion must be running)"
	@echo "  make down          Stop everything (volumes kept)"
	@echo "  make restart       down + up --build; database data preserved"
	@echo "  make clean         Nuclear: remove containers, volumes (DB wiped), and local images"
	@echo "  make db-up         TimescaleDB + Redis only"
	@echo ""
	@echo "━━━ Local dev (no Docker) ━━━"
	@echo "  make db-up && cp .env.example .env"
	@echo "  make build-ingestion   → services/data-ingestion/bin/"
	@echo "  make build-analyzer    → services/data-analyzer/bin/"
	@echo "  cd services/data-ingestion && ./bin/data-technical"
	@echo "  cd services/data-analyzer  && ./bin/technical-analysis"
	@echo ""
	@echo "━━━ Logs ━━━"
	@echo "  log-services            All ingestion containers"
	@echo "  log-analyzer            Both analyzer containers (technical-analysis + fundamental-analysis)"
	@echo "  log-technical           data-technical (OHLCV bar fetcher)"
	@echo "  log-fundamental         data-fundamental (Finnhub fundamentals fetcher)"
	@echo "  log-technical-analysis  technical-analysis (indicator computation)"
	@echo "  log-fundamental-analysis fundamental-analysis (derived ratio computation)"
	@echo "  log-docker-compose      All containers"
	@echo ""
	@echo "━━━ DB inspection ━━━"
	@echo "  db-tables              Row counts for every table"
	@echo "  db-technical           Latest computed indicators (technical_indicators)"
	@echo "  db-technical-symbol    Indicators for one symbol (default SPY; override SYMBOL=BTCUSDT)"
	@echo "  db-fundamental         Latest TTM fundamental metrics"
	@echo "  db-fundamental-symbol  Fundamentals for one symbol (default AAPL; override SYMBOL=MSFT)"
	@echo "  db-crypto-ohlcv        crypto_ohlcv  (Binance bars)"
	@echo "  db-equity-ohlcv        equity_ohlcv  (Yahoo/Alpaca/Finnhub bars)"
	@echo "  db-macro-fred          macro_fred     (FRED series)"
	@echo "  db-onchain             onchain_metrics (Etherscan/Glassnode)"
	@echo "  db-sentiment           sentiment_snapshots (LunarCrush)"
	@echo "  db-news                news_headlines (Finnhub news)"

ensure-env:
	@test -f $(ROOT)/.env || cp $(ROOT)/.env.example $(ROOT)/.env

tidy:
	$(MAKE) -C $(INGEST) tidy
	$(MAKE) -C $(ANALYZER) tidy

build-ingestion:
	$(MAKE) -C $(INGEST) build

build-analyzer:
	$(MAKE) -C $(ANALYZER) build

db-up:
	$(MAKE) -C $(INFRA) db-up

db-down:
	$(MAKE) -C $(INFRA) db-down

up deploy: ensure-env
	$(MAKE) -C $(INFRA) up

up-ingestion: ensure-env
	$(MAKE) -C $(INFRA) up-ingestion

up-analyzer: ensure-env
	$(MAKE) -C $(INFRA) up-analyzer

down:
	$(MAKE) -C $(INFRA) down

restart: ensure-env
	$(MAKE) -C $(INFRA) restart

clean:
	$(MAKE) -C $(INFRA) clean

docker-build-timescaledb:
	$(MAKE) -C $(INFRA) build-timescaledb

COMPOSE := docker compose -f $(ROOT)/infra/docker-compose.yml --profile ingestion --profile analyzer

log-docker-compose:
	$(COMPOSE) logs -f

log-services:
	$(COMPOSE) logs -f \
	  data-crypto data-equity data-fundamental data-onchain data-sentiment data-technical

log-analyzer:
	$(COMPOSE) logs -f technical-analysis fundamental-analysis

log-technical:
	$(COMPOSE) logs -f data-technical

log-fundamental:
	$(COMPOSE) logs -f data-fundamental

log-technical-analysis:
	$(COMPOSE) logs -f technical-analysis

log-fundamental-analysis:
	$(COMPOSE) logs -f fundamental-analysis

psql:
	docker exec -it infra-timescaledb-1 psql -U postgres -d trading

# --- DB table inspection targets ---
DB := docker exec infra-timescaledb-1 psql -U postgres -d trading -x -c

# row counts for all tables at once
db-tables:
	$(DB) "SET max_parallel_workers_per_gather = 0; \
	SELECT 'crypto_ohlcv'          AS \"table\", count(*) AS rows FROM crypto_ohlcv \
	UNION ALL SELECT 'crypto_global_metrics',    count(*) FROM crypto_global_metrics \
	UNION ALL SELECT 'equity_ohlcv',             count(*) FROM equity_ohlcv \
	UNION ALL SELECT 'macro_fred',               count(*) FROM macro_fred \
	UNION ALL SELECT 'onchain_metrics',          count(*) FROM onchain_metrics \
	UNION ALL SELECT 'sentiment_snapshots',      count(*) FROM sentiment_snapshots \
	UNION ALL SELECT 'news_headlines',           count(*) FROM news_headlines \
	UNION ALL SELECT 'technical_indicators',     count(*) FROM technical_indicators \
	UNION ALL SELECT 'equity_fundamentals',      count(*) FROM equity_fundamentals \
	ORDER BY 1;"

# Binance BTC/ETH bars
db-crypto-ohlcv:
	$(DB) "SELECT ts, exchange, symbol, interval, open, high, low, close, volume, source \
	FROM crypto_ohlcv ORDER BY ts DESC LIMIT 20;"

# Alpaca AAPL/MSFT/SPY bars
db-crypto-global:
	$(DB) "SELECT ts, provider, payload FROM crypto_global_metrics ORDER BY ts DESC LIMIT 20;"

# Alpaca AAPL/MSFT/SPY bars
db-equity-ohlcv:
	$(DB) "SELECT ts, symbol, interval, open, high, low, close, volume, source \
	FROM equity_ohlcv ORDER BY ts DESC LIMIT 20;"

# Fred macro data (empty until paid plan)
db-macro-fred:
	$(DB) "SELECT ts, series_id, value FROM macro_fred ORDER BY ts DESC LIMIT 20;"

# Etherscan ETH supply (Glassnode empty until paid plan)
db-onchain:
	$(DB) "SELECT ts, asset, metric, value, source FROM onchain_metrics ORDER BY ts DESC LIMIT 20;"

# LunarCrush (empty until paid plan)
db-sentiment:
	$(DB) "SELECT ts, source, symbol, score FROM sentiment_snapshots ORDER BY ts DESC LIMIT 20;"

# Finnhub news (empty until TLS fixed)
db-news:
	$(DB) "SELECT ts, source, symbol, headline, url FROM news_headlines ORDER BY ts DESC LIMIT 20;"

# Latest computed technical indicators (most recent value per symbol/exchange/interval/indicator)
db-technical:
	$(DB) "SELECT symbol, exchange, interval, indicator, round(value::numeric, 4) AS value, ts \
	FROM technical_indicators \
	ORDER BY symbol, exchange, interval, indicator, ts DESC \
	LIMIT 60;"

# Detailed view: latest indicators for a specific symbol (default SPY; override with SYMBOL=BTCUSDT)
db-technical-symbol:
	$(DB) "SELECT indicator, round(value::numeric, 4) AS value, payload, ts \
	FROM technical_indicators \
	WHERE symbol = '$(or $(SYMBOL),SPY)' \
	ORDER BY indicator, ts DESC \
	LIMIT 40;"

# Latest fundamental metrics (most recent TTM snapshot per symbol/metric)
db-fundamental:
	$(DB) "SELECT symbol, period, metric, round(value::numeric, 4) AS value, source, ts \
	FROM equity_fundamentals \
	WHERE period = 'ttm' AND metric NOT IN ('metrics_raw','report_raw','earnings_raw') \
	ORDER BY symbol, metric, ts DESC \
	LIMIT 60;"

# Fundamentals for a specific symbol (default AAPL; override with SYMBOL=MSFT)
db-fundamental-symbol:
	$(DB) "SELECT period, metric, round(value::numeric, 4) AS value, payload, source, ts \
	FROM equity_fundamentals \
	WHERE symbol = '$(or $(SYMBOL),AAPL)' \
	AND metric NOT IN ('metrics_raw','report_raw','earnings_raw') \
	ORDER BY period, metric, ts DESC \
	LIMIT 60;"