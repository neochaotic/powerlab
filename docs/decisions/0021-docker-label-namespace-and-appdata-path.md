---
title: "0021 — Docker label namespace + AppData path: io.powerlab.v1.* and PowerLabAppData/"
status: accepted
date: 2026-05-09
tags: app-management, casaos-strip, sprint-4, coexistence
---

# 0021 — Docker label namespace + AppData path

**Status:** accepted
**Date:** 2026-05-09
**Tags:** app-management, casaos-strip, sprint-4, coexistence

## Context

Two concrete coexistence bugs that surface when a host runs both
PowerLab and CasaOS in parallel:

1. **Same Docker label namespace.** PowerLab's app-management writes
   container labels via flat, unnamespaced keys: `casaos = "casaos"`
   (the sentinel that filters "this container is mine"), `origin`,
   `web`, `icon`, `desc`, `index`, `custom_id`, `show_env`, `protocol`,
   `host`, `name`. The "is mine" filter is `if Labels["casaos"] ==
   "casaos"`. CasaOS's app-management writes the same labels with the
   same values. Result: each panel lists ALL of the other product's
   containers as if they were its own. Per-app actions (start, stop,
   delete) hit the wrong product's containers.

2. **Same AppData tree.** Both products mount per-app data under
   `<StoragePath>/AppData/<app-name>` (typically `/DATA/AppData/`).
   When a user installs the same app in both panels (e.g. `nextcloud`
   in CasaOS and again in PowerLab) the two compose stacks bind-mount
   the same host directory. Concurrent writes corrupt the data; even
   non-concurrent writes silently overwrite each other.

Sprint 4 / issue #85 was opened to close both. This ADR records the
specific design choices.

## Decision

### 1. Container label namespace

Adopt the reverse-DNS namespaced convention `io.powerlab.v1.*` for
every container label PowerLab writes. The full set, mapping the
unnamespaced legacy keys to the new canonical names:

| Legacy (kept for read-compat one release) | Canonical (new) |
|---|---|
| `casaos = "casaos"` | `io.powerlab.v1.kind = "app"` |
| `origin` | `io.powerlab.v1.origin` |
| `web` | `io.powerlab.v1.web-port` |
| `icon` | `io.powerlab.v1.icon` |
| `desc` | `io.powerlab.v1.description` |
| `index` | `io.powerlab.v1.web-index` |
| `custom_id` | `io.powerlab.v1.custom-id` |
| `show_env` | `io.powerlab.v1.show-env` |
| `protocol` | `io.powerlab.v1.protocol` |
| `host` | `io.powerlab.v1.host` |
| `name` | `io.powerlab.v1.name` |
| `io.casaos.v1.app.store.id` | `io.powerlab.v1.app.store.id` |

**The "is mine" filter becomes `Labels["io.powerlab.v1.kind"] == "app"`.**
The legacy `casaos = "casaos"` filter is preserved for one release
window so existing PowerLab containers (created before this PR)
continue to be recognized. After two stable releases on the new
filter, the legacy read is removed.

### 2. AppData path

PowerLab's per-app data tree moves from
`<StoragePath>/AppData/<app>` to `<StoragePath>/PowerLabAppData/<app>`.

`<StoragePath>` continues to default to `/DATA` on Linux (Docker
Desktop-accessible path on macOS dev installs).

**Newly installed apps** (post-PR) write to the canonical path
automatically — `service.rewriteAppDataPathsToCanonical` rewrites
the bind-mount sources at install time. **Existing apps** continue
using their original `<StoragePath>/AppData/<X>` paths until manually
migrated. See "Subsequent decision: existing-app migration deferred"
below for the rationale.

A leftover `<StoragePath>/AppData/` is preserved on coexistence hosts
(CasaOS may keep using it).

### Subsequent decision: existing-app migration deferred

