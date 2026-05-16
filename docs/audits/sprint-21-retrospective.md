# Sprint 21 retrospective (B+A: Logs Phase 4 + catalog sweep → v0.6.16)

**Date:** 2026-05-15
**Plan:** `sprint-21-plan.md`
**Predecessor:** Sprint 20 (`sprint-20-retrospective.md`)
**Target release:** v0.6.16 (downgraded from v0.7.0 — see "Bump downgrade" below)
**Theme:** complete the journald viewer scope open since Sprint 14 + finish the catalog hostname cleanup started in Sprint 20.

## Shipped

| PR | Title | Status |
|---|---|---|
| #418 | catalog hostname sweep — 154 fixes across 65 apps | merged |
| #419 | sync-catalog transform — auto-rewrites future syncs | merged |
| #420 | backend journald SSE endpoint at `/v1/logs/services/{svc}/stream` | merged |
| #421 | frontend per-service tabs + live follow + severity coloring | merged |
| #422 | real-backend Playwright smoke for journald stream | merged |
| #423 | flipped catalog-hostnames lint to strict mode | merged |
| #424 | deadcode quick sweep — 36 dead funcs removed from `backend/core/` | merged |
| #425 | ADR-0037 + delta-strict deadcode gate + docs nav bulk cleanup | merged |
| #426 | install-time host-placeholder substitution (URL-embedded class) | merged |
| #427 | install-time generic chmod 0o777 on bind-mount sources | merging |

**10 PRs shipped.** Plan said 8 originally; PR 7.5 morphed into ADR-0037; PRs 9 + 10 added mid-sprint after E2E install verification surfaced two more bug classes (URL-embedded host placeholders + bind-mount perms).

## Bump downgrade: v0.7.0 → v0.6.16

The sprint-21 plan targeted **v0.7.0** based on two arguments:
1. New user-facing feature surface (per-service journald viewer + severity coloring).
2. Two CI gates flipping strict.

Reality on close:
- Only **one** CI gate flipped strict (catalog-hostnames, PR 6). The deadcode strict flip (PR 7.5) was replaced by delta-strict per ADR-0037 — that's a different model entirely.
- The journald viewer is **additive within Settings → Logs** (a new tab, not a new top-level page). Lower visibility than v0.7.0 implies.

Net: v0.6.16 is more honest. Pre-v1.0 minor numbers should align with user-visible scope changes; v0.6.16 stays in the cumulative-cleanup arc that started with v0.6.11.

## ADR-0037: delta-strict instead of zero-strict

PR 7 sweep stopped at 37 dead funcs in `backend/core/`. Pushing for zero ran into three structural problems:

1. **Mixed-live files** — `pkg/utils/file/file.go` has 16 dead funcs interleaved with 8 live ones. Per-func removal cascades into transitive callers (e.g., `pkg/utils/file/file.WriteToPath` is called by `service/system.go::UpAppOrderFile` which is itself unreachable). Each surgical removal triggers a build-and-test cycle.

2. **Test seams** — `service.NewHealthServiceWithLister` is reported dead by `deadcode` because it's not reachable from `main`, but it's consumed by `health_test.go` as the DI seam for unit tests. Deleting broke the test build. No `//deadcode:ignore` pragma exists; the alternatives are (a) maintain an allowlist file, (b) inline the test seam logic into tests and lose unit testability, or (c) accept the false-positive count in baseline.

