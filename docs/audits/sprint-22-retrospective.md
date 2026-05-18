# Sprint 22 retrospective (catalog model pivot → v0.7.0)

**Date:** 2026-05-17
**Predecessor:** Sprint 21 (`sprint-21-retrospective.md`)
**Target release:** v0.7.0 (minor — catalog model change is user-visible)
**Theme:** the ADR-0038 → ADR-0039 pivot. Started Sprint 22 implementing a toggle + passthrough integration with Umbrel; mid-sprint review rejected the "show warning + operator confirms" trust model on principle and pivoted to a fully PowerLab-native curated catalog.

## Shipped

| PR | Title | Status |
|---|---|---|
| #441 | sync-catalog hard hook filter (#432) | merged |
| #443 | remove CasaOS upstream catalog source (#438) | merged |
| #444 | image-skeleton-seed bind-mount transform (#428) | merged |
| #445 | ADR-0039 — PowerLab-native curated catalog (supersedes ADR-0038) | merged |
| #447 | compose security lint script + CI gate | merged |
| #448 | initial PowerLab-curated catalog seed (4 apps) | merged |
| #446 | Settings → Catalog source management UI | merging |

7 PRs shipped (started planning 8 in the ADR-0038 frame; pivoted mid-sprint).

## Pivot context: ADR-0038 → ADR-0039

Started Sprint 22 implementing ADR-0038 (toggle + passthrough): keep the Umbrel catalog live, gate behind an opt-in setting, hard-filter apps shipping `hooks/` or `exports.sh`, surface warnings on per-install. Built PR #441 (hard hook filter), PR #443 (CasaOS removal), PR #444 (image-skeleton-seed), and started PR 4 (Settings toggle UI).

Mid-sprint, user surfaced a strategic concern: the "show warning + operator confirms" pattern is not an acceptable trust model for PowerLab, especially as the project pivots toward the enterprise market. Memory `feedback_security_is_priority` captures the rule: smaller-and-safer beats larger-and-warned. Toggle UI was discarded (never pushed). ADR-0038 was superseded by ADR-0039 (PowerLab-native curated catalog, no live integration, no toggle).

What survived: PR #441 (hard hook filter — universal safety gate), PR #443 (CasaOS removal — aligned), PR #444 (install-time transform — orthogonal). What was added post-pivot: PR #447 (compose security lint), PR #448 (initial curated seed of 4 apps), PR #446 (catalog source management UI with "Add custom URL" admin escape hatch).

## Memories added this sprint

- **`feedback_security_is_priority`** — `"show warning + operator confirms"` is NOT a security model. Smaller-and-safer beats larger-and-warned. Mid-sprint trigger.
- **`project_enterprise_pivot`** — PowerLab targets enterprise market (not just homelab). MCP becomes headline. Security/audit/compliance gain priority. HTTPS+v1.0+SSO need revisit. The lens is "would enterprise IT accept this in production?" — pacing is "destination, not deadline".

## What went well

