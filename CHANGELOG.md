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


