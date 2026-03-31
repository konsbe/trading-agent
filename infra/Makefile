ROOT    := $(abspath ..)
# --env-file ensures variable substitution in docker-compose.yml (e.g. ${CORP_CA_PEM_PATH})
# reads from the root .env, same file that env_file: ../.env injects into containers.
COMPOSE := docker compose -f $(ROOT)/infra/docker-compose.yml --env-file $(ROOT)/.env

.PHONY: db-up db-down up down up-ingestion build-timescaledb restart clean

# Databases + Redis only (data volume persists across down/up without -v)
db-up:
	cd $(ROOT) && $(COMPOSE) up -d timescaledb redis

db-down:
	cd $(ROOT) && $(COMPOSE) --profile ingestion down

# Full stack: DB, Redis, and all data-ingestion workers (optional ../.env for API keys; see .env.example)
up:
	cd $(ROOT) && $(COMPOSE) --profile ingestion up -d --build

# Stops every service in this compose project (same project name as up); does not remove volumes.
down:
	cd $(ROOT) && $(COMPOSE) --profile ingestion down

# Stop stack and start again; does NOT pass -v (Timescale/Redis volumes keep all DB data).
restart:
	cd $(ROOT) && $(COMPOSE) --profile ingestion down && $(COMPOSE) --profile ingestion up -d --build

# Stop all compose services, remove volumes (wipes Postgres/Redis data), orphans, and locally built images for this project.
clean:
	cd $(ROOT) && $(COMPOSE) --profile ingestion down -v --remove-orphans --rmi local

# Workers only (DB must already be running)
up-ingestion:
	cd $(ROOT) && $(COMPOSE) --profile ingestion up -d --build

# Optional: Timescale image with migrations copied in (context = repo root)
build-timescaledb:
	docker build -f $(ROOT)/infra/Dockerfile -t trading-agent/timescaledb:local $(ROOT)
