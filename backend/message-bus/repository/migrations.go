package repository

import "embed"

// migrationsEventsFS embeds the events-DB goose migrations into the
// message-bus binary. The events DB is in-memory by default; even
// so, every service start runs Up to recreate the schema deterministically.
//
//go:embed migrations_events/*.sql
var migrationsEventsFS embed.FS

// migrationsPersistFS embeds the persist-DB goose migrations into the
// message-bus binary. The persist DB lives on disk and survives
// restarts — this is where YSK cards + Settings rows persist.
//
//go:embed migrations_persist/*.sql
var migrationsPersistFS embed.FS
