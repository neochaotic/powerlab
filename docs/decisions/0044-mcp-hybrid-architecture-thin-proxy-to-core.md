# 0044 — powerlab-mcp hybrid architecture: thin proxy to core for sysadmin telemetry

- **Status:** accepted
- **Date:** 2026-05-28
- **Trigger:** Adversarial review of the powerlab-mcp MVP plan (2026-05-28) surfaced that the original ADR-0034 "isolated MCP" stance was about to drive massive code duplication. powerlab-mcp was planning hand-rolled `/proc/mounts` readers, a fresh GPU detector, a separate processes/network/SMART probe — every single one of those is already implemented and battle-tested in `backend/core/`, `backend/common/external/`, and `backend/local-storage/`. The maintainer flagged it: "ja temos telemetria de quase tudo, inclusive temperatura e qualidade do disco."

## Context

ADR-0034 chose **independence** as powerlab-mcp's defining trait:

> "Independence wins debugging. Observability running alone answers 'I can't see what's broken' even when the gateway is the broken thing."

That was right for the Foundation MVP surface — `audit://` (reads a JSONL file) and `journal://` (shells to journalctl). Both are pure file/exec reads with no PowerLab-service dependencies. MCP can answer "what failed?" even when core, gateway, app-management, and the message-bus are all down.

But the agent-useful surface — the one the maintainer wants for "talk to your homelab" — is much wider:

| Resource the agent wants | Already in PowerLab? | Where |
|---|---|---|
| CPU + RAM + load + temperature | ✅ | `core::GetSystemUtilization` |
| Per-mount disk + SMART | ✅ | `core::GetSystemDiskInfo` + `local-storage::GetDiskList` (gopsutil) |
| Network interfaces + traffic | ✅ | `core::GetSystemNetInfo` + `GetNetworkInterfaces` |
| GPU utilisation (Apple Silicon ioreg + Nvidia smi) | ✅ | `common/external::GetGPUUtilization` |
| Hardware info | ✅ | `core::GetSystemHardwareInfo` |
| CPU thermal zone | ✅ | `core::GetCPUThermalZone` |
| Active users / sessions | ✅ | `core::GetSystemUsers` |
| Installed apps + their state | ✅ | `app-management::v2/*` |
| Container logs | ✅ | `logs-cli::buildAppArgs + resolveAppContainer` (wraps `docker logs`) |

Re-reading raw `/proc` files for every one of these inside powerlab-mcp would mean:

- ~2,000 lines of parser code duplicating logic that already works
- Two truths whenever PowerLab UI dashboard's data and MCP's data drift (bug fixes have to ship twice)
- Operator-visible inconsistency: `powerlab-logs` says one thing, the agent says another
- Maintenance cliff: every gopsutil bump, every CPU model the dashboard learns, every quirk we discover (Apple Silicon ioreg edge cases, btrfs Bavail oddities) — all have to be replicated in MCP

Independence was the *right call* for `audit://` and `journal://`. It's the *wrong call* for system telemetry.

## Decision

powerlab-mcp adopts a **hybrid architecture**:

1. **Independent resources** — read raw, MCP-native, survive any other service being down:
   - `audit://*` — reads `audit.jsonl` directly (no other service knows how to)
   - `journal://*` for PowerLab units — shells to `journalctl -u powerlab-*.service` (no PowerLab dep)
   - `system://schema`, `audit://schema`, `journal://schema` — literal JSON docs
   - `journal://units` — discovery, reads `/etc/systemd/system/powerlab-*.service` directly

2. **Proxied resources** — thin HTTP wrappers over `core` (and later `app-management`):
   - `system://utilization` → `GET <core>/v1/sys/utilization`
   - `system://disk` → `GET <core>/v1/sys/disk-info`
   - `system://network` → `GET <core>/v1/sys/net-info`
   - `system://gpu` → `GET <core>/v1/sys/utilization` (GPU is part of util in core today) **OR** import `common/external::GetGPUUtilization` directly (zero-network path)
   - `apps://list`, `apps://state/{id}` → `GET <app-management>/v2/app_management/*`
   - `docker://logs/{container}` → reuse `logs-cli::buildAppArgs` (no docker socket; uses `docker logs` CLI just like the operator does)

3. **A new top-level `docs://` namespace** publishes the OpenAPI specs of every PowerLab service so an agent can self-discover the API surface (mirrors the Scalar docs portal — ADR-0008):
   - `docs://api` — manifest of available specs
   - `docs://api/{service}` — the raw OpenAPI YAML for one service

### When core is down

Proxied resources return a structured error the agent recognises — **NOT** a fake snapshot:

```json
{
  "error": "core_unavailable",
  "detail": "GET http://127.0.0.1:8810/v1/sys/utilization: connection refused",
  "fallback": "audit:// and journal:// still work and may show the cause"
}
```

