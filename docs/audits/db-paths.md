# Database & runtime file paths — per-service audit

**Date:** 2026-05-10
**Status:** authoritative (single source of truth)
**Refresh trigger** (per ADR-0019): any service that adds, removes,
or renames a persistent file. Bump the table + add a regression test
that asserts the new path matches the helper in `pkg/paths/db.go`.

This audit exists because of issue #179 — the v0.5.4 incident
debug surfaced that **every PowerLab backend service uses a
different convention** for where it stores its SQLite database.
Convention drift inherited from CasaOS; never deliberately chosen.

The drift caused two real failures during the v0.5.4 / v0.5.6 / v0.5.7
debug sequence:

1. The hot-fix copied `user.db` to `/var/lib/powerlab/db/`, but
   user-service actually uses `/var/lib/powerlab/user.db` (no `/db/`
   subdir). 30 minutes of debug spent looking at the wrong file.
2. `core` still has `DBPath = /var/lib/casaos` in `/etc/powerlab/core.conf`
   on the affected host. install.sh's "skip-if-exists" preserved the
   pre-rebrand config. Result: core reads/writes `/var/lib/casaos/db/casaOS.db`
   while PowerLab thinks it's in `/var/lib/powerlab/db/casaOS.db`.
   Live split-brain.

## Canonical paths (target convention)

The going-forward convention, encoded in `backend/common/utils/paths/db.go`:

| Service | Canonical path | Description |
|---|---|---|
| user-service | `<DataPath>/user.db` | accounts + JWT signing keypair |
| core | `<DataPath>/core.db` | gateway port, system settings, samba |
| local-storage | `<DataPath>/local-storage.db` | merge points, mount metadata |
| message-bus (events) | in-memory (`file:events?mode=memory&cache=shared`) | ephemeral pub/sub |
| message-bus (persist) | `<DataPath>/message-bus.db` | persistent topic registrations |
| app-management | (none) | compose state lives in Docker daemon |

Where `<DataPath>` = `constants.DefaultDataPath`:

- Linux: `/var/lib/powerlab`
- macOS: `/opt/powerlab/lib`
- dev sandbox: `<repo>/backend/data`

**Legacy paths services may still use on existing installs**:

| Service | Legacy path | When |
|---|---|---|
| user-service | `<DataPath>/user.db` (already canonical — no legacy) | — |
| core | `<DataPath>/db/casaOS.db` | pre-rebrand naming (still used in code as of v0.5.7) |
| core | `/var/lib/casaos/db/casaOS.db` | when `DBPath` config still says `/var/lib/casaos` (skip-if-exists conf preserve) |
| local-storage | `<DataPath>/db/local-storage.db` | hot-fix copy destination (stale duplicate) |
| message-bus persist | `<DataPath>/db/message-bus.db` | current production code uses this; canonical is the same path without `/db/` |

The tension above is real: the **canonical** column is the going-forward
target, but several services have not yet been migrated to it. The
`pkg/paths/db.go` helpers expose BOTH `CanonicalUserServiceDB()` and
`LegacyUserServiceDB()` so split-brain detection can compare them.

## Convention rationale

### Why `<DataPath>/<svc>.db` (no `/db/` subdir)

The `/db/` subdir convention CasaOS used was inconsistent — only some
services followed it. Removing it:

- Halves the path complexity (one segment shorter)
- Eliminates the 2-files-with-same-name case that bit us in #179
  (`<DataPath>/user.db` vs `<DataPath>/db/user.db`)
- Aligns with how `last-upgrade.json`, `version`, and other top-level
  PowerLab artifacts already sit at `<DataPath>` root

### Why service-name-based filenames (`core.db` not `casaOS.db`)

Pre-Sprint 3 names like `casaOS.db` violated the "binary self-describes"
principle (#101 rebrand). Going-forward names match the systemd unit
name + Go module name + cmd basename — a future maintainer reading
ls output learns which service owns the file.

## Split-brain detection (new in #179 fix)

Per the same PR, every service runs a boot-time check via
`paths.AssertNoSplitBrain(ctx, log, canonical, legacy...)`:

- If only one path exists → use it (no split brain)
- If neither exists → fresh install (use canonical)
- If BOTH exist → log error + REFUSE TO START + print recovery
  instructions

The recovery instructions point operators to:

1. Compare the two files (`sqlite3 file.db .schema` + row counts)
2. Pick the authoritative one (usually the most-recently-modified)
3. Move the loser to `<file>.bak.<timestamp>`
4. Restart the service

This is L3 of the 5-layer prevention strategy (see `docs/audits/sprint-3-retrospective.md`):

| Layer | What | Where |
|---|---|---|
| L1 | Helper centralized | `backend/common/utils/paths/db.go` |
| L2 | Migration atomic + delete-source | `scripts/migrate-casaos-data.sh` |
| L3 | Pre-flight refuse-to-start | each service's `init()` calls `paths.AssertNoSplitBrain` |
| L4 | Integration test catches | `scripts/migrate-casaos-data_test.sh` opens via service GetDb |
| L5 | install.sh audit warning | install.sh emits "WARN: split brain detected" |

## Migration plan from current state

1. **Land the helpers + detection** (this PR) — no behavior change for
   existing installs that have only one path; new installs use
   canonical paths from day one.
2. **Migrate core's DBPath conf** — separate PR. install.sh detects
   `DBPath = /var/lib/casaos` and rewrites to `/var/lib/powerlab` +
   moves the DB file atomically.
3. **Rename `casaOS.db` → `core.db`** — separate PR. Service detects
   legacy filename, copies to canonical name, deletes legacy after
   one release window of dual-read.
4. **Drop `/db/` subdir for message-bus** — separate PR (likely the
   smallest of the four).

Each migration step ships independently with its own regression test.

## Open issues this audit unblocks

- **#179** — install.sh data migration copies to wrong path (this audit)
- **#170** — cmd/migration-tool/ overlap (the migration helpers from
  this audit could be the canonical migration entry point)
- **#150** — backend integration coverage (the integration test that
  opens-via-service is L4 of this strategy and an example for #150)

## Reference

- ADR-0019 — tech-debt tracking pattern (this audit lives here per that)
- ADR-0020 — JWT keypair persistence (the bug that triggered the wider
  audit)
- Sprint 3 retrospective — `docs/audits/sprint-3-retrospective.md`
- Sprint 4 prep — `docs/audits/sprint-4-app-management-prep.md`
