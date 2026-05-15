# Sprint 19 — Dead Code Removal Plan

**Generated:** 2026-05-15 (post v0.6.13)
**Base commit:** `0e0e6ff` (main)
**Predecessor audit:** [`dead-code.md`](dead-code.md) (Sprint 1, 2026-05-08)
**Trigger:** independent third-party audit found 22 entire Go packages with **zero importers**, totalling ~4,748 prod LOC + ~216 test LOC. Cross-validated by this team at 2026-05-15 — every claim still applies 31 commits past the snapshot.

Sprint 1 originally flagged 5 packages as 🟡 "verify before deletion"; 3 turned out to be alive, 2 (`core/pkg/generic_sync`, `core/internal/op`) were genuinely dead and **never followed up** through 15 sprints. This sprint closes that gap and adds a CI gate so the next miss is caught automatically.

## Goal

Delete ~4.9k LOC of provably dead Go + ~165 LOC of dead frontend across **5 isolated PRs**, in increasing scope order. Zero behaviour change — the only risk is build/test breakage if a hidden importer exists, which the kill workflow catches at PR time.

Add a `deadcode` CI gate so this stops being a recurring problem.

## Strategy

Five rules, learned from the Sprint 8 kill-list waves:

1. **One package family per PR.** A frontend PR + four backend service PRs. Each PR self-contains the LOC delta, the `go mod tidy` diff, and the verification log.
2. **TDD-adapted for deletions.** There is no "failing test first" — we're removing code. Instead the invariant is: **the existing test suite stays green per package, no test deletions allowed (except tests OF the deleted package), and `go build ./...` succeeds on every service**. Any test breakage = abort + investigate, never relax the assertion (memory [[feedback-no-apagar-test-para-passar]]).
3. **grep-then-delete contract.** Each PR description includes the grep output proving zero importers, BEFORE the delete commit. Sprint 1 had the right instinct here; the failure mode was that the proof was never captured in a PR so the 🟡 items just slipped.
4. **Same-PR `go mod tidy`.** Dropping a transitive dependency happens in the same PR as the package removal, so `go.mod` + `go.sum` diff travels with the deletion.
5. **No cosmetic sweep.** Renaming CasaOS-flavoured identifiers (`GetCasaOSPort`, etc.) is deferred — pure churn, no LOC win, complicates the diff. Listed in Section 7 of the source audit and explicitly out of scope here.

## Execution order

Smallest blast radius first, biggest dependency churn last. Each PR is independently mergeable.

### PR 1 — Frontend dead pieces (~165 LOC, 5 min)

Smallest possible warmup. Frontend has only three dead artefacts after Sprint 14 audit consolidation:

| Path | LOC | Action |
|---|---:|---|
| `ui/src/lib/components/terminal/SshModal.svelte` | 110 | delete |
| `ui/src/lib/stores/theme.svelte.ts` + `theme.test.ts` | ~50 | delete (no toggle planned; app is hard-coded dark) |
| `ui/src/lib/api/index.ts` | 3 | delete (barrel with 0 importers — everyone uses `$lib/api/client` direct) |

**Verification:** `npm run test:unit` + `npm run test:e2e` + `npx svelte-check` — all must remain green.

**Risk:** zero. None of these is reachable from a route or another component.

### PR 2 — `backend/common/` dead packages (~1,950 LOC prod + ~216 test, 30 min)

Highest LOC payoff per PR. All 7 packages have zero external importers; `pkg/mod_management` only imports `codegen/mod_management`, so the two get deleted atomically.

| Package | LOC prod | LOC test |
|---|---:|---:|
| `common/codegen/mod_management` | 1,335 | 0 |
| `common/utils/ssh` | 402 | 0 |
| `common/utils/version` | 333 | 173 |
| `common/pkg/mod_management` | 158 | 43 |
| `common/middleware` | 23 | 0 |
| `common/utils/idevice` | 20 | 0 |
| `common/model/notify` | 12 | 0 |

**Note on `common/utils/version`:** the existing `casaos-residue-2026-05-10.md` audit lists this in "keep forever" as a CasaOS co-resident detector. **In practice it has 0 importers** — the `cmd/migration-tool/` that used it was deleted in Sprint 8. With the HTTPS trust-dance + v1.0 indefinitely deferred ([[project-sprint7-no-trust-dance]], [[project-v1-deferred]]), the "future use" rationale is even weaker today. **Decision:** delete with explicit cross-reference to the old audit's stale claim, so the next reader doesn't put it back.

**Verification:** `cd backend/common && go test -race -count=1 ./...`. Then for each service that imports `common`: `go build ./...` to confirm no transitive break.

**Risk:** zero — confirmed at 2026-05-15 via import graph.

### PR 3 — `backend/core/` dead packages (~1,431 LOC + 2 deps removed, 20 min)

The `core/internal/` island (`op` + `driver` + `conf` + `sign`) is self-contained — `op` was the only external user of `driver` and `conf`, and `op` itself has zero importers. Delete the four together.

`core/pkg/{generic_sync,singleflight,gredis,github,fs}` + `core/model/{system_app,system_model}` round out the same PR. Two dependencies drop:

