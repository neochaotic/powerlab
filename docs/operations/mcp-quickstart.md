# MCP — operator quickstart

> Your homelab now has an agent-ready surface. This page is the **5-minute path from "is it running?" to "Claude is reading my journals"**. If you want the architecture (how it's wired, why, the security model), read [Concepts → MCP server](../concepts/mcp-server.md) instead.

## At a glance

PowerLab ships a built-in **MCP (Model Context Protocol) server**. It runs as `powerlab-mcp.service` on `:9090`. Your AI agent connects to it and reads the same data the panel dashboard shows you — metrics, journals, audit trail, installed apps, container logs, the entire PowerLab OpenAPI surface.

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
/usr/share/powerlab/bin/powerlab-mcp-smoke -endpoint http://localhost:9090
```

You should see something like:

```
PASS  /healthz + /version
PASS  mcp connect + initialize
PASS  resources/list (16 advertised)
PASS  system://metrics                        → 8 fields sane
PASS  system://utilization                    → proxied 903 bytes
PASS  apps://list                             → proxied 18245 bytes
PASS  audit://recent?limit=5                  → 5 records with valid ts/status/method
PASS  tools/list (3 advertised)
PASS  journal_search (unit=gateway, 10 entries)
PASS  check_disk_free / (77% used, 54 GiB available)
```

Any FAIL is actionable. The most common one — `audit://recent permission denied` — only happens when smoke runs as a non-root user against the production `/var/log/powerlab/audit.jsonl` (which is `root:root 0600`). The service itself, running as root via systemd, reads it fine.

---

## Step 3 — pair Claude Desktop (3 minutes)

Get a token first. **From a host with PowerLab UI access**:

1. Sign in to PowerLab (`http://<your-box>:8765`).
2. Open the browser dev tools → Network tab.
3. Refresh any page in the panel. Find any API request and copy its `Authorization: Bearer <token>` header.
4. (Pairing UX is roadmap — until then this is the manual path.)

Add the MCP server to Claude Desktop's config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "powerlab": {
      "transport": {
        "type": "streamable-http",
        "url": "http://<your-box>:9090/mcp",
        "headers": {
          "Authorization": "Bearer <your-token>"
        }
      }
    }
  }
}
```

Restart Claude Desktop. The `powerlab` server should appear in the MCP indicator. Ask Claude:

> *"What's running on my homelab? Check disk and any failed services."*

Claude reads `apps://list`, `system://services`, `system://disk` and answers with real data.

---

## Step 4 — validate a custom compose before installing (optional, ~30s)

If you intend to use `install_app` (operator opt-in — see Step 5), pre-validate your YAML against the same deny-list `install_app` runs:

```bash
/usr/share/powerlab/bin/powerlab-mcp-validate /path/to/docker-compose.yml
# OR pipe via stdin:
cat compose.yml | /usr/share/powerlab/bin/powerlab-mcp-validate -
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

## Step 6 — troubleshooting

| Symptom | First check |
|---|---|
| `curl /healthz` fails | `sudo systemctl status powerlab-mcp` — service may be disabled (`Disabled = true` in mcp.conf) or failed to bind (port conflict). |
| Smoke client says `audit://recent permission denied` | Smoke is running as a non-root user; the file is `root:root 0600`. Use `sudo /usr/share/powerlab/bin/powerlab-mcp-smoke` OR ignore the WARN — the service running under systemd reads it correctly. |
| Claude Desktop says "MCP server not responding" | Token expired, wrong URL, or LAN firewall. Verify `curl -H "Authorization: Bearer <token>" http://<your-box>:9090/healthz` from your Claude Desktop machine. |
| `tools/list` shows 3 not 5 | `EnableDestructiveTools = false` (the default). Step 5 to enable. |
| `docs://api` returns empty manifest | OpenAPI staging didn't run during install — re-run `install.sh` or check `/usr/share/powerlab/openapi/` exists. |
| Want to disable MCP entirely | `Disabled = true` in `/etc/powerlab/mcp.conf` + `sudo systemctl restart powerlab-mcp`. The binary exits cleanly without binding `:9090`; systemd treats it as a clean stop. |

For deeper architectural questions (why does the service run as root? why is auth two-tier?), read [Concepts → MCP server](../concepts/mcp-server.md). For the threat model that gates `install_app`, read [ADR-0046](../decisions/0046-mcp-tool-curation-strategy.md).

---

## What MCP gives your agent today (16 resources, 5 tools)

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
| `audit://recent`, `audit://action/{id}` | HTTP request audit trail |
| `apps://list`, `apps://state/{id}/*` | installed apps + per-app state |
| `docker://logs/{id}` | container logs (MCP never touches docker socket) |
| `docs://api`, `docs://api/{service}` | OpenAPI specs for self-discovery |
| Tools | `journal_search`, `check_disk_free`, `restart_app`, `install_app` (opt-in), `uninstall_app` (opt-in) |

[Concepts → MCP server](../concepts/mcp-server.md) has the full per-surface reference + Mermaid topology + the architecture (ADR-0044 hybrid proxy + ADR-0045 storage-agnostic + ADR-0046 tool curation).
