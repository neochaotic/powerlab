# Install

PowerLab supports two install paths: a production install on Linux (the real deployment target) and a dev/demo install on macOS Apple Silicon.

## Linux — production install

Tested on Pi 4/5, Intel mini-PCs, and any amd64/arm64 Linux server with `systemd` + `docker`.

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash
```

The script:

1. Detects your distro (Debian/Ubuntu, Fedora/RHEL, openSUSE, Arch, Alpine) and uses its package manager.
2. Installs Docker if missing (with the daemon configured for PowerLab's defaults).
3. Picks an HTTP port — `8765` by default; falls back to `8766..8775`, then `:80` as a last resort. (`8765` is IANA-unassigned, so it doesn't fight common services.)
4. Installs 6 systemd units: `powerlab-gateway`, `powerlab-app-management`, `powerlab-core`, `powerlab-user-service`, `powerlab-message-bus`, `powerlab-local-storage`.
5. Starts everything and prints the URL to open.

Re-run the same command to upgrade. The script auto-detects existing installs and routes to the upgrade path.

### Coexistence with CasaOS

If the install script finds an existing CasaOS install on the host it now proceeds with a friendly notice (was a hard-block). PowerLab uses different ports, different Docker labels, and different per-app data directories — see [Coexistence with CasaOS](../coexistence/README.md).

## macOS — dev / demo

For development and previewing on Apple Silicon. NOT a production target.

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install-mac.sh | bash
```

Limitations:

- HTTPS is dev-mode only (self-signed; no trust dance).
- `/DATA` doesn't exist on macOS (SIP); a sandbox path under `/opt/powerlab/lib` is used instead.
- `bazil.org/fuse` doesn't compile on darwin, so `local-storage`'s merge feature is stubbed.

For a real PowerLab experience use Linux.

## Verifying the install

```bash
# All 6 services should be active
sudo systemctl status 'powerlab-*' --no-pager

# Health endpoint
curl -fsSL http://localhost:8765/v1/sys/health

# Open the UI
xdg-open http://localhost:8765   # Linux
open    http://localhost:8765    # macOS
```

If the UI doesn't load, see [Troubleshooting](../troubleshooting.md).

## Where things land on disk

| Path | Purpose |
|---|---|
| `/etc/powerlab/` | All service config files |
| `/var/lib/powerlab/` | Persistent state (DBs, app metadata) |
| `/var/log/powerlab/` | Service logs |
| `/var/run/powerlab/` | Runtime sockets + pidfiles |
| `/usr/bin/powerlab-*` | Service binaries (6 of them) |
| `/etc/systemd/system/powerlab-*.service` | systemd units |
| `/DATA/` | App data (Docker bind mounts; per-app subdirs) |

Per-service file path canonical layout is documented in [docs/audits/db-paths.md](../audits/db-paths.md).

## Uninstall

```bash
sudo systemctl disable --now 'powerlab-*'
sudo rm /etc/systemd/system/powerlab-*.service
sudo rm /usr/bin/powerlab-*
sudo systemctl daemon-reload
# Optionally:
# sudo rm -rf /etc/powerlab /var/lib/powerlab /var/log/powerlab /var/run/powerlab
```

App containers + their `/DATA/` volumes are NOT removed — `docker ps -a` still shows them; remove via `docker compose down` per app or via `docker rm`.
