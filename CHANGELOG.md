# Changelog

All notable user-facing changes to PowerLab. We follow
[Semantic Versioning](https://semver.org/) — `vMAJOR.MINOR.PATCH`. While
PowerLab is in `v0.x`, breaking changes can land in MINOR bumps; from
`v1.0` onward we commit to backwards compatibility within MAJOR.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
A new entry MUST be added in the same commit as any user-visible change —
see `CONTRIBUTING.md` for the rule.

## [Unreleased]

### Added
- **In-UI updater (#21)** — Settings → About → Updates polls the
  PowerLab GitHub release manifest hourly, surfaces "Update
  available v0.x.y" with the changelog summary, and (when the user
  clicks Upgrade) downloads the tarball, verifies its SHA-256
  against the manifest, and hands off to `install.sh --upgrade`
  which:
    · Snapshots `/etc/powerlab/`, the binaries under
      `/usr/bin/powerlab-*`, the systemd units, the user DB, and
      the static UI to `/var/lib/powerlab/backups/pre-upgrade-<ts>/`
    · Stops services, swaps binaries / UI / units, starts services
    · Runs a 5-attempt health-check against the gateway port
    · On failure, restores the snapshot and restarts services
      (auto-rollback — the user does not need shell access)
    · Writes `/var/lib/powerlab/last-upgrade.json` with the result
  The UI polls `/v1/powerlab-update/status` while the upgrade runs
  and flips the banner to "Upgrade succeeded" / "Rolled back" the
  moment install.sh writes the result file.
- Release tarballs now ship a machine-readable `manifest.json`
  describing the version, per-arch SHA-256 + size, breaking
  changes, pre-install checks, and DB migrations. The host updater
  fetches this 2 KB file before the 60 MB tarball so it can decide
  whether to offer the upgrade in the first place. Format spec:
  `docs/UPDATE_MANIFEST.md`.
- **Change gateway port from the UI (#18)**. Settings → General →
  Network has a "Listen port" editor that walks the user through a
  confirmation modal, runs the bind on the new port server-side, and
  redirects the browser to `<host>:<newport>` with a 3-second
  countdown. The pre-confirm modal includes the exact shell command
  to revert if the new port is unreachable from the user's network.
  Backed by a pure-function `validateGatewayPort` boundary check
  (13-case test) and a typed frontend wrapper (8-case test) that
  rejects out-of-range ports without a network round-trip.


## [0.2.3] — 2026-05-06

### Fixed
- **mDNS `powerlab.local` not resolving on Linux installs (#33)**.
  Two root causes addressed:
  - The gateway was advertising every non-loopback IP on the host,
    including Docker bridge addresses (172.17.x.x), WireGuard / VPN
    interfaces, and Tailscale's CGNAT range (100.64/10). LAN clients
    that tried those IPs got connection-refused. The IP filter now
    keeps only RFC 1918 ranges (10/8, 172.16/12, 192.168/16) and
    IPv6 ULA (fc00::/7).
  - On Linux hosts where `avahi-daemon` already owns the IPv4
    multicast socket, the gateway's direct-multicast announcer was
    silently losing the race. The gateway now ALSO drops a
    `/etc/avahi/services/powerlab.service` XML file when
    `/etc/avahi/services/` exists. avahi picks it up via inotify
    and broadcasts on our behalf — the canonical pattern other
    well-behaved Linux daemons use. The direct-multicast path
    stays as fallback for hosts without avahi.
- New `TestIsLANRange` regression test pins the IP-filter decisions
  (Tailscale, Docker, public IPv4/IPv6, link-local) so a future
  refactor cannot quietly re-broadcast useless addresses.

## [0.2.2] — 2026-05-06

### Fixed
- **CI arm64 cross-compile** unblocked. The v0.2.1 multi-arch apt setup
  did not work on Ubuntu 24.04 GitHub runners (Deb822 sources format).
  The arm64 release tarball now builds with `CGO_ENABLED=0` for
  user-service and uses the bcrypt SetupWizard fallback for sign-in
  (tracked as #17 — native arm64 PAM via Docker buildx is the next step).

## [0.2.1] — 2026-05-06

### Changed
- **Go toolchain bumped 1.20/1.21 → 1.25** across all eight backend
  services and both CI workflows. CONTRIBUTING.md's required-version
  floor moved to 1.25 to match.

### Fixed
- Eight `fmt.Errorf(nonConstString)` call sites that Go 1.25 promoted
  from `vet` warnings to hard build errors. Replaced with
  `errors.New(...)` where the format string was just a passthrough.
  Files: `app-management/service/image.go`, both `core/drivers/{dropbox,
  google_drive}/util.go`, both `local-storage/drivers/{dropbox,
  google_drive}/util.go`.
- `core/service/notify.go::notifyServer.GetList` had a value receiver
  on a type embedding `syncmap.Map` (sync.Mutex-bearing). 1.25 vet now
  refuses to copy locks; switched to pointer receiver. Same fix for
  `GetSystemTempMap()` which was returning the map by value.

## [0.2.0] — 2026-05-06

### Added
- **Native Linux PAM authentication** (`amd64` only — see #17). Sign in
  with the same username and password you use for `sudo` / `ssh`. PAM
  is delegated to libpam at runtime via CGO + `github.com/msteinert/pam`,
  so PowerLab inherits whatever hash algorithm the distro chose
  (yescrypt, SHA-512, bcrypt, …).
- `/etc/pam.d/powerlab` policy installed by `install.sh` on first run.
  Minimal `pam_unix` only — no pam_nologin / pam_securetty / MOTD bag.
  Idempotent: existing file is left untouched on upgrades so admin edits
  (faillock, 2fa, …) survive.
- **Auto-versioned UI**: Vite reads `ui/package.json` at build time and
  injects `__APP_VERSION__` so the LoginScreen footer always matches
  the released version.
- **Path constants split per platform** (`paths_linux.go`,
  `paths_darwin.go`) — the macOS production install path is wired up,
  pending the rest of the macOS production work tracked in #10.

### Changed
- Linux SUPPORT matrix: `amd64` shows ✅ **OS credentials (PAM)**;
  `arm64` shows ⚠️ Setup Wizard fallback until #17 lands.
- Login handler now distinguishes `(false, nil)` (PAM rejected the
  credential) from `(false, err)` (PAM unavailable). Wrong-password
  responses no longer fall through to the bcrypt code path, which
  removes a confusing "OS authentication unavailable" message and
  closes a subtle information leak about whether a SetupWizard
  password was configured.

### Build pipeline
- `scripts/package-linux.sh` compiles user-service with
  `CGO_ENABLED=1` on amd64 (no-op on arm64). `POWERLAB_SKIP_FRONTEND_BUILD=1`
  env var lets the script reuse an existing `ui/build/` so build
  containers without Node 20+ can still produce tarballs.
- CI installs `libpam0g-dev` on the user-service backend job and the
  amd64 package job.

## [0.1.6] — 2026-05-06

### Added
- **Install bootstrappers** — `install.sh` (Linux production) and
  `install-mac.sh` (macOS dev). One-liner installs:
    `curl -fsSL .../install.sh | sudo bash`
  Idempotent — re-run any time to upgrade. Auto-detects amd64 / arm64.
  `--version vX.Y.Z` to pin a specific release.

### Fixed
- `install.sh` no longer silently moves the gateway port on upgrade.
  Pre-existing `/etc/powerlab/gateway.ini` is now respected
  unconditionally; only fresh installs probe for a free port.
- Services are stopped *before* the port probe so the probe sees the
  real host state, not our own gateway holding the configured port.
- The legacy `cd powerlab-*-linux-amd64` glob expansion failure
  (multiple matched dirs after a re-download) is gone — the
  bootstrapper extracts into a sandboxed temp dir.

### Changed
- Default gateway port is now **8765** (IANA-unassigned, no Chrome
  HTTPS-First quirk). Falls back to 8766..8775, then 80 last-resort.
- LoginScreen footer linkified to the maintainer's GitHub profile.

## [0.1.5] — 2026-05-06

### Added
- **Premium favicon** — squircle "P." wordmark with emerald accent dot
  matching the Launchpad. Single SVG source rasterised to 32 / 180 / 192
  / 512 PNG via `scripts/rasterize-favicon.mjs`.

## [0.1.4] — 2026-05-06

### Fixed
- **Reverted the broken Linux auth** that almost shipped (`unix_chkpwd`
  silently returns exit 0 for invalid passwords when called outside
  pam_unix — full password bypass). Linux returns to a stub error and
  routes users to the bcrypt SetupWizard. Native PAM lands in v0.2.0.
- Re-enabled SetupWizard in the auth flow so first-run on Linux works
  again.

### Added
- `SUPPORT.md` — per-distro support matrix, hardware tier guidance,
  the rationale for deferring PAM rather than shipping a half-secure
  shell-out.

## [0.1.3] — 2026-05-06

### Added
- **Auto-port selection on install** — probes 80 / 8765 / 8766..8775,
  picks the first free one, writes it into gateway.ini, threads the
  chosen port through the end-of-install banner.
- **Self-heal of broken systemd units** — strips the bogus
  `-c /etc/powerlab/gateway.conf` flag from older releases on every
  install. Re-running `install.sh` recovers any host that got stuck
  in the v0.1.0 / v0.1.1 restart-loop.

## [0.1.2] — 2026-05-06

### Fixed
- Gateway systemd unit dropped the bogus `-c` flag the binary did not
  accept. The gateway no longer loops on startup with
  `status=2/INVALIDARGUMENT`.

## [0.1.1] — 2026-05-06

### Fixed
- Gateway, app-management, and `constants/paths.go` no longer
  unconditionally rewrite RuntimePath / LogPath to `<cwd>/../runtime`
  in production. Under systemd `cwd` is `/`, which made every prod
  binary write `routes.json` and PIDs to `/runtime/` instead of
  `/var/run/powerlab/`. Wrapped behind a `devmode.IsDev()` check
  (probes for `/etc/powerlab` or `/etc/casaos` — production markers).

## [0.1.0] — 2026-05-05

### Added

Initial public release. Highlights:

- SvelteKit SPA frontend on top of a Go backend forked from CasaOS.
- Launchpad with iOS-style icon design.
- 300+ Docker apps in a curated catalogue with auto-port remap and
  live install logs over SSE.
- Custom App Builder with bidirectional YAML/form sync.
- Dashboard with radial gauges (CPU/RAM/GPU), dual sparklines for
  network, disk-by-disk usage. EMA-smoothed at 1Hz.
- Files manager with virtualised scroll, side-panel preview (image,
  video, audio, PDF, text), drag-and-drop chunked upload, inline
  CodeMirror editor.
- Local pseudo-terminal (no SSH config required).
- mDNS announcer publishing the box at `powerlab.local`.
- macOS dev-mode auth via `dscl`.
- License: AGPL-3.0.
