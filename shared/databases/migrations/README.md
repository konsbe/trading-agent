# Migrations

Numbered SQL files, applied in order to both databases on the live
TimescaleDB container (`ta-phase1`): `trading` (live) and `trading_test`
(integration tests). They are applied by hand:

```bash
docker exec -i ta-phase1 psql -U postgres -d trading      -v ON_ERROR_STOP=1 < shared/databases/migrations/NNN_name.sql
docker exec -i ta-phase1 psql -U postgres -d trading_test -v ON_ERROR_STOP=1 < shared/databases/migrations/NNN_name.sql
```

(Compose also mounts this directory as `docker-entrypoint-initdb.d`, which runs
only when a fresh database is initialised.)

## Conventions

- **Idempotent.** Every file can be re-run: `IF NOT EXISTS`, `DO $$ … $$`
  guards, `ON CONFLICT`.
- **Never delete data.** Relabel, supersede or add columns; history stays.
- **Header comment.** Why the change exists and what it binds, so the file
  explains itself without the commit.

## Deploying a migration

- **A migration that adds a constraint on rows a running service writes is
  applied together with that service's new code, never before it.** Applied
  early, the old code keeps writing rows that break the constraint, and each
  write fails while the service keeps running. Example (2026-09-27): migration
  031's `CHECK (bar_date IS NOT NULL)` was applied to `trading` before the
  analyst bot that sets `bar_date` was deployed. The running bot's
  `fired_alerts` inserts would have failed after its Discord posts, so alerts
  would have posted without being recorded. The check was dropped within
  minutes (no insert had failed) and is re-applied in the same step as the
  bot's deploy.
- Additive changes the old code does not notice (a new nullable column, an
  index whose key the old code never duplicates) may go first; say so in the
  migration header.
- Apply to `trading_test` whenever the tests need it; that database has no
  running writers.
