# 0045 — apps:// and docker:// resources via HTTP proxy to app-management (storage-agnostic)

- **Status:** accepted
- **Date:** 2026-05-28
- **Trigger:** Designing the next powerlab-mcp resource family — `apps://*` (installed apps + their state) and `docker://logs/{container}` (per-container logs). The maintainer flagged a key future constraint while reviewing the options: **PowerLab may migrate from SQLite to PostgreSQL in a future release.** That single fact tips a previously balanced trade-off decisively toward HTTP proxying.

## Context

powerlab-mcp's Phase 2 surface needs to cover the apps the user actually cares about — Plex, Jellyfin, Sonarr, Ollama, the dozens of containers running on a typical PowerLab box. The agent has to be able to answer:

- "Which apps are installed?"
- "Is Plex running?"
- "Show me the last 200 lines of Jellyfin's logs"
- "What's eating CPU on this box right now?"
- "Did the update break anything?"

Today the panel's apps view answers every one of those questions by calling **app-management's HTTP API** (`/v2/app_management/compose/...`). app-management owns the compose lifecycle, the Docker socket, the per-app state, the logs, the stats — every authoritative bit of "what's running on this box" data.

The agent needs the same data. The architectural question is **how MCP gets it**:

| Option | Mechanism | Coupling surface |
|---|---|---|
| **A. SQLite direct** | MCP opens app-management's `.db` file read-only via the `mattn/go-sqlite3` driver | Storage ABI (table layout, column types, query semantics) |
| **B. HTTP proxy** | MCP calls app-management over HTTP via the `coreproxy` pattern from [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md), generalised to resolve `<runtime>/app-management.url` | API contract (versioned `/v2/` endpoints) |

[ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) already established the hybrid pattern for `system://*` resources proxied to core. This ADR extends the same pattern to a second upstream (app-management) and chooses, deliberately, **not** to introduce a third pattern (direct DB).

### What app-management already exposes

A quick walk through `backend/app-management/route/v2/` confirms the API is rich enough to cover everything the MCP surface needs without us touching a single SQL row:

| Handler | Purpose |
|---|---|
| `MyComposeAppList` | manifest of installed apps |
| `MyComposeApp` | detail of one app (compose source + status) |
| `ComposeAppContainers` | running containers for one app |
| `CheckComposeAppHealthByID` | aggregate health |
| `ComposeAppStats` | per-container CPU/RAM/IO |
| `ComposeAppDiskUsage` | per-app disk footprint |
| `ComposeAppLogs` | per-container logs (the `docker logs` data path, already wrapped) |
| `AppStoreList` / `ComposeAppStoreInfoList` | catalog metadata (later Phase) |

Every PowerLab panel page that shows app data goes through these endpoints. They are the single source of truth.

### The PostgreSQL constraint

Maintainer signalled (2026-05-28) that PowerLab may move from SQLite to PostgreSQL in a future release. This is the decisive constraint. With **option A** (SQLite direct):

- powerlab-mcp imports the SQLite driver, hard-binds to the current table layout.
- The future migration becomes **a rewrite of the MCP reader**, not a no-op — and a coordinated rewrite at that, since MCP and app-management have to flip storage in lockstep.
- During the transition window (mixed deployments, rolling upgrades), MCP would have to speak **both** SQLite and PostgreSQL — doubling the test surface for what should have been a non-event.

With **option B** (HTTP proxy):

- The storage engine is invisible to MCP. app-management migrates its persistence; the HTTP response shape doesn't change; MCP never notices.
- A coordinated rollout is unnecessary — app-management can flip first, then MCP, then operators upgrade. No window of incompatibility.
- The same is true for any future backend change: podman instead of Docker, k8s in the homelab dialect, a hosted control plane — as long as app-management's `/v2/` contract holds, MCP keeps working.

## Decision

**powerlab-mcp exposes `apps://*` and `docker://*` as thin HTTP proxies to app-management**, using the same `coreproxy` package introduced in ADR-0044, generalised to resolve multiple service URLs.

### Generalisation of `coreproxy`

Today `coreproxy.Client` is hard-coded to core's `.url` file (`casaos.url` — legacy filename). It becomes a multi-service client:

