# Glossary

Short definitions for terms that recur across the docs, ADRs, and audits. The intent is to give a reader landing on any single page enough vocabulary to follow the cross-links without bouncing through three more documents first.

Each entry links to the canonical source — ADR, audit, or pattern doc — for the long form.

## Core nouns

### Compose project
A logical group of one or more containers managed as a unit by Docker Compose, identified by its project name. Every PowerLab "app" is implemented as a compose project: PowerLab generates `docker-compose.yml` under `<AppsPath>/<app>/` and runs `docker compose up` against it. The project name is the `app.id` (see below). See [data persistence](../architecture/data-persistence.md) for where the YAMLs live on disk.

### App
A user-facing installable unit in PowerLab. Two IDs identify it:

- `app.id` — the **instance** identifier, unique per install on this host. This is the compose project name on disk.
- `store_app_id` — the **catalog** identifier from the app store entry. Multiple installs of the same store app produce multiple `app.id`s.

### Custom app
An app a user installs by pasting their own `docker-compose.yml` rather than picking from the store catalog. Lives in the same `<AppsPath>/<app>/` tree, carries the `io.powerlab.v1.origin = "local"` label (vs `system` for store-installed). See [ADR-0021](../decisions/0021-docker-label-namespace-and-appdata-path.md) for the label scheme.

### AppData
The per-app persistent volume tree. Two paths in the wild today:

- `<StoragePath>/AppData/<app>` — the inherited CasaOS path. Apps installed before Sprint 4 (v0.5.7+) and CasaOS itself still write here.
- `<StoragePath>/PowerLabAppData/<app>` — the canonical PowerLab path. New installs (post-Sprint-4) write here.

`<StoragePath>` defaults to `/DATA` on Linux, a Docker Desktop-accessible path on macOS dev. The split is an explicit coexistence decision documented in [ADR-0021](../decisions/0021-docker-label-namespace-and-appdata-path.md) — see also the [coexistence overview](../coexistence/README.md).

### Store app
An app catalogued in the PowerLab app store and installable with one click. The store data is cached under `/var/lib/powerlab/appstore/` and refreshed on a cadence; see [data persistence](../architecture/data-persistence.md).

## Migration vocabulary

### Dual-write window
A bounded release window during which PowerLab writes BOTH a new canonical form AND the legacy form of some persisted data, so old code paths can keep reading while consumers migrate to the new shape. The two concrete uses today:

- **Docker labels** — one-release-window dual-write of `io.powerlab.v1.*` (canonical) alongside flat `casaos`/`origin`/etc. (legacy). [ADR-0021](../decisions/0021-docker-label-namespace-and-appdata-path.md) covers the rationale and the close-out criteria.
- **JWT keypair persistence** — N/A here (the JWT change in [ADR-0020](../decisions/0020-jwt-keypair-persisted-by-default.md) is not a dual-write — it's a single-write migration from ephemeral to persisted).

The intent is always to migrate, not to coexist with our own legacy indefinitely. After the window, the legacy writes drop and the next container/file recreate produces clean data.

### Split-brain DB
The condition where the same logical database file appears at two distinct on-disk paths and a service has to decide which is canonical. Concretely: the user-service database at both the legacy CasaOS path and the canonical PowerLab path. The current detector (`backend/common/utils/paths/db.go`) refuses to start when both paths are non-empty and disagree, surfacing `ErrSplitBrain` rather than silently picking one.

See the [database paths audit](../audits/db-paths.md) for the full path list and the recovery flow.

### Foundation packages (`pkg/*`)
Four PowerLab-owned Go packages every service composes onto: `pkg/logging`, `pkg/errors`, `pkg/lifecycle`, `pkg/tracing`. They form the spine that replaces the inherited CasaOS-Common utilities. Each is documented in its own ADR (0012–0015) and the relationship view lives in [foundation interfaces](../architecture/foundation-interfaces.md).

The motivation for owning these — rather than continuing to import CasaOS-Common — is captured in [ADR-0011 (backend/pkg coexistence)](../decisions/0011-backend-pkg-coexistence-with-casaos-common.md) and [ADR-0016 (modular kill scope)](../decisions/0016-modular-kill-scope-vs-full-extraction.md).

### Strangler pattern
The migration strategy applied across the CasaOS-strip roadmap: stand up the PowerLab-owned replacement next to the inherited code, route new work through the replacement, and remove the original once it has no callers. Each "kill" PR is one slice of the strangler. The live tracker is [`architecture/casaos-strangler.md`](../architecture/casaos-strangler.md).

### Kill PR
A pull request that removes a CasaOS-Common dependency from one service. Killing a service is a multi-PR series — typically four PRs (rebrand, middleware swap, logger swap, dead-code review) — collectively called a "kill". [ADR-0016](../decisions/0016-modular-kill-scope-vs-full-extraction.md) defines the staging: Stage A "modular kill" (per service, per sprint) vs Stage B "full extraction" (cross-cutting, in the v1.0 stabilization window).

## To expand

This page is a starter set. Terms surfacing in v0.6+ work — refresh-token flow, MCP server endpoints, mkdocs-material site model — should be added as they stabilize. Track gaps under the docs site polish issue series.