- **Pivot landed cleanly.** ADR-0038 → ADR-0039 cost 1 discarded PR (toggle UI, never pushed). PRs already merged (#441, #443, #444) aligned with both framings, so no rework. The architectural shift was captured in a new ADR rather than amending the old one — clearer audit trail.
- **TDD held throughout the pivot.** Every PR landed with failing tests first (hook filter, image seed, safety lint). No "we'll add tests later" debt.
- **Security gate at multiple layers.** Hard hook filter at sync (PR #441), compose security lint at CI gate (PR #447), install-time transforms PowerLab-authored (#444). Defense in depth.
- **Initial curated catalog is honest about size.** 4 apps verified > 240 apps with disclaimers. Operators see a smaller-but-trustworthy set.
- **Custom-URL escape hatch preserves operator agency** without forcing PowerLab into intermediary role for unaudited content.
- **Memory recall is paying compound interest.** Sprint 21's `feedback_no_ship_before_tests` shaped the initial-seed scope (only ship apps PowerLab actually verified). `feedback_clean_up_planted_test_data` informed the legacy-install marker design.

## What went poorly

- **Architecture churn within a sprint** is expensive emotionally even when small in code. Mid-sprint pivots should be done deliberately, not as a knee-jerk to a new concern. We're lucky the wasted work was bounded (one local-only PR + obsoleting 4 issues).
- **Custom-URL admin escape hatch was approved via "show warning + acknowledge".** The same pattern that motivated the pivot away from ADR-0038's toggle. Defensible because operator-added catalogs are explicit per-source registrations (not a global on/off), but the modal still asks operator to click through. Acceptable v1; revisit if it becomes a foot-gun pattern.
- **The legacy 240 → 4 wipe is a large diff** to review even when most of the change is bulk deletion. Reviewer fatigue real risk.
- **No E2E verify on .142 this sprint.** The 4 curated apps in PR #448 are tagged `verified: 2026-05-15` from Sprint 21's E2E session. Sprint 22 itself didn't re-verify install on the post-pivot binaries. Acceptable since the install transforms didn't change behavior for these apps, but worth flagging.

## Bug classes closed

| Class | Fix |
|---|---|
| Upstream hook/exports execution risk | PR #441 hard filter |
| External catalog source maintenance burden | PR #443 (CasaOS) + PR #448 (Umbrel local) removal |
| Bind-mount overlay (Laravel storage class) | PR #444 image-skeleton-seed |
| Compose-level privilege escalation | PR #447 safety lint |
| Operator "untrusted catalog" UX ambiguity | PR #446 source management UI with explicit badges |

## Bug classes NOT closed (Sprint 23+ candidates)

| Class | Notes |
|---|---|
| Safety lint strict mode flip | Was warn-only because legacy catalog had 45 findings. Now 0 findings after PR #448 wipe; flip in Sprint 23 PR 1. |
| Legacy install marker UI | #437 still open — apps installed from removed sources continue running but lack a UI distinction yet. |
| #439 pre-install image manifest validation | Carry-forward from Sprint 21 planning. Independent of catalog model. |
| #440 post-install healthcheck watch | Same. |
| Sandbox model for opt-in hook execution (Tier 2) | Future ADR. Not started; not motivated yet. |

## Memory updates landed

- Added `feedback_security_is_priority` (mid-sprint pivot)
- Added `project_enterprise_pivot` (strategic context, pacing-deliberate)
- ADR-0038 superseded; legacy issues #433/#434/#435/#436 closed with reference

## Tracked for Sprint 23

| Item | Source |
|---|---|
| Flip safety lint to strict (1-line CI env change) | this retro |
| Legacy install marker for pre-curation apps (#437) | carry-forward |
| Pre-install image manifest validation (#439) | carry-forward Sprint 21 |
| Post-install healthcheck watch (#440) | carry-forward Sprint 21 |
| Mac build fix (#414) | carry-forward Sprint 21 |
| USB/SD auto-mount (#416) | deferred — user explicitly "fica pra depois" |
| Expand curated catalog beyond founding 4 | per-app PRs as the team picks them |
| #295 apps/+page.svelte split | tech debt, low priority |
| Test seam handling for deadcode | from ADR-0037 |

## Cut decision

v0.7.0 justified because:
- New catalog architecture (user-visible: store source list, "Unaudited" badges, fewer apps shown)
- Initial curated catalog (user-visible: install set changes from 240 → 4)
- ADR-0039 supersedes ADR-0038 publicly
- Multiple CI gates added (hook filter strict, safety lint flip pending)

Not user-visible but worth flagging in release notes:
- Image-skeleton-seed transform unblocks Laravel-class apps on first install (Sprint 21 PR 10 follow-through)
- CasaOS upstream catalog source removed (no operator action needed; existing installs unaffected)

## Bottom line

Sprint 22 closed the catalog model question that had been deferred since Sprint 6+. The pivot from ADR-0038 (toggle + passthrough) to ADR-0039 (native curated) aligned PowerLab's posture with the enterprise pivot: security over breadth, audit over warnings, curation over scale. v0.7.0 ships smaller-but-honest: 4 apps PowerLab is willing to vouch for, an explicit escape hatch for operators who want more, and infrastructure (hard filter + safety lint) that holds the line as the curated set grows.
