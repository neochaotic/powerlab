# 0047. MCP agent-identity propagation — audit dogfood + JWT forwarding

- **Status:** proposed
- **Date:** 2026-05-31
- **Tracks:** [#603](https://github.com/neochaotic/powerlab/issues/603) RBAC (downstream dependency)
- **Amends:** [ADR-0034](0034-standalone-observability-mcp-service.md) (deferred items: "tool-call audit recorder dogfood", "agent-identity forwarding")
- **Builds on:** [ADR-0033](0033-audit-middleware-design.md) (per-service audit middleware contract), [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) (hybrid thin-proxy), [ADR-0046](0046-mcp-tool-curation-strategy.md) (tool curation — destructive tier already shipped)

## Context

PowerLab v0.7.5 shipped the MCP destructive tool tier (`install_app`, `uninstall_app`, `restart_app`) gated behind `EnableDestructiveTools = true`. The destructive surface is **functional but blind**:

1. **No audit dogfood.** When an MCP agent installs an app or restarts a service, **nothing** is written to `/var/log/powerlab/audit.jsonl`. The destructive action happens in the world (containers spin down, files land on disk) but the operator's compliance trail has no record of it. ADR-0034 acceptance lists this as a deferred item; ADR-0046 (tool curation) explicitly defers the "audit recorder dogfood" to a follow-up.

2. **No agent identity at the upstream.** `coreproxy.Client.RequestFrom(...)` calls core / app-management over loopback. Upstream's auth gate has a loopback-skip (correct under ADR-0034's "MCP runs on the same box" assumption). The consequence: **upstream's own audit middleware records `loopback` as the user, not the agent** — even when MCP has a fully-validated JWT from a specific PowerLab user in the request that triggered the upstream call.

Both gaps are **enterprise blockers**. The enterprise pivot (memory `project_enterprise_pivot`) explicitly named "Would enterprise IT accept this in production?" as the new design lens. The answer with the current state is "no, because we cannot tell who did what."

### Findings from grounding survey (2026-05-31)

Three findings reshaped the initial design before this ADR was written:

1. **ADR-0033 actually mandates per-service audit middleware**, not gateway-centralized. The ADR text:
   > *"Per service (not centralised at the gateway). Each service runs its own `audit.Middleware()` from `backend/common/utils/audit/`. Centralising at the gateway would require it to know the response status of downstream services after forwarding — adds an HTTP-tap that gateway doesn't have today."*

   Today only `backend/gateway/main.go` + `backend/gateway/route/management_route.go` actually wire the middleware. The constraint exists but is incompletely realized. **MCP gets to be the second service that adopts the contract — not a new pattern to invent.**

2. **`audit.Middleware()` already exists** in `backend/common/utils/audit/`. The store is JSONL append per ADR-0035 (in-memory ring buffer + on-disk file at `/var/log/powerlab/audit.jsonl`). The contract is already multi-writer-safe: ADR-0035's JSONL writer uses `O_APPEND` with mutex; multiple processes can append concurrently as long as each line is one `write(2)` call.

3. **MCP-SDK request context surfacing.** The official Go MCP SDK exposes `*mcp.ReadResourceRequest` / `*mcp.CallToolRequest` to handlers. The HTTP server in front of it sees the original `Authorization: Bearer <JWT>` header — but the MCP handler signature does not yet expose the request context's headers. The plumbing fix lives at the streamable-HTTP transport boundary.

## Decision

**MCP propagates agent identity end-to-end** by adopting two patterns that already exist in PowerLab:

### 1. MCP adopts `audit.Middleware()` (audit dogfood)

powerlab-mcp's HTTP handler chain gains the same `audit.Middleware()` core / app-management would gain when ADR-0033 is fully rolled out. Every MCP request (resource read OR tool call) writes one audit record with:

- `kind: "mcp.tool_call"` for tools, `kind: "mcp.resource_read"` for resources
- `user: <jwt.sub>` (the validated user — already extracted by the existing JWT gate at `server/server.go::preventProxyLoopbackTrust`)
- `path: /mcp` + `method: POST` for tools, `query: <resource URI>` for resources
- `status: <result>`
- `correlation_id: <X-Request-Id forwarded from client>`

**Loopback callers** (the trusted local agent — Claude Desktop on the same box) get `user: "loopback"` in the record, same sentinel core / app-management already use. Operators see "loopback called install_app on 2026-05-31 …" — which is correct: a local agent on the host did it.

