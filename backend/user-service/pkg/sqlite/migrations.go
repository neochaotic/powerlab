package sqlite

import "embed"

// migrationsFS embeds the user-service goose migrations into the
// service binary. See ADR-0018. The accompanying call site is in
// db.go::GetDb, which runs migrations.Up against this FS at startup.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
