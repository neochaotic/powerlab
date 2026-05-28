# 0046 — MCP tool curation strategy: curated tools first, with explicit escape hatches

- **Status:** accepted
- **Date:** 2026-05-28
- **Trigger:** With Phase 2 read-only resources shipped (ADR-0044 + ADR-0045), the next decision for powerlab-mcp is **how to expose PowerLab's write surface to agents**: install/restart/uninstall apps, prune orphans, journal search, disk checks, etc. The MCP spec offers two primitives (`resources` for read-only data, `tools` for actions with side-effects) plus a third community pattern (meta-prompt / system context dumping). The choice is architectural — it shapes how agents discover capability, how the threat model surfaces, and how PowerLab tracks the rapidly-evolving MCP ecosystem.

## Context

### Three patterns the ecosystem is converging on

| Pattern | What the agent does | Where it shines | Where it breaks |
|---|---|---|---|
| **Resources only** (today) | Reads `docs://api/<service>`, parses OpenAPI, builds HTTP requests itself | Discovery, rarely-used endpoints | Side-effects implicit (agent could call destructive HTTP without explicit acknowledgment); context-window hungry on large specs |
| **MCP tools** | Calls typed tools via `tools/list` + `tools/call` — MCP server translates to HTTP | Explicit side-effects (MCP distinguishes read-only resources from action tools); typed schema; lean per-call context cost | Each tool must be curated and maintained; auto-gen of 100+ tools from OpenAPI produces noise |
| **Meta-prompt / system context** | Spec is dumped into the agent's initial context window | Trivial to ship; useful for tiny APIs | Anti-pattern at scale — burns context with endpoints the agent may never use; the model "loses" detail in long prompts |

