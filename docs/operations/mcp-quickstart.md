# MCP — operator quickstart

> Your homelab now has an agent-ready surface. This page is the **5-minute path from "is it running?" to "Claude is reading my journals"**. If you want the architecture (how it's wired, why, the security model), read [Concepts → MCP server](../concepts/mcp-server.md) instead.

## At a glance

PowerLab ships a built-in **MCP (Model Context Protocol) server**. It runs as `powerlab-mcp.service` on `:9090`. Your AI agent connects to it and reads the same data the panel dashboard shows you — metrics, journals, audit trail, installed apps, container logs, raw Docker daemon visibility, the entire PowerLab OpenAPI surface, the concept docs you're reading right now, and the 137-app compose catalog as pattern reference. One MCP Prompt (`compose_authoring`) bundles conventions + worked examples + validator rules to ground an agent designing a new compose YAML in one round-trip; six chat-mode-friendly Tools (`browse_catalog`, `get_compose_conventions`, `start_compose_authoring`, `get_system_health`, `generate_artifact`, `list_capabilities`) make the same canonical content reachable through `tools/call` for agents that don't surface Prompts.

The UI is the pane of glass for you. **MCP is the pane of glass for your agent.**

---

## Step 1 — verify it's alive (30 seconds)

The MCP service is enabled by default after `install.sh`. On the box:

```bash
# Service status
sudo systemctl status powerlab-mcp --no-pager | head -3

# Liveness check
curl -fsS http://localhost:9090/healthz                              # → 200 OK
curl -fsS http://localhost:9090/version | jq                         # → {"version":"...","commit":"..."}

# Recent boot log
sudo journalctl -u powerlab-mcp -n 20 --no-pager
```

If `/healthz` returns 200, the rest works. If not, see [Step 6 — troubleshooting](#step-6--troubleshooting).

---

## Step 2 — structured smoke (1 minute)

`powerlab-mcp-smoke` (shipped with the install) reads every advertised resource + exercises read-only tools end-to-end. Run it once after install or after every upgrade:

```bash
/usr/bin/powerlab-mcp-smoke -endpoint http://localhost:9090
```

You should see something like:

```
PASS  /healthz + /version
PASS  mcp connect + initialize
PASS  resources/list (25 advertised)
PASS  system://metrics                        → 8 fields sane
PASS  system://utilization                    → proxied 903 bytes
PASS  apps://list                             → proxied 18245 bytes
PASS  catalog://index                         → 137 app(s) in catalog
PASS  docker://system                         → daemon=29.5.1 containers=2 images=2
PASS  docs://concepts/index                   → 4 concept(s) advertised
PASS  audit://recent?limit=5                  → 5 records with valid ts/status/method
PASS  tools/list (4 advertised)
PASS  journal_search (unit=gateway, 10 entries)
PASS  check_disk_free / (77% used, 54 GiB available)
```

Any FAIL is actionable. The most common one — `audit://recent permission denied` — only happens when smoke runs as a non-root user against the production `/var/log/powerlab/audit.jsonl` (which is `root:root 0600`). The service itself, running as root via systemd, reads it fine.

---

## Step 3 — pair your AI client (3 minutes)

PowerLab MCP speaks the standard MCP HTTP-streaming transport, so every spec-compliant client connects the same way. Pick yours below; the **Claude Desktop** sections are the most fleshed-out (loopback + LAN paths) and the others summarise the same shape.

> **Loopback vs LAN.** PowerLab's loopback policy ([ADR-0034](../decisions/0034-standalone-observability-mcp-service.md)) trusts every connection arriving from `127.0.0.1` / `::1` — no JWT, no signin. From the **same machine** (laptop running both client + PowerLab, or Lima/Docker-Desktop VM port-forwarded to localhost), no token is needed. From a **separate box** on your LAN, grab a Bearer token from the panel's browser network panel: sign in to `http://<your-box>:8765`, open dev tools → Network, refresh any panel page, copy the `Authorization: Bearer <token>` value from any API request. The same token works as a `--header` arg / config field in every client below.

> ⚠️ **HTTPS for the LAN path.** Sending the Bearer token over plain HTTP on an untrusted network is a credential-leak risk. For any LAN deployment, front PowerLab with a reverse proxy or cloud LB so the MCP endpoint speaks HTTPS — see [Reverse proxy + TLS recipes](reverse-proxy.md) (6 recipes: Caddy / nginx / Tailscale Funnel / Cloudflare Tunnel / cloud LB / K8s Ingress). The configs below assume `http://...` for the loopback case; swap to `https://<your-domain>/mcp` once your proxy is in place and drop the `--allow-http` flag.

### Client: Claude Desktop

Path A (loopback) and Path B (LAN) below; use `mcp-remote` as a stdio↔HTTP bridge (Claude Desktop's native HTTP transport support is shipping unevenly across versions; the bridge works everywhere).

#### Path A — same machine (loopback)

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS (`%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "powerlab": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote@latest",
        "http://127.0.0.1:9090/mcp",
        "--allow-http",
        "--transport",
        "http-only"
      ]
    }
  }
}
```

Restart Claude Desktop (Cmd+Q + reopen — config is read once at launch).

#### Path B — separate box (LAN)

When Claude Desktop and PowerLab are on different machines, grab a token from the panel (see the loopback-vs-LAN box at the top of Step 3), then config:

```json
{
  "mcpServers": {
    "powerlab": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote@latest",
        "http://<your-box>:9090/mcp",
        "--allow-http",
        "--transport",
        "http-only",
        "--header",
        "Authorization: Bearer <your-token>"
      ]
    }
  }
}
```

### Verify

After restart, the `powerlab` server should appear in the MCP indicator at the bottom of the compose area. Ask Claude:

> *"What's running on my homelab? Check disk and any failed services."*

Claude reads `apps://list`, `system://services`, `system://disk` and answers with real data.

### Gotcha — Claude Code "Code" mode is NOT a clean MCP test

Claude Desktop has a "Code" tab (formerly Claude Code embed) where the agent is project-folder-scoped. **Don't use that mode to evaluate the MCP** — once a folder is selected, the agent's Read/Glob/Grep tools dominate and MCP becomes a secondary source. A 2026-05-31 test ran "use only the MCP to author a compose YAML" in Code mode and got legacy-CasaOS conventions back (the agent had read the source tree directly) instead of the canonical `compose_authoring` Prompt output.

Use the **Chat** tab (no folder) for any "what can the agent see through MCP alone" evaluation. The MCP indicator is visible in both modes; the contamination is in what other tools the agent has available.

### Client: Claude Code (CLI)

Claude Code's CLI supports HTTP transport natively — no bridge needed. From any directory:

```bash
# Loopback (same machine)
claude mcp add --transport http powerlab http://127.0.0.1:9090/mcp

# LAN (separate box) — pass the Bearer header
claude mcp add --transport http powerlab http://<your-box>:9090/mcp \
    --header "Authorization: Bearer <your-token>"
```

Verify the registration:

```bash
claude mcp list                  # powerlab: http://... — ✓ Connected
claude mcp get powerlab          # full config
```

In a Claude Code session the registered server is available immediately — no restart. Note that **Claude Code is project-scoped by directory**: registrations live in `~/.claude.json` under the current project path, so the same MCP works across all sessions you open from inside that project tree.

### Client: Cursor

Cursor reads MCP servers from `~/.cursor/mcp.json` (create the file if missing):

```json
{
  "mcpServers": {
    "powerlab": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote@latest",
        "http://127.0.0.1:9090/mcp",
        "--allow-http",
        "--transport",
        "http-only"
      ]
    }
  }
}
```

For LAN, add the same `--header "Authorization: Bearer <your-token>"` args as the Claude Desktop Path B example. Restart Cursor to pick up the new config.

### Client: VS Code (with the Continue extension or any MCP-aware extension)

The Continue extension reads MCP servers from `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": [
            "-y",
            "mcp-remote@latest",
            "http://127.0.0.1:9090/mcp",
            "--allow-http",
            "--transport",
            "http-only"
          ]
        }
      }
    ]
  }
}
```

LAN variant adds the `--header` arg the same way as the other clients. Reload the Continue window after editing.

### Client: any other spec-compliant MCP client

Three pieces of config are enough for any client that follows the MCP HTTP-transport spec:

- **Endpoint**: `http://<your-box>:9090/mcp`
- **Transport**: streamable HTTP (the 2025-06-18 spec transport)
- **Auth**: nothing on loopback; `Authorization: Bearer <jwt>` on LAN

PowerLab's OAuth 2.1 discovery façade ([ADR-0050](../decisions/0050-mcp-docs-canonical-code-implementation.md) related, PR #653) advertises the right metadata at `/.well-known/oauth-protected-resource` and `/.well-known/oauth-authorization-server`, so clients that follow the spec strictly should auto-discover most of this — only the Bearer token (LAN case) is manual today.

---

## Step 3.5 — Try it: prompt cookbook (5 minutes)

Once Claude Desktop has the `powerlab` server connected, these prompts each exercise a specific capability. Use them as a starting point; copy literally or adapt to your box. None of them mutate anything (everything reachable here is read-only or — for `generate_artifact` — purely a draft for your review).

### Quick health snapshot

> *"Use the powerlab MCP and tell me how the system is doing — memory, disk, services, pending updates. If anything is concerning, surface it."*

Exercises `get_system_health`. Returns a per-category severity (`ok` / `warn` / `critical` / `unknown`) plus an overall verdict with remediation hints when a threshold trips. The agent shouldn't read four `system://*` resources separately when this single tool gives the correlated answer.

### Catalog discovery

> *"What apps are in the PowerLab catalog? Filter the list to anything that mentions 'cloud' or 'storage', and show me the docker-compose for the most popular one."*

Exercises `browse_catalog` (with the optional `filter` argument) and `catalog://app/<id>`. The catalog ships 137 curated apps; the agent should narrow then drill in.

### Authoring conventions

> *"What conventions should I follow when writing a docker-compose for PowerLab? Cover volumes, ports, labels, and what the validator rejects."*

Exercises `get_compose_conventions` (the canonical concepts doc) and indirectly informs the agent before it writes any YAML. If the agent skips this and writes legacy-CasaOS YAML (e.g. `x-casaos`, `/DATA/AppData/$AppID`, hardcoded ports), your prompt let the agent off the hook — push back with "use the powerlab MCP's compose conventions, not what you remember from CasaOS."

### Propose a new app (the headline flow)

> *"I want to self-host Vaultwarden. Use the powerlab MCP's `compose_authoring` prompt (or `start_compose_authoring` tool) to ground yourself in PowerLab conventions, then draft a docker-compose for me. Run it through `generate_artifact` so I see the deny-list validation result before I install."*

Exercises `start_compose_authoring` (the curated bundle of conventions + 3 catalog examples + validator rules) and `generate_artifact` (propose-then-review). The agent returns a structured artifact with `validation.status: "ok"` if the YAML passes the deny-list, or `"violations"` with specific rule hits. **Nothing is installed yet** — you review, then if happy, ask Claude to call `install_app` (which only works if you've opted into destructive tools, see Step 5).

### Search the docs

> *"Search PowerLab's docs for 'install_app' and tell me what the endpoint expects."*

Exercises `search_docs` across concepts + OpenAPI specs + the app catalog. Pre-2026-06-01 this only indexed concepts and would return `{matches: []}` for an OpenAPI term; now it covers all three surfaces and returns the canonical URI of each hit (the agent should chain to it for full context).

### Capability discovery

> *"Before suggesting any action, ask the powerlab MCP what it's allowed to do on this box — list capabilities."*

Exercises `list_capabilities`. Tells the agent whether destructive tools (`install_app`, `uninstall_app`, `restart_app`) are reachable and whether the sensitive sysadmin tier (host auth journal) is opted in. A well-behaved agent calls this BEFORE attempting any gated capability — saves a trial-and-error round-trip.

### Sensitive observability (opt-in only)

> *"Are there any failed SSH login attempts in the last 24 hours? If yes, summarise by source IP."*

Exercises `journal://system/auth` — only available when `EnableSensitiveTier=true` in `mcp.conf` (Step 5.5). The agent should refuse politely (or surface `list_capabilities` first) when the tier is off.

---

## Step 4 — validate a custom compose before installing (optional, ~30s)

If you intend to use `install_app` (operator opt-in — see Step 5), pre-validate your YAML against the same deny-list `install_app` runs:

```bash
/usr/bin/powerlab-mcp-validate /path/to/docker-compose.yml
# OR pipe via stdin:
cat compose.yml | /usr/bin/powerlab-mcp-validate -
```

Exit codes:
- `0` — OK, the YAML would pass the validator.
- `1` — REJECTED with one or more `[code] service — detail` violations printed.
- `2` — input parse error (malformed YAML).

---

## Step 5 — opt in to destructive tools (optional)

By default, the agent can **read** everything but can only act in bounded ways (`restart_app`). The destructive tier (`install_app`, `uninstall_app`) is NOT registered until you flip the gate:

```bash
sudo $EDITOR /etc/powerlab/mcp.conf
# Add (or uncomment):
EnableDestructiveTools = true

sudo systemctl restart powerlab-mcp
```

After restart, `tools/list` advertises the destructive pair (with `DESTRUCTIVE` clearly in their descriptions so Claude/Cursor surface the marker in the tool-use prompt).

Operator threat model:
- `install_app` runs the compose deny-list (Step 4) BEFORE forwarding to app-management. Privileged: true, docker.sock binds, host namespaces, sensitive caps → rejected at the MCP layer.
- `uninstall_app` DELETEs the app — same blast radius as the panel's Remove button.
- A panel-side "pending agent action" approval UI is roadmap. Until then `EnableDestructiveTools` is the gate.

---

## Step 5.5 — opt in to sensitive journals (optional)

By default the agent's journal access is **scope-locked to PowerLab units** (`journal://{unit}` only reaches `powerlab-*.service`). The sensitive tier (`journal://system/auth` + `journal://system/failures`, [ADR-0049](../decisions/0049-mcp-sensitive-sysadmin-tier-threat-model.md)) reads the HOST auth journals — `ssh.service` / `sshd.service` / `sudo` / `su`. It is NOT registered until you flip the gate:

```bash
sudo $EDITOR /etc/powerlab/mcp.conf
# Add (or uncomment):
EnableSensitiveTier = true

sudo systemctl restart powerlab-mcp
```

After restart, `resources/list` advertises `journal://system/auth` + `journal://system/failures`. Disable the same way: flip back to `false`, restart. Tokens do NOT need to be revoked — the resources simply vanish from the surface.

What the agent can now answer:
- "Did anyone try to log in last night?" / "Are we under an SSH brute force?"
- "Who ran `sudo` during the maintenance window, when, with what command?"
- "What auth failures hit the box in the last hour?" (`journal://system/failures`)

Operator threat model:
- **Wire shape is locked**: `{ts, unit, hostname, message}`. `_PID`, `_CMDLINE`, and `_AUDIT_SESSION` are omitted (`_CMDLINE` for sudo would routinely leak `--password=` style argvs — same name-only promise as `system://processes`).
- **MESSAGE is forwarded raw.** A `sudo command --password=hunter2` invocation that hits PAM's `LOG_INFO` path (rare, not the default) WILL surface that argument inside the message. Documented limit; if it bites in practice, a redaction layer is on the roadmap.
- **Token-compromise blast radius widens**: a leaked JWT now grants read on auth journals while the gate is on. Same JWT controls as the rest of MCP; the audit dogfood (ADR-0047) will record every read once implemented.
- **Selectors are fixed in code**: `ssh.service`, `sshd.service`, `sudo`, `su`. The agent does NOT supply units — flipping the gate is a single-switch-for-whole-tier decision (per ADR-0049 §Gate semantics; per-resource gates were considered and rejected).
- **Bounds are tighter than `journal://{unit}`**: `lines` defaults to 100 and ceilings at 500 (vs PowerLab journal's 200 / 2000). Per-call exfil if a token leaks stays small.

---

## Step 6 — troubleshooting

| Symptom | First check |
|---|---|
| `curl /healthz` fails | `sudo systemctl status powerlab-mcp` — service may be disabled (`Disabled = true` in mcp.conf) or failed to bind (port conflict). |
| Smoke client says `audit://recent permission denied` | Smoke is running as a non-root user; the file is `root:root 0600`. Use `sudo /usr/bin/powerlab-mcp-smoke` OR ignore the WARN — the service running under systemd reads it correctly. |
| Claude Desktop says "MCP server not responding" | Token expired, wrong URL, or LAN firewall. Verify `curl -H "Authorization: Bearer <token>" http://<your-box>:9090/healthz` from your Claude Desktop machine. |
| `tools/list` shows 4 not 6 | `EnableDestructiveTools = false` (the default). Step 5 to enable. |
| `resources/list` has no `journal://system/*` | `EnableSensitiveTier = false` (the default). Step 5.5 to enable. |
| `docs://api` returns empty manifest | OpenAPI staging didn't run during install — re-run `install.sh` or check `/usr/share/powerlab/openapi/` exists. |
| Want to disable MCP entirely | `Disabled = true` in `/etc/powerlab/mcp.conf` + `sudo systemctl restart powerlab-mcp`. The binary exits cleanly without binding `:9090`; systemd treats it as a clean stop. |

For deeper architectural questions (why does the service run as root? why is auth two-tier?), read [Concepts → MCP server](../concepts/mcp-server.md). For the threat model that gates `install_app`, read [ADR-0046](../decisions/0046-mcp-tool-curation-strategy.md).

---

## What MCP gives your agent today (25 resources, 11 always-on tools +2 gated, 1 prompt)

| Surface | What the agent reads / does |
|---|---|
| `system://metrics` | `/proc`-direct memory + load + uptime — always works |
| `system://utilization` | CPU% + temp + power + model + mem + net (rich) |
| `system://disk` | physical disks + per-mount + SMART (same as dashboard widget) |
| `system://network` | per-interface state + addresses |
| `system://gpu` | Apple Silicon + Nvidia GPU detection |
| `system://services` | ActiveState + SubState for every `powerlab-*` systemd unit |
| `system://kernel` | kernel release + arch + distro + boot time + virtualization |
| `system://processes` | top 10 by CPU and mem (name only — no argv leak) |
| `system://updates` | pending OS package updates (apt; security flag) |
| `journal://{unit}` | systemd logs scoped to PowerLab units |
| `journal://system/auth`, `journal://system/failures` | host auth journal (ssh, sudo, su) — **opt-in via `EnableSensitiveTier`** ([ADR-0049](../decisions/0049-mcp-sensitive-sysadmin-tier-threat-model.md)) |
| `audit://recent`, `audit://action/{id}` | HTTP request audit trail |
| `apps://list`, `apps://state/{id}/*` | installed apps + per-app state |
| `docker://logs/{id}` | container logs (MCP never touches docker socket) |
| `docker://containers`, `docker://images`, `docker://networks`, `docker://volumes`, `docker://system` | raw Docker daemon visibility — incl. non-PowerLab containers ([#630](https://github.com/neochaotic/powerlab/issues/630)) |
| `catalog://index`, `catalog://app/{id}` | 137 PowerLab-curated compose YAMLs as pattern reference ([ADR-0048](../decisions/0048-mcp-docs-surface-compose-authoring.md)) |
| `docs://api`, `docs://api/{service}` | OpenAPI specs for self-discovery |
| `docs://concepts/index`, `docs://concepts/{name}` | concept docs (compose-conventions, glossary, mcp-server, security-model) — same content this site lives in ([ADR-0048](../decisions/0048-mcp-docs-surface-compose-authoring.md)) |
| Tools (read-only, always on) | `journal_search`, `check_disk_free`, `search_docs` |
| Tools (write, opt-in) | `restart_app`, `install_app`, `uninstall_app` (last two gated by `EnableDestructiveTools`) |
| Tools (chat-mode discovery, always on) | `browse_catalog`, `get_compose_conventions`, `start_compose_authoring` — Tool form of catalog + concepts + `compose_authoring` Prompt for clients that don't autonomously surface Prompts |
| Tools (aggregator + meta, always on) | `get_system_health` (4 surfaces → one severity verdict), `generate_artifact` (propose-then-review for drafts), `list_capabilities` (which tiers are active) |
| Prompts | `compose_authoring(app_type?)` — curated bundle of conventions + 3 catalog examples + validator deny-list for compose authoring ([ADR-0048](../decisions/0048-mcp-docs-surface-compose-authoring.md)) |

[Concepts → MCP server](../concepts/mcp-server.md) has the full per-surface reference + Mermaid topology + the architecture (ADR-0044 hybrid proxy + ADR-0045 storage-agnostic + ADR-0046 tool curation + ADR-0048 docs/catalog/prompt surface + ADR-0049 sensitive tier threat model + ADR-0050 docs-canonical doctrine).
