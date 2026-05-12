# Community catalog — the PowerLab app store

The PowerLab app store lists apps from three sources today:

| Source | What it is | Provenance |
|---|---|---|
| **PowerLab native** | Hand-curated apps shipped with the release | committed in `backend/data/apps/` |
| **CasaOS-AppStore** | Mirror of `IceWhaleTech/CasaOS-AppStore` (legacy upstream) | fetched as a ZIP at runtime |
| **Big-Bear** | Mirror of `bigbeartechworld/big-bear-casaos` | fetched as a ZIP at runtime |
| **Umbrel** | Clean-room import of `getumbrel/umbrel-apps` (new) | weekly GH Action; see below |

This document covers the **Umbrel community catalog** specifically — what gets imported, how often, and the rules a maintainer needs to know.

For the other three sources see the existing inline loaders in `backend/app-management/service/appstore.go`.

---

## How it works

```
                                 [getumbrel/umbrel-apps] (public GitHub repo)
                                             │
                  weekly cron (Monday 06:00 UTC, manual dispatch available)
                                             │
                                             ▼
                      .github/workflows/sync-umbrel-catalog.yml
                                             │
                                             ▼
                          backend/sync-catalog/ binary clones,
                          parses, applies 4-tier filter, emits
                                             │
                                             ▼
                              community-catalog/Apps/<id>/
                                             │
                                             ▼
                       PR catalog/umbrel-sync-YYYY-MM-DD ← human review
                                             │
                                merge → app-management.conf picks
                                             │     up local path, app
                                             │     shows up in /apps
                                             ▼
                                  PowerLab catalog UI
```

The full design is in [ADR-0024](../decisions/0024-umbrel-catalog-clean-room-import.md). The umbrella tracking issue is [#307](https://github.com/neochaotic/powerlab/issues/307).

---

## What gets imported — the four-tier filter

Every upstream app is run through `backend/sync-catalog/filter.go`. The first matching rule wins.

### Tier 1 — Hard reject (never imported)

Apps that **cannot run on PowerLab without Umbrel-specific infrastructure**:

- `image:` references `getumbrel/*` or `ghcr.io/getumbrel/*` — Umbrel-team-published binaries that depend on their own runtime
- `environment:` references `${APP_<OTHER>_*}` where `<OTHER>` is another Umbrel-catalog app id (cross-app sibling dependency)
- The image is the Umbrel-only build of an app whose upstream version is published elsewhere (e.g. their bitcoin/lightning/electrs builds)

Examples from a recent sync: `bitcoin`, `lightning`, `electrs`, `umbrel-os-restore`, `tor`. **44 apps** in the current upstream.

### Tier 2 — Soft reject (opt-in via config)

Apps with clean images but whose **category is on the default-deny list**:

- `category: "Bitcoin"`
- `category: "Lightning"`
- `category: "Bitcoin Node"`

These are technically importable, but most of them are sibling-app coupled (Tier 1 already caught them) and the rest carry the same operational complexity (running a full Bitcoin node) that doesn't fit the default PowerLab use case.

**Opt-in** by editing the GitHub Action's `workflow_dispatch` form, or pass `--allow-categories Bitcoin,Lightning` to the binary locally:

```bash
make sync-catalog -- --allow-categories Bitcoin,Lightning
```

When the opt-in is set, apps in those categories pass through Tier 2 and the standard Tier 4 allow rules apply.

**45 apps** soft-rejected in the current upstream.

### Tier 3 — Manual triage (queued for human review)

Today not used; reserved for ambiguous cases the automatic filter can't resolve (optional sibling vars, unknown licenses, new manifest schema versions). When activated, apps land under `community-catalog/_pending/<id>/` and the weekly PR description calls out the queue for maintainer attention.

### Tier 4 — Allow

Everything else. Apps with:
- Third-party-published images (LinuxServer.io, official upstream registries, `ghcr.io/<app-org>`, etc.)
- No cross-app sibling references in env vars
- A recognised category (anything not Tier 2)
- An OSS license (MIT, Apache, GPL, BSD, MPL — all explicitly accepted)

**241 apps** in the current upstream land in Tier 4 → community-catalog/.

### Sample numbers (last weekly sync run)

| Tier | Apps | What you do with it |
|---|---:|---|
| Tier 4 allow | 241 | imported automatically — review the diff per app in the PR |
| Tier 1 hard-reject | 44 | never imported — re-litigation needs an ADR-0024 revision |
| Tier 2 soft-reject | 45 | flip `allow-categories` if you want them |
| Tier 3 manual triage | 0 | empty today; reserved for future ambiguity |

---

## Where things come from — icon, description, provenance

### Icon

Hot-linked to the upstream URL. No re-host.

```
https://getumbrel.github.io/umbrel-apps-gallery/<id>/icon.svg
```

User browsers fetch the SVG directly from Umbrel's GitHub Pages on every catalog render. Pros: zero PowerLab infra to maintain; the upstream catalogue already does this perfectly. Cons: if Umbrel reorganises their gallery repo, our icons break — at which point we either flip a config switch to a local cache (deferred design) or replace per-app via the override path (see below).

Decision: stay on hot-link for now. See memory `feedback_umbrel_icons_as_is` for the longer rationale (revisit when commercialisation plans firm up).

### Description

**NOT** the Umbrel-curated text. Instead:

1. The sync binary reads the upstream app's `repo:` field (the app maintainer's own GitHub URL)
2. Fetches `README.md` from that repo (`raw.githubusercontent.com/<owner>/<repo>/main/README.md`)
3. Strips markdown to plain text, truncates to 200 words
4. Writes `community-catalog/Apps/<id>/description.md` as a sidecar

