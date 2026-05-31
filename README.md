<div align="center">

<br>

<img src="docs/img/login.png" alt="PowerLab login screen" width="100%" />

<br>
<br>

# PowerLab

### One pane of glass for everything you self-host.

Your apps. Your files. Your AI. Your home server, finally beautiful.

<br>

### ⚡  Get started in 60 seconds

**Linux** (Pi 4/5, Intel mini-PC, any amd64/arm64 server) — production install:

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash
```

**macOS** (Apple Silicon) — dev / demo mode:

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install-mac.sh | bash
```

**Then open the URL the installer prints from any device on the network.**

<sub>Idempotent — re-run any time to upgrade. Source build → see <a href="#install">Install</a> & <a href="#develop">Develop</a> below.</sub>

<br>

[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-emerald?style=flat-square)](LICENSE)
[![AI-ready](https://img.shields.io/badge/AI-ready-blueviolet?style=flat-square&logo=openai&logoColor=white)](#built-for-ai)
[![Built with SvelteKit](https://img.shields.io/badge/built_with-SvelteKit-FF3E00?style=flat-square&logo=svelte&logoColor=white)](https://kit.svelte.dev)
[![Backend: Go 1.25+](https://img.shields.io/badge/backend-Go_1.25+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![CI](https://img.shields.io/github/actions/workflow/status/neochaotic/powerlab/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/neochaotic/powerlab/actions)
[![Pre-release](https://img.shields.io/badge/status-pre--release-amber?style=flat-square)]()

<br>

[Install](#install) · [Tour](#a-tour) · [App Store](#three-hundred-apps-one-click) · [AI](#built-for-ai) · [Compatibility](#compatibility) · [Architecture](#architecture) · [Develop](#develop)

</div>

<br>

---

## A new home for your home server.

Open a browser. Type `powerlab.local`. There it is — every container, every gigabyte, every blinking GPU, on one screen designed to feel like the rest of your devices.

Built on a battle-tested Go core. Wrapped in a SvelteKit interface tuned to the millisecond. PowerLab brings the polish of a finished product to the corner of the room you used to apologise for.

<br>

## Designed for the way you live with your hardware.

- **Open formats from end to end.** Apps are plain Docker Compose. Your data lives in a folder you can `cd` into. Nothing is proprietary, nothing is locked away.
- **Sign in with the password you already know.** PowerLab uses your operating-system credentials. One identity, one less thing to forget.
- **Quiet by default.** Dark theme, considered typography, animations that respect attention. The panel does its job and gets out of the way.
- **Reachable everywhere on your LAN.** mDNS announces the box at `powerlab.local` automatically — wifi, ethernet, any device, no IP juggling.
- **Green padlock, no public DNS.** PowerLab provisions a private CA on first boot and signs its own leaf certificate covering `powerlab.local`, the host's LAN addresses, and `localhost`. One-tap trust install on iOS/macOS via a signed `.mobileconfig`; raw `.crt` for everyone else. HSTS only arms after the trust dance is verified end-to-end, so you can never be locked out of your own server. See the [HTTPS guide](docs/HTTPS.md).

<br>

---

## A tour

<table>
<tr>
<td width="50%">
<img src="docs/img/login.png" alt="Login screen" width="100%" /><br>
<sub><b>Lock screen.</b> Sign in with your computer username and password. The clock greets you. There's nothing else.</sub>
</td>
<td width="50%">
<img src="docs/img/launchpad.png" alt="Launchpad" width="100%" /><br>
<sub><b>Launchpad.</b> Every native tool and every installed app, on one screen. Drag to reorder. Long-press for the per-tile menu.</sub>
</td>
</tr>
<tr>
<td width="50%">
<img src="docs/img/dashboard.png" alt="Dashboard" width="100%" /><br>
<sub><b>Dashboard.</b> Radial gauges for CPU, RAM, GPU. Dual sparklines for network. Disk-by-disk usage. Updated every second, smoothed so it never flickers.</sub>
</td>
<td width="50%">
<img src="docs/img/files.png" alt="File manager" width="100%" /><br>
<sub><b>Files.</b> Virtualised for ten thousand entries. Side-panel preview that plays video, audio, PDFs. Drop a folder anywhere on the page to upload. CodeMirror opens text files in place.</sub>
</td>
</tr>
<tr>
<td width="50%">
<img src="docs/img/apps.png" alt="App store" width="100%" /><br>
<sub><b>App Store.</b> Three hundred curated apps. One click to install. Live install logs. Auto-port remap. Fork any app into your own with a tap.</sub>
</td>
<td width="50%">
<img src="docs/img/about.png" alt="About / Settings" width="100%" /><br>
<sub><b>About.</b> Version, license, the stack we built on, links to source. Settings is for settings.</sub>
</td>
</tr>
</table>


<br>

---

## Three hundred apps. One click.

A curated catalogue of **300+ ready-to-install Docker apps**, organised by category, filtered by what your hardware can run, installable in a single tap.

<br>

| Category | A glimpse |
|---|---|
| **Media** | Plex · Jellyfin · Emby · Navidrome · Audiobookshelf · Calibre-web · Bazarr |
| **Files & Sync** | Nextcloud · Syncthing · Filebrowser · Duplicati · AList · CopyParty |
| **Network & VPN** | AdGuard Home · Pi-hole · WireGuard · Tailscale · Cloudflared · DDNS-Updater |
| **Productivity** | Vikunja · Wallabag · Bookstack · Memos · Beaver Habit Tracker |
| **AI & ML** | Ollama · ChatGPT-Next-Web · AnythingLLM · ChatbotUI · Open WebUI |
| **Database** | Adminer · CloudBeaver · Memcached · Redis · Postgres |
| **Finance** | Actual Budget · 2FAuth · Vaultwarden |
| **Developer** | Gitea · Forgejo · Drone · Verdaccio · Code-Server |
| **Smart home** | Home Assistant · Frigate · Node-RED · Mosquitto |

Behind every install: PowerLab quietly handles port collisions, streams the install logs in real time, surfaces compatibility warnings before the pull starts, and remembers everything in a clean local YAML you can read.

> **The catalog lives in its own repo: [neochaotic/powerlab-store](https://github.com/neochaotic/powerlab-store).**
> Independent product, independent release cadence. Every app passes a strict security gate (no `hooks/`, no `exports.sh`, no privileged mounts, digest-pinned images, rehosted icons) before merging. Want to **add an app** or **report a catalog bug**? Go there. Want to understand the **architecture decision** to split the catalog out? See [ADR-0041](docs/decisions/0041-powerlab-store-separate-repo.md).

<br>

---

## Build your own. Right inside the panel.

Not in the catalogue? Build it.

The **Custom App Builder** is a visual editor for Docker Compose, with the YAML always open beside it. Touch a field, the YAML updates. Edit the YAML, the form follows. Pick the side you prefer.

- **Smart fields.** Memory limits as sliders. Port mappings validated against the host *before* you deploy. Volume mounts that recognise privileged-folder requirements.
- **Pre-flight check.** Every port you publish gets probed. If something is busy, PowerLab suggests an alternative — and hands you the keyboard so you can choose.
- **Fork in one click.** Any store app can be forked into a Custom App. Tweak the image tag. Swap a volume path. Add an environment variable. The original stays pristine.
- **Yours, in plain YAML.** Custom apps live as `docker-compose.yml` files under `$AppsPath/<name>/`. Version them in git. Share them. Move them. There is no proprietary format to escape.

The full power of Compose. None of the friction.

<br>

---

## Built for AI.

PowerLab was designed with local AI in mind. The same Compose-native runtime that hosts your media library happily hosts **Ollama, Stable Diffusion WebUI, ChatGPT-Next-Web, AnythingLLM, ChatbotUI, Open WebUI, Whisper.cpp, ComfyUI** — every popular self-hosted AI tool ships as a Docker image and PowerLab knows how to install it.

> **GPU monitoring is first-class — and not standard in this category.**
> Most homelab panels still treat the GPU as an afterthought. PowerLab's
> Dashboard renders live VRAM, GPU utilization, and temperature on
> Apple Silicon (M-series via `ioreg`) and Nvidia (via `nvidia-smi`),
> auto-detected on first boot. CasaOS doesn't ship this. ZimaOS, the
> paid sibling, doesn't either. If you're running local models, the
> blinking GPU on the Dashboard is the difference between "is this thing
> even using the GPU?" and a real-time signal you can act on.

What makes the AI experience effortless:

- **GPU detection, automatic.** Apple Silicon (M-series via `ioreg`) and Nvidia (via `nvidia-smi`) appear on the Dashboard the moment you open it. No drivers to chase, no config files to edit.
- **Memory and VRAM, live.** Telemetry refreshes every second. Watch a 7B model load in real time, see how much VRAM your prompt is using, know when to scale down.
- **The catalogue knows AI.** Search the App Store for "Ollama", "AnythingLLM", "ChatGPT-Next-Web" — install in one click, ports remapped, logs streaming.
- **Designed for the lab on your shelf.** Quiet GPU rigs, Apple Silicon Macs, Nvidia Jetson, Intel mini PCs. PowerLab feels at home on the hardware you already trust.

> **Coming soon: a first-class Models tab.** Drag-and-drop GGUF imports. One-click Ollama pulls. Side-by-side benchmarks. Quantization presets. The future of local AI, with the polish of a real product.

But the bigger AI story is the next section — your server itself becoming a first-class resource your agents can read.

<br>

---

## Talk to your server. Talk to your stack.

PowerLab ships a built-in **MCP (Model Context Protocol) server** at `:9090`. Point Claude Desktop, Cursor, or Claude Code at it and your agent reads your Postgres container, your Caddy proxy, your nightly backup, your journald, your audit trail, your SMART data, and the entire PowerLab OpenAPI surface — the same data the dashboard shows you, exposed over the official MCP transport.

The UI is the pane of glass **for you.** MCP is the pane of glass **for your agent.** Same data, two surfaces. One Pi in a closet, one server in a colo, or a fleet across both — the contract is identical.

**Enterprise-acceptable by construction**: every MCP call carries the operator's JWT and lands in the same JSONL audit trail as a UI click (correlation id and all); write tools are off by default and gated behind `EnableDestructiveTools` in `/etc/powerlab/mcp.conf`; custom compose YAMLs hit a deny-list validator **before** app-management ever sees them. The threat model is documented in [ADR-0046](docs/decisions/0046-mcp-tool-curation-strategy.md) and [ADR-0049](docs/decisions/0049-mcp-sensitive-sysadmin-tier-threat-model.md), not implied.

What an agent can do today — **25 advertised resources + 1 MCP Prompt + 4 always-on tools (+2 destructive when opted in)**:

- **`system://`** (10) — CPU, memory, load, uptime, **disk + SMART, network, GPU (Apple Silicon + Nvidia), temperature, kernel + OS identity, systemd services, processes (name-only, no argv leakage), pending OS updates**. Mix of `/proc` direct + thin-proxy to core's `/v1/sys/*`. Same data the dashboard widgets read.
- **`journal://`** (3 + 2 opt-in) — PowerLab service logs (`journal://gateway?lines=200`, `journal://core`, `journal://mcp`, ...). Scope-locked to PowerLab units by default. An opt-in sensitive tier (`journal://system/auth`, `journal://system/failures`, [ADR-0049](docs/decisions/0049-mcp-sensitive-sysadmin-tier-threat-model.md)) reads host SSH / sudo / su via a single `EnableSensitiveTier` flag — fixed selectors in code, never agent-supplied.
- **`audit://`** — newest audit entries, plus filter by correlation id (everything one request triggered).
- **`apps://`** — installed-apps manifest + per-app state, containers, health, stats, disk footprint. Thin-proxy through app-management.
- **`docker://`** — `docker://logs/<app>` for container logs PLUS raw daemon visibility (`docker://containers`, `images`, `networks`, `volumes`, `system`) so the agent sees non-PowerLab containers too ([#630](https://github.com/neochaotic/powerlab/issues/630)). **MCP never touches the Docker socket** — everything proxies through app-management (security win — see [ADR-0045](docs/decisions/0045-mcp-apps-docker-via-app-management-http-proxy.md)).
- **`catalog://`** ([ADR-0048](docs/decisions/0048-mcp-docs-surface-compose-authoring.md)) — 137 PowerLab-curated compose YAMLs the agent reads as pattern reference (NOT a list to install). `catalog://index` + `catalog://app/<id>`.
- **`docs://`** — `docs://api` for OpenAPI self-discovery PLUS `docs://concepts/*` for the prose docs (`compose-conventions`, `glossary`, `mcp-server`, `security-model`) — same content this page lives in, machine-readable.

Plus the **`compose_authoring` MCP Prompt** ([ADR-0048](docs/decisions/0048-mcp-docs-surface-compose-authoring.md)) — one `prompts/get` invocation returns a curated 6-message bundle: framing, the compose-conventions doc, 3 worked catalog examples, and the validator deny-list. Replaces N discovery round-trips when an agent designs a new PowerLab compose YAML. Optional `app_type` argument (`database`, `media`, `ai`, `dashboard`) tunes which catalog examples ship.

Plus **4 always-on MCP tools + 2 destructive-gated** (per [ADR-0046](docs/decisions/0046-mcp-tool-curation-strategy.md)):

- **READ ONLY** — `journal_search` (literal substring + time-range over PowerLab journals); `check_disk_free` (one-path statfs); `search_docs` (case-insensitive substring across the concepts docs — returns `{concept, line_number, snippet, uri}` per hit).
- **SIDE EFFECT (bounded)** — `restart_app` (cycle one app's containers; same end-state).
- **DESTRUCTIVE (operator opt-in)** — `install_app` (custom Compose, validated against a deny-list of dangerous patterns before app-management ever sees it) and `uninstall_app`. Both require `EnableDestructiveTools = true` in `/etc/powerlab/mcp.conf` — when false (the default), they're NOT registered and the agent can't call them.

What it does **not** do yet:
- **No autonomous destructive defaults.** `install_app` + `uninstall_app` exist but ship NOT REGISTERED until the operator flips `EnableDestructiveTools = true`. Default-on would be a surprise; default-off is a documented opt-in.
- **No panel-side approval UI** — once destructive tools are enabled, the agent can install/uninstall apps without per-action human confirmation. The "pending agent action" approval surface is roadmap; until then `EnableDestructiveTools` is the gate.
- **No automatic pairing.** Today you paste a JWT into your client config; a `powerlab pair` CLI is roadmap.
- **No internet exposure.** Binds `:9090` on the LAN with two-tier auth (loopback free, LAN needs your PowerLab token); PowerLab does not configure port-forwarding for you.
- **No RBAC.** Any authenticated agent has full access — every PowerLab user is hardcoded admin today. Real role-based access is tracked separately.

**Storage-agnostic by construction.** When PowerLab eventually migrates from SQLite to PostgreSQL, MCP requires zero changes — the HTTP contract is the abstraction.

**Opt out anytime** — flip `Disabled = true` in `/etc/powerlab/mcp.conf` and restart the unit. The binary exits cleanly without binding `:9090`.

**30-second smoke test** — verify MCP is alive without touching a client:

```bash
curl -fsS http://localhost:9090/healthz                              # → 200 OK
curl -fsS http://localhost:9090/version | jq                         # → {"version":"...","commit":"..."}
sudo systemctl status powerlab-mcp --no-pager | head -3              # → active (running)
sudo journalctl -u powerlab-mcp -n 20 --no-pager                    # → boot + bind log
```

Want a structured contract sweep? `powerlab-mcp-smoke` (shipped with the install) reads every advertised resource + exercises read-only tools end-to-end:

```bash
/usr/share/powerlab/bin/powerlab-mcp-smoke -endpoint http://localhost:9090
```

Custom compose YAML to validate before reaching `install_app`? Use the standalone CLI:

```bash
/usr/share/powerlab/bin/powerlab-mcp-validate /path/to/docker-compose.yml
```

Full architecture + Claude Desktop / Cursor / Code wire-up + operator quickstart in the [MCP server docs](https://neochaotic.github.io/powerlab/concepts/mcp-server/) and the [MCP operator quickstart](https://neochaotic.github.io/powerlab/operations/mcp-quickstart/).

<br>

---

## Install

<details>
<summary><b>One-liner installer (recommended)</b></summary>

<br>

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash
```

Auto-detects amd64 / arm64, downloads the matching tarball, runs the bundled installer, cleans up. Re-run any time to upgrade.

Pin a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash -s -- --version v0.5.11
```

</details>

<details>
<summary><b>Inspect-first, then run (no <code>curl | bash</code>)</b></summary>

<br>

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh -o install.sh
less install.sh                        # read what it does
sudo bash install.sh                   # then run
```

</details>

<details>
<summary><b>Manual tarball install</b></summary>

<br>

If you would rather download and extract by hand. Replace `ARCH` with `amd64` or `arm64`:

```bash
curl -fL -o /tmp/powerlab.tar.gz \
  https://github.com/neochaotic/powerlab/releases/latest/download/powerlab-linux-ARCH.tar.gz
mkdir -p /tmp/powerlab-install
tar -xzf /tmp/powerlab.tar.gz --strip-components=1 -C /tmp/powerlab-install
sudo /tmp/powerlab-install/install.sh
```

The installer creates `/etc/powerlab`, `/var/lib/powerlab`, `/var/log/powerlab`, `/var/run/powerlab`, and `/DATA/AppData`, then registers and starts six systemd services. The end-of-install banner prints the URL to open in your browser.

</details>

<details>
<summary><b>macOS dev mode (Apple Silicon)</b></summary>

<br>

PowerLab is a Linux-first product — production deployments target Pi / mini-PC / arm64 boxes. On macOS we ship a **dev-mode bootstrap** that clones the repo into `~/Documents/powerlab` and runs the same SvelteKit + Go stack locally:

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install-mac.sh | bash
```

Use this for development, demos, or kicking the tires. Caveats:

- The Files page is disabled (the `local-storage` service depends on Linux fuse + xattr).
- Nothing auto-starts at boot — you keep the terminal open while `dev.sh` runs.
- Auth uses `dscl . -authonly` against your Mac's Directory Service, so you sign in with your computer username + password directly (no Setup Wizard).

Requires Homebrew, `git`, `go`, `node`, and Docker Desktop.

For real production, install on Linux instead.

</details>

<details>
<summary><b>Build from source</b></summary>

<br>

Requires **Go 1.25+**, **Node.js 20+**, **Docker Engine**.

```bash
git clone https://github.com/neochaotic/powerlab.git
cd powerlab
./scripts/package-linux.sh amd64        # or: arm64
sudo ./dist/powerlab-*-linux-amd64/install.sh
```

</details>

<br>

---

## Develop

One command, the whole stack:

```bash
git clone https://github.com/neochaotic/powerlab.git
cd powerlab
./dev.sh
```

`dev.sh` checks your prerequisites, installs UI dependencies on first run, builds and starts every backend service, then launches the Vite dev server. Stop everything with **Ctrl-C** and it tears the stack down cleanly. Pass `--no-build` to skip the backend rebuild for faster restarts, or `--stop` to shut everything down.

The dev gateway listens on port 80; the UI dev server runs at `localhost:5173` and proxies API calls to it. The Files page is unavailable in macOS dev mode (`local-storage` requires Linux fuse + xattr — that service is skipped automatically).

<details>
<summary><b>Tests</b></summary>

<br>

```bash
# Frontend
cd ui
npx svelte-check        # type check
npx vitest run          # unit tests
npm run build           # production build

# Backend (each service has its own go.mod)
cd backend/<service>
go generate ./...       # produces codegen/ from OpenAPI spec
go test -race ./...
```

CI runs all of the above on every push to `main` (`.github/workflows/ci.yml`).

</details>

<br>

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│  Browser (any device on the LAN)                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  SvelteKit SPA (adapter-static, no SSR)              │  │
│  │  Svelte 5 Runes · Tailwind v4 · Lucide · CodeMirror  │  │
│  └────────────────────────┬─────────────────────────────┘  │
└───────────────────────────┼────────────────────────────────┘
                            │  HTTPS / WSS
                            ▼
┌────────────────────────────────────────────────────────────┐
│  Gateway   :8765 (HTTP) / :8443 (HTTPS opt-in)              │
│  · JWT auth · embedded UI · WebSocket bridge                │
│  · mDNS announcer (powerlab.local)                          │
└──┬──────────┬──────────┬──────────┬──────────┬──────────┬──┘
   ▼          ▼          ▼          ▼          ▼          ▼
 core    user-svc   message-bus  app-mgmt   local-store  cli
 sys     auth       SSE          Docker     filesystem   tools
 telemetry          fan-out      Compose
```

Six independent Go services, each with its own `go.mod` and codegen pipeline so they evolve independently. The gateway routes `/v1/*` and `/v2/*` to the right service based on a `routes.json` it rebuilds at every boot.

Plus two standalone binaries that don't run as long-lived services:

- **`powerlab-sync-catalog`** — keeps `/var/lib/powerlab/community-catalog/` fresh against the upstream Umbrel repo. Runs once post-install + on demand.
- **`powerlab-logs`** — diagnostic survival CLI. Surfaces the systemd journal, Docker container logs, and install/upgrade transcripts without depending on any PowerLab daemon. When the gateway is down, this is the binary you SSH in and run. See [`docs/operations/powerlab-logs.md`](docs/operations/powerlab-logs.md) for the full reference; architecture in [`docs/architecture/log-aggregation.md`](docs/architecture/log-aggregation.md).

<br>

---

## Compatibility

| Platform | Status | Sign-in |
|---|---|---|
| **Ubuntu** 20.04 / 22.04 / 24.04 LTS · `amd64` `arm64` | ✅ Supported | OS credentials (PAM) |
| **Debian** 11 / 12 · `amd64` `arm64`                   | ✅ Supported | OS credentials (PAM) |
| **Raspberry Pi OS** Bookworm / Bullseye · `arm64`      | ✅ Supported | OS credentials (PAM) |
| **Fedora** 38+ · **Arch** · **openSUSE** · `amd64`      | ⚠️ Untested, expected to work | OS credentials (PAM) |
| **Alpine** · `amd64` `arm64`                            | ❌ Out of scope (musl + OpenRC) | — |
| **macOS** Sonoma+ · `arm64`                             | ✅ Dev mode (`./dev.sh`) | OS credentials |
| **Windows**                                            | ❌ Not planned | — |

**Sign in with your operating-system credentials** — the same username and password you use for `sudo` / `ssh` on Linux, or to log in to your Mac. PowerLab uses `pam_unix` on Linux and `dscl . -authonly` on macOS, both delegating the actual hash check to the OS so we never need to mirror your shadow file. A bcrypt **Setup Wizard** is shown only if PAM is unavailable on the host (CGO disabled at build time, missing libpam) — it stays around as a recovery fallback.

JWTs are signed with the gateway's ECDSA key, rotated on first boot. Tokens last about three hours; the session cookie persists across reloads.

See **[SUPPORT.md](./SUPPORT.md)** for the deep matrix — hardware tiers, distro testing methodology, the rationale for deferring PAM, and how to report new compatibility results.

<br>

---

## License

**[GNU Affero General Public License v3.0](LICENSE).** Free and open-source software. You can use, modify, and redistribute PowerLab — including for commercial purposes — provided that any modified version you distribute (or host as a network service) is also released under the AGPL-3.0. See the [LICENSE](LICENSE) file for the full text.

<br>

---

<div align="center">

<sub>Crafted by <a href="https://github.com/neochaotic">neochaotic</a> · <a href="https://github.com/neochaotic/powerlab/issues">Report an issue</a> · <a href="https://github.com/neochaotic/powerlab/discussions">Discussions</a> · <a href="https://github.com/neochaotic/powerlab-store">App catalog repo</a></sub>

</div>
