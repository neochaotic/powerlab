---
title: "0024 — Umbrel catalog import via clean-room transform + filter pipeline"
status: proposed
date: 2026-05-11
tags: catalog, app-management, legal, sprint-11
---

# 0024 — Umbrel catalog import via clean-room transform + filter pipeline

**Status:** proposed
**Date:** 2026-05-11
**Tags:** catalog, app-management, legal, sprint-11

## Context

PowerLab inherits the CasaOS app-management plane (ADR-0021), which
consumes catalog entries shaped as one directory per app with an
`appfile.json` manifest and an accompanying `docker-compose.yml`. The
upstream CasaOS catalog is sparse and slow-moving; the Umbrel App Store
(`getumbrel/umbrel-apps` on GitHub) is the largest curated home-server
catalog in the open source ecosystem and is updated continuously.

We want PowerLab users to benefit from that catalog without:

1. **Carrying upstream code in our repo.** The Umbrel App Store
   repository ships under a source-available license that restricts
   commercial use. Shipping their `umbrel-app.yml` / `docker-compose.yml`
   files verbatim — even transformed in place — would propagate that
   license through PowerLab.
2. **Hand-maintaining a fork.** Manual cherry-pick does not scale.
   The catalog must refresh on a schedule with diff visibility.
3. **Importing apps that cannot run on PowerLab.** A large fraction of
   Umbrel apps are deeply coupled to Umbrel-specific infrastructure
   (shared Bitcoin Core node, shared Lightning daemon, `umbrel_main_network`
   joins, `${APP_<sibling>_*}` env references). These apps would
   silently fail to start on PowerLab; importing them is user-hostile.

## Decision

Build a one-way **clean-room transform pipeline** that runs on a
weekly schedule, reads only factual fields from the public Umbrel App
Store, applies a filter, and emits a PowerLab-native catalog under
`community-catalog/` that the existing app-management loader already
knows how to read.

The pipeline:

```
upstream (umbrel-apps@main)
        │
        ▼  (1) fetch  — git clone --depth=1, read-only
        │
        ▼  (2) parse  — extract factual fields only
        │
        ▼  (3) filter — see "Filter pipeline" below
        │
        ▼  (4) emit   — write community-catalog/apps/<id>/{appfile.json, docker-compose.yml}
        │              regenerated from extracted facts, not copied
        │
        ▼  (5) commit — open PR with diff for human review
```

Nothing from the upstream repository is ever committed to PowerLab.
The PR diff shows our own generated output, with every entry carrying
a `source:` block that records the upstream id and commit SHA for
traceability (not legal cover — see "License compliance" below).

## Filter pipeline

Apps are evaluated against four tiers in order. The first matching
rejection tier wins.

### Tier 1 — HARD REJECT (always excluded, no opt-in)

These apps cannot run on PowerLab without dragging in Umbrel
infrastructure we will not ship:

- **`image:` from `getumbrel/*`** — Umbrel-published images carry the
  same upstream license risk as the catalog itself. We never pull
  binaries we cannot relicense.
- **`depends_on:` references another Umbrel app id** — sibling-app
  startup ordering implies a sibling-app runtime, which we do not
  emulate.
- **Cross-app volume mount** — any volume of the form
  `${APP_DATA_DIR}/../<other-app>/...` or `/data/<app-data>/<other-app>/...`
  reaches into a sibling app's data dir. Without the sibling, the app
  cannot start.
- **`networks:` joins only `umbrel_main_network`** and declares no app-
  scoped network. The shared network is how Umbrel apps discover each
  other; without it the app expects siblings that PowerLab does not
  provide.
- **Required env var pattern `${APP_<OTHER>_*}`** where `<OTHER>`
  resolves to a known sibling app id from the same catalog. Optional
  references (with `:-default`) are downgraded to Tier 3.

### Tier 2 — SOFT REJECT (excluded by default, opt-in via config)

Categories that are *individually* fine but where the majority of
entries are bitten by Tier 1 anyway, so the default-deny saves users
from surprise breakage:

- `category: "Bitcoin"`
- `category: "Lightning"`
- `category: "Bitcoin Node"`
- `category: "Wallet"` *(only when paired with one of the above)*

Operators who want crypto apps anyway flip
`community_catalog.allow_categories: ["Bitcoin", "Lightning"]` in
PowerLab config. Apps in those categories that still trip Tier 1 stay
rejected.

### Tier 3 — MANUAL REVIEW (not auto-imported, queued for human triage)

Auto-importing here would be reckless but the app is plausibly
usable:

- Optional sibling env vars (`${APP_<X>_*:-default}`) — usually means
  the app can run standalone with a fallback, but the fallback is
  app-specific and needs eyes.
- Missing `license:` field, or license string we have not seen before.
- Manifest schema version newer than the parser supports.