### 2. JWT subject extraction at the transport boundary

The streamable-HTTP transport ahead of the MCP SDK extracts the JWT subject from `Authorization: Bearer <token>` once per request and threads it through `context.Context` to the resource / tool handler. Handlers obtain it via a typed helper:

```go
func AgentIdentity(ctx context.Context) (sub string, isLoopback bool)
```

The helper returns `("loopback", true)` for the loopback path (no JWT). Forwarding the value to the audit record is the obvious consumer; forwarding it to upstream calls (next point) is the second.

### 3. JWT forwarding on coreproxy calls

`coreproxy.Client.RequestFrom(method, service, path, token, body, contentType)` already accepts a `token` parameter that's currently always empty from MCP. The transport-boundary extractor (above) now writes the original `Authorization` header into the request context, and the proxy reads it back when making upstream calls:

- Loopback path (no JWT): proxy calls upstream with empty `Authorization`. Upstream's loopback-skip applies — same behaviour as today.
- LAN path (validated JWT): proxy forwards `Authorization: Bearer <JWT>`. Upstream's auth gate validates the same token. Upstream's audit middleware records the **correct user** (once upstream adopts the middleware per the ADR-0033 rollout).

This is two-way safe:
- A request that **claims** to be loopback but carries a JWT is forced through JWT validation (already enforced by `preventProxyLoopbackTrust`).
- Upstream's existing loopback-skip stays intact for callers without a JWT (panel UI loopback, internal health checks).

### 4. No new gateway endpoint, no SQLite, no new ADR for `audit.kind`

The original spike considered four options (see "Alternatives considered" below). The grounding survey of ADR-0033 + ADR-0035 showed that JSONL is already multi-writer-safe, and `audit.Middleware()` is already the contract. There is **nothing structurally new to introduce** — MCP becomes the second service after gateway to fully realize ADR-0033's per-service-middleware design.

The two new `kind` values (`mcp.tool_call`, `mcp.resource_read`) extend the existing `audit.Record.Kind` set. ADR-0033 already lists the field as open; no schema migration.

## Alternatives considered

Surfaced in the initial design spike, killed by the grounding survey:

| Option | Why considered | Why rejected |
|---|---|---|
| **A** — Route every MCP-upstream call through the gateway | Preserves "single audit writer" assumption | ADR-0033 doesn't require single writer; latency hop is real cost; gateway becomes SPOF for MCP. |
| **B** — MCP writes to a **separate** `mcp-audit.jsonl` | Isolation | Splits the compliance trail. Operators have to look in two files. `audit://` would need a merge layer. |
| **C** — Gateway exposes `/audit/record` endpoint MCP POSTs to | Preserves single-writer | Net-new endpoint that needs its own auth gate; spoofing concern (anyone with gateway access can fabricate audit lines). Doesn't reduce structural surface area. |
| **D** — MCP runs `audit.Middleware()` directly (chosen) | Matches ADR-0033's actual design | Selected. Zero net-new infrastructure. MCP becomes the second service to fully adopt the existing contract. |

The discovery flipped the recommendation: the spike entered favouring C ("preserve gateway as sole writer") and exited favouring D ("ADR-0033 already designed for multi-writer; MCP just adopts it").

## Consequences

### Wins

