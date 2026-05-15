# Sprint 20 — Plan (code quality + catalog quality)

**Date drafted:** 2026-05-15 (immediately after v0.6.14 cut)
**Predecessor:** Sprint 19 (`sprint-19-dead-code-removal.md`)
**Target release:** v0.6.15 (patch bump — no breaking changes, no new features)
**Theme:** consolidation. Sprint 19 deleted dead code; Sprint 20 fixes the lingering quality issues that surfaced during that work + the dedicated CORS DRY follow-up, plus the 3 user-visible catalog bugs spotted in passing.

## Goal

Three catalog bugs squashed (#385, #386, #397) + CORS duplication eliminated (#391) + function-level deadcode swept now that the CI gate (warn-only) reports findings + CI flake (#373) investigated. **All as v0.6.15 patch release.** No feature work, no scope expansion.

Sprint 21 picks up the carry-overs (#259 /logs page, #260 restart buttons, #295 apps/+page split).

## Strategy

Five rules:

1. **TDD strict for bug fixes.** #385 / #386 / #397 each land with a failing-then-passing regression test per [[feedback-bug-regression-discipline]]. Even when the bug "self-resolves" (Adventure Log was running fine post-Sprint 19), the test locks the behaviour so the next return is caught at PR time.
2. **One bug per PR.** Catalog bugs touch the same domain but have different root causes (port_map, HTTP 400 validation, container_name label loss). Independent PRs = independent rollback.
3. **CORS DRY is a single PR, not a sweep.** All 9 services migrate to `common/middleware.Cors()` in one PR. Verify byte-identical output via a unit test BEFORE migration. Mid-sprint partial migration is forbidden — either all 9 ship together or none do.
4. **Deadcode sweep is opportunistic.** Run `./scripts/check-deadcode.sh` per service, fix what's obvious, leave what's not. No goal of zero findings — that's the v0.7.0 strict-mode flip's job.
5. **CI flake gets a time-box.** #373 testcontainer timeout investigation is 1 day max; if root cause isn't found by then, ship a band-aid (`-timeout=5m` instead of 2m) + open a follow-up. Don't let it eat the sprint.

## Execution order

Small wins first (catalog bugs are quick), refactor (CORS) in the middle, exploratory (deadcode + flake) last. Each PR independently mergeable.

### PR 1 — #386 Adventure Log regression test (~1h)

The Adventure Log install failed with HTTP 400 the day v0.6.13 cut. By the time we looked, it was running cleanly (3 containers up) and no audit trail showed the 400 anymore. The fix may have been transient (race condition? cleanup interaction?) or environmental.

**Action:** add a Playwright real-backend smoke test that installs Adventure Log against `.142` and asserts:
- POST `/v2/app_management/compose` returns 2xx
- `adventurelog-web-1` + `adventurelog-db-1` + `adventurelogserver1` containers appear in the installed list within 60s

If the test passes consistently, we have no bug — but we have coverage for next time. If it fails, we have a reproducer.

Per [[feedback-playwright-mocks-are-not-e2e]]: this is real-backend, not mocked. Uses the `realBackendTest` harness shipped Sprint 18 #344.

### PR 2 — #385 Activepieces tile click broken (~2h)

The Activepieces tile click reportedly does nothing or shows a broken state. Source audit suspected missing `port_map`.

**Action:** open Activepieces in the staging UI, observe the actual failure, trace it to compose. Likely fixes:
- (a) missing `x-casaos.port_map` in catalog entry → fix in `community-catalog/Apps/activepieces/docker-compose.yml`
- (b) UI tile-click handler crash → fix the handler

Either way, add a unit test for the failure mode (catalog manifest validator catches missing `port_map`, OR `Tile.svelte` doesn't crash on missing `port_map`).

### PR 3 — #397 2fauth `container_name` tolerant fallback (~3h)

Diagnosed during Sprint 19 staging validation. Root cause: explicit `container_name:` in compose drops the `com.docker.compose.project` label, so the post-up verification can't find the container.

**Action (option C from the issue body):**

1. Backend tolerant fallback in `backend/app-management/service/compose_app_query.go`: when the project-label filter returns empty, try a name-filter for `<project>` as a fallback. Test cases:
   - Project found via label → return immediately (preserve current behaviour)
   - Project not found via label, found via name → return (NEW)
   - Project not found by either → error (preserve "no container found" behaviour)
2. Catalog fix: `community-catalog/Apps/2FAuth/docker-compose.yml` — remove the explicit `container_name: 2fauth`. Users with existing installs get a new container ID on next upgrade; data persists in the bind-mounted volume.

Failing test first per TDD: write a test that simulates a container with no project label and asserts the fallback matches.

### PR 4 — Catalog `container_name` sweep (~2h)

Find every catalog compose file that uses `container_name:` and decide whether to keep or remove. **Action:**

```bash
grep -rln "container_name:" community-catalog/ | while read f; do
  echo "=== $f ===" ; grep "container_name:" "$f" ; echo
done
```

For each finding, decide:
- (a) remove if there's no good reason
- (b) document the reason if it's intentional (e.g. another app depends on the literal container name)

Output: a follow-up issue for any catalog entry that needs structural rework + a clean diff for the obvious removes.

### PR 5 — #391 CORS DRY (~3h)

Revive `backend/common/middleware/Cors()` (deleted in Sprint 19 PR 2/5 because nobody had adopted it) and migrate all 9 services to it. Verify byte-for-byte equivalence with a unit test BEFORE the migration commit.

**Files touched:**
- `backend/common/middleware/cors.go` (new) — single `Cors() echo.MiddlewareFunc`
- `backend/common/middleware/cors_test.go` — assert the returned config matches the current inline shape (AllowOrigins, AllowMethods, AllowHeaders, ExposeHeaders, MaxAge, AllowCredentials all byte-equal)
- 9 services migrated:
  - `app-management/route/v2.go`
  - `message-bus/route/routers.go`
  - `core/route/v{1,2}.go`
  - `local-storage/route/v{1,2}.go`
  - `user-service/route/v{1,2}.go`
  - `gateway/route/management_route.go`
- Inline `CORSWithConfig{...}` blocks (~14 LOC each) replaced with `middleware.Cors()`
- Net: −~110 LOC (14 × 8 removed, ~70 added in the helper + test)

**Risk:** CORS misconfiguration breaks the browser → API surface. Mitigation: byte-equal unit test gate + manual smoke (open the UI in browser, confirm requests pass) BEFORE merge.

### PR 6 — Deadcode sweep (~3h)

`./scripts/check-deadcode.sh` runs in warn-only mode in CI (Sprint 19 PR 5). Findings exist but aren't actionable from the warn output alone — this PR walks them service-by-service and fixes the obvious ones.

**Methodology:**
1. Run locally: `./scripts/check-deadcode.sh <service>` per service.
2. For each finding, decide:
   - **Delete** — if truly dead (no reflection, no codegen, no string-reference).
   - **Document with `//deadcode:keep`** — if reachable via reflection or build-tagged code that the tool doesn't analyse.
   - **Open issue** — if removal requires substantial refactor (>50 LOC change).

Goal: reduce findings to a number where strict-mode at v0.7.0 won't generate noise. NOT zero findings — that's overreach.

### PR 7 — Retroactive real-backend test coverage of core flows (~1.5 days)

Sprint 18 #344 shipped the `realBackendTest` Playwright harness; Sprint 18 #388 added one example (audit pane smoke). The pattern is in place — Sprint 20 expands it to cover the **5 golden-path user journeys** that today have no real-backend coverage:

| Journey | Current state | Sprint 20 target |
|---|---|---|
| **Login → session restore → Launchpad renders** | mocked smoke only | real-backend: real JWT, real installed-apps GET |
| **Install community-catalog app → container appears in launchpad** | install-flow.spec.ts uses `page.route()` mocks | real-backend: POST + SSE stream + finalizeDeploy + tile appears |
| **Uninstall app → tile disappears, container gone** | no coverage | real-backend: DELETE + post-delete state assertion |
| **Settings → Audit pane shows live records** | mocked smoke (audit-pane.smoke.spec.ts) | real-backend already shipped Sprint 18 — extend to assert ui_error records render when triggered |
| **Custom App YAML upload → container running** | mocked orchestrator.spec.ts | real-backend: full YAML POST + container check |

**Methodology** (per [[feedback-playwright-mocks-are-not-e2e]]):
- All specs use `realBackendTest` against `POWERLAB_E2E_BASE` (defaults skip if env unset; CI runs against a fresh testcontainer if we extend the harness, otherwise dev runs against `.142`)
- Each spec asserts the **full pipeline**: HTTP request shape, response code, downstream side effect (container exists, SSE event emitted, audit row written)
- Cleanup: every spec uninstalls what it installed; planted state caught by [[feedback-clean-up-planted-test-data]]

**Out of explicit scope for this PR:**
- Backend Go unit test coverage push (separate exercise — relies on `coverage.out` artifact analysis, not Playwright)
- Settings sub-panes (Network, Storage, Power) — Sprint 21 target
- File-browser flows — Sprint 21 target (#36 already tracks file scope sandbox)

**Why this matters now:** Sprint 19 deleted ~4,600 LOC of dead code with high confidence because we had grep-proof. We do NOT have the same confidence for the *live* code paths — most are exercised only via mocks. The mocks are necessary for CI speed but they miss the bug class that's bitten us repeatedly (v0.6.7 SSE buffering, v0.6.12 audit endpoints unreachable, v0.6.13 SSE Gzip). Real-backend coverage of the golden paths is the structural fix.

### PR 8 — Backend Go coverage gap audit (~half day, audit-only)

Analyse the `backend-coverage-*.json` artifacts uploaded by the existing `Backend ${service}` CI matrix (already shipping since Sprint 8). Produce a markdown audit in `docs/audits/backend-coverage-gaps-2026-05-XX.md` listing:

- Top 10 source files with <30% coverage by line count
- Per-service summary (which service has the worst coverage; which has the best)
- Risk-ranked: high-traffic + low-coverage = top of the list

**No code changes in this PR** — it's pure analysis. Output drives the Sprint 21 / 22 coverage push: each subsequent sprint takes 2-3 items from the audit and adds tests.

This is the "retroactive coverage of core funcionalities" intent compressed to one sprint: identify the highest-leverage gaps, do the Playwright-side expansion now, let the backend Go coverage push run incrementally over the following sprints.

### PR 9 — #373 CI integration flake (~1 day time-box)

The `Backend integration (app-management)` job timed out at 2m on the v0.6.12 release SHA. Likely cause: testcontainer cold-start on shared CI runner.

**Time-box:** 1 day investigation. If root cause isn't found:
- Ship a band-aid (`-timeout=5m` in `go test -tags=integration`)
- Open a follow-up issue with the investigation notes
- Move on

If found:
- Fix it, add a regression test (testcontainer warm-up loop or stub).

## Out of scope

- **#259** /logs page (per-service journald viewer) — carry-over to Sprint 21
- **#260** per-service restart buttons (Power pane) — carry-over to Sprint 21
- **#295** `apps/+page.svelte` split — Sprint 7 carry-over; its own sprint
- **deadcode strict-mode flip** — v0.7.0 decision
- **HTTPS re-enable / trust-dance** — indefinitely deferred per [[project-sprint7-no-trust-dance]]

## Acceptance for the sprint

- [ ] PR 1 merged: Adventure Log regression test green (#386)
- [ ] PR 2 merged: Activepieces tile clickable + regression test (#385)
- [ ] PR 3 merged: 2fauth tolerant fallback + catalog entry fix + regression test (#397)
- [ ] PR 4 merged: `container_name` sweep audit + targeted removes
- [ ] PR 5 merged: CORS helper revived + 9 services migrated + byte-equal test (#391)
- [ ] PR 6 merged: deadcode sweep — findings reduced significantly
- [ ] PR 7 merged: 5 real-backend Playwright specs for golden-path user journeys
- [ ] PR 8 merged: backend Go coverage gap audit doc (drives Sprint 21+)
- [ ] PR 9 merged: CI flake fix or band-aid + follow-up issue (#373)
- [ ] Sprint 20 retrospective doc in `docs/audits/sprint-20-retrospective.md`
- [ ] `release-manifest.yaml` summary updated for v0.6.15 (≤ 250 chars — memory [[feedback-run-all-check-scripts-before-release-push]])
- [ ] User explicit cut authorization (memory [[feedback-require-explicit-release-auth]])

## Risk surface

- **CORS DRY (PR 5)** is the highest-risk PR — touches 9 production files at the request entry point. The byte-equal test gate must be PR-blocking. Manual browser smoke before merge.
- **Catalog fixes (PR 1-3)** all have user-visible install paths. Each carries a regression test; the test must be reproducible against `.142` before considering the bug "fixed".
- **Catalog sweep (PR 4)** could surface unexpected cross-app dependencies (e.g. `db` container name expected by 3 other apps). Investigate before removing.
- **Real-backend Playwright (PR 7)** depends on `.142` (or a CI testcontainer setup we don't have yet) — the specs default-skip when `POWERLAB_E2E_BASE` is unset, so they DON'T block PR CI; they're a manual gate before tag. Worth wiring a CI matrix job in a future sprint, but explicit non-goal here to avoid scope creep.

## Why patch bump for v0.6.15

Per semver + [[feedback-no-v1-without-alignment]]:
- 0 breaking changes (CORS migration preserves the wire shape byte-equal)
- 0 new user-facing features
- 3 bug fixes + 2 internal refactors + 1 CI fix
- → patch bump

`feedback_no_post_v1_planning` still applies — no v1.0 ETA discussion in this plan.

## Memory invariants reinforced

- [[feedback-tdd-strict]]: every bug fix lands with failing-test-first.
- [[feedback-bug-regression-discipline]]: Adventure Log regression test even though bug self-resolved.
- [[feedback-no-apagar-test-para-passar]]: CORS migration uses byte-equal assertion, not a relaxed equivalence.
- [[feedback-playwright-mocks-are-not-e2e]]: PR 1 uses `realBackendTest` against `.142`, not `page.route()` mocks.
- [[feedback-run-all-check-scripts-before-release-push]]: pre-v0.6.15 cut runs every `check-*.sh`.
- [[feedback-require-explicit-release-auth]]: cut needs your explicit "cut v0.6.15".
