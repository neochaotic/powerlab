# Coexistence with CasaOS

PowerLab forked from [CasaOS](https://github.com/IceWhaleTech/CasaOS). The two products can run on the same host without stepping on each other since the Sprint 4 #85 work landed.

## Why this matters

A self-hosted user might:

- Currently use CasaOS but want to evaluate PowerLab without uninstalling.
- Run both: CasaOS for the apps already installed there, PowerLab for new ones.
- Forklift-migrate gradually instead of a flag-day cut-over.

Pre-Sprint-4, doing this was painful: both products tagged Docker containers identically and bound app data into the same `/DATA/AppData/<app>` directories. Each panel listed the OTHER product's containers and same-named apps fought over the same on-disk files.

## What changed

Two structural fixes in [ADR-0021](../decisions/0021-docker-label-namespace-and-appdata-path.md):

### Docker label namespace

PowerLab now writes container labels under the canonical `io.powerlab.v1.*` reverse-DNS namespace:

| Label key | Value |
|---|---|
| `io.powerlab.v1.kind` | `app` (the "is mine" sentinel) |
| `io.powerlab.v1.origin` | `system`, `local`, etc. |
| `io.powerlab.v1.icon` | URL to the app icon |
| `io.powerlab.v1.name` | Display name |
| ...and 8 others | per the ADR |

The CasaOS namespace (flat unnamespaced keys: `casaos = "casaos"`, `origin`, `icon`, etc.) is independent — neither product mistakes the other's containers for its own.

**One-release-window dual-write:** during one release after the change, PowerLab also writes the legacy unnamespaced keys, so containers PowerLab created BEFORE this change stay visible without forcing a `docker compose up --force-recreate`. After that window, the legacy writes drop. Reads accept either naming, indefinitely.

### Per-app data tree

PowerLab moves from `<StoragePath>/AppData/<app>` to `<StoragePath>/PowerLabAppData/<app>`. CasaOS keeps using `<StoragePath>/AppData/<app>`. Same-named apps in both panels write to different host directories now.

**Newly installed apps** (post-Sprint-4) write to the canonical path automatically — `service.rewriteAppDataPathsToCanonical` rewrites the bind-mount sources at install time.

**Existing apps** continue using their original `/DATA/AppData/<app>` paths until manually migrated. The original ADR-0021 draft proposed an automatic mv-based migration, but code review surfaced a correctness bug (compose YAMLs on disk would still point at the legacy path → next start, empty bind, apparent data loss). A correct migration tool that does dir-move + YAML-rewrite atomically is tracked separately.

To consolidate manually for a given app:

```bash
sudo systemctl stop powerlab-app-management
# Edit /var/lib/powerlab/apps/<X>/docker-compose.yml: replace
#   /DATA/AppData/<X>/...  →  /DATA/PowerLabAppData/<X>/...
sudo mv /DATA/AppData/<X> /DATA/PowerLabAppData/<X>
sudo systemctl start powerlab-app-management
```

## install.sh behavior on a coexistence host

Pre-ADR-0021 the installer hard-blocked when CasaOS was detected, requiring a `--allow-coexist` flag. It now proceeds with a friendly notice describing the now-clean coexistence:

```
ⓘ  Existing CasaOS installation detected — proceeding.

   Active CasaOS units:
     · casaos.service
     · casaos-gateway.service
     ...

   PowerLab and CasaOS coexist cleanly since ADR-0021 (#85):
     · Different ports — CasaOS on :80, PowerLab on :8765
     · Different Docker labels — io.powerlab.v1.* vs casaos
     · Newly installed apps use /DATA/PowerLabAppData/
       (apps already installed remain at /DATA/AppData/ —
        see ADR-0021 'existing-app migration deferred')

   Each panel only sees its own apps. Browse http://<this-host>:8765
   for PowerLab; http://<this-host>/ continues to serve CasaOS.
```

The `--allow-coexist` flag is preserved as a silently-accepted no-op so any operator runbooks that pass it continue to work.

## Verifying isolation

Spot-check that PowerLab only lists PowerLab containers:

```bash
# Containers PowerLab considers its own
docker ps --filter 'label=io.powerlab.v1.kind=app'

# Containers CasaOS considers its own (should NOT include the above)
docker ps --filter 'label=casaos=casaos'

# An older PowerLab container (created during the dual-write window)
# carries BOTH labels — it'll show in both queries. Expected.
```

After the dual-write window closes, PowerLab-created containers carry only the canonical labels. CasaOS-created ones carry only the legacy labels. The two sets are disjoint.

## Reading more

- [ADR-0021 — Docker label namespace + AppData path](../decisions/0021-docker-label-namespace-and-appdata-path.md) — the full decision record, including the rationale for `io.powerlab.v1.*` over alternatives, why dual-write, why one release window.
- [Sprint 4 prep — app-management](../audits/sprint-4-app-management-prep.md) — the original audit that scoped the work.
- [Sprint 3 retrospective](../audits/sprint-3-retrospective.md) — for the prior wave (rebrand of services + paths) that this builds on.
