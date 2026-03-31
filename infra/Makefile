ROOT    := $(abspath ..)
# --env-file ensures variable substitution in docker-compose.yml (e.g. ${CORP_CA_PEM_PATH})
# reads from the root .env, same file that env_file: ../.env injects into containers.
COMPOSE := docker compose -f $(ROOT)/infra/docker-compose.yml --env-file $(ROOT)/.env

.PHONY: db-up db-down up down up-ingestion up-analyzer build-timescaledb restart clean

# Databases + Redis only (data volume persists across down/up without -v)
db-up:
	cd $(ROOT) && $(COMPOSE) up -d timescaledb redis

db-down:
	cd $(ROOT) && $(COMPOSE) --profile ingestion --profile analyzer down

# Full stack: DB, Redis, ingestion workers, and analyzer workers.
up:
	cd $(ROOT) && $(COMPOSE) --profile ingestion --profile analyzer up -d --build

# Stops every service; does not remove volumes.
down:
	cd $(ROOT) && $(COMPOSE) --profile ingestion --profile analyzer down

# Stop and restart everything; data volumes are preserved.
restart:
	cd $(ROOT) && $(COMPOSE) --profile ingestion --profile analyzer down \
	  && $(COMPOSE) --profile ingestion --profile analyzer up -d --build

# Stop all services, remove volumes (wipes all DB data), orphans, and local images.
clean:
	cd $(ROOT) && $(COMPOSE) --profile ingestion --profile analyzer down -v --remove-orphans --rmi local

# Ingestion workers only (DB must already be running).
up-ingestion:
	cd $(ROOT) && $(COMPOSE) --profile ingestion up -d --build

# Analyzer workers only (DB + ingestion must already be running and populated).
up-analyzer:
	cd $(ROOT) && $(COMPOSE) --profile analyzer up -d --build

# Optional: Timescale image with migrations copied in (context = repo root)
build-timescaledb:
	docker build -f $(ROOT)/infra/Dockerfile -t trading-agent/timescaledb:local $(ROOT)