3. **Future-feature helpers** — `pkg/docker/digest.go::GetManifest` and `pkg/docker/helpers.go::GetArchitectures` are unused TODAY but are the obvious building blocks for the "upgrade-available indicator" backlog item (#258). Deleting now = reinventing in Sprint 22 or 23.

ADR-0037 (Sprint 21 PR 7.5, #425) captures the decision: replace zero-strict goal with delta-strict gate. Per-service `scripts/deadcode-baseline/<svc>.txt` files cap the historical count; CI fails when count exceeds baseline. Reductions are encouraged (and visible in PR diffs as the baseline ticks down) but not mandatory.

Baselines seeded:
| Service | Count |
|---|---|
| app-management | 9 |
| core | 29 |
| gateway | 0 |
| local-storage | 71 |
| message-bus | 1 |
| sync-catalog | 1 |
| user-service | 2 |

The Sprint 19 PR 5 commitment to flip strict at v0.7.0 is **explicitly retired** in ADR-0037.

## E2E verification (memory: `feedback_no_ship_before_tests`)

Pre-cut gate per `feedback_release_coverage_gate`:

### Journald SSE viewer
- ✅ `curl -i -H "Accept-Encoding: gzip"` against `/v1/logs/services/gateway/stream` on .142 — response 200 OK, `Content-Type: text/event-stream`, NO `Content-Encoding: gzip`, `X-Accel-Buffering: no`, first chunk `: stream-open`, subsequent chunks valid JSON SSE events with `severity` / `message` / `ts_micro` fields. Memory `feedback_sse_test_real_browser_headers` satisfied.
- ✅ Service-name allowlist verified — uppercase `Gateway` rejected at API.
- ⚠️ Playwright real-backend spec times out on body drain (SSE never ends). Spec is correct in intent but needs restructuring to assert headers only — tracked as follow-up.

### Catalog hostname sweep
- ✅ `GET /v2/app_management/apps/solidtime/compose` on .142 returns cooked compose with `DB_HOST: db` and `GOTENBERG_URL: http://gotenberg:3000` (no underscore form).
- ✅ Lint script's 6 self-tests still pass; lint output 0 findings.
- ✅ Live install of solidtime on .142: `solidtime-db-1` + `solidtime-gotenberg-1` reach **healthy** state (proves service-name network alias resolution works). The Sprint 21 PR 1+2+6 catalog hostname fix is **end-to-end verified**.

### URL-embedded host placeholder (PR 9 #426)
- ✅ After PR 9 deployed on .142, install of solidtime produces `APP_URL=http://192.168.18.86:8770` (was `http://:8770` before — Laravel "Invalid URI" crash).
- ✅ Substitution chain verified: install request `Host: 192.168.18.86:8765` → port-stripped → `192.168.18.86` → substituted into all 4 ADP URL refs.
- ✅ 44 affected catalog apps now ship with valid URL env vars at install time.

### Bind-mount perms (PR 10 #427)
- ✅ Post-install inspection on .142: `/DATA/PowerLabAppData/solidtime/data/app` mode = `drwxrwxrwx` (was `drwxr-xr-x` root:root before). The chmod 0o777 generic fix is applied.
- ✅ `solidtime-db-1` (postgres uid 999) starts and reaches healthy — the dir-needs-write class is closed for postgres-style apps too.
- ⚠️ **Solidtime-app-1 still crashes** but with a DIFFERENT class — Laravel needs pre-seeded `storage/framework/{cache,views,sessions}` subdirs which the image ships but the empty bind-mount overlays. **Not in scope for Sprint 21.** Tracked as #428 (Laravel storage seed) and #429 (broader Umbrel integration strategy study).

### Docs build
- ✅ `mkdocs build --strict` passes locally (~0.80s build) after bulk-nav addition + validation overrides for auto-generated godoc surface.

## What went well

- **TDD discipline held.** Every PR landed with failing tests first (PR 2 sync-catalog transform, PR 3 journald handler, PR 5 real-backend smoke). Memory `feedback_tdd_strict` honored.
- **Worktree-per-PR pattern** kept parallel branches clean. Zero cross-pollination bugs across 8 worktrees.
- **Catalog hostname class fully closed.** Sweep + auto-transform + strict lint = three layers of defense. Future upstream Umbrel/CasaOS syncs can't re-introduce the bug class.
- **ADR captured a real architectural shift in real-time.** Instead of forcing the "promised" deadcode strict flip, the team identified why the original goal was wrong and pivoted to a sustainable gate — and documented the pivot.
- **Memory recall** caught two trap classes early: `feedback_sse_test_real_browser_headers` shaped PR 5's spec; `feedback_clean_up_planted_test_data` reminded the cleanup-after-install discipline.

## What went poorly

- **Real-backend Playwright spec for SSE times out** because Playwright's `fetch` drains the entire response body, and SSE streams never end. The spec is structurally wrong — needs to assert headers and abort. Follow-up fix in Sprint 22.
- **Login rate limit** during automation kicked in twice. Reusing a cached JWT helps but breaks when the rate-limit window straddles the test. The user-service rate-limit window is aggressive (10-12 attempts → multi-minute lockout) — operator-friendly but test-hostile.
- **Docs strict mode** was a sleeping bug surfaced by PR 7.5. The `docs.yml` workflow only triggers on `docs/**` changes, so the accumulated 170-warning backlog was invisible until PR 7.5 added one ADR. Took an additional commit to bulk-add the nav + downgrade `omitted_files` / `unrecognized_links` to info for the auto-generated godoc surface.
- **PR scope creep on #425.** Originally just ADR + delta-strict implementation. Ballooned to also include docs nav cleanup + workflow restore. Defensible as "one coherent docs-pipeline fix" but should have been split for review clarity.

## Memory updates

- **No new memory added.** Sprint 21 reinforced existing memories rather than producing new ones. The "Playwright + SSE body drain" gotcha is a candidate for memory if it recurs.

## Tracked for Sprint 22

| Item | Source |
|---|---|
| Fix Playwright SSE smoke to assert headers only | this retro |
| Investigate Mac→Linux build pipeline (#414) | sprint-21-plan |
| USB/SD auto-mount end-to-end (#416) | user-requested 2026-05-15 |
| `/logs` Phase 5 (filter by severity, search, time range, download stream) | #259 follow-up |
| Per-service restart buttons in Power pane (#260) | backlog |
| `apps/+page.svelte` split (1561 LOC god file) (#295) | backlog |
| Audit JSONL migration (#363, ADR-0035) — blocks ADR-0034 observability service | sprint-21 candidate, deferred |
| Optional: bbolt spike (#364) as alternative to JSONL | sprint-21 candidate, deferred |
| `pkg/utils/file/file.go` surgical deadcode pass | this retro / ADR-0037 |
| Test-seam handling for deadcode (allowlist mechanism?) | this retro / ADR-0037 |
| Documentation builds (gen-godoc cross-ref cleanup, re-enable nav strict) | this retro |

## Bug classes closed this sprint

PR 9 + PR 10 added mid-sprint after the planned-PR work uncovered them during E2E install verification. Three install-related classes are now closed:

| Class | Bug | Fix | Surface |
|---|---|---|---|
| Catalog hostname | `<project>_<svc>_<idx>` env-var refs unresolvable under compose v2 | PR 1 (sweep) + PR 2 (transform) + PR 6 (lint strict) | sync-catalog + CI gate |
| URL-embedded host placeholder | `APP_URL=http://${DEVICE_DOMAIN_NAME}:8770` → `http://:8770` → "Invalid URI" | PR 9 (#426) install-time substitution | app-management install |
| Bind-mount perms | Container runtime user can't write to `/DATA/PowerLabAppData/<app>/...` | PR 10 (#427) generic chmod 0o777 | app-management install |

## Bug classes NOT closed (Sprint 22+ candidates)

| Class | Discovered | Tracked |
|---|---|---|
| Laravel storage subdirs missing from empty bind-mount | PR 10 E2E (solidtime) | #428 |
| Umbrel integration strategy decision (data/ + hooks/ + keys.env) | PR 10 E2E investigation | #429 |
| Test seam handling for `deadcode` (false-positive class) | PR 7 (#424) | ADR-0037 mentions |
| Remaining 29 dead funcs in `backend/core/` | PR 7 | baseline accepted |

## Bottom line

Sprint 21 closed the journald viewer scope that's been open since Sprint 14, eliminated the catalog hostname bug class permanently (lint + sweep + auto-transform), produced ADR-0037 (deadcode gate strategy correction), AND closed two additional install-reliability bug classes discovered mid-sprint (URL-embedded host placeholders + bind-mount perms). v0.6.16 ships these as cumulative install reliability improvements.

The lingering Laravel storage class + the broader Umbrel integration strategy question are explicitly deferred — they need ADR-level decisions, not patches.
