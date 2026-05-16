# 0038 — External catalog integration: filtered passthrough (Umbrel + CasaOS)

- **Status:** proposed
- **Date:** 2026-05-16
- **Trigger:** Sprint 21 E2E install verification + a viability analysis on 2026-05-16 surfaced three structural problems with the existing live `umbrel-apps` integration: arbitrary-code RCE risk (62 apps ship `exports.sh` that umbrelOS dot-sources into the host shell + 79 apps ship executable `hooks/*` that run as the orchestrator's user); license uncertainty (the `umbrel-apps` repo has no `LICENSE` file and the curated catalog is plausibly compilation-protected); and an ongoing "many apps don't work" class because PowerLab doesn't reproduce Umbrel's runtime primitives (`app_proxy` sidecar, injected `${APP_*}` env, Tor coupling, host hooks).

## Context

PowerLab inherited **two** external community catalog sources from its CasaOS lineage and subsequent Umbrel-apps integration:

1. **CasaOS upstream** — `cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip`, fetched by `backend/app-management/service/appstore.go` at startup. Heavyweight: the on-disk extraction is the heaviest single dir under `/var/lib/powerlab/`. Same architectural risk class as Umbrel (hooks, exports, no curation) and unmaintained on PowerLab's side for several sprints.
2. **Umbrel community catalog** — the headline source covered in the rest of this ADR.

Both sources sit in the same trust position: third-party community-maintained content with no vetting on PowerLab's side. The decision applies uniformly to both.

PowerLab inherited the Umbrel community catalog as the headline app store source. Over Sprints 6-20 the integration accreted a transform layer (`backend/sync-catalog/`) that ingests `docker-compose.yml` + `umbrel-app.yml` and applies PowerLab-specific rewrites: strip the `app_proxy` sidecar, substitute `${APP_DATA_DIR}` to the PowerLab namespace, rewrite v1 underscore hostnames to compose v2 service-name aliases (Sprint 21 #418), substitute URL-embedded host placeholders at install time (Sprint 21 #426), and chmod 0o777 every bind-mount source dir (Sprint 21 #427). These transforms cover the common case but the apps that "don't work" are precisely the ones whose function lives in the parts PowerLab can't cleanly rewrite — most importantly the `hooks/` and `exports.sh` execution path.

The 2026-05-16 analysis confirmed concrete RCE-shaped content **in the catalog today** — e.g. `stalwart/hooks/pre-start` does `curl -fsSL "$URL" -o script && chmod 0755 && exec` and `wget -qO- "$URL" | sh`. In umbrelOS, the `app-script` module dot-sources every app's `exports.sh` into the host shell and executes `hooks/*` as the orchestrator's user. Mirroring that execution model would make every catalog sync a remote-code-execution supply-chain channel.

Two things make the right answer non-obvious:

1. Many apps need their hooks to actually function. Laravel apps need `storage/framework/{cache,views,sessions}` pre-seeded; some apps create their admin user in `post-start`; some generate keys in `pre-install`. Without hooks, ~25% of the catalog installs but fails to start.
2. The catalog has real value — 330 curated apps with icons, descriptions, and tested compose. PowerLab can't realistically replicate that breadth from scratch.

## Decision

PowerLab keeps integration with the Umbrel community catalog as a **disabled-by-default toggle**, with a **hard sync-time filter** that removes apps which would require PowerLab to execute non-PowerLab code.

The CasaOS upstream catalog source is **removed entirely** — same trust class as Umbrel but with no operator demand and no curated upstream maintainer for PowerLab's use case. Apps installed from the CasaOS source before this ADR continue to run unchanged; the catalog source is dropped from `/v2/app_management/appstore` and the disk extraction can be reclaimed.

Image-content seeding for bind-mounts (the "Laravel storage" class — bind-mount source dir overlays image content) is a **Tier 1 PowerLab transform**, not a hook execution. PowerLab runs `docker create` against the image + `docker cp` to seed the bind-mount source from the image's own content. License-clean (image content is third-party docker hub, not catalog code), code-clean (PowerLab Go), works for Tier 1 apps regardless of hook presence.

### Principles

1. **Compose-only ingestion.** `sync-catalog` ingests `docker-compose.yml` + `umbrel-app.yml`. Never `exports.sh`. Never `hooks/`. The pipeline is provably code-free for upstream content; PowerLab's own transforms are the only mutation layer.

2. **Hard filter at sync time.** Apps that ship a `hooks/` directory or `exports.sh` file in the upstream repo are **not ingested at all**. They do not appear in the PowerLab UI. Operator sees a smaller, safer catalog rather than a larger one with surprise behaviour.

3. **Toggle default OFF.** PowerLab ships with zero Umbrel content. The operator explicitly enables the integration in `Settings → Catalog → Enable Umbrel community catalog`, gated by a one-time modal that explains the trust posture ("PowerLab does not audit these apps; the upstream catalog is community-maintained; install at your own risk").

4. **Live passthrough when enabled.** Listings, icons, descriptions, and release notes are fetched live from upstream (HTTP cache 1h) and rendered as-is. PowerLab does not vendor a snapshot, does not rewrite prose, does not bulk-redistribute curated content.

5. **No root, ever.** PowerLab never executes code that would require root, `sudo`, or filesystem mutation outside the app's own data directory. This is the floor that justifies dropping the filter — if PowerLab is willing to execute it, it must be PowerLab code, not arbitrary upstream bash.

6. **PowerLab transforms still apply.** The Sprint 21 install transforms (hostname rewrite, host-placeholder substitution, bind-mount chmod) are PowerLab-authored Go code under our license. They run on every install regardless of toggle state, both for new Umbrel installs and for any catalog source we add later.

### Expected coverage

Based on the 2026-05-16 catalog scan (330 apps total):

| Tier | Criterion | Apps (est.) | % |
|---|---|---:|---:|
| Available | No `hooks/`, no `exports.sh` | ~250 | 75% |
| Filtered | Has `hooks/` or `exports.sh` | ~80 | 24% |
| Already-soft-rejected (ADR-0024) | Bitcoin / Lightning / Tor-coupled | (overlap with Filtered) | — |

### License posture

- Toggle OFF default → no shipping content authored by Umbrel
- User-initiated enable → consent recorded; PowerLab acts as intermediary
- Live HTTP passthrough → fetch happens at user's request (browser-analogue)
- Transforms are factual edits to functional data (compose YAML is largely descriptive of container config; thin-copyright territory)
- We do not persist Umbrel's prose descriptions / release notes beyond the 1h cache TTL
- Catalog repo has no `LICENSE` — this is a documented finding, the mitigation is the consent + non-persistence model

This is not legal advice; the conservative engineering posture (toggle-gated, non-persistent, user-initiated fetch, no bulk mirroring) sidesteps the bulk of the question regardless of the legal read.

### UX

- **Settings → Catalog pane** (new): toggle disabled by default.
- **On-enable modal** (one-time, dismissible only with explicit "I understand"): explains the trust posture.
- **Per-install confirm** for any Umbrel-sourced app: small banner "From the community catalog (PowerLab does not curate this app)". No bash visible — there's nothing to show because hooks are filtered.
- **Filtered-app handling**: apps that fail the filter do not appear in browse. A direct URL or search by exact ID renders an explanation page "This app uses init scripts that require operations PowerLab does not allow. [Why?](explainer link)". No "install anyway" override.

### Existing installed apps

Apps already installed from the Umbrel catalog before this ADR's implementation continue to run unchanged. Their compose files live in `/var/lib/powerlab/apps/<name>/docker-compose.yml`, decoupled from any live catalog state. The toggle controls the *catalog browse + install* surface, not the runtime state of installed apps. Upgrades for legacy installs are best-effort: badge marks them as "legacy: from community catalog", upgrade flow only fires when the toggle is enabled.

### Future work (NOT in this ADR's scope)

- **Tier 2 sandbox** (Sprint 23+): a follow-up ADR (likely ADR-0039) will design an execution sandbox that lets PowerLab run a curated subset of upstream hooks in a constrained container (no host access, no root, no network egress, hash-pinned). This would recover ~50 of the ~80 filtered apps. Independent of this ADR — landing it does not change the decision here, only the filter threshold.
- **`x-powerlab.verified` annotation**: per-app badge marking apps PowerLab has tested end-to-end. Lightweight curation that doesn't require a separate catalog.

## Consequences

**Positive:**

- Zero RCE attack surface from the catalog. PowerLab never executes upstream bash.
- License posture is defensible. PowerLab does not bulk-redistribute unlicensed content.
- Maintenance burden is zero — PowerLab doesn't operate its own catalog.
- Operator UX is honest: "Community catalog, you enabled it, you accept the implications."
- Sprint 21's install transforms (hostname / host-placeholder / bind-mount) remain useful for every install, regardless of source.

**Negative:**

- Effective available catalog drops from 330 to ~250. The Bitcoin/Lightning/Tor-coupled cluster was already soft-rejected (ADR-0024), so the additional drop is mostly from the `hooks/`-bearing apps that don't overlap with the existing soft-reject set — Laravel apps needing seed, apps with admin-creation hooks, apps that template config from env at first run.
- Some apps that PowerLab users may expect to "just work" (because they work on Umbrel) will be absent from PowerLab's view. The explainer page is the mitigation but it's a UX-level friction.
- Drift from Umbrel: future Umbrel features that assume the hook execution model will widen the gap. This ADR makes that drift explicit and accepted.
- Narrative trade-off: "smaller catalog" reads as a competitive disadvantage vs Umbrel/CasaOS. Counter: "smaller but reliable + safe" is a different and defensible position.

**Neutral:**

- Existing Umbrel-sourced installs continue to run. No data migration.
- The decision is reversible: a future ADR could replace the hard filter with the Tier 2 sandbox without breaking any operator's install.

## Supersedes

- The implicit "live track `umbrel-apps` + apply transforms" model that grew across Sprints 6-20.
- The Sprint 19 candidate for "PowerLab-native curated catalog" — superseded by the toggle-passthrough approach, which achieves a similar safety/curation outcome with dramatically less platform-side work.

## Tracking

Implementation issues:

- **#432 — Hard filter at sync time** — `sync-catalog` skips apps with `hooks/` or `exports.sh`. Lint test enforces no-emission of these artifacts.
- **#433 — Settings → Catalog toggle** — UI toggle + persisted setting + on-enable modal.
- **#434 — Live-passthrough listing** — replace the cron pull with on-demand fetch + 1h cache.
- **#435 — Per-install confirm banner** — small UI element on Umbrel-sourced app pages.
- **#436 — Filtered-app explainer page** — public-facing explanation, linked from "filtered" UI states.
- **#437 — Legacy install marker** — badge for apps installed before the toggle existed.
- **#438 — Remove CasaOS upstream catalog source** — drop the `cdn.jsdelivr.net` zip fetcher + extraction. Existing installs continue running with "legacy: from CasaOS catalog" badge.
- **#428 (recategorized) — Image-skeleton-seed Tier 1 transform** — `docker create` + `docker cp` to seed bind-mount source from image content when source is empty. Closes the "Laravel storage" bug class without requiring hook execution.

Validation extensions (related, separate issues):

- **#439 — Pre-install image manifest validation** — `docker manifest inspect <image>` before triggering pull; surface clear "image not found / platform mismatch" instead of a stuck install.
- **#440 — Post-install healthcheck watch** — wait for declared healthcheck to reach healthy in N seconds; mark "install failed" in UI rather than "running but broken".

Future (separate ADRs):

- **ADR-0039 (candidate):** Execution sandbox for Tier 2 hook-bearing apps. Recovers the ~50 apps filtered today that have benign hooks.
- **`x-powerlab.verified` annotation contract.**

Related:

- **ADR-0024** — Umbrel catalog clean-room import (default-deny Bitcoin/Lightning categories) — preserved; this ADR extends the rejection set with the hook-based signal.
- **#428** — Laravel storage seed bug (an example of why Tier 2 sandbox would matter). Stays open as a Tier 2 motivating case.
- **#429** — Umbrel integration strategy study (this ADR is its resolution). Closes when this ADR is merged.
