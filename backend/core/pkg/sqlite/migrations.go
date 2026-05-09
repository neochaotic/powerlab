package sqlite

import "embed"

// migrationsFS embeds core's goose migrations into the binary. The
// accompanying call site is in db.go::GetDb, which runs
// migrations.Up against this FS at startup. ADR-0018.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