**Date:** 2026-05-10 (added during PR-C review of #85)

The original draft of this ADR specified a one-shot
mv-based migration for existing apps. Code review surfaced a
correctness bug: moving `<StoragePath>/AppData/<X>` invalidates the
bind-mount source paths in the on-disk compose YAMLs at
`<AppsPath>/<X>/docker-compose.yml`. On the next container start,
Docker re-creates the bind directory at the legacy path (now empty),
and the app comes up with no data — apparent data loss.

A correct migration would have to do BOTH the directory move AND
rewrite every YAML on disk in lockstep. That's a sizeable surface
(per-app YAMLs are user-modifiable; a parse-and-rewrite has to
preserve formatting, comments, custom YAML extensions like
`x-powerlab`/`x-casaos`, etc.). The risk of an in-place YAML rewrite
breaking user customizations exceeded the benefit of automatic
migration.

**Decision (Option A):** existing apps stay at the legacy path. Only
newly installed apps use the canonical path. The compose
volume-source rewrite at install time is unchanged. The on-boot
migration is removed.

**Consequence:** on coexistence hosts (PowerLab + CasaOS), apps
installed in PowerLab BEFORE this PR will continue sharing the
`<StoragePath>/AppData/<X>` tree with CasaOS — i.e. the data race
risk persists for those legacy apps. New apps are clean. Operators
who want to consolidate can manually:

```bash
sudo systemctl stop powerlab-app-management
# Update app-management/<X>/docker-compose.yml to bind PowerLabAppData
sudo mv /DATA/AppData/<X> /DATA/PowerLabAppData/<X>
sudo systemctl start powerlab-app-management
```

**Follow-up tracked separately:** issue to be opened for a proper
migration tool that does YAML rewrite + dir move atomically. Not
blocking for #85 since the new-install path is correct and the
legacy path is unchanged from pre-PR behavior.

### 3. Dual-write / dual-read window

For exactly **one release window** (whatever ships first after this
PR + the next patch release), PowerLab writes BOTH the canonical
`io.powerlab.v1.*` labels AND the legacy unnamespaced labels on every
new container. After that window, the legacy writes drop and the
container-rebuild on next app-update produces clean labels.

Reads accept either naming, indefinitely, so an upgrade does not
require a `docker compose up --force-recreate` to keep apps visible
in the panel.

## Rationale

### Why `io.powerlab.v1.*` (not `label.powerlab.*` or naked `powerlab.*`)

- **Docker label convention** (per the official Docker docs) uses
  reverse-DNS namespaced keys: `com.example.vendor = ...`. This
  matches the only existing namespaced label PowerLab inherited from
  CasaOS (`io.casaos.v1.app.store.id`).
- **Forward-compat versioning**: the `v1` segment leaves a clean
  room for `v2.*` labels if a future migration needs them, without
  conflict.
- The prep-doc's `label.powerlab.*` formulation was based on an
  incorrect reading of the actual code (the keys are flat, not
  namespaced under `label.casaos`). This ADR corrects to the real
  Docker convention.

### Why PowerLabAppData (not powerlab/AppData or AppData/powerlab/)

- Single top-level segment matches the existing single-segment
  `AppData/`. No nested directory complexity.
- CamelCase mirrors `AppData` so the visual diff in `ls /DATA/` is
  immediate ("there's one tree per product").
- Avoids `<StoragePath>/AppData/powerlab/` because that puts
  PowerLab's data inside a CasaOS-rooted tree, which preserves the
  original sin in a cosmetic disguise.

### Why dual-write, not relabel-on-startup

Relabeling existing containers requires `docker container rename`
+ `docker container update --label-add` + `docker compose down/up`
(labels on a running container are immutable in Docker). Doing this
automatically risks data-loss for any app whose volumes aren't
declared explicitly. The cost of writing two label sets per container
for one release is small; the cost of an automatic recreate-everything
is potentially catastrophic.

When the user voluntarily updates an app (which re-creates the
container), it picks up canonical-only labels. The legacy reads keep
old containers visible until then.

### Why one release window (not "indefinitely")

Two release windows of dual-write produce two label sets per
container forever, doubling the noise and risking the legacy keys
becoming the de-facto read path. One window is the minimum that
allows the in-app update flow to migrate every active container
without operator intervention.

## Consequences

**Positive:**
- PowerLab and CasaOS can run on the same host with non-overlapping
  labels and non-overlapping app data trees.
- The `install.sh` "CasaOS detected" hard-block can relax to a
  notice (per the #85 DoD).
- The unnamespaced label original sin gets fixed on the way out.

**Negative / accepted:**
- One release window writes 2× the label data per container.
  Trivial in practice (KB per container, not MB).
- Migration of AppData requires PowerLab to know which `<app>`
  directories belong to it. Heuristic: a PowerLab compose project
  named `<X>` exists. False-negative = data left in `AppData/`,
  which is repairable by hand. False-positive = NOT possible
  because the heuristic only fires on names PowerLab itself knows.
- Container labels on already-running containers stay legacy until
  the next update of that app. Acceptable: the "is mine" filter
  reads either, so panel UX is unaffected.

## Alternatives considered

1. **Force `docker container update --label-add` on every container
   at boot.** Rejected — Docker labels are immutable on running
   containers; this requires recreate, which risks data loss for
   apps with un-declared volumes.

2. **Migrate AppData via symlink (`AppData/<X>` → `PowerLabAppData/<X>`)
   instead of `mv`.** Rejected — symlinks across StoragePath are a
   support-load risk (a user moving their `/DATA` to a different
   disk will see broken symlinks). `mv` is atomic on the same
   filesystem and the AppData tree is always on `/DATA`.

3. **Skip namespacing, use flat `powerlab` instead of `casaos` as
   the sentinel.** Rejected — preserves the original sin (collides
   with any future product also using flat `powerlab` keys) and
   misses the chance to fix it.

4. **Write both label sets forever.** Rejected — see "why one
   release window" above. The intent is to migrate, not coexist
   with our own legacy.

## Refresh discipline (per ADR-0019)

- Status `accepted` until the dual-write window closes (the PR that
  removes legacy writes amends this ADR's status to `superseded by`
  the same).
- The corresponding test suite in `backend/app-management/common/labels_test.go`
  serves as the executable contract. Any future relaxation of the
  "is mine" filter or the canonical label set MUST update both the
  ADR and the tests in the same PR.

## Reference

- Issue #85 — Sprint 4 main goal (Docker labels + AppData isolation)
- `docs/audits/sprint-4-app-management-prep.md` — original audit
  (the "Required ajustes" section sketched the direction; this ADR
  corrects the namespaced reading and pins specific names)
- Docker label convention — https://docs.docker.com/config/labels-custom-metadata/
