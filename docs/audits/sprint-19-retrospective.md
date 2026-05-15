# Sprint 19 — Retrospective

**Date:** 2026-05-15
**Plan doc:** [`sprint-19-dead-code-removal.md`](sprint-19-dead-code-removal.md)
**Trigger:** independent third-party audit (commit `b5ffa31`) found 22 entire dead Go packages.
**Outcome:** 7 PRs merged. **−4,632 LOC net delta**, **12 deps dropped**, **1 CI gate added**, **1 ADR captured**, **1 architecture statement added**.

## Headline

The sprint removed code that had been dead through 15 prior sprints, closing a process gap that Sprint 1 introduced (🟡 "verify before deletion" with no CI counterpart). PR 5 (`Backend deadcode` CI matrix in warn-only mode) prevents the next miss from recurring.

Nothing was rushed. Each kill PR captured a grep-proof in its body BEFORE the delete commit, race-detector and cross-compile checks ran per service, and the 2 packages that carried lessons (`utils/version`, `middleware/echo.go`) had those lessons extracted into ADR-0036 and the architecture README before deletion.

## Delivered

| PR | Title | LOC delta |
|---|---|---:|
| #389 | Sprint 19 dead-code removal plan | docs |
| #392 | ADR-0036 + plugin-less architecture statement (PR 2 prep) | docs |
| #390 | UI dead artefacts | −165 |
| #393 | `backend/common/` dead packages | −2,166 + 8 deps |
| #394 | `backend/core/` dead packages + deps | −1,431 + 4 direct deps |
| #395 | `backend/local-storage/pkg/` dead packages | −860 |
| #396 | `deadcode` CI gate + Sprint 1 audit closure | +200 (gate) |
| **Total** | | **−4,632 net, 12 deps dropped** |

## Issues closed during housekeeping (post-merge)

| Issue | Status |
|---|---|
| #332 Custom App `[object Object]` form field | shipped Sprint 18 |
| #345 Unify Community Install + Custom App modals | shipped Sprint 14 |
| #357 Sprint 16 audit + SQLite + retention pane | superseded by ADR-0035 (JSONL) |
| #358 Frontend JS error capture sink | shipped Sprint 18 #388 |
| #364 bbolt spike for audit storage | superseded by ADR-0035 |
| #374 Custom App YAML-first | shipped Sprint 18 #383+#387 |

## What worked

