# Sprint 23 retrospective — the last sprint

**Cut**: v0.7.1 (hardening release)
**Dates**: 2026-05-17 → 2026-05-18
**Headline**: Hardening + process maturity + sprint→release cadence pivot
**Status**: **FINAL SPRINT**. After this retro + v0.7.1 cut, planning organises
around releases, not sprints (memory `feedback_sprint_23_is_last_releases_take_over`).

## Why this is the last sprint

Maintainer-set 2026-05-18:

> "essa vai ser a ultima sprint depois nao vamos ter mais sprints e as
> releases podem der um escopo maior"

The sprint cadence served well through 22 prior sprints (3 of which we
backfilled in #470 — Sprints 13-18 had no retro doc and got reconstructed
from git log). Going forward each release becomes the planning unit and
each release carries larger scope than a past sprint. The discipline that
worked under sprint cadence (TDD, ADR for decisions, retro for memory,
explicit cut authorisation) transfers — only the time unit changes.

## What we set out to do (Sprint 23 charter)

Three concurrent tracks, all sized to fit a normal sprint:

1. **Close the #450 install-path bug class** — Sprint 22 ADR-0039 shipped
   but the install/upgrade path bypassed the ADR entirely. The first 3-4
   PRs target this.
2. **Telemetry pause + Power pane operator UX** — user-reported sidebar
   freeze on `/settings` and `/files` + missing operator surface for
   service restart / host power
3. **Process recovery** — discovered 6 missing retro docs + ADR-0034
   observability skeleton that lost track since Sprint 17

Then mid-sprint the **K8s-grade audit** discussion expanded scope into
hygiene baseline (ADR-0040), coverage cadence rule, and the
maintainer-driven realisation that this would be the last sprint.

## What shipped (20+ PRs)

### Track 1 — #450 install path bug class

| PR | Title |
|---|---|
| #451 | install: wipe-then-copy community-catalog + drop post-install upstream sync |
| #452 | app-mgmt: boot migration removes catalog orphans on upgrade |
| #455 | app-mgmt: migration removes legacy workdir on disk (parity with UnregisterAppStore) |
| #457 | ci(install): upgrade-scenario integration test (OAuth-blocked, manual merge) |

### Track 2 — Operator UX

| PR | Title |
|---|---|
| #454 | ui: refcount system-store polling — sidebar telemetry survives route nav (#453) |
| #466 | core: power-action endpoints for Settings Power pane (#260) |
| #468 | ui: Settings → Power pane for service/host actions |

### Track 2.5 — Lockout defences (operator flagged risk mid-sprint)

| PR | Title |
|---|---|
| #469 | docs+ci: lockout recovery doc + systemd StartLimit hardening |

### Track 3 — Process recovery

| PR | Title |
|---|---|
| #470 | docs(audits): backfill retrospective for sprints 13-18 |

### Track 4 — Catalog + sweep work

| PR | Title |
|---|---|
| #456 | ci(catalog): flip safety lint to strict |
| #458 | ui(catalog): Settings → Catalog copy refined for ADR-0039 + #450 |
| #459 | app-mgmt: boot migration tags pre-v0.7.1 installs with legacy marker (#437) |
| #460 | dev: make stage-build for Linux/amd64 hot-swap from any dev OS (#414) |
| #461 | docs(audits): USB/SD auto-mount gap analysis (#416 Phase A) |
| #467 | chore: CasaOS rebrand residue sweep |
| #476 | feat(usb): local-storage-helper.sh + udev rule + shipping (#464 Phase C) — OAuth-blocked |

### Track 5 — K8s-grade hygiene baseline (mid-sprint expansion)

| PR | Title |
|---|---|
| #471 | docs(adr): 0040 proportional engineering hygiene baseline |
| #472 | ci: golangci-lint warn-only baseline + Dependabot weekly |
| #473 | ci: govulncheck per service in CI matrix (warn-only after first run found 10 CVEs) |
| #474 | docs(adr): amend 0040 with explicit Coverage cadence rule |
| #475 | test(ui): coverage push +3.43pp — 6 surfaces concluded (cadence rule pass 1) |

Plus this PR (sprint-23-wrap-up): final ADR-0040 amendments + this
retro doc + v0.7.1 release-manifest summary.

## The big strategic shifts

### 1. ADR-0040 — proportional engineering hygiene baseline

External audit (2026-05-18) compared PowerLab to Kubernetes-grade
hygiene + listed 6 gaps. Adopted 4 families (SAST, lint, metrics+health,
test coverage). Explicitly deferred 3 (API versioning, KEP, public
disclosure program). The ADR became the reference point for future
"should we adopt X from $project" questions.

Mid-flight amendments:
- Coverage cadence rule (+10pp per surface until estabilidade)
- Govulncheck warn-only first run (after CI found 10 CVEs)
- Cadence reframe to per-release (after sprint era ends)

### 2. Coverage cadence rule

> +10 percentage points per surface per sprint (now per release) until
> realistic ceiling concludes that surface. Sprint CI fails on regression.

Concluded surfaces in this sprint's coverage push (PR #475):

| Surface | Coverage |
|---|---|
| `src/lib/api/catalog.ts` | ~100% |
| `src/lib/api/logs.ts` | ~100% |
| `src/lib/components/settings/CatalogPane.svelte` | ~80% |
| `src/lib/components/settings/PowerPane.svelte` | ~75% |
| `src/lib/components/layout/AppHeader.svelte` | ~95% |
| `src/lib/components/files/Breadcrumbs.svelte` | ~85% |

Aggregate delta: +3.43pp (target was +10pp, capped by god-files that
defer to coverage-first refactor).

