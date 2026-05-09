# CasaOS dependency map

**Latest update:** 2026-05-09 (Sprint 3 closeout)
**Original audit date:** 2026-05-08 (Sprint 1, issue #62)

This is a living document. Per ADR-0019, structural audits get a
`Update — YYYY-MM-DD` section appended at the top whenever the
audited surface materially changes. The original Sprint 1 baseline
is preserved below as historical record.

---

# Update — 2026-05-09 (Sprint 3 closeout)

## What changed since the Sprint 1 baseline

Two complete sprints (#67 phases) plus a Sprint 3 rebrand wave have
landed since the original audit. The CasaOS surface is materially
smaller — both in LOC and in the categories of dependency.

### Sprint 1 — `gateway` + `message-bus` killed (foundation)

- `backend/pkg/foundation`, `pkg/lifecycle`, `pkg/logging`,
  `pkg/tracing`, `pkg/errors`, `pkg/security`, `pkg/migrations`
  built. ADRs 0011-0018 lock in the contracts.
- `gateway` and `message-bus` go through their kill series and
  switch to `pkg/*` foundation.

### Sprint 2 — `local-storage` + `user-service` killed

- Both services migrate to `pkg/migrations` (goose, ADR-0018) for
  versioned schema. GORM `AutoMigrate` retired (#100, #136).
- Both adopt `pkg/foundation` middleware (recover + tracing).
- `pkg/logging` adopted in `main.go` of each, but not yet in every
  package's call sites — that's Sprint 3.

### Sprint 3 — `core` rebrand wave + cloud-drive removal + logger tail

10 PRs landed in a single push (PRs #139–#148 + the v0.5.3 release
that preceded them). Net: ~3500 LOC removed, zero LOC of CasaOS
infrastructure dependency added.

| Surface                                    | PR(s)            | Delta                                            |
|--------------------------------------------|------------------|--------------------------------------------------|
| Cloud drive backends in `local-storage`    | #139             | -1500 LOC. `drivers/{dropbox,google_drive,base}` + `route/v1/{cloud,recover,driver}.go` + entire `internal/` deleted. Kills the `cloudoauth.files.casaos.app` OAuth proxy dependency. |
| `/etc/casaos` → `/etc/powerlab` paths      | #140             | 5 services migrated. Sysroot dirs renamed, `//go:embed` updated, hardcoded const literals → `constants.DefaultConfigPath`-derived. Also fixed a real prod bug: install.sh shipped `casaos.conf.sample` → `/etc/powerlab/casaos.conf` but systemd starts core with `-c /etc/powerlab/core.conf`, dropping every shipped default into nothing. Sample renamed `core.conf.sample`. |
| `casaos:* → powerlab:*` message-bus topics | #141             | `EventTypes` registry + 4 publish call-sites in core wrapped in named consts (`EventSystemUtilization`, `EventFileOperate`). |
| `PersistentTypeCasaOS → PersistentTypePowerLab` (wire format) | #142             | Disk persistence-type discriminator returned via `/v1/storage` PersistedIn field. Pre-v1.0 wire-format change. |
| Cloud drive backends in `core`             | #143             | -1862 LOC (largest single deletion). Mirrors #139. `drivers/{dropbox,google_drive,onedrive,base}` + `route/v1/{cloud,recover,driver}.go` + `service/storage.go` + `pkg/utils/httper/drive.go` deleted. `.goreleaser.yaml` Dropbox/Google/OneDrive OAuth ldflags dropped — release builds no longer need those env vars. |
| user-service `SERVICENAME` + `zimaos:` topic | #144             | `"CasaOS-UserService"` → `"PowerLab-UserService"`. `"zimaos:user:save_config"` → `"powerlab:user:save_config"`. |
| local-storage logger tail (#104)           | #145             | 165 legacy `logger.X(msg, zap.X(...))` call sites migrated to `_log.X(ctx, msg, slog.X(...))` across 14 source files. 6 packages got `_log` + `SetLogger`. **Closes #104.** |
| `service/storage.go` + `httper/drive.go` orphans | #146             | -374 LOC. Symmetric cleanup to what #143 did for core. |
| Embedded sysroot `.conf.sample` files      | #147             | 5 sample files: `/var/run/casaos` → `/var/run/powerlab`, `/var/log/casaos` → `/var/log/powerlab`, `/var/lib/casaos` → `/var/lib/powerlab`, `/usr/share/casaos` → `/usr/share/powerlab`. core.conf.sample also drops dead CasaOS upstream endpoints (`api.casaos.io/casaos-api`, `socket.casaos.io`) — these would have triggered silent network requests on first boot. |
| Hardcoded `/var/lib/casaos/...` runtime paths | #148             | 6 files with hardcoded data/log/share paths that survived #140 (different constants base). Real production bugs: `dockerRootDirFilePath`, log archiver, modules dir, ShellPath defaults, user-facing path messages. All now derived from `constants.Default{Data,Log,Constant,File}Path`. |

### Issue closeouts

- **#101** (local-storage structural CasaOS deps) — substantially
  closed by #139 + #140 + #141 + #142 + #146 + #147 + #148. Only
  open subitem: the Go module path rename
  (`github.com/IceWhaleTech/CasaOS-LocalStorage` →
  PowerLab-owned), tracked separately.
- **#104** (local-storage logger migration tail) — fully closed by
  #145.
- **#106** (user-service structural CasaOS deps) — substantially
  closed by #140 + #144. Only open subitem: Go module path rename.

### Updated module-path table (current main)

| Service          | `go.mod` declares                                 | Kill status                                          |
|------------------|----------------------------------------------------|------------------------------------------------------|
| `gateway`        | `github.com/IceWhaleTech/CasaOS-Gateway`           | Sprint 1 done; module rename pending                 |
| `message-bus`    | `github.com/IceWhaleTech/CasaOS-MessageBus`        | Sprint 1 done; replace ../common added by #140       |
| `core`           | `github.com/IceWhaleTech/CasaOS`                   | Sprint 3 rebrand wave done; module rename pending    |
| `user-service`   | `github.com/IceWhaleTech/CasaOS-UserService`       | Sprint 2 + #144 done; module rename pending          |
| `local-storage`  | `github.com/IceWhaleTech/CasaOS-LocalStorage`      | Sprint 2 + 7 PRs of Sprint 3 done; module rename pending |
| `app-management` | `github.com/IceWhaleTech/CasaOS-AppManagement`     | Sprint 4 (planned). Currently has #148 path fixes only. |
| `common`         | `github.com/IceWhaleTech/CasaOS-Common` (shared)   | Local fork at `backend/common/` used by all 6 (per ADR-0011) |
| `pkg`            | `github.com/neochaotic/powerlab/backend/pkg`       | PowerLab-owned, ~600 LOC, foundation contracts |

The `go.mod` paths are unchanged — that's the **single largest
remaining surface**. Each rename is mechanical (every import line
across the service updated, every `replace` directive in dependent
go.mods updated) but sweeping. Tracked as the next major rebrand
PR after this Sprint 3 closeout.

### What's left in the CasaOS surface

In rough priority order:

1. **Go module paths** (`github.com/IceWhaleTech/CasaOS-*` → PowerLab-owned).
   6 services × every import line + every `replace` in dependent
   go.mods. Mechanical but sweeping. **Highest remaining priority.**
2. **app-management Sprint 4 work** (#85). The largest service; not
   yet rebranded. Includes Docker labels (`config.Labels["casaos"]`)
   + AppData isolation. The container.go `casaos` label discriminator
   is part of the CasaOS-app-store wire format — needs careful
   migration so existing installs don't lose track of their apps.
3. **App store URL** — `cdn.jsdelivr.net/.../IceWhaleTech/CasaOS-AppStore`
   in the default sample (#147 kept this intentionally — it's an
   external data source). Migrating off requires running our own
   app store mirror.
4. **Migration-tool surface** (`backend/<svc>/cmd/migration-tool/main.go`,
   `backend/common/utils/version/migration.go`'s
   `GlobalMigrationStatusDirPath = "/var/lib/casaos/migration"`).
   Held back by every Sprint 3 PR — these run during legacy CasaOS
   → PowerLab migration only and intentionally read from `/etc/casaos/`
   etc.
5. **License headers / `@Website` comments** — every CasaOS-derived
   file carries an Apache 2.0 header. Per the original audit's
   guidance these are preserved-where-code-is-preserved, removed
   where files are rewritten. Currently mid-state; tracked per kill
   PR.
6. **Inline `casaos` literal strings** — Docker labels, container
   filter tokens, tmpdir prefixes (`casaos-compose-app-*`),
   appstore data refs. Mostly app-management Sprint 4 territory.

### What's NOT debt despite the name

- `backend/common/utils/version/LegacyCasaOSConfigFilePath` — explicit
  legacy CasaOS interop for co-resident hosts. Intentional.
- `/etc/casaos` in `constants/paths.go::devProductionMarkers` — used
  by binaries to detect a CasaOS host and avoid the dev-sandbox path
  rewrite. Intentional.
- `Powered by CasaOS` link in the SvelteKit settings page — correct
  AGPL attribution to the upstream project. Intentional.
- `appstore = .../big-bear-casaos/...` in app-management's sample —
  community-maintained app catalog whose name predates PowerLab.
  Intentional (data source).
- `backend/core/build/sysroot/usr/share/casaos/...` shell scripts —
  legacy CasaOS-era helpers shipped in the sysroot but not currently
  installed by `package-linux.sh`. Effectively dead but kept for
  reference until app-management Sprint 4 confirms nothing else
  needs them.

### LOC delta vs Sprint 1 baseline

Sprint 1 baseline counted ~54,000 LOC across the 7 services
(excluding `pkg/` and codegen/external).

Today, after Sprint 2 + Sprint 3 rebrand wave:
- ~-3,500 LOC removed (cloud drives in local-storage + core,
  orphan storage.go + httper, dead OAuth code).
- ~+800 LOC added (pkg/migrations adoption, foundation wiring,
  per-package logger.go files in local-storage).
- Net: ~-2,700 LOC. The remaining surface is ~51,000 LOC.

The biggest remaining surface is `app-management` (~13,000 LOC)
which has not yet had a kill series — Sprint 4 territory.

### Reference (Sprint 3 specific)

- ADR-0019: how this audit gets refreshed (the convention this
  update follows).
- v0.5.3 release: the version that shipped before the rebrand wave.
- 10 closeout PRs: #139, #140, #141, #142, #143, #144, #145, #146,
  #147, #148.

---

# Sprint 1 baseline (preserved as historical record)

**Date:** 2026-05-08
**Sprint:** 1 (CasaOS strip — issue #62)
**Status:** complete

## Headline

**All seven existing backend modules declare CasaOS module paths**.
The dependency on the upstream `IceWhaleTech` org is not a leaf-level
detail — it is the **identity** of every shared module. Stripping
CasaOS therefore means renaming every `go.mod`, regenerating every
generated file that references the old path, and updating every
import across the tree. The new `backend/pkg/` module (PowerLab-owned)
is the only one that is not CasaOS-shaped.

## Module paths today

| Service | `go.mod` declares | LOC | Files |
|---|---|---:|---:|
| `gateway` | `github.com/IceWhaleTech/CasaOS-Gateway` | 4,450 | 25 |
| `message-bus` | `github.com/IceWhaleTech/CasaOS-MessageBus` | 3,376 | 47 |
| `core` | `github.com/IceWhaleTech/CasaOS` | 14,554 | 159 |
| `user-service` | `github.com/IceWhaleTech/CasaOS-UserService` | 2,762 | 29 |
| `local-storage` | `github.com/IceWhaleTech/CasaOS-LocalStorage` | 10,596 | 98 |
| `app-management` | `github.com/IceWhaleTech/CasaOS-AppManagement` | 13,214 | 87 |
| `common` | `github.com/IceWhaleTech/CasaOS-Common` (shared by all 6) | 5,207 | 45 |
| **`pkg`** (new) | `github.com/neochaotic/powerlab/backend/pkg` | ~600 | 12 |

Total backend (excluding `pkg/` and codegen/external): ~54,000 LOC.

## What "kill" means per service

For each service, the kill PR (Sprint 1-4) does three things:

1. **Rename `go.mod`** → `github.com/neochaotic/powerlab/backend/<svc>`.
2. **Replace internal imports** of the old path with the new one
   inside that service.
3. **Migrate one shared module dependency at a time** away from
   `backend/common/` (CasaOS-Common) onto `backend/pkg/`
   (PowerLab-owned).

When the last service is killed, `backend/common/` is unreferenced
and is deleted in the same PR (per ADR-0011 strangler pattern).

## Sprint order — recap from #67

| Sprint | Service(s) killed | Notes |
|---|---|---|
| 1 | `gateway`, `message-bus` | Smallest, foundation-dependent |
| 2 | `local-storage`, `user-service` | Filesystem + auth |
| 3 | `core`, appstore | System info, drivers, hardware |
| 4 | `app-management` | The largest; compose orchestrator |

## Risk surfaces beyond Go imports

- **Generated code (`codegen/`)** — most services have OpenAPI specs
  that refer to CasaOS schemas. The kill PR for a service must
  regenerate from a PowerLab-owned spec or strip the upstream
  schema reference.
- **Hardcoded URLs** — `casaos.oss-cn-shanghai.aliyuncs.com` appears
  in `appstore` data fetched by `app-management`. Migrating off this
  requires running our own appstore mirror (Sprint 3).
- **Database migration tags** — `app-management` and others embed
  `casaos_*` table names in migrations. Renaming requires a data
  migration step on existing installs (planned for Sprint 4 with
  the app-management kill).
- **License posture** — every CasaOS-derived file carries an Apache
  2.0 license header. The kill PRs preserve attribution where the
  code is preserved-but-renamed, and remove headers when the file is
  rewritten from scratch. Tracked per kill PR.

## Reference

- Strangler pattern rationale: ADR-0011
- Issue: #62 (this audit)
- Roadmap: #67
- Dead-code findings: see `dead-code.md` (companion document)
