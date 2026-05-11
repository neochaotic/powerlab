# CasaOS residue audit — 2026-05-10

> **⚠️ Superseded by Sprints 8 + 9 (2026-05-11).** Most items
> below are closed. Read the **2026-05-11 delta** section first
> for the current state. The original Sprint 5 snapshot is
> preserved below for historical reference.

## 2026-05-11 delta — what's closed

Sprint 8 ran a **kill-list of 9 PRs that removed ~11.6 k LOC** of
CasaOS-era dead weight (#262 #263 #264 #265 #266 #267 #268 #269
#270). Sprint 9 followed with 9 more PRs (#272-#281) attacking
the cosmetic + branding residue this audit originally flagged.

### Critical-tier items — closed

| Item | Action | PR |
|---|---|---|
| `DefaultPassword = "casaos"` shipped as new-app default | Renamed to `"powerlab"`; envHelper + constants updated; TDD regression | #272 |
| `User-Agent: CasaOS` on Docker registry probes | Replaced with `PowerLab/{AppManagementVersion}` | #272 |
| Health endpoint glob `casaos*` only | Queries BOTH `casaos*` AND `powerlab-*` with dedup | #273 |
| JWT `iss="casaos"` branding leak | New tokens issued `iss="powerlab"`; bridging accept set + access-token issuer gate | #274 |
| `fstab.go` writes `.casaos.bak` / `# Added by the CasaOS` | Renamed to `.powerlab.bak` + `# Added by PowerLab` | #275 |
| Settings catalog label "CasaOS catalog" | Renamed to "Community catalog" (3 locales) | #276 |
| `v2.CasaOS` struct + `SERVICENAME = "casaos"` | Renamed to `Server` / `SERVICENAME = "powerlab"`; dead `RANW_NAME` deleted | #277 |
| `Mb.DriveModel = "Casa"` discovery fallback | Renamed to `"PowerLab"` (the most visible "pretending to be CasaOS on the LAN" residue) | Sprint 8 #265 |
| Swagger contact `@zimaboard.com` / `@icewhale.org` | Rebranded to PowerLab + GitHub URL | Sprint 8 #265 |
| "Zimaboard backers" random-name comment | Replaced with neutral description | Sprint 8 #265 |
| Orphan `RANW_NAME = "IceWhale-RemoteAccess"` constant | Deleted (zero callers) | #277 |

### Deleted entirely (Sprint 8 kill-list)

- `backend/cli/` subproject (~4 840 LOC, 61 files) — never built, CI explicitly excluded
- `backend/cli/.github/`, `casaos-cli-completion` (~95 LOC)
- `backend/core/route/v1/zerotier.go` + `route/v2/zerotier.go` + `pkg/utils/httper/zerotier.go` (~440 LOC) — VPN doesn't belong in core orchestrator
- `backend/core/route/v1/ssh.go` (`WsSsh` + `PostSshLogin`, ~113 LOC) — CasaOS SSH-to-other-host; local pty `WsShell` is preserved
- `backend/core/route/v1/file_websocket.go` (~315 LOC) — Snapdrop-style peer-broadcast; closes #261
- `backend/core/pkg/samba/` + `route/v1/samba.go` + `service/{shares,connections}.go` + Samba models (~720 LOC, 9 full-file deletes + 11 surgical edits) — Samba removed from product scope
- `cmd/migration-tool/` tree in all 6 services + orphan `MigrationTool` interfaces (~1 248 LOC) — `scripts/migrate-casaos-data.sh` covers the real migration path
- `cmd/validator/` + `cmd/message-bus-docgen/` ×4 services (~505 LOC) — Scalar + inline compose validation covers the surface
- `cmd/appfile2compose/` (~95 LOC) — AppStore is 100 % compose YAML
- 40 orphan `.github/workflows/` files in backend service dirs — GitHub Actions only honors top-level `.github`
- 5 orphan sysroot files (casaos.service unit, rclone.service, mergerfs.ctl, env stubs)
- `backend/core/Makefile` ("@echo 'call john'", refs non-existent CasaOS-UI dir)
- `notify_old.go` + `migration_0412_and_older.go` (Sprint 8 PR I)
- App-management `/v1/*` API surface (~1 365 LOC, Sprint 8 PR Q) — UI consumes only `/v2/app_management/*`

### Cosmetic + dead-code residue still standing (intentional)

- `ADR-0007`, `ADR-0021`, `ADR-0022`, `docs/audits/casaos-*`, `docs/coexistence/migrating-from-casaos.md`, `docs/architecture/casaos-strangler.md` — historical, **keep**
- `LICENSE` AGPL attribution to upstream — **keep**
- `.changes/v0.5.*.md` + root `CHANGELOG.md` release notes — **keep** (can't rewrite history)
- `README.md` single CasaOS/ZimaOS comparison line (positioning, intentional)
- `backend/app-management/common/labels.go` — `LegacyLabelKindKey/Value = "casaos"` (ADR-0021 Docker label dual-write, deliberate one-release compat — drop in v0.6.x per ADR)
- `backend/message-bus/utils/fixtures.go` — `ZimaOS*Notice` YSK onboarding cards (used in welcome flow; rebrand coordinated separately, not delete-cego)
- `backend/message-bus/pkg/ysk/adapter.go` — `ZimaIcon` const (used by fixtures above)

### Open follow-ups not in Sprint 8/9

- Sprint 7 carry-forward: `apps/+page.svelte` 1561 LOC + `settings/+page.svelte` 1469 LOC splits (#123)
- user-service v1 dead handlers (~600 LOC split-out from Sprint 8 PR Q)
- Frontend coverage baseline established 2026-05-11: 16.77 % (Sprint 9 PR #281); target ≥ 25 % Sprint 10

### Issues closed as stale during Sprint 8/9

- #249 (CLI rebrand) — moot, `backend/cli/` deleted
- #247 (test-linux-e2e casaos.service) — intentional in-container test scaffolding, not a footgun
- #261 (peer-discovery feature) — decided not to ship; file_websocket.go killed instead
- #30 (first-run onboarding tour) — never re-prioritized
- #171 (skip flag.Parse template) — niche, per-binary workaround already shipped

---

## Original Sprint 5 snapshot (2026-05-10)

Companion to `docs/audits/casaos-dependencies.md`. That doc tracks
the rolling history of the CasaOS-strip work; this one is a fresh
"what is left, what to kill, in what order" snapshot taken after
v0.5.10 ships, so Sprint 5 has a single punch-list to execute
against.

The most material change since the Sprint 3 closeout in
`casaos-dependencies.md`: **PR #151 renamed every Go module path**
from `github.com/IceWhaleTech/CasaOS-*` to
`github.com/neochaotic/powerlab/backend/*`. That was the biggest
remaining surface called out in both the Sprint 1 baseline and the
Sprint 3 update — it's done. All 9 `go.mod` files declare PowerLab
paths; zero CasaOS strings appear in any `go.mod` or `go.sum`.

What remains is mostly cosmetic, plus a small handful of items
that are still functionally tethered to CasaOS infra (one HTTP
URL, one icon CDN URL, the appstore default URL, and the legacy
`casaos = "casaos"` Docker label sentinel that ADR-0021 keeps
alive intentionally for one release window).

## Top-line summary

After 4 sprints + ~20 PRs of CasaOS-strip work, the repo still
depends on CasaOS for:

- **0** Go module imports (down from 7 services × full module
  paths in the Sprint 1 baseline; PR #151 finished the renames)
- **0** CasaOS refs in any `go.mod` or `go.sum`
- **3** runtime config defaults (`DefaultPassword="casaos"`,
  default appstore URL, `User-Agent: CasaOS` on registry probes)
- **2** upstream-hosted URLs called at runtime
  (`get.casaos.io/update`, `icon.casaos.io/main/all/*.png`) plus
  **1** as the default appstore source
  (`cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages`)
- **0** `@FilePath`/`@Author LinkLeong`/`@Website casaos` markers
  in Go files (down from ~40 — PR #194 swept the headers)
- **6** legacy markers in shell scripts under
  `backend/core/build/scripts/setup/...` and
  `backend/core/build/sysroot/usr/share/casaos/...`
- **3** inherited `*.md` files in `backend/core/`
  (`CHANGELOG.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md` — all
  point at `wiki@casaos.io` for security contact)
- **1** stale unused config sample (`backend/core/conf/conf.conf.sample`
  — the only remaining CasaOS-shaped sample; no install path
  references it)
- **1** sysroot directory still under `etc/casaos/`
  (`backend/gateway/build/sysroot/etc/casaos/gateway.ini.sample`)
- **8** sysroot systemd unit files still named `casaos-*.service`
  (one per service + buildroot variants), used by package-linux.sh
  for the Linux tarball
- **22** SDK/test-fixture occurrences in `backend/cli/` (every
  command file's "Copyright IceWhaleTech" header + the
  `casaos-cli` binary name + `BasePathCasaOS = "v2/casaos"`
  routing constant). The CLI is the least-touched service.

Estimated effort to eliminate the rest:
- **~17h, 10 separate PRs** to kill the cosmetic + functional
  residue (excluding the CasaOS appstore URL, which requires
  hosting our own mirror — separate decision).
- **+ a follow-up "stop dual-writing legacy Docker labels" PR
  one release window after Sprint 5**, per ADR-0021.

## 1. Go module dependencies

**Status: zero remaining.** PR #151 renamed every module path.
Verification:

```
grep -rin 'IceWhaleTech\|icewhale' backend/*/go.mod backend/*/go.sum
# → no output
```

| Service          | `go.mod` declares                                       |
|------------------|---------------------------------------------------------|
| `gateway`        | `github.com/neochaotic/powerlab/backend/gateway`        |
| `message-bus`    | `github.com/neochaotic/powerlab/backend/message-bus`    |
| `core`           | `github.com/neochaotic/powerlab/backend/core`           |
| `user-service`   | `github.com/neochaotic/powerlab/backend/user-service`   |
| `local-storage`  | `github.com/neochaotic/powerlab/backend/local-storage`  |
| `app-management` | `github.com/neochaotic/powerlab/backend/app-management` |
| `common`         | `github.com/neochaotic/powerlab/backend/common`         |
| `cli`            | `github.com/neochaotic/powerlab/backend/cli`            |
| `pkg`            | `github.com/neochaotic/powerlab/backend/pkg`            |

`scripts/check-package-linux-ldflags_test.sh:59` actively asserts
`-X github.com/IceWhaleTech/CasaOS/common.POWERLAB_VERSION=` is
absent from `package-linux.sh` — i.e. the rename is regression-locked.

**Recommended action:** none. Section retained so a future audit
doesn't re-investigate.

## 2. Runtime CasaOS-branded code

Categorized **functional / cosmetic / sentinel** per the audit
brief. Tests, codegen output, fixture files, and migration tools
(which intentionally read CasaOS-era paths) are excluded.

### Functional — runtime behavior depends on the CasaOS string

| Location | Why it is functional | Recommended action |
|----------|---------------------|--------------------|
| `backend/app-management/common/constants.go:35` `DefaultPassword = "casaos"` | Compose YAML pre-fill: store apps that ship `password: $DefaultPassword` get `"casaos"` substituted by `pkg/utils/envHelper/env.go:9`. Changing it changes the install-time default password for every new app install. | Rename to `DefaultPassword = "powerlab"` AND add a release-note entry calling out the password change. Junior risk: catches ALL existing store apps, so include a migration note for upgraders. |
| `backend/app-management/pkg/utils/envHelper/env.go:9` `temp = "casaos"` | Same default-password substitution path — duplicate of the above constant by literal value, NOT by reference. | Refactor to read `common.DefaultPassword` instead of duplicating the literal. After refactor, only the constant has to change. |
| `backend/app-management/service/container.go:302` `icon = "https://icon.casaos.io/main/all/" + ... + ".png"` | Runtime HTTP — when an "origin=system" container has no Docker label icon, `GetContainerAppList` synthesizes one from the icon CDN at icon.casaos.io. We don't host an alternative. | Either point at our own icon CDN, or make this a fallback PNG embedded in the binary. Requires UI design call (do we ever want a system-container icon?). |
| `backend/app-management/pkg/docker/auth.go:74` `req.Header.Set("User-Agent", "CasaOS")` | Sent on every Docker registry challenge probe. Not strictly broken if changed, but downstream registries that gate by UA (DockerHub does NOT, but private registries might) could behave differently. | Change to `"PowerLab"` after a Docker Hub + GHCR + Quay smoke test in CI. Low risk. |
| `backend/core/service/system.go:397` `curl -fsSL https://get.casaos.io/update?t=...` | Fallback for the legacy `/v1/sys/update` route when `config.ServerInfo.UpdateUrl` is empty. Spawns a shell on the host that pipes a remote install script — historically how CasaOS auto-updated. We have our own updater (`/v1/powerlab-update`) but the legacy route still exists and is reachable. | Either delete the fallback (preferred — `/v1/powerlab-update` is the canonical path) or refuse to call out to casaos.io and emit a structured error. The current code is a remote-code-execution hazard if `config.ServerInfo.UpdateUrl` is ever blanked accidentally. **High priority.** |
| `backend/core/service/health.go:16` `systemctl.ListServices("casaos*")` | Health endpoint reports running services matching `casaos*` glob. After v0.5.x our units are `powerlab-*.service` — the health endpoint silently reports nothing of ours. | Change glob to `powerlab*` (and keep `casaos*` if we want to surface co-resident CasaOS services on a coexist install). |
| `backend/local-storage/pkg/fstab/fstab.go:78,88,97,119` writes `.casaos.bak` / `.casaos.new` backup files + `# Added by the CasaOS` fstab comment | Files persisted in `/etc/fstab.casaos.bak` etc. on disk. Renaming bricks any rollback path that grep'ed for the old comment. | Rename to `.powerlab.*` AND keep reading `.casaos.*` as a fallback for one release. |
| `backend/local-storage/model/disk.go:89` `PersistedIn = "casaos"` literal in a JSON wire field comment | Wire field returns `"casaos"` for disks persisted by old code. Sprint 3 PR #142 already added `PersistentTypePowerLab` to write the new value; the legacy literal is on the read path for back-compat. | Add a one-shot migration that rewrites `"casaos"` → `"powerlab"` in stored rows, then drop the dual-read after the next release. |
| `backend/common/utils/jwt/jwt.go:55` `GenerateToken(..., "casaos", 3*time.Hour)` | The `aud` (audience) claim of every JWT. Existing in-memory + persisted tokens have `aud=casaos`; PR #20-style validation that compares `aud` would break on rename. | Change to `"powerlab"` AND keep accepting `"casaos"` on validation for one release (token TTL = 3h, so one release window covers all live sessions). |
| `backend/common/utils/version/version.go:16-23` `LegacyCasaOSServiceName = "casaos.service"`, `LegacyCasaOSConfigFilePath = "/etc/casaos.conf"` | Read by version-detection logic on a co-resident host. Per the Sprint 1 baseline + the "What's NOT debt despite the name" list in `casaos-dependencies.md`, this is intentional legacy CasaOS interop. | Keep. Document in the audit's "What we should KEEP" section. |
| `backend/common/external/notify.go:16` `CasaOSURLFilename = "casaos.url"` + `backend/core/main.go:240` writes `casaos.url` | Unix socket address handoff between `core` and `gateway` — gateway reads from `<runtimePath>/casaos.url`. Filename is internal but persisted on disk. | Rename to `core.url` AND keep reading `casaos.url` as a fallback for one release. Mechanical change. |

### Cosmetic — string in log message, comment, or var name

These are runtime-irrelevant (logs, error messages, internal var
names, inline TODO/`@Website` comments). Rebrand-only.

| Location | Type | Recommended action |
|----------|------|--------------------|
| `backend/app-management/main.go:170,172` `"...notify systemd that casaos main service is ready"` | Log message | One-line edit |
| `backend/core/main.go:253,255,271` `"casaos main service ready"` / `"CasaOS main service is listening..."` / `@title casaOS API` | Log message + swagger title | One-line edits |
| `backend/core/route/v2/route.go:8-13` `type CasaOS struct {}` and `NewCasaOS()` constructor | Type name | Rename to `Core` or `CoreServer`; receivers in v2/{file,health,zerotier}.go follow |
| `backend/core/route/v2/{file,health,zerotier}.go` — every method receiver `(s *CasaOS)` / `(c *CasaOS)` | Method receiver | Sed after the type rename |
| `backend/core/route/v1/system.go:42-169` — `GetCasaOSErrorLogs`, `PostKillCasaOS`, `GetCasaOSPort`, `PutCasaOSPort` | Function names | Rename + update OpenAPI spec |
| `backend/core/route/v1.go:95,99,109,110` route registration | Route handler refs | Follows the rename above |
| `backend/core/service/casa.go` — `casaService.GetCasaosVersion()` | Service interface method | Rename to `GetVersion()` |
| `backend/core/model/heart.go:3` `type CasaOSHeart struct` | Model type name | Rename to `Heartbeat` |
| `backend/core/common/constants.go:4-7` `SERVICENAME = "casaos"`, `RANW_NAME = "IceWhale-RemoteAccess"` | String constant | Rename to `"powerlab"` |
| `backend/core/common/message.go:10` (comment about old `casaos:file:recover` topic) | Comment | Edit |
| `backend/core/pkg/sqlite/db.go:16-40` — DB filename `casaOS.db` | SQLite filename — see #3 below | Already split-brain-handled by `paths.LegacyCasaOSCoreDB()`. Rename target file to `core.db`, write migration that copies on first boot. |
| `backend/core/service/shares.go:71,92,97,128` — Samba banner block (`# Copyright (c) 2021-2022 CasaOS Inc.` ASCII-art block) | Persisted to `/etc/samba/smb.conf` on first run | Rewrite the banner with PowerLab attribution. |
| `backend/core/service/system.go:50,82,84,116,125,133,162,297,386` — log lines, default Windows path `C:\\CasaOS\\DATA`, `keyName := "casa_version"` cache key | Mixed — some are log-only, the Windows path is a default for a config the UI exposes | Edits |
| `backend/core/route/v1/file.go:472`, `backend/core/route/v1/samba.go:37`, `backend/core/route/v1/powerlab_update.go:139`, `backend/core/route/v2/health.go`, `backend/core/route/v1.go:50,121` | Comments | Edits |
| `backend/message-bus/service/ysk.go:85,90,91`, `backend/message-bus/utils/fixtures.go:41,42,68,69,95,96,160,161`, `backend/message-bus/pkg/ysk/adapter.go:23` | YSK sidebar payloads — `casaos-ui:*` event keys + `/modules/icewhale_files/*` icon paths | These names cross the wire to the UI. Rename requires UI side too. **Defer until v1.0 wire-format consolidation.** |
| `backend/local-storage/service/disk.go:69-72` (comment about wire-format const) | Comment | Edit |
| `backend/local-storage/service/v2/fs/mergerfs.go:14` (comment) | Comment | Edit |
| `backend/local-storage/route/v1.go:82-85` (comment about cloudoauth.files.casaos.app) | Comment | Edit |
| `backend/local-storage/main.go:116`, `backend/user-service/main.go:87`, `backend/user-service/common/version.go:8-10`, `backend/message-bus/main.go:48,90`, `backend/gateway/main.go:40,149`, `backend/gateway/route/logger.go:15`, `backend/gateway/service/logger.go:14`, `backend/pkg/errors/doc.go:4` | Comments referencing the CasaOS-strip work itself | Keep as historical context (intentional per the audit format) |
| `backend/common/utils/random/random.go:163` (comment) | Comment | Edit |
| `backend/cli/` — every file under `cmd/` carries `Copyright © 202X IceWhaleTech` + `casaos-cli` binary name + `Short: "...CasaOS..."` cobra descriptions | Cosmetic (the binary builds + runs fine) but USER-FACING — it's the literal command line | Full CLI rebrand. ~22 files. Rename binary to `powerlab-cli`. Sed the Copyright headers. |

### Sentinel — legacy backward-compat (intentional)

These exist BECAUSE removing them breaks something. ADR-0021
documents the contract.

| Location | Why it is intentional | When can it go |
|----------|----------------------|---------------|
| `backend/app-management/common/labels.go:47-63` `LegacyLabelKindKey = "casaos"`, `LegacyLabelKindValueApp = "casaos"`, `LegacyLabelOriginKey = "origin"`, `LegacyLabelIconKey = "icon"`, etc. | Per ADR-0021, every container PowerLab creates dual-writes BOTH `io.powerlab.v1.*` and the legacy unnamespaced labels. Required during one release window so existing containers stay recognized by `IsPowerLabApp`. | After one release post-Sprint 5: drop `BuildLabels`'s legacy writes, keep `IsPowerLabApp` + `LabelValue`'s legacy reads for one further release. |
| `backend/app-management/common/labels.go:69-72` (comment about the dual-write) | Documentation of the contract. | Keep until both writes AND reads of legacy labels are dropped. |
| `backend/app-management/common/appdata.go:9-21` `LegacyAppDataDirName = "AppData"` | Per ADR-0021, PowerLab moved from `<StoragePath>/AppData/<app>` to `<StoragePath>/PowerLabAppData/<app>`. Legacy const used by the migration logic. | Keep until a hypothetical future audit confirms no install still has `AppData/` populated; effectively forever. |
| `backend/app-management/common/constants.go:9-25` `ComposeAppAuthorCasaOSTeam = "CasaOS Team"`, `ComposeExtensionNameXCasaOS = "x-casaos"` | Author-string discriminator for the upstream CasaOS app store; extension-name fallback for store apps that ship `x-casaos:` (most of them). | Keep — these read upstream data. |
| `backend/app-management/service/extension.go:5-15` extension priority list `[x-powerlab, x-web, x-casaos]` | Resolution order for compose extensions. | Keep. |
| `backend/app-management/service/compose_app.go:170-171` `if storeInfo.Author == ComposeAppAuthorCasaOSTeam { return codegen.ByCasaos }` | Wire-format identifier exposed via `/v2/app_management/compose` — UI uses it to badge "by CasaOS" vs "official" vs "community". | Keep until wire-format consolidation in v1.0. |
| `backend/app-management/route/v1.go:66`, `route/v1/docker.go:481` (comments about legacy `casaos_apps` JSON key) | Comments documenting the v1 wire format for back-compat. | Keep. |
| `backend/app-management/route/v2/appstore.go:36` `// but it be used by CasaOS` (deprecation guard) | Comment guarding a route that the upstream CasaOS UI still calls when a coexist install talks to PowerLab. | Keep until coexist support drops. |
| `backend/common/utils/version/version.go:16-23` `LegacyCasaOSServiceName`, `LegacyCasaOSConfigFilePath`, `_casaOSBinFilePath` | CasaOS-co-resident version-detection (per Sprint 1 baseline). | Keep. |
| `backend/common/utils/version/migration.go:16` `GlobalMigrationStatusDirPath = "/var/lib/casaos/migration"` | CasaOS → PowerLab migration tool. | Keep — Sprint 1 baseline marked it intentional. |
| `backend/common/utils/constants/paths.go:34` and `backend/common/utils/devmode/devmode.go:28` `"/etc/casaos"` in `devProductionMarkers` | Detects a CasaOS host so dev-mode doesn't rewrite paths. | Keep. |
| `backend/common/utils/paths/db.go:99-109` `LegacyCasaOSCoreDB() = "/var/lib/casaos/db/casaOS.db"` | Split-brain detection in `core/main.go`. Refuses to start if both old and new core DB exist. | Keep until canonical-rename PR + a release of in-place migrations have settled. |
| `backend/cli/cmd/migration-tool/...` (every backend/`<svc>/cmd/migration-tool/`) | One-shot CasaOS-conf reader that converts pre-PowerLab installs. | Keep — explicit migration tool. |

## 3. CasaOS-flavored URLs

| URL | Where used | Functional? | Alternative status | Recommended action |
|-----|-----------|-------------|--------------------|--------------------|
| `https://get.casaos.io/update?t=<MANUFACTURER>` | `backend/core/service/system.go:397` (UpdateSystemVersion fallback) | **YES — runtime curl pipe to bash** | We have `/v1/powerlab-update` (issue #21) | **Highest URL kill priority.** Either delete the fallback or replace with a structured "no update URL configured" error. |
| `https://icon.casaos.io/main/all/<image>.png` | `backend/app-management/service/container.go:302` (system-container icon synth) | **YES — runtime HTTP** | None (we have no icon CDN) | Embed a generic system-container PNG in the binary, OR drop the icon and let the UI render a placeholder. |
| `https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip` | `scripts/package-linux.sh:177`, `start.sh:183`, `backend/app-management/build/sysroot/etc/powerlab/app-management.conf.sample:12` | **YES — default appstore in shipped conf** | None (we'd need to host our own mirror) | **Decision required.** Options: (a) keep as-is — community appstore is a feature; (b) mirror the IceWhaleTech repo to a `powerlab/AppStore` org and pin version; (c) host on our own CDN. Per `casaos-dependencies.md` Sprint 3 closeout this was deliberately kept. **Recommend keeping for Sprint 5 + revisiting pre-v1.0.** |
| `https://github.com/bigbeartechworld/big-bear-casaos/...` | `scripts/package-linux.sh`, `start.sh:184`, `backend/app-management/build/sysroot/etc/powerlab/app-management.conf.sample:13`, UI `settings/+page.svelte:790` | YES — default secondary appstore | Community-maintained catalog. Repo name predates PowerLab. | Keep — this is a third-party data source whose name happens to contain "casaos". Already documented as intentional in `casaos-dependencies.md`. |
| `https://api.casaos.io/casaos-api`, `https://socket.casaos.io` | `backend/core/conf/conf.conf.sample:17,18` | NO — sample is unused | n/a | **Delete the unused sample file** (see #4). |
| `https://www.casaos.io` (link) | `ui/src/routes/settings/+page.svelte:1246` (the "Powered by CasaOS" attribution link) | UI link, AGPL attribution | n/a | Keep — required AGPL attribution to the upstream project (see Sprint 1 baseline + ADR-0025). |
| `https://raw.githubusercontent.com/IceWhaleTech/CasaOS-MessageBus/...openapi.yaml` | `//go:generate` directives in 5 services' `main.go` | Codegen-time only | n/a | Either commit our own copy of the MessageBus OpenAPI spec under `backend/common/api/` and point `//go:generate` at the local file, OR mirror the spec and rewrite. **Medium priority** — every fresh codegen run today reaches out to upstream IceWhaleTech. |
| `https://github.com/IceWhaleTech/CasaOS-AppManagement/...` (in `.goreleaser.yaml` cmd-build instructions) | `backend/app-management/.goreleaser.yaml`, `.goreleaser.debug.yaml` | Build-time, fetches `appfile2compose` source | n/a | These goreleaser files are not used by `package-linux.sh` (the actual build path) — they're inherited from upstream. Delete or rewrite to point at our copy of `appfile2compose`. |
| `wiki@casaos.io` (security contact) | `backend/core/SECURITY.md:9`, `backend/core/CODE_OF_CONDUCT.md:63` | Doc | We have our own SECURITY.md at root + CODE_OF_CONDUCT.md | **Delete** the inherited copies under `backend/core/`. |

## 4. Configs + defaults

| Config / file | Current value | Impact if changed | Recommended action |
|---------------|--------------|-------------------|--------------------|
| `backend/app-management/common/constants.go:35` `DefaultPassword = "casaos"` | Substituted into compose YAML's `$DefaultPassword` env vars | Every newly installed app gets the new password as its initial admin password. Existing apps unaffected (already-baked containers don't re-substitute). | **Change to `"powerlab"`** + release-note callout. Add a regression test that asserts the constant. |
| `backend/app-management/build/sysroot/etc/powerlab/app-management.conf.sample:12-13` `appstore = https://cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip` + big-bear | Default app sources baked into the shipped tarball | Removing them deletes the only reason a fresh install has anything to browse. | Keep both (decision per #3). |
| `backend/gateway/build/sysroot/etc/casaos/gateway.ini.sample` (path itself) + contents (`runtimepath=/var/run/casaos`) | Embedded into the gateway binary at compile time via `//go:embed`; written to `/etc/powerlab/gateway.ini` on first run by install.sh | If install.sh's `[ -f ]`-guarded write skipped a real CasaOS-era `/etc/powerlab/gateway.ini`, the runtime would point at `/var/run/casaos`. Today install.sh does the right thing (see migrate-casaos-data.sh). | **Move file to `backend/gateway/build/sysroot/etc/powerlab/gateway.ini.sample`**, fix `//go:embed` directive in `gateway/main.go:80` and `cmd/migration-tool/main.go:19`, change `runtimepath=/var/run/powerlab`. Mechanical. |
| `backend/core/conf/conf.conf.sample` (entire file) | Stale CasaOS-shaped sample | Nothing references it (verified by `grep -rn 'conf.conf.sample\|conf/conf' backend/`) | **Delete the file.** |
| `backend/gateway/common/config.go:43` `os.LookupEnv("CASAOS_CONFIG_PATH")` | Honoured env var on startup | Existing operators with `CASAOS_CONFIG_PATH` set in their systemd unit would lose the override | Add `POWERLAB_CONFIG_PATH` AND keep reading `CASAOS_CONFIG_PATH` as a fallback for one release. |
| `backend/cli/cmd/root.go:30,38,41,54` `BasePathCasaOS = "v2/casaos"`, `GatewayPath = "/etc/casaos/gateway.ini"`, `RootGroupID = "casaos-cli"`, `Use: "casaos-cli"` | CLI binary name + default gateway config path + Cobra group ID | Existing CLI-using operators run `casaos-cli ...` from muscle memory; pkg installers ship `/etc/casaos/gateway.ini`. | Rename to `powerlab-cli` AND ship a shim symlink for `casaos-cli` → `powerlab-cli` for one release. Update `GatewayPath` to read both `/etc/powerlab/gateway.ini` and `/etc/casaos/gateway.ini`. |
| `backend/core/api/casaos/openapi.yaml` (path) | Embedded via `//go:embed` in `core/main.go:50` and referenced by `package.json` codegen scripts | Renaming the directory breaks every reference; needs simultaneous update of `main.go`, the v2 server URL prefix `/v2/casaos`, and frontend `ui/src/lib/api/endpoints.ts` constants `HEALTH_SERVICES = "/v2/casaos/health/services"` etc. | **Coordinate with v1.0 wire-format change.** Renaming the route prefix is a breaking API change — defer until the v1.0 contract freeze. |
| `backend/gateway/build/sysroot/usr/lib/systemd/system/casaos-gateway.service` and 6 sibling files | Systemd unit filenames shipped in the tarball | If renamed without an alias, `systemctl start casaos-gateway` (which install.sh and ops docs use) breaks. | Rename to `powerlab-gateway.service` etc. AND ship `Alias=casaos-gateway.service` in the `[Install]` section. **Verify install.sh stops the old units before re-enabling.** |

## 5. Docs / READMEs / license headers

### Markers in Go (already cleaned)

`grep -rn '@FilePath\|@Author.*LinkLeong\|@Website.*casaos' backend/ --include='*.go'` → **0 matches.** PR #194 swept these.

### Markers still present in shell scripts

| File | Action |
|------|--------|
| `backend/core/build/scripts/setup/service.d/casaos/debian/setup-casaos.sh` | Sysroot setup script, uses `# @Website: https://www.casaos.io`. Not invoked by `install.sh`. **Delete or rewrite** as part of sysroot rename. |
| `backend/core/build/scripts/setup/service.d/casaos/arch/setup-casaos.sh` | Same as above. |
| `backend/core/build/sysroot/usr/share/casaos/cleanup/script.d/03-cleanup-casaos.sh` | Cleanup helper invoked by uninstall. Rename + update header. |
| `backend/core/build/sysroot/usr/share/casaos/shell/{update,delete-old-service,helper,assist}.sh` | Helper scripts, some have `@Author: LinkLeong`. Audit which are still invoked by core's runtime — anything orphan can be deleted; the rest get header-edited. |
| `backend/core/build/sysroot/usr/share/casaos/shell/usb-mount{.sh,.service}` | USB auto-mount. Used. Header-only edit. |
| `backend/data/appstore/github.com/.../big-bear-casaos-master/default-icon.svg` | Third-party content. Keep. |

### Inherited READMEs / module docs

| File | Action |
|------|--------|
| `backend/core/CHANGELOG.md` | Inherited from upstream CasaOS. **Delete** — we have a root `CHANGELOG.md` generated by changie. |
| `backend/core/CODE_OF_CONDUCT.md` | Inherited (`wiki@casaos.io`). **Delete or replace** with PowerLab CODE_OF_CONDUCT at the repo root. |
| `backend/core/SECURITY.md` | Inherited (`wiki@casaos.io`). **Delete**; security policy is at the repo root. |
| `backend/app-management/package.json:12` `homepage: https://github.com/IceWhaleTech/CasaOS-AppManagement#readme` | Edit to point at PowerLab repo. |

### Docs that should stay

| File | Why |
|------|-----|
| `docs/architecture/casaos-strangler.md` | ADR-context architecture doc. |
| `docs/coexistence/migrating-from-casaos.md` | User-facing migration guide for CasaOS users — must reference CasaOS by name. |
| `docs/audits/casaos-dependencies.md` (this doc's companion) | Living history of the strip work. |
| `docs/decisions/0011-backend-pkg-coexistence-with-casaos-common.md`, `0021-docker-label-namespace-and-appdata-path.md` | ADRs documenting why the legacy surface exists. |
| Every `.changes/v0.5.X.md` and the root `CHANGELOG.md` | Release notes. Cannot edit history. |

## Recommendation: kill order for Sprint 5

Ordered by **leverage / risk ratio** (high-leverage low-risk first).
Each numbered item is a separate PR.

1. **Delete the orphan `backend/core/conf/conf.conf.sample`.** Zero
   references in the repo (verified). Pure win. **30 min.**

2. **Delete inherited `backend/core/{CHANGELOG,CODE_OF_CONDUCT,SECURITY}.md`.**
   Repo root has the canonical versions. **30 min.**

3. **Kill the `get.casaos.io/update` fallback in
   `backend/core/service/system.go:397`.** Replace with a
   structured `"no update URL configured"` error. The legacy
   `/v1/sys/update` endpoint stays but stops being a remote-code-execution
   hazard. **2h** including a regression test that the fallback
   path returns an error instead of executing curl. **HIGH PRIORITY** —
   security-shaped.

4. **Move `backend/gateway/build/sysroot/etc/casaos/gateway.ini.sample`
   to `etc/powerlab/`** + update the two `//go:embed` directives
   + change `runtimepath=/var/run/powerlab`. Mechanical, regression
   test exists (the gateway boot test). **1h.**

5. **Sed cosmetic-only cleanups in core + message-bus + local-storage:**
   log strings ("CasaOS main service is listening" etc.), comments
   about CasaOS, the `CasaOSHeart` type, `casaService.GetCasaosVersion()`
   method, `SERVICENAME = "casaos"` constant, `keyName = "casa_version"`,
   the `C:\\CasaOS\\DATA` Windows default. ~30 occurrences across
   ~10 files. **2h** including running the full backend test suite.

6. **Rebrand the CLI** (`backend/cli/`). Rename binary to
   `powerlab-cli`, rebrand all 22 cobra command files' Copyright
   headers + Short descriptions, change `BasePathCasaOS` to a
   versioned constant, ship `casaos-cli` symlink for one release.
   Largest single PR by LOC touched, mostly mechanical. **3h.**

7. **JWT audience claim rename + `casaos.url` filename rename
   + Docker auth User-Agent rename + health endpoint glob fix**
   (one bundled PR — all cross-service "rename + dual-read for
   one release" changes). Cluster of small surgical edits with
   regression tests. **3h.**

8. **Rename systemd units `casaos-*.service` → `powerlab-*.service`**
   in the sysroot tree, add `Alias=` directive for back-compat,
   update install.sh stop-services list, run E2E. **2h.**

9. **Replace `icon.casaos.io` icon synth** in
   `backend/app-management/service/container.go:302` — embed a
   generic system-container icon in the UI bundle and stop
   making the runtime HTTP call. Requires UI design call. **1h.**

10. **Vendor the upstream OpenAPI specs** (CasaOS-MessageBus,
    CasaOS-AppManagement, CasaOS-LocalStorage, CasaOS-UserService,
    CasaOS) under `backend/common/api/upstream/` and rewrite the
    `//go:generate` directives to point at the local copies.
    Stops `go generate ./...` from reaching out to IceWhaleTech.
    **2h.**

11. **`DefaultPassword = "casaos"` rename** — needs release-note
    callout + a feature-flag for one release so existing users
    aren't blindsided. **Defer to Sprint 5.5.**

12. **Drop the legacy Docker label dual-WRITE** (per ADR-0021,
    one release window after Sprint 5 ships). Keeps reads. **1h
    once the window is up.**

**Sprint 5 deliverable scope:** items 1–10 (~17h, 10 PRs). Items
11–12 are follow-ups with explicit user-facing release notes.

**Out of scope for Sprint 5:**
- The CasaOS appstore URL (decision pending).
- The `casa.go` v2 route prefix `/v2/casaos` (deferred to v1.0
  wire-format freeze).
- The `x-casaos` compose extension fallback (data interop).
- The `casaos.oss-cn-shanghai.aliyuncs.com` reference under
  `backend/data/appstore/` (third-party data).

## What we should KEEP from CasaOS forever

These items are CORRECT to remain CasaOS-shaped and any future
audit should not flag them:

- **`backend/data/appstore/cdn.jsdelivr.net/.../casaos.oss-cn-shanghai*`**
  — third-party app store catalog data, not our code.
- **The v0 / v1 wire-format `casaos = "casaos"` Docker label** —
  read for back-compat per ADR-0021.
- **The `x-casaos` compose extension key** — most upstream store
  apps still ship it; we read it via the priority list.
- **`ComposeAppAuthorCasaOSTeam = "CasaOS Team"`** — discriminator
  for upstream-authored apps, displayed in the UI.
- **`backend/common/utils/version/{version,migration}.go`'s
  Legacy CasaOS detection** — lets a co-resident host be detected
  for graceful coexist.
- **`/etc/casaos` in `devProductionMarkers`** — production-host
  detection.
- **The `Powered by CasaOS` link in the SvelteKit settings page** —
  AGPL attribution to the upstream project (per ADR-0025 / Sprint
  1 baseline).
- **`backend/<svc>/cmd/migration-tool/...`** — explicitly
  CasaOS-shaped because their job is to read CasaOS-era state and
  migrate it.
- **`docs/architecture/casaos-strangler.md`,
  `docs/coexistence/migrating-from-casaos.md`,
  `docs/decisions/0011-*` and `0021-*`, this audit's companion
  `docs/audits/casaos-dependencies.md`** — documentation that
  describes the relationship.
- **The `big-bear-casaos` community catalog reference** — repo
  name predates PowerLab; we just consume the data.

## Reference

- Companion: `docs/audits/casaos-dependencies.md` (rolling history)
- Sprint 4 retro: `docs/audits/sprint-4-retrospective.md`
- ADR-0025: `backend/pkg` coexistence with `backend/common`
- ADR-0019: Tech debt tracked in audits + ADRs + issues
- ADR-0021: Docker label namespace + AppData path migration
- PR #151: Module path rename (the surface this audit can now
  declare done)
- PR #194: License header sweep
