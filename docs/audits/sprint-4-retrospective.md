# Sprint 4 retrospective

**Date:** 2026-05-10
**Sprint:** 4 (CasaOS coexistence polish — Docker labels + AppData isolation)
**Status:** complete

Per ADR-0019, retrospectives live in `docs/audits/`. This one covers
the Sprint 4 work (`#85` main goal — clean coexistence with CasaOS),
the parallel DB-paths split-brain audit (#179 / PR #180), the
Sprint 4.5 docs site Phase 3 work brought forward (#71), and the
v0.5.8 lock-out regression I shipped + had to hot-fix.

The point of writing this is the same as Sprint 3's: capture what
bit us so the next sprint doesn't repeat the class — not to assign
blame. Where a lesson turned into a process change, it's named.

## Headline

Sprint 4 shipped issue #85 (the "mountain" of the casaos-strip
roadmap) as three sub-PRs (#181 PR-A foundation + ADR-0021, #187
PR-B wire call sites, #183 PR-C compose rewrite + coexistence
relax), plus parallel work on the DB paths audit (#179 / PR #180),
the docs site Phase 3 brought forward (#71 / PR #188 + 4 polish
PRs), and the v0.5.8 → v0.5.9 hot-fix.

Three releases shipped: v0.5.7 (JWT keypair persistence), v0.5.8
(coexistence + DB hardening + docs foundation), v0.5.9 (lock-out
hot-fix + UI auto-reload).

**v0.5.8 shipped a real upgrade-path bug**: my own split-brain
detector blocked user-service from booting on hosts that still had
the v0.5.4 hot-fix sobra. Same external symptom as the v0.5.7 JWT
bug ("can't login after upgrade"), completely different root cause
introduced by my own fix. Caught manually by the user, hot-fixed
live via SSH, permanently fixed in v0.5.9 with auto-clean +
integration test. Same shape as Sprint 3's v0.5.4 incident — a
fix that wasn't tested against the realistic upgrade state.

## What went well

1. **#85 sub-PR breakdown held its scope.** PR-A landed the helpers
   + ADR + tests with no service code changes. PR-B wired the call
   sites. PR-C closed the user-visible work (compose rewrite +
   install.sh notice). Each PR was independently reviewable in
   under 10 minutes. The pattern from Sprint 3 prep ("split into
   smallest-first") generalised cleanly.

2. **Documentation Phase 3 brought forward, not deferred.** Memory
   rule "Airflow-level docs are part of v1.0 — do not let docs
   slide" was honoured: instead of bundling docs into a pre-v1.0
   sprint, the mkdocs-material site (#71 / PR #188) shipped during
   Sprint 4 and now collects every subsequent ADR + audit
   automatically. Companion polish PRs (#191 stub pages, #194 README
   sweep, #195 Mermaid actually-renders, #197 godoc + Scalar) all
   landed in the same window.

3. **Bug fix = regression test held even under pressure.** Every
   incident this sprint landed with a test:
   - #176 (JWT keypair) → 5 Go tests + the THE-#176 regression
     `TestLoadOrGenerate_StableAcrossCalls`
   - #179 (split-brain) → 8 Go cases + 18 bash assertions
   - #85 / ADR-0021 → 16 + 12 + 4 tests across 3 PRs
   - **v0.5.8 lock-out** → 7 new Go cases + 4 UI cases + 1 bash
     integration test that builds the binary + simulates the
     v0.5.4 mishap state. The integration test is the one that
     SHOULD have run before v0.5.8 shipped; it now blocks any
     future regression of the class.

4. **Live SSH debug + manual fix worked as a recovery channel.**
   The v0.5.8 lock-out was caught + fixed in production within ~5
   minutes from the user's report. SSH credentials in memory let
   the live diagnosis happen without back-and-forth; the fix was
   `mv legacy → .bak.<ts>` and `systemctl restart`. The permanent
   v0.5.9 ships the same logic at startup.

## What went wrong

1. **I shipped v0.5.8's strict refuse-to-start without auto-clean
   for unambiguous cases.** The PR description explicitly chose
   not to auto-clean ("operator-actionable instructions instead of
   risking destructive automatic actions"). For `user.db` /
   `local-storage.db` specifically, the legacy path is **provably
   never read by the service** — auto-move-aside is non-destructive
   AND fixes the lock-out without operator action. The right design
   was always: differentiate ambiguous (core's casaOS.db at multiple
   paths) from unambiguous (user.db legacy = always stale). I made
   the wrong tradeoff. v0.5.9 fixed it.

   Lesson: "operator can read the log" is only useful when the
   operator CAN actually see the log. For end-user-facing services
   where the failure surface is "login broken," log-only recovery
   is unusable.

2. **Same upgrade-path test gap as Sprint 3.** Sprint 3's
   retrospective named #169 (Phase 1.5 release-checklist as
   automated upgrade test) and put it on Sprint 5. I didn't run it
   manually before v0.5.8 either. Result: the same class of bug
   hit twice, and the test that would have caught it was
   already-promised-but-not-shipped. The integration test that
   ships in v0.5.9 (`scripts/test-upgrade-resolves-stale-legacy_test.sh`)
   is half of #169's scope; the rest (real v(N-1)→v(N) install +
   login-flow assertion) is still on Sprint 5.

   Lesson: **a promised test is not a test.** Issues like #169 need
   to be done OR explicitly removed from the gate, not held in
   limbo while the same class of bug ships.

3. **Background agents stomped on the working tree mid-PR.** I
   dispatched a docs-polish agent in parallel while writing the
   v0.5.9 hot-fix; the agent's `git checkout main` stashed my
   in-progress work (without me realising), then a later branch
   switch left my edits orphaned across 3 different branches. Took
   ~10 minutes to untangle with cherry-pick + reset --hard. Lost
   nothing, but the user-facing latency on the v0.5.9 fix was
   higher than it should have been.

   Lesson: when running parallel agents that touch the working
   tree, **use git worktrees**. Each agent gets its own isolated
   directory. Multiple branches can be active simultaneously
   without stash dance. Used worktrees for the rest of the sprint
   after this realisation.

4. **mkdocs site shipped without verifying Mermaid actually
   rendered live.** PR #190 added the superfences custom_fence
   config, I tested locally + saw `<pre class="mermaid">` blocks
   in built HTML, called it done. The user reported "alguns
   graficos quebrados" the next day. Reality: Material 9.5's
   documented auto-load of mermaid.js doesn't fire — the script
   tag was missing on every architecture page. Fixed in PR #195
   with explicit `extra_javascript` + initialiser.

   Lesson: "the build produces the right HTML" is not the same as
   "the page renders correctly." For client-side JS features, the
   verification is loading the page in a real browser, not
   inspecting the build output. Same shape as the v0.5.4 / v0.5.8
   "I tested in isolation, not in the realistic environment"
   pattern.

5. **Per-service godoc coverage scorecard is bad.** Audited as
   part of the gomarkdoc work (PR #197): only `pkg/*` is at 100%;
   gateway 21%, message-bus 17%, app-management 35%, local-storage
   27%, core 39%, user-service 40%, common 49%. Sprint 2 Phase 6
   covered the kills but didn't enforce coverage on every
   exported decl across the whole service surface. Tracked in
   issue #196 with per-module raise plan.

6. **`ui/ ` (trailing space) 73MB junk lived in the repo for
   weeks.** Issue #174 was opened but the cleanup kept getting
   deferred. Closed in PR #194 finally. Trivial in scope, just
   needed a directed sweep.

7. **Conflated Sprint 4 + Sprint 4.5 + Sprint 5 prep into one
   working day.** The "stability + cleanup + docs + features"
   priorities all bled together. v0.5.10 ships docs polish that
   could've been Sprint 5; v0.5.9 hot-fix was technically Sprint 4
   recovery; the godoc + scalar work could've been in either. Net
   shipped the work; the sprint boundaries were artificial.

   Lesson: **sprint boundaries serve planning, not delivery.** When
   the work shape doesn't match the sprint label, ship the work
   and let the labels follow.

## Outcome scoreboard

- **3 releases**: v0.5.7, v0.5.8, v0.5.9
- **15+ PRs merged**: #181, #187, #183 (#85 trio), #188 (docs
  foundation), #189 (v0.5.8 release), #180 (DB paths), #176/#177
  (JWT keypair), #190 (Mermaid initial), #191 (docs wave 1), #192
  (v0.5.9 hot-fix), #193 (v0.5.9 release), #194 (README sweep),
  #195 (Mermaid actually-renders), #197 (godoc + Scalar)
- **3 ADRs added**: ADR-0020 (JWT keypair), ADR-0021 (Docker label
  namespace + AppData path), no third yet (the audit doc at
  `docs/audits/db-paths.md` substitutes)
- **1 issue closed implicitly**: #174 (ui/ junk) via PR #194
- **6 follow-up issues opened**: #184 (AppData migration tool),
  #185 (Sprint 5 plan refined twice), #196 (per-service godoc
  raise plan), and the 3 still-from-Sprint-3 (#169, #170, #171,
  #172, #173 are on Sprint 5)
- **1 prod incident**: v0.5.8 lock-out, hot-fixed in <5 min,
  permanently fixed in v0.5.9 with full test coverage
- **~80 regression tests added**: 16 + 12 + 4 from #85, 8 + 18
  from #179, 5 from #176, 7 + 4 + integration from v0.5.9
- **Net code direction**: significant deletion (READMEs, license
  headers, dead code) — exact LOC count not tracked

## What's now better

- Container labels use canonical `io.powerlab.v1.*` namespace; no
  more "PowerLab claims CasaOS containers" or vice-versa
- AppData tree per product (`/DATA/PowerLabAppData/<app>` for new
  installs); no more silent overwrite of CasaOS data
- DB paths have a single source of truth audit
  (`docs/audits/db-paths.md`) + 5-layer split-brain prevention,
  with auto-clean for unambiguous cases (no more lock-outs)
- JWT keypair persists across upgrades (no more
  refresh-logs-you-out)
- install.sh "CasaOS detected" relaxes from hard-block to notice
- Docs site live at https://neochaotic.github.io/powerlab/, MMermaid
  actually rendering, Go godoc surfaced for `pkg/*`, REST API
  reference page in nav
- Updater UI shows success toast + auto-reloads instead of
  silently completing
- Repo cleaned of ~40 inherited CasaOS license-header markers + 8
  module READMEs + the `ui/ ` 73MB junk

## Recommendations for Sprint 5

Per Sprint 5 plan #185 (already refined twice today):

1. **Make #169 a blocker.** Real v(N-1)→v(N) upgrade test as part
   of the release checklist, not a "we should do this" promise.
   Run it before tagging EVERY release. Half of #169 already shipped
   in v0.5.9; finish the rest first thing.

2. **Use git worktrees by default for parallel agents.** I
   discovered this mid-sprint. Document the pattern in CLAUDE.md /
   AGENTS.md so future agent dispatches don't repeat the stash dance.

3. **Per-service godoc raise** (#196) — picks the lowest-coverage
   modules first. gateway (21%, 14 funcs) is the smallest and
   highest-leverage; message-bus (17%, 41 funcs) is second.
   Attempting >70% coverage on each unblocks adding it to the
   docs site automatically.

4. **Bug-hunt sweep** — systematic walk-through of the running
   product, every glitch logged + ranked. The class of "I ship
   the fix but the upgrade context bites" repeats — bug hunt
   should specifically include "test on a host with realistic
   leftover state from prior versions."

5. **Dead-code elimination** — `staticcheck` + `go vet` per module,
   manual review of orphans. Sprint 4 exposed how much CasaOS
   inheritance is still in the tree; cleanup phase should prune
   what's never called.

6. **Phase 1.5 of release-checklist** automated as CI step (#169
   continuation) so EVERY future release passes through the same
   gate that would have caught v0.5.8 lock-out.

## Reference

- Sprint 4 prep: `docs/audits/sprint-4-app-management-prep.md`
- Sprint 5 plan: GitHub issue #185
- ADR-0020: JWT keypair persisted by default
- ADR-0021: Docker label namespace + AppData path
- DB paths audit: `docs/audits/db-paths.md`
- Sprint 3 retro (companion): `docs/audits/sprint-3-retrospective.md`