- **Compliance trail complete.** Operator query "did the agent install anything overnight?" answerable via `audit://recent?limit=200` or `audit://action/<correlation>` against the SAME audit log every other surface uses.
- **Enterprise pivot unblocked** on the destructive-tier surface. Real user identity in the audit record is table-stakes for "would enterprise IT accept this?".
- **Two-way JWT propagation enables downstream RBAC.** When [#603](https://github.com/neochaotic/powerlab/issues/603) (RBAC) lands, MCP's forwarded JWT already carries the role/claim — no MCP-side change needed.
- **ADR-0033 second adopter validates the contract.** Forcing a second service onto the per-service-middleware design surfaces any gaps that gateway-only never exposed (multi-writer JSONL contention under sustained load, `correlation_id` propagation, kind taxonomy).
- **No latency overhead.** No new gateway hop; the audit write is async (per ADR-0035's ring-buffer-then-flush design); JWT forwarding adds one header.

### Costs / risks

- **JSONL multi-writer contention.** Two writers on the same file relies on `O_APPEND` + per-line-atomic writes. ADR-0035 already covers this design but the contract has only had ONE writer in production. **Mitigation:** integration test asserting interleaved concurrent writes from gateway + MCP processes never produce a torn JSONL line; gated as a `Backend integration (powerlab-mcp)` job.
- **Upstream auth-skip drift.** If core / app-management ever DROP the loopback-skip and start requiring JWT for loopback calls (security hardening), MCP's empty-token loopback path breaks. **Mitigation:** the JWT-forwarding fallback already covers the case — pass the JWT we have; upstream validates it normally. The empty-token path stays for the "no JWT received" case.
- **MCP-SDK lock-in on context plumbing.** The agent-identity helper depends on the streamable-HTTP transport extracting the header. If the SDK changes how it surfaces headers, the extractor needs an update. **Mitigation:** the extractor is one ~20-line file; rewrite is mechanical.
- **`audit.kind` value sprawl.** Two new values land here; future MCP tools could add more (`mcp.prompt_invoked` if we ship MCP Prompts; `mcp.resource_subscribed` for active resources). **Mitigation:** lock the kind set as a typed constant in `backend/common/utils/audit/types.go`; add a kind requires a one-line code change + the documented set in ADR-0033.

### Operational changes

- `mcp.conf` gains one new optional key: `AuditPath = /var/log/powerlab/audit.jsonl` (matches the existing `audit://` reader path; default sample shipped). Operator can point MCP at a different path for dev isolation.
- `powerlab-mcp.service` gains write access to `/var/log/powerlab/audit.jsonl`. Currently it has read access (root:root 0600 file; MCP runs as root per ADR-0034). No unit change needed — already root.
- `journal://mcp` continues to capture transport-layer logs (boot, bind, errors). `audit://recent` captures business-level events (which tool ran, who called it). Two surfaces, complementary; documented.

### Out of scope (separate ADRs)

- **MCP Prompts primitive** — ADR-0046 reserved flexibility; the docs surface follow-up (planned PR δ) will use it for `compose_authoring`. Identity propagation here does not block that work.
- **RBAC enforcement** — #603 is the downstream consumer of forwarded JWTs; this ADR just ensures the JWT reaches the upstream.
- **Audit retention policy for MCP records** — ADR-0035 already covers retention (ring buffer + file rotation); MCP records get the same handling.

## Acceptance criteria

- [ ] `backend/common/utils/audit/types.go` declares `KindMCPToolCall = "mcp.tool_call"` and `KindMCPResourceRead = "mcp.resource_read"` constants.
- [ ] `backend/powerlab-mcp/server/server.go` wires `audit.Middleware()` ahead of the MCP transport handler; tool calls + resource reads produce one audit record each.
- [ ] Audit records carry the correct `user` field — JWT subject for LAN callers, `"loopback"` sentinel for loopback callers.
- [ ] `backend/powerlab-mcp/server/server.go` exposes `AgentIdentity(ctx) (sub string, isLoopback bool)` helper; handlers (current + future) consume it via context.
- [ ] `coreproxy.Client.RequestFrom` forwards the JWT extracted by `AgentIdentity` to upstream calls when present; empty-token loopback path preserved when absent.
- [ ] Integration test: two concurrent processes appending to the same audit.jsonl produce no torn lines under sustained load (1000 records each, interleaved).
- [ ] Integration test: an MCP tool call from a LAN client (carrying a valid JWT) produces one audit record with `user=<jwt.sub>` AND triggers an upstream call carrying the same `Authorization` header.
- [ ] `docs/concepts/mcp-server.md` updated to reflect identity propagation, the new `AuditPath` key, and the journal-vs-audit complementarity.

## References

- [ADR-0033](0033-audit-middleware-design.md) — audit middleware contract (per-service, async writer, JSONL append)
- [ADR-0034](0034-standalone-observability-mcp-service.md) — powerlab-mcp foundation; this ADR closes the "audit dogfood" + "agent identity forwarding" deferred items
- [ADR-0035](0035-audit-storage-jsonl.md) — JSONL storage (multi-writer safe via `O_APPEND` + per-line-atomic writes)
- [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) — coreproxy already designed with `token` parameter we now wire end-to-end
- [ADR-0046](0046-mcp-tool-curation-strategy.md) — explicitly deferred "audit recorder dogfood" to this ADR
- [memory: project_enterprise_pivot](../../../memory/project_enterprise_pivot.md) — design lens that promoted this work
- [memory: feedback_mcp_reuse_observability](../../../memory/feedback_mcp_reuse_observability.md) — reuse the audit middleware that exists, do not write a new one
