# ADR-0034: Opt-in schema migration

**Date:** 2026-06-23  
**Status:** Accepted  
**Deciders:** Contributors (dev workflow + cloud Postgres latency)

## Context

`taskapi` ran GORM AutoMigrate on every process start. Over remote Postgres, hundreds of introspection queries added 90–120+ seconds before HTTP bound, causing `dev.ps1` to kill the process (90s readiness timeout) and produce misleading `exit -1` errors.

Operators need fast dev restarts while still applying schema changes safely after code pulls and production deploys.

## Decision

1. **Two-step operator workflow:** schema migrate is explicit (`scripts/migrate.*` / `dbcheck -migrate`), separate from starting servers (`scripts/dev.*` / `taskapi`).
2. **Opt-in migrate on `taskapi` boot:** `-migrate` flag or `HAMIX_MIGRATE=1`; default is skip.
3. **Schema revision versioning:** integer `SchemaRevision` in code; `schema_meta` row updated after successful `postgres.Migrate`.
4. **Drift detection on every boot:** compare code vs DB revision; alert via stderr, structured log, and `GET /health/ready` (503 when pending/downgrade).
5. **CI:** fail when domain/migrate files change without bumping `SchemaRevision`.

Production deploy: run `dbcheck -migrate` (or `scripts/migrate.sh`) as release step 1; roll out `taskapi` without migrate on step 2.

## Consequences

### Positive

- Dev restarts against cloud DB complete in seconds when schema is current.
- Operators get an explicit, visible migrate step.
- Missed migrate after `git pull` surfaces immediately (stderr + `/health/ready`).

### Negative / Trade-offs

- Contributors must run migrate after schema-changing pulls.
- Existing databases need one `migrate.ps1` / `dbcheck -migrate` after this lands to set `schema_meta`.
- `SchemaRevision` must be bumped manually when domain models change (CI enforces).

## Alternatives Considered

| Alternative | Reason rejected |
| --- | --- |
| Keep migrate-on-every-boot | Cloud latency; dev script timeout failures |
| Numbered SQL migrations (golang-migrate) | Large migration from GORM AutoMigrate; deferred |
| Skip drift detection | Opt-in migrate alone leaves silent schema mismatch |
