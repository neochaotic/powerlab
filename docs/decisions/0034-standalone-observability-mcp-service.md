# 0034. Standalone observability + MCP service

- **Status:** proposed
- **Date:** 2026-05-14
- **Tracks:** Sprint 17 critical path
- **Builds on:** ADR-0033 (audit middleware), ADR-0026 (slog → journald), Sprint 14 #150 (powerlab-logs CLI)
- **Research:** [mcp-linux-server-landscape-2026.md](../research/mcp-linux-server-landscape-2026.md) — landscape scan that informs the URI scheme + gating decisions below

## Context

Sprint 16 shipped the audit middleware (ADR-0033) and Sprint 14 shipped the `powerlab-logs` CLI (#150 Phase 1). Together they give PowerLab the **raw observability primitives**: per-service request audit (SQLite) + journal aggregation (journalctl wrap) + install-log capture (`/var/log/powerlab/install-*.log`). What's missing is the **surface** that exposes them to operators and — increasingly — to AI agents.

Three user-driven constraints set the design:

1. **Independence.** The observability surface must run even when the rest of PowerLab is broken. Today's flow (Settings → Audit pane served by the SvelteKit UI behind the gateway) is fragile — if the gateway is down, the operator can't see the logs that would explain *why*. Anti-pattern.

2. **Coexistence with the existing UI.** Settings → Audit (Sprint 16 B1f) stays — it's the in-app deep-dive. A new header-level button opens the standalone observability surface in a new tab, on its own port. Two surfaces, one data source.

3. **MCP-first.** This service is also PowerLab's MCP server. The protocol shape ([MCP spec — Resources](https://modelcontextprotocol.io/specification/2025-06-18/server/resources)) maps 1:1 to what the service already exposes: typed resources (audit log, journal, system metrics) and gated tools (restart, prune, backup). Bolting MCP on later would force a refactor; designing MCP-first costs little extra now and gates the broader product vision (see [[project_mcp_linux_connector_vision]]).

The landscape scan (research doc above) confirms there is no existing OSS MCP server that combines (a) full Linux-box surface, (b) MCP-native design, (c) gated actions, (d) embedded audit, (e) standalone runtime, (f) under one binary. SSH wrappers like [ssh-mcp](https://github.com/tufantunc/ssh-mcp) and [linux-mcp-server](https://pypi.org/project/linux-mcp-server/) cover the read-only diagnostic case but require the agent to know shell. Single-purpose servers ([docker-mcp](https://github.com/QuantGeekDev/docker-mcp), [log-mcp](https://github.com/ascii766164696D/log-mcp)) cover one capability at a time. PowerLab has the opportunity to fill the gap.

## Decision

Ship a new standalone binary, working name `powerlab-observability`, that:

1. **Runs independently** of every other PowerLab service. Reads per-service audit DBs read-only, journal logs via journalctl, system metrics via direct `/proc` and `/sys` reads (NOT via the gateway's `/v1/sys/*` proxy). Listens on its own port. Survives gateway restarts, app-management crashes, and HTTPS misconfigurations.

2. **Listens on port 9090 by default** (Prometheus convention — operators recognise it, easy to remember). Configurable via `/etc/powerlab/observability.conf`. Bound to `*:9090` so it's reachable from LAN; auth gates the actual access (see point 4).

3. **Speaks three transports off the same binary:**
   - **HTTP + SSE** for the browser UI (own minimal SPA OR Settings → Audit pane proxies via this surface).
   - **MCP over stdio** (JSON-RPC) for local agent use (Claude Code, Cursor on the same host).
   - **MCP over HTTP** for remote agent use (Claude Desktop on a different machine, hosted agent integrations).

4. **Auth tiers — three levels:**

   | Tier | Who | What |
   |---|---|---|
   | **read** | unauthenticated reachable from loopback only; LAN access requires JWT | observability resources (audit, journal, metrics) |
   | **auth** | valid PowerLab JWT (Bearer per RFC 6750, the L3 fix from Sprint 15 makes this work) | tools that mutate user-visible state (`restart_app`, `prune_orphans`) |
   | **admin** | JWT + explicit `X-Powerlab-Admin-Confirm: true` header | destructive tools (`reset_audit`, `revoke_token`) |

   MCP transport adds: stdio assumes the OS user is trusted (loopback equivalent — read tier free, auth tier reads JWT from `~/.powerlab/token`); HTTP MCP requires a per-client API key issued by the gateway's existing user-service.

5. **Resource URI namespace** — custom schemes per the MCP spec's "free to use additional schemes" allowance:

   | URI pattern | Source | Tier |
   |---|---|---|
   | `audit://<service>/recent?limit=N&since=T` | `<service>/audit.db` (RO attach) | read |
   | `audit://<service>/stats` | same | read |
   | `journal://<unit>?lines=N&since=T&priority=P` | `journalctl -u <unit> -o json` | read |
   | `system://metrics` | `/proc/stat`, `/proc/meminfo`, `/proc/diskstats`, `/proc/net/dev` direct | read |
   | `apps://list` | `<storage>/PowerLabAppData/*/manifest` direct | read |
   | `apps://<id>/status` | `docker inspect <id>` | read |
   | `containers://<id>/logs?lines=N` | `docker logs <id>` | read |
   | `install-logs://recent` | `/var/log/powerlab/install-*.log` | read |

   URI templates per [RFC 6570](https://datatracker.ietf.org/doc/html/rfc6570) for parameterised forms — clients construct valid URIs from server-published templates instead of guessing.

6. **Tool model — gated actions:**

   | Tool | Action | Tier |
   |---|---|---|
   | `restart_app(id)` | `docker compose restart` | auth |
   | `prune_orphans(app)` | the orphan-cleanup fix tracked separately | auth |
   | `check_disk_free()` | `df` wrap | read |
   | `read_file(path)` | bounded `cat` (size cap, root-relative deny-list) | read |
   | `journal_search(query)` | `journalctl -g <pattern>` | read |
   | `reset_audit(service)` | drops audit DB rows | admin |

   Each tool's input/output JSON schema published via the MCP `tools/list` endpoint so clients (Claude, Cursor) auto-discover.

7. **`powerlab-logs` CLI becomes a client** of this service. Replaces the current direct `journalctl` invocation with HTTP calls to `:9090`. Single source of truth — same query, same auth gating, same audit trail of who-asked-what. CLI keeps its current UX (`powerlab-logs journal --service core`) but becomes thinner.

8. **UI button placement:** new icon in the header (alongside theme toggle), glyph TBD but think `Activity` / `Terminal` / `ScrollText`. Click → `window.open('http://<host>:9090/', '_blank')`. The Settings → Audit pane (Sprint 16 B1f) stays — it's the in-app deep-dive when the user is already in Settings. Two surfaces, deliberate.

9. **What we DO NOT build in this slice:**
   - **Not** an MCP gateway (survey [MCPJungle](https://github.com/mcpjungle/MCPJungle), [MintMCP](https://www.mintmcp.com/blog/mcp-gateways-self-hosted-deployments) listings first — gateway pattern is solved by others).
   - **Not** an MCP App `ui://` resource yet (the [draft spec](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/draft/apps.mdx) is tracked; revisit when stable — would let the AuditPane render inside Claude Desktop without its own SPA).
   - **Not** publishing to the [official MCP Registry](https://registry.modelcontextprotocol.io/) yet — gate on production stability proof first.

## Rationale

- **Independence wins debugging.** Every "I can't see what's broken" report this project has chased could have been answered faster if observability ran alone. The audit middleware foundation (Sprint 16) put the data on disk; this ADR exposes it without making the operator hostage to the rest of the stack.

- **MCP-first costs little extra.** The same HTTP+SSE handlers serve the browser UI; the MCP transport is a thin JSON-RPC adapter over the same resource methods. Doing it later means refactoring the resource shape; doing it now means the resource shape IS the MCP shape.

- **Custom URI schemes are spec-blessed.** Per the [MCP Resources spec](https://modelcontextprotocol.io/specification/2025-06-18/server/resources), implementations are explicitly free to use custom schemes — observed in the wild (e.g. `db://customers/recent`). Reserving `file://` for actual filesystem reads keeps the semantic boundary clean.

- **Three-tier action gating maps cleanly to today's auth.** The PowerLab JWT (with the Sprint 15 L3 Bearer-prefix fix) handles the auth tier. Loopback gating handles the read tier. Admin tier needs only an explicit confirm header — no new auth machinery.

- **Port 9090 is operator muscle memory.** Prometheus operators know it; nothing on a typical PowerLab host conflicts. Configurable for the rare conflict.

- **Pivot-friendly without committing to the pivot.** Designing the URI namespace homelab-agnostic (`audit://`, `system://`, etc — nothing PowerLab-specific in the URIs) means the same binary could be repackaged later for the broader Linux-MCP-connector vision (see [[project_mcp_linux_connector_vision]]) without protocol changes.

## Alternatives considered

- **Stay with the gateway-mounted Audit pane only (Sprint 16 status quo).**
  Rejected: violates the independence constraint. When operators need observability most (something is broken), the gateway is often what's broken.

- **Bolt MCP onto the existing gateway.**
  Rejected: the gateway is the front-door reverse proxy, not a tool-server. Mixing concerns increases blast radius for security mistakes (a bad MCP tool could compromise reverse-proxy state). Separation of concerns + independence both win with a separate binary.

- **Use `file://` for everything.**
  Rejected: the MCP spec uses `file://` for "filesystem-shaped" resources. Audit records are not files; serving them as `file://audit/recent` confuses agents and conflates with actual filesystem access we'd expose for `read_file(path)`.

- **Make the existing `powerlab-logs` CLI a server too (process-per-call).**
  Rejected: CLI's current model is one-shot read. MCP clients expect a long-lived server with stateful transport (especially MCP Apps + SSE). Forking a process per request kills latency for agentic workloads.

- **Adopt an existing OSS MCP server (e.g. linux-mcp-server, homelab_mcp).**
  Rejected after the landscape scan: closest competitors are SSH-shell wrappers or read-only diagnostic tools. None match the (independent + audit-embedded + gated-actions + MCP-native + single-binary) shape. Forking would mean rewriting what we already have in PowerLab's audit middleware. Building fresh is cheaper than retrofitting.

## Consequences

**Positive:**
- Operator can debug PowerLab even when PowerLab itself is partly broken.
- Single binary, single port, single auth surface for "everything observability + MCP."
- Future MCP-for-Linux pivot is unblocked at the protocol level — repackaging is a marketing/binary-rename exercise, not a refactor.
- The audit middleware foundation (ADR-0033, Sprint 16) gets a consumer beyond the in-app pane — actual return on the recorder/store/retention investment.

**Neutral:**
- One more `systemd` unit on the host (`powerlab-observability.service`).
- One more port to document (`:9090`) and configure firewalls for.
- Adds a dependency direction: `powerlab-logs` CLI → standalone service → audit DBs (read-only).

**Negative (controlled):**
- Two surfaces for the same data (Settings → Audit + standalone `:9090`). Coexistence is deliberate but introduces "where do I look" UX cost. Mitigation: consistent terminology + the header button visibly labelled.
- The MCP API surface becomes a contract that's expensive to change once external agents wire to it. Pin the resource URI patterns + tool schemas in this ADR; future changes go through ADR amendment.
- Building MCP-native means reading the spec carefully and revisiting drafts (MCP Apps `ui://`, registry conventions). Time cost in Sprint 17 vs. shipping bare HTTP first.

## Open questions

1. **Binary name.** `powerlab-observability` is descriptive but long. Alternatives: `powerlab-mcp` (MCP-forward), `powerlab-agent` (collides with monitoring "agents"), `powerlab-watch`. Defer until Sprint 17 spike — the binary path appears in systemd unit, install logs, CLI, and any future marketing copy.

2. **Auth for remote MCP — API key vs mTLS vs OAuth?** API key is the path-of-least-resistance and matches GitHub MCP / Azure MCP; mTLS is more correct but UX-heavier; OAuth requires an IdP. Threat-model in Sprint 17 before locking.

3. **Service vs sidecar deployment model.** Single instance per host (decided here) vs one per service vs gateway-as-aggregator. Single instance per host is simpler and matches the independence intent — but if someone runs multiple PowerLab instances on one box (rare), naming + port collisions matter.

4. **MCP Apps `ui://` adoption timeline.** The [draft spec](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/draft/apps.mdx) is moving fast. If it lands GA before the standalone service ships, AuditPane being an MCP App is a much bigger leverage point. Pin a checkpoint: re-evaluate at Sprint 17 mid-point.

5. **Read-only attach to other services' audit DBs — locking semantics.** SQLite WAL handles concurrent read + single-writer cleanly, but cross-process attach across services hasn't been integration-tested. Validate in Sprint 17 spike.

## Acceptance for Sprint 17 implementation

- [ ] New binary `powerlab-observability` (name TBD per Q1) cross-compiles CGO-free, ships in `scripts/package-linux.sh`.
- [ ] Listens on configurable port (default `:9090`).
- [ ] Read-only attaches to per-service `audit.db` files; aggregates `audit://` resource queries across services without touching the writers.
- [ ] HTTP + SSE transport for browser; MCP over stdio for local agent; MCP over HTTP for remote.
- [ ] Three auth tiers enforced + audited (recorder built into this service too — eat our own dog food).
- [ ] Resource URI templates published via MCP `resources/list`.
- [ ] Tool schemas published via MCP `tools/list`.
- [ ] Unit tests for the resource handlers + tool handlers + auth tier enforcement.
- [ ] Integration test (`//go:build integration`) covering an MCP client → resource fetch → tool call cycle.
- [ ] `powerlab-logs` CLI updated to call the service instead of `journalctl` directly (still works headless via stdio MCP if the HTTP port is unreachable).
- [ ] UI: header button mounted, glyph chosen, opens `:9090` in new tab.
- [ ] Settings → Audit pane keeps working (both surfaces coexist).
- [ ] Live SSH+browser smoke on .142: standalone service running independently of gateway, MCP query works from a local Claude Code session.

## References

- [Model Context Protocol — Resources spec (2025-06-18)](https://modelcontextprotocol.io/specification/2025-06-18/server/resources)
- [Official MCP Registry](https://registry.modelcontextprotocol.io/)
- [MCP Apps draft (`ui://`)](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/draft/apps.mdx)
- [modelcontextprotocol/servers — reference implementations](https://github.com/modelcontextprotocol/servers)
- [Filesystem MCP Server (official reference)](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem)
- [linux-mcp-server (closest competitor in spirit)](https://pypi.org/project/linux-mcp-server/)
- [ssh-mcp](https://github.com/tufantunc/ssh-mcp), [homelab_mcp](https://mcpservers.org/servers/washyu/mcp_python_server) — alternatives surveyed
- [docker-mcp](https://github.com/QuantGeekDev/docker-mcp), [log-mcp](https://github.com/ascii766164696D/log-mcp) — single-purpose patterns
- [Docker AI Agent + MCP Server](https://www.docker.com/blog/simplify-ai-development-with-the-model-context-protocol-and-docker/), [Azure MCP 2.0](https://devblogs.microsoft.com/azure-sdk/announcing-azure-mcp-server-2-0-stable-release/), [GitHub MCP](https://github.com/github/github-mcp-server/releases) — first-party patterns from incumbents
- [MintMCP — Best MCP Gateways for Self-Hosted Deployments 2026](https://www.mintmcp.com/blog/mcp-gateways-self-hosted-deployments)
- [MCPJungle — Self-hosted MCP Gateway](https://github.com/mcpjungle/MCPJungle)
- [The MCP Server Ecosystem in 2026](https://dev.to/sahil_kat/the-mcp-server-ecosystem-in-2026-integration-layer-for-ai-agents-2mln)
- [RFC 6570 — URI Templates](https://datatracker.ietf.org/doc/html/rfc6570)
- [PowerLab landscape research note](../research/mcp-linux-server-landscape-2026.md)
- ADR-0033 (audit middleware design) — provides the data this ADR exposes
- ADR-0026 (slog → journald) — provides journal data this ADR proxies
- Sprint 14 #150 (powerlab-logs CLI) — becomes a client of this service