```go
// (sketch — final API lands with the implementation PR)
type Client struct {
    runtimePath string
    services    map[string]string   // service name → .url filename
    cache       map[string]urlEntry // per-service URL cache
    httpClient  *http.Client
}

// Get proxies to the upstream identified by `service` (e.g. "core", "apps").
// Unknown services return service_unavailable so the agent can pattern-match
// the same way it does for core_unavailable today.
func (c *Client) Get(ctx context.Context, service, path, token string) ([]byte, error)
```

Service-to-URL-file mapping:

| Service ID | `.url` file under RuntimePath | Owner |
|---|---|---|
| `core` | `casaos.url` (kept for compat) | `backend/core` |
| `apps` | `app-management.url` | `backend/app-management` |

The cache, JWT-forward, structured-error semantics, and fallback payload shape are unchanged from ADR-0044. `core_unavailable` becomes the prototype; new code adds `apps_unavailable` (and a future `service_unavailable` umbrella) with identical contract.

### Resources this ADR enables

Listed for clarity — implementation lands in the follow-up PR(s).

| URI | Upstream endpoint | Notes |
|---|---|---|
| `apps://list` | `GET /v2/app_management/compose` | installed-apps manifest |
| `apps://state/{id}` | `GET /v2/app_management/compose/{id}` | per-app detail |
| `apps://state/{id}/containers` | `GET /v2/app_management/compose/{id}/containers` | live containers |
| `apps://state/{id}/health` | `GET /v2/app_management/compose/{id}/health` | aggregate health |
| `apps://state/{id}/stats` | `GET /v2/app_management/compose/{id}/stats` | per-container CPU/RAM/IO |
| `apps://state/{id}/disk` | `GET /v2/app_management/compose/{id}/disk` | per-app disk |
| `apps://schema` | literal JSON | self-describing |
| `docker://logs/{app_id}` | `GET /v2/app_management/compose/{id}/logs` | container logs, proxied through app-management |

### Why `docker://` rides on app-management, not the Docker socket

This is the second non-obvious win the proxy choice delivers. The original Phase 2 plan (PR 4 in the working notes) called for a **separate ADR for Docker socket access** — adding `SupplementaryGroups=docker` to `powerlab-mcp.service` and shipping a Docker client. That whole strand goes away:

- app-management already speaks to Docker (it owns the lifecycle).
- `ComposeAppLogs` wraps `docker logs` correctly, with the right container resolution + scoping to PowerLab-managed apps.
- MCP calls `ComposeAppLogs` over HTTP. **MCP never needs Docker socket access.**

The threat surface of `docker://logs/{id}` is now identical to `apps://state/{id}` — same JWT gate, same audit chain, no new privileged dependency in the MCP unit.

### What stays independent (audit + journal of PowerLab units)

ADR-0044's scoping of "independent vs proxied" is unchanged:

- `audit://*` and `journal://powerlab-*` remain raw reads. They survive every other service being down — exactly when the operator most needs to investigate.
- Everything sysadmin-or-apps that has an upstream becomes a proxy. ADR-0045 just adds a second upstream.

## Consequences

### Wins

1. **PostgreSQL migration becomes a no-op for MCP.** This is the headline. Storage is app-management's concern, not MCP's. When the migration happens, MCP doesn't get a PR.
2. **Single source of truth.** Whatever the panel can show, the agent can read. When app-management learns a new field, MCP sees it automatically (the proxy passes the body verbatim).
3. **Docker socket stays out of MCP.** No `SupplementaryGroups=docker`, no Docker client dependency, no separate threat-model ADR. The `docker://logs` surface is a regular HTTP proxy through app-management.
4. **Pattern reuse.** No new architectural concept. `apps://` is "system://utilization, with a different upstream." Cognitive load is zero for the next maintainer.
5. **Auth chain stays clean.** When JWT forwarding lands (tracked separately), it routes through app-management's own auth — same identity the panel uses.
6. **Smaller PR surface.** Implementing `apps://list` is ~30 lines on top of the generalised `coreproxy`. The plan was 2 PRs (one for apps, one for Docker, each with its own ADR); it collapses to one.