| Dependency | Sole user | Action |
|---|---|---|
| `github.com/gomodule/redigo v1.8.9` | `core/pkg/gredis` | remove + `go mod tidy` |
| `github.com/google/go-github/v36 v36.0.0` | `core/pkg/github` | remove + `go mod tidy` |

**Verification:** `cd backend/core && go test -race -count=1 ./...` + `go build ./...` + `go mod tidy && git diff go.mod go.sum`. Verify the diff makes sense (transitive `golang.org/x/oauth2` should also drop).

**Risk:** zero.

### PR 4 — `backend/local-storage/pkg/` dead packages (~1,022 LOC, 15 min)

| Package | LOC | Note |
|---|---:|---|
| `local-storage/pkg/generic_sync` | 412 | byte-identical copy of `core`'s — deleted in PR 3 |
| `local-storage/pkg/singleflight` | 212 | byte-identical copy — deleted in PR 3 |
| `local-storage/pkg/utils` | 312 | misc helpers (`IsBool`) |
| `local-storage/pkg/sign` | 86 | signing |
| `local-storage/pkg/utils/encryption` | 12 | duplicates `user-service/encryption` — the live one |

**Verification:** `cd backend/local-storage && go test -race -count=1 ./...` + `go test -tags=fuse -count=1 ./...` (the fuse build tag must still compile). Then `go mod tidy`.

**Risk:** zero. The fuse-tag verification is the only nuance — `local-storage` is the one service that has platform-conditional compilation.

### PR 5 — `deadcode` CI gate + Sprint 1 follow-up closure (~80 LOC added, 1 h)

Sprint 1's audit (`docs/audits/dead-code.md`) ran `golang.org/x/tools/cmd/deadcode` and reported 368 function-level dead items, but **the tool never became a CI gate**. The 🟡 items that slipped for 15 sprints prove the gap is structural, not procedural.

This PR adds:

1. `scripts/check-deadcode.sh` — runs `go run golang.org/x/tools/cmd/deadcode@latest ./...` per service. Exits 1 if dead code is detected; exits 0 with `OK` otherwise.
2. `.github/workflows/ci.yml` job `Backend deadcode` — runs the script per service in matrix. Allowed to fail soft for one release window (warn-only), then hard-fail starting v0.7.0.
3. `docs/audits/dead-code.md` update — annotates the 5 Sprint 1 🟡 items with their final disposition (3 alive ✓, 2 deleted in PR 2/3). Closes that thread.

This is the only PR that adds rather than removes — it locks the invariant the next 15 sprints will rely on.

**Verification:** the script must produce `OK` against the post-PR-4 codebase; the matrix job must execute on a probe PR.

**Risk:** low. The hard-fail-after-v0.7.0 phasing keeps the existing PR pipeline unblocked while developers see warnings.

## Out of scope

Captured here so reviewers don't ask:

- **Cosmetic CasaOS renames** (`GetCasaOSPort`, `type CasaOSHeart`, `casaService.GetCasaosVersion()`). Source audit's Section 8. Pure churn, deferred indefinitely.
- **`apps/+page.svelte` split (#295, 1,561 LOC god component)**. Structural debt, not dead code. Stays on the backlog as its own sprint.
- **Function-level dead code in live packages.** PR 5 wires the `deadcode` tool — Sprint 20 can run a sweep of its findings once the gate is in place.
- **`scripts/setup-dev.sh` + `scripts/cleanup-dev.sh`**. No workflow / doc / code references, but these are commonly invoked manually by operators. Defer until we can confirm with the user whether they're in their personal `~/.zshrc`-style flow.

## Acceptance for the sprint

Sprint 19 is done when:

- [ ] PR 1 merged: frontend dead pieces removed, vitest + Playwright green.
- [ ] PR 2 merged: `backend/common/` dead packages removed, race-detector green.
- [ ] PR 3 merged: `backend/core/` dead packages removed, two deps dropped, `go mod tidy` clean.
- [ ] PR 4 merged: `backend/local-storage/pkg/` dead packages removed, fuse build tag still compiles.
- [ ] PR 5 merged: `deadcode` CI gate active, Sprint 1 audit annotated.
- [ ] Sprint 19 retrospective doc with final LOC delta, captured in `docs/audits/sprint-19-retrospective.md`.
- [ ] `release-manifest.yaml` summary updated for v0.6.14 (or v0.7.0 if the dep drops + CI gate justify a minor bump — to be decided at cut time per [[feedback-no-v1-without-alignment]]).

## How to reproduce the audit

```bash
# Confirm a package has zero external importers (run from repo root):
IMP="github.com/neochaotic/powerlab/backend/core/pkg/gredis"
grep -rl "\"$IMP\"" backend --include="*.go" | grep -v "/$(basename $IMP)/"
# Empty output → safe to delete.

# After deleting:
cd backend/<service> && go test -race -count=1 ./... && go build ./... && go mod tidy
git diff go.mod go.sum   # diff sanity-check the dep drops
```

## Memory invariants this sprint reinforces

- [[feedback-no-apagar-test-para-passar]]: any test breaking during a delete is a signal the package wasn't actually dead — investigate, never weaken.
- [[feedback-bug-regression-discipline]]: dead-code deletion is not a "bug" but the same discipline applies — capture the grep proof in the PR body.
- [[feedback-run-all-check-scripts-before-release-push]]: PR 5 adds one more `check-*.sh` to that list.
