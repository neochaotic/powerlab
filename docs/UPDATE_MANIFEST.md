# Release manifest — spec for the in-UI updater

The updater (issue [#21](https://github.com/neochaotic/powerlab/issues/21))
needs to know, *before* it downloads a 60 MB tarball, whether installing
the new release on this host is **safe**. The release manifest is the
machine-readable answer.

This document is the contract. Once it ships in v0.2.4 every subsequent
release has to honour it; the updater on the host is allowed to refuse a
release whose manifest does not parse cleanly.

---

## Where the manifest lives

A release publishes the manifest in three places, each with a
specific purpose:

| Location | Purpose | Size |
|---|---|---|
| `manifest.json` at the root of every tarball (`powerlab-linux-<arch>.tar.gz`) | Authoritative copy. `install.sh` reads it after extraction to decide whether to proceed. | ~2 KB |
| `https://github.com/neochaotic/powerlab/releases/download/v<X.Y.Z>/manifest.json` | Standalone asset. The updater fetches this **first** to decide whether it wants the tarball at all. Avoids downloading 60 MB just to read 2 KB of "this release is incompatible with what you have". | ~2 KB |
| `releases/latest/download/manifest.json` (GitHub redirect) | Convenience for the "what's the newest" probe. | ~2 KB |

All three are byte-identical. The package script is what writes them.

---

## Format

JSON, UTF-8, no comments (no JSON-with-comments). Top-level fields:

```json
{
  "version": "0.2.4",
  "released_at": "2026-05-06T20:00:00Z",
  "min_upgrade_from": "0.1.6",
  "skip_release": false,
  "summary": "Settings → change port from the UI + safe in-UI updater",
  "changelog_url": "https://github.com/neochaotic/powerlab/blob/main/CHANGELOG.md#024",
  "tarball": {
    "amd64": {
      "url": "https://github.com/neochaotic/powerlab/releases/download/v0.2.4/powerlab-linux-amd64.tar.gz",
      "sha256": "abcd1234...",
      "size_bytes": 64321056
    },
    "arm64": {
      "url": "https://github.com/neochaotic/powerlab/releases/download/v0.2.4/powerlab-linux-arm64.tar.gz",
      "sha256": "efgh5678...",
      "size_bytes": 60123456
    }
  },
  "breaking_changes": [],
  "pre_install_checks": [
    {"kind": "disk_free_mb", "path": "/var/lib/powerlab", "min": 500},
    {"kind": "docker_healthy"},
    {"kind": "no_apps_unhealthy"},
    {"kind": "no_active_install_task"}
  ],
  "db_migrations": []
}
```

### Field-by-field

#### `version` (string, required)

The semver of *this* release. Must equal the git tag with the leading
`v` stripped. `1.0.0`, `0.2.4`, `0.2.4-rc.1`. Anything that does not
parse as semver is a release-blocking bug.

#### `released_at` (string, required, RFC 3339)

UTC timestamp of when the release was tagged. Used for sort order in
the "what's new" UI and for the "downgrade not supported" check.

#### `min_upgrade_from` (string, required, semver)

The oldest version that can upgrade *directly* to this one. If the
host's current version is older than this, the updater refuses with:

> Cannot upgrade from v0.1.5 directly to v0.5.0. Upgrade to v0.4.x first.

This lets us land breaking changes that need an intermediate stop.
For most patch / minor releases this equals the previous minor:
v0.2.4 sets `min_upgrade_from: "0.1.6"` because v0.2.0's PAM
introduction is the floor below which a direct upgrade is not safe.

#### `skip_release` (bool, default false)

The kill switch. Set to `true` after a release goes out and turns out
to be broken. The updater hides any release with `skip_release: true`
from the "Update available" banner — users on older versions skip
straight past it.

This is preferable to *deleting* a tag (which breaks anyone with a
direct URL bookmark) or to *yanking* the GitHub Release (which loses
the changelog history).

#### `summary` (string, required, ≤ 250 chars)

One-paragraph plain-text summary for the "Update available" toast.
The full changelog goes in `changelog_url`.

#### `changelog_url` (string, required, https URL)

Where the user reads the full changelog. By convention the anchor in
`CHANGELOG.md`, but can be any URL.

#### `tarball` (object, required)

One key per architecture. Each entry has:

- `url` — direct download URL for the arch's tarball
- `sha256` — hex-encoded SHA-256 of the tarball, computed by the
  package script. The updater verifies this **before** extracting.
- `size_bytes` — for the progress bar

If the host's `runtime.GOARCH` does not match any key, the updater
refuses with "no build available for this architecture".

#### `breaking_changes` (array, optional, default `[]`)

Each item:

```json
{
  "kind": "config_format" | "db_schema" | "auth_path" | "api_contract" | "other",
  "description": "Plain-text summary",
  "manual_action": null | "Plain-text instructions for the user before they upgrade"
}
```

If any item has `manual_action` non-null, the updater shows it as a
**warning modal** that the user must dismiss explicitly before the
upgrade button enables.

If the list is empty, the upgrade is "safe" — at least according to
the maintainer's declaration.

#### `pre_install_checks` (array, optional)

Each item is a check the updater runs on the host *before* downloading.

Supported kinds (extensible):

| Kind | Args | Failure means |
|---|---|---|
| `disk_free_mb` | `path`, `min` | Less than N MB free on the named path |
| `disk_free_gb` | `path`, `min` | Less than N GB free |
| `docker_healthy` | — | Docker daemon not responding |
| `no_apps_unhealthy` | — | At least one installed Docker app in unhealthy state |
| `no_active_install_task` | — | An app install or uninstall is in flight |
| `min_uptime_minutes` | `min` | Host booted less than N minutes ago (refuse upgrades on a flapping box) |
| `no_pending_db_migration` | — | A DB migration from a previous upgrade has not finished |

Each check returns one of:

- `pass` — green tick, no UI noise
- `warn` — yellow chevron, user can override and proceed
- `fail` — red cross, upgrade button stays disabled until resolved

The exact `pass`/`warn`/`fail` mapping per kind is hardcoded in the
updater — the manifest does not get to override severity.

#### `db_migrations` (array, optional)

Each item is a migration the updater runs *after* extraction but
*before* starting the new services:

```json
{
  "id": "0001-add-last-login-at",
  "description": "Adds users.last_login_at column",
  "sql": "ALTER TABLE o_users ADD COLUMN last_login_at TIMESTAMP NULL"
}
```

The user-service tracks applied migrations in a small `_migrations`
table; the updater skips migrations that are already applied. If a
migration fails, the updater rolls back the entire upgrade (see the
snapshot/rollback flow below).

`db_migrations` schema is intentionally minimal — we don't try to be
Liquibase. PowerLab's DB is small (users, app metadata, settings) and
schema changes will be rare.

---

## Update flow on the host

1. **Probe** — once per hour, `GET releases/latest/download/manifest.json`. If
   the manifest's `version` differs from the running `__APP_VERSION__`
   *and* `skip_release == false` *and* `current >= min_upgrade_from`,
   show the "Update available" pill in the sidebar.
2. **Click pill** — render the changelog excerpt + the breaking-changes
   list + run pre-flight checks. The "Upgrade now" button is gated on
   no `fail` checks.
3. **Approve** — download the tarball matching the host's arch. Verify
   SHA-256. Refuse on mismatch.
4. **Snapshot** — copy `/etc/powerlab/`, `/var/lib/powerlab/db`,
   currently-installed binaries, currently-installed systemd units to
   `/var/lib/powerlab/backups/pre-upgrade-<from>-<to>-<timestamp>/`.
   The snapshot is the rollback target.
5. **Stop services** — `systemctl stop powerlab-*`.
6. **Move binaries** — `/usr/bin/powerlab-*` → `.bak` siblings; install
   new binaries from the tarball.
7. **Run migrations** — for each item in `db_migrations` not in
   `_migrations`, execute its SQL inside a transaction. Insert the id
   into `_migrations` on success. On failure: abort, roll back.
8. **Start services** — `systemctl start powerlab-*`.
9. **Health-check** — `curl https://localhost:<port>/v1/users/status`,
   5 attempts at 2 s intervals, expect HTTP 200.
10. **Decide**:
    - Health-check passes → write `/var/lib/powerlab/last-upgrade.json`
      with `{from, to, succeeded_at}`. Schedule snapshot deletion in 7
      days. UI shows "Upgrade succeeded — running v<X.Y.Z>".
    - Health-check fails → roll back: stop services, restore `.bak`
      binaries, restore snapshot, restart services. UI shows the
      "Upgrade failed — rolled back to v<X.Y.Z>" banner with a link to
      the captured journalctl excerpt.

---

## Authoring a release

Maintainers edit `release-manifest.yaml` at the repo root **before
tagging**:

```yaml
# release-manifest.yaml — source of truth for the next release.
# `version`, `released_at`, `tarball.*.{url,sha256,size_bytes}` and
# `changelog_url` are filled in automatically by package-linux.sh.

min_upgrade_from: "0.1.6"
skip_release: false
summary: |
  Settings → change port from the UI + safe in-UI updater.
breaking_changes: []
pre_install_checks:
  - {kind: disk_free_mb, path: /var/lib/powerlab, min: 500}
  - {kind: docker_healthy}
  - {kind: no_apps_unhealthy}
  - {kind: no_active_install_task}
db_migrations: []
```

`scripts/package-linux.sh` reads this YAML, fills in the
release-specific fields (version from `git describe`, sha256 from
the produced tarball, etc.), and emits `manifest.json` into the
tarball + into `dist/manifest.json` for upload as a separate
release asset.

---

## Compatibility with the old releases (no manifest)

v0.1.0 through v0.2.3 do not ship a manifest. The host's updater
treats `manifest.json fetch returned 404` as "this release predates
the manifest spec — refuse to auto-upgrade and tell the user to do
the upgrade manually with `install.sh`". This means the in-UI
updater is **opt-in for users who got there from v0.2.4+** and we do
not retroactively try to manage older installs.

---

## Test coverage (mandatory)

Per the project's "every feature ships with tests" rule, the updater
implementation in v0.2.4 lands with these tests at minimum — none of
this is "nice to have", they are release-blocking:

### Unit tests

- `manifest_test.go` — parses every example manifest in `testdata/`,
  asserts every field round-trips through marshal/unmarshal, asserts
  unknown fields are ignored, asserts malformed JSON returns a typed
  error.
- `version_test.go` — semver compare for the `min_upgrade_from`
  decision: rejects older-than-floor, accepts equal, accepts newer.
- `preflight_test.go` — each `pre_install_check` kind has a
  table-driven test covering pass / warn / fail. `disk_free_mb` is
  mocked through a filesystem interface; `docker_healthy` is mocked
  through a Docker-client interface; etc. No test reaches the real
  filesystem or Docker socket.
- `migration_test.go` — applies a sequence of two migrations to an
  in-memory SQLite, verifies idempotence (re-running a migration that
  has already been applied is a no-op), verifies a failing migration
  rolls back the transaction and is not recorded in `_migrations`.

### Integration tests (Docker, in `scripts/test-updater.sh`)

- **happy path** — install v0.2.3, drop a stub v0.2.4 tarball + manifest
  into a local mirror, click upgrade in a headless Chrome session, verify
  the host is now running v0.2.4 and `/v1/users/status` returns 200.
- **broken release** — same but the v0.2.4 tarball ships a gateway
  binary that exits 1 immediately. Verify the rollback ran, the host
  is on v0.2.3 again, and the UI shows the "Upgrade failed — rolled
  back" banner.
- **skip_release honoured** — set `skip_release: true` in the v0.2.4
  manifest. Verify the v0.2.3 host does NOT show the update banner.
- **min_upgrade_from refused** — try to upgrade v0.1.6 directly to a
  manifest with `min_upgrade_from: 0.2.0`. Verify refusal with the
  intermediate-stop message.
- **SHA-256 mismatch** — corrupt the tarball after writing the manifest.
  Verify the updater rejects the download and does not extract.

### Regression bar

Every fix that lands during the v0.2.4 cycle must include either a
unit test in the appropriate `_test.go` file or an integration scenario
in `scripts/test-updater.sh`. PRs without an associated test get sent
back. The validator (`scripts/validate.sh --full`) runs the test
suite before any release tag — see CONTRIBUTING.md "Pre-push
validation".

## Upgrade duration

The tarball is ~70 MB (v0.6.6 amd64); on a home connection that's
sub-second to a few seconds to download. The wall-clock cost of an
upgrade is dominated by **two** post-extract steps, not the
download:

1. **Service restart cycle** (~3–5 s). `install.sh` stops the six
   PowerLab systemd units, swaps binaries in `/usr/bin`, copies the
   UI to `/usr/share/powerlab/www`, then starts the units. Fast and
   bounded; the running services hold their own state in
   `/var/lib/powerlab/` so this is just a binary swap.

2. **Community-catalog refresh** (~30–60 s on first run, ~5–15 s on
   warm runs). Since v0.6.5 (#326) the bundled
   `/usr/bin/powerlab-sync-catalog` is invoked post-install to
   `git clone --depth=1` the upstream
   `getumbrel/umbrel-apps` repo and re-emit the
   `/var/lib/powerlab/community-catalog/` tree against the current
   transform logic. The clone is the heavy part — the umbrel-apps
   repo contains 300+ app folders. **This is what makes a v0.6.5+
   upgrade feel slower than a v0.6.4 era upgrade** — not the tarball
   size (which only grew ~5 MB / ~8%, sub-second on most links).
   The step is bounded by `timeout 60` so an unreachable GitHub or
   a slow link does NOT wedge the upgrade — `install.sh` logs
   `sync skipped — bundled catalog will be used` and proceeds.

### Skipping the catalog refresh

Set `POWERLAB_SKIP_SYNC=1` in the install.sh environment to skip
step 2 entirely. The tarball ships a bundled `community-catalog/`
snapshot that is already good for new installs and most upgrades;
the post-install sync is for keeping the catalog fresh between
PowerLab releases when the upstream Umbrel repo has moved on.

When to use it:

- **Air-gapped boxes.** GitHub is unreachable; let the bundled
  catalog stand and run `powerlab-sync-catalog` later out-of-band
  from a host that has internet, then copy the output.
- **Fast offline upgrades.** You know the bundled catalog matches
  what you already have and you just want a clean binary swap.
- **CI / packaging tests.** Reproducible installs without
  dependencies on the live Umbrel repo state.

Example:

```bash
sudo POWERLAB_SKIP_SYNC=1 ./install.sh
```

Refresh on demand later:

```bash
sudo /usr/bin/powerlab-sync-catalog \
  --output /var/lib/powerlab/community-catalog
```

Or restart the install.sh path without the skip:

```bash
sudo ./install.sh   # runs the full post-install including catalog
```

The user can also disable the post-install sync globally by editing
`/usr/local/etc/powerlab/install.conf` (not yet implemented — file
this as a follow-up if the env-var escape proves insufficient).

## Forward-compatibility

The updater on the host parses unknown fields permissively (ignores
them) so a future manifest version can add fields without breaking
older updaters. New `pre_install_checks` kinds however require an
updater that knows about them — unknown kinds default to `warn` to
keep behaviour conservative.

We can introduce a `manifest_version` field if we ever need a
breaking format change. v0 of the manifest does not declare one;
absence is the implicit version 0 marker.
