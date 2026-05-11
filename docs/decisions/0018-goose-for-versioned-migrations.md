# 0018 — `goose` for versioned schema migrations

**Status:** accepted
**Date:** 2026-05-09
**Tags:** foundation, sprint-3, pre-v1.0, data-safety

## Context

Every PowerLab service with persistent state currently uses GORM's
`AutoMigrate(&Model{})` at startup:

- `backend/core/pkg/sqlite/db.go:44`
- `backend/local-storage/pkg/sqlite/db.go:123`
- `backend/user-service/pkg/sqlite/db.go:44`
- `backend/message-bus/repository/repository_db.go:192,198`

`AutoMigrate` is GORM-internal — not a CasaOS dependency, so this
is purely an engineering robustness choice. The known failure modes
documented in #100:

| Failure mode | Behavior | User impact |
|---|---|---|
| Rename column | Both columns kept | Silent data loss |
| Add NOT NULL with rows | `ALTER TABLE` fails | Service won't start |
| Remove deprecated field | Refused for safety | Schema drift |
| User skips a version | No record applied | Intermittent bugs |
| Mid-migration failure | No transaction | Inconsistent DB |
| Need to roll back | No `down` step | Restore from backup |

Today, none of these have bitten us — schemas are small, change
rarely, SQLite is forgiving, single-user. They will bite when v1.0
ships changes between releases or when we add Postgres / clustering.

## Decision

Adopt **`pressly/goose`** (https://github.com/pressly/goose) as the
versioned-migration runner, wrapped in `backend/pkg/migrations`.

Each service that owns state ships an `embed.FS` of `.sql` migration
files (`0001_initial.sql`, `0002_add_email_index.sql`, etc.) and
calls `migrations.Up(ctx, db, embedded, "migrations")` at startup
in place of `db.AutoMigrate(...)`.

## Rationale

### Why a runner at all (vs. AutoMigrate)

Versioned migrations give us:

- **Auditable schema history** — every change is a reviewable PR.
- **Determinism** — same migrations run dev → CI → prod, in order.
- **Rollback** — `goose down` undoes the most recent step.
- **Atomicity** — each migration is wrapped in a transaction
  (where the DB supports it; SQLite does for non-DDL).
- **Schema drift detection** — `goose status` shows applied vs
  pending; CI can gate this.
- **Cross-version skip detection** — the `goose_db_version` table
  records what's been applied. Skipping a version is detectable.

### Why `goose` specifically

We considered three runners. The trade-offs:

#### `pressly/goose` ✅

- **`embed.FS` first-class.** Migrations ship inside the binary;
  no separate distribution.
- **Plain `.sql` files** with `-- +goose Up` / `-- +goose Down`
  delimiters. Reviewable in PRs, runnable manually with the
  `sqlite3` CLI when debugging.
- **Go API + CLI.** `goose.Up(db, fsys, dir)` for runtime;
  `goose -dir migrations sqlite3 ./db.sqlite up` for ops.
- Active maintenance; used at Hashicorp, Pressly, others.
- Single dependency; no transitive bloat.

#### `golang-migrate/migrate` ⛔

- Most popular, supports many DB drivers.
- **`embed.FS` requires extra setup** (`iofs.New`); not as ergonomic.
- File loader and DB driver are separate deps, more wiring.
- Fine choice for multi-DB shops; overkill for SQLite-only PowerLab.

#### `ariga.io/atlas` ⛔

- Declarative schema (you write the desired state, atlas computes
  the diff).
- Powerful but **conceptually heavier** for a small team — the
  diff engine introduces a class of "atlas guessed wrong" bugs we
  don't have today with imperative migrations.
- Designed for ORG-scale schema management; mismatched against
  PowerLab's "single SQLite per service, ~10 tables each" reality.

### Why a thin wrapper (`pkg/migrations`)

Two reasons:

1. **Testability.** `pkg/migrations` exposes a small surface
   (`Up`, `Down`, `Version`) that we test against in-memory SQLite.
   Goose's own API is broader; we wrap to the part we use.
2. **Switching cost insurance.** If goose loses maintenance or we
   need to support a backend it doesn't (unlikely but possible),
   the wrapper keeps the call sites identical and we swap the
   implementation in one place.

## Consequences

- **AutoMigrate calls retired** across all four services. Each
  service ships an `embed.FS` of `0001_initial.sql` capturing its
  current schema, plus the `goose_db_version` table.
- **Migration discipline** — every schema change lands as a new
  numbered .sql file. Renaming columns becomes a two-step
  process (add new, copy data, drop old) — explicitly recorded
  in the migration history.
- **CI gates** — a `migrations.Up(in-memory)` smoke per service
  in CI catches malformed migrations before they ship.
- **Pre-install check** — the existing `pre_install_checks` array
  in `manifest.json` gains a check that the user's current
  `goose_db_version` ≥ the new release's `min_db_version`. Skipping
  versions becomes loud instead of silent.
- **goose CLI as ops tool** — sysadmins can run `goose status`
  on a live DB to inspect migration state. Currently impossible.

## Alternatives considered (rejected)

- **Stick with AutoMigrate + careful PR review** — discipline-based
  fix for a structural problem. Will fail under load; humans miss
  things.
- **Custom in-house runner** — reinvents wheel poorly. Goose's CLI,
  transaction handling, and `embed.FS` integration are non-trivial
  to replicate.
- **Defer past v1.0** — v1.0 is when our backwards-compat contract
  starts. Shipping v1.0 with AutoMigrate makes the contract a lie
  the first time we change a schema.

## Implementation

This sprint:

1. `backend/pkg/migrations/migrations.go` — wrapper over goose.
2. `backend/pkg/migrations/migrations_test.go` — TDD coverage.
3. ADR-0018 (this).

Per service (one PR each):

4. `backend/<svc>/migrations/0001_initial.sql` — extract current
   schema. `goose status` on a live install confirms the
   migration is a no-op (the schema already exists at v1).
5. Replace `db.AutoMigrate(...)` call with `migrations.Up(...)`.
6. Add `migrations.Up` smoke to that service's CI test job.

## Reference

- Issue: #100
- goose repo: https://github.com/pressly/goose
- ADR-0025 (strangler — pkg/* foundation)
- ADR-0017 (changie — companion DX investment)
