# 0039 — PowerLab-native curated catalog (security-first, no external passthrough)

- **Status:** accepted
- **Date:** 2026-05-17
- **Supersedes:** ADR-0038 (Umbrel catalog passthrough — toggle + hard filter)
- **Trigger:** ADR-0038 proposed an opt-in toggle that would let operators turn on live passthrough to the Umbrel community catalog with PowerLab acting as intermediary. Mid-sprint review of that decision (2026-05-17) rejected the "show warning + operator confirms" trust model on principle — see `feedback_security_is_priority`. The conclusion was that any architecture which routes through PowerLab the risk of executing third-party code (even hooks/exports.sh which we'd hard-filter) keeps PowerLab on the hook for the trust posture of an unaudited upstream. The clean answer is to **not route** at all.

## Context

PowerLab inherited two upstream community catalog integrations across its CasaOS-and-then-Umbrel lineage. Sprint 21's E2E install verification + the 2026-05-16 viability analysis surfaced four structural problems:

1. **Security**: 62 Umbrel apps ship `exports.sh` (dot-sourced into umbrelOS host shell) and 79 ship `hooks/` (executed as the orchestrator's user). Concrete RCE-shaped content in the catalog today (e.g. `stalwart/hooks/pre-start` does `wget -qO- "$URL" | sh`).
2. **License**: the `umbrel-apps` repo has no `LICENSE` file. Bulk redistribution of the curated catalog (descriptions, gallery, release notes) sits in a grey area.
3. **Breakage**: ~22% of upstream apps have cross-app dependencies (Bitcoin/Lightning cluster) that PowerLab doesn't replicate; another large chunk needs hook execution to function.
4. **Drift**: Umbrel's runtime evolves around assumptions PowerLab doesn't share. Tracking upstream means perpetual transform-fragility firefighting.

ADR-0038 (now superseded) proposed solving this by making the integration opt-in via a toggle and hard-filtering hook-bearing apps. Mid-sprint we reconsidered: a toggle still leaves PowerLab as the intermediary serving content authored, curated, and (without hooks/exports) only partially functional from upstream. The trust posture is ambiguous and the maintenance is recurring.

## Decision

PowerLab ships a **native, vendored, curated catalog**. There is **no live integration** with Umbrel, CasaOS, big-bear, or any other external community catalog. There is **no toggle** for an "experimental" passthrough.

Operators who want a third-party catalog can add a **custom catalog URL** in Settings — explicit admin action, one-time acknowledgement that PowerLab does not audit it, no further hand-holding. This is the escape hatch, not a feature.

### Principles

1. **Single default catalog source: PowerLab-curated.** Ships with PowerLab. Lives in `community-catalog/Apps/` in the repo. Per-app entries are reviewed before commit.

2. **Security gate per-app, enforced.** Every entry passes the existing `sync-catalog` filter (ADR-0038 PR #432 — hard filter on hooks/exports.sh) PLUS an additional compose-level security lint (no `privileged: true`, no `/var/run/docker.sock` bind, no `network: host` unless declared, no `cap_add: ALL`, no obvious foot-guns). Lint script gates CI on new catalog entries.

3. **No upstream tracking.** PowerLab does not pull from `umbrel-apps`, `CasaOS-AppStore`, `big-bear-casaos`, or any successor on any schedule. New apps land because the team decides to add them, not because upstream added them. This is an **explicit non-feature** — operators expecting Umbrel parity will see fewer apps and that's correct.

4. **Apps added per PowerLab demand, not upstream cadence.** Each new app is a deliberate PR. Discovery sources (which apps to add) can include upstream catalogs as inspiration but never as ingestion — we look, we decide, we author.

5. **Custom URL escape hatch.** Settings → Catalog → "Add custom catalog URL". Operator pastes a URL, types "I understand", PowerLab fetches and serves WITHOUT applying the security audit. The escape hatch is opt-in per URL, and the catalog renders with a permanent visual marker ("unaudited").

6. **Initial curated set ships non-empty.** A first wave of ~15-30 apps PowerLab has tested end-to-end (passes security lint, installs cleanly, reaches healthy on staging). PowerLab does not ship with an empty store.

7. **Per-app `x-powerlab.verified` annotation.** Apps that PowerLab has install-tested get a `verified: <date>` field. UI shows a badge. Apps without it show "untested" — both still installable, both still in the curated set.

### Security model

This is the load-bearing decision; the rest follows.

**What PowerLab will run:**
- Compose YAML it has authored or audited
- Docker image pulls (third-party, vendor-pinned with digest)
- PowerLab Go orchestration code (install transforms, secrets, host substitution, bind-mount perms, image-skeleton-seed)

**What PowerLab will NEVER run:**
- `hooks/*` scripts of any kind (from anywhere)
- `exports.sh` dot-sourcing of any kind
- Operator-pasted bash with "show + confirm" UX
- Untrusted catalog scripts even in sandbox (until ADR-0040+ defines a sandbox model with explicit threat model)

**License posture:**
- Compose YAMLs are functional content (thin copyright on individual files); curation/arrangement is PowerLab's
- Descriptions: PowerLab-authored from scratch OR rewritten via AI from public information then human-reviewed. We do not redistribute upstream prose verbatim.
- Icons: PowerLab-rehosted set OR consciously hot-linked with attribution. Not bulk-mirrored.
- Images: pulled by operator's daemon from public registries; PowerLab is not a registry.

### UX

- **Settings → Catalog pane.**
  - List of registered catalog sources. Default: 1 entry, "PowerLab Curated", non-removable.
  - "Add custom catalog URL" button. Modal: URL + one-time acknowledgement modal ("PowerLab does not audit catalogs added here. Apps may include init scripts, network egress, or behavior PowerLab does not validate. Continue?"). On confirm, source added with permanent "unaudited" badge.
  - Remove button for operator-added sources only.
- **Store browse.** Apps from PowerLab curated catalog render normally. Apps from operator-added sources render with the "unaudited" badge throughout (browse, detail, install confirm).
- **No "filtered" UI for upstream apps.** Operators don't see what PowerLab decided not to ship — that's discovery noise, not value.

### What we keep from ADR-0038's work

- **PR #441 (hard hook filter):** retained as the safety gate on ANY ingestion path (including future operator-added custom URLs). It's the "no hooks" floor.
- **PR #443 (remove CasaOS sources):** retained AS-IS — CasaOS was already a non-default source per ADR-0038's framing; the migration also strips Umbrel default in this ADR's implementation PR.
- **PR #444 (image-skeleton-seed):** retained — install-time fix, orthogonal to catalog source.
- **Settings → Catalog toggle UI (drafted, not pushed):** discarded. No toggle in this model; the toggle was the architecture choice ADR-0038 made that this ADR rejects.

### Existing installs

Apps already installed from CasaOS or Umbrel sources before this ADR continue to run unchanged. Compose files live in `/var/lib/powerlab/apps/<name>/`, decoupled from any live catalog. They're badged "legacy: from removed catalog source" in the Apps UI; upgrades for these are best-effort and do not regress.

### Initial curated set (Phase A scope)

PowerLab ships v0.7.0 with a curated catalog of ~15-30 apps. Selection criteria:

- Passes hard hook filter (no `hooks/`, no `exports.sh`)
- Passes compose security lint (no privileged, no docker.sock, no network host)
- Single-image-namespace where possible (no `getumbrel/*`, no app-specific Umbrel-runtime images)
- PowerLab has install-tested on staging (`.142` box) and confirmed healthy
- Covers a few categories: media (Jellyfin, Plex), files (Nextcloud, Baikal), automation (n8n, Node-RED), self-hosting basics (Gitea, Vaultwarden), AI (Ollama, Open WebUI)

Each app entry has:
- `community-catalog/Apps/<id>/docker-compose.yml` — PowerLab-curated, transforms-applied
- `community-catalog/Apps/<id>/description-powerlab.md` — PowerLab-authored
- `community-catalog/Apps/<id>/x-powerlab.yml` — manifest with `verified: <date>`, category, tagline, etc.

### Per-app discovery workflow (post-Phase A)

When the team decides to add an app:

1. Pick the app from any public source (Umbrel, Docker Hub, vendor docs)
2. Author a compose entry under `community-catalog/Apps/<id>/`
3. Pass the compose security lint
4. Install on staging, verify healthy + functional
5. Mark `x-powerlab.verified: <date>`
6. PR — reviewer checks: compose audit, license sanity on image, basic threat-model walk
7. Merge → ships in next release

This is hand-curated and intentional. There is no automation pretending to scale this — each app is a deliberate decision.

## Consequences

**Positive:**

- Security posture is unambiguous: PowerLab runs PowerLab-authored content.
- License posture is clean: we do not redistribute third-party curated catalogs in bulk.
- Maintenance burden is bounded: we own what we ship; updates are deliberate.
- Operator UX is honest: "this is what PowerLab supports". No toggles, no warnings, no opt-in maze.
- The custom-URL escape hatch preserves operator agency without making PowerLab the trust intermediary.

**Negative:**

- Catalog ships smaller (~15-30 apps vs Umbrel's 330). Competitive narrative needs work.
- "Don't have my favorite app" is a real operator complaint. Mitigations: custom-URL escape hatch + per-PR add-an-app workflow + clear discovery docs.
- One-time cost to build the initial 15-30 entries (compose audit, install test, description write).
- Discovery-from-upstream effort: looking at Umbrel/CasaOS for inspiration is fine; we just stop the runtime dependency.

**Neutral:**

- Reversible: a future ADR could re-introduce a curated upstream-mirror feed if the curation cost outweighs the catalog size pain.
- Sprint 22 in-flight work mostly aligns. PR #441, #443, #444 retain value; PR #4 (toggle UI) discarded; issues #434-#436 closed as obsolete.

## Tracking

Implementation issues (Phase A — Sprint 22):

- **#441 (merged)** — sync-catalog hard hook filter
- **#443 (in CI)** — remove CasaOS upstream catalog source — to be **extended** in a follow-up PR to also drop the Umbrel local default and add the custom-URL escape hatch backend
- **#444 (in CI)** — image-skeleton-seed Tier 1 transform
- **NEW** — compose security lint (`scripts/check-catalog-app-safety.sh`): no privileged, no docker.sock, no network host, no cap_add ALL, etc.
- **NEW** — custom-URL admin UI + backend escape hatch
- **NEW** — initial curated catalog seed: 15-30 PowerLab-authored entries with PowerLab descriptions + x-powerlab.verified
- **NEW** — `legacy: from removed catalog source` install marker (replaces issue #437 partially)

Implementation issues (Phase B — Sprint 23+):

- AI-rewrite description harness (LLM + human-in-loop per app)
- Per-app security audit deepening (image namespace, CVE check, dockerfile review)
- Discovery dashboard ("apps in upstream we haven't yet curated") — informational only

Obsoleted issues (close with reference to this ADR):

- #433 — Settings → Catalog toggle UI (no toggle)
- #434 — sync-catalog live passthrough fetch (no live fetch)
- #435 — per-install confirm banner for Umbrel-sourced apps (no Umbrel source)
- #436 — filtered-app explainer page (no filtered apps visible)

Memory:

- `feedback_security_is_priority` (2026-05-17) — the operator-trust-warning model is rejected on principle.
