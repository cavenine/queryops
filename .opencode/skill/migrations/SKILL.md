---
name: migrations
description: Create, modify, and manage PostgreSQL database migrations using golang-migrate with idempotent patterns
---

# Database Migrations Skill

Create and manage PostgreSQL schema migrations for this Go project using golang-migrate.

## When to Use

- Creating new database tables, columns, or indexes
- Modifying existing schema (adding/removing columns, changing constraints)
- Adding seed data or configuration rows
- Running or rolling back migrations
- Checking current migration version

## Project Context

- **Migration tool**: golang-migrate (embedded via Go tools)
- **Location**: `migrations/sql/`
- **Naming**: `YYYYMMDDHHMMSS_<name>.up.sql` and `.down.sql`
- **Runner**: Custom app runner that also handles River migrations

## Workflow

### 1. Create Migration Files

```bash
go tool task migrate:create -- <migration_name>
```

This creates timestamped `.up.sql` and `.down.sql` files in `migrations/sql/`.

**Naming conventions:**
- Use `snake_case` for names
- Be descriptive: `add_users_table`, `add_organization_id_to_hosts`
- No spaces allowed

### 2. Write Idempotent SQL (When Possible)

Prefer patterns that can be re-run safely:

```sql
-- Tables
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS nickname TEXT;

-- Seed/config rows (upsert)
INSERT INTO configs (name, value)
VALUES ('default', '{}')
ON CONFLICT (name) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = CURRENT_TIMESTAMP;
```

### 3. Write Down Migration

Every `.up.sql` must have a corresponding `.down.sql` that reverses it:

```sql
-- Down migration for adding a column
ALTER TABLE users DROP COLUMN IF EXISTS nickname;

-- Down migration for creating a table
DROP TABLE IF EXISTS users;

-- Down migration for creating an index
DROP INDEX IF EXISTS idx_users_email;
```

**Order matters**: Drop in reverse order of creation (indexes before tables, columns before constraints).

### 4. Run Migrations

```bash
# Apply all pending migrations
go tool task migrate

# Roll back one migration
go tool task migrate:down

# Check current version
go tool task migrate:version

# Migrate to specific version
go tool task migrate:to -- VERSION=20251218094501

# Force-set version (use with care)
go tool task migrate:force -- VERSION=20251218094501
```

## Common Patterns

### Foreign Keys with Cascade

```sql
CREATE TABLE IF NOT EXISTS organization_members (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    PRIMARY KEY (user_id, organization_id)
);
```

### UUID Primary Keys

```sql
CREATE TABLE IF NOT EXISTS hosts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

### JSONB Columns

```sql
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS host_details JSONB;

-- With default
ALTER TABLE configs ADD COLUMN IF NOT EXISTS settings JSONB DEFAULT '{}'::jsonb;
```

### Unique Constraints (Composite)

```sql
-- Add unique constraint on (org_id, host_identifier)
ALTER TABLE hosts 
ADD CONSTRAINT hosts_org_host_identifier_unique 
UNIQUE (organization_id, host_identifier);
```

### Partial Indexes

```sql
-- Index only active records
CREATE INDEX IF NOT EXISTS idx_secrets_active 
ON organization_enroll_secrets(organization_id) 
WHERE active = true;
```

## Non-Idempotent Cases

Some operations must run exactly once:

- Data backfills with complex logic
- Renaming columns/tables (use explicit renames, not IF EXISTS)
- Constraint modifications that could fail on existing data

For these, ensure `.down.sql` correctly reverses the change.

## Troubleshooting

### Migration Stuck (Dirty State)

If a migration fails mid-way:

```bash
# Check current state
go tool task migrate:version

# Force to last known good version
go tool task migrate:force -- VERSION=<previous_version>
```

### Schema Conflicts

When multiple developers create migrations:

1. Check for conflicting timestamps
2. Rebase and rename if needed
3. Ensure migrations are ordered correctly

## Example: Complete Migration

**File**: `migrations/sql/20251220123456_add_campaigns.up.sql`

```sql
CREATE TABLE IF NOT EXISTS campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    query TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_campaigns_organization_id ON campaigns(organization_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);

CREATE TABLE IF NOT EXISTS campaign_targets (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    host_id UUID NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    result JSONB,
    completed_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (campaign_id, host_id)
);
```

**File**: `migrations/sql/20251220123456_add_campaigns.down.sql`

```sql
DROP TABLE IF EXISTS campaign_targets;
DROP TABLE IF EXISTS campaigns;
```

## Integration with River

The app migration runner (`go tool task migrate`) applies both:
1. App migrations from `migrations/sql/`
2. River job queue migrations

This ensures the database is fully ready for the application.
