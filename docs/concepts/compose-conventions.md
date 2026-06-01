# PowerLab compose conventions

What makes a `docker-compose.yml` *PowerLab-idiomatic*. Read this once before designing a new app or adapting one from upstream.

These conventions are what `install_app` (and the [composevalidator](https://github.com/neochaotic/powerlab) deny-list) expect. The 137 apps in `community-catalog/Apps/` follow them.

## Volume paths

**Use `/DATA/PowerLabAppData/<app_id>/<purpose>` for every named bind mount the app owns.**

```yaml
volumes:
  - /DATA/PowerLabAppData/jellyfin/config:/config
  - /DATA/PowerLabAppData/jellyfin/cache:/cache
```

Why: PowerLab's backup + restore expects user data under `/DATA/PowerLabAppData/`. Anything outside that tree is not protected. Operators add/remove storage devices via mergerfs at the `/DATA` mount point; binding outside it breaks that abstraction.

**Exceptions** (specific, documented):
- Media app exposing the operator's existing media library: bind their actual library path (`/DATA/Movies/` etc.). Do NOT copy media into `/DATA/PowerLabAppData/<id>/`.
- Hardware passthrough (`/dev/dri/renderD128` for transcoding, `/dev/snd` for audio): allowed; documented separately in security-model.md.

**Never**:
- `/etc`, `/proc`, `/sys`, `/root`, `/var/lib`, `/var/log`, `/dev` (except specific device files above), `/boot`, library + binary dirs (`/usr`, `/bin`, etc.). [composevalidator](#validator) rejects these.
- `/var/run/docker.sock` or any docker-socket bind. **Container escape; auto-rejected.**

## Networks

**Declare named networks** instead of relying on the default. PowerLab's panel renders network topology and the named entries make it readable.

```yaml
networks:
  jellyfin_default:
    driver: bridge
```

Use one network per app by default, named `<app_id>_default`. For apps that talk to each other (a stack like Sonarr+Radarr+qBittorrent), use a shared bridge with an explicit name.

**Never**:
- `network_mode: host` — bypasses isolation; auto-rejected by validator.
- `network_mode: container:<other>` — same reason.

## Ports

Declare host ports the user reaches via the panel. Default to ephemeral binding (`"0:80"`) and let PowerLab assign — operators set the user-facing port via the panel's app settings. Hardcoded host ports (`"8123:8123"`) are accepted but cause the panel's port-conflict detector to warn the operator.

```yaml
ports:
  - "0:8123"   # PowerLab assigns; user picks via panel
```

## Labels

PowerLab reads several labels for panel UI behaviour:

```yaml
labels:
  - "powerlab.app.name=Jellyfin"
  - "powerlab.app.icon=https://example.com/icon.svg"  # OR rehosted in Apps/<id>/icon.svg
  - "powerlab.app.description=Open source media server"
  - "powerlab.app.category=Media"
  - "powerlab.app.author=jellyfin"
```

The full label vocabulary is in the gateway's OpenAPI spec (read via MCP at `docs://api/gateway`). Above is the minimum panel UI needs to render a tile.

## Healthchecks

Add a healthcheck even if upstream doesn't ship one. PowerLab's app health surface (`apps://state/{id}/health` via MCP) reads it.

```yaml
healthcheck:
  test: ["CMD", "curl", "-fsS", "http://localhost:8123/api/health"]
  interval: 30s
  timeout: 5s
  retries: 3
  start_period: 60s
```

Pick `interval` ≥ 30s — sub-30s is noisy for the operator and rarely changes the operational picture.

## Capabilities + privileges

**Default to nothing.** Most apps need zero special capabilities.

```yaml
# no cap_add, no privileged, no security_opt:unconfined
```

**Allowed exceptions** (documented inline as a comment in the compose):
- `NET_RAW` for ping-based health checks (rare).
- Specific devices passed via `devices:` rather than `privileged: true`.

**Always rejected** by validator:
- `privileged: true` (container escape)
- `cap_add: [SYS_ADMIN, ALL]` and the dangerous capability set documented in [ADR-0046 §4](../decisions/0046-mcp-tool-curation-strategy.md)
- `security_opt: [apparmor:unconfined]`, `seccomp:unconfined`

## Restart policy

```yaml
restart: unless-stopped
```

PowerLab assumes apps survive reboots. `restart: no` is accepted but warns the operator; `restart: always` is fine but `unless-stopped` is the convention so an operator-stopped container stays stopped across reboots.

## Image references

**Specific tag, NEVER `:latest`.** Operators need reproducible upgrades; `:latest` makes "what version am I running?" unanswerable.

```yaml
image: jellyfin/jellyfin:10.10.5
```

For multi-arch, prefer `image: jellyfin/jellyfin:10.10.5` (Docker Hub or vendor registry) over arch-tagged variants — Docker pulls the matching arch automatically.

**Trusted registries** per memory `feedback_catalog_trust_policy`:
- Docker Official (`library/<name>`)
- `ghcr.io/<verified-org>/...`
- Vendor-published (`jellyfin/jellyfin`, `lscr.io/linuxserver/...`, etc.)
- **Never** unverified user-published images for the catalog.

## Environment variables

Declare all required env vars explicitly. Provide sensible defaults where possible; mark anything that needs operator input with a comment.

```yaml
environment:
  - TZ=${TZ:-UTC}                # default to UTC if operator does not set
  - JELLYFIN_PublishedServerUrl  # operator MUST set in panel before install
```

**Never hardcode secrets** in environment lines. Use `secrets:` (compose v3.1+) or operator-managed `.env` files for anything sensitive.

## Anti-patterns the validator rejects

The validator runs BEFORE `install_app` forwards the YAML upstream. If your design trips any of these, the install never reaches app-management:

| Pattern | Reason |
|---|---|
| `privileged: true` | Container escape — full host control |
| `network_mode: host` / `container:*` | Host namespace pollution / inter-container coupling |
| `pid: host`, `ipc: host`, `uts: host`, `userns_mode: host` | Same — namespace bypass |
| `cap_add: SYS_ADMIN/ALL/NET_ADMIN/SYS_PTRACE/...` (and 10+ more) | Container escape via specific syscalls |
| `devices: [/dev/<anything raw>]` (specific device files exempt) | Hardware passthrough escape |
| `volumes: [/var/run/docker.sock:..., /proc:..., /sys:..., /etc:..., ...]` | Sensitive host path bind |

See [ADR-0046 §4](../decisions/0046-mcp-tool-curation-strategy.md) for the full deny-list.

## A worked example

A minimum-viable PowerLab-idiomatic compose:

```yaml
services:
  helloworld:
    image: nginx:1.27.3-alpine
    container_name: helloworld
    restart: unless-stopped
    networks:
      - helloworld_default
    ports:
      - "0:80"
    volumes:
      - /DATA/PowerLabAppData/helloworld/html:/usr/share/nginx/html:ro
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    labels:
      - "powerlab.app.name=Hello World"
      - "powerlab.app.description=Static page demo"
      - "powerlab.app.category=Demo"

networks:
  helloworld_default:
    driver: bridge
```

For 137 real examples spanning every category, query the MCP `catalog://index` resource and read `catalog://app/<id>`.

## Compose extension keys (parser accepts)

PowerLab's app-management parser reads the top-level extension block under a small set of key names, in priority order. Most operators will only ever see the canonical name; the legacy aliases exist for backward compatibility with imported app catalogs.

| Key | Status | Notes |
|---|---|---|
| `x-powerlab` | **canonical** — use this for new apps | The PowerLab UI authors apps with this key; new compose YAML SHOULD use it. |
| `x-web` | legacy alias | Intermediate alias used by an upstream catalog at one point. Still parsed for backward compatibility; not for new code. |
| `x-casaos` | legacy (CasaOS-era) | What most imported CasaOS store apps ship with. Still parsed for backward compatibility; not for new code. |

Drift between this list and what the parser actually accepts is gated by `scripts/check-mcp-docs-canonical.sh` per ADR-0050 — any extension key the parser reads MUST appear in the table above.

## Where to learn more

- `mcp://catalog/index` + `mcp://catalog/app/<id>` — real PowerLab apps (machine-readable)
- `mcp://docs/api/<service>` — the REST APIs your compose interacts with
- `compose_authoring` MCP Prompt — curated bundle of these conventions + worked examples + validator rules, ready for an agent to reason from
