# Updating

PowerLab has two upgrade paths: in-app (recommended) and re-running the install script.

!!! note "Updating PowerLab vs. updating an app"
    This page is about upgrading **PowerLab itself** (the panel). To move an
    **installed app** — Jellyfin, 2FAuth, etc. — to a newer image, jump to
    [Updating an installed app](#updating-an-installed-app). The two are
    independent: upgrading PowerLab never bumps your apps' image versions.

## In-app update (recommended)

When a new release is published on GitHub, the in-app updater detects it within ~6 hours (or immediately if you click "Check for updates" in Settings → System).

The flow:

1. Updater fetches `https://raw.githubusercontent.com/neochaotic/powerlab/main/manifest.json`.
2. Compares the manifest's `version` against the installed version.
3. If newer AND not blocked by the manifest's `min_upgrade_from` field, surfaces an "Update available" toast.
4. You click → preview the release notes (rendered from the manifest's `summary` + linked `changelog_url`) → confirm.
5. Updater downloads the per-arch tarball, runs pre-install checks (disk free, docker healthy, no in-progress install task, no unhealthy apps), takes a snapshot of the current install, then runs `install.sh --upgrade` in a controlled environment.
6. On success: the new version comes up, snapshot rolled into the backup retention window. On failure: snapshot restored, you stay on the old version.

The manifest format + the staleness-check that prevents a re-released summary from going out is documented in [Update manifest](../UPDATE_MANIFEST.md).

## Re-running install.sh

Equivalent to in-app update; the in-app path is just a wrapper.

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash
```

The script auto-detects the existing install and routes through the upgrade path (snapshot → swap binaries → restart).

## What an upgrade preserves

| Preserved across upgrade | Why |
|---|---|
| Your admin account | `user.db` is in `/var/lib/powerlab/`, untouched |
| Your JWT session | The signing keypair persists since v0.5.7 (ADR-0020) — no force-logout |
| Your installed apps | Compose state survives; containers may restart |
| Your app data (`/DATA/`) | Bind-mounted, never touched by the upgrade |
| Your config (`/etc/powerlab/`) | Preserved; new defaults merged into `.sample` files |
| Last 3 upgrade snapshots | Auto-rotated; older are pruned |

Apps installed BEFORE Sprint 4 v0.5.7+ continue using the legacy `/DATA/AppData/<app>` tree. Apps installed AFTER the upgrade use the canonical `/DATA/PowerLabAppData/<app>` tree. See the **[Coexistence with CasaOS](../coexistence/README.md)** page for the rationale.

## When NOT to use the in-app updater

- The release notes mention a manual step that must run BEFORE the upgrade (the "breaking_changes" array in manifest.json — the updater surfaces this as a confirmation gate that disables the upgrade button until you dismiss).
- You're running on a host with disk pressure (less than 500MB free at `/var/lib/powerlab`) — the pre-install check refuses.
- You're in the middle of installing an app — the pre-install check refuses to upgrade with a hot install.

## Rollback

If an upgrade misbehaves, restore the latest snapshot:

```bash
sudo /usr/bin/powerlab-rollback   # restores most recent snapshot, restarts services
```

Snapshots live at `/var/lib/powerlab/backups/pre-upgrade-<timestamp>/`. Three are kept by default.

## Updating an installed app

Upgrading PowerLab does **not** change the version of the apps you installed
(Jellyfin, 2FAuth, …). Each app is its own Docker image, pinned independently.
There are two ways to move an app to a newer version.

### Rolling update (catalog apps)

If an app came from the built-in catalog and the maintainers have bumped its
pinned version, its tile shows an **Update Available** badge:

1. Open **Apps** and click the app.
2. Click **Update** and confirm the **Rolling Update**.
3. PowerLab pulls the new image, recreates the container, and keeps your data.

The rolling update follows the version **pinned by the curated catalog**, so it
may lag the very latest upstream release, and it only works while the app still
matches its catalog entry.

### Fork as Custom App (pin any version)

To jump to a *specific* newer version — ahead of what the catalog pins — fork
the app and set the image tag yourself:

1. Open **Apps**, click the app, and choose **Fork as Custom App**.
2. In the compose editor, set the image tag to the version you want:

   ```yaml
   services:
     jellyfin:
       image: jellyfin/jellyfin:10.10.3   # instead of the catalog-pinned tag
   ```

3. Deploy. The container is recreated on the new image.

Your library and config survive the recreate because app data lives in the
bind-mounted volume under `/DATA/` (see [What an upgrade
preserves](#what-an-upgrade-preserves)). From then on you control the version by
editing that tag and redeploying — check the image's registry (e.g.
[Docker Hub](https://hub.docker.com/r/jellyfin/jellyfin/tags)) for available
tags. Pin an explicit tag rather than `latest` so redeploys stay reproducible.

## Getting older releases

The in-app updater hides any release with `skip_release: true` in its manifest. This kill-switch is how a known-bad release is taken out of the upgrade path without deleting the GitHub tag (so users on old versions can still download it manually for analysis).

For manual download:

```bash
gh release download v0.5.7 --repo neochaotic/powerlab
```
