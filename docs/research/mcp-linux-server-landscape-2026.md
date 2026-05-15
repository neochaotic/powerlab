# MCP-for-Linux landscape — 2026 snapshot

**Status:** research note, NOT a decision
**Date:** 2026-05-14
**Feeds:** ADR-0034 (standalone observability + MCP service)
**Related memory:** [[project_mcp_linux_connector_vision]]

Quick scan of existing Model Context Protocol (MCP) servers that overlap with what PowerLab's standalone observability service would expose. Captures direct competitors, official patterns, and gaps PowerLab could fill. **Revisit before locking ADR-0034.**

## 1. Direct prior art (Linux server / homelab management via MCP)

### Generic SSH-based

- **[ssh-mcp](https://github.com/tufantunc/ssh-mcp)** — MCP server that exposes SSH control for Linux + Windows. Pure shell pass-through — no semantic resources, agent has to know what command to run.
- **[Remote MCP Server](https://conare.ai/marketplace/mcp/remote-mcp-server-a87d)** — Lightweight MCP server on a remote machine, exposes the box over HTTPS to MCP-compatible clients (Claude Desktop, Cursor, etc.).
- **[SSH MCP](https://agentpedia.codes/mcp/ssh)** — Agentic SSH wrapper for "execute commands, transfer files, manage infrastructure" via API-driven workflows.

### Diagnostics-focused

- **[linux-mcp-server (PyPI)](https://pypi.org/project/linux-mcp-server/)** — RHEL-based, **read-only** Linux system administration via MCP: system info, services, processes, **logs, network, storage**. SSH-based with key auth, multi-host. Closest single competitor in spirit; PowerLab's scope would extend (apps, audit, actions).

### Homelab-specific

- **[homelab_mcp (washyu)](https://mcpservers.org/servers/washyu/mcp_python_server)** — SSH-based homelab management. Generates SSH key pair on first run, onboards hosts, discovers hardware, installs services, controls VMs, maps network. Python.
- **[Homelab MCP (theonlytruebigmac)](https://lobehub.com/mcp/theonlytruebigmac-homelab-mcp)** — separate project, similar scope.
- **[Homelab Unified Management](https://mcpmarket.com/server/homelab-1)** — Docker/Podman containers + Ollama models + Pi-hole + Unifi + Ansible from one MCP surface.

### Single-purpose

- **[docker-mcp (QuantGeekDev)](https://github.com/QuantGeekDev/docker-mcp)** — Docker-only MCP server.
- **[log-mcp (ascii766164696D)](https://github.com/ascii766164696D/log-mcp)** — log file analysis without loading whole file into context. Smart cursor-based access.
- **[MCP-Analyzer (klara-research)](https://github.com/klara-research/MCP-Analyzer)** — reads MCP server's own logs for debugging.
- **[Filesystem MCP Server (official)](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem)** — official reference impl. Configurable directory access controls. Node.js.

### First-party from incumbents

- **[Docker AI Agent + MCP](https://www.docker.com/blog/simplify-ai-development-with-the-model-context-protocol-and-docker/)** — Docker is shipping its own MCP server in Docker Desktop. Container/compose stack management.
- **[Azure MCP Server 2.0](https://devblogs.microsoft.com/azure-sdk/announcing-azure-mcp-server-2-0-stable-release/)** — self-hosted agentic cloud automation, GA'd recently.
- **[GitHub MCP Server](https://github.com/github/github-mcp-server/releases)** — official, repo/PR/issue surface.

## 2. Official MCP standards + conventions

- **[MCP spec — Resources](https://modelcontextprotocol.io/specification/2025-06-18/server/resources)** — current spec for resource URIs.
- **[Official MCP Registry (Anthropic)](https://registry.modelcontextprotocol.io/)** — launched 2025-09-08. Authoritative directory of public MCP servers. PowerLab would publish here.
- **[modelcontextprotocol/servers](https://github.com/modelcontextprotocol/servers)** — reference implementations (official, maintained by steering group).
- **[MCP Apps spec (draft)](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/draft/apps.mdx)** — extends MCP servers with interactive UIs via the new `ui://` scheme. Would be how a PowerLab UI surface gets exposed without its own SPA.

### URI scheme conventions per spec

| Scheme | When to use |
|---|---|
| `https://`, `http://` | Resources the **client** can fetch directly off the public web — server just hands back a URL. NOT for resources the MCP server proxies/transforms. |
| `file://` | Filesystem-shaped resources. Spec note: doesn't have to map to a physical FS. |
| `ui://` | MCP Apps draft — embedded interactive UI tied to a tool. |
| custom | Anything else — explicit allow per spec. Examples in the wild: `db://customers/recent`. |
| URI templates (RFC 6570) | Parameterised lookups: `db://customers/{id}`. |

## 3. Observability infra patterns (worth borrowing)

- **[AWS deploying MCP on ECS](https://aws.amazon.com/blogs/containers/deploying-model-context-protocol-mcp-servers-on-amazon-ecs/)** — pattern uses CloudTrail + CloudWatch with **JSON-structured logging** for advanced querying ("avg tool invocation latency by tool name"). Container Insights + Prometheus/Grafana/Jaeger/Loki monitoring stack. Template for what PowerLab's observability service should expose as metrics about itself.
- **[MintMCP — best self-hosted gateways 2026](https://www.mintmcp.com/blog/mcp-gateways-self-hosted-deployments)** — survey of MCP gateways, useful for not-rebuilding-the-wheel calls.
- **[mcpjungle/MCPJungle](https://github.com/mcpjungle/MCPJungle)** — self-hosted MCP gateway. Worth checking if PowerLab can BE this gateway for a homelab.

## 4. Gaps PowerLab can fill (positioning)

What the landscape is missing — opportunity space:

| Existing pattern | Gap |
|---|---|
| Generic SSH wrappers (`ssh-mcp`, etc.) | No semantics — agent must know commands. PowerLab can expose **typed resources** (audit, journal, apps, system, containers). |
| Single-purpose servers (just Docker, just logs) | Operator has to install + auth N servers for one box. PowerLab is **batteries-included** for the whole Linux box. |
| Read-only diagnostic servers (`linux-mcp-server`) | No actions. PowerLab can expose **gated tools** (restart, prune, backup) with auth tiers (read / auth / admin). |
| Homelab-only SSH wrappers (`homelab_mcp`, etc.) | Bolted-on MCP, not MCP-native. PowerLab would be **MCP-first** in design. |
| Filesystem + log servers | Don't carry **audit context** (who did what when). PowerLab's audit middleware (Sprint 16) gives this for free. |

Summary: nothing in the wild today combines (a) full Linux-box surface, (b) MCP-native design, (c) gated actions, (d) embedded audit, (e) standalone (no UI dependency), (f) under one open-source binary.

## 5. Implications for ADR-0034

Decisions to lock based on this landscape:

1. **URI namespace** — use custom schemes (`audit://`, `journal://`, `apps://`, `system://`, `containers://`) per spec's "free to use additional schemes" allowance. Avoid `file://` for our semantic resources — reserve it for actual file paths the agent might want to read literally.

2. **Action gating** — three tiers visible in the market: read-only / requires-auth / admin-confirm. Map cleanly to PowerLab's existing JWT (no/yes) + a "destructive" prompt for the admin tier.

3. **Don't rebuild the gateway pattern** — survey existing MCP gateways (MCPJungle, MintMCP listings) before building one from scratch. The standalone service may not need to ALSO be a gateway.

4. **Publish to the official registry** when stable (registry.modelcontextprotocol.io). PowerLab's discoverability story.

5. **Watch the MCP Apps draft** (`ui://` scheme) — if it stabilises, the Settings → Audit pane could become an MCP App embedded in any MCP-aware host (Claude Desktop, Cursor) instead of an in-PowerLab Svelte component. **Big leverage** for the "MCP-for-Linux" pivot vision.

## Sources

Listed inline above as markdown links. Full revisit recommended before ADR-0034 acceptance — landscape moves fast.
