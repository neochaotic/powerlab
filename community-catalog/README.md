# PowerLab community catalog

PowerLab-curated app catalog (ADR-0039). Every entry under `Apps/` is:

- **Hard-filter clean** — no `hooks/`, no `exports.sh`. PowerLab will never execute upstream host scripts (scripts/check-catalog-hostnames.sh + filter_hooks.go enforce at sync time).
- **Safety lint clean** — no `privileged: true`, no `/var/run/docker.sock` bind, no `network_mode/pid/ipc: host`, no `cap_add: ALL/SYS_ADMIN`, no system-path bind-mounts outside the app's data dir. Enforced by `scripts/check-catalog-app-safety.sh`.
- **Install-verified on staging** — `x-powerlab.yml` carries a `verified: <date>` annotation for apps PowerLab has installed end-to-end on a real Linux box and confirmed reach a healthy state.

## Adding an app

This is hand-curated work, intentionally. Each app is a deliberate PR. The workflow:

1. Author the entry under `Apps/<id>/`:
   - `docker-compose.yml` — PowerLab-curated compose. PowerLab transforms (hostname rewrite, host substitution, bind-mount chmod, image-skeleton-seed) apply at install time.
   - `description-powerlab.md` — PowerLab-authored description. **Do not** copy upstream prose verbatim.
   - `x-powerlab.yml` — manifest (see schema below).
2. Run `scripts/check-catalog-app-safety.sh` locally. Must pass strict mode.
3. Install the app on a staging box. Verify all containers reach healthy.
4. Set `verified: <date>` in `x-powerlab.yml`.
5. Open the PR. Reviewer checks: compose audit, image source, threat model walk.

## x-powerlab.yml schema

```yaml
# Per-app PowerLab manifest. Lives at community-catalog/Apps/<id>/x-powerlab.yml.
id: <kebab-case-id>            # must match dir name
title: <short display name>
tagline: <one-line summary, ≤ 100 chars>
category: <one of: utility, productivity, files, media, automation, dev, ai, security, networking>
license: <SPDX identifier of the upstream app>
upstream:
  source: <upstream repo URL>  # provenance; PowerLab does not pull from here at runtime
  inspiration: <yes|no>         # whether the compose was derived from an upstream catalog
verified: <YYYY-MM-DD>           # date PowerLab install-tested on staging. Omit if unverified.
verified_by:                     # who tested and how
  - name: <reviewer>
    staging: <hostname or release>
    notes: <optional caveats>
notes: <optional free-form, e.g. "data dir at /DATA/PowerLabAppData/<id>/data">
```

## Why not bigger

ADR-0039 explains the trade-off in detail. Short version: a smaller, audited catalog beats a bigger catalog with disclaimers. Operators who want third-party catalogs add them via Settings → Catalog at their own risk (per-app sources get a permanent "Unaudited" badge).
