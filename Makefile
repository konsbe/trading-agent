ROOT := $(abspath .)
INFRA := $(ROOT)/infra
INGEST := $(ROOT)/services/data-ingestion

.PHONY: help tidy build-ingestion db-up db-down up down deploy ensure-env docker-build-timescaledb restart clean log-docker-compose check-services-logs psql \
	db-crypto-ohlcv db-crypto-global db-equity-ohlcv db-macro-fred db-onchain db-sentiment db-news db-tables

help:
	@echo "Run the stack"
	@echo "  1) make db-up     — docker compose starts TimescaleDB + Redis; first start runs SQL in shared/databases/migrations/"
	@echo "  2) make up        — same as deploy: ensures .env exists, then builds data-ingestion images and starts"
	@echo "     all four workers (profile ingestion) with DATABASE_URL pointing at the DB container."
	@echo "  3) make down      — stops workers + DB + Redis (volumes kept)."
	@echo "  4) make restart   — down then full up --build; same DB data (no -v)."
	@echo "  5) make clean     — nuclear: removes containers, named volumes (DB wiped), local compose images."
	@echo ""
	@echo "Local dev (no Docker for Go): make db-up, cp .env.example .env, make build-ingestion, then e.g."
	@echo "  cd services/data-ingestion && ./bin/data-crypto"
	@echo ""
	@echo "Targets:"
	@echo "  db-up           - TimescaleDB + Redis only"
	@echo "  db-down         - Stop infra stack (data persists unless you use docker compose down -v)"
	@echo "  build-ingestion - Compile all ingestion binaries under services/data-ingestion/bin/"
	@echo "  up / deploy     - Full stack: DB + Redis + four ingestion containers (--build)"
	@echo "  restart         - Recreate stack with --build; preserves database volumes"
	@echo "  clean           - Remove compose stack, volumes (DB data), and built images"
	@echo "  tidy            - go mod tidy in data-ingestion"
	@echo "  ensure-env      - Create .env from .env.example if missing"
	@echo ""
	@echo "DB inspection (latest 20 rows per table):"
	@echo "  db-tables       - Row counts for every table"
	@echo "  db-crypto-ohlcv - crypto_ohlcv (Binance bars)"
	@echo "  db-crypto-global- crypto_global_metrics (CoinGecko)"
	@echo "  db-equity-ohlcv - equity_ohlcv (Alpaca/Finnhub bars)"
	@echo "  db-macro-fred   - macro_fred (FRED series)"
	@echo "  db-onchain      - onchain_metrics (Etherscan/Glassnode)"
	@echo "  db-sentiment    - sentiment_snapshots (LunarCrush)"
	@echo "  db-news         - news_headlines (Finnhub news)"

ensure-env:
	@test -f $(ROOT)/.env || cp $(ROOT)/.env.example $(ROOT)/.env

tidy:
	$(MAKE) -C $(INGEST) tidy

build-ingestion:
	$(MAKE) -C $(INGEST) build

db-up:
	$(MAKE) -C $(INFRA) db-up

db-down:
	$(MAKE) -C $(INFRA) db-down

up deploy: ensure-env
	$(MAKE) -C $(INFRA) up

down:
	$(MAKE) -C $(INFRA) down

restart: ensure-env
	$(MAKE) -C $(INFRA) restart

clean:
	$(MAKE) -C $(INFRA) clean

docker-build-timescaledb:
	$(MAKE) -C $(INFRA) build-timescaledb

log-docker-compose:
	docker compose -f $(ROOT)/infra/docker-compose.yml logs -f

check-services-logs:
	docker compose -f $(ROOT)/infra/docker-compose.yml logs -f data-crypto data-equity data-onchain data-sentiment

psql:
	docker exec -it infra-timescaledb-1 psql -U postgres -d trading

# --- DB table inspection targets ---
DB := docker exec infra-timescaledb-1 psql -U postgres -d trading -x -c

# row counts for all 7 tables at once
db-tables:
	$(DB) "SET max_parallel_workers_per_gather = 0; \
	SELECT 'crypto_ohlcv'        AS \"table\", count(*) AS rows FROM crypto_ohlcv \
	UNION ALL SELECT 'crypto_global_metrics',  count(*) FROM crypto_global_metrics \
	UNION ALL SELECT 'equity_ohlcv',           count(*) FROM equity_ohlcv \
	UNION ALL SELECT 'macro_fred',             count(*) FROM macro_fred \
	UNION ALL SELECT 'onchain_metrics',        count(*) FROM onchain_metrics \
	UNION ALL SELECT 'sentiment_snapshots',    count(*) FROM sentiment_snapshots \
	UNION ALL SELECT 'news_headlines',         count(*) FROM news_headlines \
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