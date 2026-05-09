package sqlite

import "embed"

// migrationsFS embeds local-storage's goose migrations into the
// binary. Called from db.go::GetDBByFile in place of the legacy
// `db.AutoMigrate(...)`. ADR-0018.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
