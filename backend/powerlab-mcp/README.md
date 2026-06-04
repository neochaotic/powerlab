# powerlab-mcp

> Model Context Protocol server for [PowerLab](https://github.com/neochaotic/powerlab) — turns a PowerLab homelab/enterprise host into a structured, agent-driven surface for catalog browsing, app management, system observability, and compose-yaml authoring.

[![PowerLab](https://img.shields.io/badge/powerlab-MCP%20server-emerald)](https://github.com/neochaotic/powerlab)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

`powerlab-mcp` is the [Model Context Protocol](https://modelcontextprotocol.io) bridge between an AI agent (Claude Desktop, Cursor, MCP-aware tooling) and a running PowerLab host. The agent gets:

- **9 read-only Tools** + 3 opt-in Tools (`install_app`, `uninstall_app`, `restart_app`) gated behind operator opt-in
- **4 Prompts** that encode multi-Tool PowerLab workflows so agents don't have to re-derive them per session
- **~20 Resources** spanning live system metrics, app state, container logs, audit history, and the bundled app catalog

Built on the [official `modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk). All schemas are code-first (no JSON-schema duplication). Transport: Streamable HTTP (2025-06-18 spec) on path `/mcp`. Compliant with the MCP 2025-03 protocol revision.

## Why this exists

PowerLab is a self-hostable platform for Docker-Compose apps with first-class observability (audit, journal, security tiers). The MCP server exposes that surface to agents so the operator can say *"why did the Jellyfin install fail?"* and the agent runs the diagnostic chain — `audit_query → journal_search → get_system_health` — without scraping a UI or shelling in.

The design lens is **enterprise** (security-tiered, audit-logged, capability-discoverable) rather than convenience-first.

## Quickstart — Streamable-HTTP transport

`powerlab-mcp` ships with every PowerLab release (`install.sh` installs it as a systemd unit and binds it on the configured port). Once the unit is running, point your MCP client at the URL:

```
http://<powerlab-host>:<ListenAddr port>/mcp
```

The endpoint path is always `/mcp`. The port is whatever you put in `ListenAddr` in `/etc/powerlab/mcp.conf` (PowerLab's installer picks a sane default).

Authentication: PowerLab JWT, sent as `Authorization: Bearer <token>`. The operator JWT is the same token the PowerLab panel uses. Loopback (`127.0.0.1`, `::1`) is trusted by default for the local-agent case; off-host calls require the Bearer.

For clients that only support MCP stdio (e.g. older Claude Desktop builds), wrap the HTTP endpoint with [mcp-proxy](https://github.com/sparfenyuk/mcp-proxy) or a similar stdio↔HTTP bridge.

## Capability summary

| Surface | Count | Notes |
| --- | ---: | --- |
| Tools (read-only) | 9 | Always advertised |
| Tools (opt-in) | 3 | `install_app`, `uninstall_app`, `restart_app` — require `EnableDestructiveTools=true` |
| Prompts | 4 | Authoring + observability recipes |
| Resources | ~20 | `catalog://`, `apps://`, `audit://`, `journal://`, `docs://`, `system://`, `docker://`, `prompt://` |

`list_capabilities` is itself a Tool — call it first to discover which tiers are active on a given host before attempting any gated operation.

## Tools

### Always available (read-only)

| Tool | Purpose |
| --- | --- |
| `browse_catalog` | List PowerLab community-catalog apps available for install or as compose-pattern references. Optional substring filter. |
| `get_compose_conventions` | Returns the canonical PowerLab docker-compose conventions document (x-powerlab metadata, volume paths, image trust policy). Call before drafting any compose YAML. |
| `start_compose_authoring` | Bundles conventions + 3 representative catalog examples + the composevalidator deny-list as the grounding context for a new compose-yaml. Tool form of the `compose_authoring` prompt. |
| `get_system_health` | Aggregate ok/warn/critical across memory, disk, PowerLab services, and pending OS updates. One call instead of four resource reads. |
| `check_disk_free` | Quick free-space check for a single mount point. |
| `journal_search` | Search PowerLab service journals (`gateway.log`, `core.log`, `app-management.log`, etc.) by pattern + time range. |
| `search_docs` | Case-insensitive substring search across PowerLab docs/concepts/api specs + the bundled app catalog. |
| `list_capabilities` | Reports which operator-opt-in tiers are active (destructive Tools, sensitive sysadmin Resources). |
| `generate_artifact` | Propose-then-review path for compose-yaml / shell / config drafts. Runs composevalidator for compose-yaml; surfaces a "no validator" note for other kinds. |

### Gated behind `EnableDestructiveTools=true`

| Tool | Purpose |
| --- | --- |
| `install_app` | Install a compose-yaml app. The yaml is locally validated against the [ADR-0046](../../docs/decisions/0046-mcp-tool-curation-strategy.md) deny-list (no privileged, no host namespaces, no Docker socket, no sensitive host path binds, etc.) before app-management sees it. Set `dry_run=true` to validate without installing. |
| `uninstall_app` | Uninstall a PowerLab app — removes containers and (per app-management config) may remove persistent data. Not reversible without a backup. |
| `restart_app` | Restart all containers of one installed app. Side-effect, not destructive. |

## Prompts

Prompts encode multi-Tool workflows so chat-mode agents don't have to derive them per session.

| Prompt | Tool chain | Optional args |
| --- | --- | --- |
| `compose_authoring` | conventions + 3 catalog examples + validator deny-list | `app_type` (database, media, ai, dashboard) |
| `troubleshoot_install_failure` | `audit_query` → `journal_search` → `get_system_health` | `app_id`, `since_minutes` |
| `debug_unhealthy_service` | `get_system_health` → `journal_search` → `check_disk_free` | `service`, `since_minutes` |
| `onboard_new_powerlab_host` | `list_capabilities` → `browse_catalog` → `get_system_health` | `goal`, `experience_level` (beginner/intermediate/expert) |

## Resources

Read-only data the agent can fetch by URI. URIs use stable schemes; templates use `{}` placeholders.

| Scheme | Description |
| --- | --- |
| `catalog://app/{id}` | One PowerLab catalog app — `docker-compose.yml` + metadata. `catalog://index` lists every app. |
| `apps://state/{id}` | Live runtime state of one installed app. `apps://list` enumerates installed ids. |
| `docker://logs/{id}` | Recent container stderr for one app. |
| `audit://recent` | PowerLab audit JSONL ([ADR-0035](../../docs/decisions/0035-audit-storage-jsonl.md)) — last N entries. |
| `journal://{unit}` | Raw tail of one PowerLab service journal. `journal://units` lists available unit names. |
| `journal://system/auth`, `journal://system/failures` | Sensitive sysadmin streams — gated behind `EnableSensitiveTier=true`. |
| `docs://concepts/{name}` | PowerLab concept documentation (compose-conventions, security, audit). |
| `docs://api/{name}` | Bundled OpenAPI specs. |
| `system://memory`, `system://disk`, `system://services`, `system://updates`, `system://network`, `system://kernel`, `system://processes`, `system://gpu` | Live host telemetry. Aggregated via `get_system_health`. |
| `prompt://{name}` | Direct read of a Prompt's bundled message set (for clients that don't render Prompts natively). |

## Capability tiers — operator opt-in

The MCP server runs in a least-privilege default. The operator must explicitly turn on:

| Config key | What unlocks | Risk |
| --- | --- | --- |
| `EnableDestructiveTools` | `install_app`, `uninstall_app`, `restart_app` | Container lifecycle + data loss potential |
| `EnableSensitiveTier` | `journal://system/auth`, `journal://system/failures` | Authentication failure logs — secrets-adjacent |

Both are read off `/etc/powerlab/mcp.conf`. `list_capabilities` reports current state; `tools/list` and `resources/list` honor the gate — a disabled tier's Tools/Resources are not advertised, so the agent can't accidentally call them.

## Architecture

```
┌────────────────────────────────────┐
│ Agent — Claude Desktop / Cursor /  │
│ MCP-aware tooling                  │
└──────────────┬─────────────────────┘
               │ MCP Streamable HTTP
               │ POST /mcp + Bearer <JWT>
               ▼
┌──────────────────────────────────┐
│ powerlab-mcp (this binary)       │
│  - Tools (9 + 3 gated)           │
│  - Prompts (4)                   │
│  - Resources (~20)               │
└──────────────┬───────────────────┘
               │ HTTP, service-discovery
               │ via *.url files in RuntimePath
               ▼
┌──────────────────────────────────┐
│ PowerLab core + app-management   │
│ (audit + journal read direct     │
│  from /var/log/powerlab/)        │
└──────────────────────────────────┘
```

The server is a thin protocol bridge — it owns no state. `audit://` + `journal://` read directly from `/var/log/powerlab/`; everything else proxies to PowerLab services with the calling agent's JWT, with upstream URLs resolved from the `.url` files each service publishes at startup (see `coreproxy/coreproxy.go`).

## Configuration

`/etc/powerlab/mcp.conf` is a flat `Key = value` file. The installer writes a working default. Keys:

| Key | Default | Purpose |
| --- | --- | --- |
| `Disabled` | `false` | Kill switch — `true` makes the binary exit cleanly without binding (so `systemd Restart=always` doesn't loop). |
| `ListenAddr` | (set by installer) | TCP bind address for the Streamable-HTTP transport. |
| `RuntimePath` | `/run/powerlab` | Directory where PowerLab services publish their `*.url` files. |
| `AuditDir` | `/var/log/powerlab` | Audit JSONL location. |
| `OpenAPIDir` | `/usr/share/powerlab/openapi` | Bundled OpenAPI specs. |
| `ConceptsDir` | `/usr/share/powerlab/docs/concepts` | docs://concepts/* source. |
| `CatalogDir` | `/usr/share/powerlab/catalog` | Catalog snapshot. |
| `SystemdSystemDir` | `/etc/systemd/system` | Unit-file discovery for `journal://units`. |
| `EnableDestructiveTools` | `false` | See [Capability tiers](#capability-tiers--operator-opt-in). |
| `EnableSensitiveTier` | `false` | Same. |

CLI flag: `--conf=/path/to/mcp.conf` (overrides the default config path).

## Development

```bash
# Run tests
go test ./...

# Lint (8-linter set, see ../../.golangci.yml)
golangci-lint run ./...

# Build the binary
go build -o ./powerlab-mcp ./cmd

# Smoke test against a deployed host (Lima, VM, real box)
go run ./cmd/smoke
```

The smoke client (`cmd/smoke`) drives the deployed binary through every advertised Tool + Resource, asserting both transport success and data quality. Run it before any release cut — and after any new Tool/Resource lands.

## Related ADRs

- [ADR-0034](../../docs/decisions/0034-standalone-observability-mcp-service.md) — Standalone observability MCP service (foundation)
- [ADR-0035](../../docs/decisions/0035-audit-storage-jsonl.md) — Audit storage as JSONL (read source for `audit://`)
- [ADR-0046](../../docs/decisions/0046-mcp-tool-curation-strategy.md) — MCP tool curation + compose deny-list
- [ADR-0048](../../docs/decisions/0048-mcp-docs-surface-compose-authoring.md) — Docs surface + `compose_authoring` prompt

## License

AGPLv3 — see [LICENSE](../../LICENSE). PowerLab's source of truth is at [neochaotic/powerlab](https://github.com/neochaotic/powerlab); issues + PRs go there.
