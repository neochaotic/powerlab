# 0036 — File-system migrations: per-service marker file pattern

- **Status:** proposed
- **Date:** 2026-05-15
- **Trigger:** knowledge extracted from `backend/common/utils/version` before it is deleted in Sprint 19 PR 2. The original CasaOS code carried a `GlobalMigrationStatus` type that wrote a per-service marker file at `/var/lib/casaos/migration/<service>` with the last-migrated version. PowerLab never adopted it, but the *pattern* is useful and there is no other place where it is captured.

## Context

PowerLab has one well-understood migration path:

- **Database schema migrations** — managed by goose (ADR-0018). Per-service `migrations/*.sql` directory, applied transactionally on service start. Battle-tested.

We have **no equivalent pattern** for migrations that are NOT a database schema change:

- Renaming a directory under `/var/lib/powerlab/` because we moved data out of `/DATA/PowerLabAppData/`.
- Rewriting a config file because we changed the key name (`SoftwareName` → `AppName`).
- Replacing a Docker label namespace (`io.casaos.app` → `io.powerlab.app`), once the dual-write window ends (#201).
- Re-encoding the JSONL audit ring buffer when its schema changes.
- Removing a marker file that's now obsolete.

Today these are handled ad-hoc: a one-off function in service startup, or a release-note instruction the user is expected to follow. Both fail under reinstall + upgrade-skip scenarios. The unreplaced `migration-tool` from CasaOS (Sprint 8 PR Q deleted it) used to ship as a separate binary, which is the wrong unit of distribution — the operator never knew it existed.

## The pattern (extracted from `common/utils/version`)

```go
type GlobalMigrationStatus struct {
    ServiceName         string
    LastMigratedVersion string
}

// Marker file location (one per service):
//   /var/lib/powerlab/migrations/<service>
// Contents: a single line, the last-migrated version with "v" prefix.

func (m *GlobalMigrationStatus) Done(version string) error { ... }
func GetGlobalMigrationStatus(serviceName string) (*GlobalMigrationStatus, error) { ... }
```

On service start, the migration runner reads the marker. For each registered file-system migration, if its version is newer than the marker, it runs; on success the marker advances. Idempotent: re-running a service that already migrated reads the same marker and skips.

The CasaOS code wrote to `/var/lib/casaos/migration/` — the PowerLab path would be `/var/lib/powerlab/migrations/` (already a sibling of `audit.jsonl`, `apps/`, etc.).

## Decision

**Propose** (not yet implemented) — adopt the marker-file pattern as the standard PowerLab approach for **non-DB** migrations, when we have one to land. Until that first concrete migration exists, this ADR captures the shape so we don't reinvent it.

When the first file-system migration arrives (likely #201 — drop legacy `casaos` label dual-writes after the v0.6.x window — or the JSONL schema bump), the implementation lives in a new `backend/common/utils/fsmigrations/` package:

```go
package fsmigrations

type Migration struct {
    Version     string                       // semver, e.g. "0.7.0"
    Description string
    Apply       func(ctx context.Context) error
}

func RunForService(serviceName string, migrations []Migration) error
```

The marker file format remains the single-line `v<semver>\n` from CasaOS — operator-greppable, no parser needed.

## Why not alternatives

- **goose for everything** — goose is a SQL migration runner. Adding a `goose.Go` migration that exec()s a bash script is a hack; the SQL contract leaks into non-SQL work.
- **One-shot functions in service main** — what we do today. No idempotency contract, no audit trail of which migrations ran on which host, easy to skip on upgrade-skip-path.
- **A separate `migration-tool` binary** — CasaOS's choice. Operator never knew it existed; documented in shipped install.sh; nobody ran it. Wrong unit of distribution.
- **DB row in user-service** — coupling state-of-the-host to the user table is wrong; the marker file lives next to the data it migrated.

## Consequences

- Until a concrete migration lands, this ADR is documentation, not code.
- When the first migration lands, it can reference this ADR by number so future readers find the design instead of reinventing.
- The marker-file location `/var/lib/powerlab/migrations/` is reserved.

## Knowledge-extraction provenance

This ADR was lifted from `backend/common/utils/version/migration.go` immediately before Sprint 19 PR 2 deletes the package. The deleted code:

- hardcoded `/var/lib/casaos/migration` (no PowerLab path knowledge)
- offered no `Migration` abstraction (only the marker reader/writer)
- had no concept of an ordered migration list
- was never wired into any service

The pattern survives; the implementation does not. Sprint 19 plan: `docs/audits/sprint-19-dead-code-removal.md`.

## Related

- ADR-0018 — goose for versioned (SQL) migrations
- ADR-0035 — audit storage JSONL (one of the schemas that may someday need this)
- #201 — drop legacy unnamespaced container label dual-writes (first concrete candidate)
