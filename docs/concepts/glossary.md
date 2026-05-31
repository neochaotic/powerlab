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
Four PowerLab-owned Go packages every service composes onto: `pkg/logging`, `pkg/errors`, `pkg/lifecycle`, `pkg/tracing`. They form the spine that replaces the inherited CasaOS-Common utilities. Each is documented in its own ADR (0013–0015 + 0026 for pkg/logging — renumbered from 0012 to break a collision with the CA series) and the relationship view lives in [foundation interfaces](../architecture/foundation-interfaces.md).

The motivation for owning these — rather than continuing to import CasaOS-Common — is captured in [ADR-0025 (backend/pkg coexistence)](../decisions/0025-backend-pkg-coexistence-with-casaos-common.md) and [ADR-0016 (modular kill scope)](../decisions/0016-modular-kill-scope-vs-full-extraction.md).

### Strangler pattern
The migration strategy applied across the CasaOS-strip roadmap: stand up the PowerLab-owned replacement next to the inherited code, route new work through the replacement, and remove the original once it has no callers. Each "kill" PR is one slice of the strangler. The live tracker is [`architecture/casaos-strangler.md`](../architecture/casaos-strangler.md).

### Kill PR
A pull request that removes a CasaOS-Common dependency from one service. Killing a service is a multi-PR series — typically four PRs (rebrand, middleware swap, logger swap, dead-code review) — collectively called a "kill". [ADR-0016](../decisions/0016-modular-kill-scope-vs-full-extraction.md) defines the staging: Stage A "modular kill" (per service, per sprint) vs Stage B "full extraction" (cross-cutting, in the v1.0 stabilization window).

## MCP vocabulary

### MCP (Model Context Protocol)
The open protocol for connecting AI agents to data + tools, defined at [spec.modelcontextprotocol.io](https://spec.modelcontextprotocol.io). PowerLab ships its own MCP server (`powerlab-mcp.service` on `:9090`) exposing the homelab as an agent-readable surface. See [MCP overview](mcp-server.md) for the architecture; see [Operations → MCP quickstart](../operations/mcp-quickstart.md) for the 5-minute path.

### MCP resource
Read-only data exposed via an MCP URI scheme. PowerLab advertises 16 today across six namespaces: `system://`, `journal://`, `audit://`, `apps://`, `docker://`, `docs://`. Each has a `<namespace>://schema` introspection entry-point.

### MCP tool
A *callable* MCP operation with side effects. PowerLab ships 5 tools across three tiers (per [ADR-0046](../decisions/0046-mcp-tool-curation-strategy.md)): READ ONLY (`journal_search`, `check_disk_free`), SIDE EFFECT bounded (`restart_app`), DESTRUCTIVE gated (`install_app`, `uninstall_app` — operator opt-in via `EnableDestructiveTools = true` in mcp.conf).

### Hybrid architecture (ADR-0044)
The principle that powerlab-mcp **proxies** existing core/app-management endpoints rather than reimplementing telemetry. Audit + journal are independent (survive core being down); system/apps/docker thin-proxy to upstream. The point is "no duplicated gopsutil bumps, no parallel /proc parsers". See [ADR-0044](../decisions/0044-mcp-hybrid-architecture-thin-proxy-to-core.md).

### Storage-agnostic (ADR-0045)
The promise that a future migration (SQLite → PostgreSQL, etc.) on any backend service requires **zero** MCP changes — the HTTP contract is the abstraction. See [ADR-0045](../decisions/0045-mcp-apps-docker-via-app-management-http-proxy.md).

### compose-conventions
The PowerLab patterns every catalog app follows: `/DATA/PowerLabAppData/<id>/` volume paths, named networks, healthcheck idioms, never `:latest`, never `privileged: true` or docker.sock binds. The composevalidator (and `install_app`) enforce the deny-list before forwarding YAML upstream. See `compose-conventions` concept doc and [ADR-0046 §4](../decisions/0046-mcp-tool-curation-strategy.md).

### EnableDestructiveTools
The mcp.conf knob that controls whether `install_app` and `uninstall_app` are advertised in `tools/list`. **Default: false** (the destructive tools don't exist as far as the agent can tell). Set to `true` to opt in; restart `powerlab-mcp` to apply.

### Loopback skip
The auth policy where same-host (`127.0.0.1`) callers don't need a JWT. PowerLab's MCP gate honours this for trusted local agents (Claude Desktop running on the box itself) while requiring JWT for LAN callers. Per [ADR-0034](../decisions/0034-standalone-observability-mcp-service.md) amended.

## To expand

This page is a starter set. Refresh-token flow + mkdocs-material site model still pending. Track gaps under the docs site polish issue series.
