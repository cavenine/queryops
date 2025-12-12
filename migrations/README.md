# Database Migrations

This directory contains PostgreSQL 18 schema migrations managed by [golang-migrate](https://github.com/golang-migrate/migrate).

- Migrations are stored under `sql/` and embedded into the binary via Go's `embed` package.
- Filenames follow the pattern `{version}_{title}.up.sql` and `{version}_{title}.down.sql`,
  where `version` is an incrementing integer (e.g. `1_init_schema.up.sql`).
- Every `.up.sql` file must have a corresponding `.down.sql` that reverses its changes when possible.
- Migrations should be small, focused changes and use transactional DDL where PostgreSQL allows it.

## Authoring a New Migration

1. Pick the next integer version (e.g. `2`).
2. Create `sql/2_some_change.up.sql` and `sql/2_some_change.down.sql`.
3. Add the appropriate PostgreSQL statements to each file.
4. Run the `cmd/migrate` tool to apply or roll back migrations.

## Running Migrations

- Use the dedicated `cmd/migrate` command for database migrations.
- The application and migration tooling share the same `DATABASE_URL`, which must use the `postgres://` URL scheme.
- In development, you can optionally enable automatic migrations on web startup via the `AUTO_MIGRATE=true` environment variable.
