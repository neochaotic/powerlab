# Changelog

All notable user-facing changes to PowerLab. We follow
[Semantic Versioning](https://semver.org/) — `vMAJOR.MINOR.PATCH`. While
PowerLab is in `v0.x`, breaking changes can land in MINOR bumps; from
`v1.0` onward we commit to backwards compatibility within MAJOR.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## How entries land here

Each PR adds a tiny YAML fragment under `.changes/unreleased/<id>.yaml`.
At release time, `changie batch <version>` aggregates the fragments into
a new section below this header. See `CONTRIBUTING.md` for the workflow.

## [v0.5.6] — 2026-05-09
### Changed
- Sprint 4 PR5 — rename `service.ErrComposeExtensionNameXCasaOSNotFound`
→ `service.ErrComposeExtensionNotFound`. Per the audit's PR
breakdown (`docs/audits/sprint-4-app-management-prep.md`).

The `XCasaOS` specificity in the original name was misleading
after PR #141 landed the extension-key priority chain
(`service/extension.go::extensionPriority` accepts `x-powerlab`,
`x-web`, OR `x-casaos`). The error is raised when NONE of the
three keys are present — its name should describe that
generically.

6 sites mechanically renamed:
  - `service/errs.go`            (declaration)
  - `service/compose_app.go`     (3 returns / comparisons)
  - `route/v2/compose_app.go`    (1 comparison)
  - `cmd/validator/pkg/validate.go` (1 return)

No wire format. No UI consumer. Pure-internal rename.

### Removed
- Sprint 4 PR3 — drops dead `MyAppList` handler + renames Go vars
in active sites. Per the audit's PR breakdown
(`docs/audits/sprint-4-app-management-prep.md`), this is the
third smallest-first chunk after PR1 (cosmetic literals) and
PR2 (CasaOSGlobalVariables rename).

Removed:

  backend/app-management/route/v1/docker.go::MyAppList
      Dead code — its route registration in route/v1.go was
      commented out for an unknown duration. Was the only
      consumer of the legacy `casaos_apps` JSON wire-format
      key. Active app-list flow lives in
      route/v2/internal_web.go's WebAppGridItem* — untouched.

  backend/app-management/route/v1.go (the commented-out reg)
      Removed the `// v1ContainerGroup.GET("", v1.MyAppList)`
      line for clarity.

Renamed (Go vars only — no wire format change):

  casaOSApps → managedApps   (4 sites in service/container.go,
                              1 site in route/v2/internal_web.go)
  casaOSApp  → managedApp    (1 site)

No UI consumer (verified by grep), no remaining wire-format
references to `casaos_apps`. PR4 (Docker label dual-write — the
big one) and PR5 (ErrComposeExtensionNameXCasaOSNotFound rename)
remain in the Sprint 4 backlog.

### Fixed
- user-service `EventListen` no longer crashes its goroutine when
message-bus disconnects or sends a malformed event payload (#160).

The original code had three nil-deref paths that combined to
panic on every message-bus restart cycle:

  1. `ws.Read` err → no continue, fell through to unmarshal of
     zero bytes
  2. `json.Unmarshal` err → no continue, fell through to
     `*event.Uuid`
  3. `event.Uuid == nil` even when unmarshal succeeded (no uuid
     in payload) → panic at the assignment

SafeGo (pkg/lifecycle) recovered the panic so the process kept
running, but the goroutine died on every cycle. Visible in
production logs every time the user clicked the Update button
(which restarts message-bus mid-flight) — see #160 for context.

Fix extracts payload parsing into `parseEventPayload()` returning
`(*EventModel, error)` instead of mutating shared state inline.
Each error path returns a useful message; caller skips that
message and stays connected.

Regression test at `event_listen_test.go` — 6 cases:
  - empty payload returns error (the v0.5.4 disconnect shape)
  - malformed JSON returns error
  - missing uuid returns error (the actual line:77 crash shape)
  - null uuid returns error
  - valid payload returns full model
  - fuzz: 9 crash-prone inputs, none panic

Bonus fix: `ws.Read` error now `break`s the inner loop (was
silent fall-through) so the outer reconnect loop fires
immediately instead of cycling on dead websocket reads.

Closes #160.

- install.sh now prunes old upgrade snapshots after each successful
install. Before this fix, every upgrade left ~100MB of binaries +
UI bundle behind in `/var/lib/powerlab/backups/pre-upgrade-<ts>/`
→ disk filled up over time. v0.5.4 prod incident retrospective
surfaced this when the user accumulated 4 snapshots in a single
debugging session.

Default: keep the 3 newest snapshots. Override with
`POWERLAB_BACKUP_KEEP=N` env var:
  - `POWERLAB_BACKUP_KEEP=5` → paranoid retention
  - `POWERLAB_BACKUP_KEEP=0` → keep ALL (forensic mode for
    post-mortems)

Only directories matching `pre-upgrade-*` are touched. Manual
exports, README files, or any other backups dir contents are
left alone.

Regression test at `scripts/check-backup-retention_test.sh` —
14 assertions across 5 scenarios (5→3 pruning order, no-op when
under threshold, empty dir, KEEP=0 disables, non-snapshot dirs
preserved).

### Internal
- Release v0.5.6 — v0.5.4 incident retrospective. Long tail of
bugs surfaced during the user's prod upgrade debug, plus
defenses so the class doesn't repeat.

Bug fixes:
  - #160: user-service event-listener no longer crashes on
    message-bus disconnect or malformed payload (3 nil-deref
    paths fixed). 6 regression tests including 9-input fuzz.

Operational improvements:
  - install.sh prunes old upgrade snapshots (keep last 3 by
    default, POWERLAB_BACKUP_KEEP env var override). 14
    regression tests.

Sprint 4 cosmetic continuations:
  - PR3 (#85): drop dead MyAppList handler + casaos_apps JSON
    wire-format key + Go-side casaOSApps → managedApps rename.
  - PR5 (#85): ErrComposeExtensionNameXCasaOSNotFound →
    ErrComposeExtensionNotFound (the X-CasaOS specificity was
    misleading after the x-powerlab/x-web/x-casaos extension
    chain landed).

Sprint 4 PR4 (Docker label dual-write — the big one) remains
in backlog: needs design + integration testing for in-place
container migration.

No end-user behavior change beyond the bug fixes.



## [v0.5.5] — 2026-05-09
### Added
- Sprint 4 prep audit: `docs/audits/sprint-4-app-management-prep.md`.

Read-only deep-dive on the CasaOS surface still inside
app-management (the largest service, ~13,300 LOC, the only one
without a dedicated kill series). Maps every legacy item into 5
risk-categorized buckets:

1. Compose extension `x-casaos` (ecosystem-coupled, dual-read
   already in place via `service/extension.go::extensionPriority`)
2. Docker label `"casaos"` discriminator (4 sites; needs
   dual-write migration before legacy label can be dropped)
3. CasaOS-team URLs (intentional data sources, kept)
4. Code-internal literals (~10 files; mostly mechanical, pre-v1.0
   wire-format renames allowed)
5. License headers / @FilePath markers (intentional attribution,
   kept)

Includes a suggested Sprint 4 PR breakdown ordered smallest-to-
largest. Companion deep-dive to `docs/audits/casaos-dependencies.md`.

Per ADR-0019: read-only audit, refreshed when Sprint 4 lands or
when the compose-extension contract changes.

### Changed
- Two cosmetic CasaOS literals in app-management renamed to
PowerLab equivalents. First of 5 PRs proposed in the Sprint 4
prep audit (`docs/audits/sprint-4-app-management-prep.md`),
ordered smallest-first.

Renames:

  .casaos-appstore       →  .powerlab-appstore
      Marker file written into each app store dir to identify
      store provenance from disk. Regenerated on every store
      sync — no migration needed.

  casaos-compose-app-*   →  powerlab-compose-app-*
      os.MkdirTemp prefix for the working directory used while
      parsing a docker-compose.yml. Temporary by definition,
      cleaned up in the same function via defer.

Both are pure-internal: no UI consumer, no wire format, no
on-disk state worth migrating. Cosmetic surface cleanup so
logs / strace / process listings stop advertising upstream
CasaOS branding from inside a PowerLab process.

- Rename `CasaOSGlobalVariables` struct → `AppLifecycleFlags`.
Sprint 4 PR2 from the prep audit
(`docs/audits/sprint-4-app-management-prep.md`).

Same struct, same single field (`AppChange bool`), better name.
Used as process-global state to invalidate the app-list cache
when an install/uninstall handler in `route/v1/docker.go` flips
the flag, then `service/container.go::GetContainerAppList`
consumes it.

6 sites renamed across 4 files:
  - `model/sys_common.go`     (the struct itself + new godoc)
  - `pkg/config/init.go`      (the package-global init)
  - `route/v1/docker.go`      (2 setters in install + uninstall)
  - `service/container.go`    (1 reader + 1 reset)

No wire format. No UI consumer. No on-disk state. Pure-internal
rename so the type name describes what it does instead of where
it came from.

### Fixed
- install.sh now migrates `/var/lib/casaos/*` → `/var/lib/powerlab/*`
on upgrade. Closes the v0.5.4 prod incident (issue #158) where
hosts upgrading from v0.5.x lost access to user accounts because
PR #140 flipped data paths but install.sh didn't migrate the data.

Symptom on the affected host: every API returned 401 Unauthorized,
login returned 400 Bad Request — user-service was reading an empty
`/var/lib/powerlab/db/user.db` while the actual user data sat in
`/var/lib/casaos/db/user.db`. Hot-fixed manually by copying the DBs.

Fix: extracted the migration logic into a standalone testable
script, `scripts/migrate-casaos-data.sh`, which install.sh sources
and invokes after stopping services + before starting them. The
script:

  - Copies subdirs `db apps appstore conf 1` from
    `/var/lib/casaos/<sub>` to `/var/lib/powerlab/<sub>` only
    when destination doesn't exist (never overwrites live data).
  - Copies known DB files individually (db/user.db,
    db/message-bus.db, db/casaOS.db, db/local-storage.db,
    db/local-storage.json) for the case where the destination
    dir EXISTS but specific files are missing — the actual v0.5.4
    mishap shape (message-bus had created /var/lib/powerlab/db/
    with just message-bus.db, no user.db).
  - Idempotent — safe to run on every upgrade; no-op when the
    destination is fully populated.
  - Source preserved (`/var/lib/casaos/*` not deleted) — leaves
    a manual rollback path.

Test coverage at `scripts/migrate-casaos-data_test.sh`:
  - Test 1: v0.5.4 mishap scenario (user.db missing) → migrated;
    message-bus.db NOT overwritten.
  - Test 2: fresh install (no /var/lib/casaos) → no-op.
  - Test 3: full upgrade (no /var/lib/powerlab) → all subdirs copied.
  - Test 4: idempotent re-run → user mutations preserved.
  - Test 5: source preservation → casaos files untouched.

10 assertions across 5 scenarios. PREFIX env var lets the test
point at a sandbox dir so no real /var/lib paths are touched.

Closes #158.

- Release builds now correctly stamp `commit`, `date`, and PowerLab
version into the binaries. Closes the v0.5.4 mishap where the
in-UI updater showed `current="dev"` and prompted "Update
available" forever even when the user was on the latest release
(issue #159).

Two bugs in one ldflag string in `scripts/package-linux.sh`:

  1. `-X main.version=$VERSION` — wrong variable name. Each
     service's `main.go` declares `commit` and `date`, never
     `version`. Go's `-X` is fail-soft: if the target var doesn't
     exist, the build still succeeds, the assignment is silently
     dropped, and the binary keeps the default `"private build"`.

  2. `-X github.com/IceWhaleTech/CasaOS/common.POWERLAB_VERSION=...`
     — dead path after PR #151 renamed every Go module to
     `github.com/neochaotic/powerlab/backend/*`. Same fail-soft
     behavior: silently no-op, version constant stays "dev".

Fix sets the four ldflags that actually exist in the codebase:

  -X main.commit=<git short SHA>                     (every service)
  -X main.date=<UTC ISO-8601 timestamp>              (every service)
  -X github.com/neochaotic/powerlab/backend/core/common.POWERLAB_VERSION=$VERSION
                                                     (core only —
                                                     Go ignores the
                                                     flag for other
                                                     services)
  -X github.com/neochaotic/powerlab/backend/core/route/v1.powerLabVersionAtCompileTime=$VERSION
                                                     (read by the
                                                     updater's
                                                     currentPowerLab-
                                                     Version() to
                                                     compare against
                                                     the manifest)

Includes regression test at
`scripts/check-package-linux-ldflags_test.sh` (8 assertions):
  - All 4 expected ldflag target paths are present in package-
    linux.sh
  - The 2 deprecated targets (the v0.5.4 mishap shapes) are
    absent
  - The 2 target Go vars (POWERLAB_VERSION, powerLabVersion-
    AtCompileTime) actually exist in the source — catches future
    renames that would silently break the build pipeline.

Closes #159.

### Internal
- Pre-tag check: `scripts/check-manifest-fresh.sh` refuses to
proceed when `release-manifest.yaml` summary is identical to
the previously published GitHub release's summary.

Catches the failure mode that bit v0.5.4: maintainer forgot
to update the YAML, so the manifest.json shipped with v0.5.0's
summary text. Hot-fixed via `gh release upload manifest.json
--clobber` after the fact (see issue #156).

Wired into `docs/release-checklist.md` Phase 1.

Includes regression tests at
`scripts/check-manifest-fresh_test.sh` covering:
  - Identical summary → exit 1 (the v0.5.4 case)
  - Different summary → exit 0
  - Empty summary in fixture → exit 0 (defensive)
  - Nonexistent fixture path → exit 2

Also refreshed `release-manifest.yaml` summary to match v0.5.4's
hot-patched text — so v0.5.5 maintainer must edit the YAML
before tagging, otherwise the new check blocks them.

- Release v0.5.5 — v0.5.4 incident hotfix. Two real upgrade-path
bugs fixed with regression tests:

  - #158: install.sh now auto-migrates /var/lib/casaos/* →
    /var/lib/powerlab/* on upgrade. Without this, v0.5.x → v0.5.4
    hosts ended up with empty /var/lib/powerlab/db/ → user-service
    couldn't find users → login returned 400 → UI unusable.
    Hot-fixed manually on the affected host; this PR closes the
    class permanently. 5-scenario regression test
    (scripts/migrate-casaos-data_test.sh) locks the behavior.

  - #159: release builds now correctly inject commit / build-date /
    version into the binary. The v0.5.4 ldflag string was double-
    broken — wrong variable name AND dead module path (after #151
    module rename). Result: binary identified itself as "dev",
    in-UI updater showed "Update available" forever even on the
    latest release, triggered no-op upgrade loop that restarted
    services + invalidated JWT. Two regression tests
    (check-package-linux-ldflags_test.sh + main_version_stamp_test.go)
    catch future bit-rot of the build pipeline.

  - #156: pre-tag check that release-manifest.yaml summary was
    refreshed for the new version. The check failed when this
    paragraph was being drafted — exactly the case it was added
    to catch. 4-assertion regression test
    (check-manifest-fresh_test.sh).

Plus Sprint 4 progress (still in flight):
  - #85 PR1: rename .casaos-appstore + casaos-compose-app-* internal
    literals.
  - #85 PR2: CasaOSGlobalVariables struct → AppLifecycleFlags.
  - Sprint 4 prep audit at
    docs/audits/sprint-4-app-management-prep.md.

No end-user behavior change beyond the bug fixes — same wire
formats, same DB schema, same settings.



## [v0.5.4] — 2026-05-09
### Added
- Sprint 3 closeout documentation.

- **ADR-0019**: codifies the project's tech-debt tracking pattern.
  Three sources of truth — `docs/audits/` for structural audits,
  `docs/decisions/` for ADRs, GitHub issues with labels for the
  live work queue. **No `TECH-DEBT.md` / `TODO.md` at the repo
  root** (they would inevitably go stale and lie). Includes the
  refresh-discipline rules so the next person who has the reflex
  to add one finds the reasoning first.

- **`docs/audits/casaos-dependencies.md`** refreshed per the
  ADR-0019 convention (Update section appended at top, Sprint 1
  baseline preserved below as historical record). Captures the
  Sprint 2 + Sprint 3 outcomes: 10 closeout PRs (#139–#148),
  ~3500 LOC removed, cloud-drive infrastructure killed in both
  local-storage and core, /etc/casaos → /etc/powerlab paths
  completed across 5 services, casaos:* → powerlab:* topics,
  PersistentTypeCasaOS rebranded, 165 logger sites migrated.
  Documents what's left in the CasaOS surface (Go module path
  rename is the next major target).

### Changed
- Renamed all Go module paths from legacy `github.com/IceWhaleTech/CasaOS-*`
to PowerLab-owned `github.com/neochaotic/powerlab/backend/*`. This is the
final structural rebrand step — every Go service in the tree now compiles
under a PowerLab module identity.

**Renamed modules (6):**

| Service          | Old module                                     | New module                                              |
|------------------|------------------------------------------------|---------------------------------------------------------|
| `app-management` | `github.com/IceWhaleTech/CasaOS-AppManagement` | `github.com/neochaotic/powerlab/backend/app-management` |
| `user-service`   | `github.com/IceWhaleTech/CasaOS-UserService`   | `github.com/neochaotic/powerlab/backend/user-service`   |
| `core`           | `github.com/IceWhaleTech/CasaOS`               | `github.com/neochaotic/powerlab/backend/core`           |
| `local-storage`  | `github.com/IceWhaleTech/CasaOS-LocalStorage`  | `github.com/neochaotic/powerlab/backend/local-storage`  |
| `common`         | `github.com/IceWhaleTech/CasaOS-Common`        | `github.com/neochaotic/powerlab/backend/common`         |
| `cli`            | `github.com/IceWhaleTech/CasaOS-CLI`           | `github.com/neochaotic/powerlab/backend/cli`            |

All `replace github.com/IceWhaleTech/CasaOS-Common => ../common` directives
across dependent services were updated to point at the new module path.
The `cli` service gained a new `replace` directive (it previously fetched
`CasaOS-Common` from the network).

**Intentionally NOT touched:**

- `go:generate bash -c "... https://raw.githubusercontent.com/IceWhaleTech/..."`
  URLs in `main.go` files. These point at the upstream IceWhaleTech repos on
  GitHub as external data sources for `oapi-codegen`. The generated package
  names (e.g. `-package casaos`, `-package user_service`) are also untouched
  — they are local Go package names, not import paths.
- License headers, `@Website`, and `@FilePath: /CasaOS/...` markers in file
  headers. These are historical attribution per ADR-0019.
- Filesystem path constants in `cmd/migration-tool/main.go` (those are the
  legacy CasaOS migration tools that intentionally read from upstream
  `/etc/casaos/...` paths).

- Service config paths fully rebranded from `/etc/casaos` to
`/etc/powerlab`. Sprint 3 Phase 3 — completes the structural
CasaOS strangler for filesystem layout (#101).

Why this matters now: install.sh + the systemd units already
wrote configs to `/etc/powerlab/` and started services with
`-c /etc/powerlab/<svc>.conf`, but the in-binary defaults
(used when `-c` is absent and at first-boot file creation)
still pointed at `/etc/casaos/<svc>.conf`. Two services
(message-bus, local-storage) also imported the upstream
CasaOS-Common Go module rather than the local fork at
`backend/common/`, so even `constants.DefaultConfigPath`
resolved to `/etc/casaos`. Net effect: a class of subtle
divergence between what install.sh shipped and what the
binary read.

Production bug uncovered while doing this:
install.sh used to ship `casaos.conf.sample` into
`/etc/powerlab/casaos.conf`, but systemd starts the core
service with `-c /etc/powerlab/core.conf`. The file basenames
disagreed → the binary opened a non-existent path,
silently created an empty `core.conf`, and dropped every
shipped default. Renamed the sample + the in-binary const
(`CasaOSConfigFilePath` → `CoreConfigFilePath`) so the
three sources (sample basename, systemd `-c` flag, Go
default) finally agree.

Concrete changes:

- 5 services: `build/sysroot/etc/casaos/` →
  `build/sysroot/etc/powerlab/` (renamed, not deleted —
  the embedded sample is still shipped via `//go:embed`).
- 5 services: `//go:embed` directives updated to point at
  the new sysroot path.
- `core/pkg/config/config.go`: `CasaOSConfigFilePath` →
  `CoreConfigFilePath`, value `casaos.conf` → `core.conf`.
- `message-bus/config/config.go`,
  `local-storage/pkg/config/config.go`: hardcoded
  `/etc/casaos/<svc>.conf` literal → derived via
  `filepath.Join(constants.DefaultConfigPath, ...)`.
- `message-bus/go.mod`, `local-storage/go.mod`: added
  `replace github.com/IceWhaleTech/CasaOS-Common =>
  ../common` so they share the local fork's Linux/darwin
  paths (`/etc/powerlab` / `/opt/powerlab/etc`) like
  app-management/user-service/core already did.
- `scripts/package-linux.sh`: emit `core.conf.sample`
  instead of `casaos.conf.sample` (matches systemd `-c`).
- 4 new `pkg/config/config_test.go` files (TDD): assert
  the path matches `constants.DefaultConfigPath` AND
  contains no legacy `/casaos/` substring. These would
  have caught the prod mismatch.

Behavior on existing installs:
- Sample is only written on first install when the file
  is absent (install.sh line 504: `if [[ ! -f ... ]]`).
  Existing `/etc/powerlab/casaos.conf` from prior installs
  is left untouched — but ALSO never read by core, since
  systemd passes `-c /etc/powerlab/core.conf`. So the old
  file becomes a harmless orphan; no migration needed.

Compatibility:
- The legacy `/etc/casaos.conf` (single file, distinct
  from the per-service `/etc/casaos/<svc>.conf`) is still
  read by `version.LegacyCasaOSConfigFilePath` for
  co-resident hosts; that path is intentional CasaOS
  interop and unchanged.
- The `/etc/casaos` production marker in
  `constants/paths.go::devProductionMarkers` is also
  unchanged — it lets a binary running on a co-resident
  CasaOS host detect the install rather than falling
  into dev-sandbox mode.

- Message-bus topic prefixes migrated from `casaos:*` to
`powerlab:*` for self-describing routing in logs + traces.
Sprint 3 Phase 3 — third structural rebrand PR (#101).

Concrete renames in core's EventTypes registry + all
publish call-sites:

  casaos:system:utilization  → powerlab:system:utilization
  casaos:file:operate        → powerlab:file:operate

Both topics + their publish call-sites are now wrapped in
named constants in `backend/core/common/message.go`
(EventSystemUtilization, EventFileOperate) so the rename
is single-source.

Held back: `casaos:file:recover` stays on the legacy
prefix (kept as `EventCloudFileRecover` const) because
it's still referenced by core's parallel cloud-drive
infrastructure (drivers/{dropbox,google_drive,onedrive},
route/v1/recover.go). That topic dies together with that
infrastructure in a follow-up PR mirroring #139's
local-storage cloud-drive removal.

Safety: verified by grep across the SvelteKit UI + all
6 backend services that no PowerLab component subscribes
to the `casaos:` prefix. The rename is non-breaking.

- /v1/storage `PersistedIn` field value `"casaos"` → `"powerlab"`.
Sprint 3 Phase 3 — fourth structural rebrand PR (#101). The
Go-side const also renamed: `PersistentTypeCasaOS` →
`PersistentTypePowerLab` (value `"powerlab"`).

Wire-format change. Pre-v1.0 (current is v0.5.3), so allowed.
Verified by grep across the SvelteKit UI that no PowerLab
consumer switches on the literal `"casaos"` value.

Risk: external API consumers (apps installed in the user's
PowerLab) that read /v1/storage and switch on the
`PersistedIn` field. Such consumers would need to update.
No PowerLab-shipped app does this. v1.0 wire format will be
documented as part of #71 (mkdocs-material site).

- Embedded sysroot config samples rebranded — internal CasaOS
paths swapped for PowerLab equivalents. Sprint 3 Phase 3
follow-up to #140 (which renamed the directories but left
contents alone).

Why this matters: each service `//go:embed`s its sample as
the `_confSample` string, used to seed `/etc/powerlab/<svc>.conf`
on first boot when no config file exists. install.sh's heredoc
samples already wrote PowerLab paths to disk in production,
but the embedded samples still leaked CasaOS paths into:

  - dev mode (running the binary by hand, no install.sh)
  - any future install path that didn't go through install.sh
  - readers of the source code looking for documentation

Path renames inside each sample:

  /var/run/casaos       → /var/run/powerlab
  /var/log/casaos       → /var/log/powerlab
  /var/lib/casaos       → /var/lib/powerlab
  /usr/share/casaos     → /usr/share/powerlab

core.conf.sample: dropped the dead CasaOS upstream endpoints
(`ServerApi = https://api.casaos.io/casaos-api`,
`Handshake = socket.casaos.io`, `Token =`). PowerLab has its
own version + updater stack (#21 in-UI updater) and never
reads these — leaving them in the sample risked silent network
requests to casaos.io infrastructure on first boot. Removed
rather than blanked so they're not even mentioned.

Kept (intentional, external 3rd-party data source):
app-management.conf.sample's `appstore = https://github.com/
bigbeartechworld/big-bear-casaos/...` — community-maintained
app store repo whose name predates PowerLab. The data flowing
through it is the actual app catalog; renaming the URL would
break the install.

- local-storage finishes the migration off the legacy CasaOS
`logger.X(msg, zap.X(...))` helpers and onto PowerLab's own
`pkg/logging` Logger interface (built on `log/slog`). Same
pattern user-service, gateway, message-bus, and core were
migrated to in earlier sprints (ADR-0011).

Per-package work:

  backend/local-storage/service          (disk, storage, usb, notify)
  backend/local-storage/service/v2       (merge, mount, fstab)
  backend/local-storage/route/v1         (storage, disk, usb)
  backend/local-storage/route/v2         (merge)
  backend/local-storage/pkg/utils/merge  (merge.go)
  backend/local-storage/pkg/mergerfs     (mergerfs.go)
  backend/local-storage/main.go          (residual sites + SetLogger
                                          wiring for every package)

Each migrated package exposes a `var _log pkglogging.Logger`
+ `func SetLogger(l)` so main() can hand the configured
foundation logger to every package; before main() runs every
`_log` defaults to a permissive json/info logger so init-time
goroutines don't crash on a nil receiver.

Mechanical 1:1 mapping:

  logger.Info(msg, zap.X(...))                → _log.Info(ctx, msg, slog.X(...))
  logger.Error(msg, zap.Error(err), zap.X..)  → _log.Error(ctx, msg, err, slog.X(...))
  logger.Warn(msg, ...)                       → _log.Warn(ctx, msg, ...)

HTTP handlers pass `ctx.Request().Context()`; background
goroutines and helpers off the request path use
`context.Background()`. No behaviour change beyond the
emission backend.

Side-effect cleanup:

  - Drop the legacy `logger.LogInit(...)` file-rotation
    bootstrap from main.go — with zero remaining call
    sites it was setting up zap log files nothing wrote
    to.
  - Drop the now-dead CasaOS `utils/logger` and
    `go.uber.org/zap` imports from every migrated file.

Out of scope (intentionally untouched):

  - `cmd/migration-tool/main.go` keeps the legacy logger;
    it's a one-shot DB migration tool that runs outside
    the service process.
  - `service/storage.go` (StorageService) and
    `pkg/utils/httper/drive.go` are dead code after #139
    (cloud-drive removal) — registered in service.go but
    never called. Grep confirms zero non-self callers.
    Deletion left to a follow-up PR to keep this one
    mechanical.

Final logger-call-site count in local-storage/ (excluding
cmd/migration-tool, tests, comments): 0 — down from ~143
before this PR.

- user-service SERVICENAME and message-bus topic rebranded.
Sprint 3 Phase 3 — sixth structural rebrand PR (#106).

Renames:

  SERVICENAME           "CasaOS-UserService" → "PowerLab-UserService"
  Event topic           "zimaos:user:save_config"
                        → "powerlab:user:save_config"

SERVICENAME is published as the SourceID on every event the
service emits and registers under in /v2/message_bus/event_type.
Surfaces in every cross-service log line that mentions a
user-service event — the legacy value advertised CasaOS
branding from inside a PowerLab process. The topic rename
follows the powerlab:* convention established in #141.

Wire-format change. Pre-v1.0 (current is v0.5.3). Verified by
grep across the SvelteKit UI that no PowerLab consumer
switches on either of the legacy values.

Held back: the Go module path
`github.com/IceWhaleTech/CasaOS-UserService/...` is still
upstream-named because renaming a module is a sweeping
refactor in its own right (every import line in the service
+ the cli that consumes the user-service codegen). Tracked
for a separate PR; the user-visible rebrand surface (event
SourceID + topics) is what this PR fixes.

### Removed
- Cloud drive backends (Dropbox, Google Drive) and the
/v1/cloud, /v1/driver, /v1/recover endpoints removed entirely
in Sprint 3 Phase 3 (#101 / casaos-strip option 3).

Why: cloud drive flow depended on the CasaOS-team-hosted OAuth
proxy at `cloudoauth.files.casaos.app`. Keeping it tethered the
product to CasaOS infra forever — incompatible with the v1.0
goal of removing all CasaOS dependencies.

Per #101 option 3, dropped entirely for v1.0 instead of
spinning up our own OAuth proxy (which needs domain + cloud
worker infra + per-provider OAuth app registrations). If we
bring cloud drives back post-v1.0, it'll be on PowerLab-owned
infrastructure with the trust-dance redo (#118) addressed
first.

Removed:
- `backend/local-storage/drivers/` (dropbox, google_drive,
  base, all.go) — 826 LOC
- `backend/local-storage/route/v1/cloud.go` (ListStorages,
  UmountStorage handlers — body was already commented out)
- `backend/local-storage/route/v1/recover.go`
  (GetRecoverStorage OAuth callback flow)
- `backend/local-storage/route/v1/driver.go`
  (ListDriverInfo — fed from internal/op which only existed
  for cloud drivers)
- `backend/local-storage/internal/` entire dir
  (op, driver, sign, conf — all alist-derived cloud-driver
  infrastructure with zero non-cloud callers)

Routes removed:
- GET    /v1/cloud
- DELETE /v1/cloud
- GET    /v1/driver

Net diff: ~1500 LOC removed, zero added. Eliminates the
`cloudoauth.files.casaos.app` external dependency.

- Dead-code cleanup in local-storage. After #139 dropped the
cloud-drive routes, two files lost all production callers
but were missed in that PR's scope:

  backend/local-storage/service/storage.go         (206 LOC)
  backend/local-storage/pkg/utils/httper/drive.go  (161 LOC)

Verified zero non-self callers via grep across the service.
StorageService was still registered in service.go's Services
interface + DI store, but no live code path called
MyService.Storage() — only a commented-out reference in
main.go (kept the kill-PR's TODO marker, now removed too).

Net: 374 LOC removed, zero added.

service.go also pruned: Storage() method, struct field,
NewStorageService() invocation all gone.

- Cloud drive backends (Dropbox, Google Drive, OneDrive) and the
/v1/cloud, /v1/driver, /v1/recover endpoints removed from the
core service. Sprint 3 Phase 3 — fifth structural rebrand PR
(#101). Mirrors the local-storage cloud-drive removal in #139,
closing the second half of the cloud-drive surface.

Why: same as #139 — the OAuth flow tied PowerLab to the
CasaOS-team-hosted proxy at `cloudoauth.files.casaos.app`,
incompatible with the v1.0 goal of a clean fork. Hosting our
own OAuth proxy is post-v1.0 work IF cloud drives come back.

Removed (1862 LOC, zero added):

- `backend/core/drivers/`            14 files
                                     dropbox, google_drive, onedrive,
                                     base — the alist-derived cloud
                                     driver fork
- `backend/core/route/v1/cloud.go`   ListStorages, UmountStorage
- `backend/core/route/v1/recover.go` GetRecoverStorage OAuth callback
- `backend/core/route/v1/driver.go`  ListDriverInfo
- `backend/core/service/storage.go`  StorageService interface +
                                     storageStruct (the rclone
                                     client wrapper, only used
                                     by the now-deleted routes)
- `backend/core/pkg/utils/httper/drive.go`
                                     MountList / MountPoints types
                                     (only consumed by storage.go)

Updated:

- `backend/core/route/v1.go`         /v1/recover registration +
                                     /v1/cloud + /v1/driver groups
                                     removed (replaced with comment)
- `backend/core/main.go`             /v1/cloud, /v1/driver,
                                     /v1/recover removed from
                                     gateway routers list
- `backend/core/route/init.go`       Storage().CheckAndMountAll()
                                     boot-time auto-mount removed
- `backend/core/service/service.go`  Storage() interface method,
                                     NewStorageService(), struct
                                     field all removed
- `backend/core/.goreleaser.yaml`    OAuth credential ldflags
                                     (DropboxKey/Secret, GoogleID/
                                     Secret, OneDriveID/Secret) for
                                     both amd64 and arm64 builds.
                                     Goreleaser no longer requires
                                     these env vars at release time.
- `backend/core/common/message.go`   EventCloudFileRecover const +
                                     its EventTypes entry removed
                                     (had no remaining publisher
                                     after route/v1/recover.go gone)
- `backend/core/common/message_test.go`
                                     drops the special-case skip
                                     for casaos:file:recover; now
                                     asserts unconditional powerlab:
                                     prefix on every registered topic

### Fixed
- Multiple hardcoded `/var/lib/casaos/...` and `/var/log/casaos/...`
paths in runtime code that survived the #140 etc-paths rebrand
because they reference data/log/share dirs (constants.Default*Path)
rather than config dirs (constants.DefaultConfigPath).

Each is a real production bug: PowerLab installs put data under
`/var/lib/powerlab/`, logs under `/var/log/powerlab/`, and shared
data under `/usr/share/powerlab/`, so the hardcoded `/casaos/`
paths pointed at directories that don't exist.

Fixes:

  backend/app-management/route/v1/docker.go
    dockerRootDirFilePath
    "/var/lib/casaos/docker_root"
    → filepath.Join(constants.DefaultDataPath, "docker_root")

  backend/core/route/v2/health.go
    GetHealthlogs log archiver
    "/var/log/casaos" (3 sites)
    → constants.DefaultLogPath

  backend/core/service/system.go
    GenreateSystemEntry + GetSystemEntry
    "/var/lib/casaos/www/modules" (2 sites)
    → filepath.Join(constants.DefaultDataPath, "www", "modules")

  backend/core/pkg/config/init.go
    AppInfo.ShellPath default
    "/usr/share/casaos/shell"
    → filepath.Join(constants.DefaultConstantPath, "shell")

  backend/local-storage/pkg/config/init.go
    AppInfo.ShellPath default
    "/usr/share/casaos/shell"
    → filepath.Join(constants.DefaultConstantPath, "shell")

  backend/local-storage/route/v2/merge.go
    User-facing error messages mentioned "/var/lib/casaos/files"
    → use the resolved constants.DefaultFilePath value

Held back: backend/common/utils/version/migration.go's
GlobalMigrationStatusDirPath = "/var/lib/casaos/migration" is
intentionally CasaOS-pointed (it tracks legacy CasaOS-→PowerLab
migration status). Belongs to the migration-tool surface that
#140 explicitly held back.

### Internal
- Release v0.5.3 — patch release. Closes the toast/UUID regression
introduced by v0.5.2's HTTPS disable (#137 / crypto.randomUUID
needs secure context, broke on http://IP:port). Plus completes
Sprint 3 Phase 2 migrations (#100): core and local-storage now
use pkg/migrations alongside user-service and message-bus —
AutoMigrate fully retired across all 4 state-owning services.

- Release v0.5.4 — Sprint 3 closeout. The "kill CasaOS" sprint
substantially closes:

  - Cloud drive backends removed from local-storage AND core
    (-1500 + -1862 LOC). Kills the
    `cloudoauth.files.casaos.app` OAuth proxy dependency.

  - Go module paths renamed across all 6 services
    (`github.com/IceWhaleTech/CasaOS-*` →
    `github.com/neochaotic/powerlab/backend/*`).

  - Filesystem paths migrated `/etc/casaos` → `/etc/powerlab`
    across 5 services. Plus a real prod bug fix: install.sh
    shipped `casaos.conf.sample` → `/etc/powerlab/casaos.conf`
    while systemd started core with `-c /etc/powerlab/core.conf`,
    dropping every shipped default into nothing. Sample renamed
    `core.conf.sample`.

  - Hardcoded `/var/lib/casaos/...` runtime paths fixed in 6
    files (real prod bugs that pointed at non-existent dirs on
    PowerLab installs).

  - Message-bus topic prefixes rebranded
    (`casaos:* → powerlab:*`).

  - Disk persistence-type discriminator rebranded
    (`PersistentTypeCasaOS → PersistentTypePowerLab`,
    wire value `"casaos" → "powerlab"`).

  - user-service `SERVICENAME` rebranded
    (`"CasaOS-UserService" → "PowerLab-UserService"`) +
    `zimaos:user:save_config` topic rebranded.

  - 165 legacy `logger.X(msg, zap.X(...))` call sites in
    local-storage migrated to `_log.X(ctx, msg, slog.X(...))`
    via `pkg/logging`.

  - Embedded sysroot config samples + dead `casaos.io`
    endpoints purged.

Net: ~3500 LOC removed. Remaining CasaOS surface is in
app-management (Sprint 4 territory, audit at
`docs/audits/sprint-4-app-management-prep.md`).

Process: ADR-0019 codifies the tech-debt tracking pattern
going forward — `docs/audits/` + `docs/decisions/` + labeled
GitHub issues, no `TECH-DEBT.md` at the repo root.

No behavior change for end users beyond the bug fixes above.
Wire-format changes (PersistentType, topic prefixes,
SERVICENAME, casaos_apps key) are pre-v1.0 allowed renames
with no PowerLab UI consumer.



## [v0.5.3] — 2026-05-09
### Changed
- core now uses `pkg/migrations` (versioned goose migrations) in
place of GORM's `db.AutoMigrate(...)`. Schema captured by
running existing AutoMigrate against in-memory SQLite and
dumping `sqlite_master` so it matches what installs already
have on disk. `CREATE TABLE IF NOT EXISTS` keeps upgrades safe.
The legacy CasaOS table cleanup (`o_application`, `o_friend`,
`o_person_download`, `o_person_down_record`) that used to live
in `db.Exec` calls right after `AutoMigrate` is now in the
migration SQL itself — co-located with the schema definition.
Plus stripped the legacy LinkLeong file header from `db.go`.
Two new smoke tests in `pkg/sqlite/migrations_test.go` lock
the 4-table schema + idempotent re-run. Third of 4 services
(#100); local-storage remains for Phase 2.4.

- local-storage now uses `pkg/migrations` (versioned goose
migrations) in place of GORM's `db.AutoMigrate(...)`. Final
service of the 4 to retire AutoMigrate per #100. Schema captured
verbatim from existing AutoMigrate output: `o_disk` (Volume,
table-name override preserved for legacy CasaOS compat),
`o_merge` (Merge), `o_merge_disk` (many2many junction with
foreign keys). `CREATE TABLE IF NOT EXISTS` keeps upgrades
safe. Two new smoke tests in `pkg/sqlite/migrations_test.go`
cover the 3-table schema + idempotent re-run. Closes #100.

### Fixed
- Toast store no longer crashes on `crypto.randomUUID is not a
function` when accessed via insecure context (http://IP:port).
v0.5.2 disabled HTTPS by default per #130; that turned every
fresh install into a non-secure-context environment.
`crypto.randomUUID` requires a secure context (HTTPS, localhost,
or file://); without it, every `toast.add()` threw, no toasts
appeared, and downstream UI flows that depend on toast feedback
(deploy success, save confirmation) silently failed.

Fix: new `$lib/utils/uuid.ts::generateID()` with three-tier
fallback — `crypto.randomUUID` → `crypto.getRandomValues` (Web
Crypto, NOT secure-context-restricted) → `Math.random`
composition (last-resort for old jsdom). Toast store now uses
generateID. 9 regression tests in `uuid.test.ts` cover each
fallback branch + uniqueness.

Surfaced during user testing of v0.5.2 on a fresh
http://192.168.x.y:8765 install.

### Internal
- Release v0.5.2 — incident response patch over v0.5.1. Closes the
v0.5.0 → v0.5.1 in-app upgrade incident class via three coordinated
fixes: install.sh cgroup escape (#129/#132), 4 boot-time gateway
bugs (#131), and surgical HTTPS feature gate (#130/#133). HTTPS is
now opt-in via `POWERLAB_HTTPS_ENABLED=true` and gates re-enable in
v0.6 on trust-dance redo (#118) + integration tests. Plus the
message-bus migration adoption (#127) from Sprint 3 Phase 2.



## [v0.5.2] — 2026-05-09
### Changed
- message-bus now uses `pkg/migrations` (versioned goose migrations)
in place of GORM's `db.AutoMigrate(...)`. The service has TWO
embedded migration filesystems — `migrations_events` (event_types,
action_types, property_types + the two many2many junction tables)
and `migrations_persist` (settings, ysk_cards) — because each
goose run owns its own goose_db_version sequence per DB. The
0001_initial.sql files were captured by running the existing
AutoMigrate against an in-memory SQLite and dumping
`sqlite_master` so the schema matches what existing installs
already have on disk; `CREATE TABLE IF NOT EXISTS` keeps
upgrades safe. Two new smoke tests in `repository_test.go`
cover both DBs. Fixed `NewDatabaseRepositoryInMemory()` to use
distinct named in-memory shared caches per DB — previously both
used `file::memory:?cache=shared` (same identifier = same
backing store), which made goose's version table conflict
between the two migration runs.

- HTTPS feature is now opt-in via `POWERLAB_HTTPS_ENABLED=true`
environment variable (default: gated). Closes #130.

When gated:
- Cert manager `Setup()` is a no-op — no CA generated, no
  server cert written
- Cert download endpoints (`.crt`, `.mobileconfig`, `.cer`,
  `ca-certificate` redirect) return 503 with
  `{code: "https.gated", message: ...}` JSON
- Trust mutations (`/v1/sys/trust-confirmed`, `/v1/sys/rotate-ca`)
  return the same 503
- HSTS header NOT emitted on HTTP requests; emitted as
  `max-age=0` on HTTPS requests to actively reset cached HSTS
  in already-locked browsers
- HTTP→HTTPS 301 redirect is suppressed; users access the
  panel directly on port 8765
- HTTPS listener (8443) is not bound (cert files don't exist
  so the existing graceful skip in main.go fires)
- Read-only `/v1/sys/trust-state` is INTENTIONALLY NOT gated
  so the UI can render the gated banner

When the env var is set to exactly `"true"` (strict comparison —
no "1" / "yes" / "TRUE"), the full HTTPS flow is restored. v0.6
ships with HTTPS re-enabled by default after trust-dance redo
(#118) and integration tests land.

Surgical change — no code deletion, no frontend changes. Re-enable
in v0.6 = flip one default. New `pkg/security.HTTPSEnabled()`
helper + 11 regression tests across 3 test files lock the gate
shape.

### Fixed
- In-app upgrade no longer kills install.sh mid-flight (#129).
Previously `core` spawned `install.sh` with
`SysProcAttr.Setsid=true`, betting that escaping the SESSION
would prevent SIGTERM propagation when install.sh ran
`systemctl stop powerlab-core`. But systemd tracks units by
CGROUP, not session — so the default `KillMode=control-group`
on `powerlab-core.service` sent SIGTERM to every process in
core's cgroup, INCLUDING install.sh. Result: binaries copied,
services stopped, restart loop never executed → users locked
out. Now `core` spawns install.sh via `systemd-run --no-block
--collect --unit=powerlab-upgrade ...` which creates a
transient systemd scope unit with its own cgroup. install.sh
survives the stop of core and completes the upgrade-restart-
health-check cycle as designed. Two regression tests in
`powerlab_updater_test.go` lock the spawn pattern (must use
systemd-run, must NOT set SysProcAttr).

- Gateway: 4 boot-time SIGSEGV / infinite-loop bugs surfaced during the
v0.5.0 → v0.5.1 in-app upgrade incident (#130). All in
`backend/gateway/main.go`:

1. `checkURL` nil-deref on err path — bug-#64 ressuscitado: original
   code did `defer response.Body.Close()` unconditionally even when
   `http.Get` returned `(nil, err)`. Was thought "structurally
   closed" by `pkg/foundation.RecoverMiddleware` but that
   middleware only covers HTTP handlers — not boot-time fx OnStart
   hooks where checkURL runs. Fixed by checking err FIRST.

2. `checkURL` StatusCode logic was inverted (`== StatusOK`
   returned `ErrCheckURLNotOK`). Worked-by-accident because
   bug 1 returned early on success. Now: any HTTP response
   means the listener is up (preserves boot semantics where the
   gateway redirects HTTP→HTTPS with 301 — checking for 200
   would loop forever).

3. `reloadGateway` self-ping URL constructed from `listener.Addr()`
   which returns the BIND address (`[::]:PORT`) — invalid as TCP
   CLIENT destination on IPv6-strict configs (`http.Get` to `[::]`
   fails). Fixed by new `clientLoopback()` helper that rewrites
   bind addresses to 127.0.0.1.

4. `checkURLWithRetry` used `count uint` with `for count >= 0;
   count--`. uint never goes negative; `count--` from 0 wraps to
   MAX_UINT64 → infinite retry. Combined with bug 3 (the URL was
   unreachable), this turned the gateway into a 100% CPU spinner
   that blocked all subsequent fx OnStart hooks (HTTPS, static, etc.)
   from running, leaving 8443 unbound and users locked out by HSTS.
   Fixed by switching to `int` with bounded `for i := 0; i <= retry`.

Plus an init() restructure: skip the heavy startup work (flag.Parse,
config load, logger init) in test binaries — `flag.Parse` was
rejecting `-test.*` flags and crashing the test binary. Also adds
7 regression tests in `main_check_url_test.go` covering all 4 bugs
+ happy paths.

### Internal
- Release v0.5.1 — bug-fix patch over v0.5.0. Three user-reported
bugs from the v0.5.0 testing session are addressed: discoverable
file selection on Files page (#121), cert download diagnostics
on Settings (#124), and orchestrator deploy phase indicator +
timeout parity with native-app install (#125). Plus pkg/migrations
foundation (#115) and user-service migration adoption (#117) from
Sprint 3 Phase 1-2.



## [v0.5.1] — 2026-05-09
### Added
- `backend/pkg/migrations` — a thin wrapper over `pressly/goose/v3`
that gives every PowerLab service a versioned, transactional,
rollbackable schema migration runner. Three exported functions:
`Up(ctx, db, fsys, dir)`, `Down(ctx, db, fsys, dir)`,
`Version(ctx, db) (int64, error)`. Six TDD'd tests cover happy
path, idempotency, partial-already-applied, malformed-SQL-fails,
fresh-DB-returns-zero, and Down rollback. `Version` queries
`goose_db_version` directly so it can be called from health
endpoints without threading a migration FS. Lays the foundation
for retiring `db.AutoMigrate(...)` across all four
state-owning services in subsequent Sprint 3 PRs (see ADR-0018
for the rationale and goose-vs-golang-migrate-vs-atlas
comparison).

### Changed
- user-service now uses `pkg/migrations` (versioned goose
migrations) in place of GORM's `db.AutoMigrate(...)`. Schema
is captured in `backend/user-service/pkg/sqlite/migrations/0001_initial.sql`
and embedded in the binary via `embed.FS`. Existing installs
continue to work because the migration uses
`CREATE TABLE IF NOT EXISTS` — running it on a DB that already
has the tables (created previously by AutoMigrate) is a safe
no-op that simply records the schema as version 1 in
`goose_db_version`. Two new tests in `pkg/sqlite` smoke the
embedded migrations: `TestEmbeddedMigrations_Up_ProducesExpectedTables`
and `TestEmbeddedMigrations_Up_Idempotent`. First service to
retire AutoMigrate; message-bus, core, local-storage follow
in subsequent PRs.

- Custom-app deploy (#116 item 3): the orchestrator's deploy
overlay now shows the same `Phase N/M — label` indicator and
progress bar that the native-app install overlay has, parsed
from the SSE log stream via the existing `parseLatestPhase`
helper. Adds a 10-minute safety timeout (matches native-app
`streamInstallLogs`); when the SSE stream wedges past that
window the overlay surfaces a "taking longer than expected"
banner instead of spinning silently. Existing
`install-phase.test.ts` (12 tests) covers the parsing; the
wiring is template-only.

- Settings → CA certificate download UX (#118): the
"Could not download the certificate" toast now includes the
HTTP status code and (when short) the response body excerpt.
Empty-body responses produce a distinct "(empty body)" toast
instead of a silent dropped download. Both branches also log
the failure context to the browser console so the next bug
report has a fingerprint to act on. Diagnostic-only change —
the underlying #118 fix (rebuild trusted-dance flow) requires
the extraction tracked in #123 first to be safely
regression-tested.

### Fixed
- Files page (#116 item 1): adds an always-visible checkbox column
to FileTable so users can select files without needing to know
about Cmd/Ctrl/Shift-click. The Delete button in the toolbar
already shows when `selectedCount > 0` — the missing piece was
a discoverable selection affordance. Header checkbox toggles
select-all / deselect-all. Row checkboxes don't propagate clicks
so they don't fire the row's open handler (selecting and opening
are now cleanly separated).

Adds 3 regression tests (`FileTable.test.ts`) locking: checkboxes
render per row, click-to-select fires onSelect, and clicking a
checkbox does NOT fire onOpen.

Adds 1 regression test (`TextEditor.test.ts`) covering the
create-flow save → toast.success path (#116 item 1, save toast
half). The PUT-path equivalent is intentionally skipped at the
vitest level because jsdom + CodeMirror don't simulate typing
reliably enough to flip isDirty; that path is queued for
Playwright coverage in #108.

Adds ResizeObserver no-op polyfill to dom-polyfills so component
tests that observe their container size (FileTable here, others
later) don't throw at render time.

### Internal
- Release v0.5.0 — Sprint 2 ships. CHANGELOG.md generated from the
unreleased fragments via `changie batch v0.5.0` + `changie merge`,
consumed fragments archived to `.changes/v0.5.0.md`. This is
the first release using the changie workflow #98 introduced.



## [v0.5.0] — 2026-05-09
### Changed
- Migrate the easy slice of `local-storage` logging to PowerLab's
`pkg/logging` (slog-based, with correlation-ID propagation):
`main.go`'s `main()` and `RegMsg()` functions, plus all of
`misc.go` (uevent monitor + disk/usb notify helpers). Logs now
carry `request_id` / `correlation_id` automatically when emitted
from contexts that have one, and structured attrs use `slog.Attr`
instead of `zap.Field`. The remaining ~190 call sites across
`route/`, `service/`, `drivers/`, `internal/`, `pkg/utils/*`,
`cmd/migration-tool/` are tracked in #104 — they need
`ctx`-threading work that doesn't belong in this PR. The legacy
`logger.LogInit` call and `init()`-context legacy logger calls
remain in place because `_log` is constructed in `main()` and is
nil at init-time; they migrate together with the init refactor
in part 4.

- Wire PowerLab foundation middleware into the `local-storage`
service: `wrapWithFoundation` applies `pkgtracing.Middleware`
(correlation IDs via X-Request-Id) and
`pkglifecycle.RecoverMiddleware` (panic recovery, logs with
stack + correlation ID, writes 500 via `pkg/errors.WriteHTTP`)
around the single `http.Server.Handler`. Goroutines spawned from
`main()` (uevent monitor, storage stats, message-bus event
registration, disk init check) now run via `pkglifecycle.SafeGo`
so a panic in any of them is recovered and logged rather than
crashing the process. Goroutines spawned from `init()` retain
their bare `go fn()` calls because `_log` is not yet constructed
at init time (will be addressed in part 4 cleanup or absorbed
by the logger migration in part 3). Closes the bug-#64 SIGSEGV
class within the local-storage process.

- Rebrand user-facing surface in the `local-storage` service: error
message returned when a referenced volume is not PowerLab-managed
now reads "PowerLab storage" (was "CasaOS storage"); legacy CasaOS
IDE-generated file headers stripped from `route/v1/storage.go`,
`model/disk.go`, `service/model/o_volume.go`; internal comments
mentioning "CasaOS UI" updated to "PowerLab UI". Dev-only path
defaults for non-Linux developer environments rebranded
(`C:\\PowerLab\\DATA`, `./PowerLab/DATA`). Structural CasaOS
dependencies (filesystem paths under `/etc/casaos` and
`/var/lib/casaos/files`, the `casaos:file:recover` message-bus
topic, the `PersistentTypeCasaOS` DB constant, and the
`cloudoauth.files.casaos.app` OAuth proxy) are deferred to #101
because each requires a deliberate migration plan rather than a
textual rename.

- Sprint 2 Kill #4 (user-service) bundled middleware + logger
migration. Wires PowerLab foundation middleware
(`pkgtracing.Middleware` for correlation IDs +
`pkglifecycle.RecoverMiddleware` for panic recovery) around the
user-service's `http.Server.Handler`. Migrates all 25 active
legacy `CasaOS-Common/utils/logger` call sites in `main.go`,
`route/event_listen.go`, `route/v1/user.go`, `service/user.go`,
and `pkg/sqlite/db.go` to PowerLab's `pkg/logging` (slog-based,
with `correlation_id` propagation from request context).
Introduces per-package `logger.go` files with
`var _log` + `SetLogger(l)` setters following the gateway pattern;
`main()` constructs the foundation logger and overrides each
package's `_log` so all log lines flow through one instance.
Drops `go.uber.org/zap` from every migrated file. The
`EventListen` goroutine now runs via `pkglifecycle.SafeGo` so a
panic in the websocket loop is recovered and logged rather than
crashing the process. `logger.LogInit` is removed entirely —
user-service no longer uses the CasaOS-Common legacy logger
outside `cmd/migration-tool` (which has its own `_logger`
package-local variable, unrelated). Structural CasaOS items
(filesystem paths, the `SERVICENAME` constant, the
`zimaos:user:save_config` event name) are deferred to #106
because they require migration plans, not text changes.

### Internal
- Sprint 2 Phase 6 (Airflow-level docs commitment, Phase 2 of three):
godoc completeness on the public symbols touched by the Sprint 2
kills. Specifically: `EventListen` (user-service event-bus loop),
`RegMsg` (local-storage event-type registration), `GetDb`
(user-service SQLite singleton), `NewUserService` (user-service
constructor + JWT keypair lifecycle), and
`MessageMergerFSNotEnabled` (local-storage merge endpoint
503 message). Each comment starts with the symbol name per Go
godoc convention and explains the non-obvious why (e.g. why
GetDb sets MaxOpenConns=1, why NewUserService deliberately
loses sessions on restart). Stale Chinese comment removed from
NewUserService.

- Stand up the Playwright E2E baseline for the SvelteKit frontend.
`ui/playwright.config.ts` boots the dev server automatically and
runs chromium on `http://localhost:5173`. `npm run test:e2e`
runs the suite locally; CI runs it via the new
`Frontend E2E (Playwright)` job. The baseline ships with one
smoke test (`tests/smoke.spec.ts`) proving the pipeline works
end-to-end. Two pre-existing specs (`auth.spec.broken.ts.txt`,
`orchestrator.spec.broken.ts.txt`) were renamed with the
`.broken.ts.txt` extension so Playwright skips them — they were
written against a UI revision predating the launchpad clock
redesign and the `/apps/new` rework, and need rewrites. Real
per-area test coverage is tracked in #108.

- Adopt changie for changelog fragment workflow (#98). Each PR adds
a tiny YAML fragment under .changes/unreleased/; release time
aggregates fragments into a single CHANGELOG section. Eliminates
the conflict-on-CHANGELOG class that consumed merge-cascade time
during Sprint 1 (~5-10 min per cascade).

- Extract the foundation middleware composition (`pkgtracing.Middleware`
+ `pkglifecycle.RecoverMiddleware`) into `pkg/foundation.Wrap`,
making it the single source of truth for how every PowerLab service
wraps its `http.Server.Handler`. Each service's `main.go` now
delegates to this helper instead of inlining the chain — closes the
bug class where four duplicated compositions could silently drift
apart. Six new unit tests in `pkg/foundation` cover the full
contract: pass-through happy path, panic-yields-500, panic log
carries `correlation_id`, happy-path correlation propagation,
pkg/errors.WriteHTTP body envelope shape, nil-logger tolerance.
This fills the integration-layer test gap surfaced during the
Sprint 2 stabilization audit (mechanical wirings without
per-service coverage). Subsequent `pkglifecycle` / `pkgtracing`
unused imports dropped from gateway / message-bus / local-storage /
user-service main.go where they are no longer referenced
directly.

- Sprint 2 Phase 5 closer — three pre-tag gates that were missing.
(1) Verified `changie batch --dry-run v0.5.0` end-to-end: all 8
unreleased fragments aggregate cleanly into a `## [v0.5.0]`
section with the right kind groupings — the workflow #98
introduced now has empirical proof of working at tag time.
(2) Added `Frontend E2E (Playwright)` to branch-protection
required status checks so the job introduced by #109 actually
gates merges instead of being a passive CI signal.
(3) Added `docs/release-checklist.md` — the authoritative
playbook for cutting a PowerLab release, split into
Phase 1 (verification, ~10min) and Phase 2 (release, ~5min)
with a separate v1.0 gate covering contract, docs site,
pkg/migrations exercise, and explicit user sign-off. All three
gaps were surfaced during the post-#111 stabilization audit.


