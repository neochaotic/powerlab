---
title: "0022 — CasaOS upstream is abandoned; PowerLab takes no new dependencies on it"
status: accepted
date: 2026-05-10
tags: casaos-strip, governance, sprint-5
---

# 0022 — CasaOS upstream is abandoned; PowerLab takes no new dependencies on it

**Status:** accepted
**Date:** 2026-05-10
**Tags:** casaos-strip, governance, sprint-5

## Context

PowerLab forked from [IceWhaleTech/CasaOS](https://github.com/IceWhaleTech/CasaOS)
in early 2025. From the start the strategy was to gradually rename
modules and replace per-component CasaOS code with PowerLab-owned
equivalents (the "strangler" pattern in ADR-0025 and the kill PRs of
Sprints 1-3). Sprint 5's residue audit (`docs/audits/casaos-residue-2026-05-10.md`)
shows zero remaining `go.mod` dependencies on CasaOS — but multiple
runtime references to CasaOS-hosted infrastructure (`get.casaos.io`,
`api.casaos.io`, `icon.casaos.io`, `cdn.jsdelivr.net/gh/IceWhaleTech/...`)
remained until that sprint started cleaning them up.

The decision needed: do we treat upstream CasaOS as a living source
to track, or as an abandoned ancestor to detach from completely?

## Upstream CasaOS state (verified 2026-05-10 via gh api)

| Repo | Last push | Open issues | Latest release |
|---|---|---:|---|
| `IceWhaleTech/CasaOS` | 2025-08-06 (10 months ago) | 795 | **v0.4.15** (2024-12-19, **1.5 years old**) |
| `IceWhaleTech/CasaOS-AppManagement` | 2025-04-17 (~1 year) | 4 | — |
| `IceWhaleTech/CasaOS-UserService` | 2025-04-17 (~1 year) | 0 | — |
| `IceWhaleTech/CasaOS-Common` | 2026-04-20 (3 weeks) | 0 | — |
| `IceWhaleTech/CasaOS-MessageBus` | (irregular) | — | — |
| `IceWhaleTech/CasaOS-LocalStorage` | (irregular) | — | — |

CasaOS itself: no release in 1.5 years, ~800 open issues, last
commit ~10 months ago. That qualifies as **abandoned** by any
reasonable software-maintenance definition.

The peripheral repos (Common, MessageBus, LocalStorage) get
occasional drive-by commits but no coherent release cadence and no
issue triage. None of them have a v1.0 yet.

The IceWhaleTech organisation appears to be focused on a successor
product ("ZimaOS"); CasaOS is in maintenance-mode-by-neglect rather
than declared end-of-life.

## Decision

PowerLab takes the position that **CasaOS upstream is abandoned**
and acts accordingly:

### 1. NO new runtime dependencies on CasaOS infrastructure

PowerLab will not introduce new code that calls:
- `*.casaos.io` (get / api / icon / cloudoauth / etc.)
- `*.casaos.app`
- `cdn.jsdelivr.net/gh/IceWhaleTech/*`
- `raw.githubusercontent.com/IceWhaleTech/*` (at runtime; codegen
  build-time pulls are tolerated short-term but tracked for vendoring)
- Any other IceWhaleTech-hosted endpoint

Any such reference in code is a build-time error. Lint check via
`scripts/check-no-casaos-runtime-deps.sh` (to be added) runs in CI.

### 2. NO new code dependencies on CasaOS Go modules

PowerLab will not `require github.com/IceWhaleTech/...` in any
`go.mod`. Already enforced de facto since PR #151 finished the
module rename — formalised here as policy. Only paths under
`github.com/neochaotic/powerlab/backend/*` are allowed for
PowerLab-owned code.

### 3. Existing references are deprecated, not maintained

For the residue still referenced in PowerLab (per audit):
- **Delete on sight** when removal is safe (Sprint 5 kill list)
- **Vendor when functionally needed** (upstream OpenAPI specs being
  vendored under `backend/common/api/upstream/` — kill #10)
- **Replace with PowerLab equivalent** when functionality is needed
  but the upstream is abandoned (icon CDN, appstore mirror)
- **Keep but document** for the few cases where upstream is still
  the de-facto data source (the appstore content at
  `casaos.oss-cn-shanghai.aliyuncs.com` — third-party data, not code)

### 4. NO synchronisation with upstream

PowerLab does not pull commits from upstream CasaOS forks. We do
not watch upstream for fixes. If a bug is fixed in CasaOS, that's
fine — we may notice via the audit cycle but treat it as parallel
discovery.

The fork divergence is intentional and one-way.

### 5. Public communication

The README + docs site will state plainly that PowerLab is a fork
of an abandoned upstream. Users should not expect CasaOS commits
to appear in PowerLab; PowerLab releases are independent. The
"CasaOS coexistence" wording (per ADR-0021) describes runtime
coexistence on the same host, not project lineage.

## Rationale

### Why the call is "abandoned" not "slow"

Software with active maintenance ships releases. CasaOS hasn't
shipped one in 1.5 years. Software with active triage closes
issues; CasaOS has 795 open. Software with active development
gets commits beyond drive-by; the recent activity on Common is
README touch-ups and dependency bumps, not features or fixes.

The signal-to-noise of inheriting from a tree that doesn't
release means PowerLab pays the cost of compatibility (every
deviation has to be deliberate) without getting the benefit
(no upstream improvements to pull).

### Why "no new dependencies" instead of "remove all"

Some upstream content is genuinely useful + correctly upstream:
the AppStore catalog (`casaos.oss-cn-shanghai.aliyuncs.com`) is
third-party data the PowerLab app store consumes. Replacing it
would mean curating our own catalog from scratch — out of scope
for "obliterate CasaOS" and contrary to "use upstream where
upstream is the right source of truth."

The line is: **runtime control flow** (auth tokens, update URLs,
icon-rendering CDN) MUST be PowerLab-owned. **Data sources** that
are independently useful (the appstore content) MAY remain on
upstream infra.

### Why a public ADR (not internal note)

If a future contributor opens a PR that adds `cdn.jsdelivr.net/gh/IceWhaleTech/...`
to a Svelte component, the reviewer needs a citable rule, not
"@neochaotic disagrees." This ADR is the citation.

## Consequences

**Positive:**
- Sprint 5 kill list has explicit blessing as policy work, not
  preference. Anyone touching the audit's flagged items knows
  the goal.
- Future PRs that would reintroduce CasaOS dependencies are
  rejected by reference to a stable rule.
- README + marketing can describe PowerLab as a "fork of an
  abandoned project" honestly without pretending we still
  benefit from upstream activity.

**Negative / accepted:**
- We lose the optional path of "merge in an upstream fix" if
  CasaOS resurrects later. Acceptable trade — if upstream
  resurrects, we evaluate then.
- Some functionality that depended on CasaOS infra (icon CDN,
  update channel) needs PowerLab-owned replacements. Already
  scoped in Sprint 5 kill list.

## Alternatives considered

1. **Keep tracking upstream** — discarded. Tracking a non-shipping
   fork costs review cycles for zero value. Issues opened against
   PowerLab can't be resolved by "wait for upstream fix."

2. **Hard fork: rewrite from scratch in PowerLab repo** — discarded
   as too aggressive. The strangler pattern (per ADR-0025) is
   working. Most code in PowerLab has been touched + rebranded
   organically; a from-scratch rewrite would discard tested logic
   for cosmetic gain.

3. **Soft policy ("avoid CasaOS deps when reasonable")** — discarded
   as unenforceable. Without a clear rule, the residue audit will
   need to repeat every 6 months. A bright-line policy + CI lint
   means new violations are caught at PR time.

## Refresh discipline (per ADR-0019)

- Status `accepted` until upstream CasaOS publishes a release that
  changes this analysis (e.g. a v1.0 release, a clear roadmap, an
  active maintainer team). At that point this ADR is reconsidered.
- The "Upstream CasaOS state" table at the top is a snapshot;
  re-verify before citing it. The decision stands regardless of
  refresh — only the snapshot is volatile.
- The residue audit at `docs/audits/casaos-residue-2026-05-10.md`
  is the operational checklist; this ADR is the policy that
  justifies it.

## Reference

- ADR-0025 — strangler pattern justification
- ADR-0021 — Docker label namespace + AppData path (the
  coexistence work this ADR formalises the position behind)
- Issue #67 — original 4-sprint roadmap to v1.0
- `docs/audits/casaos-residue-2026-05-10.md` — the kill list
- Upstream `IceWhaleTech/CasaOS` — fork ancestor; do not depend on
