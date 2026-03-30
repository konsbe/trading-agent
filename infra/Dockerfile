# TimescaleDB image with shared migrations baked in (build from repository root).
# Local dev typically uses infra/docker-compose.yml volume mount instead.
FROM timescale/timescaledb:latest-pg16
COPY shared/databases/migrations/ /docker-entrypoint-initdb.d/
