package migrations

import "embed"

// Files contains all SQL migration files.
//
//go:embed sql/*.sql
var Files embed.FS