- **Knowledge extraction before delete.** PR 2 prep (#392) lifted two ideas out of `common/` packages before they died: ADR-0036 (file-system migrations marker-file pattern from `utils/version`) and the plugin-less architecture statement (lifted from `mod_management`). Neither would have been recoverable from git history without prompting; both are now first-class docs. Lesson: any time we delete a sizeable subsystem, force the prep pass.
- **Grep-then-delete contract.** Every PR body captures the grep output proving 0 importers. Reviewers don't need to re-run it; the proof travels with the diff. The audit's caveat about not being able to run `deadcode` directly was handled correctly — import-graph grep is reliable for whole-package liveness.
- **Audit correction during PR 4.** Spot-check on `local-storage/pkg/utils/` found that `merge/` and `command/` SUBPACKAGES were alive (2 + 4 importers respectively), even though the top-level package was dead. The source audit's claim "all of `pkg/utils/` was dead (312 LOC)" was imprecise. Caught during the PR's grep proof, narrowed the delete to the 5 top-level files, kept the directory for the live subpackages. Documented in PR 4 changelog fragment.
- **`go mod tidy` over-delivered.** PR 3 was expected to drop `redigo` + `go-github`. Actual drops: `go-github` direct + `json-iterator/go` + `pkg/errors` + `golang.org/x/oauth2` + transitives `golang/protobuf`, `go-querystring`, `appengine`, `modern-go/concurrent`, `modern-go/reflect2`. Worth re-running `go mod tidy` after every deletion sprint.
- **CI gate as the structural fix.** PR 5 closes the loop that Sprint 1 opened. Going forward, even a single dead function in a live package will surface as a warning on every PR. Strict mode flips at v0.7.0 cut time (memory `feedback_no_v1_without_alignment`).

## What surprised me

- **byte-identical CORS in 9 services.** Knowledge-extraction pass on `common/middleware` found that all 9 services define the SAME CORS config inline. A `Cors()` helper existed (in the dead `middleware` package) but nobody adopted it. Deciding whether this is "duplication that doesn't matter" or "real tech debt" is a separate exercise — tracked as #391 for a follow-up PR rather than inflating Sprint 19 scope.
- **`internal/op`'s dead-island geometry.** The Sprint 1 audit flagged `op` as 🟡 but didn't note that `op` was the ONLY consumer of `internal/{driver,conf}`. Once `op` proved dead, the other two became dead too. A whole isolated graph of ~722 LOC. The audit could have stated this — it would have made the deletion a single-decision call instead of three.

## Process gap diagnosed (Sprint 1 → 16)

Sprint 1's `docs/audits/dead-code.md` flagged 5 packages 🟡 with the instruction "each kill PR should grep before deleting." Three were resolved correctly (kept, alive). Two (`generic_sync`, `internal/op`) were genuinely dead and the grep step was **never carried out** through 15 sprints. The verify step had no CI counterpart, so reviewer fatigue on big kill PRs let them slip every time.

PR 5 fixes this structurally with the `Backend deadcode` CI gate. The annotated disposition table in `dead-code.md` documents the diagnosis so future readers find the gap, not the dead code.

## Friction

- **macOS cross-compilation for `local-storage`.** PR 4's verification required `GOOS=linux GOARCH=amd64` cross-compile because the native build hits `syscall.Listxattr` / `syscall.SockaddrNetlink` undefined errors on Darwin. Worked around with explicit cross-compile in the verification log, but it's a recurring pain point for anyone touching `local-storage` from a Mac dev box. No action item — the platform-conditional code is inherently Linux-only.
- **`redigo` clinging to indirect status.** PR 3 dropped `redigo` from the direct require block, but it remained as `// indirect` even after `go mod tidy`. `go mod why redigo` says "main module does not need package github.com/gomodule/redigo" yet tidy keeps the line. Some transitive in the Echo/socketio chain still references it. Harmless but noisy in the go.mod diff. Out of scope for this sprint; flag for a future dep-clean pass.

## Numbers

- 7 PRs merged in one session
- 59 files changed (530 insertions, 5,162 deletions)
- 12 dependencies dropped (~6 direct + 6 indirect)
- 0 test failures across the sprint
- 0 production behaviour changes (every deletion was unreachable code)
- 5 issues closed during housekeeping
- 2 new docs (`sprint-19-dead-code-removal.md`, `ADR-0036`)
- 1 architecture README update (plugin-less statement)
- 1 audit doc updated (`dead-code.md` 🟡 disposition table)
- 1 follow-up issue opened (#391 — CORS DRY tech debt)

## Memory invariants reinforced

- `feedback_tdd_strict` adapted for deletions: the existing test suite stays green, no test deletions (except of-the-deleted-package), `go build ./...` succeeds per service.
- `feedback_no_apagar_test_para_passar`: every deletion verified via race-detector + cross-compile; nothing was relaxed to pass.
- `feedback_bug_regression_discipline`: dead-code PRs carry grep-proof BEFORE the delete commit, same discipline as a bug fix.
- `feedback_run_all_check_scripts_before_release_push`: `check-deadcode.sh` joins the list for the next release.

## Out of scope (carried forward)

- **#391** — CORS DRY across 9 services. Real duplication; deliberately deferred to keep Sprint 19 scope tight.
- **#295** — `apps/+page.svelte` split (1,561 LOC god component). Structural debt, not dead code.
- **`deadcode` strict mode flip** — pending v0.7.0 cut authorization per memory `feedback_no_v1_without_alignment`.
- **Function-level dead code inside live packages.** PR 5 wires the tool; Sprint 20 can sweep findings.

## Release implications

The 4 deletion PRs (#390, #393, #394, #395) are user-invisible (dead code by definition); the gate PR (#396) is operator-visible only via CI logs. **v0.6.14 cut becomes a pure code-hygiene release** when the user authorizes — no UX changes, no migration steps, just leaner binaries + a CI guardrail.

If a future user finds a missing feature they thought existed, the grep-proof in each PR body documents what was removed and why. No archaeology required.