The agent is told *exactly* what failed and what it can still read. This preserves the "I can see what's broken when things are broken" guarantee for the independent resources, while accepting that sysadmin telemetry is a feature of *PowerLab as a whole* — when core is down, the operator's main panel is down too, and that's the bigger problem.

### Auth forwarding

When the MCP request carries a Bearer JWT (the LAN path), powerlab-mcp **forwards that token** to the downstream core call. core's own auth gate validates it the same way it validates a request from the panel. Loopback requests to MCP get loopback-trust forwarded too: MCP calls core on `127.0.0.1` so core's own loopback skip applies. No service account, no shared secret.

### Service URL discovery

powerlab-mcp resolves each service's URL the same way every other PowerLab service does today: by reading the `.url` file the service writes at startup under `RuntimePath` (typically `/var/run/powerlab/`). core publishes `casaos.url` (legacy filename — kept stable for compatibility). The resolver is cached with a short TTL so a brief core restart doesn't lock MCP into a stale URL.

## Consequences

### Wins

- **No duplication.** Every existing telemetry endpoint becomes an MCP resource with a ~30-line proxy. No `/proc` parsers, no gopsutil bumps, no parallel SMART logic.
- **Single source of truth.** When PowerLab learns to read a new disk health metric, MCP sees it on the next request. When the dashboard learns about Apple Silicon thermals, the agent does too.
- **Agent gains the full API surface.** `docs://api` lets the AI discover every endpoint, every parameter, every response shape — without us hand-coding each as an MCP tool. Combined with future MCP tool wrappers (Phase 3), this makes the agent a first-class PowerLab API client.
- **Audit + journal of PowerLab units stay independent.** When core panics or app-management deadlocks, the agent can still ask "what happened in the gateway journal in the last 5 minutes?" — which is exactly when the answer matters most.

### Costs / risks

- **MCP is no longer entirely standalone.** Sysadmin reads need core up. Mitigation: the structured-error path; ADR-0034's independence promise is now scoped to audit/journal only and documented as such.
- **HTTP cost.** Every system://* read crosses loopback HTTP. Negligible (microseconds), and core is already paying that cost for the panel. No regression.
- **Auth coupling.** A core auth bug surfaces in MCP too. Mitigation: both already use `jwt.Validate`, so this was true even under the isolated design.
- **Service URL discovery brittleness.** If `RuntimePath` is misconfigured or `casaos.url` is stale, proxy reads fail. Mitigation: structured error tells the agent + operator what's wrong; the discovery cache TTL is short (10s) so recovery is fast.

### Operational changes

- `mcp.conf` gains no new keys for this — `RuntimePath` already pointed at where the URL files live; it now drives both JWKS lookup (existing) and service URL resolution (new).
- The systemd unit gains `Wants=powerlab-core.service` (already present per ADR-0034 acceptance) and a documented soft dependency — MCP still starts when core is down, it just can't proxy then.

### Out of scope (separate ADRs)

- **MCP write tools** that mutate state (`install_app`, `restart_service`, `prune_orphans`) — those need their own threat-model ADR; this one is read-only proxying.
- **Schema-driven tool generation from OpenAPI** — auto-emitting MCP tools from `docs://api` is a future Phase 3 capability.
- **Tool-call audit recorder dogfood** — MCP writing its own actions to `audit.jsonl` was already planned (ADR-0034 acceptance); not touched here.

## Acceptance criteria

- [ ] `coreproxy` package resolves `<runtime>/casaos.url`, fetches with JWT forwarding, caches the URL for ~10s, returns a structured error on failure.
- [ ] `docs://api` returns a manifest of bundled specs; `docs://api/{service}` returns the raw YAML.
- [ ] `system://utilization` proxies `GET /v1/sys/utilization` and round-trips the payload to the agent.
- [ ] When core is down, `system://utilization` returns the `core_unavailable` shape; `audit://` and `journal://` remain readable.
- [ ] `audit://`, `journal://` (powerlab units), and `journal://units` remain independent of core (regression-tested).
- [ ] ADR-0034 amended with a pointer back to this ADR and the scope of "isolated" tightened to audit + journal.

## References

- [ADR-0034](0034-standalone-observability-mcp-service.md) — original "isolated MCP" decision, now amended.
- [ADR-0033](0033-audit-system-design.md) — audit middleware shape powerlab-mcp tails.
- [ADR-0035](0035-audit-jsonl-migration.md) — JSONL migration that enables `audit://`.
- [ADR-0008](0008-api-docs-portal-scalar.md) — Scalar API docs portal; `docs://api` is the same OpenAPI surface, MCP-shaped.
- [`feedback_mcp_reuse_observability`](../../../memory/feedback_mcp_reuse_observability.md) — internal note locking the reuse principle in.
