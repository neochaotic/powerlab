# Migrating from CasaOS to PowerLab

You currently run CasaOS and want to evaluate, gradually move to, or fully replace it with PowerLab. This page is the honest walkthrough — what you can do today, what is manual, what is genuinely not migrated automatically, and what to expect along the way.

The short version: PowerLab and CasaOS **coexist cleanly on the same host** since Sprint 4 (v0.5.7+, [ADR-0021](../decisions/0021-docker-label-namespace-and-appdata-path.md)). You install PowerLab alongside, both panels run, each only sees its own apps. Migration of individual apps from CasaOS into PowerLab is currently a **manual reinstall** in the PowerLab UI; we do not yet ship an importer that converts an existing CasaOS app into a PowerLab-managed one.

For the technical underpinning of why coexistence works (Docker label namespace, AppData path split), see the [coexistence overview](README.md). For the broader theory of how PowerLab is being separated from CasaOS over time, see the [strangler tracker](../architecture/casaos-strangler.md).

## Step 1 — Install PowerLab alongside CasaOS

Run the standard PowerLab installer. As of v0.5.7+ the installer detects an existing CasaOS install and proceeds with a friendly notice rather than refusing:

```bash
curl -fsSL https://raw.githubusercontent.com/neochaotic/powerlab/main/install.sh | sudo bash
```

You will see a coexistence banner explaining:

- **Different ports** — CasaOS continues to serve on `:80`, PowerLab on `:8765`.
- **Different Docker labels** — PowerLab uses `io.powerlab.v1.*`, CasaOS uses flat `casaos`. Each panel only lists its own containers.
- **Different AppData trees for new apps** — PowerLab installs new apps under `<StoragePath>/PowerLabAppData/`. CasaOS continues using `<StoragePath>/AppData/`.

After the install completes:

- CasaOS at `http://<host>/`
- PowerLab at `http://<host>:8765/`

Both panels keep working. Existing CasaOS apps continue to run under CasaOS management; PowerLab does not see them and does not touch them.

## Step 2 — Set up the PowerLab admin

Open `http://<host>:8765/`. The first visit drops you into the SetupWizard, which prompts for an admin username + password. This is independent of any CasaOS users — PowerLab maintains its own user database (`/var/lib/powerlab/db/user.db`) and does not import or read CasaOS users.

The auth choices and trade-offs are summarized in the [security model page](../concepts/security-model.md#authentication).

## Step 3 — Re-install your apps in the PowerLab UI

This is the manual part. **Apps installed under CasaOS are not automatically imported into PowerLab.** To move an app, you reinstall it via the PowerLab app store (or as a custom compose) and accept that the new instance starts with empty AppData.

The honest reasons:

- **App data lives at different host paths.** Even if PowerLab "knew" about an existing CasaOS-installed Nextcloud, its bind mounts point at `/DATA/AppData/nextcloud/` (CasaOS's tree). The PowerLab-managed reinstall would mount `/DATA/PowerLabAppData/nextcloud/`. Pointing the new PowerLab compose at the old CasaOS data path is not a supported migration step (see [ADR-0021's "existing-app migration deferred"](../decisions/0021-docker-label-namespace-and-appdata-path.md) for why an automatic dir-move + YAML-rewrite is genuinely hard to get right).
- **Custom modifications don't survive a label rewrite.** If you customized the CasaOS compose YAML (env vars, extra services, labels), pushing it through the PowerLab installer would either lose those edits or require a YAML-aware merge that PowerLab does not currently implement.

The supported flow for each app:

1. **In CasaOS**, stop the app (do not uninstall yet — preserve the data).
2. **In PowerLab**, install the same app from the app store (or paste the compose YAML as a custom app).
3. **Manually copy app data** from `/DATA/AppData/<app>/` to `/DATA/PowerLabAppData/<app>/`. For most apps this is a `cp -a` (preserve perms) while both copies of the app are stopped.
4. **Start the app in PowerLab**, verify it sees the data.
5. **Once verified, uninstall the app from CasaOS.**

For apps with internal state in a database (Nextcloud, Vaultwarden, etc.), follow the app's own export/import flow rather than copying raw bind data — the schemas may differ between versions and a raw copy can produce subtle corruption.

## Step 4 — Decide whether to uninstall CasaOS

Once every app you care about is running cleanly in PowerLab, you can:

- **Leave CasaOS installed** — it continues to consume some resources but stays out of PowerLab's way. Useful if you have apps you have not migrated yet.
- **Uninstall CasaOS** — follow CasaOS's own uninstall instructions. PowerLab is unaffected; the `/DATA/AppData/` tree may stick around with stale data you can clean up by hand.

PowerLab does not ship a "clean up CasaOS leftovers" command. The two products are independent installs; the OS-level uninstall is owned by each respectively.

## What does NOT migrate

Honesty matters more than completeness here. The following are **not** automatic today:

| Not migrated | Why | Workaround |
|---|---|---|
| CasaOS users | Different DB, different hash format assumptions | Recreate admin in PowerLab SetupWizard |
| Installed apps | Different label namespace + different AppData path | Reinstall in PowerLab UI per Step 3 |
| App data | Different on-disk path; auto-move risks YAML breakage | Manual `cp -a` between AppData trees |
| Custom CasaOS compose edits | No YAML-aware importer exists | Re-apply manually after reinstall |
| CasaOS files-app shares | PowerLab's files surface is independent | Re-add roots in PowerLab Files |
| CasaOS user-modified app icons / display names | Stored in CasaOS DB, not in compose labels we read | Re-apply via PowerLab's app settings |

A future "import from CasaOS" tool is plausible (issue tracker would carry the design discussion if/when proposed) but is **not** on the v1.0 roadmap. The strategic priority is finishing the CasaOS-strip ([strangler tracker](../architecture/casaos-strangler.md)), not building a forklift importer.

## Limitations and gotchas

- **Port conflicts on `:80`.** CasaOS owns `:80` by default; PowerLab uses `:8765`. If you uninstall CasaOS and want PowerLab on `:80` instead, edit `/etc/powerlab/gateway.ini` and restart.
- **HTTPS is disabled by default in v0.5.x** ([per #130 + ADR-0007](../concepts/security-model.md#default-disabled-in-v05x)). If you were used to CasaOS's HTTPS posture, expect that to differ until you complete the trust dance and re-enable it.
- **Apps that bind to host network mode** can collide between the two products (e.g. two DNS resolvers fighting for `:53`). PowerLab does not detect cross-product host-network collisions; treat them as you would any two products on the same host.
- **Disk pressure scales with both products' AppData trees.** If you keep CasaOS apps installed for months while migrating, you may end up with two copies of large datasets. Plan disk sizing accordingly.

## Reverse migration

We assume PowerLab → CasaOS is the unusual direction, but the same coexistence properties apply. CasaOS's own docs cover its install flow; PowerLab does not stop you from moving back, and the same manual app-by-app pattern works in reverse. PowerLab will not delete anything CasaOS owns.

## To expand

- A guided "5 most common apps" walkthrough (Nextcloud, Jellyfin, Pi-hole, Vaultwarden, Home Assistant) with per-app data-copy specifics.
- A CLI helper that scans `/DATA/AppData/` and reports which apps are still CasaOS-only vs already mirrored in PowerLab.

Track gaps under the docs site polish issue series.
