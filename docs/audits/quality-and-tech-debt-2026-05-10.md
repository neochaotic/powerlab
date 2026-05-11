# Quality + tech debt audit — 2026-05-10

**Sprint:** 5.5 (quality wave / pre-Sprint-5 hardening)
**Reviewer:** comprehensive audit, single doc, single PR
**Method:** live docs probe (curl) + source grep + per-module godoc
analyzer + ADR/audit cross-link diff + smell sweep
**Scope:** five dimensions — live docs site, README/repo-root docs,
godoc coverage, ADR/audit inter-link health, tech-debt smells
**Repo state at audit time:** branch off `origin/main` at commit
`d551123` (post-PR #213 gateway godoc raise; **gateway is the first
service module promoted into `gen-godoc.sh:27`** — `MODULES=("pkg"
"gateway")`).

This is a **read-only audit** — no code or product doc was modified
beyond adding this file + its changie fragment. Every finding cites a
file:line. The closing section ranks fixes by leverage so the
maintainer can execute Sprint 5.5 as a punch-list.

Companion to:

- `audits/work-review-2026-05-10.md` — code/design quality of the day's PRs
- `audits/casaos-residue-2026-05-10.md` — leftover CasaOS rebrand work
- `audits/sprint-4-retrospective.md` — process retro

## Top-line scorecard