Anthropic's MCP spec explicitly frames **tools = actions with side-effects** and **resources = read-only data**. PowerLab's `docs://api/<service>` falls into a grey area (it's *data about* potential actions). The 2026 community consensus, visible across `modelcontextprotocol/servers` (100+ official + community implementations), Block / Cloudflare / Stripe engineering posts, and the Anthropic Builder's Day sessions, is a **hybrid**: curate the top tools, expose the OpenAPI spec as a resource for discovery of rarely-used endpoints, never paste full specs into system prompts.

### What PowerLab already has

- `docs://api` manifest + `docs://api/<service>` resources (per [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) — the Scalar-equivalent for agents). The agent can discover the entire PowerLab API surface today.
- A complete read-only surface (`system://`, `journal://`, `audit://`, `apps://`, `docker://logs`).
- Zero MCP tools registered. The agent can read but cannot act.

### What PowerLab does *not* have

- No RBAC (every authenticated user is admin — see [ADR-0034 amendment](0034-standalone-observability-mcp-service.md)). Every tool, when added, runs at the same privilege level. Backlog [#603](https://github.com/neochaotic/powerlab/issues/603).
- No JWT forwarding from MCP to upstream (MCP-to-upstream calls today rely on the upstream's loopback skip; the upstream's audit trail records "loopback" instead of the agent's user). Tracked.
- No confirmation flow for destructive actions in the panel UI today. If we add `install_app`, the panel will need a "Pending agent action" surface for human approval — otherwise the agent's tool is purely autonomous.

These three gaps shape what we *can* safely expose as tools today, vs. what waits for the prerequisite work.

## Decision

powerlab-mcp adopts a **curated-tools-first** strategy for write actions, with **explicit escape hatches** for the patterns the community has not converged on yet.

### 1. Curate the tools

Hand-write the MCP tool registrations for the **5–10 most useful PowerLab actions**, with descriptions tuned for an LLM consumer and JSON Schema validation on inputs. The first batch (subject to refinement during implementation):

| Tool | What it does | Backing endpoint | Side-effect class |
|---|---|---|---|
| `journal_search` | Search PowerLab service journals by pattern + time range | `journalctl -u powerlab-{unit} -g {pattern} --since={t}` | Read (curated for ergonomics) |
| `check_disk_free` | Quick available-bytes check at a path | `core::/v1/sys/disk` (filter to one mount) | Read |
| `restart_app` | Restart all containers of an installed app | `app-management::/v2/app_management/compose/{id}/status` | **Destructive** — restart cascade |
| `prune_orphans` | Clean up orphan containers under one app | `app-management` orphan-cleanup endpoint | **Destructive** — deletes containers |
| `install_app` | Install a custom Docker Compose app | `app-management::/v2/app_management/compose` POST | **Destructive** — adds containers + storage |

Each tool's input schema is hand-written (not OpenAPI auto-gen). Descriptions are tuned for the LLM — short, action-oriented, including the *side-effect class* in the description so the model surfaces it to the user.

### 2. Keep `docs://api` as the discovery escape hatch

The OpenAPI specs stay published as resources. For any PowerLab endpoint *not* exposed as a curated tool, an agent can:
1. Read `docs://api` to see what's available.
2. Read `docs://api/<service>` for the OpenAPI YAML.
3. Tell the user "here's what I'd call" — the user copy-pastes into curl or invokes via the panel.

This preserves the "agent knows the full API surface" benefit without bloating `tools/list` with 100+ rarely-used endpoints.

### 3. Reserve flexibility for emerging patterns

The MCP ecosystem is moving fast. Patterns we explicitly *do not lock against* and may adopt later if they prove robust:

- **Meta-prompt for small contexts.** When packaging powerlab-mcp for an embedded use case (a tiny CLI, a single-purpose agent), pasting a compact tool summary into the system prompt may beat a full `tools/list` roundtrip. We reserve the option.
- **OpenAPI → tool auto-generation.** Several community packages auto-emit MCP tools from OpenAPI specs. They produce too much noise today (every CRUD endpoint becomes a tool). When a community tool emerges that supports *filtering by tags / curation hints in the spec*, we may revisit.
- **MCP prompts / sampling primitives.** The MCP spec defines `prompts` and `sampling` primitives we haven't used yet. They may turn out to be the right vehicle for guided-workflow tooling (e.g., "install Plex with Nvidia transcoding" as a prompt template rather than a tool).
- **Resource sub-protocols.** If the spec evolves to standardise "active resource" semantics (a resource read that subscribes to changes), we may use that instead of polling-via-tools for status changes.

Lock-in to one pattern would be premature. The decision in this ADR is: **start with curated tools because that's the path most validated by the ecosystem today**, with explicit acknowledgment that the substrate is moving.

### 4. The order in which tools land

To avoid shipping a destructive tool before the supporting infrastructure (RBAC, confirmation flow) is ready:

1. **Read tools first** (`journal_search`, `check_disk_free`) — no side-effects, dogfood the tool-call pattern + agent UX without risk.
2. **Stateful-but-reversible tools next** (`restart_app`) — side-effects are real but bounded (containers come back up); the audit trail captures who-asked-when.
3. **Net-new-resource tools last** (`install_app`, `prune_orphans`) — gated on:
   - A panel UI surface showing "agent-pending action" with an explicit human approve button, **OR**
   - An explicit `--unattended` operator opt-in in `mcp.conf` for autonomous-agent deployments (homelab dogfood) with a documented threat-model warning.

`install_app` specifically requires a custom-compose validation pass (no `privileged: true`, no `/var/run/docker.sock` bind mount, no host network without an explicit flag) — this lands as a separate component (`mcp-compose-validator`) that the install_app tool calls before forwarding to app-management.

### 5. Audit + correlation

Every MCP tool call produces an audit record with:
- The MCP session id (which agent)
- The tool name + arguments (subject to PII review for the few that could carry sensitive data)
- The correlation id (X-Request-Id), so `audit://action/{correlation_id}` aggregates the agent's action + the cascade of upstream calls it triggered

This makes "what did the agent do?" answerable end-to-end via the existing audit:// surface — no new resource needed.

## Consequences

### Wins

1. **Side-effects are explicit at the MCP protocol level.** Tools are categorised by MCP as side-effecting; `resources/read` is not. Anthropic's clients (Claude Desktop, Code) surface this distinction to the user — "Claude wants to use the tool restart_app" is a different UX from "Claude wants to read system://utilization."
2. **Threat model lands per-tool.** Each tool gets its own ADR / threat-review checklist (see the per-tool acceptance criteria below). No big-bang "expose the entire API" decision.
3. **Discovery survives.** `docs://api` keeps working — agents that need a rare endpoint find it; the tool list stays lean.
4. **Ecosystem-tracking is cheap.** When a better pattern emerges (true OpenAPI-to-tools auto-gen with curation, prompts, active resources), we add it alongside the curated tools rather than replacing them. The escape hatches in this ADR are explicit.

### Costs

1. **Curation is manual work.** ~5–10 tool registrations, each with hand-tuned descriptions + input schemas. Estimated effort: ~1 day of focused work per release, sustained.
2. **Coupling to the MCP spec at this point in time.** If the MCP spec evolves to deprecate `tools` or reshapes the side-effect contract, we have rework. Mitigation: the tools are thin wrappers over coreproxy + app-management — the upstream HTTP calls outlive the MCP wire format.
3. **RBAC backpressure.** With every user hardcoded `admin`, a curated destructive tool (`install_app`) is effectively root-level. Mitigation: gate the destructive tools behind the panel-side approval flow OR an explicit mcp.conf opt-in until RBAC ships ([#603](https://github.com/neochaotic/powerlab/issues/603)).
4. **No upstream identity yet.** Until JWT forwarding lands, the audit trail at app-management records "loopback" for the call, not the agent's user. The MCP-side audit chain captures the agent identity, but cross-service correlation is partial. Tracked separately.

### What this ADR does not yet decide

- **Specific tool input schemas.** Per-tool threat-model + schema review lands per-tool, not in this ADR.
- **The panel-side "pending agent action" UI.** Separate frontend ADR if/when it lands.
- **Whether `install_app` requires a separate signed-compose model.** A custom-app threat-model review may decide that signed compose templates from a known issuer are required for the autonomous-agent path. Out of scope here.
- **MCP prompts + sampling primitives.** Reserved as escape hatches; no implementation commitment yet.

## Acceptance criteria

Per this ADR:

- [ ] First batch of curated tools (read-only: `journal_search`, `check_disk_free`) shipped + integration-tested.
- [ ] `restart_app` shipped with an audit-trail correlation test (the action + the cascade are findable via `audit://action/{id}`).
- [ ] `prune_orphans` + `install_app` gated on the approval-flow / mcp.conf opt-in (whichever lands first); shipped behind that gate.
- [ ] Custom-compose validator (`mcp-compose-validator` or equivalent) shipped + unit-tested before any `install_app` rollout.
- [ ] `docs://api` continues to advertise the full PowerLab API surface (the discovery escape hatch).
- [ ] mcp-server.md gains a "Tools" section once the first batch lands.
- [ ] A per-tool threat-model line in each tool's registration (`Description` field) so the LLM surfaces side-effect class to the user.

## Posture on ecosystem evolution

This ADR locks the **current best practice as we see it today (2026-05-28)**, not the final answer. The MCP ecosystem is rapidly evolving:

- Auto-OpenAPI-to-MCP tooling may mature.
- The MCP spec may add semantically richer primitives (active resources, scoped tools, structured prompts).
- Community consensus on tool granularity may shift.

We commit to **revisiting this ADR per major release** (or earlier if the spec evolves substantially) and amending the curation list as the substrate moves. The escape hatches above are deliberate — they encode that the "right" answer is moving.

## References

- [ADR-0034](0034-standalone-observability-mcp-service.md) — foundation (read-only Foundation MVP)
- [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) — hybrid read-only architecture; `docs://api` introduced here
- [ADR-0045](0045-mcp-apps-docker-via-app-management-http-proxy.md) — app-management as a second proxied upstream; sets the storage-agnostic promise tools inherit
- [Model Context Protocol spec — Tools section](https://modelcontextprotocol.io/docs/concepts/tools)
- [Model Context Protocol spec — Resources section](https://modelcontextprotocol.io/docs/concepts/resources)
- [`modelcontextprotocol/servers`](https://github.com/modelcontextprotocol/servers) — community implementations to learn pattern from
- [`feedback_security_is_priority`](../../../memory/feedback_security_is_priority.md) — "smaller-and-safer beats larger-and-warned" — informs the curated-not-auto-gen choice
- [`feedback_prefer_official_sdk`](../../../memory/feedback_prefer_official_sdk.md) — the official Go SDK's primitives are what we build tools on