### 3. Lockout defences (5 layers, 3 shipped this sprint)

| Layer | Status | PR |
|---|---|---|
| L1 — UI shutdown opt-in + modal acks | ✓ shipped | #468 |
| L2 — systemd StartLimit hardening | ✓ shipped | #469 |
| L3 — operator-facing recovery doc | ✓ shipped | #469 |
| L4 — backend delayed-exec gateway restart | deferred to v0.7.2 | — |
| L5 — backend preflight + UI consume | deferred to v0.7.2 | — |

### 4. Process recovery

7 missing sprint retros (Sprint 13-18 + 22) caught + backfilled
(#470 covers 13-18; 22 actually existed but was filed differently —
audit was partially wrong about 22; 5 stays out of scope).

Lesson encoded: **every release ships its retro/release-notes doc
as a companion PR**.

## What didn't work / lessons

### The "tudo importante v0.7.1" anti-pattern

Mid-sprint the maintainer asked "can we ship everything important in
v0.7.1?" Honest answer was no — 3-4 weeks of focused work, monster
release risk, coverage push needs time. Recommendation accepted:
hardening basics now, larger features (Layer 4/5, ADR-0034 MCP,
app update UX, USB Phase D) deferred to v0.7.2.

This is what the **sprint→release cadence pivot** captures: future
releases are sized for larger scopes by design, so this kind of
discussion happens at planning time, not mid-sprint.

### Govulncheck strict-day-1 was too aggressive

First CI run found 10 CVEs in existing deps. Strict-day-1 would have
blocked every PR. Amendment to warn-only first run + per-service
dep-bump backlog for v0.7.2.

ADR-0040 already had the escape hatch: "If too aggressive, ADR
amended, not implementation rolled back." That's what happened.

### OAuth scope on `.github/workflows/ci.yml`

Two PRs (#457 #476) shipped clean but couldn't merge via `gh pr merge`
because the local OAuth token lacks `workflow` scope. Manual web merge
required. This isn't a sprint failure — it's an environment limit —
but it's worth knowing for v0.7.2: PRs touching workflow files take an
extra manual step.

### god-file refactor still deferred

apps/+page.svelte (1561 LOC), settings/+page.svelte (760), Sidebar.svelte
(697), files/+page.svelte (474), dashboard/+page.svelte (387) all stay
at 0% coverage. ADR-0040 explicitly defers god-file splits until
coverage ≥50% to avoid silent regressions — chicken-and-egg the cadence
rule will work through over multiple releases.

## Memories created this sprint

- `feedback_adr_needs_enforcement_pass` — Sprint 22 didn't audit install/
  upgrade/packaging for ADR-0039; we paid for it in Sprint 23 PRs #451 #452 #455
- `feedback_version_cadence_v07_to_v030` — never anchor work to specific
  patches; v0.8.0 only after v0.7.30
- `feedback_coverage_cadence_rule` — the +10pp / surface rule
- `feedback_sprint_23_is_last_releases_take_over` — sprint era ends; releases
  take over as planning unit

## Stats

- 22 PRs merged + 3 OAuth-blocked + this wrap-up PR
- 4 new memory entries (recent record)
- 1 new ADR (0040) with 3 amendments mid-sprint
- 2 backfill docs (#461 USB audit, #470 sprints 13-18 retros)
- Coverage delta +3.43pp aggregate
- 6 surfaces concluded at estabilidade
- 0 rollbacks
- 0 cut authorisations made without explicit user approval

## E2E pre-release verification

**Required before v0.7.1 cut** (memory `feedback_no_ship_before_tests` +
`feedback_pre_release_coverage_gate`):

- [ ] SSH .142 + deploy v0.7.1 candidate binaries
- [ ] Verify USB helper present at /usr/share/powerlab/shell/local-storage-helper.sh
- [ ] Verify Power pane visible + Restart per-service works (Reboot/Shutdown gates)
- [ ] Verify telemetry doesn't pause on `/settings` or `/files` navigation
- [ ] Verify lockout recovery doc reachable in `/usr/share/powerlab/docs/`
- [ ] Hot-plug a USB → confirm auto-mount under /mnt/powerlab/<label>
- [ ] Smoke: install + uninstall adventurelog without errors

Cut sequence post-E2E:

1. Maintainer authorises "cut v0.7.1" explicitly (memory `feedback_require_explicit_release_auth`)
2. Cut commits the manifest summary, tags v0.7.1, pushes tag
3. Release workflow builds tarballs + manifest.json + GitHub release
4. Confirm release published

## Deferred to v0.7.2 release (no more sprints)

| Item | Type |
|---|---|
| Layer 4 backend delayed-exec gateway restart | feature |
| Layer 5 preflight + UI consume | feature |
| Bug triage #402 #386 | cleanup |
| Per-service govuln dep bumps (10 CVEs) | security |
| god-file split + ongoing coverage push | refactor |
| #442 app update UX | feature |
| Logo refinement | UX |
| ADR-0034 MCP observability impl (Step 2+) | headline enterprise |
| USB Phase D Files sidebar (#462) | feature |
| USB Phase E hardware E2E (#465) | test |
| ADR-0040 lint family ratchet (one family per release) | hygiene |

## Reference

- ADR-0040: `docs/decisions/0040-proportional-engineering-hygiene-baseline.md`
- USB/SD audit: `docs/audits/usb-sd-automount-gap-2026-05-17.md`
- Sprints 13-18 backfill: `docs/audits/sprints-13-18-backfill.md`
- Lockout recovery: `docs/operations/lockout-recovery.md`
- Memories: see new entries above plus existing ones referenced inline