| Dimension                      | Score / Status        | Notes                                                                                |
|--------------------------------|------------------------|--------------------------------------------------------------------------------------|
| Docs site coverage             | 9 / 10                | 38 pages probed, **0 broken links (HTTP 404)**, all 6 architecture pages render Mermaid + load `mermaid.min.js`. **Nav is stale**: 9 published pages reachable by URL but absent from `mkdocs.yml` sidebar. |
| README freshness               | 7 / 10                | Reflects v0.5.11 product. **2 stale facts**: pinned-version example uses `v0.1.5` (line 198) and the architecture diagram shows ports `:80 / :443` (line 332) instead of the real `:8765 / :8443`. |
| Repo-root docs freshness       | 4 / 10                | **CONTRIBUTING.md:148 declares the wrong license** (PolyForm Noncommercial — repo is AGPL-3.0). Also: Go version mismatch (1.21+ doc'd vs 1.25.7 actual), gateway port `8089` (real: `8765`), Sprint-4 v0.5.x not reflected in SUPPORT.md ("Out of scope for v0.1.x"). |
| Go godoc coverage              | per-module table below | **`pkg` + `gateway` are now published** (post-#213 `gen-godoc.sh:27 MODULES=("pkg" "gateway")`); 6 of 7 remaining service modules sit between **2.6 % and 28.9 %**, far below the 70 % bar. |
| ADR + audit cross-link health  | 5 / 10                | ~~Two pairs of duplicate ADR numbers (0011, 0012)~~ resolved 2026-05-11 (renumbered to 0025 + 0026). **ADR index in `decisions/README.md`** also brought current in the same PR. **Two audits don't cite ADRs** (`dead-code.md`, `ui-feature-map.md`). **Three load-bearing framework choices have no ADR** (Uber fx, Echo, GORM). |
| TODO / FIXME debt              | 25 backend, 0 UI, 0 scripts | Inside ADR-0019's "27 ceiling." Mostly inherited CasaOS code (message-bus, local-storage). Two are real product-shaped tech-debt (HTTP polling instead of WS in `app-management/service/container.go:80`; "TODO: Remove containersWorkaround" in `app-management/route/v2/compose_app.go:629`). |
| `panic()` discipline           | 113 calls / **3 hot-path** | Most are startup paths (`main.go`, `migration-tool/`, `route/v2.go` codegen-spec load) — acceptable. **3 live request-path panics** (`backend/local-storage/service/disk.go:90/114/147`, `backend/core/route/v1/file.go:243`) — should return errors, not crash the service. |
| Tests skipped                  | **1 UI**, 8 Go        | UI: `ui/src/lib/components/files/TextEditor.test.ts:229` `it.skip(...)` — TODO test that should either be implemented or deleted. Go skips are all environmental (Windows path, network, Docker daemon, port collision) — fine. |
| Files > 1000 lines             | 4                     | `backend/app-management/service/compose_app.go` (1,276), `backend/core/route/v1/file.go` (1,166), `ui/src/routes/apps/+page.svelte` (1,561), `ui/src/routes/settings/+page.svelte` (1,469). |
| Funcs > 100 lines              | 25 (top), 8 over 130  | Worst offenders: `core/route/v1.go:17 InitV1Router` (223), `message-bus/main.go:72 main` (196), `app-management/service/container.go:382 CreateContainer` (193). |

---

## 1. Docs site (live)

Probed every page listed in the audit prompt + the entire mkdocs nav.
**Every URL returned HTTP 200.** No broken links. No raw `[broken
link]` markers. No literal `TODO` / `TBD` / `coming soon` / `placeholder`
text in the rendered body of any page (the matches reported by `grep`
were all theme machinery — search input placeholder, `__config` JSON).

### Mermaid render check (all green)

| Page                                                             | `class="mermaid"` count | `mermaid.min.js` loaded |
|------------------------------------------------------------------|--------------------------|-------------------------|
| `architecture/topology/`                                         | 2                        | yes                     |
| `architecture/dependency-graph/`                                 | 3                        | yes                     |
| `architecture/request-lifecycle/`                                | 3                        | yes                     |
| `architecture/casaos-strangler/`                                 | 1                        | yes                     |
| `architecture/data-persistence/`                                 | 1                        | yes                     |
| `architecture/foundation-interfaces/`                            | 1                        | yes                     |

The vendored `js/mermaid.min.js` + `js/mermaid-init.js` setup
documented in `mkdocs.yml:88-100` is working as advertised.

### Source-side placeholder hits (not currently visible on the site)

Only one genuine **TBD** in a published doc:

- `docs/decisions/0011-backend-pkg-coexistence-with-casaos-common.md:95`
  > `backend/common/` after a cut-off date (TBD per kill PR).

Trivial — give it a date or change the language.

### Stale `nav:` in `mkdocs.yml` — 9 published pages absent from the sidebar

These pages exist in `docs/`, are reachable by URL (HTTP 200), and
make sense to a human reader, but **no nav item points at them** so
they're effectively orphaned in the IA:

| File                                                     | Why it should be in nav                                          |
|----------------------------------------------------------|------------------------------------------------------------------|
| `docs/coexistence/migrating-from-casaos.md`              | Companion to `coexistence/README.md`; primary user-journey doc.  |
| `docs/operations/backup-restore.md`                      | Operations runbook on the same level as `api-reference.md`.      |
| `docs/audits/sprint-4-retrospective.md`                  | Listed as "complete" — prior sprint-3 retro IS in nav.           |
| `docs/audits/work-review-2026-05-10.md`                  | Day's work review with senior-engineer hat.                       |
| `docs/audits/casaos-residue-2026-05-10.md`               | Active kill-PR backlog tracker.                                   |
| `docs/concepts/glossary.md`                              | Reference doc — should be under "Concepts" or "Reference" tab.    |
| `docs/concepts/security-model.md`                        | Same.                                                              |
| `docs/patterns/https-trust-onboarding-pattern.md`        | Sister doc to ADR-0009; orphan when not in nav.                   |
| `docs/STORE-COVERAGE.md`                                 | Test-design doc; could go under Operations or Audits.             |

Recommendation: a single PR adds all 9 to the nav, possibly creating
a "Reference" tab (`Glossary`, `Security model`, `Pattern: HTTPS trust`).

### Missing Go API reference markdown (auto-generated, but worth pinning)

Pre-PR-#213, `mkdocs.yml` listed six per-package pages under
`api/pkg/*` but only `api/pkg/index.md` existed in source — the rest
were generated at build time by `scripts/gen-godoc.sh`. That worked
(all six probed HTTP 200) but it meant a contributor running
`mkdocs serve` locally without first running `gen-godoc.sh` would
hit a half-broken nav.

**PR #213 (gateway raise)** scaled this up: `mkdocs.yml:136-140` now
lists `api/gateway/{index,common,route,service}.md`, all four
generated. Same caveat applies, more files. Worth a one-line note in
both `docs/api/pkg/index.md` and `docs/api/gateway/index.md`
("regenerate via `./scripts/gen-godoc.sh` before `mkdocs serve`").

---

## 2. README + repo-root docs

### `README.md`

Reading top-to-bottom against the v0.5.11 product:

| Line  | Issue                                                                                                    | Severity |
|-------|----------------------------------------------------------------------------------------------------------|----------|
| 41    | Badge says "Backend: Go 1.21+". Real `go.mod` files declare `1.25.7` (or `1.25` for legacy modules). Misleading for contributors guessing what compiler to install. | low      |
| 167-176 | "**Coming soon: a first-class Models tab.**" — explicit "coming soon" placeholder copy. Either deliver, soften the language, or move to roadmap. | low      |
| 198   | `--version v0.1.5` example — current line is v0.5.11. Replace with `v0.5.x` (e.g. `v0.5.11` or `v0.5.10`) or remove the version literal. | medium   |
| 332   | ASCII architecture box shows gateway listening on `:80 / :443`. Real ports: `:8765` (HTTP) and `:8443` (HTTPS). HTTPS line in line 69 above already references the correct setup, so the diagram is the only liar. | medium   |

The hero image (`docs/img/login.png`) and the six tour PNGs all
exist on disk — no broken `<img>` references.

CasaOS surface check: README mentions CasaOS exactly once
(line 49 — that's the historical-fork pointer, kept on purpose per
ADR-0022). Compliant.

### `CONTRIBUTING.md` — multiple stale facts

| Line  | Issue                                                                                                             | Severity                       |
|-------|-------------------------------------------------------------------------------------------------------------------|--------------------------------|
| 90    | "Go 1.21 or higher." Should be "Go 1.25 or higher" to match `go.mod`.                                              | low                            |
| 91    | "Node.js (v18+ recommended)" — README says v20+. Pick one (v20 is what `dev.sh` actually requires).                | low                            |
| 99    | `./start.sh --build` — repo's onboarding flow is `./dev.sh`, not `start.sh`. Both exist; `dev.sh` is the documented one. | medium                         |
| 101   | Gateway port "`http://localhost:80` (or `8089` depending on config)". Real ports: `8765` HTTP / `8443` HTTPS. `8089` is not used anywhere in the codebase. | **high — actively misleading** |
| 132   | "`backend/common/utils/constants/paths.go`" — file still exists, but the canonical path-resolution helpers are now in `backend/common/utils/paths/db.go` (per the work review of 2026-05-10). Direct contributors at the wrong file. | medium |
| 148   | **"Your contributions will be licensed under the PolyForm Noncommercial License 1.0.0."** — `LICENSE` file is **AGPL-3.0**. README, mkdocs site, and SECURITY.md all say AGPL. CONTRIBUTING is a single-source-of-truth bug that misrepresents the legal contract under which contributions land. | **CRITICAL** |

The CONTRIBUTING.md license bug is the highest-severity finding in
this audit. Land a fix in the next PR.

### `SUPPORT.md` — line 17

`Out of scope for v0.1.x` — at v0.5.11, the "v0.1.x" framing is stale.
Replace with "Out of scope" (no version qualifier) or "Out of scope
through the v0.5.x line."

### `SECURITY.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`

- `SECURITY.md` — current and correct. Cites ADR-0022 (line 11) for
  the CasaOS relationship. No findings.
- `CODE_OF_CONDUCT.md` — minimal but accurate. No findings.
- `CHANGELOG.md` — 12 release headers, format consistent
  (`## [vX.Y.Z] — YYYY-MM-DD`), Keep-a-Changelog compliant. No findings.

---

## 3. Go godoc raise punchlist

### Methodology + caveat

A Python analyzer scans every non-codegen, non-test, non-vendor
`.go` file under `backend/<module>/`. A decl is "documented" when
the line immediately above starts with `//`.

**Known under-counting** (this matters for reading the table below):

- Methods on **unexported** types (e.g. `func (r *recoveredPanic)
  Error()` in `backend/pkg/lifecycle/middleware.go:84`) get flagged
  as "exported undocumented" because the method name is capital.
- `var ( ... )` blocks where the block has a header comment but
  individual entries don't get a per-line comment (e.g. the
  canonical HTTP error catalogue in
  `backend/pkg/errors/errors.go:117-129`) are counted as undocumented.
- This is why the analyzer reads `pkg` at 61.7 % (real ~95 %+) and
  reads gateway at 42.5 % (PR #213 reports 85 %).

The **relative ranking across modules and the per-decl file:line
citations remain correct** — only the absolute % is conservative.

### Per-module table (vs 70 % bar)

| Module          | Exported | Documented | Analyzer cov. | vs 70 % bar | Promoted to `gen-godoc.sh`? |
|-----------------|----------|------------|---------------|-------------|------------------------------|
| `pkg`           | 47       | 29         | 61.7 %        | -8.3 pp     | **yes** (since Sprint 4)     |
| `gateway`       | 73       | 31         | 42.5 %        | -27.5 pp    | **yes** (PR #213, Sprint 5)  |
| `common`        | 318      | 92         | 28.9 %        | -41.1 pp    | no                           |
| `user-service`  | 124      | 31         | 25.0 %        | -45.0 pp    | no — godoc raise IN FLIGHT (`sprint-5/godoc-user-service-raise` worktree active at audit time) |
| `local-storage` | 385      | 80         | 20.8 %        | -49.2 pp    | no                           |
| `core`          | 737      | 147        | 19.9 %        | -50.1 pp    | no                           |
| `app-management`| 726      | 84         | 11.6 %        | -58.4 pp    | no                           |
| `message-bus`   | 308      | 8          | 2.6 %         | -67.4 pp    | no                           |
| `cli`           | 39       | 1          | 2.6 %         | -67.4 pp    | no                           |

### Punch-list — top 5 undocumented exports per under-bar module

These are the highest-impact sites (route entry points, public
service constructors, `TableName()` methods on GORM models) — fixing
them first delivers the most reader value per minute of doc-writing.

#### `common` (28.9 %)

1. `backend/common/model/gateway.go:3 type Route` — describes the gateway's route table entry as serialised on disk and over the wire.
2. `backend/common/model/gateway.go:8 type ChangePortRequest` — payload for the runtime port-change endpoint; tells callers what fields to populate.
3. `backend/common/pkg/mod_management/sdk.go:19 type ModManagementClient` — public SDK type used by every service that talks to app-management.
4. `backend/common/pkg/mod_management/sdk.go:27 func NewClient` — constructor; document the required `ModManagementClientOpts` and the gateway it points at.
5. `backend/common/model/notify/application.go:3 type Application` — payload model emitted on every app lifecycle event.

#### `core` (19.9 %)

1. `backend/core/route/v1.go:17 func InitV1Router` — top-of-tree v1 router builder; explain when it's called (boot) and what middleware it wires.
2. `backend/core/route/v2.go:53 func InitV2Router` — v2 OpenAPI-bound router; explain the codegen contract.
3. `backend/core/route/v2.go:30 var V2APIPath` / `:31 var V2DocPath` — exported route prefixes; callers depend on these literals.
4. `backend/core/route/periodical_darwin.go:6 func SendAllHardwareStatusBySocket` — macOS-only telemetry sender; document the socket protocol + cadence.
5. `backend/core/service/model/o_*.go` — five `TableName()` methods for GORM models — document the table name + retention/ownership.

#### `app-management` (11.6 %)

1. `backend/app-management/route/v1.go:21 func InitV1Router` + `:94 func InitV1DocRouter` — entry points for the legacy app-management API.
2. `backend/app-management/route/v2.go:47 func InitV2Router` + `:173 func InitV2DocRouter` — entry points for v2 (the only one the UI consumes).
3. `backend/app-management/route/v1/docker.go:431 func GetDockerNetworks` and 4 sibling exported route handlers — document the JSON shape callers receive.
4. `backend/app-management/service/container.go:382 func CreateContainer` (193 LoC) and `:576 func RecreateContainer` (192 LoC) — load-bearing for the install pipeline; document the SSE-task contract.
5. `backend/app-management/route/v2/global.go:19 method GetGlobalSettings` + `:34 method GetGlobalSetting` — exposed via OpenAPI; document the persisted fields.

#### `local-storage` (20.8 %)

1. `backend/local-storage/route/v1.go:16 func InitV1Router` and `route/v2.go:46 func InitV2Router` — module entry points.
2. `backend/local-storage/route/v1/disk.go:23 type StorageMessage` — wire type emitted on disk events.
3. `backend/local-storage/route/v1/disk.go:38 func GetDiskList` (118 LoC) — document the platform-specific behaviour (USB hotplug, RAID detection).
4. `backend/local-storage/route/v1/storage.go:138 func PostAddStorage` (146 LoC) — destructive operation; document side effects + idempotency.
5. `backend/local-storage/service/model/o_volume.go:3 type Volume` + `:10 method TableName` — primary persistence model.

#### `message-bus` (2.6 %)

1. `backend/message-bus/route/api_route.go:9 type APIRoute` + `:15 NewAPIRoute` — the entire HTTP surface.
2. `backend/message-bus/route/api_route_event.go:24-82` — the five `GetEventTypes` / `RegisterEventTypes` / `GetEventTypesBySourceID` / `GetEventType` / `PublishEvent` methods. The HTTP shape is **the** contract this service exposes.
3. `backend/message-bus/route/api_route_action.go:21-109` — same shape, action side.
4. `backend/message-bus/route/api_route_event.go` (subscribe path) — document the SSE/WebSocket protocol.
5. `backend/message-bus/route/ysk.go:12 method DeleteYskCard` + `:24 method GetYskCard` — YSK-card legacy endpoints; either document or schedule for kill.

#### `user-service` (25.0 %, raise in flight)

The `sprint-5/godoc-user-service-raise` worktree is active at audit
time, so this list is the **starting state** that PR will land
against. If the PR completes before this audit merges, treat this
section as historical context.

1. `backend/user-service/route/v1.go:15 func InitRouter` + `route/v2.go:45 func InitV2Router` + `:96 InitV2DocRouter` — entry points.
2. `backend/user-service/route/v1/user.go:476 func GetUserInfoByUsername` and 5 sibling user-CRUD routes — document the JSON contract + auth requirements.
3. `backend/user-service/route/v1/user.go:679 func PostUserUploadImage` + `:720 func GetUserImage` — user-avatar surface; document size limits + storage path.
4. `backend/user-service/route/v1/user.go:620 func DeleteUser` — destructive; document cascade + auth.
5. `backend/user-service/service/model/o_user.go:21 method TableName` — primary user table name.

#### `cli` (2.6 %)

1. `backend/cli/cmd/user.go:30 const BasePathUsers` — CLI command base.
2. `backend/cli/cmd/messageBus.go:30 const BasePathMessageBus` + `:32-37` flag constants — document each flag's accepted values.
3. `backend/cli/cmd/appManagement.go:23-25` flag constants — same.
4. `backend/cli/cmd/appManagementSearch.go:31-33` flag constants — same.
5. `backend/cli/cmd/appManagementLogs.go:28 const FlagAppManagementLogsLines` — log-tail behaviour.

### Suggested raise-PR shape

Pattern is now established (PR #213 for gateway): **one PR per
module, smallest-first.** Remaining order (after the in-flight
`user-service` PR):

1. `cli` (39 exports — mechanical, mostly flag constants)
2. `common` (318 — many GORM model namesake docs)
3. `local-storage` (385)
4. `core` (737)
5. `app-management` (726)
6. `message-bus` (308 — needs a foundation pass first; consider an ADR documenting "message-bus is a thin pub/sub; the OpenAPI schema is the contract" before per-route godoc)

Each PR adds godoc to the top 15-25 exported decls in its module
and updates `gen-godoc.sh:27 MODULES=("pkg" "gateway" …)` once the
module crosses the bar. Don't try to do all modules in one mega-PR
— review fatigue will swallow the gain.

---

## 4. ADR + audit inter-link

### Cross-link health summary

| Check                                                                                | Status     | Notes                                                                                       |
|--------------------------------------------------------------------------------------|------------|---------------------------------------------------------------------------------------------|
| ADR-0021 references ADR-0019                                                         | OK         | `0021-docker-label-namespace-and-appdata-path.md:232`                                       |
| ADR-0022 references ADR-0025 (strangler, renumbered from 0011) + ADR-0021             | OK         | `0022-...:19, 104, 169, 193, 194`                                                            |
| All ADRs have `Status: accepted`                                                     | OK         | None in `proposed` limbo. None marked `superseded by`.                                       |
| 7 of 9 audits cite at least one ADR                                                  | partial    | `audits/dead-code.md` and `audits/ui-feature-map.md` cite zero ADRs.                        |
| ADR index (`decisions/README.md`) lists every ADR                                    | **resolved 2026-05-11** | ~~Index table stops at ADR-0012. Ten ADRs (0013–0022) are missing from the index.~~ Index now lists 0011–0023 + 0025 + 0026 (updated in the same renumber PR).  |
| ADR file numbers are unique                                                          | **resolved 2026-05-11** | Foundation pair renumbered to 0025 + 0026 (see below).                                       |
| All load-bearing framework choices have an ADR                                       | partial    | **No ADR for Uber fx, Echo, GORM** (see below).                                              |

### ~~Critical: duplicate ADR numbers~~ — resolved 2026-05-11

Two pairs of ADR files originally shared a number. The CA series (0010–0012) was filed on 2026-05-07; the foundation `backend/pkg/` series was filed on 2026-05-08 in a parallel branch and accidentally re-used 0011 + 0012.

**Resolution (Sprint 11):** the foundation pair was renumbered:

| Old | New | File |
|-----|-----|------|
| 0011 | **0025** | `decisions/0025-backend-pkg-coexistence-with-casaos-common.md` |
| 0012 | **0026** | `decisions/0026-pkg-logging-built-on-stdlib-slog.md` |

ADR-0011 + ADR-0012 are now unambiguously the CA-mismatch + CA-rotation ADRs. Cross-references in `backend/`, `docs/`, `CHANGELOG.md` were updated; each renumbered ADR carries a "Renumber history" note at the top so historical references still resolve.

### Critical: missing ADR index entries

`decisions/README.md` has a markdown table at lines 64-77 listing
ADRs 0001 through 0012. Then it stops. The following ADRs are
filed in `docs/decisions/` but **absent from the index**:

| ADR  | Title                                                                                       | File                                                                          |
|------|---------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| 0013 | `pkg/errors` — typed error with code + i18n + status                                         | `0013-pkg-errors-typed-error-with-code-i18n-status.md`                        |
| 0014 | `pkg/lifecycle` — graceful shutdown + panic recovery                                         | `0014-pkg-lifecycle-graceful-shutdown-and-panic-recovery.md`                  |
| 0015 | `pkg/tracing` — correlation ID via X-Request-ID                                              | `0015-pkg-tracing-correlation-id-via-x-request-id-header.md`                  |
| 0016 | Modular kill scope vs full extraction                                                        | `0016-modular-kill-scope-vs-full-extraction.md`                               |
| 0017 | `changie` for changelog fragments                                                            | `0017-changie-for-changelog-fragments.md`                                     |
| 0018 | `goose` for versioned migrations                                                             | `0018-goose-for-versioned-migrations.md`                                      |
| 0019 | Tech debt lives in audits + ADRs + labeled issues                                            | `0019-tech-debt-tracked-in-audits-adrs-and-issues.md`                         |
| 0020 | JWT keypair persisted by default                                                             | `0020-jwt-keypair-persisted-by-default.md`                                    |
| 0021 | Docker label namespace + AppData path (CasaOS coexistence)                                   | `0021-docker-label-namespace-and-appdata-path.md`                             |
| 0022 | CasaOS upstream is abandoned — no new dependencies                                           | `0022-casaos-upstream-is-abandoned-no-new-dependencies.md`                    |

### Audits with no ADR cross-reference

- `docs/audits/dead-code.md` — should at least cite ADR-0019 (where dead-code-pruning is named as a tech-debt category).
- `docs/audits/ui-feature-map.md` — same; also a candidate to cite ADR-0021 if it touches CasaOS-coexistence pages.

### Decisions made in code that should be ADRs

These are load-bearing framework / architectural choices visible in
the codebase but unrecorded as ADRs. ADR-0019 is explicit that
"locked-in technical decisions live in ADRs," so by the project's
own rules these should land:

| Decision                                                                                  | Evidence in code                                                       | Suggested ADR                                                          |
|-------------------------------------------------------------------------------------------|------------------------------------------------------------------------|------------------------------------------------------------------------|
| **Uber `fx` for DI in the gateway**                                                        | `backend/gateway/main.go` + `backend/gateway/go.mod` (sole importer)   | "0023 — Uber `fx` is the DI framework for the gateway only"           |
| **Echo (v4) as the HTTP framework for every service**                                      | All `route/*.go` files import `github.com/labstack/echo/v4`            | "0024 — Echo v4 over chi/gin — why"                                    |
| **GORM as the ORM for every persistent service** (with goose for migrations per ADR-0018) | Service-side `service/model/o_*.go` files inherit the GORM contract    | "0025 — GORM is the ORM; AutoMigrate is forbidden (use goose per ADR-0018)" |
| **Svelte 5 Runes lock-in (no Svelte 4 stores anywhere in the UI)**                         | `ui/src/**/*.svelte` + the `index.svelte.ts` i18n trick                | "0026 — Svelte 5 Runes; no stores; never rename `index.svelte.ts`"     |
| **`oapi-codegen` as the OpenAPI -> Go binding generator**                                  | Per-service `openapi.yaml` + `.gitignored` `codegen/` dirs             | "0027 — `oapi-codegen` for OpenAPI -> Go; codegen output is gitignored" |
| **mDNS via Avahi (Linux) with a direct-multicast fallback**                                | `backend/gateway/service/mdns.go` + Avahi service file at install      | "0028 — mDNS strategy: Avahi preferred, direct-multicast fallback"     |
| **`backend/pkg/` is a separate go.mod; never imports from `common`**                       | `backend/pkg/go.mod` is standalone; documented in ADR-0025             | Already covered by ADR-0025 (renumbered from the duplicate ADR-0011-coexist).      |

The first six are real backlog items. Items beyond can wait until
v1.0 prep without harm — they're decisions but not ones a reader is
likely to second-guess in the next quarter.

---

## 5. TODO / FIXME + smells

### TODO/FIXME inventory (25 backend, 0 UI, 0 scripts)

Within ADR-0019's "27 ceiling." Distribution:

| File / package                                                                  | Count | Category                                                                                              |
|---------------------------------------------------------------------------------|-------|-------------------------------------------------------------------------------------------------------|
| `backend/message-bus/service/socketio_service.go`                               | 5     | Inherited CasaOS code; mostly "// TODO add connector info" — minor. Two are "// TODO remove this debug setting" on `CheckOrigin` returns — security smell, see below. |
| `backend/message-bus/service/event_*_service*.go` + `action_*_service*.go`      | 4     | "TODO ensure properties are valid" / "ensure URL safe" — input-validation gaps.                       |
| `backend/app-management/route/v2/compose_app.go`                                | 3     | Real product debt: `:110 status uncontrolled if user not update`, `:629 containersWorkaround`, `:780 needs re-design`. |
| `backend/app-management/service/container.go`                                   | 2     | `:45 unused NewVersionApp map`, `:80` **HTTP polling instead of WebSocket** — real architectural debt. |
| `backend/app-management/service/compose_app.go`                                 | 2     | Refactor markers.                                                                                      |
| `backend/app-management/service/appstore_management.go:162`                     | 1     | "TODO: refactor the function and above function".                                                      |
| `backend/app-management/route/v2/appstore.go:444`                               | 1     | Bare `// TODO`.                                                                                        |
| `backend/core/route/v1/zerotier.go:68`                                          | 1     | Bare `// TODO` — dead-codish (zerotier is inherited from CasaOS).                                      |
| `backend/local-storage/pkg/utils/command/command_helper.go:18`                  | 1     | External link in comment — minor.                                                                      |
| `backend/local-storage/pkg/mount/dir.go:39, 40, 123`                            | 3     | All "// FIXME" inside the FUSE mount adapter — inherited.                                              |
| `backend/user-service/route/v1/user.go:219`                                     | 1     | "TODO:1 Database fields cannot be external" — real but tiny.                                           |
| `backend/message-bus/route/api_route_event_test.go:93`                          | 1     | "// subscribe event type - TODO" — incomplete test (and hides a missing assertion path).               |

### Highest-severity smells (with file:line)

#### A. WebSocket `CheckOrigin` bypassed in message-bus (security)

**File:** `backend/message-bus/service/socketio_service.go:53` and `:58`

```go
return true // TODO remove this debug setting
return true // TODO remove this debug setting
```

Two `CheckOrigin` callbacks unconditionally return `true`, comment says
"TODO remove this debug setting." This is a **CSRF / cross-site
WebSocket hijacking** primitive. message-bus is reachable through the
gateway from any browser the user is logged into. Should land a
proper origin check (allowlist or `Origin` <-> `Host` equality) before
v0.6.

#### B. HTTP polling instead of WebSocket (architecture)

**File:** `backend/app-management/service/container.go:80`

```go
// FIXME - should use WebSocket or SocketIO instead of HTTP polling (tiger)
```

Pre-fork CasaOS comment; this drives the install-progress UI today.
Either commit to "polling is fine, drop the FIXME" or schedule the
WebSocket migration as a Sprint 6 backlog item.

#### C. Three live request-path `panic()` calls (reliability)

| File:line                                                          | Call          | Why it's bad                                                              |
|--------------------------------------------------------------------|---------------|---------------------------------------------------------------------------|
| `backend/local-storage/service/disk.go:90`                         | `panic(err)`  | In disk-listing logic; a transient `udev` error crashes the service.      |
| `backend/local-storage/service/disk.go:114`                        | `panic(err)`  | Same code path.                                                           |
| `backend/local-storage/service/disk.go:147`                        | `panic(err)`  | Same.                                                                     |
| `backend/core/route/v1/file.go:243`                                | `panic(err)`  | **Inside an HTTP handler** — propagates as 500 + (depending on recovery middleware) potentially crashes the worker. |

The `pkg/lifecycle` recover middleware (per ADR-0014) catches these,
so the user sees a 500 instead of a process restart — but they're
still avoidable. Convert to `return err` + the standard error
response shape.

#### D. Files > 1000 lines

| File                                                              | Lines | Why it's a smell                                                       |
|-------------------------------------------------------------------|-------|------------------------------------------------------------------------|
| `backend/app-management/service/compose_app.go`                   | 1,276 | The compose lifecycle "god file." Already has 3 of its own TODOs.      |
| `backend/core/route/v1/file.go`                                   | 1,166 | File-manager v1 router; mixed responsibilities (upload, websocket, panic-on-encode). |
| `ui/src/routes/apps/+page.svelte`                                 | 1,561 | App-store page with install pipeline + filter + grid all inline.       |
| `ui/src/routes/settings/+page.svelte`                             | 1,469 | Settings monolith; should split per-pane.                              |

Not a v0.6 blocker; a slow-burn "split when you touch this file."

#### E. Functions > 130 lines (the worst eight)

These are obvious extraction candidates:

| Function                                                             | Lines | File                                                |
|----------------------------------------------------------------------|-------|------------------------------------------------------|
| `InitV1Router`                                                       | 223   | `backend/core/route/v1.go:17`                        |
| `main`                                                               | 196   | `backend/message-bus/main.go:72`                    |
| `CreateContainer`                                                    | 193   | `backend/app-management/service/container.go:382`   |
| `RecreateContainer`                                                  | 192   | `backend/app-management/service/container.go:576`   |
| `SendFileOperateNotify`                                              | 157   | `backend/core/service/notify.go:73`                  |
| `PostAddStorage`                                                     | 146   | `backend/local-storage/route/v1/storage.go:138`     |
| `main`                                                               | 139   | `backend/app-management/main.go:51`                  |
| `main`                                                               | 136   | `backend/core/main.go:142`                           |

The four `main()` bodies are wiring code; less of a smell. The four
service/route ones are real candidates for extraction.

#### F. Skipped UI test that hides missing coverage

**File:** `ui/src/lib/components/files/TextEditor.test.ts:229`

```ts
it.skip('calls toast.success on PUT save of an existing file', async () => {
```

Either implement (and let v0.6 ship with the toast verified) or
delete (and accept that toast is verified manually). A skipped test
silently rots.

---

## Recommendation: Sprint 5.5 quality wave

Ordered by **leverage / risk** — each item is a single PR.

1. **License clarity in CONTRIBUTING.md** *(line 148: PolyForm -> AGPL-3.0)*. Single-line PR. **CRITICAL — legal contract bug.** Land first.
2. **CONTRIBUTING.md port + tooling sweep** *(line 90 Go 1.21 -> 1.25; line 91 Node 18 -> 20; line 99 `start.sh` -> `dev.sh`; line 101 ports 80/8089 -> 8765/8443)*. Aligns onboarding with reality.
3. **README architecture diagram + version pin** *(line 198 v0.1.5; line 332 :80/:443; line 41 Go 1.21+ badge; line 167-176 "Coming soon")*.
4. **SUPPORT.md v0.1.x -> drop version qualifier** *(line 17)*.
5. ~~**Resolve duplicate ADR numbers (0011, 0012)**~~. **Done 2026-05-11.** Renumbered the foundation pair to 0025 (`backend-pkg-coexistence-with-casaos-common`) + 0026 (`pkg-logging-built-on-stdlib-slog`); cross-references in `backend/`, `docs/`, `CHANGELOG.md` updated; `decisions/README.md` index now lists 0011–0023 + 0025 + 0026 (with the renumber note in the footer).
6. **Update `decisions/README.md` index table to cover ADR-0013 through ADR-0022**. Same PR as #5 if cohesive.
7. **mkdocs.yml nav refresh** — add the 9 orphaned pages (concepts, migrating-from-casaos, backup-restore, three audits, the trust-onboarding pattern, STORE-COVERAGE). Possibly create a "Reference" tab.
8. **WebSocket origin check in message-bus** *(`socketio_service.go:53`/`:58`)*. Security-tier fix.
9. **Convert the four live-path panics to error returns** *(`local-storage/service/disk.go:90/114/147`, `core/route/v1/file.go:243`)*.
10. **Implement-or-delete the skipped UI test** *(`TextEditor.test.ts:229`)*.
11. **`user-service` godoc raise** — already in flight (`sprint-5/godoc-user-service-raise` worktree). Promote to `gen-godoc.sh:27` MODULES once it crosses 70 % per the analyzer.
12. **`cli` godoc raise** — flags-and-base-paths only; mostly mechanical.
13. **`common` godoc raise** — 318 exports; the GORM model namesake docs are mostly one-liners.
14. **`local-storage` godoc raise** — 385 exports.
15. **`core` godoc raise** — 737 exports; do this AFTER the cli/user-service/common wins to build muscle.
16. **`app-management` godoc raise** — 726 exports; biggest module + most complex public contracts (SSE tasks). Hold for last.
17. **`message-bus` godoc raise + ADR for the message-bus contract** — at 2.6 % it needs a foundation pass first; consider an ADR documenting "message-bus is a thin pub/sub; the OpenAPI schema is the contract" before adding godoc per route.
18. **Six new ADRs for undocumented framework choices** (Uber fx, Echo, GORM, Svelte 5 runes, oapi-codegen, mDNS). Spread across PRs as touched by other work; do not block on a six-ADR mega-PR.
19. **Note in `docs/api/pkg/index.md` and `docs/api/gateway/index.md`**: "regenerate via `./scripts/gen-godoc.sh` before running `mkdocs serve` locally." One-line PR; closes the local-dev half-broken-nav footgun.
20. **Split `compose_app.go`, `file.go`, `apps/+page.svelte`, `settings/+page.svelte`**. Sprint 6 / "when you touch it" — not a 5.5 must.

Items 1-10 are achievable in a single weekend; together they retire
the highest-leverage debt and unblock items 11-17 (godoc raise
work) which is the **structural** lift from "we have docs" to
**Airflow-level** ("every export is documented, every page is in
the IA, every framework choice has an ADR"). Items 18-20 are the
slow-burn refactor backlog that lives most happily on `someday` /
`sprint-6` / opportunistic touch-and-go work.

---

## Audit doc inventory (so the next audit can diff)

- 38 docs site URLs probed (live, all 200).
- 6 architecture pages confirmed with mermaid render + script tag.
- 459 Go source files under `backend/` (excl. codegen, vendor, tests).
- 25 backend `// TODO` / `// FIXME` markers; **0 in UI src**, **0 in `scripts/`**.
- 113 `panic()` calls; **3 in live request paths**.
- **1** UI test skip; 8 environmental Go test skips.
- 4 files > 1000 lines; 25 functions > 100 lines.
- 24 ADR files (with **2 number collisions**); 9 audits.
- ADR index in `decisions/README.md` covers **only 12 of 24 ADRs**.
- 9 published doc pages reachable by URL but **absent from the
  mkdocs nav**.
- Per-module godoc coverage (analyzer numbers): `pkg` 61.7 % (real
  ~95 %+), `gateway` 42.5 % (PR #213 reports 85 %), then
  `common` 28.9 %, `user-service` 25.0 %, `gateway` was 23.3 %
  pre-PR-#213; **6 of 7 remaining service modules below 30 %**.
- Two of those 7 have an in-flight godoc raise PR at audit time
  (`user-service`).
