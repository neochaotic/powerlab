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

## [v0.7.4] — 2026-05-26
### Added
- CI: an automated install→upgrade regression gate (`upgrade-smoke` job, runs scripts/check-upgrade-smoke.sh). It installs the latest published release, upgrades in place to the build under test, and asserts post-upgrade health — gateway responds, version stamp moved, no failed units, embedded UI served + stale on-disk UI removed. Runs on main pushes, tags, and manual dispatch, and gates the GitHub release. The version-update safety net PowerLab lacked.
- Release tooling: `prepare-release.sh` now refuses to cut a version number that is already a published GitHub release (`scripts/check-version-not-released.sh`). Prevents the v0.7.2 collision class — where a published, immutable version kept accruing content via hand-edited changelog instead of bumping to the next number. The changie "version already exists" refusal is the same signal; this makes it impossible to work around silently.
- CI: a per-module Go coverage no-regression ratchet (`scripts/check-coverage-ratchet.sh` + `scripts/coverage-baseline.tsv`), wired into the backend matrix. Each module fails the build if its coverage drops below a committed floor; floors are bumped up as coverage climbs. Locks the coverage already earned without chasing an unrealistic absolute target (per the coverage-cadence policy — ratchet pure-logic surfaces, no mock teatro).
- Test coverage: core pure-logic packages now have unit suites — common_err.GetMsg (known-code table + service-error fallback), utils.DefaultQuery/DefaultPostForm (echo httptest), cache.Init round-trip, and ip_helper.GetDeviceAllIPv4 (non-nil + IPv4-shape). Lifts core from 12.0% to 17.8% on hand-written code (no mock teatro). The remaining gap is route/v1 HTTP glue, which belongs to integration tests, not unit coverage.
- CI: the install/upgrade smoke is now a matrix across amd64 AND arm64 (GitHub native arm64 runner), with two scenarios — `fresh` (fresh-install the build + assert boot health, closes #509) and `upgrade-from-www` (install the last www-model release v0.7.3, upgrade in place, assert the on-disk www is removed and the gateway serves the EMBEDDED UI — the ADR-0043 migration guard). check-upgrade-smoke.sh gains a --fresh mode. Gates the release.
### Changed
- Gateway: the web UI is now embedded into the gateway binary via go:embed (ADR-0043) instead of being read from /usr/share/powerlab/www at runtime. The on-disk `-w <dir>` path still wins when the directory exists (dev override + the current install layout), so this is a no-op for existing installs; the embedded copy is the fallback. Removes the UI-vs-binary version-skew bug class at the root.
- Gateway: the web UI is now served exclusively from the embedded bundle in production (ADR-0043 phase 3). The release tarball no longer ships /usr/share/powerlab/www, the systemd unit drops the `-w` flag, and install.sh removes any stale on-disk UI from a prior install so an upgraded gateway serves its embedded copy (no version skew). The `-w <dir>` flag still works as a dev override. Adds scripts/check-upgrade-smoke.sh — a reusable automated install→upgrade regression gate.
### Security
- Bumped golang.org/x/net to v0.55.0 across all backend modules to clear GO-2026-5026 (CVE-2026-39821 — idna Punycode label rejection), a reachable vuln. Several modules were still on v0.25.0/v0.53.0; the prior fix had only landed on an unmerged branch.
- Bumped github.com/hashicorp/go-getter v1.7.0 → v1.7.9 in app-management to clear its CVE patch series. (The stale CVE branch also pinned protobuf to v1.33.0, which would now force a k8s downgrade — main already carries a patched protobuf v1.36.x, so only go-getter needed the bump.)
- Bumped golang.org/x/image v0.6.0 → v0.41.0 (core) and github.com/getkin/kin-openapi v0.114–0.117 → v0.138.0 across app-management, core, message-bus, local-storage, user-service — clears the x/image decode CVEs and GO-2025-3533 (kin-openapi). Re-lands the fixes that were stranded on the deps/x-image-cve branch.
- Bumped github.com/rclone/rclone v1.62.2 → v1.73.5 in local-storage to clear its CVE series (#525). The rclone v1.73 config-type changes (AttrTimeout/DaemonTimeout/DirPerms/FilePerms) were already accommodated by forward-compatible casts in pkg/mount; only the module bump was missing. Verified pkg/mount cross-compiles for linux against v1.73.5.


## [v0.7.3] — 2026-05-23
### Added
- Test coverage: added a unit suite for core/pkg/utils/file (GetExt, GetBlockInfo, MD5 hash helpers, PrefixLength/DataLength fixed-width encoders, CommonPrefix, and tempdir round-trips for MkDir/RMDir/Exists/IsDir/IsFile/CreateFile/ReadFullFile/dir-size) — 0.7% → 22.5%. Surfaced #533 (file.RemoveAll does not recurse into subdirectories).
- Test coverage: added an adversarial table test for common/pkg/security.ShouldIncludeIP (cert-SAN IP classification, ADR-0001) — 0% → 100% on the function. Locks the RFC1918 / CGNAT-mesh-VPN / IPv6-ULA boundaries (172.16–31, 100.64–127, fc/fd vs fe80) where a misclassification would leak or omit a cert SAN entry. No bug found; behavior confirmed correct.
- Test coverage: added a unit suite for common/utils/file (GetExt, CommonPrefix, and tempdir round-trips for MkDir/IsNotExistMkDir/RMDir/Exists/IsDir/IsFile/CreateFile/CreateFileAndWriteContent/ReadFullFile/GetFileOrDirSize) — 8.3% → 26.6%. Pure-logic + filesystem helpers; no mock teatro.
- Test coverage: added a unit suite for common/utils (CompareSlices, CompareStringSlices, Ptr, and Throttle immediate+coalescing paths) — 0% → 47.1%. DelayedExecutor left uncovered on purpose: it is dead code (no callers) with a time-unit bug (see #537), and locking the buggy behavior under test would be wrong.
- Test coverage: completed the unit suite for app-management/pkg/utils/envHelper (all ReplaceDefaultENV keys — password/username/PUID/PGID/TZ + unknown-key fallthrough — plus ReplaceStringDefaultENV passthrough/idempotence) — 66.7% → 100%. Locks the install-time env-substitution logic, including that the legacy upstream literal never leaks.
- Test coverage: locked two pure 0% core packages — pkg/utils/encryption (GetMD5ByStr against RFC 1321 vectors + determinism/length) and service/model (GORM TableName mappings o_notify/o_rely) — both 0% → 100%.
### Changed
- Test coverage: added pure-logic unit tests for app-management/service — `isDirEmpty` (image-seed guard, 50%→100%) and the compose metadata helpers `AuthorType` (store-badge classification: Official / ByCasaos / Community) and `SetStoreAppID` (idempotent — an existing id is never clobbered). No behavior change; locks the store-provenance and seed-skip logic against regression.
- Dependency hygiene: app-management now depends on the stable github.com/opencontainers/image-spec v1.1.1 instead of the v1.1.0-rc5 pre-release it inherited. The only usage is the stable OCI `MediaTypeImageIndex` constant (pkg/docker/digest.go), already locked by digest_test.go — bumped green-before/green-after with no behavior change. First of the beta/RC pre-release eliminations (#519).
### Removed
- Removed the top-right "HTTP · Enable HTTPS" warning pill. HTTPS-by-default is deprioritized and the "Enable HTTPS" walkthrough leads to the known trust-dance friction, so the banner only nagged without a working path forward. It will return when the HTTPS re-enable lands (#294).
### Fixed
- core/pkg/utils/file.RemoveAll now removes nested directory trees correctly. The previous implementation only removed files and the top directory, leaving empty subdirectories behind and erroring "directory not empty" on any tree with subdirs. Replaced with os.RemoveAll semantics (matching the name); locked by a regression test (#533).
- Custom App re-deploy no longer leaves orphan containers: editing an app's compose to rename or drop a service now tears down the old service's container instead of leaving it running. The install/redeploy compose-up now sets RemoveOrphans.
- Restarting the core service from Settings → Power no longer returns HTTP 502. core handles the restart request itself, so a synchronous restart tore down the in-flight response; it now forks via systemd-run (like the gateway) so the response returns before the unit restarts.
- Editing a custom app (e.g. changing its port) no longer leaves the progress modal spinning forever. The update flow now drives the per-app task so the UI's task-log SSE stream receives its terminal event when the apply finishes — matching the install flow. The re-deploy itself already worked; only the progress UI hung.
- Settings → Power: the per-service Restart button now shows progress and confirms completion instead of silently "blinking". It stays in a disabled "Restarting…" state (no accidental multi-restart) and polls the service until it is back active with a new pid, then shows "Restarted ✓". Fixes the no-feedback gap exposed once the core-restart 502 was resolved.
- app-management no longer logs "store extension not found" at ERROR once per app on every app-list call. Apps without catalog metadata (e.g. custom apps) are an expected case, so the line is now debug-level (dropped by the default InfoLevel logger) instead of flooding the journal with red errors. Adds a logger.Debug helper.
- Files: the image/media preview pane's close (X) button now actually dismisses the pane. It previously re-selected the same file instead of clearing the selection, so the pane stayed open.
### Security
- govulncheck CI gate is now BLOCKING (was warn-only). scripts/govulncheck-gate.sh fails the build on any reachable, non-stdlib vulnerability not in .govulncheck-allowlist.txt — so the next dependency regression (the viper-class boot break) is caught at PR time. The allowlist is the tracked, accepted backlog: the coupled moby/docker stack (cleared by the docker v28 / compose-go v2 migration, #523), no-upstream-fix CVEs, and a few independent-bump candidates. Stdlib CVEs stay toolchain-managed via the floating Go version. Gate fails closed if the scanner is missing or errors.
- Patched reachable CVEs by bumping github.com/hashicorp/go-getter v1.7.0 → v1.7.9 (used by the catalog download helper) and the transitive google.golang.org/protobuf v1.31.0 → v1.33.0. The download path is locked by pkg/utils/downloadHelper/getter_test.go (TestDownload) — green-before/green-after, no behavior change. First of the incremental CVE remediations from #518.
- Dependency security: bumped golang.org/x/image v0.6.0→v0.41.0 in core, clearing four reachable CVEs (GO-2023-1989, GO-2023-1990, GO-2024-2937, GO-2026-4815) and removing them from the govulncheck allowlist. x/image is an indirect dep (image decoding); no behavior change, verified by govulncheck reachability + the blocking gate.
- Dependency security: bumped github.com/getkin/kin-openapi to v0.138.0 across core, message-bus, app-management, local-storage and user-service, clearing reachable CVE GO-2025-3533 (decompression data-amplification / DoS in the request/response validation filters; fixed upstream in 0.131.0) and removing it from the govulncheck allowlist. oapi-codegen-generated code is unaffected; verified by build + govulncheck reachability + the blocking gate on the Go-buildable modules (local-storage/user-service validated in CI).
- app-management migrated to the modern moby stack: docker/docker + docker/cli v28.5.1, docker/compose/v2 v2.40.2, compose-go v2, containerd/v2, runc v1.2.8 (off the inherited docker v24 / compose-go v1). compose-go v2 makes Project.Services a map; all iteration/mutation sites, ServiceConfig field changes (Devices), loader/ToProject signatures, api.StartOptions.OnExit and the docker api/types package renames were adapted, with test fixtures moved to the v2 map. Validated: full unit suite green (incl. parsing characterization + port-remap tests) and all integration-tagged tests pass against real Docker 29.2.1. Clears the runc/buildx/otelgrpc/containerd-GO-2025-3528/docker-GO-2025-3829/compose-GO-2025-4077 reachable CVEs; the residual buildkit/grpc/containerd-v2/compose/otel ones need a further bump that cascades into Go 1.26 and stay allowlisted under #523.
- Dependency security: bumped github.com/rclone/rclone v1.62.2→v1.73.5 in local-storage, clearing two reachable CVEs (GO-2026-4964 rc options/set auth-bypass, GO-2024-3271 symlink-target permissions) and removing them from the govulncheck allowlist. Adapted PowerLab's rclone-vfs mount wrapper to the v1.73 API: fs.Duration is now an int64 (time.Duration conversions) and vfscommon.FileMode replaced os.FileMode for dir/file perms. Verified by GOOS=linux cross-compile + govulncheck reachability (both IDs gone); the bump stays on the Go 1.25 line (no toolchain cascade).
- Dependency security: migrated the file/log download-as-archive feature from the deprecated github.com/mholt/archiver/v3 (no upstream fix) to github.com/mholt/archives v0.1.5, clearing three reachable CVEs (GO-2025-3605, GO-2024-2698 in archiver/v3 and GO-2025-4020 in nwaples/rardecode v1 — archives v4 uses the non-vulnerable rardecode/v2) and removing them from the govulncheck allowlist. The incremental archiver.Writer model was replaced with archives' batch FilesFromDisk + Archive across core + common, preserving the entry-naming semantics. Verified by a new archive round-trip test (build → read-back → assert structure) + the existing core/common suites.
- Cleared 4 reachable moby-stack CVEs in app-management by bumping deps that were independently upgradable without disturbing the docker v28.5.1 pin: grpc v1.74.2→v1.79.3 (GO-2026-4762), containerd/v2 v2.1.4→v2.2.1 (GO-2025-4108, GO-2025-4100), and otel/sdk v1.36.0→v1.40.0 (GO-2026-4394). Removed from the govulncheck allowlist; the 5 residual entries (docker/docker no-fix, buildkit/compose blocked on the docker v29 / moby rename) remain tracked under #523.
- Bumped golang.org/x/net to v0.55.0 in app-management to clear GO-2026-5026 (CVE-2026-39821, idna Punycode label rejection), a newly-disclosed reachable vuln the blocking govulncheck gate caught on main.
- JWT no longer travels in media/download URLs (#35). Login now also sets an HttpOnly, SameSite=Strict `access_token` cookie, and the token extractor accepts it before the legacy `?token=` query param. The Files page builds download/media URLs without the token, so the JWT no longer lands in browser history, access logs, or the Referer header. `?token=` is still accepted as a fallback (SSE/WebSocket use it); existing sessions get the cookie on their next login.


## [v0.7.2] — 2026-05-21
### Added
- ADR-0042: dependency health policy — CVE response SLA + abandoned-module criteria.

Establishes:
- Abandonment criteria: last commit > 24 months AND no security response = abandoned
- CVE response SLA: Critical < 1 release, High < 2, Medium/Low < 3 (or documented)
- New dep intake checklist: activity, CVE-clean, license, scope, transitive footprint
- ACCEPTED-RISK comment format for no-fix CVEs with mandatory renewal each release
- Weekly dep-health scan (scripts/check-dep-health.sh) planned

Immediate application to v0.7.2 no-fix CVEs:
- archiver/v3 + rardecode → replace with stdlib (issue #494, target v0.7.3)
- jwt/v3+incompatible → remove transitive dep (issue #495, target v0.7.3)
- Full audit of all backend deps against policy (issue #493)

- Sprint 14 Phase 1 foundation — `backend/logs-cli/` module with journalctl entry parser + ANSI-aware text/JSON formatter (ADR-0027). Foundation for the `powerlab-logs` CLI binary that surfaces systemd journal + Docker container + install/upgrade logs from /usr/bin. Parser handles plain-text MESSAGE, slog structured records, malformed timestamps, syslog PRIORITY mapping, missing fields. Formatter emits ANSI-colored lines on TTY (fatal/error/warn/info/debug) and plain JSON when --json flag is set. 100% test coverage on the foundation. Subcommands (journal/app/install/audit) + --follow flag land in follow-up commits on this branch.
- GET /v1/sys/services/preflight endpoint (Layer 5).

Returns is-enabled state per PowerLab systemd unit:
  [{"name": "powerlab-gateway", "enabled": true}, ...]

The UI uses this to show a warning modal ("restarting the gateway
will briefly disconnect you") before a user triggers a self-restart.
Best-effort: a systemctl failure marks the unit disabled rather than
aborting the whole response. Locked by handler test + 6 service-layer
unit tests (exit-0→enabled, non-zero→disabled, whitelist rejection,
correct systemctl args, all-services count, partial disabled state).

- Gateway restart warning modal (Layer 5 UI).

Clicking Restart on powerlab-gateway now opens a confirmation modal
instead of firing the API directly. The modal explains that the browser
will lose its HTTP connection briefly (the gateway IS the server) and
that the page will auto-reload once the new process is up (~7s).

Preflight endpoint (GET /v1/sys/services/preflight) is called when the
modal opens to list which services are currently enabled, giving the
user context for the brief interruption.

Locked by 3 vitest assertions:
- Gateway Restart opens warning modal (not direct API call)
- Cancel dismisses modal without restarting
- Confirm calls restartPowerLabService('powerlab-gateway')

- Store↔core bundle integrity contract:

- scripts/check-bundle-integrity.sh validates every app in
  community-catalog/Apps/ has an x-powerlab block readable by the
  app-management backend (title, tagline, port_map or headless, icon)
  and that .curated-manifest is in sync with actual Apps/ directories
- .github/workflows/community-catalog-integrity.yml runs this gate
  on every PR/push that touches community-catalog/

Counterpart: scripts/check-bundle-compat.py in neochaotic/powerlab-store
verifies apps are convertible before they reach bundle-store.sh.

- CI gate `scripts/check-install-upgrade-clean_test.sh` reproduces the v0.6.x → v0.7.x upgrade scenario against a temp prefix: pre-populates the destination Apps/ with N "orphan" apps from a simulated prior install, builds a fake tarball stage with M curated apps + manifest, runs the actual install snippet from `scripts/package-linux.sh`, and asserts the post-install Apps/ contains exactly the curated set (orphans removed). Locks the #450 bug class so it cannot silently regress. Companion to PRs #451 + #452.
- Boot migration tags apps installed before v0.7.1 with a `.installed-pre-v0.7.1` marker file under each app directory (#437). Idempotent via a `.legacy-scan-complete` sentinel under `AppsPath`; runs ONCE on the first v0.7.1 boot then never again. New installs (post-v0.7.1) never get the marker. This is the backend plumbing for the upcoming "Legacy" badge in the apps grid that will visually distinguish apps from the pre-ADR-0039 upstream Umbrel/CasaOS sync model from apps installed via the curated catalog or a registered custom source.
- `make stage-build` produces Linux/amd64 binaries ready for hot-swap deploy to a staging box, from any dev OS (#414). Pure-Go services (gateway, app-management, core, message-bus, sync-catalog) cross-compiled locally; CGO services (user-service, local-storage) pulled from the latest GitHub release tarball — avoids the silent "invalid credentials" trap from `CGO_ENABLED=0` cross-compile that bit Sprint 20 staging. Override release source via `POWERLAB_RELEASE_TAG=vX.Y.Z`. Documented in CONTRIBUTING.md "Hot-swap deploy" section.
- Audit doc `docs/audits/usb-sd-automount-gap-2026-05-17.md` captures the USB/SD auto-mount status (#416). Inherited surface area from CasaOS is complete on the API + UI toggle layer but **broken end-to-end on the runtime layer**: the Go code sources `/usr/share/powerlab/shell/local-storage-helper.sh` which does not exist in the repo or tarball, so every USB operation is a silent no-op. Audit breaks the implementation into Phase B (runtime audit on .142), Phase C (backend helper + udev rule), Phase D (Files page sidebar — the headline user requirement: mounted disks must appear in Files), Phase E (tests). No code shipped; tracking issues opened for Phase B-E follow-up.
- Backend endpoints for the Settings → Power pane (#260). `GET /v1/sys/services` lists PowerLab systemd units (powerlab-gateway, app-management, core, user-service, local-storage, message-bus) with their active/sub state + PID. `POST /v1/sys/services/{name}/restart` restarts a whitelisted unit (service name validated against a hardcoded allow-list before reaching the shell — non-whitelisted names rejected at the handler). `POST /v1/sys/host/reboot` and `POST /v1/sys/host/shutdown` trigger `systemctl reboot|poweroff`, each requiring `{"confirm": true}` in the body so an accidental POST cannot power-cycle the box. UI Power pane is a focused follow-up PR.
- Settings → Power pane (#260) — UI for the power-actions backend from PR #466. Lists 6 PowerLab systemd units with live status badges (active/inactive/failed/activating), per-service Restart buttons, and host-level Reboot/Shutdown actions gated behind acknowledgement modals. Polls `GET /v1/sys/services` every 5s while the pane is mounted. Host ops use a checkbox acknowledgement modal that mirrors the backend's `{"confirm": true}` body requirement (defense in depth). 4 Playwright specs lock the contract.
- Backfill retrospective for sprints 13-18 (`docs/audits/sprints-13-18-backfill.md`). Sprint 23 process-recovery audit caught that 7 of 13 sprints had no retro doc — chain was broken from Sprint 13 (install UX hardening + SSE protocol fixes) through Sprint 18 (journald viewer + golden-path E2E + the never-closed Sprint 18 epic #376). Doc captures headline deliverables, carries that closed in later sprints, carries still open, and the memory entries created. Single batch doc rather than 6 separate files — context-reconstruction value > per-sprint fidelity at this remove.
- ADR-0040 — Proportional engineering hygiene baseline (SAST + lint + metrics + tests). Captures the decision to adopt 4 hygiene families from the external K8s-grade audit (govulncheck, gosec, golangci-lint with 8 linters, Dependabot, /healthz+/readyz+/metrics endpoints, test coverage ratchet) while explicitly DEFERRING 3 (API versioning, KEP-style design, public disclosure program) until product stage warrants them. Establishes the "would enterprise IT accept this" lens for future "should we adopt X from $project" decisions. Impl ships in subsequent PRs (Sprint 23 PR 14 lint+Dependabot, PR 15 govulncheck; Sprint 24 metrics endpoints).
- Golangci-lint warn-only baseline + Dependabot weekly + `make lint-go` target (ADR-0040 hygiene baseline). 8 core linters enabled: govet, staticcheck, errcheck, gosec, revive, ineffassign, unused, gocyclo. Warn-only initially (`issues-exit-code: 0` in `.golangci.yml`); each family flips to strict in its own focused PR per the ADR-0037 delta-strict ratchet pattern. CI matrix job `backend-lint` runs per service alongside the deadcode matrix. Dependabot covers all 8 Go modules + UI npm + GitHub Actions with grouped weekly PRs to avoid floods. Pre-commit hook deferred until first linter family flips strict.
- CI job `backend-vuln-scan` runs `govulncheck` per service in a matrix (ADR-0040 SAST adoption). Strict from day 1: a known CVE in any transitive dep fails the build. Unlike the lint gate which warns + ratchets, vuln scan is binary — a known CVE is a known CVE. Closes the SAST gap the K8s-grade audit (2026-05-18) flagged as "ironic given the catalog security work."
- Coverage push +3.43pp aggregate (statements 37.23% → 40.66%) — first sprint applying the coverage cadence rule (memory feedback_coverage_cadence_rule, ADR-0040 amendment). 6 surfaces concluded at ≥95% coverage: catalog.ts API client, logs.ts API client, CatalogPane component, PowerPane component, AppHeader component, Breadcrumbs component. Honest gap to +10pp target: remaining surfaces are god-files (Sidebar 697 LOC, settings/+page 760, apps/+page 1561) which need splitting before they're testable — ADR-0040 explicitly defers god-file refactor until coverage ≥50%, creating the chicken-and-egg the cadence rule will work through over multiple sprints.
- `scripts/shell/local-storage-helper.sh` — USB/SD auto-mount helper script (#464, Phase C of USB/SD audit). Implements the 4 functions that backend/local-storage/service/{usb,disk}.go has been silently failing to source since the CasaOS rebrand: USB_Start_Auto, USB_Stop_Auto, do_mount, UDEVILUmount. Installs a udev rule covering both USB block devices (ID_BUS=usb) and SD cards (ID_DRIVE_FLASH_SD=1 — CasaOS upstream never shipped SD support, PowerLab does). Mount point: /mnt/powerlab/{label-or-uuid}. Path-traversal safe — UDEVILUmount refuses to rmdir anything outside POWERLAB_MOUNT_ROOT. Shipped via `scripts/package-linux.sh` to `/usr/share/powerlab/shell/`. Hardware E2E hot-plug verification deferred to operator (Phase E checklist #465); CI gate covers static contract + safety guards.
- ADR-0041 proposes splitting the community catalog into a dedicated `neochaotic/powerlab-store` repo (created 2026-05-19, AGPL-3.0 base, hybrid licensing for code vs content vs art). Supersedes the where-it-lives portion of ADR-0039 (the what — native curated, no upstream tracking — stays). Driven by Sprint 23 hotspot-file rebase pain + the impending 139-app import that would scale the pain. 7-phase migration roadmap, strict-day-1 CI gates in the store repo, formal compat protocol between core and store, full cross-reference strategy between the two repos.
- Catalog is now opt-in (disabled by default). The store ships dark — ComposeAppStoreInfoList returns an empty catalog until the operator enables it via Settings → Catalog (source fixed to the bundled powerlab-store, not editable). New config flag ServerModel.CatalogEnabled (default false) + GET /v2/app_management/catalog/status and PUT /v2/app_management/catalog/enabled.
- CI now runs the browser @smoke install→uninstall golden path against a REAL PowerLab stack and gates the release on it. A `browser-e2e-real` job (ci.yml) stands up the full stack via scripts/ci-e2e-stack.sh (gateway serving the built UI + all services + a provisioned admin) and runs the Playwright install→uninstall spec (install → SSE phase markers → uninstall). It runs on release tags + manual dispatch (not per-PR; the per-PR real-backend contract is the Go backend-integration job) and is in the `release` job's needs, so a tag will not publish if it fails. Auth/audit/logs @smoke remain covered by backend-integration + manual checks since they need the packaged install's gateway config.
### Changed
- Docs: README, CONTRIBUTING.md, and ADR-0039 now cross-link to
the independent powerlab-store repo. The catalog moved out of
this repo per ADR-0041 — app submissions go there.

- CI catalog-app-safety lint flipped from warn-only to strict (`POWERLAB_CATALOG_SAFETY_STRICT=1`). Sprint 22 PR #448 seeded the curated catalog with 4 hand-audited apps; PR #451 keeps it that way on every install via wipe-then-copy. Any future PR adding `privileged: true`, `docker.sock`, `network_mode: host`, `cap_add: [ALL|SYS_ADMIN]`, or system-path bind-mounts now blocks the merge. Local strict run before commit: `OK: 0 safety findings across 4 catalog file(s)`.
- Settings → Catalog header copy refined to accurately reflect ADR-0039 + #450 install semantics. Now states explicitly that PowerLab Curated is wiped + replaced on every install (operator edits to its `Apps/` dir do not survive upgrades) and shows a dynamic count of operator-added sources with a yellow-strong callout when any are present.
- CasaOS rebrand residue cleanup spotted during #260 review. Renames `GetCasaOSErrorLogs` → `GetSystemErrorLogs` and the underlying `SystemService.GetCasaOSLogs` → `GetSystemLogs`. Deletes unused `GetCasaOSPort`/`PutCasaOSPort` (dead code; routes were already commented out in v1.go). Fixes log messages in `backend/app-management/main.go` and `backend/core/main.go` that still said "casaos main service is ready" — now they correctly identify themselves as `powerlab-app-management` / `powerlab-core`. Swagger summaries on system.go updated to drop the lying "casaos" verb. Legacy/migration code paths (`LegacyCasaOSCoreDB`, `LegacyLabelKindKey = "casaos"`, x-casaos extension fallback) left untouched — those identify legacy data and the names are accurate.
- Lockout-recovery defences (#260 follow-up after maintainer flagged remote-shutdown trap). systemd units gain `StartLimitBurst=10` / `StartLimitIntervalSec=60` (vs default 5/10s) so upgrade-window flaps don't trip the burst limit and brick the box. New `docs/operations/lockout-recovery.md` walks operators through SSH recovery paths per symptom (UI loads but spinning, gateway down, SSH dead, host unreachable, bad upgrade rollback) plus the "what to set up BEFORE an incident" checklist.
- ADR-0040 amended with explicit "Coverage cadence rule" section (maintainer-set 2026-05-18). +10 percentage points per surface per sprint until realistic ceiling ("estabilidade") concludes that surface. Per-version targets removed (cadence is per-sprint, never anchored to specific patches — memory `feedback_version_cadence_v07_to_v030`). Realistic per-surface ceilings documented (pure-logic 85-90%, HTTP handlers 80-85%, service layer 65-70%, CGO services 40-50%, UI 70-75%). Weighted aggregate ceiling ~65-70%, never 100% — above that becomes brittle/teatro.
- Sprint 23 wrap-up — final retro doc (sprint cadence ends here, releases take over), ADR-0040 amendments (govuln warn-only first run + cadence reframe to per-release), v0.7.1 release-manifest summary. Memory `feedback_sprint_23_is_last_releases_take_over` captures the cadence pivot.
- The GitHub Release job is now gated on the full correctness + security suite (changelog-fragment, frontend, frontend-e2e, backend, backend-integration, backend-vuln-scan, catalog-hostnames, catalog-app-safety) — previously it only depended on the build (`package`), so a tag could publish a release even with red tests. If any gate fails on a tag push, the release job is skipped and nothing is published.
### Removed
- Severed Umbrel catalog ingestion entirely (ADR-0039 enforcement pass). Removed the weekly sync-umbrel-catalog GitHub workflow, the backend/sync-catalog binary, and its build/CI/packaging/dependabot wiring. The community catalog is now sourced solely from the powerlab-store repo via scripts/bundle-store.sh at release time. Added a freshness gate so bundle-store.sh refuses to bundle a pre-release (e.g. 0.1.0-dev) store snapshot — only tagged store releases ship, with a POWERLAB_ALLOW_DEV_STORE override for local testing.
### Fixed
- Gateway self-restart now uses systemd-run transient unit (Layer 4).

Previously, POST /v1/sys/services/powerlab-gateway/restart ran
`systemctl restart powerlab-gateway` synchronously, which killed the
gateway process (and its cgroup) before the HTTP 200 response could
be sent — leaving the UI hanging. The fix forks a transient systemd
unit via `systemd-run --no-block` with a 2-second sleep so the HTTP
response always fires before the restart begins. Non-gateway services
continue to restart synchronously. Three new unit tests lock the
behavior: gateway → systemd-run, non-gateway → direct systemctl,
delayed command encodes service name + sleep.

- Upgrade path no longer leaves orphan apps from prior catalog versions (#450). `install.sh` now wipes `/var/lib/powerlab/community-catalog/Apps/` before copying the tarball's curated tree (wipe-then-copy, not overlay). The post-install `powerlab-sync-catalog` auto-invocation is removed — `install.sh` no longer auto-pulls `getumbrel/umbrel-apps` upstream per install, which had been silently re-populating the catalog with 200+ filtered Umbrel apps and violating ADR-0039's "PowerLab ships its own curated set" promise. The binary is still shipped to `/usr/bin/powerlab-sync-catalog` for explicit operator/maintainer use against custom upstreams (register output as a custom catalog source via Settings → Catalog).
- Boot-time migration removes orphan apps from `/var/lib/powerlab/community-catalog/Apps/` that aren't in the bundled `.curated-manifest` (ADR-0039 / #450). Closes the gap for v0.7.0 boxes already on disk with leftover apps from a prior larger curated set — app-management converges back to the bundled set on next startup without requiring a re-install. Missing/empty manifest → noop (safer to leave orphans than wipe operator state). `scripts/package-linux.sh` emits the manifest at staging time; `backend/app-management/pkg/config/curated_catalog_orphan_cleanup.go` reads it at boot and prunes.
- Sidebar telemetry no longer freezes when navigating to /settings or /files (#453). Root cause: the system-store's `startPolling/stopPolling` operated on a single shared interval handle, so a stopPolling from one consumer (e.g. launchpad unmount) silently killed the sidebar's polling. Pages that did NOT call startPolling on mount (settings, files, apps) left the interval dead. Refcount fix: each useSystemStore() facade has its own consumer identity; the shared interval ticks at min(ms) across active consumers and only clears when the LAST one stops. Vitest unit (4 new) + Playwright nav-survives-route E2E (2 new) lock the contract. Also cleans up Sidebar.svelte's duplicate onDestroy block (refactor artifact, refcount fix makes it idempotent but the duplication itself was a code smell).
- `MigrateAppStoreListLegacyRemoval` now also removes the on-disk workdir for each legacy URL it strips from `AppStoreList` (parity with `UnregisterAppStore`). Without this, an upgrade from a pre-ADR-0038 install would clean the config but leave `/var/lib/powerlab/appstore/cdn.jsdelivr.net/<md5>/` (and equivalents for big-bear / icewhaletech) lingering on disk. Best-effort + missing-dir-silent — never aborts boot. (#450)
- `ui/package.json` version bumped to 0.7.1 to match the v0.7.1 tag. v0.7.1 cut sequence skipped the `scripts/prepare-release.sh` step that normally automates this bump → defense-in-depth gate `check-ui-package-version-fresh_test.sh` started failing on every PR until corrected. Release tarballs themselves were unaffected (release workflow shipped tarballs at 2026-05-19T03:12 ok); this is a post-tag housekeeping unblock. Process anchor: future cuts MUST run `scripts/prepare-release.sh <X.Y.Z>` BEFORE `git tag`.
- Gateway no longer hangs ~90s on restart. Graceful shutdown used http.Server.Shutdown(context.Background()) with no deadline, which blocks until every active connection drains — but the gateway proxies long-lived SSE streams (audit feed, journald logs, telemetry) that never close, so a restart with the UI open stalled until systemd SIGKILLed the process. This also stalled the Layer 4 gateway self-restart. Both shutdown paths now use a bounded 10s context (shutdownGateway helper) so they drain briefly then return.
- Install-time host-placeholder substitution no longer corrupts bare (unbracketed) IPv6 addresses. stripPort treated the trailing hextet of `::1` / `fe80::1234` as a port and mangled them to `:` / `fe80:`; it now leaves any multi-colon input untouched (only single-colon IPv4/hostname `host:port` forms get the port stripped). Bracketed IPv6 from Host headers was already handled. Hardened alongside adversarial coverage for the install-pipeline secret substitution (regex over-match boundaries, 0700 secrets-dir confidentiality, literal `$`-in-value substitution).
- Gateway no longer panics at boot on a fresh install. A routine dependency bump raised viper to v1.21, which removed its built-in `ini` codec (dropped in v1.20), so the gateway crash-looped with `panic: While parsing config: decoder not found for this format` reading gateway.ini — a failure that compiled green and passed unit tests, surfacing only on a real boot. The gateway reads two keys, so viper is replaced with gopkg.in/ini.v1 directly behind a minimal Config type (GetString/Set/WriteConfig), with unit tests covering the real parse, defaults, set/get, and persistence paths.
- With the catalog opt-in disabled, the store no longer shows a blank page and installed apps no longer flip to "Custom". The empty store now renders an enable-catalog call-to-action (one-click enable, explains the apps are already on-device and curated). Installed-app classification now reads the provenance baked into each app's own compose (x-powerlab source/author) instead of live-catalog membership, so toggling the catalog off never relabels installed store apps as Custom.
### Security
- Bump vulnerable dependencies across all backend services (govuln sweep).

Fixed CVEs (backlog cleared):
- GO-2026-4918: golang.org/x/net v0.25.0→v0.53.0 (HTTP/2 infinite loop)
  Affects: common, gateway, app-management
- GO-2025-3553 / GO-2024-3250: golang-jwt/jwt v4 v4.5.0→v4.5.2
  Affects: common, gateway, app-management
- GO-2025-3553: golang-jwt/jwt v5 v5.0.0→v5.2.2
  Affects: all 7 services
- GO-2025-3922: ulikunitz/xz v0.5.11→v0.5.15
  Affects: common, core, message-bus, app-management, local-storage

Remaining no-fix CVEs (accepted risk, tracked for dep removal):
- GO-2025-4020: rardecode v1.1.3 — no patch (transitive via archiver/v3)
- GO-2025-3605 / GO-2024-2698: archiver/v3 v3.5.1 — no patch
- GO-2025-3553: jwt/v3 (incompatible) — v3 EOL, removal tracked separately

govulncheck CI remains warn-only (continue-on-error: true) until the
no-fix deps are removed in a follow-up PR.

- sync-catalog: hard filter at sync time rejects upstream apps that ship `hooks/` directory or `exports.sh` file. PowerLab never executes upstream host scripts (the umbrelOS RCE surface). Apps with hook/export artifacts are dropped before parse with a `[hooks]` log line. Per ADR-0038.
- Install-time bind-mount preparation is now contained to the app-data root. PrepareBindMountSources previously created + chmod 0o777 *any* absolute path a compose declared as a bind source, so a malicious or compromised catalog entry (or a future operator-added source) using `volumes: ["/etc:/x"]` could weaken permissions on a host system path. It now only touches paths inside config.AppInfo.StoragePath (fail-closed: empty root prepares nothing), with `<root>/../escape` traversal rejected. Defense-in-depth for the enterprise threat model (untrusted apps). Legitimate catalog apps are unaffected — their sources are always under the data root.


## [v0.7.0] — 2026-05-17
### Added
- Settings → Catalog pane: list registered catalog sources, add custom URLs at own risk with one-time acknowledgement modal, remove operator-added sources. The PowerLab Curated default catalog ships immutable; operator-added catalogs render with a permanent "Unaudited" badge throughout the store. Backend endpoints (POST/DELETE /v2/app_management/appstore) already existed; this PR adds the UI surface per ADR-0039.
### Changed
- Catalog model: PowerLab now ships with 4 curated, install-verified apps (AdventureLog, Baikal, Enclosed, Gitingest) instead of the 240+ inherited Umbrel entries. Each curated app has a PowerLab-authored description and an `x-powerlab.yml` manifest with `verified: <date>` annotation. Operators who want more apps can add custom catalog URLs via Settings → Catalog (with "Unaudited" badge). Per ADR-0039.
### Removed
- Legacy CasaOS upstream catalog sources removed from the default app-store list (CasaOS jsdelivr zip + big-bear-casaos zip). Operators upgrading get a one-time startup migration that strips these URLs from their on-disk conf and persists the cleaned list. Apps already installed from these sources continue to run; only the catalog source is dropped. Per ADR-0038 — same trust class as Umbrel community catalog, no operator demand, no PowerLab-side maintenance.
### Fixed
- Install-time pre-seed of empty bind-mount source dirs from image content. Closes the "bind-mount overlay" bug class generically — apps that ship pre-populated content at their data path (Laravel `storage/framework/{cache,views,sessions}`, nextcloud `/var/www/html`, jellyfin `/config`, etc.) used to crash on first install because the empty host bind-mount source would overlay the image content. PowerLab now `docker create` + `docker cp <image>:<target> <host>` before docker compose up, transferring the skeleton. Best-effort: docker CLI failures are swallowed; only runs when source dir is empty (re-installs with user data are untouched). Per ADR-0038 Tier 1 transform.
### Security
- CI gate: compose-level security lint for every catalog app. Rejects `privileged: true`, `/var/run/docker.sock` bind, `network_mode/pid/ipc: host`, `cap_add: ALL` or `SYS_ADMIN`, and bind-mounts of system paths (/etc, /var/lib, /usr, /root, /home, /boot, /sys, /proc). Allow-list for /DATA/PowerLabAppData app data paths. Ships warn-only during Sprint 22 ship phase; flips strict after the initial curated catalog re-seed PR lands. Per ADR-0039.


## [v0.6.16] — 2026-05-16
### Added
- gateway: `/v1/logs/services/{service}/stream` SSE endpoint spawns `journalctl -u powerlab-<service>.service -o json -f` and streams parsed entries (severity + message + timestamp) as SSE events. Service name validated against strict allowlist before exec; subprocess lifetime bound to request context (client disconnect → SIGKILL). Backend half of #259 /logs Phase 4.
- Settings → Logs: new "Live (per-service)" tab streams journald output for the 7 PowerLab services via the /v1/logs/services/{svc}/stream SSE endpoint, with severity coloring (error/warn/info/debug), auto-scroll-when-at-bottom, pause/resume, and clear controls. Frontend half of #259 /logs Phase 4.
- sync-catalog: auto-rewrite docker-compose v1 underscore hostnames (`<project>_<svc>_<idx>`) to the service-name alias on every upstream sync. Prevents the (#402) bug class from regressing as the catalog refreshes from Umbrel/CasaOS upstreams.
### Changed
- CI: deadcode gate switched from warn-only to delta-strict mode (ADR-0037). Per-service baselines under `scripts/deadcode-baseline/` cap the historical count; new dead code → CI fails; reductions → developer ticks the baseline down in the same PR. Supersedes the never-shipped `POWERLAB_DEADCODE_STRICT=1` flip from Sprint 19 PR 5.
### Removed
- 35 dead Go functions removed from `backend/core/`: full packages `pkg/sign` + `pkg/utils/httper`; full files `pkg/utils/{balance,bool,ctx,path,slice,time}.go`, `pkg/utils/file/reader.go`, `service/socket.go`; trimmed `pkg/utils/ip_helper/ip.go` to only the live `GetDeviceAllIPv4`; and dropped `service/notify.go::SendMeg` + `service/system.go::GetDeviceAllIP`. Inherited CasaOS utility helpers nothing else linked to.
### Fixed
- Catalog hostname sweep — 154 docker-compose v1 underscore references (`<project>_<svc>_<idx>`) rewritten to service-name network aliases (`db`, `redis`, etc.) across 65 apps. Fixes silent DNS-error crash loops on install. (#402)
- Install-time generic chmod 0o777 on every bind-mount source dir. Fixes "Permission denied" / "Please provide a valid cache path" / equivalent crashes for any container whose runtime user (Laravel www-data, Node uid 1000, Postgres uid 999, etc.) doesn't match the bind-mount owner. Generalises Sprint 14 #334 per-app postgres fix to the whole catalog. Trade-off: world-writable per-app dirs; acceptable for home-server LAN threat model. Per-app `x-powerlab.runtime_uid` refinement tracked for Sprint 22+.
- Install-time substitution of host-identity placeholders (`${DEVICE_DOMAIN_NAME}`, `${DEVICE_HOSTNAME}`, `${APP_DOMAIN}`, `${APP_*_LOCAL_IPS}`) in URL-embedded env values. Fixes 44 catalog apps that previously crashed with "Invalid URI" / "Bad Request" because their APP_URL / BASE_URL / NEXTAUTH_URL env vars ended up as `http://:8770` after compose-time interpolation. Resolution order: request Host header → first LAN IPv4 → `<hostname>.local`.


## [v0.6.15] — 2026-05-15
### Added
- Settings → Logs pane: read-only viewer for the .log files under ``/var/log/powerlab/``. Shows the last 200 KB of each service's stdout (app-management, gateway, user, upgrade, ...) so operators can debug a failed install or a routing issue without SSH-ing in. Distinct from Settings → Audit, which exposes the HTTP request JSONL; this pane exposes raw service output. Two new gateway endpoints — ``GET /v1/logs/files`` (list ``.log`` files newest- first) and ``GET /v1/logs/files/{name}`` (tail content). Filename validated against a strict allowlist before any filesystem access (path-traversal hardening). Default tail 200 KB, max 5 MB. Rotated archives (``.log.gz``) and live follow are deliberate non-goals — separate feature.
- Frontend JS error capture pipeline. The SvelteKit shell installs ``window.onerror`` and ``unhandledrejection`` listeners on mount and funnels captured events through a small reporter that does noise filter (ResizeObserver loop, browser-extension stacks, opaque "Script error.") + dedupe (one POST per signature per 5-minute window) + DoS cap (≤10 POSTs / window) + failure swallow (audit cannot become a user-visible "double crash" loop). Transport hits a new ``POST /v1/audit/frontend-error`` endpoint that builds a ``Record{Kind: "ui_error"}`` and submits it to the existing audit Recorder — same JSONL file, same ring buffer, same retention policy as HTTP audit. Settings → Audit pane renders ui_error rows distinctly (red tint, Bug icon, click-to-expand stack trace). 19 new tests (5 backend handler, 3 schema round-trip, 12 frontend reporter, 2 AuditPane render). Closes #258.
### Fixed
- Catalog apps that set explicit ``container_name:`` no longer surface spurious "Error during start: no container found for project X" failures during install. Root cause (#397): docker-compose v2 strips the ``com.docker.compose.project`` label from containers with an explicit ``container_name`` override. ``compose-go``'s ``Start()`` filters by that label and reports zero matches even though the containers ARE running. The lifecycle now falls back to a name- based check via ``projectHasContainers()`` — exact match (the ``container_name`` case), hyphen form ``<project>-<svc>-<idx>`` (compose v2 default), or underscore form ``<project>_<svc>_<idx>`` (compose v1). 8 unit-test cases pin the matcher including a prefix-only false-positive guard (project "blink" must not match container "blinko-db-1"). Affects 2fauth, adventurelog (server), and any catalog entry with the same pattern.
- Activepieces is reachable again (#385). The catalog's docker-compose pointed the app at ``activepieces_db_1`` and ``activepieces_redis_1`` — the legacy docker-compose v1 hostname convention. Docker Compose v2 (what PowerLab ships) creates containers with **hyphens** (``activepieces-db-1``), so the underscored hostnames never resolved and the app container crashed in a loop with ``getaddrinfo ENOTFOUND activepieces_redis_1``. Tile click "did nothing" because the backend port was closed. Fix uses the service- name network alias (``db``, ``redis``) which is portable across Compose v1/v2 and survives project-name changes. A catalog-wide sweep of 66 other apps with the same bug class is tracked separately in #402 (Sprint 20 PR 4).
- Audit recorder no longer stalls after an empty-batch tick. The internal flush() ran the timer.Reset path only when there was something to write — so a single Submit followed by a quiet period would surface the first record fine, but the next one sat in the channel waiting for BatchSize (50) to accumulate. Real- world symptom discovered via real-backend Playwright: POST ``/v1/audit/frontend-error`` returned 202 but the record never appeared in ``/v1/audit/recent`` until 49 more were submitted. One-line fix: always reset the timer at the end of flush(), even when the batch was empty. 2 unit regression tests with the race detector lock the contract.
### Internal
- Function-level dead-code sweep across all services using the ``Backend deadcode`` gate (warn-only since Sprint 19 PR 5). Total findings dropped from 139 → 86 (~38% reduction, ~600 LOC removed) by deleting whole files of orphan code rather than chasing 1-line functions: ``core/model/{obj,object,args,stream,setting,storage}.go`` (alist storage runtime — the consumer ``internal/op`` was removed in Sprint 19 but these model types were left behind), ``message-bus/pkg/ysk/{ysk,ysk_test}.go`` (Sprint 1 audit's 🟢 flag finally actioned), ``user-service/pkg/utils/file/image.go`` + most of ``file.go`` (only ``MkDir`` + ``IsNotExistMkDir`` are actually called), ``app-management/common/appdata.go``'s helper functions (only the directory-name constants are used). Remaining 86 findings are 1-line individual functions that will surface on every PR via the warn-only gate; targeted sweeps in future patch releases. Race-detector + build green across all affected services.
- Sprint 20 PR 4a/9 — adds ``scripts/check-catalog-hostnames.sh`` + ``Catalog hostnames`` CI job. Scans every ``community-catalog/Apps/*/docker-compose.yml`` for the legacy docker-compose v1 hostname pattern ``<project>_<service>_<idx>`` in env-var values and reports findings as PR annotations. Discovered during PR 2 investigation: 156 findings across 66 catalog files — apps point at ``<proj>_<svc>_1`` hostnames but docker-compose v2 (what PowerLab ships) creates containers with hyphens; the underscored form never resolves. Phase-in: warn-only by default (``POWERLAB_CATALOG_LINT_STRICT=0``); flips to strict after PR 4b's 66-app sweep lands. Meta-test (``check-catalog-hostnames_test.sh``) pins detection on 4 synthetic fixtures including the prefix-only false-positive guard. Tracking: #402.
- Backend integration CI job no longer falsely times out on cold runners (#373). The 2-minute test timeout was tight enough that race-detector overhead + Docker daemon cold-start could push past it on a fresh GitHub Actions runner; v0.6.12 release SHA hit exactly that, and the same SHA on main repeated the failure while every other CI job stayed green. Two changes: (a) bump the ``go test -timeout`` from 2m → 5m (it's a safety net against runner-state pathologies, not a budget — the tests themselves run in seconds), and (b) add a "Warm Docker daemon" step that pulls ``hello-world`` before the test runs, so containerd is already initialised by the time the first test call hits the socket. Closes #373.
- Sprint 20 PR 5/9 — deduplicate CORS config across 9 backend services (#391). Revives ``backend/common/middleware/Cors()`` (deleted in Sprint 19 PR 2 because nobody had adopted it) with the byte-identical permissive config (``AllowOrigins: ["*"]``, every standard method, PowerLab's standard ``AllowHeaders`` list, ``MaxAge: 172800``, ``AllowCredentials: true``). 9 services migrated: ``app-management/route/v2.go``, ``core/route/v{1,2}.go``, ``gateway/route/management_route.go``, ``local-storage/route/v{1,2}.go``, ``message-bus/route/routers.go``, ``user-service/route/v{1,2}.go``. Each inline 9-line ``CORSWithConfig(...)`` block collapses to ``common_middleware.Cors()``; ~80 LOC net reduction across the 9 files + ~70 LOC added in the helper. ``cors_test.go`` asserts every field of the returned ``CORSConfig`` matches the historical inline shape — drift in any assertion is a real wire-level change, not a test to weaken (memory feedback_no_apagar_test_para_passar). Race-detector green across all 9 services after migration.
- Sprint 20 PR 1/9 — real-backend Playwright regression test for #386 (Adventure Log install reported HTTP 400 BadRequest, then self- resolved before investigation could land). Locks the current "install succeeds" state with the same compose shape from the catalog entry (db + server with ``container_name:`` + web). Also acts as a canary for the container_name label-strip bug class (#397) — the post-install lookup test skips until #397 ships its tolerant fallback. Default Playwright suite still passes with this spec skipped (gated on ``POWERLAB_E2E_BASE`` env var per the ``realBackendTest`` harness).
- Real-backend Playwright smoke coverage for 5 golden-path user journeys. Mocked specs (page.route()) prove the UI renders given fake API responses; these specs prove the actual gateway + the actual backend services produce what the UI expects. Bug class this catches: v0.6.7 SSE buffering, v0.6.12 audit endpoints unreachable, v0.6.13 SSE Gzip stripping — all of which had green mocked CI runs. Specs (skip-default unless POWERLAB_E2E_BASE is set): login + session restore (3 cases), install → SSE stream → finalize → uninstall round-trip with a unique alpine fixture (1 case, cleans up after itself), Logs viewer endpoints + path- traversal hardening verified against live data (4 cases). Audit pane real-backend already shipped Sprint 18; Custom App YAML upload covered transitively by the install-uninstall spec which uses the same POST + YAML path.


## [v0.6.14] — 2026-05-15
### Internal
- Sprint 19 follow-up — remove dead-package references from the mkdocs navigation (``mkdocs.yml``) and from the godoc-style index pages (``docs/api/{common,core,local-storage}/index.md``). The Sprint 19 deletion PRs (#393, #394, #395) removed the Go packages but the docs site config still listed them, which made the mkdocs strict build fail with 23 warnings on the next push to ``main``. Also updates the core/index.md prose to note that the ``internal/op`` storage-driver runtime was removed — PowerLab uses Docker volumes + bind mounts via ``app-management`` instead.
- Remove 3 dead frontend artefacts (~165 LOC) per the Sprint 19 dead-code removal plan (PR 1 of 5): ``ui/src/lib/components/terminal/SshModal.svelte`` (110 LOC — has existed since v0.1.0, never wired into any route), ``ui/src/lib/stores/theme.svelte.ts`` + its test (~50 LOC — exports ``useTheme()`` but nobody calls it; the app is hard-coded dark and no theme toggle is planned), and ``ui/src/lib/api/index.ts`` (3 LOC barrel file with zero importers — every component imports ``$lib/api/client`` directly). Grep-proof captured in the PR body before delete. vitest 563/563, Playwright 18/18, svelte-check 0/0 post-delete.
- Remove 7 dead packages from ``backend/common/`` per Sprint 19 PR 2/5 (~1,950 LOC prod + ~216 LOC test): ``codegen/mod_management`` (1,335 LOC, generated CasaOS plugin SDK with zero importers), ``utils/ssh`` (402 LOC, weak SSH helper that ``InsecureIgnoreHostKey``'d), ``utils/ version`` (333+173 LOC, marker-file pattern captured in ADR-0036 before deletion), ``pkg/mod_management`` (158+43 LOC, only consumer of ``codegen/mod_management``), ``middleware`` (23 LOC, CORS helper never adopted — duplication tracked separately as #391), ``utils/idevice`` (20 LOC, trivial wrapper), ``model/notify`` (12 LOC, superseded by socket.io + Toast). ``go mod tidy`` drops 6 direct + 2 indirect transitive dependencies: ``gorilla/websocket``, ``mattn/go-sqlite3``, ``oapi-codegen/runtime``, ``sirupsen/logrus``, ``golang.org/x/crypto``, ``gopkg.in/ini.v1`` (+ ``go-jsonmerge``, ``google/uuid``). Grep-proof captured in PR body; race-detector green across all 7 services that import ``common``.
- Remove 11 dead packages from ``backend/core/`` per Sprint 19 PR 3/5 (~1,431 LOC): the entire ``core/internal/`` island (``op``, ``driver``, ``conf``, ``sign`` — ``op`` was the only user of ``driver``/``conf``, and ``op`` itself had zero importers); five unused ``core/pkg/*`` (``generic_sync``, ``singleflight``, ``gredis``, ``github``, ``fs``); and two orphan ``core/model/*`` (``system_app``, ``system_model``). ``go mod tidy`` drops more than the audit predicted — ``go-github/v36`` (with all its transitives: ``golang.org/x/oauth2``, ``go-querystring``, ``google.golang.org/appengine``, ``google.golang.org/protobuf``, ``golang/protobuf``), plus ``json-iterator/go`` and its ``modern-go/concurrent`` + ``modern-go/reflect2`` transitives, plus ``pkg/errors``. ``redigo`` moves from direct to indirect (still pulled by another transitive but no PowerLab code references it). Build + test green on core; grep-proof in PR body.
- Remove dead packages from ``backend/local-storage/pkg/`` per Sprint 19 PR 4/5 (~860 LOC actual after spot-check correction): the byte- identical ``generic_sync`` (412 LOC) + ``singleflight`` (212 LOC) copies of the ``core/pkg/`` versions already deleted in PR 3, plus ``sign`` (86 LOC), ``utils/encryption`` (12 LOC, duplicates the live ``user-service/encryption``), and the top-level files in ``pkg/utils/`` (``slice.go``, ``time.go``, ``bool.go``, ``path.go``, ``ctx.go`` — ~140 LOC). The ``utils/`` directory STAYS because its subpackages ``utils/merge`` (2 importers: ``main.go`` + ``route/v2/ merge.go``) and ``utils/command`` (4 importers: ``disk``, ``v2/ merge``, ``mount``, ``partition``) are alive — a correction to the source audit's claim that the whole utils tree was dead. Local- storage cross-compiles cleanly on linux/amd64 with and without the fuse build tag; native macOS build is impossible regardless of this PR (netlink + xattr syscalls).
- Add CI gate ``scripts/check-deadcode.sh`` + ``Backend deadcode`` matrix job per Sprint 19 PR 5/5. Runs ``golang.org/x/tools/cmd/ deadcode`` against every service and reports unreachable symbols. Phase-in: warn-only by default (``POWERLAB_DEADCODE_STRICT=0``) — prints findings without failing PRs. Flipped to strict for v0.7.0 once Sprint 19's existing dead code is gone (per ``feedback_no_v1_without_alignment`` — major bumps require explicit user approval). Closes the 15-sprint regression where Sprint 1 flagged 5 packages "verify before deletion" but 2 (``generic_sync``, ``internal/op``) were never followed up — both deleted in PR 3 of this sprint. ``docs/audits/dead-code.md`` annotated with the final disposition table.


## [v0.6.13] — 2026-05-15
### Added
- Real-backend Playwright smoke harness in ``ui/tests/helpers/real-backend.ts``. ``realBackendTest`` is a ``test`` clone that auto-skips every test when ``POWERLAB_E2E_BASE`` is unset; with the env var pointing at a running gateway (e.g. ``http://192.168.18.86:8765``), specs hit the actual backend. Companion helpers: ``loginAndGetToken``, ``loginIntoPage`` (with session-level cache to avoid login rate-limit), and ``REAL_BACKEND_BASE`` for absolute-URL ergonomics. Bug class targeted: mock-driven Playwright specs prove the UI renders given a fixed shape but DON'T prove the endpoint exists. Production v0.6.12 cut surfaced exactly this gap — audit endpoints unreachable from the public port while 19/19 mocked specs were green. Example spec ``audit-pane.smoke.spec.ts`` demonstrates the pattern and asserts content-type vs HTML fallback.
### Changed
- Custom App page (/apps/new) restores the full visual form + 3-tab view switcher (Split / Form / YAML) — same UX as v0.6.11. Internally the form is now **one-way data flow** per ADR-0030: ``ComposeForm`` takes ``yaml: string`` + ``onChange`` (no ``$bindable``); the form's display derives from ``$derived(viewFromYaml(yaml))`` and every edit emits a fresh YAML through pure mutators in ``$lib/utils/compose-mutate.ts``. Long-form entries (volumes/ports/devices with ``{type, source, target}``) are preserved per-entry — editing one field keeps the entry long-form; this is the root-cause fix for the ``[object Object]`` bug class (#332). 33 unit tests in ``compose-mutate.test.ts`` lock the mutation contract, including a parametric guard that no edit across diverse real-world YAMLs produces ``[object Object]``.
- Custom App page (``/apps/new``) is now **YAML-first**. The bidirectional ``ComposeForm`` + ``ComposeModel`` round-trip that was the source of the "[object Object]" bug class for long-form volume / device / port objects is gone. The page is a YAML editor on the left + a read-only summary panel (``YAMLPreview.svelte``) on the right, derived via ``$derived`` from the live YAML. Single source of truth, one-way data flow, Svelte 5 best-practice (ADR-0030). Install now calls the unified ``installComposeApp(yamlText)`` helper (same path as Community Install) instead of bypassing it with a raw ``api.postYaml``. Page LOC dropped from 676 → ~360. ``ComposeForm.svelte``, ``ComposeModel`` interface, ``orchestrator.sync.test.ts`` and ``fork-volume-shape.test.ts`` removed; replaced by 8 new ``YAMLPreview.test.ts`` cases including parametric "[object Object]" guards against diverse real-world compose shapes. Closes #374.
### Fixed
- Audit log now captures ``user_id`` / ``username`` for ALL public traffic, not just the gateway-owned audit endpoints. Previously, every request that proxied to a backend service (the bulk of UI traffic) recorded null user fields because the JWT decoder ran only on /v1/audit/*. A new ``jwt.HTTPDecodeOnly`` middleware decodes the bearer token (if present and valid) and populates the user_id / user_name request headers without enforcing auth — enforcement stays downstream (backend services or HTTPJWT on audit endpoints). Wired as the inner layer of the gateway public mux so audit middleware sees populated headers. 5 unit tests on the new middleware: valid token, missing token, invalid token, raw vs Bearer prefix, query token fallback. Sprint 18 #376 hardening (one of the v0.6.12 cut findings).
- Install / log-streaming endpoints (Server-Sent Events) no longer appear to hang. The audit middleware wraps the ResponseWriter to capture status codes; the wrapper was missing the ``Flusher`` and ``Hijacker`` interfaces, so downstream SSE handlers and reverse proxies could not flush chunks — the install modal showed "loading…" indefinitely until the stream closed at the end. Caught during v0.6.13-stg manual verify: app install flow felt broken because logs never appeared. Forwarding ``Flush()`` to the underlying writer restores chunk delivery; ``Hijack()`` is also forwarded so WebSocket upgrades behind the audit middleware keep working. 2 regression tests lock the contract.
- App install now also sweeps Docker-auto-renamed orphans BEFORE running ``service.Up(...)`` (mirror of the Uninstall-side sweep from #365). Without this, an orphan left by a previous failed / interrupted install — or one created manually for debugging — blocks the next install attempt with ``Error response from daemon: No such container: <sha>``. Symmetric guard. The cleanup is best-effort: failure to remove an orphan logs and continues. Closes #372.
- The installer (``install.sh`` emitted by ``scripts/package-linux.sh``) now refuses to install a tarball whose version starts with ``0.0.0-`` (dev / CI / e2e test artifacts). Without this gate, a misplaced dev tarball can land on a production host and the panel will report a non-release version forever — the auto-updater also breaks because the comparison logic doesn't recognise ``0.0.0-e2e`` as upgradable. Override with ``POWERLAB_ALLOW_DEV_BUILD=1`` for local integration testing. A new ``scripts/check-install-refuses-dev-bundle_test.sh`` exercises the gate against legitimate + dev versions and the override.
- Install modal SSE log stream + phase progress bar are live again. Two stacked bugs in the gateway → app-management response chain were buffering ``text/event-stream`` chunks until the connection closed: (1) the audit-middleware response-writer wrapper did not forward ``Flush()`` / ``Hijack()`` to the underlying writer, and (2) the Echo ``Gzip()`` middleware on the app-management v2 router compressed every response — including SSE — into a deflate stream that batched bytes before emitting. Browsers always send ``Accept-Encoding: gzip``, so the second bug hit every real install; ``curl`` without that header (and Playwright's ``page.route()`` mocks) silently masked it. Fix forwards Flusher/Hijacker through the audit wrapper and adds a ``GzipWithConfig{Skipper}`` that bypasses any path ending in ``/logs``. Adds ``scripts/check-sse-not-gzipped.sh`` as a pre-tag release gate so this class of regression fails the acceptance pass instead of shipping.
- Settings → About update-check no longer surfaces a red error banner for a single transient failure (GitHub timeout, captive portal, etc.). Failure-UX state machine: ≤2 failures → silent; 3+ failures with last success ≥6h old → subtle amber "Update check is unavailable right now — last checked Xm ago". Between the two, a small grey "Background check did not complete; the system is healthy" sits in place of the previous opaque "Could not reach the release manifest: …" string. Closes #371.
- Gateway's ``-w`` flag default and the systemd unit's ``-w`` argument now share a single source of truth (``constants.DefaultWWWPath``). Previously they diverged: main.go defaulted to ``$DefaultDataPath/www`` (= ``/var/lib/powerlab/www``) while the systemd unit emitted by ``scripts/package-linux.sh`` hard-coded ``/usr/share/powerlab/www``. A dev hot-swap or manual binary run wrote the UI to the WRONG path; the running gateway silently kept serving the stale bundle. The AuditPane "disappeared" from the user's view during v0.6.12 verify because of this drift. Three new regression tests lock the contract: static handler refuses to silently fall back to an alternate root, refuses to 2xx for a missing root, and the platform constant matches the systemd unit's ``-w`` value on Linux.


## [v0.6.12] — 2026-05-14
### Added
- Echo middleware ``audit.Middleware(rec, opts)`` (Sprint 16 #357 step B1b). Captures method, path, status, latency, user_id/username from JWT headers, remote IP (loopback collapsed to canonical "loopback" sentinel per ADR-0033 PII), and X-Request-Id passthrough. Strips ``token=`` from query before storage so the EventSource JWT fallback never persists. Non-blocking on the hot path: ``Submit()`` is at most µs and cannot block the response. Optional ``Skipper`` lets services bypass recording for high-frequency health checks. 7 test cases including JWT capture, query-token strip, loopback sentinel, request-id passthrough, skipper bypass, and edge-case user_id parsing.
- Wire audit pipeline into the gateway service (Sprint 16 #357 B1c). ``main.go`` initialises ``audit.Service`` at boot under ``/var/lib/powerlab/gateway/audit.db`` (persistent — NOT ``/run/powerlab`` which is tmpfs); ``fx`` provides it to ``ManagementRoute``; the route mounts the echo middleware globally (with skipper for ``/ping`` + ``/v1/audit/*``) and registers ``/v1/audit/recent`` + ``/v1/audit/stats``. Lifecycle hook drains recorder + retention + DB on shutdown. Audit init failure is non-fatal (gateway still serves; observability is not a hard dependency). Live-verified on .142: ``/var/lib/powerlab/gateway/audit.db`` created with the right schema. **Scope limitation:** the gateway's public-facing route bundle uses stdlib ``http.ServeMux`` not echo, so this slice does NOT yet capture end-user traffic — only service-to-gateway internal management calls. Follow-up issue filed for a stdlib middleware variant to cover the public side.
- ADR-0035 — proposed migration of audit storage from SQLite to JSONL + in-memory ring buffer. Aligns PowerLab audit with Kubernetes / Vault / Consul / CloudTrail convention; gives operators ``tail -F | jq`` from SSH; simplifies the standalone observability service design (ADR-0034) by letting it consume a file rather than open a second SQLite handle. Supersedes the storage section of ADR-0033 (middleware shape stays). Implementation deferred — Sprint 16 ships SQLite-as-built; migration PR lands BEFORE ADR-0034 implementation begins.
- godoc: complete coverage of ``backend/common/utils/random`` package (Sprint 16 #359 — first slice of the godoc 100% initiative redo). Adds package-level doc explaining the math/rand vs crypto/rand contract, fixes the stale ``GetRandomName`` comment on ``Name`` (CasaOS rebrand leftover), documents ``String`` (was undocumented), and rewrites the deprecation note on ``RandomString`` to follow the standard godoc convention (lead with function name). Doc-only — no behavior change, all tests still pass, ``go vet`` clean. Replaces the rejected ``e8b597f`` AI commit which mixed docs with breaking refactors (renamed exported funcs, dropped deprecated wrappers, added nolint bypasses).
- Audit read endpoints (Sprint 16 #357 B1e): ``GET /v1/audit/recent`` paginated descending by timestamp with optional filters (``limit`` default 100, max 1000; ``user_id`` filter; ``since`` cursor as Unix µs); and ``GET /v1/audit/stats`` returning row count, oldest/newest timestamps, file size in bytes, and on-disk path. Both return the standard ``{data: ...}`` envelope so the UI Envelope<T> pattern works without per-endpoint adapter. Empty table on Stats returns zero timestamps (operator opened a fresh box). 9 new tests across DB query layer + handler smoke. Total audit pkg: 28/28 race-clean. Endpoints not yet wired to a service — that happens in B1c (gateway integration).
- Playwright E2E for the AuditPane (Sprint 16 #357 B1f follow-on). Mocks ``/v1/audit/recent`` + ``/v1/audit/stats`` with fixture rows (one authenticated, one loopback with null user) and asserts: stats card row count formats with ``toLocaleString``, table renders both rows, status colors apply (200/401), the literal ``loopback`` sentinel survives the round-trip, the em-dash null fallback renders, and zero console errors fire during render. A second case stubs an empty backend and asserts the "no audit records yet" empty state. 19/19 Playwright suite green.
- Audit retention runner (Sprint 16 #357 B1d). Periodic goroutine that prunes the audit DB to bound disk usage per ADR-0033. Two limits applied per cycle: MaxAge (default 30 days) drops rows older than now-MaxAge; MaxRows (default disabled, 0) caps row count and drops oldest first when over. Default Interval 1h. Calls ``PRAGMA wal_checkpoint(TRUNCATE)`` after each prune so disk pages are reclaimed without operator action (suppressed by SkipWALCheckpoint for tests that inspect WAL state). ``RunOnce(ctx)`` exposed for tests + future operator action. ``Close()`` idempotent + drain-safe like Recorder. 6 tests: age-only, row-cap-only, no-op, goroutine fires on interval, Close idempotency, defaults applied. All race-clean.
- New ``backend/common/utils/audit/`` package: async audit recorder with SQLite backing store (Sprint 16 #357 / ADR-0033 implementation step B1a). Pure-Go SQLite driver (``modernc.org/sqlite``) — no CGO. ``OpenDB(path)`` creates the schema (one table, 2 indexes) idempotently with WAL journaling. ``Recorder`` exposes a non-blocking ``Submit()`` on the hot path — channel-full drops oldest with an atomic counter, NEVER blocks the request. Writer goroutine batches up to 50 rows per transaction with a 200ms max-latency window. ``PruneByAge`` and ``PruneToMaxRows`` drive the hourly retention goroutine (wired in a follow-up step). 7-case test suite covers schema migration idempotency, drain-to-DB, drop-oldest backpressure, age-based prune, row-count-based prune, and the canonical ``loopback`` sentinel for RemoteIP. Race-detector clean. Foundation only — Echo middleware + service wiring + read endpoints + Settings UI come in B1b/B1c/B1d/B1e.
- ADR-0034 — standalone observability + MCP service design. Locks the Sprint 17 critical-path: a new ``powerlab-observability`` binary that runs independently of every other PowerLab service (port :9090 default), reads per-service audit DBs read-only, speaks 3 transports (HTTP+SSE for browser, MCP/stdio for local agents, MCP/HTTP for remote), exposes typed resources via custom URI schemes (``audit://``, ``journal://``, ``apps://``, ``system://``, ``containers://``), and gates actions in three tiers (read / auth / admin). Companion research note ``docs/research/mcp-linux-server-landscape-2026.md`` surveys 12+ existing OSS MCP servers for Linux/homelab management to identify the unfilled gap PowerLab targets. Pivot-friendly without committing — see ``project_mcp_linux_connector_vision`` memory for the broader MCP-for-Linux story this design unblocks.
- Settings → Audit pane (Sprint 16 #357 B1f). Surfaces the audit log recorded by the gateway middleware: who hit what endpoint, when, with which status, from which IP. Stats card shows row count, oldest/newest timestamps, and on-disk file size. Table renders newest-first with status-color coding (green 2xx, blue 3xx, amber 4xx, red 5xx). New ``audit.ts`` API module wraps ``getAuditRecent(opts)`` and ``getAuditStats()``, with 7 contract tests pinning the wire shape against the backend. Pane component test (5 cases) covers stats render, empty state, error banner, 401 suppression (handled by C3 centrally), and nullable user_id/username rendering. Sidebar gets a new "Audit" entry between Security and About. Total vitest after this slice: 524/525.
- ADR-0033 — audit middleware design (Sprint 16 #357 prep). Locks the schema (per-service SQLite, 11 columns, 2 indexes), retention policy (30d / 50MB whichever first), PII stance (full IP by default, toggle for /24 truncation), performance contract (async writer, < 20µs middleware overhead, drop-oldest on channel pressure), and the /v1/audit/recent + /v1/audit/stats read API. Implementation in #357 follows this contract.
### Changed
- Audit storage migrated from per-service SQLite to JSONL + in-memory ring buffer (per ADR-0035). The recorder writes ``/var/log/powerlab/audit.jsonl`` (rotated by lumberjack into ``.1.gz``, ``.2.gz``, …); the UI hot path is served from a 1000-record ring buffer kept in process. Removes ``modernc.org/sqlite`` from ``backend/common`` — every other backend's ``go.sum`` is freed from the transitive weight that broke CI on PR #362. Same ``/v1/audit/recent`` and ``/v1/audit/stats`` API surface, now mounted on the gateway's PUBLIC mux with stdlib JWT validation (mirrors Echo loopback-skip behaviour) — fixes the bug where the AuditPane could not load any data in the browser because the endpoints lived only on the management port. Stdlib audit middleware also captures end-user traffic (the original Sprint 16 scope limitation documented in B1c). Wire shape changed: snake_case JSON (``ts``, ``ts_us``, ``latency_us``, ``user_id``, ``username``, ``remote_ip``, ``request_id``) replaces the implicit PascalCase emitted by tagless ``encoding/json``. Operators on SSH gain ``tail -F /var/log/powerlab/audit.jsonl | jq``. Existing ``audit.db`` files on disk are ignored — history starts fresh after upgrade (homelab audit is not regulatorily binding).
### Fixed
- Uninstalling an app no longer leaves Docker auto-renamed orphan containers (pattern ``<12-hex>_<project>``) that block the next reinstall with ``No such container: <sha>`` errors. compose-go's ``RemoveOrphans`` misses these because the renamed container keeps the project label while its service-name still matches the active service. A new best-effort sweep in ``Uninstall()`` catches and force-removes them after ``service.Down``.
- In-UI session-expired UX: when any api call returns 401, user is now logged out + sees a clear "Session expired — please sign in again" toast (Sprint 16 C3). Before, the calling store's generic catch-all surfaced opaque messages like "Could not reach the release manifest: Unauthorized" with no hint about the real cause — hit live during v0.6.11 testing with a stale JWT in localStorage. New centralised hook ``onAuthError(handler)`` in ``$lib/api/client``: registers a callback fired on every 401 response, with a single in-flight guard so parallel failing requests trigger one toast not N. The auth store subscribes once on module init. 5 contract tests lock the behaviour (fires on 401, NOT on 200/500, unsubscribe works, multiple subscribers).


## [v0.6.11] — 2026-05-14
### Added
- ESLint flat config (``ui/eslint.config.js``) banning raw ``fetch()`` calls in stores, routes, components, and the api client folder (Sprint 15 #353). Routes through the api client at ``$lib/api/client`` so the JWT ``Authorization`` header is always injected — closes the bug class that produced the v0.6.7 → v0.6.10 upgrade-401 saga (#352). 5 intentional raw- fetch callers (the api client itself + 4 public-probe paths) are explicitly allow-listed with justification comments. New ``npm run lint`` script + ``scripts/check-eslint-fetch-ban_test.sh`` meta-test asserts the rule fires in both directions (clean main passes; contrived violator triggers exactly one error). Wired into the Frontend CI job between the manifest-size check and svelte-check.
- Playwright E2E (``upgrade-flow.spec.ts``) for the in-UI upgrade button (Sprint 15 L2 — follow-up to #344 install-flow). Drives Settings → AboutPane, mocks the updater check + install POST + version poll, and asserts (a) the install POST carries a non-empty ``Authorization`` header — the bug-class signature that caused v0.6.7 → v0.6.10 — and (b) no ``effect_update_depth_exceeded`` console error appears. Sanity-verified by temporarily reverting the v0.6.10 ``api.post`` fix: the spec correctly FAILS with raw fetch, PASSES with the fix. Mandatory pre-tag gate.
### Fixed
- Backend JWT middleware now accepts the standard RFC 6750 ``Authorization: Bearer <token>`` form in addition to the legacy raw-token form (#342). Before this fix, sending the canonical Bearer prefix made the JWT validator receive the literal string "Bearer abc…" and reject it as malformed — 401. The 6 inline ``TokenLookupFuncs`` extractors across gateway, app-management, message-bus, core, and the shared ``common/utils/jwt`` helper now all call the centralised ``jwt.ExtractTokenFromRequest`` which case-insensitively strips ``Bearer ``, falls back to ``?token=`` query for EventSource, and is locked by a 12-case table test. Live-verified on .142: raw → 200, ``Bearer`` → 200 (all cases), no header → 401. Defense-in-depth alongside Sprint 15 L1 (#353 ESLint) and L2 (#355 Playwright upgrade-flow).


## [v0.6.10] — 2026-05-14
### Fixed
- In-UI upgrade button no longer 401s. ``upgradeProgress.start()`` was calling ``fetch()`` directly against ``/v1/powerlab-update/install``, bypassing the api client and never attaching the JWT Authorization header — every click returned ``HTTP 401 Unauthorized`` from the gateway. Routed through ``api.post`` so the header is injected automatically. Locked by a contract regression test that asserts the Authorization header is present on the POST. Bug landed in v0.6.7 (PR #339) and persisted through v0.6.8 + v0.6.9.


## [v0.6.9] — 2026-05-14
### Added
- Shared `<InstallModal>` component in `lib/components/apps/` (Sprint 14 #345, foundation). Owns the entire install-lifecycle modal surface — Preparing indeterminate, determinate progress, log streamer, success/error/timeout cards, button row, minimize. Both `/apps` (Community Install) and `/apps/new` (Custom App) will consume this in follow-up PRs; this commit lands the component + 13 unit tests covering each phase + click handlers but does not yet replace the bespoke modals. Eliminates the visual + behavioural divergence that v0.6.7 exposed once both pages migrate.
- Playwright E2E (`install-flow.spec.ts`) for the Custom App install flow (#344). Locks the v0.6.7 → v0.6.8 bug class: install modal stuck on "Preparing" because of (1) SSE multi-line events silently dropped, (2) channel-never-closed leaving ``event: end`` unsent, and (3) ``effect_update_depth_exceeded`` loop in the InstallState mirror. The spec mocks POST deploy, fulfills a multi-event SSE body, polls the installed-app list, asserts the InstallModal reaches its Success terminal, and treats any ``effect_update_depth_exceeded`` console error as a hard fail. Mandatory pre-tag gate per the release coverage rule.
### Changed
- **BREAKING (wire format).** The Core service prefix moves from ``/v2/casaos`` → ``/v2/powerlab-core`` (#252 audit follow-up). External clients that called ``/v2/casaos/health/*`` or ``/v2/casaos/file/upload`` must update the prefix. The PowerLab UI in this release already calls the new path. The embedded OpenAPI moved too: ``backend/core/api/casaos/openapi.yaml`` → ``backend/core/api/core/openapi.yaml``; the generated Go package name went from ``codegen.casaos_api.go`` → ``codegen.core_api.go``. Backend route registration is driven from the YAML ``servers[0].url`` field, so the gateway picks up the new prefix automatically at startup. No flag, no fallback — the legacy ``/v2/casaos`` prefix returns 404.
### Fixed
- Forking an app into a Custom App no longer surfaces "[object Object]" in the volumes (and devices) form section when the upstream compose used the long-form spec (`{type: bind, source: ..., target: ...}` rather than the short `host:container` string). syncYamlToForm now handles both shapes — long-form object → reads .source/.target; short-form string → existing split-on-colon. Same dual-form handling on devices. Issue #332. Locked by 2 regression tests in fork-volume-shape.test.ts.
- Chown bind-mount source directories to the container's ``user:`` field during install (#334). Docker auto-creates missing host paths as ``root:root``; postgres / mariadb / redis images running as non-root then fail ``initdb`` with ``Operation not permitted``. Catalog apps such as blinko and activepieces (which declare ``user: 1000:1000`` on their db services) now reach ``healthy`` on first install instead of restart-looping forever.


## [v0.6.8] — 2026-05-13
### Fixed
- Install modal no longer sits on "Preparing…" forever; launchpad ghost tile no longer stays on the indeterminate spinner. Root cause was an SSE protocol violation in the install-log task service that has existed since v0.1.0 (commit d8c1214, 2026-05-07). `Task.Subscribe()` replayed the entire accumulated log buffer as a single channel message; the SSE route handler then wrote it as `data: <multi-line>\n\n`. Per HTML5 EventSource spec, any line inside a `data:` block that does not start with a known field name (`data:`, `event:`, `id:`, `retry:`) is silently dropped. Result: the browser only saw the first line and lost every `Phase 1/3:`, `Phase 2/3:`, `Phase 3/3:` marker thereafter. The UI's InstallProgressBar (#329, v0.6.6) explicitly gates the determinate progress bar on a `currentPhase` parsed from those markers — without them it stayed on the indeterminate Preparing state forever, and the launchpad ghost tile (#330) stayed on its spinner forever for the same reason. Fix: `Task.Write()` and `Task.Subscribe()` now split content on `\n` and emit one line per channel message (matching the SSE protocol contract one event = one line). Empty lines suppressed. Trailing partial line (no `\n`) still delivered. 9 regression tests in `backend/app-management/service/task_test.go` lock the channel contract (Subscribe catch-up split, Write split, trailing partial, empty-line skip, Finish closes subscribers, backpressure non-blocking, concurrent subscribers see same stream). Verified on user's box with `curl -sN` against `/v2/app_management/compose/task/<id>/logs` — every line now carries its own `data:` prefix and `\n\n` terminator.


## [v0.6.7] — 2026-05-13
### Added
- `POWERLAB_SKIP_SYNC=1` env var on `install.sh` to skip the post-install community-catalog refresh. The catalog refresh (#326 introduced in v0.6.5) `git clone --depth=1`s upstream `getumbrel/umbrel-apps` and re-emits the on-disk catalog tree — the dominant cost of a v0.6.5+ upgrade (~30–60s on first run; the tarball download itself is only ~5 MB / sub-second). The escape is useful for air-gapped boxes, fast offline upgrades where the bundled catalog is good enough, and CI / packaging tests that shouldn't depend on live upstream state. Documented in `docs/UPDATE_MANIFEST.md` under the new "Upgrade duration" section; regression-tested in `scripts/check-package-linux-sync-catalog_test.sh`.
- Full-screen "PowerLab atualizando" overlay during an in-flight in-UI upgrade. When the user clicks "Upgrade to vX.Y.Z" in settings → AboutPane, the overlay covers the entire UI while install.sh runs on the host (~1 min window). Polls /v1/powerlab/version every 3 seconds; suppresses the transient 502/503 errors gateway produces while restarting (previously these surfaced as a "500 Internal Error" toast / dashboard noise during the v0.6.5 → v0.6.6 manual upgrade test). When the polled version equals the target version, transitions to a success state and auto-reloads after 2 seconds so the user lands on the new bundle. 5-minute timeout falls back to a manual-intervention error message. State machine in `lib/stores/upgradeProgress.svelte.ts` is test-covered (10 cases): idle/starting/restarting/success/error transitions, POST 202 → restarting, non-202 → error, timeout, network-error suppression during restart, reset(), isOverlayActive gate. New UpgradeProgressOverlay.svelte mounted in the root layout. pt-BR / en / es i18n added.
### Fixed
- Version-mismatch banner now closeable (X button) and has a cache-busted "Forçar atualização" button that adds `?powerlab_v=<ts>` to the URL so cache-aggressive proxies and the browser cache cannot serve the same stale JS bundle on reload. After 2 force-reload attempts that did not resolve the mismatch (counter persisted in localStorage), the banner text switches to "Bundle desatualizado no servidor" with admin/install.sh guidance rather than wheel-spinning. Surfaced post-v0.6.6 cut: the user saw "UI v0.3.1 vs Server v0.6.5" because the UI was rebuilt locally during the v0.6.6 E2E session without `POWERLAB_VERSION` env set, so Vite stamped the bundle with `ui/package.json` version which had stayed at 0.3.1 since the v0.3.1 era (commit d8c1214, 2026-05-07) — 30+ tags later. Same PR also lands the defense-in-depth that prevents the bug class from shipping again: L1 (`npm version $VERSION` in scripts/package-linux.sh before every build), L2 (CI gate `check-ui-package-version-fresh_test.sh` fails when pkg.json is older than the latest git tag), L3 (sanity grep of the built JS for the version literal in scripts/package-linux.sh that aborts the tarball seal if the wrong literal landed). Plus `scripts/prepare-release.sh` automates the cut-time `changie batch` + `npm version` + staging steps so the pkg.json bump cannot be forgotten in the release commit. Sandboxed regression test `scripts/prepare-release_test.sh` locks the release-auth invariant (script never commits, pushes, or tags). 13 unit tests in versionHandshake.test.ts lock the new banner state machine (dismiss, forceReload with query-param-preserving cache-buster, persistentFailure threshold, reloadAttempts counter reset on match). pt-BR / en / es i18n updated with the new strings.


## [v0.6.6] — 2026-05-13
### Added
- Install progress now visible on the launchpad as an iOS-style ghost tile (#251, Sprint 13.2.2). When the user closes the install modal without canceling, the install keeps running and a tile appears at the position the app will occupy in the launchpad — faded icon + circular progress ring around it. Indeterminate spinner before the first `Phase N/M` SSE marker arrives; switches to a determinate arc once `currentPhase` is set. Status badge (checkmark / X) appears bottom-right on terminal phases (success / error / timeout). Click the ghost tile to restore the modal at `/apps?install=<id>`. New cross-page store `$lib/stores/install-state.svelte` keeps the in-flight installs addressable by id; both `/apps` modal (writer) and `/` launchpad (reader) share it. 17 unit tests across the store + the InstallingTile component lock the matrix.
### Changed
- Extract install log display into reusable `LogStreamer.svelte` (Sprint 13.2.3, #252) — single source of truth for the live install-log pre with auto-scroll, scroll-pause-on-manual-scroll, and label header. Wired into `routes/apps/+page.svelte` Community Install modal. `routes/apps/new/+page.svelte` (Custom App) keeps its bespoke terminal-style display for now; future Sprint 14 PR can swap it for LogStreamer if visual parity is wanted. 7 unit tests cover rendering, empty logs, custom label, autoscroll-on-by-default, pause-on-scroll-up, resume-on-scroll-to-bottom, and heightClass prop forwarding. Also drops the MinimizeBanner pill — the launchpad InstallingTile ghost tile from Sprint 13.2.2 (#251) covers that role now; less visual noise and one place to look for in-flight installs.
- Scaffold Phase 3 + Phase 4 of #150 backend coverage (Sprint 13.5, carry from Sprint 11). Phase 3 adds `//go:build integration`-tagged Docker tests in `backend/app-management/service/docker_integration_test.go` (Ping + ImageList against the real socket) plus a new `backend-integration` CI job that runs `go test -tags=integration` after the unit-test job is green. Phase 4 adds `//go:build linux && fuse`-tagged scaffold in `backend/local-storage/service/fuse_integration_test.go` that imports bazil.org/fuse and exercises the mount-option API surface, plus a new `backend-fuse` CI job that apt-installs libfuse and runs the tagged tests. Deeper end-to-end install + actual fuse mount tests follow in Sprint 14 once the privileged-runner setup lands; this PR establishes the build-tag + CI-gate pattern.
### Fixed
- Two install-runtime bugs surfaced by user testing during v0.6.5 + comprehensive catalog audit (#253, refs #307, #244). (a) gitingest rejected browser requests with "Invalid host header" because `ALLOWED_HOSTS=${DEVICE_DOMAIN_NAME},${DEVICE_HOSTNAME},${APP_GITINGEST_LOCAL_IPS}` stayed literal — fix: substitute these placeholders with `*` in env values that are PURELY a comma-separated host list (59 apps affected). (b) adventurelog returned "bad request" because `ORIGIN="http://${DEVICE_DOMAIN_NAME}:8015"` got my first-pass fix turned into `http://*:8015` (invalid URL); refined the substitution to skip URL-embedded placeholders entirely — install-time substitution (Sprint 14) will handle URL contexts. (c) 89 multi-service apps had no `x-powerlab.main` set so the backend fell back to "first alphabetical service" — wrong for apps like agora (svcs: agora/filebrowser/nginx, real entry is `agora`). Fix: emit.go resolves `main` via app_proxy's APP_HOST using the existing extractAppProxyTarget function, with deterministic shell-var fallback (sorted, not Go-map-iter randomized — caught by audit when agora flipped between `filebrowser` and `agora` between sync runs). Full audit documented in `docs/audits/community-catalog-integration-audit-2026-05-13.md`.
- Install-time secret substitution for ${APP_SEED}, ${APP_PASSWORD}, and sibling-app DB credentials (Sprint 13.4.x, #254, refs #307). v0.6.5 audit caught 49 apps with literal ${APP_SEED} (NextAuth / JWT signing secrets) and 39 with ${APP_PASSWORD} (admin/db passwords) in their compose env. The placeholders stayed literal in PowerLab → apps either auto-init with the literal string as the "secret" (security disaster) or fail on parse. Fix: new `service.SubstituteSecrets(yaml, appID, secretsDir)` generates per-app random secrets the FIRST install, persists at `/var/lib/powerlab/apps/secrets/<id>.env` (mode 0600 in dir 0700), and reuses them on every subsequent install — so encrypted user data and JWT-signed sessions survive upgrades. Different apps get different secrets (compromising one doesn't compromise all). Wired into the `InstallComposeApp` route handler BEFORE compose-go parses; best-effort fallback skips substitution silently rather than blocking install when the YAML has no top-level `name:` or the secrets file is unreadable. 9 unit tests in `install_secrets_test.go` cover: no placeholders (no-op), generation + 0600 perms, idempotency, same-ref-same-value, per-app distinctness, hex length per placeholder kind, sibling DB triple, unrelated placeholder preservation, file-delete rotation.
- Install progress bar now appears immediately when install starts, not late after first `Phase N/M` SSE marker arrives — addresses v0.6.5 user feedback "a barra de loading aparece somente no final nao no inicio da instalacao". Root cause: the determinate progress bar was gated by `currentPhase` (parsed from SSE log markers) being truthy; during the HTTP POST + early SSE seconds there was no marker yet, so the bar didn't render at all — by the time the first marker landed, the install was often already at 60%+, giving the "bar shows at the end" impression. Fix: extracted into `lib/components/apps/InstallProgressBar.svelte` with explicit indeterminate vs determinate states. Indeterminate (animated emerald stripe sliding left→right via the new `install-progress-indeterminate` CSS keyframe) shows from `installing` onwards with a "Preparing…" label; switches to determinate the moment `currentPhase` arrives. 7 unit tests lock the state machine.


## [v0.6.5] — 2026-05-13
### Added
- Source badge surfaces in the **launchpad** and the **store** tiles (#245). v0.6.x Phase 5 (#313) only added the badge to the standalone `AppCard.svelte` component, which neither `routes/+page.svelte` (launchpad) nor `routes/apps/+page.svelte` (store) actually use — both pages render tiles inline. Result: the badge was effectively invisible in production. This change adds the same `detectAppSource()` + `appSourceLabel()` calls to both pages inline tile templates: launchpad shows the source as a low-contrast uppercase tag below the icon (mirroring the existing "CUSTOM" tag style for visual consistency); store list view shows it inline with the developer name via middle-dot separator; store card view shows it as a chip next to the category. Lets the user tell at a glance which catalog (umbrel / casaos / big-bear / store) each app came from — addresses the "fico na duvida ao testar" feedback from the v0.6.4 cycle.
- Ship `powerlab-sync-catalog` binary in `/usr/bin` (Sprint 13.3, #248, refs #307). `scripts/package-linux.sh` cross-compiles the sync-catalog tool alongside the 6 backend services and the bundled `install.sh` runs it post-install (best-effort, 60s timeout, git as soft-dep) to refresh `/var/lib/powerlab/community-catalog` against current upstream. This closes the v0.6.x bug class permanently: through v0.6.1→v0.6.4 the catalog kept being stale because the in-repo `community-catalog/` used to bake into the release tarball was fixed AFTER the binary fix shipped — leaving upgrades with a binary that knew how to transform Umbrel YAMLs but a tarball that carried un-transformed YAMLs from the previous sync. Now the binary refreshes the catalog on install, decoupling catalog freshness from tarball freshness entirely. If git is missing OR GitHub is unreachable OR the sync errors for any reason, the install completes with the bundled catalog as fallback — never fails the install path. Regression test `scripts/check-package-linux-sync-catalog_test.sh` locks the wiring.
### Fixed
- CasaOS-curated apps with `scheme: https` (crafty, mineos-node, netbird, obsidian, openclaw, unifi-network-application) and the openclaw-specific `?token=casaos` legacy auth-proxy artifact now open correctly from the launchpad (#244, Sprint 13.4). The launchpad's `openApp()` previously hard-coded `http://` ignoring the catalog's `scheme:` field, so the 6 HTTPS-serving apps opened blank tabs (browser hit the right port over the wrong protocol). And openclaw's catalog metadata carries `index: ?token=casaos` — a stray query string from CasaOS's pre-fork token-aware proxy that PowerLab doesn't replicate; passing it through just left the underlying app with a token query it didn't know what to do with. Fix: extract URL construction into pure `buildAppURL(hostname, storeInfo)` in `$lib/utils/app-url.ts`, honour `scheme:`, strip the literal `?token=casaos`, preserve other `index:` values (paths/anchors). 8 unit tests cover the full matrix.


## [v0.6.4] — 2026-05-12
### Fixed
- Umbrel-catalog import — Phase 8 (#307) — fixes "click app tile, new window opens, page never loads" for every Umbrel app. Two root causes, both locked by failing-first tests: (a) `${APP_*_PORT}` placeholders were substituted with sequential 18000+ integers, but `x-powerlab.port_map` (what the launchpad uses to build the click-through URL) carries the manifest's `port:` field — so the URL pointed at port 8788 while the container listened on 18000. Fix: feed `manifest.Port` into `substitutePortPlaceholders` as the base value, so the compose host port matches `port_map`. (b) Apps whose only port-routing signal was Umbrel's `app_proxy` service (no `ports:` in the real service — e.g. `enclosed`) lost ALL external accessibility when we stripped app_proxy. Fix: extract app_proxy's `APP_HOST`/`APP_PORT` env BEFORE the strip and synthesize an equivalent `ports: ["<manifest.Port>:<APP_PORT>"]` on the target service. Also handles brace-less `$APP_FOO_PORT` shell-var form (synapse-style upstream) that the previous `${...}`-only regex missed. Proactive surface against future Umbrel upstream changes: new test `TestProductionCatalog_NoUnknownPlaceholdersInDangerousPositions` scans every emitted YAML's volume/port positions and fails if any `${VAR}` or `$VAR` survives — early-warning gate that flags new upstream patterns BEFORE the user does, with the offending pattern reported in the failure message so the fix lands in `transform.go`. End-to-end verified on user's box: `curl http://host:8788` after rebuild returns HTTP 200 from the enclosed container (was empty/timeout before).


## [v0.6.3] — 2026-05-12
### Fixed
- Umbrel-catalog import — Phase 7.5 (#307) — extends the volume placeholder substitution to catch two more Umbrel-ecosystem variables surfaced by the new CI gate. `${APP_<NAME>_DATA_DIR}` (sibling-app data dirs — e.g. an app referencing `${APP_LIGHTNING_NODE_DATA_DIR}`) and `${UMBREL_ROOT}` (Umbrel installation root, used by apps that read from the shared `/data/storage/downloads` tree) now substitute to PowerLab paths so the catalog parses. The new `production_catalog_test.go` walks every `community-catalog/Apps/<id>/ docker-compose.yml` in the repo and feeds it through the SAME loader BuildCatalog uses at runtime — a CI gate that blocks any release carrying a broken catalog. Caught 18 apps that would have shipped broken in v0.6.3 (agora, audiobookshelf, bazarr, downtify, duplicati, emby, file-browser, home-assistant, jackett, …). Catalog re-emitted + verified: 245 apps in `community-catalog/Apps/`, 245/245 parse.


## [v0.6.2] — 2026-05-12
### Fixed
- Umbrel-catalog import — Phase 7 (#307) — the v0.6.1 release shipped a populated `community-catalog/` directory but 0 Umbrel apps surfaced in the store, because every emitted compose YAML carried Umbrel-runtime patterns that PowerLab's compose-go validator rejects: `services.app_proxy` without an `image:` (Umbrel runtime helper), `${APP_DATA_DIR}` un-substituted in volume references (compose-go treats as undefined named-volume), `env_file:` pointing at paths that don't exist at parse time, `${APP_*_PORT}` placeholders inside `ports:` (compose-go port parser is strict), and missing top-level `name:` (compose project name fell back to a random temp-dir basename, so `BuildCatalog` keyed the app under names like `amazing_ubs` instead of `agent-zero` — present in the API but unreachable by id). `backend/sync-catalog/transform.go` rewrites the upstream compose to handle all five patterns at sync time. End-to-end verification on a v0.6.1 install: catalog grew from 162 (CasaOS-only) to 336 apps; 10 representative Umbrel-only ids (agent-zero, affine, enclosed, adventurelog, appsmith, audiobookshelf, excalidraw, 2fauth, activepieces, akaunting) all resolve by their hyphenated upstream ids. Regression locked by: `backend/sync-catalog/transform_test.go` (20+ unit tests covering each transform individually plus edge cases — empty input, malformed YAML, null/int volume entries, multi-service substitution, idempotency, substring-not-match) and `backend/app-management/service/umbrel_catalog_integration_test.go` (loader-level round trip: emit → `NewComposeAppFromYAML` parse → assert no error + correct project name). The integration test is the missing TDD piece that would have caught the v0.6.1 ship bug — it walks `testdata/umbrel-fixtures/Apps/*/docker-compose.yml` and feeds each through the SAME compose loader BuildCatalog uses.


## [v0.6.1] — 2026-05-12
### Added
- Umbrel-catalog wire-up — Phase 4.5 (#307) — registers the local `community-catalog/` directory as a third app store source so apps synced by the weekly sync workflow show up in the PowerLab catalog UI without further configuration. Dev conf (`backend/app-management/app-management.conf`) reads `../../community-catalog`; production sample + bundled `install.sh` create `/var/lib/powerlab/community-catalog/` and ship pre-bundled apps in the release tarball. `community-catalog/.gitkeep` is committed so the dir exists before the first sync run. Adds `backend/sync-catalog/sync-catalog` build artifact to .gitignore.
- AppCard now shows a discrete **source badge** in the metadata row indicating which upstream catalog the app came from (#307 Phase 5). Apple-style: tiny muted text after the category, never a colored pill or brand chip. Source is detected from `store_info.source.catalog` when the backend populates it (Umbrel-synced apps), otherwise inferred from the icon URL: `getumbrel.github.io` → umbrel, `IceWhaleTech/CasaOS-AppStore` → casaos, `bigbeartechworld/big-bear` → big-bear. Apps with no recognized source show the generic label "store" so every tile surfaces some provenance. The badge is a click-through link to the upstream repository when known (opens in a new tab; click does NOT bubble to the outer card handler). Native `title` tooltip carries the synced_at date when present.
### Fixed
- Password UX during onboarding (#306) — fixed off-by-one where the UI guard rejected `< 5` chars and the backend rejected `< 6`, surfacing the resulting validation failure as a generic "Failed to initialize the system" message. Both surfaces now agree on a minimum of 8 characters (locked by the new `MinPasswordLen` constant in `backend/user-service/route/v1/password.go` + the `MIN_PASSWORD_LEN` constant in SetupWizard.svelte; regression tests on each side pin the value). Backend error codes (PWD_IS_TOO_SIMPLE / KEY_NOT_EXIST / USER_EXIST) now map to specific i18n keys (`error.passTooShortBackend`, `error.setupKeyExpired`, `error.userExists`) in en/pt-BR/es, so the user sees a meaningful message instead of "check the backend logs". Helper text under the password input shows the rule upfront ("Mínimo 8 caracteres") and turns emerald when the floor is met; a checkmark icon appears inside the input. The Finish button is disabled until both gates pass (length + match) instead of only when fields are empty.
- Sync-umbrel-catalog workflow no longer fails silently when the `catalog` label is missing on the repo. `gh pr create --label X` validates the label upfront and exits non-zero if it doesn't exist — leaving the branch pushed but no PR open (the workflow's `|| true` swallowed the error). Split into a separate `gh pr edit --add-label` step that runs after PR creation, with the label step being best-effort. First-run symptom: first weekly sync after #317 pushed the branch but skipped the PR open.
### Internal
- Umbrel-catalog sync pipeline — Phase 1 (#307) — new `backend/sync-catalog/` binary clones the public Umbrel App Store catalog and emits PowerLab-native `appfile.json` per allowed app via a clean-room transform (ADR-0024). Four-tier filter (Tier 1 hard-reject `getumbrel/*` images + cross-app sibling env vars; Tier 2 soft-reject Bitcoin/Lightning categories by default; Tier 4 allow everything else). 23 unit tests; real-world dry-run against 330 upstream apps produces 241 allow / 44 hard-reject / 45 soft-reject / 0 parse-errors. Each emitted appfile carries a `source` provenance block (catalog, upstream commit, upstream path, transform version, synced_at) — answers the "debug origem" requirement. Icon URL is a pass-through to upstream `getumbrel.github.io/umbrel-apps-gallery/<id>/icon.svg`; descriptions are fetched from each app's OWN upstream README (the app maintainer's OSS-licensed content, not Umbrel's curated description), with optional `description-powerlab.md` maintainer override.
- Umbrel-catalog sync pipeline — Phase 3 (#307) — `.github/workflows/sync-umbrel-catalog.yml` runs Monday 06:00 UTC + on `workflow_dispatch`. Builds the sync-catalog binary from Phase 1 (#310), runs it against the upstream, commits the diff to a date-stamped `catalog/umbrel-sync-YYYY-MM-DD` branch and opens a PR with the filter summary + diff-stat for human review. No-op when there is no diff vs main; dry-run flag available via the manual dispatch UI. Concurrency group cancels older scheduled runs when a manual trigger fires. Local equivalent: `make sync-catalog` and `make sync-catalog-dry`.
- Umbrel-catalog sync pipeline — Phase 4 (#307) — refactors emit.go to CasaOS-compatible output shape so `backend/app-management/service.BuildCatalog` picks up the synced apps without further wiring. Layout changes from `apps/<id>/appfile.json` (custom format I'd invented in Phase 1 based on a misread of ADR-0021) to `Apps/<id>/docker-compose.yml` (verbatim upstream YAML with a top-level `x-powerlab:` block appended containing store_app_id, title, tagline, icon URL, category, port_map, author + the `source` provenance sub-block). 6 emit tests updated; dry-run against 330 upstream apps unchanged (241/44/45/0/0). The legal posture in ADR-0024 still holds — the upstream docker-compose.yml is functional config (factual: image refs, ports, env names), the only expressive content (descriptions, screenshots) was already dropped at the parser stage.
- Umbrel-catalog sync pipeline — Phase 6 (#307) — adds a `sync-catalog --validate-only=<dir>` flag that walks `community-catalog/Apps/*/docker-compose.yml` and asserts shape invariants without running a sync: file parses as YAML, has top-level `services:` + `x-powerlab:`, the `x-powerlab.store_app_id` / `title.en_us` / `source.catalog` fields are non-empty, and `x-powerlab.icon` (if present) parses as a URL. Exit 0 clean / exit 1 with per-rule error lines suitable for CI. 12 unit tests cover happy path, empty catalog, missing dir (no-op), malformed YAML, each required field missing, multiple errors per app, deterministic ordering. Usable in CI to gate weekly sync PRs and locally by maintainers editing description-powerlab.md / icon overrides.


## [v0.6.0] — 2026-05-11
### Internal
- ADR renumber chore — the duplicate ADR-0011 and ADR-0012 numbers (CA series filed 2026-05-07 in one branch, foundation `backend/pkg/` series filed 2026-05-08 in another) are resolved by renumbering the foundation pair to ADR-0025 (`backend-pkg-coexistence-with-casaos-common`) and ADR-0026 (`pkg-logging-built-on-stdlib-slog`). ADR-0011/0012 are now unambiguously the CA-mismatch + CA-rotation ADRs. Cross-references updated across `backend/`, `docs/`, `CHANGELOG.md`, and `.changes/`. Each renumbered ADR carries a "Renumber history" note for traceability. `decisions/README.md` index brought current (0023 + 0025 + 0026 now listed). Closes the action item flagged in `docs/audits/quality-and-tech-debt-2026-05-10.md`.
- Backend integration coverage Phase 1+2 (#150) — wires `go test -coverprofile` per service in CI with 14-day artifact upload, lands HTTP-surface regression locks asserting `/v1/cloud/*`, `/v1/driver/*`, `/v1/recover/*`, `/v1/sys/version/check` and `/v1/sys/update` return 404 on both core and local-storage. Baseline: core 6.1%, app-management 15.5% (local-storage measured by CI Linux runner). Phases 3 (testcontainers) and 4 (fuse build-tag) deferred to Sprint 12+.
- Frontend vitest coverage lifted from 16.77% baseline (Sprint 9) to **28.75% statements / 24.21% branches / 26.41% functions / 29.60% lines** — all four Sprint 11 targets met. 23 new test files: 5 store tests (theme/ui/system/settings/versionHandshake), 5 settings panes (AppsPane/GeneralPane/NetworkPane/SecurityPane/AboutPane), 3 apps modals (ForkAppModal/UninstallAppModal/UpdateAppModal), AppCard, 3 dashboard widgets (MiniProgress/RadialGauge/Sparkline), plus utility regression locks (`compose-name` for #240, `compose-extension` for ADR-0021 priority chain, `format`, expanded `os`). Small extraction: `lib/utils/compose-name.ts` lifts the Docker Compose service-name validation out of `apps/new/+page.svelte` so the contract is unit-tested independently. Test count: 230 → 401 passing. Closes #296.


## [v0.5.13] — 2026-05-11
### Added
- **Headline v0.6 feature**: Dashboard storage card now exposes per-drive **SMART status + temperature** badges (closes #255). Backend already populated `Disk.Temperature` + `Disk.Health` from `smartctl` — UI was throwing the data away. New `Drive Health` section under the existing storage usage rows lists each physical disk with model + bus type + temperature (color-coded: <50°C green, 50–59°C amber, ≥60°C red) + SMART OK/FAIL pill. Smartctl-unavailable hosts (macOS dev, containers without /dev passthrough) render gracefully — badges hide when values are 0/empty. Storage device list polls every 10th utilization tick to keep smartctl call frequency low. 3 locale strings added (en/es/pt-BR).
### Changed
- Settings → App Sources card now labels the third-party AppStore as "Community catalog" instead of "CasaOS catalog". i18n key renamed `settings.casaCatalog` → `settings.communityCatalog` in all 3 locales (en/es/pt-BR). The hardcoded `<p>CasaOS catalog</p>` literal in `+page.svelte` now uses `{t(...)}` properly. The underlying URL (cdn.jsdelivr.net/.../CasaOS-AppStore@gh-pages) is unchanged — content sourcing decision is ADR-0021. Closes #250.
- Internal API surface rebrand (#251): renamed `backend/core/route/v2.CasaOS` struct + `NewCasaOS()` constructor to `Server` / `NewServer()` (the type implements the v2 codegen ServerInterface; "Server" is the conventional name and gets rid of branding in godoc + IDE autocomplete). Renamed message-bus SourceID `SERVICENAME = "casaos"` → `"powerlab"` in `backend/core/common/constants.go` — UI consumers filter by event Name, not SourceID, so the rename is invisible to clients. Removed orphan `RANW_NAME = "IceWhale-RemoteAccess"` constant (zero callers, CasaOS-era remote-access tunnel identifier we never adopted). Closes #251.
### Removed
- Delete two confirmed-dead source files: `backend/core/route/v1/notify_old.go` (62 LOC, zero callers — superseded by `notify.go` long ago) and `backend/app-management/cmd/migration-tool/migration_0412_and_older.go` (77 LOC, orphaned constructor never wired into `main.go`). 139 LOC of dead weight removed; zero behavioural change. First batch of the Sprint 8 kill-list (~17.7k LOC total queued).
- Remove the entire Samba/SMB feature surface — UI never consumed any of the 7 Samba endpoints, and the user explicitly removed Samba from PowerLab's product scope on 2026-05-11. Drops 813 LOC net (9 full-file deletes + 11 surgical edits + go-smb2 dependency). Files-page coupling was 3 cosmetic annotations the UI never read. Closes the Samba kill of the Sprint 8 kill-list (~17.7k LOC total queued).
- Delete `backend/app-management/cmd/appfile2compose/` (95 LOC) — CasaOS-era one-shot tool that converted the legacy `appfile.json` format to docker-compose YAML. PowerLab's App Store has been 100% native compose YAML for the entire fork's history; the binary was never invoked from any script, install path, or Makefile target. Sprint 8 kill-list batch 3/5.
- Quick-win sweep of CasaOS-era orphan files: 40 dead workflow files (.github/workflows/ inside backend/* — GitHub Actions only honors top-level .github), 5 orphan sysroot files (casaos.service unit, rclone.service unit, mergerfs.ctl, env file with stub key, app-management/env), backend/core/Makefile ("call john"), the dead `model.DeviceInfo` type + `systemService.GetDeviceInfo()` method (zero callers), 3 dead UI endpoint constants (ZT_INFO, SYS_PORT, GATEWAY_PORT) + ZTInfo type, plus 4 string cleanups: swagger contact rebrand (zimaboard.com → PowerLab), random.go "Zimaboard backers" comment, and the "Casa" → "PowerLab" device-discovery fallback in route/init.go (the most visible "pretending to be CasaOS on the LAN" residue). Sprint 8 kill-list batch 4/5; 1709 LOC removed, 48 files changed.
- Remove network-feature surface that does not belong in PowerLab core: ZeroTier (entire `/v1/zt/*` proxy + `/v2/casaos/zt/*` v2 endpoints + httper helper, ~440 LOC), `WsSsh` + `PostSshLogin` (CasaOS SSH-to-other-host browser terminal — local pty `WsShell` for "open a shell on this server" stays untouched, ~113 LOC), CasaOS Snapdrop-style peer-broadcast `file_websocket.go` (`/v1/file/ws` + `/v1/file/peers`, closes #261, ~315 LOC), and the orphan `pkg/ddns/` constants (zero callers, ~15 LOC). Net: 11 files changed, 915 LOC removed. Aligned with the architectural principle that VPN/DDNS/SMB belong as App Store apps, not core orchestrator features. Sprint 8 kill-list batch 5/9.
- Delete dev-only standalone main packages that no script ever invokes: `backend/app-management/cmd/validator/` (411 LOC, validated CasaOS appfile.json — but PowerLab installs do compose validation inline in `service/compose_service.go`), and `cmd/message-bus-docgen/` in 3 services (~94 LOC, generated markdown docs nobody publishes — Scalar + openapi.yaml cover this). 7 files removed, 505 LOC. Sprint 8 kill-list batch 6/9.
- Remove the `cmd/migration-tool/` Go binary tree across all 6 backend services (1248 LOC, 22 files). The CasaOS-era pattern of "run a separate Go binary before service start to migrate v0.x.y → v0.x.z data paths" was never used in production: `package-linux.sh` does not build it, `install.sh` does not invoke it, and `scripts/migrate-casaos-data.sh` already covers the full filesystem-level CasaOS → PowerLab migration sourced by install.sh. Also drop the now-orphan `MigrationTool` interface in `backend/common/interfaces.go` and `backend/core/interfaces/migrationTool.go`. Sprint 8 kill-list batch 7/9.
- Delete the entire `backend/cli/` subproject (4840 LOC across 61 files). The legacy CasaOS CLI binary was never built (`package-linux.sh` SERVICES list excludes it), never distributed (install.sh has zero refs), and explicitly skipped by CI (workflow comment: "cli is excluded — its codegen sub-packages live in a separate repository (CasaOS-CLI) that we have not forked yet"). All operator paths flow through the SvelteKit panel + Docker orchestration; CLI maintenance was pure overhead. Sprint 8 kill-list batch 8/9 — biggest single delete of the wave.
- Remove the entire app-management `/v1/*` API surface (1365 LOC). UI consumes only `/v2/app_management/*`; the v1 handlers (`AppUsageList`, `ContainerUpdateInfo`, `ToComposeYAML`, `DockerTerminal`, `UninstallApp`, `UpdateSetting`, `ArchiveContainer`, `GetDockerNetworks`) were CasaOS legacy with zero callers. Drops route/v1/ entire dir, route/v1.go, the v1 OpenAPI spec, the v1 Scalar docs HTML, and the gateway routing entries `/v1/apps`, `/v1/container`, `/v1/app-categories`, `route.V1DocPath`. Sprint 8 kill-list batch 9/9 — final batch of the wave.
- Drop 10 dead `/v1/users/*` endpoints in user-service that no UI route + no `backend/common/external` caller ever invoked: `/users/{name, refresh, image, avatar}`, `/users/current/{custom/:key, image/:key}`, `/users/{:id DELETE, :username GET, "" DELETE}`. Single-user PowerLab does not exercise multi-user CRUD, avatars, or custom-conf storage. Keeps the 5 endpoints UI actually uses: `register`, `login`, `status`, `current GET`/`current PUT`, `current/password`. Net: 527 LOC removed (route/v1.go: 27 LOC trimmed; route/v1/user.go: 520 LOC of handlers + now-unused imports). Sprint 9 PR K (split-out from Sprint 8 PR Q scope).
### Fixed
- Sprint 8 PR B — convert 3 remaining panics in
`backend/local-storage/service/disk.go` to logged error +
return false. Audit #216 §C item 2 follow-up; same pattern
as PR #230 (GetDownloadSingleFile fix).

Affected lines:
  - line 135 (was: panic on GetMergeAllFromDB error)
  - line 159 (was: panic in else-branch of CreateMerge errors)
  - line 192 (was: panic on CreateMergeInDB error)

All 3 are inside `EnsureDefaultMergePoint() bool` — both
callers (main.go boot path + route/v2/merge.go enable
endpoint) already handle false gracefully ("mergerfs is
disabled" log + config flip / "default merge point is not
empty" 400 response). The pkg/lifecycle recover middleware
was catching these today and dressing them up as 500s; now
the proper "mergerfs disabled" path runs instead.

Closes audit #216 §C entirely (the 4th panic in disk.go
is inside a commented-out block).

- Sprint 8 PR C — fix #50: CA download "Security Profile" /
"CRT file" / "CA Certificate" links inside the Settings →
Security walkthrough lists were `<a href="/v1/sys/ca-
certificate.X">` anchors that bypassed the JS-driven
`downloadCA()` helper.

When the handler returned an error (CA not yet generated, or
storage path unreadable), the browser navigated to the URL +
rendered the plain-text error in place of the SPA — same
class of "stranded outside the app" UX as the v0.2.7 trust-
dance test bug.

Fix: replaced the 5 inline anchors with `<button>` elements
that call `downloadCA(format)` (which already had the
fetch-based pre-flight + toast.error on failure + no-page-
navigation behavior, in use by the bottom CTAs since #118
prep).

Per memory `feedback_no_text_cert.md`, the cert remains a
binary artifact (.crt / .mobileconfig / .cer) — no copy-to-
clipboard PEM, no .txt rename. Only the trigger surface
changed.

Verified: 10/10 E2E pass locally (3.7s).

- Custom App name field now shows inline validation error (red border + helper text under input) when empty or contains invalid characters. Previously the only feedback was a toast on Deploy + tooltip on the disabled button, leaving users guessing why their input was rejected. Closes
- Files page now exposes a select-all checkbox in the table header so the toolbar Delete button is reachable without Cmd/Ctrl-click chord shortcuts. The header checkbox is tri-state (checked / indeterminate / unchecked) and toggles `store.selectAll` ↔ `store.clearSelection`. Closes #66.
- Editing an existing Custom App and re-deploying no longer fails with "there are ports in use" when the only conflict is the app's own running ports. The orchestrator now routes edit-mode (URL has `?id=X` without `&fork=1`) to the PUT applyComposeAppSettings endpoint, which carries the backend's skip-self port-conflict logic. POST install is unchanged. Closes #65.
- Health endpoint (`/v2/casaos/health/services`) now queries BOTH `casaos*` and `powerlab-*` systemd glob patterns instead of just `casaos*`. PowerLab fresh installs (where units are named `powerlab-*`) previously got an empty health dashboard because the legacy glob never matched. Co-resident installs (operator migrating from CasaOS with `casaos-*` units still present) continue to surface the legacy units too. Results are deduped across globs. Closes #245.
- fstab writes now create `.powerlab.bak` / `.powerlab.new` backup files and a `# Added by PowerLab` marker comment on each appended line, instead of the legacy `.casaos.bak` / `.casaos.new` / `# Added by the CasaOS`. Surprises co-resident installs migrating from CasaOS where those names overlap real CasaOS-written files; harmless on greenfield installs. Existing `.casaos.bak` files on disk are not consumed by code (backup-only artifacts) so no migration step is required. Closes #248.
- Custom App tile click in the Launchpad now opens the app in a new tab even when the user didn't fill the "Web UI Port (Host Port)" field explicitly (#278). The orchestrator now falls back to the first host port from the `ports:` mapping when `web_port` is empty, so a basic Compose like `ports: [8080:80]` produces a clickable tile out-of-the-box — matching native-app tile behavior. Explicit `web_port` still wins. Closes #278.
- Fresh `package-linux.sh` installs now ship the `[security] AllowedOrigins=` section in `/etc/powerlab/message-bus.conf` (Sprint 8 #241 carry-forward). Previously only the embedded sysroot conf.sample carried the section, so operators editing `/etc/powerlab/message-bus.conf` after fresh install found the section missing. Default value is empty (same-origin-only — secure default per ADR-0023); no behaviour change.
### Security
- message-bus SocketIO transports (websocket + polling) now enforce an Origin allowlist instead of unconditionally accepting `return true`. Same-origin requests pass without configuration; cross-origin callers must be listed in the new `[security] AllowedOrigins` section of `message-bus.conf`. Closes #219, ADR-0023.
- Replace 2 hardcoded `"casaos"` literals shipped as PowerLab defaults: (a) `DefaultPassword` substituted into every newly installed Compose app via `$DefaultPassword` placeholder is now `"powerlab"` (closes #243), and (b) Docker registry probe `User-Agent` is now `PowerLab/{AppManagementVersion}` instead of the literal `CasaOS` (closes #244 — branding leak + private-registry log fingerprinting). TDD: 3 regression tests authored failing-first, then implemented.
- JWT access tokens are now issued with `iss="powerlab"` instead of the legacy `"casaos"` (closes #246). The bridging-release accept set in `AcceptedAccessIssuers` lets legacy `iss=casaos` tokens validate too so existing sessions don't get logged out on upgrade — that path drops in v0.7. Also adds a missing access-token issuer gate to `Validate`: refresh tokens (iss=refresh) and tokens from unknown issuers now correctly fail the access path (previously they passed the signature check and were accepted as access tokens, a real bug). Refresh-endpoint code paths use `ParseToken` directly and are unaffected.
### Internal
- Add Playwright regression coverage for the v0.3.0 Files-editor inert-textarea bug (#57). The vitest suite already covered `.cm-editor` mount in jsdom; this adds production-fidelity coverage that opens the editor through the real click flow, types via the actual keyboard pipeline, and asserts the dirty-indicator flips on. The original regression was fixed in earlier polish cycles (v0.3.2 / #116 / #121); this PR locks the fix in place. Closes #57.
- Frontend coverage measurement infrastructure (Sprint 9 PR I). Adds `@vitest/coverage-v8`, configures vitest with the v8 provider + text/html/json-summary reporters, exposes `npm run test:coverage`, and wires CI to upload `ui/coverage/` as a 14-day artifact on every push. Baseline established at **16.77 % statements** (1261/7517) — documented in `docs/audits/frontend-coverage-baseline.md` with targets for Sprint 10 + the v0.6 cut gate. No threshold gates yet; Sprint 10 retro decides the floor.
- Sprint 7 carry-forward kicked off (#123): extract the `apps` section of `settings/+page.svelte` into `lib/components/settings/AppsPane.svelte` as the pattern-proving PR. Net reduction 46 LOC on the god file (1469 → 1423); the new component takes 3 props (`storagePath`, `copiedKey`, `onCopy`) so future panes follow the same shape. 4 remaining panes (general/network/security/about) carry forward to Sprint 10 — each needs user smoke-test in browser per Sprint 7 retro's "user is the verification gate" rule. vitest: 239/239 pass.
- Sprint 10 PR A — extract `GeneralPane.svelte` (~145 LOC) from `settings/+page.svelte` (1423 → 1294 LOC, -129). Component takes 9 props (osHostname, timezone, onTimezoneChange, reachableUrl, currentPort, portInput, onPortInputChange, onRequestPortChange, timezones); locale picker calls `setLocale/getLocale/availableLocales` directly (no parent wiring needed). Port-change flow + reboot/shutdown power UI moved inside the pane. Continues #123 carry-forward — 3 panes left (Network, Security, About).
- Sprint 10 PR B — extract `NetworkPane.svelte` (~85 LOC) from `settings/+page.svelte` (1294 → 1227 LOC, -67). Component takes 5 props (mdnsHostname, reachableUrl, copiedKey, onCopy, networkInterfaces). Continues #123 carry-forward — 2 panes left (Security, About).
- Sprint 10 PR C — extract `SecurityPane.svelte` (~250 LOC) from `settings/+page.svelte` (1227 → 1011 LOC, -216). The biggest pane: HTTPS onboarding walkthrough (4 OS tabs — iOS/macOS/Android/Windows), CA download buttons, HTTP-fallback for blocked downloads, verification button, reset-trust + rotate-CA recovery actions, account section. 9 props (state + 5 callbacks). Continues #123 carry-forward — 1 pane left (About).
- Sprint 10 PR D — extract `AboutPane.svelte` (~280 LOC) from `settings/+page.svelte`, finishing the 5-pane settings split (#123). The pane is mostly static markup (hero, highlights grid, "built with" chips, resources, footer) plus the updater store check/install UI. Reads directly from `$lib/stores/updater.svelte` — no parent wiring needed; zero props. **Settings page final: 1469 → 739 LOC (-730 / 50% reduction).** Closes #123. Apps/+page split (1561 LOC) carries to Sprint 11.
- Sprint 10 PR E — extract 3 modal components from `apps/+page.svelte` (1561 → 1492 LOC, -69): `ForkAppModal`, `UninstallAppModal`, `UpdateAppModal`. Each takes minimal props (open + callbacks). Pattern-proving PR for #123 carry on apps page. Larger modals (Install confirm with port-conflict UI, Detail modal, Install fullscreen + minimized banner) stay in the orchestrator — they have heavy state interaction and need user smoke gate. Continues #123.
- Sprint 10 PR G — implement-or-delete the 2 Go `t.Skip("MUST FIX!")` tests that violated memory `feedback_no_apagar_test_para_passar`. (1) Rewrite `backend/core/service/file_test.go::TestNewInteruptReader` as 6 proper unit tests for `NewReader`/`NewWriter` context-cancellation (was a 10-second sleep loop reading from upstream CasaOS dev's hardcoded `/Users/liangjianli/Downloads/` path with no assertions). (2) Delete `backend/core/pkg/utils/network_detection.go` + its test entirely — zero production callers, dead code from CasaOS era; drops the `github.com/Curtis-Milo/nat-type-identifier-go` dependency. 13 LOC removed from go.mod/sum.


## [v0.5.12] — 2026-05-10
### Fixed
- Sprint 5.5 quality wave — 3 quick-win fixes from the audit
at `docs/audits/quality-and-tech-debt-2026-05-10.md` (PR #216):

1. **CONTRIBUTING.md license bug** — declared "PolyForm
   Noncommercial License 1.0.0" but the actual `LICENSE` file
   is AGPL-3.0 (matched by README, SECURITY.md, mkdocs site).
   Single-line legal contract bug; corrected to AGPL-3.0.

2. **README stale facts** — install command pinned `v0.1.5`
   (current is v0.5.11); architecture diagram showed gateway
   on `:80 / :443` (real ports `:8765 / :8443`).

3. **ADR index outdated** — `docs/decisions/README.md` listed
   only ADRs 0001-0012; missing 11 newer ADRs (including the
   governance-critical ADR-0019, ADR-0020 JWT keypair,
   ADR-0021 coexistence, ADR-0022 CasaOS-abandoned). Added all
   missing entries + a note explaining the duplicate 0011/0012
   numbering (two files at each ID because they landed in
   parallel branches).

- Sprint 6 #1 — kill the flaky `TestSearch` in
`backend/core/service/other_test.go` that randomly broke CI
with `'NoneType' object has no attribute 'replace'` ... wait,
that was the mkdocs one. The Go test broke with goleak detecting
a still-running DNS lookup goroutine on `www.baidu.com` after
the test's wg.Wait() returned (resty's underlying transport
goroutine outlived the test).

Root cause: `Search()` was DEAD CODE — the only call site in
`route/v1/other.go:20` had been commented out. The function
fanned out HTTP calls to 5 search engines for typeahead
suggestions, but the route handler used `AgentSearch()`
(the URL-proxy variant) instead.

Fix: deleted dead code. Removed:
  - `OtherService.Search()` method + entire body
  - `model.SearchEngine` type (used only by Search)
  - `TestSearch` test (no longer applicable)
  - Commented-out call site in `route/v1/other.go`

Bonus: kept `AgentSearch` with proper godoc + an SSRF-shape
warning comment (since it'll fetch any URL, behind JWT auth).

Net diff: -160 LOC, eliminates a recurring CI flake (PR #190
hit it; PR #203 hit it; PR #214 hit it; PR #215 hit it).

### Internal
- Sprint 5 obliterate-CasaOS wave 2 — eliminates the runtime
`icon.casaos.io` dependency + sweeps the dead-CasaOS-build-
artifact tree across all 5 backend services. Per ADR-0022,
no runtime deps on upstream CasaOS infrastructure remain.

## Kill #9 (icon CDN)

`service/container.go::ContainerList` no longer synthesises
icon URLs by calling `https://icon.casaos.io/main/all/<image>.png`
for system-origin containers. Falls through to the container's
own icon label or the UI's MyAppList fallback.

## Dead-build sweep (~80 files)

Pattern: same as PR #208's gateway cleanup. None of these were
referenced by `scripts/package-linux.sh` (PowerLab's actual
install pipeline). All inherited CasaOS-era artifacts shipped
with the source tarball as cruft:

  backend/<svc>/build/scripts/setup/service.d/**/setup-*.sh
  backend/<svc>/build/sysroot/usr/share/casaos/**

Across: app-management, message-bus, core, local-storage,
user-service.

## .goreleaser configs (~14 files)

`.goreleaser.yaml` + `.goreleaser.debug.yaml` per service.
PowerLab uses `scripts/package-linux.sh` for releases, not
goreleaser. The configs reference
`github.com/IceWhaleTech/CasaOS-AppManagement/cmd/appfile2compose`
which was the upstream release pipeline — dead.

## Cosmetic

`ui/src/routes/settings/+page.svelte` About card link
"Powered by CasaOS" → "PowerLab on GitHub" pointing at our
own repo.

## Net diff

~80 files deleted, 2 modified. Build verified for all
services that build on macOS (local-storage's pre-existing
fuse/netlink issue unaffected).

- Sprint 5.5 quality wave (#196) — gateway godoc raise from
21% → 85%, second module surfaced on docs site after pkg/*.

9 exported decls gained godoc:
  - common.LoadConfig
  - service.NewMDNSService
  - service.Management + NewManagementService
  - service.State + NewState
  - route.GatewayRoute + NewGatewayRoute
  - route.ManagementRoute + NewManagementRoute
  - route.StaticRoute + NewStaticRoute
  - route.NewSecurityRoute
  - route.NewDocsRoute

Implementation:
  - scripts/gen-godoc.sh restructured to support N modules under
    docs/api/<mod>/. MODULES list now includes "gateway" alongside
    "pkg".
  - .gitignore generalised: docs/api/*/*.md ignored, only
    index.md per module is committed (was hard-coded to pkg's
    6 files).
  - mkdocs.yml nav grows a "gateway" sub-section under "Go API
    reference".
  - docs/api/gateway/index.md committed as the curated landing.

Per-service raise plan (#196): gateway done ✅, message-bus +
user-service next (smallest remaining at 17% / 40%).

- Sprint 5.5 quality wave (#196) — user-service godoc raise from
40% → 57%. **Below the 70% bar to surface on the docs site**;
needs a follow-up PR for the 12 route-handler godocs to clear
the gap.

10 high-leverage decls gained godoc:
  - service: Repository, NewService, UserService, EventService,
    NewEventService
  - model: EventModel, CommonModel, APPModel, Result
  - model/system_model: VerifyInformation
  - route: InitRouter, InitV2Router, InitV2DocRouter
  - route/v2: UserService, NewUserService

Skipped on purpose:
  - 12 V1 route handlers in `route/v1/user.go` already have
    `/** @description: */` Swagger-style blocks (legacy CasaOS
    annotation, not parsed by current build). Converting them
    to godoc is mechanical but adds noise to this PR; deferred.
  - 3 cmd/migration-tool/ funcs — out of scope, audit #170
    reviews delete-or-promote.

Next per #196 raise plan:
  - **user-service follow-up** — convert 12 V1 handlers' Swagger
    blocks to godoc → unblocks docs-site surface (will hit ~75%)
  - **message-bus** — 41 funcs at 17%, needs ~27 godocs (~3h)
  - then common, local-storage, app-management, core

- Sprint 5.5 quality wave kick-off — single comprehensive audit doc
at `docs/audits/quality-and-tech-debt-2026-05-10.md` covering five
dimensions (live docs site health, README/repo-root doc freshness,
per-module Go godoc coverage vs the 70 % bar, ADR + audit
inter-link integrity, TODO/FIXME + smell sweep).

Read-only audit — no code or product doc was modified. Findings
are file:line-cited and ranked by leverage in a 20-PR Sprint 5.5
punch-list at the end of the doc. Audited against `origin/main`
at commit d551123 (post-PR #213 gateway godoc raise).

## Top-line findings

- **Docs site (live)**: 38 URLs probed, all 200; 6 architecture
  pages render Mermaid + load `mermaid.min.js` (vendored setup
  works). 9 published pages are reachable by URL but absent from
  `mkdocs.yml` nav (concepts, migrating-from-casaos,
  backup-restore, three new audits, the trust-onboarding pattern,
  STORE-COVERAGE).

- **CRITICAL — `CONTRIBUTING.md:148`** declares the wrong license
  ("PolyForm Noncommercial 1.0.0" — repo is AGPL-3.0 per
  `LICENSE` file, README, mkdocs site, SECURITY.md). Single-line
  PR.

- **Stale repo-root docs**: `CONTRIBUTING.md` ports `8089` (real:
  `8765/8443`), Go 1.21 (real: 1.25), `start.sh` (real: `dev.sh`);
  `README.md` pins `v0.1.5` example (current: v0.5.11) and shows
  architecture box with ports `:80/:443`; `SUPPORT.md:17` says
  "Out of scope for v0.1.x".

- **Per-module godoc coverage** (vs 70 % bar, analyzer numbers):
  `pkg` 61.7 % (under-counted; real ~95 %+), `gateway` 42.5 %
  (PR #213 reports 85 %), then `common` 28.9 %, `user-service`
  25.0 %, `local-storage` 20.8 %, `core` 19.9 %, `app-management`
  11.6 %, `message-bus` 2.6 %, `cli` 2.6 %. Punch-list of the top
  5 highest-impact undocumented exports per module is in the doc.
  `cli` (39 exports, mostly mechanical) is the cheapest next win
  after the in-flight `user-service` raise.

- **ADR + audit health**: two pairs of duplicate-numbered ADRs
  (0011 — both `-ca-mismatch-detection-and-recovery` and
  `-backend-pkg-coexistence-with-casaos-common`; 0012 — both
  `-ca-rotation-flow` and `-pkg-logging-built-on-stdlib-slog`).
  ADR index in `decisions/README.md` stops at #0012; the other
  10 ADRs are missing from the table. Six load-bearing framework
  choices have no ADR (Uber fx, Echo, GORM, Svelte 5 runes lock,
  oapi-codegen, mDNS strategy).

- **TODO/FIXME**: 25 backend (within ADR-0019's 27 ceiling), 0 UI,
  0 scripts. Two security-tier smells: WebSocket `CheckOrigin`
  bypassed in `message-bus/service/socketio_service.go:53/:58`;
  four live-request-path `panic()` calls
  (`local-storage/service/disk.go:90/114/147`,
  `core/route/v1/file.go:243`).

- **Smells**: 4 files > 1000 lines (compose_app.go, file.go,
  apps/+page.svelte, settings/+page.svelte); 25 functions > 100
  lines, 8 over 130 lines (worst:
  `core/route/v1.go:17 InitV1Router` at 223 lines).

## Sprint 5.5 punch-list (head)

Top 10 PRs ordered by leverage (full list of 20 in the doc):

1. CONTRIBUTING license fix (CRITICAL)
2. CONTRIBUTING port + tooling sweep
3. README diagram + version pin
4. SUPPORT.md v0.1.x removal
5. Resolve duplicate ADR numbers (0011, 0012)
6. Refresh `decisions/README.md` index (cover ADR-0013–0022)
7. mkdocs.yml nav refresh — add the 9 orphaned pages
8. WebSocket origin check in message-bus (security)
9. Convert 4 live-path panics to error returns (reliability)
10. Implement-or-delete `TextEditor.test.ts:229` skipped test

Items 11–17 are per-module godoc raise PRs (one per module,
smallest-first: cli -> common -> local-storage -> core ->
app-management -> message-bus, with `user-service` already in
flight). Items 18–20 are slow-burn ADRs + file splits.

- Sprint 5 progress dashboard at
`docs/audits/sprint-5-progress-dashboard.md`. Living doc that
summarises the day's PRs, what's now true (acceptance criteria),
per-service godoc coverage scorecard, what's NOT in current
release, what to test, and recommended next moves.

Companion to the residue audit (#203 / kill list) and the
Sprint 4 retrospective.

- Sprint 6 #2 — TODO/FIXME burn-down. Audit (#216) flagged 25
backend TODO/FIXME items (within ADR-0019's 27 ceiling but
worth chipping at). This PR resolves 5:

## Cleaned

- `backend/app-management/service/container.go:45` — misleading
  `// TODO - make use of NewVersionApp map` comment. The map
  IS used (line 293 in same file). Replaced TODO with proper
  godoc explaining what the map holds + how it's populated.

- `backend/core/route/v1/zerotier.go:68` — bare `// TODO` line
  after the response was written. Meaningless dangling TODO.
  Deleted.

- `backend/app-management/route/v2/appstore.go:444` — bare
  `// TODO` above an error-log + continue. Replaced with
  explanatory comment about the chosen behavior (skip + log
  loud rather than break the list).

## Documented + tracked as issue

- `backend/message-bus/service/socketio_service.go:53,58` —
  WebSocket + polling `CheckOrigin` always returns true (CORS
  bypass). **Real security finding** flagged by the audit
  (P1, mitigated by JWT auth at gateway). TODO comments
  replaced with SECURITY block + link to **issue #219** which
  has the full fix plan (origin allowlist, ~4 unit tests).

## Burn-down score

Backend TODO/FIXME: 24 → 19 (-21%). Within audit's
recommended 20-50% per sprint. Remaining 19 are mostly
design-wish TODOs (refactor recommendations) + FUSE FIXMEs
in local-storage/pkg/mount/dir.go that need careful work.

## Verification

  cd backend/{core,message-bus,app-management} && go build ./...
  # all clean

- Sprint 6 #3 — user-service godoc raise complete (57% → 75%).
Third module surfaced on the docs site after pkg/* + gateway.

12 V1 route handlers in `route/v1/user.go` got proper godoc
(`// HandlerName ...` + Route line). The legacy
`/** @description: */` Swagger blocks (CasaOS-era) are
replaced — they were never parsed by the build pipeline anyway.

Handlers documented:
  GetUserInfoByUsername, GetUserAllUsername,
  GetUserCustomConf + PostUserCustomConf + DeleteUserCustomConf,
  DeleteUser, PutUserImage + PostUserUploadImage +
  GetUserImage + DeleteUserImage,
  PostUserRefreshToken, DeleteUserAll

Infrastructure:
  - scripts/gen-godoc.sh MODULES grew "user-service"
  - mkdocs.yml nav adds "user-service" sub-section under
    "Go API reference"
  - docs/api/user-service/index.md committed as curated landing
  - .gitignore generalised: docs/api/*/*/ now ignored too
    (gomarkdoc generates subpackage subdirectories like
    route/v1.md, model/system_model.md)

## Per-service raise progress

  ✅ pkg          100%
  ✅ gateway       85% (PR #213)
  ✅ user-service  75% (this PR — over the 70% bar)
  ⏳ message-bus   17% (next, smallest absolute count)
  ⏳ common        49%
  ⏳ local-storage 27%
  ⏳ app-management 35%
  ⏳ core          39%

- Sprint 6 #4 — message-bus godoc raise complete (~5% → ~75%).
Fourth module surfaced on the docs site after pkg/* +
gateway + user-service. The message-bus had the lowest
coverage in the repo (3 of 66 exported decls documented at
the audit baseline) and was the last big godoc raise queued
for Sprint 6.

## What got documented

- `model/structs.go` — package doc + EventType, Event,
  ActionType, Action, PropertyType, GenericType, the type-
  list constants. Removed the dead `// type Property` block
  that had been commented out for years.
- `model/settings.go` + `model/sys_common.go` — Settings,
  CommonModel, APPModel.
- `repository/repository.go` — Repository interface (with a
  sketch of the two-DB layout).
- `repository/repository_db.go` — DatabaseRepository struct +
  every method (GetYSKCardList, UpsertYSKCard, DeleteYSKCard,
  Get/Register EventType variants, Get/Register ActionType
  variants, Get/UpsertSettings, Close) and the four generic
  helpers (GetTypes, RegisterType, GetTypesBySourceID,
  GetType). Constructors NewDatabaseRepository +
  NewDatabaseRepositoryInMemory documented.
- `service/services.go` — Services container, Start, NewServices.
- `service/event_service_websocket.go` — EventServiceWS struct
  + Publish, Subscribe, Unsubscribe, Start, NewEventServiceWS.
- `service/action_service_websocket.go` — ActionServiceWS struct
  + Trigger, Subscribe, Unsubscribe, Start, NewActionServiceWS.
- `service/event_type_service.go` + `service/action_type_service.go`
  — every method + constructor.
- `service/socketio_service.go` — SocketIOService struct +
  Publish, Start, Server, NewSocketIOService.
- `service/ysk.go` — YSKService struct, NewYSKService,
  YskCardList, UpsertYSKCard, DeleteYSKCard, Start
  (covers seed-on-first-boot + event-driven upsert path).
- `route/api_route.go` — package doc, APIRoute, NewAPIRoute.
- `route/routers.go` — NewAPIRouter + NewDocRouter.
- `route/api_route_event.go` — Get/Register EventType handlers,
  PublishEvent, SubscribeEventWS — each with a Route line.
- `route/api_route_action.go` — symmetric set for actions.
- `route/api_route_socketio.go` — SubscribeSIO + PollSIO + their
  trailing-slash duplicates.
- `route/ysk.go` — DeleteYskCard + GetYskCard.
- `route/adapter/in/*` + `route/adapter/out/*` — package docs +
  every adapter function (Event, Action, EventType, ActionType,
  PropertyType, both directions).
- `config/init.go` — InitSetup.

## Infrastructure

  - scripts/gen-godoc.sh MODULES grew "message-bus" → fourth
    surfaced module
  - mkdocs.yml nav adds a "message-bus" sub-section under
    "Go API reference" with every generated package
  - docs/api/message-bus/index.md committed as the curated
    landing page (publisher/subscriber call-chain pointers,
    coverage summary, codegen carve-out note)

## Per-service raise progress

  ✅ pkg          100%
  ✅ gateway       85% (PR #213)
  ✅ user-service  75% (PR #221)
  ✅ message-bus  ~75% (this PR — over the 70% bar)
  ⏳ common        49%
  ⏳ local-storage 27%
  ⏳ app-management 35%
  ⏳ core          39%

Codegen package intentionally untouched — it is regenerated
from the OpenAPI spec by oapi-codegen and any doc edits
would be overwritten on the next `go generate`.

- Sprint 6 #5 — common module godoc raise (~49% → ~75%).
Fifth module surfaced on the docs site after pkg/* +
gateway + user-service + message-bus.

`backend/common` is the most-imported module in the repo:
every other backend service imports its SDK (`external/*`),
JWT verifier (`utils/jwt`), shared response shapes (`model/*`),
cert manager (`pkg/security`), and CORS middleware. Lifting
its godoc coverage has higher leverage than any of the
service-specific raises because the doc shows up wherever
the SDK is imported.

## What got documented

- `interfaces.go` — MigrationTool interface (each method now
  explains what install/setup expects)
- `middleware/echo.go` — Cors (with rationale: CORS is
  permissive because the gateway's JWT middleware is the
  real auth gate)
- `model/*.go` — Result, DeviceInfo, Route, ChangePortRequest,
  ComposeAppWithStoreInfo, ComposeAppStoreInfo
- `external/gateway.go` — package doc + ManagementService
  interface (each method) + NewManagementService
- `external/message_bus.go` — EventType, PropertyType,
  PrintEventTypesAsMarkdown, GetMessageBusAddress,
  PublishEventInSocket
- `external/user_service.go` — ParsedToken, GetPublicKey
  (with ADR-0020 reference for keypair persistence + 10s
  cache rationale), ParseToken (two-tier LRU cache, fall-back
  to gateway socket when the .url file isn't there)
- `external/app_manage.go` — AppManageService interface +
  NewAppManageService
- `external/notify.go` — NotifyService interface +
  NewNotifyService
- `external/share.go` — ShareService interface +
  NewShareService
- `external/gpu.go` — GPUInfo, GPUInfoListWithSMI,
  GPUInfoList, NvidiaGPUInfo aliases, GPUUtilization,
  GetGPUUtilization (with macOS Apple Silicon vs Linux
  nvidia-smi fallback explained)
- `utils/jwt/jwt.go` — package doc, Claims, GenerateToken,
  ParseToken, GetAccessToken, GetRefreshToken, Validate
- `utils/jwt/jwt_helper.go` — JWK, JWKS, JWKSPath, JWT
  (echo middleware), GenerateKeyPair, GenerateJwksJSON,
  PublicKeyFromJwksJSON, JWKSHandler
- `pkg/security/cert.go` — CertManager struct + GetCAPaths,
  GetServerPaths, GetHSTSPath, CheckAndRotate, StartTicker,
  GenerateRootCA, GenerateServerCert, ArmHSTS, IsHSTSArmed
  (the rest of the file was already documented to a high
  bar — only the gaps got filled)

## Infrastructure

  - scripts/gen-godoc.sh MODULES grew "common" → fifth
    surfaced module
  - mkdocs.yml nav adds a "common" sub-section under
    "Go API reference" with the high-traffic packages
    (external, model, middleware, utils/jwt, pkg/security,
    utils) — the long tail of utils/* helpers stays
    generated-but-unlinked
  - docs/api/common/index.md committed as the curated
    landing page (auth flow + HTTPS flow call-chain
    pointers, ADR cross-refs)

## Per-service raise progress

  ✅ pkg          100%
  ✅ gateway       85%
  ✅ user-service  75%
  ✅ message-bus  ~75%
  ✅ common       ~75% (this PR — over the 70% bar)
  ⏳ local-storage 27%
  ⏳ app-management 35%
  ⏳ core          39%

Long-tail `utils/*` packages intentionally left at lower
coverage — most are 1-2 line wrappers whose names already
document them. codegen package untouched (regenerated from
OpenAPI on every `go generate`).

- Sprint 6 #6 — local-storage module godoc raise (~27% → ~75%).
Sixth module surfaced on the docs site after pkg/* + gateway +
user-service + message-bus + common. Closes the lowest-coverage
service-specific module (local-storage was at the bottom of the
per-service scorecard going into the sprint).

## What got documented

- `common/constants.go` — package doc + Version, ServiceName,
  DefaultMountPoint
- `model/sys_common.go` — package doc + CommonModel, APPModel,
  ServerModel
- `model/setting.go` — Setting Group + Flag iota constants
  (with stability notes), SettingItem, IsDeprecated
- `model/storage.go` — StorageA, Sort, Proxy + every method
  (GetStorage, SetStorage, SetStatus, Webdav302, WebdavProxy,
  WebdavNative)
- `model/stream.go` — FileStream + every method
  (GetMimetype, NeedStore, GetReadCloser, SetReadCloser, GetOld)
- `service/service.go` — package-level Cache + MyService
  singletons, Services interface (each method), NewService
  (with panic-on-gateway rationale), store struct
- `service/notify.go` — NotifyServer interface + NewNotifyService
- `service/usb.go` — USBService interface (each method) +
  NewUSBService
- `service/disk.go` — DiskService interface (full per-method
  docs covering EnsureDefaultMergePoint, AddPartition,
  DeletePartition, CheckSerialDiskMount, FormatDisk, GetDiskInfo,
  GetPersistentTypeByUUID, GetUSBDriveStatusList, LSBLK,
  MountDisk, RemoveLSBLKCache, SmartCTL, UmountPointAndRemoveDir,
  UmountUSB, UpdateMountPointInDB, DeleteMountPointFromDB,
  GetSerialAllFromDB, SaveMountPointToDB, InitCheck, GetSystemDf)
  + NewDiskService + IsDiskSupported + IsFormatSupported +
  WalkDisk + ParseBlockDevices
- `pkg/mount/mount.go` — package doc + Mount, UmountByMountPoint,
  UmountByDevice (with --force/--recursive rationale)
- `pkg/fstab/fstab.go` — package doc + Pass iota constants +
  Entry/FStab + String + Add (with replace semantics) +
  RemoveByMountPoint (with comment-mode rationale) + GetEntries +
  GetEntryByMountPoint + GetEntryBySource + Get
- `pkg/partition/partition.go` — package doc + Partition struct +
  GetDevicePath + GetPartitions + ProbePartition + AddPartition +
  CreatePartitionTable
- `pkg/mergerfs/mergerfs.go` — package doc + ControlFile +
  ListValues + SetSource + GetSource + AddSource + RemoveSource +
  AddPath + RemovePath
- `pkg/sign/sign.go` — package doc + Sign interface
- `pkg/sign/hmac.go` — HMACSign + Sign + Verify + NewHMACSign
- `pkg/cache/cache.go` — package doc + Init (with TTL rationale)
- `pkg/sqlite/db.go` — package doc + ContextKey + GetDBByFile +
  GetGlobalDB

## Infrastructure

  - scripts/gen-godoc.sh MODULES grew "local-storage" → sixth
    surfaced module
  - mkdocs.yml nav adds a "local-storage" sub-section under
    "Go API reference" with all 14 high-traffic packages
  - docs/api/local-storage/index.md committed as the curated
    landing (disk-management call-chain pointers, merge-pool
    flow pointer, coverage note about FUSE handlers + codegen)

## Per-service raise progress

  ✅ pkg            100%
  ✅ gateway         85%
  ✅ user-service    75%
  ✅ message-bus    ~75%
  ✅ common         ~75%
  ✅ local-storage  ~75% (this PR — over the 70% bar)
  ⏳ app-management 35%
  ⏳ core           39%

FUSE handler methods on pkg/mount/{file,dir,rmount}.go already
had brief comments and follow the bazil.org/fuse interface
contract — left as-is to avoid noise. codegen package
untouched (regenerated from OpenAPI on every `go generate`).

- Sprint 6 #7 — app-management module godoc raise (~35% → ~75%).
Seventh module surfaced on the docs site after pkg/* + gateway +
user-service + message-bus + common + local-storage. Penultimate
service-specific raise of Sprint 6 — only core remains, and that
one is large enough to need splitting across multiple PRs.

## What got documented

- `common/context_properties.go` — WithProperties +
  PropertiesFromContext (event correlation bag)
- `model/sys_common.go` — package doc + CommonModel, APPModel,
  ServerModel, GlobalModel (with secret-injection note for
  GlobalModel)
- `model/docker.go` — DockerStatsModel + DockerDaemonConfigurationModel
- `model/category.go` — ServerCategoryList + Category
- `model/manifest.go` — TCPPorts, UDPPorts, PortMap (with
  "CommendPort typo is wire-format" note), PortArray, Env,
  EnvArray, PathMap, PathArray, CustomizationPostData
- `model/app.go` — ServerAppListCollection, StateEnum
  constants, ServerAppList, MyAppList, Ports, Volume, Envs,
  Devices, Strings, MapStrings (gorm-stored slice types)
- `service/app.go` — App type + StoreInfo (x-extension extract)
- `service/compose_service.go` — ComposeService struct,
  PrepareWorkingDirectory, IsInstalling, Install (with
  macOS-vs-Linux StoragePath rationale), Uninstall, Status,
  List, NewComposeService, ApiService
- `service/compose_app.go` — ComposeApp type (with full overview
  of owned methods), StoreInfo, DiskUsage, AuthorType,
  SetStoreAppID, SetTitle, Update, App, Apps, MainService,
  MainTag, Containers, Pull (streams to logWriter), Up,
  UpWithCheckRequire, Create, PullAndInstall, PullAndApply,
  Uninstall, Apply
- `service/appstore.go` — AppStore interface (each method),
  AppStoreByURL, NewDefaultAppStore, LoadCategoryMap,
  LoadRecommend, BuildCatalog, StoreRoot
- `service/appstore_management.go` — AppStoreManagement struct,
  AppStoreList, OnAppStoreRegister, OnAppStoreUnregister,
  ChangeGlobal, DeleteGlobal, RegisterAppStore (async) +
  RegisterAppStoreSync, UnregisterAppStore, AppStoreMap,
  CategoryMap, Recommend, Catalog, UpdateCatalog, ComposeApp,
  WorkDir, IsUpdateAvailable, IsUpdateAvailableWith,
  IsUpdating, StartUpgrade, FinishUpgrade, NewAppStoreManagement

Pre-existing rich docs on common/labels.go (IsPowerLabApp,
LabelValue, BuildLabels) and common/appdata.go (PowerLabAppDataPath,
LegacyAppDataPath) left as-is — already cover the ADR-0021
rationale at the level the audit asks for.

## Infrastructure

  - scripts/gen-godoc.sh MODULES grew "app-management" → seventh
    surfaced module
  - mkdocs.yml nav adds "app-management" sub-section under
    "Go API reference" with the high-traffic packages
  - docs/api/app-management/index.md committed as the curated
    landing (install-flow + app-store call-chain pointers,
    ADR-0021 cross-ref for label namespace)

## Per-service raise progress

  ✅ pkg             100%
  ✅ gateway          85%
  ✅ user-service     75%
  ✅ message-bus     ~75%
  ✅ common          ~75%
  ✅ local-storage   ~75%
  ✅ app-management  ~75% (this PR — over the 70% bar)
  ⏳ core             39% (last remaining; needs split into
                          multiple PRs — 355 exports vs ~160
                          for the others)

codegen package untouched (regenerated from OpenAPI on every
`go generate`). The bundled CasaOS appstore data tree under
`backend/data/appstore/...` is upstream-managed assets, not source.

- Sprint 6 #8 — core module godoc raise (~39% → ~75%).
Eighth and FINAL module surfaced on the docs site. Closes the
per-service godoc raise initiative (#196) — every backend
service is now over the 70% bar and has a curated landing
page on the public docs site.

Largest module in the repo (355 exports, 126 files). Strategy
this PR: focus on the high-leverage public surface, skip per-
method docs on internal helpers + auto-generated codegen + the
big SystemService gopsutil-wrapper interface (35 self-
describing methods). Result is a docs site that surfaces every
package + every Service interface contract — the "what does
this do?" surface, not the "how does this work?" body.

## What got documented

- `common/constants.go` — package doc + service identity
  constants (SERVICENAME, VERSION, BODY, RANW_NAME, with
  notes on why SERVICENAME stayed "casaos")
- `model/user.go` — package doc + UserInfo, UserDBModel,
  SystemUser
- `model/req.go` — PageReq + numeric bounds + Validate
- `model/share.go` — Shares
- `model/setting.go` — Group + Flag iota constants,
  SettingItem, IsDeprecated
- `model/storage.go` — StorageA, Sort, Proxy + every method
  (Webdav302/WebdavProxy/WebdavNative)
- `model/stream.go` — FileStream + every method
- `model/zima.go` — Path, DeviceInfo
- `model/sys_common.go` — SysInfoModel, ServerModel, APPModel,
  CommonModel, Result, RedisModel, SystemConfig, FileSetting,
  BaseInfo (with single-letter-JSON-keys-are-wire-format note)
- `model/object.go` — ObjWrapName, Object, Thumbnail, Url,
  ObjThumb, ObjectURL, ObjThumbURL
- `interfaces/migrationTool.go` — package doc + MigrationTool
  interface (each method)
- `service/service.go` — Cache + MyService + WebSocket
  bookkeeping vars, Repository interface (each method covered),
  NewService (with dev-vs-prod gateway tolerance rationale),
  store struct
- `service/connections.go` — ConnectionsService (with the
  "MountSmaba typo is wire-format" note)
- `service/health.go` — HealthService
- `service/peer.go` — PeerService
- `service/rely.go` — RelyService
- `service/shares.go` — SharesService (with smb.conf rewriter
  note on UpdateConfigFile + InitSambaConfig)
- `service/notify.go` — NotifyServer (with SystemTempMap
  purpose note), SendMeg legacy WS broadcaster, NewNotifyService
- `service/system.go` — SystemService interface (with
  intentional "no per-method docs" rationale), GetDeviceAllIP,
  GetCPUThermalZone, NewSystemService
- `service/socket.go` — Name (peer-display descriptor) +
  GetPeerId/GetIP/GetName/GetNameByDB
- `pkg/cache/cache.go` — package doc + Init
- `pkg/sign/sign.go` — package doc + Sign + error sentinels
- `pkg/sign/hmac.go` — HMACSign + Sign + Verify + NewHMACSign
- `pkg/config/init.go` — InitSetup
- `internal/conf/config.go` — package doc + Database, Scheme,
  LogConfig, Config
- `internal/op/hook.go` — ObjsUpdateHook +
  Register/HandleObjsUpdateHook, SettingItemHook + Register/
  HandleSettingItemHook, StorageHook + Register/CallStorageHooks
- `internal/op/driver.go` — New constructor type, RegisterDriver,
  GetDriverNew, GetDriverNames, GetDriverInfoMap
- `internal/sign/sign.go` — package doc + Sign, WithDuration,
  NotExpired, Verify, Instance (with frank note on the load-
  bearing "token" placeholder secret)

Existing docs on `pkg/sqlite/db.go.GetDb`,
`service/other.go` (whole file), and various fully-documented
helpers were left untouched.

## Infrastructure

  - scripts/gen-godoc.sh MODULES grew "core" → eighth and
    final surfaced module
  - mkdocs.yml nav adds "core" sub-section under
    "Go API reference" with every documented package
  - docs/api/core/index.md committed as the curated landing
    (system-stats + SMB-shares + signed-URL + driver-runtime
    call-chain pointers, intentional-skip rationale)

## Per-service raise progress — INITIATIVE COMPLETE

  ✅ pkg            100%
  ✅ gateway         85%
  ✅ user-service    75%
  ✅ message-bus    ~75%
  ✅ common         ~75%
  ✅ local-storage  ~75%
  ✅ app-management ~75%
  ✅ core           ~75% (this PR — over the 70% bar)

Every backend module now has docs over the threshold + a
curated landing page. Closes #196.

## Intentional non-coverage

- `service/system.go` SystemService interface: 35 methods,
  each a wrapper around gopsutil/os/exec. Names self-document;
  per-method docs would add no signal.
- `internal/driver`: vendored alist/openlist storage-driver
  framework, kept at upstream's documentation level.
- `service/file.go`, `service/file_upload.go`, `service/
  powerlab_updater.go`: per-line internal helpers; the package
  + service interface docs cover the contract surface.
- `codegen` package: regenerated from OpenAPI on every
  `go generate`.

- Sprint 6 retrospective doc — closure artifact for the Quality
Consolidation YOLO sprint.

Headline: 8-module godoc raise initiative (#196) closed —
every backend service is at ≥70% coverage with a curated
landing page on the public docs site (pkg, gateway,
user-service, message-bus, common, local-storage,
app-management, core). 6 godoc PRs (#221-#226) + the
obliterate wave from audit #203 + v0.5.10 + v0.5.11 + the
TODO/FIXME burndown all landed in this sprint window.

Retro covers: what went well (raise pattern generalized,
scoping survived the biggest module, YOLO discipline held,
bug-fix=regression-test held on the flaky kill, Sprint 7 prep
landed in Sprint 6), what went wrong (stray ui/ trailing-space
dir, stale wakeup callbacks, gofmt fights, the SystemService
meta-question), what surprised us (audit coverage numbers
inaccurate, common had highest leverage, UI files biggest in
the repo), what we shipped (releases, PR table, doc deltas),
process changes ratified (worktree-per-PR, intentional-non-
coverage sections, plan-only doc PRs as Sprint-N+1 prep), open
backlog (long-file refactor #216 → #227, integration coverage
#150, real-upgrade test #169, E2E #108), and what to do
differently in Sprint 7.

Per ADR-0019 retrospectives live in docs/audits/.

- Sprint 7 #1 — split compose_app.go (1276 LOC) into 5 files
per refactor proposal #227. Mechanical lift-and-shift; no
behaviour changes.

New layout:
  - compose_app.go (288 LOC) — type declaration, factories
    (LoadComposeAppFromConfigFile, NewComposeAppFromYAML),
    and private package-level helpers (isSystemPath,
    removeRuntime, gpuCache, getNameFrom,
    injectEnvVariableToComposeApp)
  - compose_app_metadata.go (220 LOC) — x-extension read/write
    (StoreInfo, getExtension, getExtensionMap, AuthorType,
    SetStoreAppID, SetTitle, SetUncontrolled,
    UpdateEventPropertiesFromStoreInfo)
  - compose_app_lifecycle.go (486 LOC) — mutation surface
    (Update, PullAndApply, PullAndInstall, Apply, Uninstall,
    SetStatus)
  - compose_app_runtime.go (318 LOC) — docker engine surface
    (Up, UpWithCheckRequire, Create, Pull, Containers, Logs,
    HealthCheck, Stats)
  - compose_app_query.go (149 LOC) — read-only helpers
    (App, Apps, MainService, MainTag, DiskUsage,
    GetPortsInUse)

All under 500 LOC. Existing tests in compose_app_test.go,
compose_app_disk_test.go, extension_test.go, autoremap_test.go
cover the public surface unchanged.

First of 7 PRs in the Sprint 7 refactor track. Closes the
compose_app.go arm of #216 §D.

- Sprint 7 #2 — split backend/core/route/v1/file.go (1166 LOC)
into 6 files per refactor proposal #227. Mechanical lift-and-
shift split EXCEPT one behaviour fix: the audit-flagged
panic at line 243 in GetDownloadSingleFile is converted to
an error return (audit #216 §C item 2).

## New layout

  - file.go (87 LOC) — shared types (ListReq, ObjResp,
    FsListResp) + the package-level WebSocket upgrader +
    conn/err state + a package doc enumerating where each
    handler lives
  - file_browse.go (244 LOC) — read paths: GetFilerContent,
    GetLocalFile, DirPath, GetSize, GetFileCount, GetFileImage
  - file_mutate.go (279 LOC) — write paths: RenamePath,
    MkdirAll, DeleteFile, PostOperateFileOrDir,
    DeleteOperateFileOrDir, PutFileContent, PostFileContent
  - file_router_upload.go (243 LOC) — multipart + chunked
    upload: GetFileUpload, PostFileUpload, PostFileOctet
  - file_download.go (174 LOC) — download paths:
    GetDownloadFile, GetDownloadSingleFile (with the panic
    fix)
  - file_websocket.go (304 LOC) — legacy peer-broadcast
    subsystem: CenterHandler, Client, PeerModel,
    ConnectWebSocket, init (cron + monitoring goroutine),
    writePump, readPump, monitoring, GetPeers

## Behaviour fix (audit #216 §C item 2)

Original GetDownloadSingleFile contained `panic(err)` at the
os.Open call site. Audit flagged this as one of four live-path
panics that should be error returns. The pkg/lifecycle recover
middleware (per ADR-0014) caught the panic but the user saw a
500 instead of the expected 404-shaped "file does not exist"
response that the rest of the handler returned (line 267).
Now both the early os.Open and the late os.Open paths return
the same error envelope.

## Test plan

  - go build ./route/v1/... clean (codegen-not-on-disk warning
    is pre-existing local-only)
  - gofmt -l clean on the new files
  - existing route/v1 test suite covers the public surface
    unchanged

## Position in the refactor track

PR 2 of 7 from #227. Next: 3 UI splits (apps/+page.svelte,
settings/+page.svelte) + 3 Go god-function extractions
(CreateContainer, RecreateContainer, SendFileOperateNotify,
PostAddStorage). Closes the file.go arm of #216 §D.

- Sprint 7 #5 — split god functions in container.go per audit
#216 §E.

Original CreateContainer + RecreateContainer were 192 + 191 LOC
each. Both drop to ~95 LOC after extraction. No behaviour changes.

## Extracted helpers (new container_helpers.go, 171 LOC)

- **wrapContainerEvents** — replaces the IIFE-with-events pattern
  that repeated 6+ times in RecreateContainer. Wraps fn with
  begin/end PublishEventWrapper calls; on fn error fires errType
  with the error message merged into props. Preserves the
  original `go PublishEventWrapper` async semantics.
- **buildPortBindings** — translates `[]model.PortMap` →
  `(nat.PortSet, nat.PortMap, error)`. Protocol "both" expands
  to ["tcp","udp"]. Host bindings skipped in network mode "host".
- **buildEnvVars** — renders `[]model.Env` → docker env-var slice
  + show-env label list. Handles $-prefix system substitution +
  "port_map" sentinel.
- **buildContainerResources** — translates CPU/memory/devices
  from the form into `container.Resources`. Memory shifted left
  20 (MiB → bytes).
- **buildVolumeMounts** — walks `[]model.PathMap` → docker bind
  mounts + legacy host-config bind strings. Auto-creates missing
  host dirs (`mkdir -p` semantics); per-volume errors logged +
  skipped (matches pre-extract behaviour).

## Function shrinks

- CreateContainer: 192 → ~95 LOC. Now reads as orchestration —
  build helpers + inspect existing + assemble HostConfig +
  ContainerCreate. The 5-line port-protocol switch and the
  25-line volume-mount loop are gone from the body.
- RecreateContainer: 191 → ~95 LOC. Each phase (clone, stop
  old, start new, rollback, remove old, rename) is now a 5-7
  line wrapContainerEvents call. The original 6 IIFE blocks
  each had identical begin/end/error PublishEventWrapper
  boilerplate; now the boilerplate lives once, in the helper.

## Test plan

  - go build ./service/... clean (codegen-not-on-disk warning is
    pre-existing local-only)
  - gofmt -l clean on the touched files
  - container.go drops from 890 → 719 LOC (171 LOC moved into
    the helpers file)
  - existing container_test.go covers the public surface

## Position in the refactor track

PR 5 of 7 from #227. Closes the container.go arm of #216 §E.

- Sprint 7 #6 — extract publishFileOperateSnapshot helper from
SendFileOperateNotify per audit #216 §E.

Original was 157 LOC where the nowSend=true and nowSend=false
branches duplicated an 80-LOC build-snapshot-and-publish body.
Now SendFileOperateNotify is 12 LOC (just the once-vs-loop
dispatch) and calls publishFileOperateSnapshot which holds the
shared body.

Plus a third helper publishFileOperateMessage extracted from
the marshal+publish-and-log-on-error tail that appeared 3 times.

Result:
  - SendFileOperateNotify: 157 → 12 LOC
  - publishFileOperateSnapshot: ~80 LOC (the shared body, once)
  - publishFileOperateMessage: ~14 LOC (the publish tail)
  - notify.go overall: 391 → 331 LOC

No behaviour changes — the publish + queue-mutation + ExecOpFile
fan-out are identical to the pre-extract behaviour. Comments on
the helpers call out the side-effects on FileQueue + OpStrArr
so a reader doesn't have to walk the whole call chain to know.

- Sprint 7 #7 — split PostAddStorage god function per audit
#216 §E.

Original was 146 LOC mixing parse/validate/format/mount.
Split into 3 orthogonal helpers; orchestrator now reads
top-down without scrolling.

Helpers (kept in route/v1/storage.go):
  - parseAndValidateAddStorageRequest(ctx) → (path, name,
    format, errResp). Bind body + early-out checks.
  - optionallyFormatStorage(ctx, currentDisk, path) error.
    The destructive umount + delete-partition + add-partition
    flow.
  - mountStorageChildren(ctx, children, name) string. The
    per-child mount + DB-save + ADDED-notification loop.
    Returns newline-joined failed-path string (matches
    pre-extract partial-success behaviour).

PostAddStorage drops from 146 → ~36 LOC. No behaviour
changes. Also deleted the 40-LOC dead-comment block in the
no-children special case (was commented out before — pure
whitespace cleanup).

Closes the storage.go arm of #216 §E. Last of the 4 god-
function extractions in audit §E.

- Sprint 7 #8 — E2E coverage expansion per #108.

Replaces the single-page baseline (smoke.spec.ts only) with
per-area smoke coverage. Will protect the upcoming UI splits
in #227 PR 3 (apps/+page.svelte 1561 LOC) and PR 4
(settings/+page.svelte 1469 LOC) — those are behaviour-
sensitive splits that needed an E2E safety net before they
were safe to attempt.

## New tests

  - tests/auth.spec.ts — replaces stale auth.spec.broken.ts.txt
    Setup wizard appears on first-run + LoginScreen appears
    when initialized + no session.
  - tests/apps.spec.ts — /apps page header + back-to-launchpad
    link.
  - tests/settings.spec.ts — sidebar + pane navigation + logout
    button. Verifies > 1 nav button (catches a bad-extract
    that drops a pane).
  - tests/files.spec.ts — file browser shell renders even with
    empty /v1/file/dirpath.
  - tests/orchestrator.spec.ts — replaces stale
    orchestrator.spec.broken.ts.txt. /apps/new compose
    orchestrator loads.
  - tests/smoke.spec.ts — slimmed down to launchpad-renders
    only. Per-area coverage moved out.
  - tests/helpers/mock-backend.ts — shared installBaselineMocks
    helper. Per-test mocks register before the catch-all so
    specific routes (e.g. /v1/file/dirpath returning an empty
    list shape) work.

## Stale specs deleted

  - ui/tests/auth.spec.broken.ts.txt (pre-launchpad UI)
  - ui/tests/orchestrator.spec.broken.ts.txt (pre-/apps/new
    rework)

## Selector strategy

Tests use accessible role/text selectors (page.locator('h1'),
hasText filters, href-based link matching) instead of brittle
CSS classes or data-testid attributes. Two reasons:

  1. Minimizes UI changes — the upcoming UI splits don't
     have to thread testid attributes through every component.
  2. Keeps tests readable — selectors describe what the user
     sees, not implementation details.

data-testid will be added later if/when the role-based selectors
become unstable.

## What's NOT covered yet

  - Auth-form happy path (login → main interface). Needs
    richer mocks for /v1/users/login and the JWT refresh.
  - File operations (delete, rename, move). The bug-#2
    TextEditor regression is covered separately by vitest.
  - App install pipeline. Needs heavy mocks for
    /v2/app_management/compose; deferred to a later PR if
    audit asks.

Closes the E2E baseline arm of #108. Real install/file/op
flows will land per kill-PR as those features change.

- Sprint 7 retrospective doc — closure artifact for the
Refactor track + E2E expansion sprint.

Headline: 6 PRs delivered (#229 #230 #231 #232 #233 #234) —
every Go-side god file/function from audit #216 §D + §E split,
the audit-flagged GetDownloadSingleFile panic converted to
proper error return, and E2E went from 1 baseline smoke to
10 tests across 6 specs.

Open Sprint 7 backlog: UI splits #3+#4 from #227 (await user
OK due to behaviour-sensitivity; now have E2E safety net via
#234), backend integration coverage #150 (needs Docker),
real-upgrade test finish #169.

Trust-dance UX redo (#118) explicitly removed from Sprint 7
scope per user mid-sprint.

Process changes ratified: run E2E locally before push (memory
feedback_e2e_run_local_first), never weaken tests just to
make them pass (memory feedback_no_apagar_test_para_passar),
plan-only doc PR during sprint-end CI waits (#227 was
drafted during Sprint 6 close), explicit Playwright route
precedence for overlapping mocks (CI-vs-local divergence on
the version-handshake mock burned 1 round of CI).

Per ADR-0019 retrospectives live in docs/audits/.



## [v0.5.11] — 2026-05-10
### Security
- Removed the inherited CasaOS self-update path that did
`curl -fsSL https://get.casaos.io/update?t=… | bash` from
upstream CasaOS infrastructure. Real curl-pipe-bash from
third-party DNS was a supply-chain risk, not just a branding
concern. The path was already dead in the UI (the legacy
`/v1/sys/update` endpoint had zero frontend consumers); this
PR removes the backend code so an attacker can't reach the
curl-pipe-bash via direct API call either.

PowerLab's own in-app updater under `/v1/powerlab-update/*`
(manifest.json + signed-tarball pipeline) is unaffected and
remains the only update path.

Removed:
  - `GET /v1/sys/version/check` route + handler
  - `POST /v1/sys/update` route + handler
  - `service.MyService.Casa()` accessor + `casaService` struct
  - `systemService.UpdateSystemVersion()` method
  - `version.IsNeedUpdate()` + the `model.Version` type that fed it
  - `httper.OasisGet()` + the `ServerApi`/`UpdateUrl` config fields
    it depended on
  - Stale `backend/core/conf/casaos.conf` + `conf.conf.sample`
    (still pointed at `api.casaos.io/casaos-api`)
  - `SYS_VERSION` constant in `ui/src/lib/api/endpoints.ts`
    (already had zero references)

Audit reference: `docs/audits/casaos-residue-2026-05-10.md`
kill #1 (highest priority).

### Internal
- Sprint 4 retrospective at `docs/audits/sprint-4-retrospective.md`.

Per ADR-0019, sprint retrospectives live as audits. This one
captures #85 sub-PR delivery (foundation + wire + compose
rewrite), the parallel #179 DB paths split-brain work, the docs
Phase 3 brought forward, and the v0.5.8 lock-out regression I
shipped + had to hot-fix. 7 lessons named, 6 recommendations
for Sprint 5.

- CasaOS residue audit at `docs/audits/casaos-residue-2026-05-10.md`.

Companion to `casaos-dependencies.md` — fresh "what is left, in
what order to kill" snapshot for Sprint 5. Confirms PR #151
finished the module-path rename (zero CasaOS refs in any go.mod
or go.sum). Catalogues 10 functional, ~30 cosmetic, and ~13
intentional-sentinel CasaOS strings still in the tree, plus 3
runtime URL dependencies and 1 unused stale config sample.
Recommends 10 separate PRs for Sprint 5 (~17h total) ordered by
leverage/risk ratio, with `get.casaos.io/update` curl-pipe-bash
fallback called out as the highest-priority kill (security
surface, not just rebrand).

- Self-review of the Sprint 4 closure day at
`docs/audits/work-review-2026-05-10.md`.

Companion to the Sprint 4 retrospective. Where retro covers
process lessons, this doc rates the CODE that landed: 3 things
to keep as-is, 5 things to fix soon (small tech debt I created
today), 3 things to fix later (not urgent). Plus 3 risks to
watch and a recommended Sprint 5 order weighted by leverage.

Net assessment: 17 PRs / 3 releases / ~5,500 LOC churned in
one day, with 80 new regression tests as the safety net. High
velocity sustained without obvious quality regression — the
one prod incident (v0.5.8 lock-out) was caught + permanently
fixed in <30 min via the discipline that was working.

- Two technical-debt items from the Sprint 4 self-review (#200)
cleared:

1. **Mermaid.js vendored** at `docs/js/mermaid.min.js` (3.3MB)
   instead of loaded from the unpkg CDN. Docs site now works
   fully offline (CI builds, self-hosted mirrors), no SRI
   concerns, version pinned by file content.

2. **Generated godoc files moved out of git** —
   `docs/api/pkg/{errors,foundation,lifecycle,logging,migrations,tracing}.md`
   are now `.gitignored` and produced by `scripts/gen-godoc.sh`
   during the docs CI build. Eliminates the diff churn on every
   refactor of internal types. Only `docs/api/pkg/index.md`
   stays committed (it's the curated landing).

Docs CI workflow grew a Go setup step + a gen-godoc step before
mkdocs build. Trigger paths extended to also fire on
`backend/pkg/**/*.go` changes (so godoc updates land on the
site when the source changes).

- Two paired changes for the Sprint 5 obliterate-CasaOS work:

1. **ADR-0022** — formalises the policy that PowerLab takes no
   new dependencies on upstream CasaOS infrastructure. Cites
   the upstream's verified abandonment status: latest release
   v0.4.15 (Dec 2024, 1.5 years stale), 795 open issues, no
   coherent release cadence. Becomes the citable rule that
   justifies Sprint 5's kill list and rejects future PRs that
   would reintroduce CasaOS coupling.

2. **Kill #2 from audit #203** — deletes inherited
   `backend/core/{CHANGELOG,CODE_OF_CONDUCT,SECURITY}.md` (all
   CasaOS-flavored, pointing at wiki@casaos.io). Replaces
   missing root `CODE_OF_CONDUCT.md` + `SECURITY.md` with
   PowerLab versions that route reports correctly + explicitly
   redirect anyone confused about the project lineage.

- Sprint 5 kill #4 (audit #203) — gateway sysroot tree rebrand:

- `backend/gateway/build/sysroot/etc/casaos/gateway.ini.sample`
  → `backend/gateway/build/sysroot/etc/powerlab/gateway.ini.sample`
- sample's `runtimepath=/var/run/casaos` → `/var/run/powerlab`
- `//go:embed` directive in `backend/gateway/main.go` updated

Plus dead-CasaOS-artifact cleanup (the audit's adjacent items —
none of these were referenced by `scripts/package-linux.sh`,
PowerLab's actual install pipeline; pure inheritance debt):

- `casaos-gateway.service` + `.buildroot` (PowerLab installs
  `powerlab-gateway.service` per Sprint 3)
- `build/scripts/setup/service.d/gateway/{arch,debian,ubuntu}/setup-gateway.sh`
- `build/sysroot/usr/share/casaos/cleanup/**`

13 files deleted, 1 renamed, 2 modified. Net diff -260 LOC.

- Sprint 5 audit #203 kills #5 (cosmetic) + #8 (dead systemd
units) bundled. All low-risk, no wire format changes.

## Deleted (dead inheritance, none referenced by install pipeline)

- `casaos-message-bus.service`
- `casaos-app-management.service` + `.buildroot`
- `casaos-local-storage.service` + `casaos-local-storage-first.service`
- `casaos-user-service.service`
- `backend/core/model/heart.go` (CasaOSHeart type, zero usages)

install.sh installs `powerlab-*.service` directly per Sprint 3
work; these were CasaOS-era artifacts that just shipped in the
source tarball as cruft. Same pattern as PR #208's gateway
cleanup.

## Cosmetic rebrands

- `backend/core/main.go:271` log message
  "CasaOS main service is listening" → "PowerLab core service is listening"
- `backend/cli/cmd/appManagementShowLocal.go:191` error hint
  "is the casaos-app-management service running?" → "powerlab-..."
- `backend/cli/cmd/appManagementListApps.go:75` same fix

## Process

Added `backend/*/local_data/log/` to `.gitignore` — a stale
log file at `backend/core/local_data/log/casaos/log.log` had
leaked into a previous commit. The path is dev-only runtime
output; this prevents future accidents.

## Deferred

`SERVICENAME = "casaos"` in `backend/core/common/constants.go`
— this is the message-bus topic prefix for events emitted by
core (see `notify.go` callers). Changing it is wire-format
breakage: every subscriber filtering on this would need to
update simultaneously. Per ADR-0021's lesson learned, this
needs a dual-write window. Tracked separately.

- Sprint 5 audit #203 kill #10 — `//go:generate` directives no
longer pull from `raw.githubusercontent.com/IceWhaleTech/...`
for codegen. Per ADR-0022 (CasaOS upstream is abandoned), even
build-time pulls from upstream infra are policy violations.

All 8 cross-service codegen directives now reference the
LOCAL openapi.yaml files (already present in each service's
`api/<svc>/` dir) via relative paths:

  backend/app-management/main.go  → ../message-bus/api/...
  backend/core/main.go            → ../message-bus/api/...
  backend/local-storage/main.go   → ../message-bus/api/...
  backend/user-service/main.go    → ../message-bus/api/...
  backend/cli/main.go             → ../{app-mgmt,core,...}/api/...
                                    (5 directives, one per service)

Confirmed via `go generate ./...` in each module — codegen
produces identical output (the local copies already matched
upstream).

Side-effect: `go generate ./...` is now offline-capable (CI
builds, isolated dev environments) and faster (no GitHub
rate-limit risk).

Also deleted: `backend/core/build/sysroot/usr/share/casaos/shell/update.sh`
— a `curl … | bash` from `IceWhaleTech/get/main/update.sh`.
Dead artifact, never referenced after PR #206 killed the
upstream-update path.



## [v0.5.10] — 2026-05-10
### Added
- Go API reference for `backend/pkg/*` foundation packages now lives
on the docs site at `/go-api-reference/`. Auto-generated from godoc
via `gomarkdoc` (`scripts/gen-godoc.sh`); regenerated on every
release so it never drifts from the code.

Currently scoped to `pkg/*` (100% godoc coverage, Sprint 2 Phase 6).
Per-service Go packages will be surfaced once each module hits ≥70%
coverage — tracked at #196 with a per-module audit + coverage
scorecard.

Also: REST API reference page added to nav, with embedded host link
to the live Scalar portal (per ADR-0008).

### Fixed
- Documentation site Mermaid diagrams now actually render. PR #190
added the superfences custom_fence config but Material 9.5's
documented auto-load of mermaid.js didn't actually fire on the
live site — diagrams stayed as raw `<pre class="mermaid">` blocks
with no JS to convert them.

Fix: load mermaid.js explicitly via `extra_javascript` plus a
`docs/js/mermaid-init.js` initialiser that re-runs on Material's
`navigation.instant` page swaps (so diagrams render on every
architecture page, not just the first one the user landed on).

Also auto-themes between Material's light/slate schemes so
diagrams look right in both modes.

### Internal
- CasaOS provenance sweep: deleted inherited service-level READMEs and
stripped IceWhale/CasaOS file-header banners from inherited Go files.

Two pieces:

1. **Service READMEs deleted.** All 7 `backend/<svc>/README.md` files
   plus `backend/cli/README.md` were verbatim CasaOS upstream READMEs
   (IceWhaleTech badges, codecov tokens, "Auto publish via
   `git push origin dev**`" instructions, etc.). They contained no
   PowerLab content. Removed entirely; the docs site
   (`docs/architecture/`, `docs/decisions/`, mkdocs portal) is the
   single source of truth.

   `backend/core/README.md` was the worst case — a full copy of the
   upstream IceWhaleTech/CasaOS marketing README living one
   directory below our own PowerLab top-level README.

   `backend/common/pkg/mod_management/README.md` was kept — it's a
   short PowerLab-relevant client snippet with no CasaOS branding.

2. **File-header banners stripped from 39 Go files.** Removed the
   `/* @Author: LinkLeong link@icewhale.com … @FilePath: /CasaOS/…
   @Website: https://www.casaos.io … Copyright (c) 2022 by icewhale,
   All Rights Reserved. */` koroFileHeader banners that sat above
   `package` declarations in inherited files. These banners had
   stale `@FilePath` paths pointing to the upstream `/CasaOS/`
   module layout, false copyright assertions, and zero documentary
   value — they were noise cargo-culted from the original VS Code
   extension config.

   Modules touched: app-management (3 files), core (33 files),
   local-storage (1), common (1), user-service (2), gateway (0).
   `go build ./...` is green for every touched module on darwin/arm64
   dev host (and local-storage on `GOOS=linux` per usual cross-target
   constraint — pre-existing macOS netlink/xattr stub limitation).

3. **Repo debris.** Deleted untracked `ui/ ` (trailing-space)
   73MB gzip blob — accidental Mac Finder cp from months ago,
   tracked by issue #174. Closes #174 implicitly.

No functional changes. Inline per-function `koroFileHeader` blocks
(`/** @description: @param {*} src … */` above functions in
network_detection.go, file.go, image.go, user.go) were left alone in
this pass — they will be cleaned up alongside the godoc Phase 2
per-package doc work, not as a pure-noise sweep.



## [v0.5.9] — 2026-05-10
### Fixed
- v0.5.9 hot-fix — closes the v0.5.8 lock-out regression.

v0.5.8 shipped the split-brain DB detector (refuse-to-start when a
service finds its DB at multiple paths) but did NOT auto-clean the
v0.5.4 hot-fix sobras. Result: hosts that had the v0.5.4 mishap state
upgraded to v0.5.8, the strict check fired, user-service refused to
boot, login broke. Symptom looked identical to the v0.5.7 JWT-keypair
bug but was a completely different cause introduced by my own fix.

Three pieces:

1. **Backend: `paths.AutoMoveLegacyAside`** — for known-stale legacy
   paths (user.db, local-storage.db: paths the service NEVER reads
   from), automatically move the duplicate to `<path>.bak.<unix-ts>`
   before the strict split-brain check runs. Non-destructive: the
   file is preserved as a backup, not deleted. user-service and
   local-storage main.go now call this BEFORE `AssertNoSplitBrain`.

   For genuinely-ambiguous duplicates (core's casaOS.db at multiple
   paths where either could be authoritative), `AssertNoSplitBrain`
   remains as the safety net — operator picks.

2. **UI: success toast + auto-reload** — the updater store now shows
   a visible success toast when an upgrade completes successfully
   and auto-reloads the page after 2.5s so the user doesn't have to
   refresh manually. Pre-v0.5.9 the upgrade silently completed and
   the user was left staring at "Upgrading…" until they refreshed
   by hand. Failures show an error toast (no auto-reload, the
   previous version is still running and reloading would be
   confusing).

3. **Integration test** at `scripts/test-upgrade-resolves-stale-legacy_test.sh`
   — builds the user-service binary and runs it against a sandbox
   simulating the v0.5.4 mishap state (both canonical and legacy
   paths exist). Asserts: legacy moved aside, canonical untouched,
   stderr mentions the move, .bak content preserved. This is the
   test that SHOULD have run before v0.5.8 shipped — closes
   #169 in spirit (Phase 1.5 release-checklist as automated test).

7 new Go test cases at backend/common/utils/paths/db_test.go cover
the AutoMoveLegacyAside contract: moves stale duplicate, no-op when
canonical missing/empty, no-op when legacy missing, multiple legacy
paths, idempotent re-run, and integration with AssertNoSplitBrain.

4 new UI test cases at ui/src/lib/stores/updater.test.ts lock the
success toast + reload timing + failure toast + diagnostic
surfacing.



## [v0.5.8] — 2026-05-10
### Added
- Documentation site at https://neochaotic.github.io/powerlab/ —
mkdocs-material foundation. The Sprint 4.5/pre-v1.0 docs Phase 3
is brought forward into the active flow rather than waiting for a
pre-tag bundle (per the v1.0-deferred decision).

Initial nav covers:

  - Home (landing + project status)
  - Getting started: Install, First boot, Updating
  - Architecture: 6 existing pages reused (topology, request
    lifecycle, foundation interfaces, dependency graph, data
    persistence, CasaOS strangler)
  - Coexistence with CasaOS: new overview translating ADR-0021
    for end users
  - Operations: HTTPS setup, update manifest, release checklist,
    troubleshooting (existing top-level docs)
  - Audits: db-paths, casaos-dependencies, sprint retros, UI
    feature map, endpoint usage, dead code
  - Decisions (ADRs): index of all 21 records

Build runs in CI on every PR (--strict mode catches broken links
+ missing pages). Deploys to GitHub Pages on push to main.

Going forward, every PR that adds a feature or changes behavior
should consider whether a docs page change is needed alongside.

### Fixed
- Sprint 4 / #179 — installer no longer leaves stale duplicate DB
files. The v0.5.4 hot-fix migration copied `user.db` and
`local-storage.db` to `/var/lib/powerlab/db/<file>.db`, but those
services actually read from `/var/lib/powerlab/<file>.db` (no
`/db/` subdir). Result was 30+ minutes of debug looking at the
wrong file during a real prod incident. The migration now writes
to the canonical paths, and a boot-time split-brain check in
user-service refuses to start if both copies still exist with
data — printing recovery instructions instead of silently picking
one and risking persistent data drift.

New centralised path helpers at `backend/common/utils/paths/db.go`
expose the canonical destination for every service's persistent
files; future migrations consume them so a path convention change
happens in one place. `docs/audits/db-paths.md` is the new
single-source-of-truth audit (per ADR-0019).

Five layers of defense land in this PR:

- L1 helpers centralised
- L2 migration writes to correct destinations + has split-brain regression test
- L3 boot-time refuse-to-start check
- L4 18-assertion regression suite (8 Go + 10 bash)
- L5 install.sh audit step that warns operator before the service crashes

Existing installs are not auto-cleaned — the boot-time check
surfaces split-brain with operator-actionable instructions
rather than risking destructive automatic actions on data the
operator might still need.

- Sprint 4 / #85 PR-C — closes the CasaOS coexistence story for
newly installed apps. PowerLab and CasaOS can now run on the
same host without label collisions or AppData races for any new
app installed after this PR. install.sh's hard-block relaxes to
notice. #85 DoD met for new installs; existing-app migration
deferred to a follow-up tool.

Two pieces:

1. **Compose volume-source rewrite at install time**
   (`service/compose_service.go::rewriteAppDataPathsToCanonical`).
   Runs after the existing `remapVolumePaths` pass. Substitutes
   `<storagePath>/AppData/` → `<storagePath>/PowerLabAppData/` in
   every bind-mount source. Newly installed apps bind into the
   per-product canonical tree from day one.

2. **install.sh CasaOS coexistence block relaxed**
   (`scripts/package-linux.sh`). Was: hard refuse-to-install
   unless `--allow-coexist`. Now: friendly notice describing
   the now-clean coexistence (with explicit caveat that apps
   already installed remain at `/DATA/AppData/`). The
   `--allow-coexist` flag is preserved as a silently-accepted
   no-op for any operator runbooks that pass it.

4 new regression tests at `service/autoremap_test.go` lock the
rewrite contract: rewrites legacy AppData prefix, honors custom
storagePath (macOS dev installs), no-op when storagePath empty,
ignores non-bind volumes.

ADR-0021 amended in this PR ("Subsequent decision: existing-app
migration deferred") explaining why the original on-boot
mv-based migration was removed: it would have invalidated
bind-mount sources in on-disk compose YAMLs, producing apparent
data loss on next container start. Doing it correctly requires
an atomic dir-move + YAML-rewrite which is sizeable enough to
deserve its own PR + test suite.

This completes the #85 sub-PR sequence:
  - PR-A (#181): ADR-0021 + label/path helpers + 16 tests
  - PR-B (#182): wire all label call sites; dual-write active
  - PR-C (this one): compose volume rewrite + coexistence relax

### Internal
- Sprint 4 / #85 foundation — adds ADR-0021 + the
`backend/app-management/common/labels.go` package that will be the
single source of truth for every Docker container label PowerLab
reads or writes. No service code changes yet; the next sub-PR
rewrites the call sites in service/container.go and friends to
consume `common.IsPowerLabApp` / `common.LabelValue` /
`common.BuildLabels`.

ADR-0021 records two coexistence decisions:

- Container labels move from unnamespaced flat keys (`casaos`,
  `origin`, `icon`, ...) to canonical `io.powerlab.v1.*`
  reverse-DNS names. One release window of dual-write keeps
  existing PowerLab containers visible.
- AppData tree moves from `<StoragePath>/AppData/` (collides with
  CasaOS) to `<StoragePath>/PowerLabAppData/` (per-product).

16 regression test cases in `common/labels_test.go` +
`common/appdata_test.go` lock the dual-read / dual-write contract,
the AppData rename, and the legacy-key completeness invariant.

This is the first of ~3 sub-PRs for #85. PR-B wires service code
to the new helpers; PR-C does the AppData on-boot migration.

- Sprint 4 / #85 PR-B — wires every container-label call site in
app-management to consume the helpers landed in PR-A. Containers
PowerLab creates from this point forward carry both the canonical
`io.powerlab.v1.*` labels AND the legacy unnamespaced labels
(per ADR-0021's one-release-window dual-write); the "is mine"
filter accepts either, so containers PowerLab created before this
PR stay visible in the panel without forcing a recreate.

Sites rewritten:

- `service/container.go` — list filter, value reads, write block
  (12 inline label writes consolidated to one `BuildLabels` call)
- `service/v1/app.go` — label reads in legacy V1 Custom App
  inspect path
- `route/v1/docker.go` — origin check during delete

Orphan constant `ContainerLabelV1AppStoreID` removed from
`common/constants.go` (its single string is now the
`LegacyLabelAppStoreIDKey` helper constant; reads go through
`LabelValue(LabelAppStoreIDKey)` which dual-reads both).

No new tests — the dual-read / dual-write contract is exhaustively
covered by the 12 cases in `common/labels_test.go` from PR-A.
Direct map reads are now an anti-pattern that bypasses the
contract; reviewer can grep for `Labels["origin"]` etc. to verify
none remain.

Next sub-PR (#85 PR-C): on-boot AppData migration + install.sh
coexistence-block relax (per #85 DoD).



## [v0.5.7] — 2026-05-09
### Added
- Sprint 3 retrospective at `docs/audits/sprint-3-retrospective.md`.

Per ADR-0019 (tech-debt tracking pattern), retrospectives live as
audits. This one captures the v0.5.4 prod incident + the long-tail
bugs and process gaps surfaced during the rebrand wave, with each
follow-up tracked as a labeled GitHub issue (#169–#174).

Includes:
  - What went well (4 items)
  - What went wrong (7 items, each with the lesson + remediation
    already in flight)
  - 6 follow-up issues opened (#169 phase 1.5 test, #170 migration-
    tool audit, #171 flag.Parse template, #172 branch protection,
    #173 goleak convention, #174 ui/ cleanup)
  - Sprint 4 fit recommendation per issue (which to add to Sprint
    4 vs defer)
  - Outcome scoreboard (4 releases, 12+ structural PRs, ~3500 LOC
    removed, 80+ regression test assertions, 3 prod incidents
    fixed + tested, 4 process improvements)

### Changed
- **Sessions now survive PowerLab upgrades.** The JWT signing
keypair is persisted to `user.db` and reused across service
restarts, instead of being regenerated fresh on every startup.

Pre-v0.5.7, every restart of `user-service` (including every
in-app upgrade) silently invalidated every outstanding JWT
cookie — users got logged out on every upgrade. This was
inherited from CasaOS; PowerLab kept the behavior unchanged
and a misleading godoc comment described it as a "deliberate
trade-off." It wasn't — see ADR-0020 for the full story and
the threat-model discussion.

Behavior change in this release:

  - **Default**: keypair persists in `user.db` (new
    `jwt_keypair` table, single-row, PEM-encoded). First boot
    generates one; every subsequent restart reuses it.
  - **Opt-out** for higher-threat environments: set
    `POWERLAB_EPHEMERAL_JWT_KEY=true` to restore the
    pre-v0.5.7 ephemeral behavior.

Threat model trade-off documented in ADR-0020. Summary: the
cost ("every upgrade logs everyone out") is recurring and
certain; the benefit ("stolen disk image can't forge tokens")
is contingent on an attacker who already has bcrypt password
hashes + config secrets + the ability to install backdoors
in the binary. Net positive for the home-server use case.

Regression test at `backend/user-service/service/keypair_store_test.go`
— 5 cases including the THE regression for #176
(`TestLoadOrGenerate_StableAcrossCalls`) which asserts two
consecutive `NewUserService`-equivalent calls return the same
keypair.

Closes #176.

### Internal
- Release v0.5.7 — JWT keypair persistence + Sprint 3 retrospective.

Headline user-visible change:
  - #176 / ADR-0020: sessions now survive PowerLab upgrades.
    JWT signing keypair persists in user.db; opt back into
    pre-v0.5.7 ephemeral behavior via POWERLAB_EPHEMERAL_JWT_KEY=true.
    First time a real PowerLab-owned decision overrode an inherited
    CasaOS one (rather than just rebranding the surface).

Plus:
  - docs/audits/sprint-3-retrospective.md: formal retro on the
    v0.5.4 incident + Sprint 3 outcomes. 6 follow-up issues opened
    (#169–#174) tracking remaining process improvements.

Migration: 0002_jwt_keypair.sql adds a single-row table to user.db.
Idempotent on re-run; CHECK (id = 1) prevents drift.

Behavior on upgrade from v0.5.6: first restart after the upgrade
generates + persists the keypair. Every subsequent restart reuses
it. Net effect: ONE more "logged out on refresh" event during the
v0.5.6 → v0.5.7 upgrade itself; zero from there on.



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
migrated to in earlier sprints (ADR-0025).

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


