# Sprint 10 retrospective

**Date:** 2026-05-11
**Sprint:** 10 (Visible polish + v0.6 cut prep)
**Status:** complete — **v0.5.13 published**

Per ADR-0019, retrospectives live in `docs/audits/`. This one covers
Sprint 10: the settings split that's been carried-forward for 3
sprints, a partial start on the apps split, two quick-win
follow-ups from earlier sprints' open promises, and the v0.5.13
release cut.

## Headline

Sprint 10 was the **settings split sprint**. The 1469 LOC
`settings/+page.svelte` god file, which has been on the
carry-forward list since Sprint 7 retro, finally landed as
**5 pane components + a 739 LOC orchestrator** — a 50 % LOC
reduction that unlocks per-pane component testing for Sprint 11.

The apps split kicked off with **3 small modals extracted** as
the pattern PR (#290) — the larger Install/Detail modals
deferred to Sprint 11 (#295) with explicit user-smoke gating
per the Sprint 7 retro rule.

Plus: 2 quick-win follow-ups (package-linux.sh AllowedOrigins,
2 MUST-FIX skipped Go tests), housekeeping (Issue #185 closed,
4 external user issues triaged, memory `feedback_rebrand_before_https`
de-staled), and the **v0.5.13 release cut + published**.

## What we shipped

### Settings split — done (PRs A–D + #285 from Sprint 9)

| PR | Pane | LOC |
|---|---|---:|
| #285 (Sprint 9) | AppsPane | 69 |
| #286 | GeneralPane | 159 |
| #287 (closed-by-arrasto into #288) | NetworkPane | 95 |
| #288 | SecurityPane | 277 |
| #289 | AboutPane | 280 |
| **Orchestrator** | settings/+page.svelte | **739** |

**Total**: `settings/+page.svelte` from **1469 → 739 LOC** (−730, −50 %).

The orchestrator now holds only: imports, sectional state
(`activeSection`, `copiedKey`), port-change confirmation
modal + countdown, hash-deeplink handler, certificate
handlers (`downloadCA`, `openHttpDownload`, `resetTrust`,
`confirmRotateCA`, `testHttpsConnection`), copy-to-clipboard
helper, and the sidebar/main layout. **All UI markup is in
panes.** Each pane is jsdom-friendly + props-driven so Sprint
11 can add per-pane component tests for the coverage lift.

### Apps split — kickoff (PR E)

| PR | Modal | LOC |
|---|---|---:|
| #290 | ForkAppModal | 32 |
| #290 | UninstallAppModal | 38 |
| #290 | UpdateAppModal | 67 |

**Net**: `apps/+page.svelte` from **1561 → 1492 LOC** (−69).
Smaller win than settings, by design — the 3 modals were the
isolated ones; the heavier surfaces (Detail, Install
confirmation with port-conflict UI, Install fullscreen,
minimized banner) need a state-machine-aware extraction +
user smoke. Carried to Sprint 11 as issue **#295**.

### Quick-win follow-ups (PRs F + G)

| PR | Item |
|---|---|
| #291 | `package-linux.sh` mirrors `[security] AllowedOrigins=` section that PR #241 added to the embedded sysroot conf.sample. Sprint 8 retro carry-forward; fresh installs now ship with the section visible to operators. |
| #292 | 2 Go `t.Skip("MUST FIX!")` tests resolved: `core/service/file_test.go::TestNewInteruptReader` was a 10-second sleep loop reading from upstream CasaOS dev's machine path with no assertions — rewritten as 6 proper unit tests for `NewReader`/`NewWriter` context cancellation. `pkg/utils/network_detection_test.go` was dead code (no production callers); deleted entirely + dropped the `github.com/Curtis-Milo/nat-type-identifier-go` dep. |

### Release (#293 + tag)

**v0.5.13 published** at <https://github.com/neochaotic/powerlab/releases/tag/v0.5.13>.

- `manifest.json` + amd64/arm64 tarballs (with `latest-` aliases)
- Summary in `release-manifest.yaml` rewritten to cover the
  Sprint 9 + 10 wave (was carrying Sprint 6 + 7 text)
- `scripts/check-manifest-fresh.sh` passed (no duplicate summary)
- 34 changie fragments aggregated into the v0.5.13 section of
  `CHANGELOG.md`

### Housekeeping (this turn)

- Issue **#185** (Sprint 5 stability sweep umbrella) closed
  with link to the closed retros that shipped its workstreams
- Issue **#122** (frontend test DX) closed-as-partial — coverage-v8
  shipped (#281); jest-dom + msw deferred until real DX pain
- Issue **#123** updated with the settings split completion;
  apps split moved to **#295**
- Memory `feedback_rebrand_before_https.md` de-staled —
  rebrand is now considered done; HTTPS re-enable is
  *indefinitely deferred*, not *gated on rebrand*. Points
  at memory `project_sprint7_no_trust_dance` for current state
- 4 external user issues triaged: **#114**, **#128**, **#205**
  closed with helpful redirects; **#119** open pending logs
  from reporter
- 3 new issues opened for Sprint 11 visibility:
  - **#295** apps split completion (Detail + Install modals
    + minimized banner)
  - **#296** coverage lift 16.77 → ≥25 %
  - **#297** vitest threshold gate (after #296)

## What went well

1. **Stacked-branch worktree pattern matured.** Sprint 10
   shipped 5 stacked PRs (#286 → #287 → #288 → #289 → #290).
   Each rebased on the previous; vitest 239/239 every step;
   the squash-merge cascade worked (the `#287` content
   was already in main via the squash-merge of `#288` —
   GitHub couldn't auto-detect this so I closed `#287`
   manually with an "into #288 squash" note). Lesson:
   stacked PRs are workable but the auto-detection of
   "this branch's content is already on main" is weak
   — explicit close-as-merged saves cycle time.

2. **`feedback_yolo_means_decide` exercised correctly.**
   When the carry-forward sweep flagged "UI splits #123
   need user-as-verification-gate," I made the call to
   ship a small pattern PR (`AppsPane`, 69 LOC) rather
   than push 1.4 k LOC unsupervised. Sprint 10 then
   completed all 5 settings panes step by step with
   vitest green at each step. Risk-managed velocity.

3. **Memory `feedback_no_apagar_test_para_passar` survived
   the dead-code temptation.** Both MUST-FIX skips
   COULD have been deleted-as-broken to "fix" the issue.
   PR #292's response respected the rule: rewrite the
   one with real intent (Reader/Writer context cancel)
   into 6 proper tests; delete the one with no production
   callers as dead code (which makes the test moot, not
   weakened).

4. **In-flight quick-win discovery worked.** The "Sprint
   6-10 sweep" agent run mid-sprint surfaced concrete
   short items (package-linux.sh AllowedOrigins,
   MUST-FIX skips) that closed during the same sprint
   instead of carrying forward another turn. Pattern
   worth keeping: every sprint runs a "what's still
   carrying that shouldn't be" sweep before retro.

5. **Documentation-as-issues policy.** The user-direction
   *"documentar e abrir issue para oq nao esta aberto"*
   turned implicit backlog into explicit, linkable
   tracking (#295 #296 #297). Each new issue has effort,
   gating, definition of done, and recommended order
   — Sprint 11 starts cold-readable instead of needing
   me to remember context.

## What we got wrong

1. **`backend/cli/codegen/**` accidentally swept into the
   release commit.** PR K (Sprint 8) deleted the entire
   `backend/cli/` subproject (#269), but a local `go generate`
   somewhere along the line had repopulated `codegen/`
   files. When I ran `git add -A` for the v0.5.13 changie
   batch, those 11k LOC came along for the ride. Caught
   by reading `git log -1 --stat` BEFORE pushing. Reset
   the commit, `rm -rf backend/cli`, re-committed. Lesson:
   `git add -A` is unsafe after `go generate`. Future
   release commits should use explicit `git add .changes/
   CHANGELOG.md release-manifest.yaml` and stop. Or add a
   `.gitignore` rule for `backend/cli/` since it's dead.

2. **The PR #287 squash-merge confusion.** Stacked PR
   chain hit the "content already in main" detection gap.
   I rebased #287 first (manual NetworkPane), but when
   merging #288 (which had #287 + #288 commits squashed
   together), #287's content shipped via #288's squash.
   GitHub didn't auto-close #287 — I had to close it
   manually with a "by-arrasto" explanation. Reviewers
   reading the PR list would see this as confusing. For
   Sprint 11's apps split, plan to land each PR
   sequentially (wait for #295 PR 1 to merge fully
   before opening PR 2) instead of pre-stacking.

3. **`#287` rebase produced "skipped previously applied
   commit" warnings** that I interpreted as cosmetic but
   actually indicated the branch had drifted. The cleaner
   path on a stacked-rebase: use `git rebase --onto`
   with explicit base instead of letting rebase guess.
   Documented for next time.

## Process / scope notes

- **External user triage**: 4 issues from `insxa` user
  were noise in the open backlog for the entire Sprint
  6-10 window. Triaged in 5 minutes when surfaced. Lesson:
  add "untriaged external" to the weekly sweep, not
  just sprint retros.

- **The Sprint 6-10 sweep agent flagged 6 real
  pendencies**; Sprint 10 closed 3 of them in-sprint
  (package-linux.sh, MUST-FIX skips, Issue #185
  cleanup). The other 3 carry forward as Sprint 11
  issues (#150 backend integration coverage was already
  an issue; #295 + #296 + #297 are new). Net: agent
  ROI was high — those items would have slipped
  another 1-2 sprints without it.

- **Bake observation begins now**. v0.5.13 cut at
  18:27 UTC 2026-05-11. v0.6 cut earliest 2026-05-18
  if zero prod regressions reported on issues 
  closed in v0.5.13's wave (#243-#248, #250-#252,
  #255, #278). Watch for in-UI updater bug reports
  + GitHub issues with `bug` label.

## Sprint 10 metrics

- **PRs:** 8 (#286–#293 + an admin close on #287)
- **LOC delta:** ~−800 net (Settings/apps splits redistribute;
  `network_detection.go` deletion + go-smb2 dep drop in
  Sprint 9 still echoing; new pane components add markup)
- **Issues closed:** 6 (#185 closed, #122 partial-closed,
  3 externals — #114, #128, #205 — #244 closed earlier
  in week)
- **Issues opened:** 3 (#295, #296, #297) + 1 tracking
  (#294 HTTPS restoration path)
- **ADRs:** 0 new
- **Memories updated:** 1 (`feedback_rebrand_before_https`)
- **Release:** v0.5.13 published (binaries + manifest.json
  + GitHub Release)
- **Tests added:** 6 vitest cases (file_test rewrite)
- **Coverage delta:** unchanged at 16.77 % statements
  (no new vitest tests on the extracted panes yet;
  that's Sprint 11 #296)

## Carry-forward to Sprint 11

Sprint 11 has a **clean readable backlog** as 3 explicit issues:

1. **#295** — apps/+page.svelte split completion (4 modals:
   Detail, InstallProgress, InstallMinimizedBanner,
   InstallConfirm with port-conflict UI). User-smoke gated.
   ~4-6h of work + per-PR smoke time.

2. **#296** — Coverage lift 16.77 % → ≥ 25 %. Autonomous.
   Highest bang/buck: 3 trivial store tests (+2 %) plus
   component tests on the 5 extracted Settings panes
   (+4-6 %) plus the apps modals (+1-2 %). Should lock
   regression behaviors from Sprints 8-10 rather than
   chasing the percentage.

3. **#297** — Add `coverage.thresholds` to vitest.config.ts
   after #296 lands. Floor at *Sprint 11 number − 5 %* to
   leave refactor headroom.

4. **#150** — Backend integration coverage (open since
   Sprint 3, autonomous). Heavy service packages need
   per-module unit tests. Doesn't block v0.6 but closes
   5 sprints of carry.

5. **v0.6 cut decision** (final week of Sprint 11):
   Gated on bake clean + #295 shipped + #296 trend up.
   Earliest cut 2026-05-18; realistic 2026-05-25.

## Closing

Sprint 10 closed three carry-forwards that had been
slipping since Sprint 7 (settings split, package-linux.sh
AllowedOrigins, MUST-FIX skips) and cut v0.5.13 on
schedule. The settings split — the biggest single piece
of refactor debt in the UI — is done in 5 reviewable
PRs with vitest green at every step. Apps split partial
but with a clear PR map for Sprint 11.

The `feedback_yolo_means_decide` muscle held: small-PR
extractions over big-bang refactors, every PR shipped
with passing vitest, no test-weakening to chase coverage,
no autonomous decisions on UX-risk pieces.

PowerLab is now **3 PRs from finishing the UI split
debt** (#295), **8 % from the Sprint 11 coverage target**
(#296), and **7 days bake from v0.6 cut eligibility**.
The path from v0.5.13 to v0.6.0 is short and the gates
are well-defined.
