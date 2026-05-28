# 0034. Standalone observability + MCP service (`powerlab-mcp`)

- **Status:** accepted
- **Date:** 2026-05-14 (proposed) · 2026-05-27 (revised + accepted)
- **Tracks:** `powerlab-mcp` Foundation MVP — release-keyed (v0.7.x), MVP-first
- **Builds on:** ADR-0033 (audit middleware), **ADR-0035 (audit storage = JSONL)**, ADR-0026 (slog → journald), ADR-0015 (correlation-id), ADR-0027 (Uber FX), #150 (powerlab-logs CLI)
- **Research:** [mcp-linux-server-landscape-2026.md](../research/mcp-linux-server-landscape-2026.md)

## Reconciliation (2026-05-27)

This ADR sat `proposed` since 2026-05-14 and was never implemented. Before accepting it, four premises were corrected against the current codebase. Acceptance = merge of this revision.

1. **Binary name resolved → `powerlab-mcp`** (Open question Q1). Sells the MCP positioning and is descriptive. The name appears in the systemd unit (`powerlab-mcp.service`), config path (`/etc/powerlab/mcp.conf`), and install logs.
2. **Audit source is JSONL, not SQLite.** ADR-0035 (accepted, shipped PR #370 / v0.6.12) migrated audit storage from SQLite to a JSONL file + in-memory ring buffer (`/var/log/powerlab/audit.jsonl`, `audit.Record` wire type). The `audit://*` resources **tail the JSONL read-only** — there is no SQLite attach. This is what ADR-0035 explicitly unblocked; the old "RO attach SQLite / WAL-checkpoint" concern (former Q5) is moot.
3. **Transports: HTTP+SSE first; stdio + MCP-over-HTTP deferred.** stdio only matters when the agent runs on the same host; HTTP+SSE covers the dogfood and LAN cases. Starting with one transport family cuts 2–3 days and keeps the test surface small. The others are added on demand.
4. **Release-keyed, not "Sprint 17."** Sprint 17 already happened (it shipped ADR-0035, v0.6.12) and sprint framing has ended; work is sequenced as v0.7.x release increments, MVP-first.

JWT is **single-issuer**: the MCP validates tokens from the existing user-service via `backend/common/utils/jwt.Validate` — no parallel token store (avoids the duplicate rotation/revocation surface and the raw-fetch-bypasses-JWT bug class).

## Context

ADR-0033 shipped the audit middleware and #150 shipped the `powerlab-logs` CLI. Together they give PowerLab the **raw observability primitives**: per-service request audit (**JSONL**, ADR-0035) + journal aggregation (journalctl wrap) + install-log capture (`/var/log/powerlab/install-*.log`). What's missing is the **surface** that exposes them to operators and — increasingly — to AI agents.

Three user-driven constraints set the design:

1. **Independence.** The observability surface must run even when the rest of PowerLab is broken. Today's flow (Settings → Audit pane served by the SvelteKit UI behind the gateway) is fragile — if the gateway is down, the operator can't see the logs that would explain *why*. Anti-pattern.

2. **Coexistence with the existing UI.** Settings → Audit stays — it's the in-app deep-dive. A new header-level button opens the standalone surface in a new tab, on its own port. Two surfaces, one data source.

3. **MCP-first.** This service is also PowerLab's MCP server. The protocol shape ([MCP spec — Resources](https://modelcontextprotocol.io/specification/2025-06-18/server/resources)) maps 1:1 to what the service already exposes: typed resources (audit log, journal, system metrics) and gated tools (restart, prune). Bolting MCP on later would force a refactor; designing MCP-first costs little extra now and gates the broader product vision (see [[project_mcp_linux_connector_vision]]).

The landscape scan confirms no existing OSS MCP server combines (a) full Linux-box surface, (b) MCP-native design, (c) gated actions, (d) embedded audit, (e) standalone runtime, (f) one binary. The design lens is now **enterprise** — "would enterprise IT accept this in production?" — which tightens the host-ops scope (below).

## Decision

Ship a new standalone binary **`powerlab-mcp`** that:

1. **Runs independently** of every other PowerLab service. Reads per-service JSONL audit files read-only (tail), journal logs via journalctl, system metrics via direct `/proc` and `/sys` reads (NOT via the gateway's `/v1/sys/*` proxy). Listens on its own port. Survives gateway restarts, app-management crashes, and HTTPS misconfigurations.

2. **Listens on port 9090 by default** (Prometheus convention). Configurable via `/etc/powerlab/mcp.conf`. Bound to `*:9090` so it's reachable from LAN; auth gates the actual access (point 4).

3. **Transports (phased):**
   - **HTTP + SSE** — browser UI + remote MCP. **Ships in the Foundation MVP.**
   - **MCP over stdio** (local agent, Claude Code/Cursor on the same host) — **deferred**, add on demand.
   - **MCP over HTTP** for remote agents — folds into the HTTP transport; remote *pairing* is a later block.

   Built on the **official `modelcontextprotocol/go-sdk`** (v1.x, GA, maintained in collaboration with Google, tracks the current spec; provides `AddResource` / `AddResourceTemplate`, `AddTool`, and a `StreamableHTTPHandler`). A thin PowerLab wrapper adds auth tiers, audit, and validation. Tool/resource schemas are **code-first** via struct tags — not a YAML→Go generator (and our `oapi-codegen` is the deprecated `deepmap` v1). _(Amended 2026-05-28: the first cut shipped on the third-party `mark3labs/mcp-go` v0.54.x; once the official SDK was found to be GA with full feature parity, the module was migrated to it while the surface was still small — see Q6.)_

4. **Auth tiers — three levels, single JWT issuer (user-service):**

   | Tier | Who | What |
   |---|---|---|
   | **read** | loopback free; LAN requires a valid user-service JWT | observability resources (audit, journal, metrics) |
   | **auth** | valid PowerLab JWT (Bearer per RFC 6750) | tools that mutate user-visible state (`restart_app`, `prune_orphans`) |
   | **admin** | JWT + explicit `X-Powerlab-Admin-Confirm: true` header | destructive tools (`reset_audit`) |

   Reuses `backend/common/utils/jwt.Validate` + the loopback-skip pattern from `jwt.HTTPJWT`. MCP over stdio (when added) treats the OS user as trusted (loopback-equivalent); MCP over HTTP from LAN requires the JWT, issued through the existing user-service via a pairing step (later block).

5. **Resource URI namespace** — custom schemes per the MCP spec's "free to use additional schemes" allowance:

   | URI pattern | Source | Tier |
   |---|---|---|
   | `audit://<service>/recent?limit=N&since=T` | `<service>` JSONL audit file (RO tail) | read |
   | `audit://<service>/stats` | JSONL ring buffer | read |
   | `audit://action/<correlation_id>` | JSONL filtered by `request_id` | read |
   | `journal://<unit>?lines=N&since=T&priority=P` | `journalctl -u <unit> -o json` | read |
   | `journal://schema` | hardcoded (fields + ADR-0013 error codes) | read |
   | `system://metrics` | `/proc/stat`, `/proc/meminfo`, `/proc/diskstats`, `/proc/net/dev` direct | read |
   | `install-logs://recent` | `/var/log/powerlab/install-*.log` | read |

   URI templates per [RFC 6570](https://datatracker.ietf.org/doc/html/rfc6570). Product-surface resources (`catalog://`, `apps://`, `containers://`) land in a later block via local HTTP to the gateway.

6. **Tool model — gated actions (Foundation MVP set):**

   | Tool | Action | Tier |
   |---|---|---|
   | `restart_app(id)` | `docker compose restart` (via gateway HTTP) | auth |
   | `prune_orphans(app)` | orphan-cleanup | auth |
   | `check_disk_free()` | `df` wrap | read |
   | `read_file(path)` | bounded read (size cap + deny-list) | read |
   | `journal_search(query)` | `journalctl -g <pattern>` | read |
   | `reset_audit(service)` | truncates the service's audit JSONL | admin |

   Each tool's input/output JSON schema is published via MCP `tools/list`. **Host-ops scope (enterprise lens):** OS maintenance tools land in a later block and are **PowerLab-scoped only** — `cleanup_powerlab_logs`, `systemctl_{status,restart}` (unit allowlist), `disk_usage`, `prune_docker`, `clear_app_cache`. Arbitrary `kill_process` and generic `list_directory` are **out** (CVE-class surface with no enterprise justification; `stop_app` covers the legitimate "stop a container" case).

7. **`powerlab-logs` CLI becomes a client** of this service — HTTP calls to `:9090` instead of direct `journalctl`. Single source of truth: same query, same auth gating, same audit trail. CLI keeps its UX, becomes thinner.

8. **UI button placement:** new header icon (alongside theme toggle) → opens `:9090` in a new tab. The Settings → Audit pane stays — deliberate two-surface coexistence.

9. **What we DO NOT build:**
   - **Not** an MCP gateway (solved by others — MCPJungle, MintMCP).
   - **Not** an MCP App `ui://` resource yet (draft spec; revisit when stable).
   - **Not** publishing to the [MCP Registry](https://registry.modelcontextprotocol.io/) yet — gate on a **dogfood week on the maintainer's box first**, then publish. The MCP-for-Linux positioning is not committed publicly until the features prove out.
   - **Not** `create_custom_app` / `update_app` / arbitrary host-ops — separate ADRs, image-trust unresolved.

## Rationale

- **Independence wins debugging.** Observability running alone answers "I can't see what's broken" even when the gateway is the broken thing. ADR-0033/0035 put the data on disk as greppable JSONL; this ADR exposes it without making the operator hostage to the rest of the stack — and JSONL means the surface just tails a file, no second DB handle.

- **MCP-first costs little extra.** The same HTTP+SSE handlers serve the browser UI; the MCP transport is a thin adapter over the same resource methods (mostly our existing middleware chain — audit, correlation-id, slog — applied to a new transport). The resource shape IS the MCP shape.

- **Custom URI schemes are spec-blessed.** Reserving `file://` for actual filesystem reads (`read_file`) keeps the semantic boundary clean.

- **Three-tier gating maps cleanly to today's auth.** `jwt.Validate` handles the auth tier; loopback-skip handles read; admin needs only a confirm header. No new auth machinery, one issuer.

- **Pivot-friendly without committing.** Homelab-agnostic URIs (`audit://`, `system://`) mean the binary could be repackaged for the broader Linux-MCP-connector vision without protocol changes — but the public commitment waits for the dogfood proof.

## Alternatives considered

- **Gateway-mounted Audit pane only (status quo).** Rejected: violates independence — when observability is needed most, the gateway is often what's broken.
- **Bolt MCP onto the gateway.** Rejected: the gateway is the front-door reverse proxy, not a tool-server. Mixing concerns increases blast radius for security mistakes. Separate binary wins on separation + independence.
- **`file://` for everything.** Rejected: audit records are not files; conflates with the actual filesystem access `read_file` exposes.
- **`powerlab-logs` CLI as a process-per-call server.** Rejected: MCP clients expect a long-lived stateful server (SSE); fork-per-request kills latency for agentic workloads.
- **Adopt an existing OSS MCP server.** Rejected after the landscape scan: closest competitors are SSH-shell wrappers or read-only diagnostics; none match the (independent + audit-embedded + gated + MCP-native + single-binary) shape. Building fresh on our existing audit/JWT infra is cheaper than retrofitting.

## Consequences

**Positive:**
- Operator can debug PowerLab even when PowerLab is partly broken.
- Single binary, single port, single auth surface for "everything observability + MCP."
- Future MCP-for-Linux pivot unblocked at the protocol level.
- The audit foundation (ADR-0033/0035) gets a consumer beyond the in-app pane.

**Neutral:**
- One more `systemd` unit (`powerlab-mcp.service`) and one more port (`:9090`) to document/firewall.
- Dependency direction: `powerlab-logs` CLI → `powerlab-mcp` → JSONL audit files (read-only).

**Negative (controlled):**
- Two surfaces for the same data (Settings → Audit + standalone `:9090`). Mitigation: consistent terminology + a labelled header button.
- The MCP API surface becomes a contract that's expensive to change once external agents wire to it. Mitigation: pin resource URI patterns + tool schemas here; **versioning is introduced at registry-publication time** (not before — pre-publication there is no external consumer to protect); changes go through ADR amendment.

## Open questions

1. ~~Binary name.~~ **Resolved → `powerlab-mcp`** (Reconciliation §1).
2. **Remote MCP auth — pairing UX.** A `powerlab pair` CLI mints a short-lived user-service token shown as a QR/string to paste into the agent config; longer-lived token after the first handshake. Threat-model before locking. (Not in the Foundation MVP — loopback/LAN dogfood first.)
3. **Deployment model.** Single instance per host (decided). Multiple PowerLab instances on one box (rare) would need port/name disambiguation.
4. **MCP Apps `ui://` timeline.** Re-evaluate when the draft spec stabilizes.
5. ~~SQLite cross-process attach locking.~~ **Moot** — audit is JSONL (ADR-0035); the surface tails the file.
6. ~~Go MCP SDK.~~ **Resolved → the official `modelcontextprotocol/go-sdk`** (v1.x GA). The initial spike picked `mark3labs/mcp-go` (most popular) and missed the official SDK; on review the official one was GA, spec-current, and feature-complete (resources, resource templates, streamable HTTP, typed tools), so the module migrated to it while only `system://` + `journal://` existed (cheapest moment). Resource-subscription support (for later install-log streaming) to be confirmed against the official SDK when that block lands.

## Acceptance — Foundation MVP (release-keyed, v0.7.x)

- [ ] New binary `powerlab-mcp` cross-compiles CGO-free (amd64 + arm64), ships in the Linux packaging script.
- [ ] Listens on a configurable port (default `:9090`); config at `/etc/powerlab/mcp.conf`; `powerlab-mcp.service` systemd unit.
- [ ] **Tails per-service JSONL audit files read-only**; serves `audit://` resources without touching the writers.
- [ ] HTTP + SSE transport for browser + remote MCP (stdio deferred).
- [ ] Three auth tiers enforced + audited (recorder built into this service too — dogfood), reusing `jwt.Validate`.
- [ ] Resource URI templates published via MCP `resources/list`; tool schemas via `tools/list` (code-first via struct tags).
- [ ] Resources: `system://*`, `journal://*`, `audit://*`, `install-logs://*`.
- [ ] Tools: `restart_app`, `prune_orphans`, `journal_search`, `read_file`, `check_disk_free`, `reset_audit`.
- [ ] Unit tests for resource handlers + tool handlers + auth-tier enforcement; integration test (`//go:build integration`) covering an MCP client → resource fetch → tool call cycle.
- [ ] `powerlab-logs` CLI calls the service instead of `journalctl` directly.
- [ ] UI: header button mounted, opens `:9090` in a new tab. Settings → Audit pane keeps working.
- [ ] Live SSH+browser smoke on `.142`: service running independently of the gateway; MCP query works from a local Claude Code session.

## References

- [MCP — Resources spec (2025-06-18)](https://modelcontextprotocol.io/specification/2025-06-18/server/resources)
- [MCP Registry](https://registry.modelcontextprotocol.io/) · [MCP Apps draft (`ui://`)](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/draft/apps.mdx) · [reference servers](https://github.com/modelcontextprotocol/servers)
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) (official, in use) · [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) (initial cut, superseded) · [invopop/jsonschema](https://github.com/invopop/jsonschema)
- [RFC 6570 — URI Templates](https://datatracker.ietf.org/doc/html/rfc6570) · [RFC 6750 — Bearer tokens](https://datatracker.ietf.org/doc/html/rfc6750)
- ADR-0033 (audit middleware) · **ADR-0035 (audit JSONL — provides the data this ADR tails)** · ADR-0026 (slog → journald) · ADR-0015 (correlation-id) · ADR-0027 (Uber FX) · #150 (powerlab-logs CLI)
- [PowerLab landscape research note](../research/mcp-linux-server-landscape-2026.md)
