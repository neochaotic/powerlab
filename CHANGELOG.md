# Changelog

All notable user-facing changes to PowerLab. We follow
[Semantic Versioning](https://semver.org/) — `vMAJOR.MINOR.PATCH`. While
PowerLab is in `v0.x`, breaking changes can land in MINOR bumps; from
`v1.0` onward we commit to backwards compatibility within MAJOR.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
A new entry MUST be added in the same commit as any user-visible change —
see `CONTRIBUTING.md` for the rule.

## [Unreleased]

## [0.3.2] — 2026-05-07

Patch on v0.3.1. Several bugs reported in production, all under TDD —
each fix lands with a regression test that fails on the bug's input
and passes on the new behavior.

### Fixed

- **Apps disappear after page refresh** (Critical). Race condition
  between the layout's async `auth.checkSession()` (which called
  `setAuthToken`) and `+page.svelte`'s `fetchInstalledApps()` —
  the apps fetch fired before the JWT was rehydrated into the http
  client, so the gateway responded 401 and `installedApps` stayed
  `{}`. The store now rehydrates the JWT synchronously at module
  init. Locked in by `auth-rehydration.test.ts`.
- **Sidebar `sidebar.files` / `sidebar.models` keys leaking** as
  literal text in the launchpad. Keys were referenced from
  `routes/+page.svelte` but never added to the locale JSONs.
  Added in en/pt-BR/es.
- **Files: clicking a file with an uncommon text extension
  (`.py`, `.go`, `.toml`, `.env`, dotfiles) opened a new browser
  tab instead of the editor**. The previous handler had a narrow
  whitelist of editable extensions and fell through to
  `window.open(downloadUrl)` for everything else. Now the routing
  is positive-by-default: directory → navigate, media → preview
  pane, large file (>10 MB) → toast pointing at right-click
  Download, everything else → editor. Mirrors the filebrowser UX
  rules. Locked in by `file-open.test.ts` (33 cases).
- **Files: save toast invisible** because the toast container
  rendered at `z-50` but the editor modal is `z-[100]` — the
  toast was behind the modal. Bumped toast container to
  `z-[200]` so it's always above any modal layer.
- **Files: no Delete button in the toolbar** when items were
  selected; deletion was right-click-only and undiscoverable.
  Added a contextual Delete button that surfaces in the toolbar
  whenever `selectedCount > 0`, with a count badge — matches
  filebrowser's affordance.
- **Verify (Test) button silently no-op'd in production**. The
  4-guard probe armed HSTS correctly but the final redirect
  target was identical to the current URL, so `window.location.
  href = same` produced no visible navigation. User reported
  "nada acontece". The dance now distinguishes redirect from
  already-secure-noop and shows a distinct success toast in the
  latter case. Locked in by `trust-dance.test.ts` (6 cases).
- **CA download blocked by Chrome's High-Risk File policy** when
  the panel was on HTTPS-with-untrusted-cert (catch-22 of Trust
  Onboarding). Added four escape hatches in Settings → Security:
  (1) primary advice banner explaining that clicking "Keep" on
  Chrome's warning is safe, (2) "Download as .txt" using a new
  backend endpoint (`/v1/sys/ca-certificate.txt`) that serves
  the same PEM bytes with a NOT_DANGEROUS extension Chrome
  won't block — user renames after, (3) "Show as text" inline
  view with copy-to-clipboard, (4) "Open via HTTP" new-tab nav
  that bypasses HTTPS-context blocking. Backend test
  `TestHandleCATxt` asserts byte-equivalence with `.crt`.
- **Install progress invisible** during compose deploys. Backend
  was emitting "Phase N/M" markers in the SSE stream; the UI
  ignored them. Added a parser and a visual progress bar with
  percentage above the live log. Locked in by
  `install-phase.test.ts` (12 cases).