This means **the description carries the app's own OSS license** (whatever the app uses — MIT/Apache/GPL all permit display + truncation), not Umbrel's curated copy. The legal posture from ADR-0024 turns on this distinction.

If a maintainer wants to **override** the description for a specific app, they can write `community-catalog/Apps/<id>/description-powerlab.md` and that file wins. The sync binary respects the override on write (won't overwrite it) and the UI's `DescriptionResolver` consults it first on read.

### Provenance (the "debug origem" rule)

Every emitted `docker-compose.yml` carries a top-level `x-powerlab:` block with a `source:` sub-block:

```yaml
x-powerlab:
  store_app_id: nginx-proxy-manager
  title: { en_us: "Nginx Proxy Manager" }
  icon: https://getumbrel.github.io/umbrel-apps-gallery/nginx-proxy-manager/icon.svg
  category: networking
  source:
    catalog: umbrel-apps
    upstream_id: nginx-proxy-manager
    upstream_repo: https://github.com/getumbrel/umbrel-apps
    upstream_commit: abc123def456...
    upstream_path: nginx-proxy-manager/umbrel-app.yml
    transform_version: "1.0"
    synced_at: 2026-MM-DDTHH:MM:SSZ
```

This answers two questions at the file level: **"where did this entry come from?"** (the upstream commit SHA + path) and **"how stale is it?"** (the synced_at). Useful when a user files a bug on a specific app — the maintainer can trace it back to the exact upstream version that produced our copy.

The `transform_version` field is bumped when the sync binary's logic changes in a way that warrants re-running over previously imported apps. Older transform versions become obvious in the catalog → next sync re-emits them.

---

## Maintainer rules

### When the weekly PR opens

Every Monday 06:00 UTC the GH Action opens a PR titled `catalog: weekly umbrel-apps sync — YYYY-MM-DD`. Walk through the PR description:

- **Filter summary** — counts per tier. If allow-count drops sharply week-on-week, something is off upstream (an Umbrel migration, a broken manifest schema). Investigate before merge.
- **Diff stat** — file-by-file what changed. New apps appear as additions; deprecated apps as deletions.
- **Review checklist** — grep for `getumbrel/` in the new appfile.json files (none should appear; if any do, Tier 1 had a leak). Check no Umbrel-curated description text leaked into `description.md`.

If anything looks wrong, close the PR and file an issue at `#307` referencing the upstream commit SHA from the offending entry's `source.upstream_commit`. The sync isn't trustworthy until the bug is understood — don't merge "to fix it later".

### When you want to override an app

Two override surfaces exist:

| Override | Path | What it does |
|---|---|---|
| Description | `community-catalog/Apps/<id>/description-powerlab.md` | Wins over the auto-fetched README. Hand-curated copy for apps with bad upstream READMEs. |
| Icon | `community-catalog/Apps/<id>/icon.svg` | Currently NOT respected by the sync binary (decided in `feedback_umbrel_icons_as_is`). If you need this, file an issue first. |

Maintainer-written description files are **never overwritten** by the weekly sync. The sync binary checks for `description-powerlab.md` before writing `description.md` and skips if the override is present.

### When Umbrel removes an app you care about

**Soft-keep** — by default the next sync removes the entry from `community-catalog/`, but the existing `description-powerlab.md` (if you wrote one) survives in git history. If you want to keep the app available on PowerLab after Umbrel drops it, copy the deleted entry to `backend/data/apps/<id>/` (the PowerLab-native source) before merging the sync PR.

### When the upstream license posture changes

The legal posture from ADR-0024 turns on two things:

1. **Functional fields are facts** (image refs, ports, env names, volume paths, deps) — uncopyrightable in the relevant jurisdictions
2. **Expressive content** (descriptions, screenshots, marketing copy) — NEVER imported from the Umbrel catalogue

If Umbrel adds a clear license to `umbrel-apps` or `umbrel-apps-gallery`, the posture may change. The parser's field allowlist (`backend/sync-catalog/types.go::UmbrelManifest`) is the gate — adding a new field is a deliberate code change that goes through review. **Do not add fields that carry expressive content** without re-reading ADR-0024 first.

---

## Operational reference

### Cadence

| Surface | When | Notes |
|---|---|---|
| Weekly sync | Monday 06:00 UTC | Automatic; opens a PR |
| Manual sync | Any time | GH Actions UI → `workflow_dispatch` |
| Local dev | Any time | `make sync-catalog` (writes to repo) or `make sync-catalog-dry` (scan + summary, no writes) |

### Files on disk after a sync

```
community-catalog/
  Apps/
    <id>/
      docker-compose.yml         # upstream + x-powerlab block (read by app-management)
      description.md             # auto-generated from upstream README
      description-powerlab.md    # optional maintainer override (wins on read)
```

### Validation

`sync-catalog --validate-only=community-catalog` walks the tree and asserts shape invariants. Used by:

- CI gate on weekly sync PRs (catches malformed emits before merge)
- Local pre-commit when editing description-powerlab.md / future icon overrides

Rules checked: YAML parses; `services:` + `x-powerlab:` present; `store_app_id`, `title.en_us`, `source.catalog` non-empty; `icon` parses as URL when present. Exit 0 clean, exit 1 with per-rule errors. See `backend/sync-catalog/validate.go` for the canonical rule list.

### Provenance lookup

```bash
# Where did this entry come from?
grep -A 8 "x-powerlab:" community-catalog/Apps/nginx-proxy-manager/docker-compose.yml

# What's the upstream's current state for that commit?
git -C /tmp clone --depth=1 https://github.com/getumbrel/umbrel-apps
git -C /tmp/umbrel-apps log --pretty='%H %ci' -1 nginx-proxy-manager/umbrel-app.yml
```

### Source badge in the UI

Every AppCard tile shows a discrete source label in the metadata row (`networking · umbrel`). The label is rendered from `store_info.source.catalog` when the backend populates it (umbrel-synced apps), or inferred from the icon URL when not (covers CasaOS / Big-Bear / generic). See `ui/src/lib/utils/app-source.ts`.

Click-through opens the upstream repository in a new tab. Hover (desktop) / long-press (mobile) → native tooltip with the synced_at date when present.

---

## References

- ADR-0024 — design + legal posture (`docs/decisions/0024-umbrel-catalog-clean-room-import.md`)
- ADR-0021 — Docker label namespace + AppData path
- ADR-0025 — strangler pattern (renumbered from old 0011)
- Audit (Phase 0) — `docs/audits/catalog-overlap-2026-05-11.md`
- Umbrella issue — #307 (Phase 1 → Phase 6 implementation)
- Memory `feedback_umbrel_icons_as_is` — why we don't rehost icons
