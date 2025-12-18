# Database Migrations

This directory contains PostgreSQL schema migrations managed by [golang-migrate](https://github.com/golang-migrate/migrate).

- Migrations are stored under `sql/` and embedded into the binary via Go's `embed` package.
- Filenames follow the pattern `{version}_{title}.up.sql` and `{version}_{title}.down.sql`.
- `version` is a human-readable UTC timestamp: `YYYYMMDDHHMMSS` (e.g. `20251218094501_add_users_table.up.sql`).
- Every `.up.sql` file must have a corresponding `.down.sql` that reverses its changes when possible.
- Migrations should be small, focused changes and use transactional DDL where PostgreSQL allows it.

## Creating a New Migration

Use the Taskfile wrapper (preferred):

```bash
go tool task migrate:create -- add_users_table
```

Under the hood this uses the upstream `golang-migrate` CLI, which is included as a Go-managed tool in `go.mod`.

- Use it for authoring only (`create`), so migration filenames are always consistent.
- Do not use it to apply/rollback migrations in this repo; use the app runner (`go tool task migrate*`) so River migrations are handled too.

```bash
go tool migrate -help
```

This creates:

- `migrations/sql/<timestamp>_add_users_table.up.sql`
- `migrations/sql/<timestamp>_add_users_table.down.sql`

## Writing Migrations (Idempotent When Possible)

When reasonable, write migrations so they can be re-run safely (helpful during development, branch switching, and recoveries).

Prefer these patterns:

- Tables: `CREATE TABLE IF NOT EXISTS ...`
- Indexes: `CREATE INDEX IF NOT EXISTS ...`
- Columns:
  - Add: `ALTER TABLE ... ADD COLUMN IF NOT EXISTS ...`
  - Drop: `ALTER TABLE ... DROP COLUMN IF EXISTS ...`
- Drops:
  - `DROP TABLE IF EXISTS ...`
  - `DROP INDEX IF EXISTS ...`
- Seed/config rows:
  - Prefer upserts: `INSERT ... ON CONFLICT (...) DO UPDATE ...`

Example (idempotent seed row):

```sql
INSERT INTO osquery_configs (name, config)
VALUES ('default', '{"schedule": {}}'::jsonb)
ON CONFLICT (name) DO UPDATE
SET config = EXCLUDED.config,
    updated_at = CURRENT_TIMESTAMP;
```

Trade-offs:

- Not every migration can be safely idempotent (e.g. data backfills that must run exactly once).
- If an operation must run exactly once, keep it non-idempotent and make sure the `.down.sql` is correct.

## Running Migrations

Use the app’s migration runner (it applies app migrations and River migrations together):

- Apply all pending migrations: `go tool task migrate`
- Roll back one migration: `go tool task migrate:down`
- Print current version: `go tool task migrate:version`
- Migrate to a version: `go tool task migrate:to -- VERSION=20251218094501`
- Force-set version (use with care): `go tool task migrate:force -- VERSION=20251218094501`

Notes:

- Migration tasks use the same `DATABASE_URL` as the app, and it must use the `postgres://` URL scheme.
- In development, you can optionally enable automatic migrations on web startup via the `AUTO_MIGRATE=true` environment variable.