- **Docker subnet-pool exhaustion error opaque**. When the user
  has installed enough apps that Docker's default 15-subnet pool
  fills up, new installs fail with "all predefined address
  pools have been fully subnetted" — a daemon-level error that
  required searching forums. The install-error overlay now
  detects this string and renders inline remediation:
  explanation, the two prune commands the user can copy, and a
  follow-up pointer to expanding `default-address-pools` in
  `/etc/docker/daemon.json`.

### Internal

- 4 new test files (`auth-rehydration`, `trust-dance`,
  `file-open`, `install-phase`) — 51 new test cases. Full
  vitest suite at 219 passing.
- New utility modules `lib/utils/trust-dance.ts`,
  `lib/utils/file-open.ts`, `lib/utils/install-phase.ts` —
  pure functions extracted from page components for testability.

## [0.3.1] — 2026-05-07

Patch on the v0.3.0 bundle. Bugs reported within hours of the release
plus a polish pass on translations and download UX.

### Fixed

- **Updater no longer rejects upgrades from non-SemVer builds** (#55).
  Binaries built without the `-X POWERLAB_VERSION=...` ldflag (dev /
  CI / source builds) report as `vdev`, which made the updater
  refuse to upgrade with a misleading "intermediate release first"
  error. The check now allows the upgrade with a soft warning when
  the current version isn't a tagged SemVer release.
- **CA download buttons** in Settings → Security no longer
  navigate the browser to the cert URL — fetched via JS, saved as
  Blob, triggered with a programmatic `<a download>` click (#50).
  Failure modes degrade to a toast instead of a plain-text error
  page, and the file always lands on the device where the browser
  runs (not on the server).
- **`launchpad.uninstall` rendered as the literal key** in the
  Launchpad app menu (#56). Key was missing from all three locale
  tables. Added in en/pt-BR/es.
- **Uninstall confirmation modal** in the Launchpad was hardcoded
  in English even when the locale was Portuguese or Spanish (#58).
  Three new keys (`apps.uninstallTitle`,
  `apps.uninstallSubtitle`, `apps.uninstallDataPreserved`) added
  to all three locales.
- **Settings toast strings** (Trust established / Trust reset / CA
  rotated / Port range error / Already current port) migrated to
  `t()` calls. Five strings, three locales.
- **`neochaotic` byline on the product page** is now a clickable
  link to https://github.com/neochaotic. The hardcoded `&copy; 2026`
  also became `new Date().getFullYear()` so the year stays right.
- **Custom App name validation** (#48). The form's silent
  fallback to `'web'` when the name was cleared is gone — the
  Deploy button is now disabled until the user provides a valid
  Docker Compose name (lowercase letters, digits, hyphen,
  underscore, dot). Initial value changed from `'web'` to empty so
  the placeholder reads as the example it always was. Hover over
  the disabled Deploy button shows the validation reason.

### Known issues (under investigation)

- **#57 — Files editor not editable in some user environments.**
  Component-level vitest covers the open-empty / open-existing /
  404-fallback paths and all pass; user reproduces the bug only on
  a real-browser session. Needs DevTools console output from the
  affected user before we can land a fix. If you hit this, F12 →
  Console, copy any error lines onto the issue, please.

### Documentation

- **HTTPS Trust Onboarding Pattern spec expanded** with four new
  sections derived from v0.3.0 hardening:
  - **Persistence** — where to store CA, public-backup file
    convention, what to back up vs what NOT to back up.
  - **CA mismatch detection & recovery** — `/v1/sys/trust-state`
    fingerprint endpoint, RFC 6797 §6.1.1 disarming window
    (`max-age=0`), browser HSTS pin eviction.
  - **CA rotation** — separate destructive flow with
    type-to-confirm modal, `.previous` audit trail, why-not-cross-sign.
  - **Download UX** — JS-driven Blob download instead of
    `window.location.href` navigation; failure modes degrade
    gracefully.

## [0.3.0] — 2026-05-07

The "Local HTTPS + Localization + Developer Portal" release. Three
significant features land together: an Apple-grade local HTTPS
trust dance, three locales (en / pt-BR / es) with persistence and
autodetection, and an embedded Scalar-powered API documentation
portal. The HTTPS choreography is also formalized as a portable
public-domain framework — the **HTTPS Trust Onboarding Pattern**.

### Added — Local HTTPS (#43)

- **Apple-grade local HTTPS out of the box.** First-boot ECDSA P-256
  root CA + 1-year leaf cert. Apple `.mobileconfig` is signed by the CA
  itself so iOS and macOS render "Verified by PowerLab Local CA"
  instead of the red "Unverified" banner. Windows gets DER (`.cer`) for
  the Certificate Import Wizard; everyone else gets plain `.crt`. The
  user-agent-aware `/v1/sys/ca-certificate` endpoint picks the right
  format automatically.
- **Daily renew ticker + IP-change watcher.** The leaf is re-issued
  60 days before expiry and immediately whenever the host's bound IP
  set changes (DHCP renewal, network swap, multi-NIC toggle), so HTTPS
  never silently breaks because the SAN went stale.
- **HSTS gate** (`POST /v1/sys/trust-confirmed`). HSTS is NOT armed
  until the user has proven the trust dance worked end-to-end (the
  request must arrive over HTTPS from a non-localhost peer). Prevents
  the classic homelab lock-out where HSTS ships before the user has
  installed the CA. See ADR 0006.
- **Soft HTTP pill banner** in the top-right corner, dismissible
  per-session. Click → deep-links into `/settings#security` walkthrough.
- **Per-platform walkthrough** in Settings → Security: iOS, macOS,
  Android, Windows tabs with the right download link for each.
- **4-guard Test Connection** before redirect-to-HTTPS: TLS reachable +
  SPA served on HTTPS + HSTS arm acked + only-then-redirect. The
  white-screen-of-death class of failure is closed.
- **SPA fallback chain** in the gateway's static handler so deep-link
  routes (`/settings`, `/dashboard`, refresh-on-route) never 404.
- **Port-change probe-before-redirect** (`probePortReachable`) so the
  Settings → Network → Listen port editor never strands the user on
  a connection-refused page.
- 7 new ADRs (`docs/decisions/0001`…`0007`) documenting cert validity,
  pkcs7 signer choice, reset-trust UX, walkthrough UX, PWA scaffolding,
  the HSTS gate, and the LAN-only initial deployment posture.
- Regression tests: SAN classification (17 cases), PKCS#7 signing,
  HSTS middleware (4 scenarios), port-probe helper (9 cases), SPA
  fallback (15 cases), CA download UA-redirects (6 cases).

### Added — Localization in en, pt-BR, es (#51)

- **Three locales** ship in this release: English, Português
  (Brasil), Español. ~390 strings each.
- **JSON-backed**: each locale lives in `ui/src/lib/i18n/locales/<id>.json`
  so translators contribute by editing JSON, never the host
  TypeScript. Vite imports them statically at build time so the
  bundle ships them all (no runtime fetch, no waterfall).
- **`navigator.language` autodetection** on first visit. `pt-PT` →
  pt-BR, `es-MX` → es, anything else → English. Override via
  Settings → General → Language.
- **localStorage persistence** (`powerlab_locale` key). The chosen
  locale survives a refresh.
- **Language picker** in Settings → General. Each option labeled in
  its own script (`Português (Brasil)`, not "Portuguese (Brazil)").
- Data-quality regression tests pin: every locale has the same key
  set, every translation has the same `{param}` placeholders as
  English, no Portuguese-only words leak into Spanish.

### Added — API documentation portal (#52)

- **Embedded Scalar-powered portal at `/docs`**. New routes
  `/docs[?service=<id>]`, `/docs/spec?service=<id>`,
  `/docs/scalar.js`, and `/docs/logo.svg` serve a self-contained
  interactive API reference for all six backend services. The
  Scalar runtime + per-service OpenAPI specs + the PowerLab squircle
  logo are bundled into the gateway binary via `embed.FS` — no CDN,
  no network calls, fits the LAN-only deployment posture in
  ADR 0007.
- **Service switcher** dropdown jumps between the six APIs while
  preserving the URL hash so deep-linked operations survive.
- **Bearer-token pre-fill** via `#access_token=...` URL hash; tokens
  stay client-side (URL fragments are not sent to servers).
- **Sidebar entry** for `/docs` (BookOpen icon). Opens in a new tab
  because Scalar mounts its own router inside the page.
- **OpenAPI specs rebranded CasaOS → PowerLab** with a surgical edit
  that touches only `info.title` and `info.description` (no
  endpoint / parameter / schema descriptions mutated, per the
  "specs are immutable inputs" rule in ADR 0008).
- **PowerLab logo** at the top of every spec page (the same squircle
  used in the Launchpad).
- ADR 0008 documents the design choice (Scalar over Swagger UI /
  Redoc) and the firm "specs are immutable inputs" rule that keeps
  the source `openapi.yaml` files round-trippable through codegen.
- 16 regression tests for the route handlers + the data-driven
  service registry.

### Added — Trust durability hardening

Day-2 polish to make the trust dance survive realistic operational
scenarios: data-dir wipes, accidental cleanups, browser-side HSTS
caches, scheduled CA rotations.

- **CA storage moved to a stable path independent of the runtime
  data dir** (ADR 0010). Production: `/etc/powerlab/security/`.
  Dev: `~/.config/powerlab/security/`. A one-shot migration
  brings users upgrading from v0.2.7 over without intervention.
- **`ca-public-backup.crt` written next to `ca.crt`** with the
  same bytes but an explicit "this is the file to back up"
  filename. The CA private key is NEVER part of the backup;
  exposing it would void every device.
- **`GET /v1/sys/trust-state`** endpoint returns the server's
  current CA fingerprint + HSTS arm state. Unauthenticated by
  design (CA cert is public). Used by the new
  `TrustStateChecker` UI component to surface a "Trust changed —
  re-install CA" pill *before* the user hits a cert error wall.
  ADR 0011.
- **HSTS disarming window**: after `Reset trust` or `Rotate CA`,
  the server emits `Strict-Transport-Security: max-age=0` for 15
  minutes. Per RFC 6797 §6.1.1, browsers MUST evict their cached
  HSTS pin on `max-age=0` — this is the only mechanism that
  reliably recovers a browser that already pinned. ADR 0011.
- **`POST /v1/sys/rotate-ca`** — a deliberately destructive
  rotation endpoint, separate from `Reset trust`. Triple-gated:
  HTTPS + non-localhost + `?confirm=ROTATE_CA` query parameter.
  Surfaces a rose-tinted modal in Settings → Security → Recovery
  with explicit consequences and a type-`ROTATE`-to-confirm
  input. ADR 0012.
- **`DELETE /v1/sys/trust-confirmed`** — the lighter "Reset
  trust" action. Clears the HSTS gate without touching the CA;
  drops the disarming marker so browsers also clear their pin.
- 13 new regression tests across security_route + gateway_route
  pinning trust-state shape, rotation guards, HSTS disarming
  window, fingerprint format, public-backup write.

### Added — HTTPS Trust Onboarding Pattern (framework spec, #53)

- New canonical reference document at
  `docs/patterns/https-trust-onboarding-pattern.md` formalizing the
  v0.2.7 HTTPS choreography as a portable public-domain framework:
  problem statement, decision tree for when-to-use, Mermaid
  architecture + state machine + 4 sequence diagrams, 8 components
  mapped to reference impl file paths, "why these design choices"
  UX-first walkthrough, threat model, per-language implementation
  guide (Go / Node / Python / Rust), testing checklist, FAQ.
- ADR 0009 names the pattern and records why we license the spec
  public-domain (so commercial / MIT / Apache projects can adopt
  it) while keeping the reference implementation AGPL-3.0.

### Fixed

- **Footer "Crafted with ❤ by neochaotic"** rendered as
  "Crafted withby" when the Heart icon failed to load (slow
  hydration, ad-block extensions). Whitespace is now explicit and
  the author handle links to the GitHub profile.
- **Per-machine `gateway.ini` was leaking dev home paths into the
  repo.** Untracked + gitignored; the gateway binary regenerates it
  from the embedded sample on first boot. New
  `scripts/check-no-absolute-paths.sh` guard wired into
  `scripts/validate.sh` step 0 prevents the regression class.

## [0.2.6] — 2026-05-06

The "stable Files / App Store" release. v0.2.5 shipped a backend that
fixed several Linux bugs but its UI bundle was stamped 0.2.0 (the
package.json had never been bumped) — so users with cached browsers
got a UI that didn't know about any of those fixes. v0.2.6 closes
that loop and adds the E2E + UI gates that prevent the same shape of
regression from ever shipping again.

### Added — App Store

- **Render `x-casaos.tips` in the UI** (#41). Backend already
  exposed `tips: { before_install, custom }`; the UI ignored the
  field. Vaultwarden / Gitea / Pi-hole / etc. install fine but
  surface no clue what the initial admin password is — the user
  was stuck. Now: `before_install` shows in the install confirm
  modal as an amber "First-run note" block; `custom` shows on the
  app detail drawer through the existing `Markdown.svelte`
  renderer so apps that already use bullet lists / code spans
  display correctly.

- **App Store install-flow coverage gate** (#42 +
  `scripts/test-store-sample.sh` + `docs/STORE-COVERAGE.md`).
  Three modes:
    · `--quick` (5 apps, ~3 min) — CI patch tags
    · default (10 apps, ~7 min) — every release
    · `--full` (18 apps, ~15 min) — tag-time
  Set-cover sampling: 18 apps cover 99% of install-flow features
  vs the 196-app random sample needed for the same statistical
  confidence. Pass criteria ≥94% (one Docker Hub flake allowed).
  Wired into `validate.sh --full` as the final release gate.
  Forces the inner dockerd to use the `vfs` storage driver — the
  default `overlay2` fails on macOS Docker Desktop and most
  shared-runner CI hosts because nested overlay can't be mounted
  on top of overlay.

- **Files page lands on `~/PowerLab/`** by default for users
  authenticated via PAM/dscl. Falls back to the daemon's process
  home (`/Users/<dev>/PowerLab` on macOS dev,
  `/root/PowerLab` on Linux production) for SetupWizard users
  with no OS account. Path is short, writable, and exists —
  replaces the previous `/DATA` default that didn't exist on
  fresh dev hosts.

- **POST = create / PUT = update file split** (#37). Filebrowser-
  style REST: POST returns 409 on conflict (or 200 with
  `?override=true`), PUT returns 404 if the file is missing.
  Auto-mkdir-p the parent on POST. Editor's "Save" picks the
  right verb based on whether the file existed at open time.

- **Version handshake** (`/v1/powerlab/version`) — UI compares
  its compiled-in `__APP_VERSION__` to the backend's
  link-stamped version on app boot. Mismatch shows a
  non-dismissible amber banner: "Update available — please
  reload" with the two version numbers and a Reload button.
  Closes the silent-stale-UI failure mode that bit v0.2.5.

- **Cache-Control hardening** on the gateway: `index.html` and
  SPA fallbacks get `no-cache, must-revalidate`; hashed
  `_app/immutable/*` assets get `public, max-age=31536000,
  immutable`. Browsers no longer hold onto stale UI shells
  across deploys.

- **install.sh detects CasaOS coexistence** and either refuses
  (clear error + remove command) or proceeds with
  `--allow-coexist`. Once both are installed, subsequent
  `--upgrade` runs auto-detect coexist mode so the in-UI
  updater works without manual flags.

### Fixed — Files

- **Editor was stuck on a grey screen** because `initEditor()`
  ran while `loading=true` (before the editor div mounted).
  Replaced the imperative call with a `$effect` that fires only
  once `editorContainer && readyToInit && !view`. Component
  test asserts `.cm-editor` is in the DOM after mount so the
  race cannot return.

- **Single click on a file now opens it** (filebrowser style),
  Cmd/Ctrl/Shift-click multi-selects. Previously single click
  only highlighted; users tried to double-click but
  double-click was easy to miss.

- **`Element.animate` polyfill** for jsdom so component tests
  using Svelte transitions stop failing with `TypeError:
  element.animate is not a function`.

- **Editor UX matches filebrowser**: `•` indicator on the title
  while dirty, Save button disables when there's nothing to
  save, Save button reads "Create" while `isNewFile`, Esc / X /
  backdrop-click all run the same `confirm` flow before
  discarding changes, Save success surfaces as a toast.

- **Editor JSON tag drift** that broke save in v0.2.4 — the
  request body's keys are pinned by `file_test.go` so `path` /
  `content` can't accidentally come back.

- **Auto-mkdir on upload** (`/DATA` doesn't exist on dev hosts
  → 500 became 200 on first upload). E2E covers this directly.

- **404 (not 500) for missing-file reads.** The editor
  inspects the status to switch to "create new" mode; 500
  looked like a backend crash and broke that affordance.

- **Range-request streaming + JWT-in-URL for `<video>` /
  `<audio>` / `<a download>`**. Browsers can't attach
  Authorization headers to those elements; without
  `?token=…` every download from any non-localhost client got
  401 from the gateway's JWT middleware.

- **Service Worker registration removed** — the SW was a
  pass-through (`fetch(event.request)`) with no caching
  strategy of its own. Under vite dev it intercepted SPA
  navigations to `/apps` and surfaced "Failed to fetch
  / sw.js:11 Uncaught TypeError" in the console for no benefit.

### Fixed — gateway / proxy

- **Listen-port change** (Settings → Network → Listen port)
  actually moves the listener and `/v1/gateway/port` reports
  the new value within the gateway's 1-2s drain window. E2E
  covers the full move-and-revert path.

- **`/v2/app_management/compose/:id/disk-usage`** route
  registered (handler existed but the OpenAPI middleware was
  rejecting with 400 "no matching operation found"). Same fix
  applied to `/v2/app_management/config`.

- **`/v1/sys/hardware`** path corrected in the UI client
  (was `/v1/sys/hardware/info` which 404'd; the swagger
  annotation was misleading).

- **Multipart upload Content-Type handling** in `client.ts`:
  the JSON default no longer overrides FormData bodies, so the
  browser's `multipart/form-data; boundary=…` reaches the
  server intact. Tests assert the contract both ways.

- **`/v1/sys/logs`** now actually tails — the `line` parameter
  was plumbed through but ignored, so the endpoint returned
  the entire log file (megabytes of historical noise including
  panics from boots that already recovered).

### Build / packaging

- **`ui/package.json` bumped to `0.2.6`**, `vite.config.ts` reads
  `POWERLAB_VERSION` env first so the bundle always matches the
  released tag without anyone bumping by hand. `package-linux.sh`
  stamps `ui/build/.powerlab-version` after each build and
  refuses to reuse a build directory whose stamp doesn't match.

- **`scripts/test-linux-e2e.sh`** now asserts ALL of: UI bundle
  version matches backend version, version handshake responds
  the same string, login OK, editor read/write existing+new
  file, app list, terminal websocket pty echo, file upload +
  missing-file rejected with 400, upload auto-creates parent
  dir, missing-file read returns 404 (not 500), download +
  Range request returns 206, hardware-info / app-management-
  config / disk-usage routes wired, port-change moves
  listener, plus three install scenarios (clean / casaos
  refuse / casaos coexist with `--allow-coexist`). 23
  assertions across the three scenarios.

## [0.2.5] — 2026-05-06

### Fixed (Linux production hardening)

Six bugs that masked themselves under macOS dev — all surfaced by the
first real Linux deployment.

- **`install.sh` detects existing CasaOS installation.** PowerLab is a
  fork of CasaOS; running both side-by-side without knowing which
  port is which produces a confusing experience (port 80 = CasaOS,
  port 8765 = PowerLab; apps don't cross over). install.sh now
  detects systemd `casaos*` unit files and refuses to install with
  a clear remove-or-coexist message. Add `--allow-coexist` to opt
  in to side-by-side mode; the end-of-install banner highlights the
  port distinction so users browse to the correct UI.
- **systemd unit ordering rewritten with the actual service
  topology.** Gateway starts first (writes management.url), then
  message-bus, then user-service / app-management / local-storage,
  then core. Units use `Wants=` (soft) + `After=` + an
  `ExecStartPre` that polls for the dependency's URL file. Without
  this, user-service nil-deref-panicked at startup ~30% of the
  time when message-bus had not finished writing its url yet.
- **`Environment=HOME=/root` on every PowerLab unit.** Without it,
  the terminal pty's `bash -l` exited immediately because $HOME is
  unset under systemd; surfaced as "Lost connection" before the
  prompt could draw.
- **`/var/lib/powerlab/db` is now created on install.** message-bus
  panicked at startup with sqlite's confusing "out of memory (14)"
  error (really SQLITE_CANTOPEN) because its persistent DB's parent
  directory did not exist. install.sh creates it; the repository
  code also `MkdirAll`'s it as a belt-and-braces.
- **`model.FileUpdate` JSON tags changed `path`/`content` →
  `file_path`/`file_content`** to match what the editor UI sends.
  The old tags zero-bound both fields and PUT /v1/file always
  returned "File already exists" on every save attempt.
- **Terminal websocket now passes the JWT in the URL.** Browser
  WebSocket constructors don't accept custom headers, so the
  Authorization header was never sent — the gateway already
  accepted `?token=...` as a fallback, the UI just wasn't using
  it. Added `getAuthToken()` to the API client and Terminal.svelte
  appends `&token=<jwt>`.
- **mDNS service file no longer declares `<host-name>`.** avahi only
  publishes hostnames it owns (the system hostname); the
  hardcoded `<host-name>powerlab.local</host-name>` made avahi
  silently reject the registration. Without the element, services
  advertise against `<system-hostname>.local` correctly.
- **`PostFileUpload` no longer swallows the FormFile error.**
  Malformed multipart bodies returned nil for `f`, then nil-deref'd
  on io.Copy. Now returns 400 with a clear diagnostic.

### Added

- **`scripts/test-linux-e2e.sh`** — end-to-end smoke test that spins
  up a privileged Ubuntu 22.04 + avahi + dockerd container and
  exercises three scenarios: (A) clean install — all 6 services
  cold-start with 0 restarts, then login → editor write → apps
  list → terminal websocket pty echo → file upload (+ missing-file
  rejected with 400); (B) CasaOS present, no flag → install
  refuses with exit 1; (C) CasaOS + --allow-coexist → install
  proceeds, banner highlights port distinction. Wired into
  `validate.sh --full` as a release gate.
- Regression test `backend/core/model/file_test.go` pins the
  `FileUpdate` JSON contract so future drift back to `path` /
  `content` cannot ship.

## [0.2.4] — 2026-05-06

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

### Fixed
- `install.sh` now writes `/etc/powerlab/version` on every install,
  not just on `--upgrade`. Without this, the FIRST upgrade out of
  any host would record `from: "unknown"` in `last-upgrade.json`
  because there was no previous-version file to read. Caught by the
  end-to-end Docker integration test of the v0.2.4 updater.

### Verified end-to-end
- Fresh install of v0.2.4 → gateway HTTP 200, version file persisted.
- Upgrade v0.2.4 → v0.2.5 (same binary, smoke) → snapshot created in
  `/var/lib/powerlab/backups/pre-upgrade-<ts>/`, binary swap clean,
  gateway recovered, `last-upgrade.json` `result: "success"`.
- Broken-binary upgrade → install.sh exited 1, gateway recovered via
  snapshot restore, `last-upgrade.json` `result: "rolled_back"` with
  diagnostic. Auto-rollback works without shell access.


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
