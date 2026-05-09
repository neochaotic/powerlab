# Data persistence map

Every PowerLab on-disk path, what owns it, and how upgrades preserve
or migrate it.

## Layout

```mermaid
flowchart TD
    subgraph etc[/etc/powerlab/]
        SECURITY[security/<br/>CA cert + private key + leaf<br/>chmod 0600 ca.key]
        HSTS[.hsts-armed<br/>presence = HSTS active]
        VERSION[version<br/>VERSION=0.3.2]
        CONF1[gateway.ini]
        CONF2[app-management.conf]
        CONF3[user-service.conf]
        CONF4[message-bus.conf]
        CONF5[local-storage.conf]
    end

    subgraph var[/var/lib/powerlab/]
        DB[db/<br/>SQLite databases per service]
        APPS[apps/<br/>Compose YAML staging]
        APPSTORE[appstore/<br/>Cached app store data]
        BACKUPS[backups/<br/>pre-upgrade-TIMESTAMP/]
    end

    subgraph data[/DATA/]
        APPDATA[AppData/<br/>App-specific volumes<br/>shared with CasaOS today]
    end

    subgraph runtime[/var/run/powerlab/]
        UDS[*.sock<br/>UDS files for inter-service]
        URL[*.url<br/>Boot-order sentinels]
    end

    subgraph share[/usr/share/powerlab/]
        WWW[www/<br/>SPA static bundle]
        SHELL[shell/<br/>Helper scripts]
    end
```

## Per-path ownership

| Path | Owner service | Survives upgrade? | Notes |
|---|---|---|---|
| `/etc/powerlab/security/` | gateway | ✅ yes | CA + leaf cert. Decoupled from runtime per ADR-0010 — never wiped by `start.sh --build` or routine cleanups |
| `/etc/powerlab/.hsts-armed` | gateway | ✅ yes | Gate file presence arms HSTS header (ADR-0006) |
| `/etc/powerlab/version` | install.sh | ✅ yes | Single line `VERSION=x.y.z` — read by updater for current version |
| `/etc/powerlab/*.conf` | individual services | ✅ yes | install.sh refuses to overwrite an existing `.conf` (preserves user edits) |
| `/var/lib/powerlab/db/*.db` | individual services | ✅ yes | SQLite per service. Backed up to `backups/pre-upgrade-*` on `--upgrade` |
| `/var/lib/powerlab/apps/` | app-management | ✅ yes | Generated compose YAMLs |
| `/var/lib/powerlab/appstore/` | app-management | 🟡 cached | Refetched if missing or stale |
| `/var/lib/powerlab/backups/` | install.sh | ✅ yes | Snapshot dir per `--upgrade` run; cleaned up by older-than-N-days policy |
| `/DATA/AppData/` | apps themselves | ✅ yes | **Shared with CasaOS today**; will move to `/DATA/PowerLabAppData/` in Sprint 4 (#85) |
| `/var/run/powerlab/*.sock` | individual services | ❌ ephemeral | Recreated on boot |
| `/var/run/powerlab/*.url` | individual services | ❌ ephemeral | Boot-order sentinels |
| `/usr/share/powerlab/www/` | install.sh | 🔄 replaced | Fresh on each install/upgrade |
| `/usr/share/powerlab/shell/` | install.sh | 🔄 replaced | Fresh on each install/upgrade |
| `/usr/bin/powerlab-*` | install.sh | 🔄 replaced | Fresh on each install/upgrade |
| `/etc/systemd/system/powerlab-*.service` | install.sh | 🔄 replaced | Fresh on each install/upgrade (snapshot kept) |

## Upgrade flow (preservation guarantees)

`install.sh --upgrade` (the path the in-UI updater triggers):

1. Snapshot current state to `/var/lib/powerlab/backups/pre-upgrade-<TS>/`:
   - `/etc/powerlab/` (configs + security + version)
   - `/var/lib/powerlab/db/` (databases)
   - `/usr/bin/powerlab-*` (binaries)
   - `/etc/systemd/system/powerlab-*.service`
2. Stop services in reverse boot order.
3. Replace binaries, www, systemd units, sample configs.
4. **Preserve** existing `*.conf` files (don't overwrite).
5. Start services in boot order.
6. Run a 5-attempt health check against the gateway.
7. On failure, restore the snapshot and start services.
8. Write `/var/lib/powerlab/last-upgrade.json` with `{from, to, succeeded_at|failed_at, snapshot_path}` so the UI can surface the result.

## Coexistence with CasaOS

Two paths PowerLab and CasaOS currently share, scheduled for separation
in Sprint 4 (#85):

- **`/DATA/AppData/`** — apps install their volumes here regardless of
  which panel triggered the install. Migration in Sprint 4 moves
  PowerLab's installs to `/DATA/PowerLabAppData/`.
- **Docker labels** — `createdBy=casaos` on all containers. Sprint 4
  changes new installs to `createdBy=powerlab` and migrates existing.

## Reference

- ADR-0010 — CA storage decoupled from runtime data dir
- ADR-0007 — internal-network-only deployment posture
- Issue #85 — Sprint 4 coexistence polish (Docker labels + AppData)
- `scripts/package-linux.sh` — owns all path layout + upgrade flow
