# Sprint 6 retrospective

**Date:** 2026-05-10
**Sprint:** 6 (Quality consolidation YOLO)
**Status:** complete

Per ADR-0019, retrospectives live in `docs/audits/`. This one covers
the Sprint 6 work: the 8-module godoc raise initiative (#196), the
"Obliterate CasaOS" Sprint 5 kill wave that bled into Sprint 6
(#203 audit + 12 follow-ups), the v0.5.10 + v0.5.11 releases, the
TODO/FIXME burndown, the flaky search-test kill, the 3-file quality
audit (#216), and the Sprint 7 refactor proposal queued for the
next sprint.

The point is the same as Sprint 4's: capture what bit us so the
next sprint doesn't repeat the class — not to assign blame.

## Headline

Sprint 6 closed the per-service godoc raise initiative (#196) —
**every backend service is now at ≥70% godoc coverage with a
curated landing page on the public docs site.** That's 8 modules
(pkg, gateway, user-service, message-bus, common, local-storage,
app-management, core), 6 PRs landed today (#221–#226), ~3000 LOC
of docs added across ~120 files.

In the same sprint window: shipped v0.5.10 + v0.5.11, completed
the CasaOS-residue obliterate wave (#206 #207 #208 #209 #210 #212
all from the #203 audit kill list), killed a long-standing flaky
test (#218 — TestSearch / dead Search method), burned 5 TODO/FIXME
markers down (#220), and queued a 174-line Sprint 7 refactor
proposal for the four >1000 LOC files (#227).

**Two production reports caught this sprint:** the v0.5.8 lock-out
hot-fix is fully shipped (its v0.5.9 + v0.5.10 follow-ups landed
clean), and the v0.3.2 "9 bug fixes from production reports" wave
that started Sprint 6 (commit `70bffc3`) is settled.

## What went well

1. **The per-service godoc raise pattern generalized cleanly.**
   First raise (gateway #213) needed deliberation — "what's a
   curated landing page look like?", "where in mkdocs nav does this
   go?", ".gitignore patterns for generated vs committed files."
   By PR #5 (common #223) the steps were mechanical: read package,
   add docs to high-leverage exports, update `gen-godoc.sh`
   MODULES, add mkdocs nav block, write index.md, write changie
   fragment, commit + PR. Largest module (core, 355 exports)
   still finished within the same window.

2. **Per-service raise scoping survived the biggest module.**
   Strategy on core (#226) was explicit: focus on package docs +
   Service interface contracts + model types, skip per-method docs
   on the 35-method `SystemService` gopsutil-wrapper interface
   ("names self-document"). Documented in the changie fragment
   under "Intentional non-coverage" so a future audit doesn't
   re-flag the same surface as missing docs. Same idea applied to
   the FUSE-shaped `pkg/mount/{file,dir}.go` in local-storage
   (#224) — bazil.org/fuse interface contract is upstream's docs
   problem, not ours.

3. **YOLO-decide-and-ship discipline held under load.** Six
   sequential godoc PRs in one day (#221 → #226), each one
   queued + merged + cleanup-worktree before the next started.
   No "should I split this?" pauses; no "want me to add X?"
   prompts. The few times CI was slow (PR #224 amd64 smoke ran
   ~18 min), I prepped the next worktree in parallel rather than
   blocking. Memory rule `feedback_yolo_means_decide.md` paid out.

4. **Bug fix = regression test held even on the flaky-test kill.**
   PR #218 (kill flaky `TestSearch`) didn't just delete the test —
   it deleted the dead `Search()` method + the `SearchEngine`
   model type that the test was the last consumer of, and added a
   commit message explaining why the deletion was safe (only call
   site was already commented out). Same memory rule
   `feedback_bug_regression_discipline.md` applied to the
   non-feature-fix case: when removing dead code that had a flaky
   test as its only validation, the kill itself becomes the
   regression-protection.

5. **Sprint 7 prep landed in Sprint 6.** While #226 was in CI,
   drafted the refactor proposal (#227) for the four >1000 LOC
   files. Plan-only — no code changes — but Sprint 7 starts with
   a runnable target list instead of having to re-read the
   audit cold. ~10 minute investment that compounds.

## What went wrong

1. **`ui/ ` (with trailing space) directory accident on main.**
   At some point during the worktree juggling I created a
   directory literally named `ui ` (trailing space) in the main
   worktree. Caught it during the #227 commit when `git status`
   showed `?? "ui/ "`. Empty, harmless, but should not have
   happened. Root cause unclear — likely a `cd "ui /"` typo from
   a copy-paste during one of the many worktree cleanups.
   **Process change:** avoid `cd` in prompts where the path
   matters; use absolute paths only.

2. **Stale wakeup callbacks from `/loop` mode kept asking me to
   merge already-merged PRs.** The dynamic-pacing loop fires the
   same prompt verbatim each cycle — when CI completed faster
   than expected, I'd merge + start the next PR in the same
   session, then the next wakeup would re-fire "merge PR #224"
   for a PR that was already merged. Defensive `gh pr view --json
   state` check at the top of each handler caught it, but the
   noise was real. **Process change:** when scheduling wakeups
   for PR-merge tasks, the prompt should re-derive the active
   PR number rather than hard-coding it. Or use Monitor on the
   GitHub status webhook instead of timed wakeups.

3. **gofmt fights twice.** Both #224 (local-storage) and #225
   (app-management) needed an unplanned `gofmt -w` pass mid-
   commit because the godoc additions tripped the alignment-
   sensitive struct-tag formatting. Pre-existing files (e.g.
   `service/disk.go`) were already gofmt-dirty so my own touches
   inherited the formatting drift. **Process change:** run
   `gofmt -w <file>` before the first edit on any file the audit
   flagged as long; treat the formatting fix as a separate first
   commit so the godoc-add diff stays clean.

4. **The `SystemService` 35-method interface raised a meta-
   question I answered by writing a justification.** When I hit
   the `SystemService` interface in core (#226), every method was
   a thin wrapper around gopsutil/os/exec calls that were
   identical in name to the wrapped function. Adding per-method
   docs would have been pure noise; skipping them risked a
   future audit re-flagging the same surface. **Resolution:**
   wrote an "intentional non-coverage" section in the changie
   fragment + the curated landing page so the decision is
   discoverable. **Lesson:** when an audit-driven raise hits a
   "nothing useful to add" surface, document the skip explicitly
   instead of hoping the reviewer notices.

## What surprised me

- **Coverage numbers in the audit were ROM (rough order of
  magnitude), not exact.** message-bus was reported at 17% in
  audit #216 but my hands-on count put it closer to 4.5%. The
  raise still pushed it over the 70% bar regardless, but the
  audit's numbers turned out to be an analyzer-driven undercount
  of undocumented exports. **Action:** the analyzer probably
  weights doc-comment LENGTH; very-short package docs ("Package
  X provides Y.") count as documented even when 60+ exports
  inside the package have nothing.

- **`backend/common` had the highest leverage of any raise.**
  Common is imported by every service; lifting its `external/*`
  SDK + `utils/jwt` + `pkg/security.CertManager` docs over 70%
  surfaces docs wherever those types appear in any module's
  generated reference. Should have been #2 not #5 in the order
  if measured purely by leverage.

- **The `ui/+page.svelte` files turned out to be huge.** The
  audit flagged `apps/+page.svelte` at 1561 LOC and
  `settings/+page.svelte` at 1469 — both bigger than any single
  Go file in the repo except `core/service/system.go` (1010).
  Sprint 7 refactor proposal #227 puts them on the punchlist;
  worth flagging in any future "where is the complexity?"
  audit.

## What we shipped

### Releases

- **v0.5.10** (PR #199) — first release of Sprint 6, picked up
  the v0.5.9 hot-fix tail. User verified: "teste ok".
- **v0.5.11** (PR #211) — catch-up for the obliterate-wave +
  rebrand work. User verified: "feedback a atulizacao para 5.9
  deu certo".
- **v0.5.12 — NOT YET CUT.** 14 changie fragments queued for
  it; awaits user verification of v0.5.10/v0.5.11 first per
  the ongoing release-test cadence.

### PRs (today's wave only — 30 PRs total since v0.5.9)

**Godoc raise initiative (#196 — INITIATIVE COMPLETE):**

| Module | Coverage Δ | PR |
|---|---|---|
| pkg/* | 100% (Sprint 2) | — |
| gateway | 35% → ~85% | #213 |
| user-service | 40% → ~75% | #211 + #221 |
| message-bus | ~5% → ~75% | #222 |
| common | ~49% → ~75% | #223 |
| local-storage | ~27% → ~75% | #224 |
| app-management | ~35% → ~75% | #225 |
| core | ~39% → ~75% | #226 |

**CasaOS obliterate wave (from audit #203):** #206 #207 #208 #209
#210 #212 (security: kill curl-pipe-bash self-update path; ADR-0022
documenting CasaOS abandonment; gateway sysroot rebrand; cosmetic
strings + dead systemd units; codegen local paths; wave 2 icon CDN
+ build-artifact sweep).

**Quality + housekeeping:** #215 (Sprint 5 dashboard), #216
(quality + tech debt audit), #217 (3 quick-win fixes from
audit), #218 (kill flaky TestSearch), #220 (TODO/FIXME burn-down
24 → 19), #227 (Sprint 7 refactor proposal — plan-only).

**Pre-Sprint 6 closing items:** #203 (CasaOS residue audit), #204
(vendor mermaid.js), #197 (godoc + Scalar surface), #200 (Sprint 4
self-review).

### Documentation deltas

- 8 new curated landing pages on the docs site (one per backend
  module's Go API reference)
- Sprint 7 refactor proposal added (`docs/audits/sprint-7-
  refactor-proposal.md`) — non-binding plan with 7 PRs
  enumerated for the four god files + four god functions
- ADR-0022 added (CasaOS abandoned)
- Sprint 5 progress dashboard added

## Process changes ratified this sprint

1. **Per-service godoc raise = canonical pattern.** Codified by
   the pkg → gateway → user-service → message-bus → common →
   local-storage → app-management → core sequence; documented
   inline in `scripts/gen-godoc.sh` MODULES + each PR's changie
   fragment.

2. **Worktree-per-PR hygiene.** Every PR this wave used a
   dedicated worktree (`/Users/neochaotic/Documents/dev/powerlab-
   <topic>`) so concurrent CI never blocked the next PR's prep.
   Cleanup-worktree-on-merge added to the merge prompt template.

3. **"Intentional non-coverage" sections in changie fragments
   for godoc raises.** Wherever a raise deliberately skipped a
   surface (codegen, FUSE handlers, gopsutil-wrapper methods),
   the changie fragment names the skip + reasoning. Future
   audits can read this and not re-flag the same surface.

4. **Plan-only doc PRs as Sprint-N+1 prep during Sprint-N CI
   waits.** Sprint 7 refactor proposal (#227) was authored
   entirely while #226 was in CI. Pattern is: when a sprint's
   last PR is in CI and there's no blocking work, draft a
   plan-only doc for the next sprint's biggest item.

## Open Sprint 6 backlog (intentional non-completion)

These were flagged in Sprint 6 scope but deliberately deferred:

- **Long-file refactor (audit #216 §D + §E)** — now planned in
  PR #227, awaits user authorization to start the splits.
  Estimated 7 PRs.
- **Backend integration coverage (#150)** — needs Docker; user
  has been running tests outside containerized environments.
- **Real upgrade test finish (#169)** — partial work landed; full
  flow needs a 2-step host (clean install of v0.4.x → upgrade
  path → verify state).
- **E2E test expansion (#108)** — "se sobrar tempo" per user;
  not a Sprint 6 must.

These all roll into Sprint 7 by default unless re-prioritized.

## Strategic state on close

- **HTTPS** — disabled by default since v0.5.2. Per memory
  `feedback_rebrand_before_https.md`: re-enable AFTER the trust
  dance #118 + integration tests gate. Defer to Sprint 7+.
- **v1.0** — no committed ETA. Per memory `project_v1_deferred.md`
  + `feedback_no_v1_without_alignment.md`: requires explicit
  user approval; HTTPS re-enable is a prerequisite.
- **Phase 3 mkdocs site** — structurally complete; every backend
  module has a Go API reference page. Phase 3 polish (search,
  tabs, theming) was opportunistically done across Sprint 4 + 5.
- **CasaOS dependency** — operationally zero. Audit #203 closed
  out the residue list. ADR-0022 documents upstream abandonment.

## What to do differently in Sprint 7

Carry-forward lessons:

- **Run `gofmt -w` on touched files BEFORE the first edit.**
  Formatting drift in pre-existing files inherits into doc-add
  diffs.
- **For PR-merge wakeup loops, re-derive the PR number from
  open-PRs list rather than hard-coding it in the prompt.**
  Stale prompts asking to merge already-merged PRs are pure
  noise.
- **When a refactor PR is plan-only, write the changie fragment
  AS the plan summary** so it shows up in the next release notes
  even if the actual splits don't ship that release.
- **Lead with the highest-leverage module on multi-module
  initiatives.** `common` should have been raise #2 not #5 — its
  docs surface in every other module's reference output.

That's a wrap on Sprint 6. Sprint 7 starts with a clean board, 14
changie fragments queued for v0.5.12, and a runnable refactor
target list ready in #227.