These land in `community-catalog/_pending/<id>.json` (not loaded by
the runtime) and the weekly PR description lists them as "needs
triage". A maintainer either promotes them or files an exclusion.

### Tier 4 — ALLOW

Everything else: standalone Docker compose apps with a recognised
OSS license (MIT, Apache-2.0, BSD-*, GPL-*, AGPL-*, MPL-2.0), no
sibling-app coupling, and pulling images from a public registry the
user can reach. These flow through to `community-catalog/apps/<id>/`.

## Why a scheduled GitHub Action, not a pre-commit hook

The first instinct was a pre-commit hook that refreshes the catalog
before every commit. Rejected:

- **Network in pre-commit is hostile.** Every developer pays the
  fetch cost on every commit. The first commit of the day eats the
  full sync; offline commits break entirely.
- **No diff visibility.** A pre-commit silently mutates the catalog
  inline; the resulting commit may bundle catalog churn with unrelated
  source changes, making code review noisy.
- **Wrong scope of failure.** A flaky upstream fetch should not block
  a UI bug fix from being committed.

Instead, the sync runs as a weekly scheduled GitHub Action
(`.github/workflows/sync-umbrel-catalog.yml`) plus a manual
`make sync-catalog` for ad-hoc runs. The Action opens a PR
(`catalog/umbrel-sync-YYYY-MM-DD`) with the diff. A maintainer
reviews and merges, just like any other change.

Pre-commit retains a narrow role: a **local validator** that runs
offline and asserts the shape of `community-catalog/apps/*/appfile.json`
is valid (required fields present, no orphaned references, image
names parseable). Validation is fast, deterministic, and catches the
"I hand-edited a manifest and broke it" class of mistake.

## License compliance

The legal posture rests on two pillars, not on attribution:

1. **We import facts, not expression.** Functional metadata — app id,
   image reference, port mappings, env var names, volume paths,
   declared dependencies — is factual and uncopyrightable in the
   jurisdictions PowerLab targets. The transform extracts these
   fields and discards everything else.
2. **We regenerate expression from upstream OSS sources.** App names
   and short descriptions are mechanical (the app's own name).
   Longer descriptions, screenshots, and tagline copy are
   **dropped**, not transformed. If we want a description for an
   imported app, we fetch it from the app's own upstream repository
   (which carries its own permissive license, typically MIT or
   Apache-2.0) or write our own. We never copy the Umbrel-curated
   description text.

The pipeline encodes this by **field allowlist**: the parser declares
the set of fields it reads, and any field not on the allowlist is
ignored even if present. Adding a new field to the allowlist is a
deliberate code change that goes through review, not an oversight.

Attribution (the `source:` block in each generated appfile) exists
for *traceability* — so we can diff against upstream and detect
upstream removals — not as a license cover. We are not relying on
"we credited them" to justify the import.

## Alternatives considered

- **Fork the Umbrel App Store repository and patch in place.** The
  license follows the fork. Non-starter.
- **Build a PowerLab catalog from scratch.** Months of curation we
  do not have. Loses the network effect of an existing curated set.
- **Per-app manual import.** Scales linearly with one human; the
  catalog grows faster than that. Acceptable as a fallback for Tier 3
  apps but cannot be the primary path.
- **Daily sync instead of weekly.** Weekly catches new apps within
  a sprint window and keeps PR noise low. Daily would bury the
  review queue.
- **Import upstream `docker-compose.yml` verbatim and rewrite
  in-place.** Same license problem as a fork — the original
  expression sits in our repo even if mutated.

## Consequences

**What this commits us to:**

- A `cmd/sync-catalog/` Go binary that owns parse + filter + emit.
- A `community-catalog/` directory in the repo, gitignored only for
  the `_pending/` subtree.
- A weekly GH Action with a CODEOWNERS-protected merge gate on the
  catalog directory.
- A local pre-commit validator (offline, fast).
- An explicit field allowlist that we are disciplined about extending.

**What it makes harder:**

- We cannot "just sync" a new Umbrel feature (e.g. a new manifest
  field) without code review. This is the intended friction.
- A user who *wants* a Tier 1 / Tier 2 app has to install it manually
  via Custom App, not from the catalog. Not all Umbrel apps will work
  on PowerLab; we are explicit about that rather than shipping
  broken entries.

**Out of scope for this ADR:**

- The exact wire format of `appfile.json` (already defined by
  ADR-0021).
- Catalog UI changes — the existing /apps page renders whatever the
  app-management plane loads.
- Importing other catalogs (CasaOS upstream, Runtipi, etc.). The
  filter design generalises but each catalog needs its own field
  allowlist.

## References

- ADR-0021 — Docker label namespace and appdata path (catalog schema)
- ADR-0022 — CasaOS upstream is abandoned, no new dependencies
- Issue (to file) — "Implement umbrel-catalog sync pipeline"
- Issue (to file) — "Filter rule unit-test corpus for catalog import"
