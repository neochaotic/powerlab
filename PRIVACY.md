# Privacy

**PowerLab has no telemetry.** It does not collect, transmit or analyse
usage data, crash reports, analytics or identifiers. There is no
"phone home", no account and no cloud service operated by the project.
Everything the panel knows about your box stays on your box.

PowerLab does make a small number of outbound network calls to do its
job (pull images, check for updates). This page lists every one of them
so you can audit, firewall or proxy them. If you find an outbound call
that is not listed here, that is a bug: please
[open an issue](https://github.com/neochaotic/powerlab/issues).

## Outbound calls made by the PowerLab services

| What | Destination | When | Sends | Source |
|---|---|---|---|---|
| **Update check** (release manifest) | `github.com/neochaotic/powerlab/releases/latest/download/manifest.json` (redirects to GitHub's release CDN) | When Settings is open: once on load, then hourly; and when you press **Check now** | A plain `GET`: no identifiers, no version, no host info (GitHub sees your IP, as for any download) | `backend/core/service/powerlab_updater.go` |
| **Update download** (release tarball) | The tarball URL listed in that manifest (GitHub Releases) | Only when you click **Upgrade** | A plain `GET` | `backend/core/service/powerlab_updater.go` (`RunInstall`) |
| **Docker image pulls** | The registry named in each image reference (Docker Hub, `ghcr.io`, `lscr.io`, …) | When you install or update an app | Done by your Docker daemon; standard registry protocol | Docker Engine, driven by `backend/app-management` |
| **App update detection** for `:latest` images | The image's registry (manifest `HEAD`/`GET`, plus a token request if the registry requires one) | When the app list asks whether an installed app on a `latest` tag has a newer digest | Image name and tag; registry credentials only if you configured them | `backend/app-management/pkg/docker/digest.go`, `auth.go` |
| **Extra app stores** (opt-in) | The URL you register in **Settings → Catalog** | At startup and every 10 minutes, only for stores *you* added (a `HEAD` size check, then a download when it changed) | A plain `HEAD`/`GET` | `backend/app-management/service/appstore.go` |

The default app catalog is **bundled** into the release tarball
(`/var/lib/powerlab/community-catalog`), so browsing the store does not
download a catalog from the internet. The catalog itself is off until you
enable it.

No other service (gateway, user-service, message-bus, local-storage,
powerlab-mcp) talks to hosts outside your machine; their HTTP clients
only reach other PowerLab services over loopback or unix sockets.

The Docker Compose library linked into app-management carries an
OpenTelemetry exporter that stays inert unless you set
`OTEL_EXPORTER_OTLP_ENDPOINT` yourself; PowerLab never sets it.

## Requests made by your browser while using the panel

These come from the web UI running in your browser, not from the server:

| What | Destination | Why |
|---|---|---|
| UI font | `fonts.googleapis.com`, `fonts.gstatic.com` | The Inter web font loaded by the app shell |
| App icons and screenshots | Whatever URL each catalog entry or custom app declares (mostly `raw.githubusercontent.com`, `cdn.jsdelivr.net`) | Shown in the app store and on your dashboard |
| Setup wizard background texture | `grainy-gradients.vercel.app` | Decorative noise image on the first-run setup screen |
| Links you click | GitHub, docs, an app's own web UI | Only when you click them |

If you run PowerLab offline these simply fail to load; the panel keeps
working, just with a fallback font and missing images.

## At install time

`install.sh` downloads the release tarball from GitHub Releases
(`github.com/neochaotic/powerlab/releases/...`); `install-mac.sh` clones
the repository from GitHub. Neither reports anything back.

## Data stored locally

Sign-in uses your OS accounts. Settings, app definitions and logs live
on your machine (under `/etc/powerlab`, `/var/lib/powerlab` and
`/var/log/powerlab` on Linux). Nothing is mirrored elsewhere.

## Changes

Any change that adds an outbound call must update this file in the same
pull request.