### Costs

1. **app-management must be up for `apps://*` to work.** Mitigation: when app-management is down, the panel apps view is down too — the operator already knows. The agent can still read `journal://powerlab-app-management` to investigate why, and `audit://` to see whether something triggered the failure. The structured `apps_unavailable` payload routes the agent there explicitly.
2. **API contract coupling.** A change in app-management's response shape ripples to MCP. Mitigation: app-management already versions its API (`/v2/`); shape changes have always been considered breaking. The proxy returns the body verbatim, so the agent — not MCP — owns the shape parsing.
3. **One extra HTTP hop per read.** Microseconds on localhost loopback. Negligible.
4. **Coupling to `app-management.url` discovery.** Same brittleness as ADR-0044's `casaos.url` — if `RuntimePath` is misconfigured, proxies fail. Same structured-error payload routes the operator to the fix.

### Operational changes

- `powerlab-mcp.service`'s soft dependency list grows: `Wants=powerlab-app-management.service` (per the same pattern that ADR-0044 added `Wants=powerlab-core.service` in PR #610).
- `mcp.conf` gains no new keys — `RuntimePath` continues to be the single discovery root.
- Packaging: no changes (app-management already publishes its `.url` file at startup; MCP just learns to read it).

### What stays out of scope

- **MCP write tools** that mutate app state (`install_app`, `restart_app`, `uninstall_app`, etc.) — those need a dedicated threat-model ADR; this one is read-only proxying. The path to writes is also a proxy (call app-management's POST/DELETE endpoints) but the agent capability + audit story is a separate decision.
- **`apps://catalog`** — the installable-app catalog from the store. Lands in a follow-up PR once `apps://list` + `apps://state` are dogfooded.
- **`docker://stats` / `docker://exec`** — `stats` lands trivially via `ComposeAppStats` after the implementation PR; `exec` is a separate threat-model ADR (write surface, arbitrary command execution).
- **Cross-service correlation by request_id** — MCP already exposes audit's `request_id`; correlating it across `apps://state` → `journal://app-management` is an agent-side concern, not a new resource.

## Acceptance criteria

- [ ] `coreproxy.Client` generalised to multi-service registration (core + apps minimum); existing `core_unavailable` tests keep passing unchanged.
- [ ] `apps://list`, `apps://state/{id}`, `apps://schema` registered + tested (happy path + apps_unavailable + missing-id).
- [ ] `docker://logs/{app_id}` registered + tested (same shape — JSON body forwarded from `ComposeAppLogs`).
- [ ] `powerlab-mcp.service` gains `Wants=powerlab-app-management.service` (soft dep — MCP boots when it's down).
- [ ] `apps_unavailable` structured payload shape mirrors `core_unavailable`: `{error, detail, body?, fallback}` with the fallback pointing at audit + journal.
- [ ] No Docker socket access added to the MCP unit (deliberate non-acceptance criterion — Docker integration goes through app-management).
- [ ] mcp-server.md updated with the new resource list + the proxy-error pattern.
- [ ] **Pre-cut validation**: the live battery on .142 exercises the SQLite path; the same battery (without changes to MCP) should pass after a future PostgreSQL migration on app-management. That's the future-proof statement.

## Future-proof statement

> **When PowerLab migrates from SQLite to PostgreSQL, powerlab-mcp will not require a single line of change.** The HTTP contract of app-management is the abstraction; storage is an implementation detail it owns. Same applies to any future backend swap — podman, k8s, hosted control planes — as long as the `/v2/` contract holds.

That is the architectural promise this ADR ships.

## References

- [ADR-0034](0034-standalone-observability-mcp-service.md) — original "isolated MCP" framing, amended by ADR-0044 to allow hybrid proxying.
- [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) — hybrid architecture decision that ADR-0045 extends to a second upstream.
- [ADR-0033](0033-audit-system-design.md) — audit chain that stays orthogonal to the resource path.
- [`feedback_mcp_reuse_observability`](../../../memory/feedback_mcp_reuse_observability.md) — internal note locking the reuse-over-reimplement principle that this ADR honours.
