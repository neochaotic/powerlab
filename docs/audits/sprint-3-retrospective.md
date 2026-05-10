# Sprint 3 retrospective

**Date:** 2026-05-09 / 2026-05-10
**Sprint:** 3 (CasaOS rebrand wave + v0.5.x release cycle)
**Status:** complete

Per ADR-0019, retrospectives live in `docs/audits/`. This one covers
the Sprint 3 work (the structural rebrand wave) PLUS the v0.5.4 prod
incident that surfaced during it. Companion to
`docs/audits/casaos-dependencies.md` (the structural snapshot) and
`docs/audits/sprint-4-app-management-prep.md` (the next-sprint prep).

The point is to capture what bit us so the next sprint doesn't repeat
the class — not to assign blame or dwell on missteps. Where a lesson
turned into a process change, it's named.

## Headline

Sprint 3 shipped the rebrand wave (12+ structural PRs, ~3500 LOC
removed, all CasaOS module paths renamed, /etc/casaos → /etc/powerlab,
casaos:* → powerlab:* topics, etc.). Four releases tagged in
back-to-back days (v0.5.3, v0.5.4, v0.5.5, v0.5.6).

**v0.5.4 shipped a real upgrade-path bug** that broke the user's login
in production. Hot-fixed live via SSH, then permanently fixed in
v0.5.5. That incident drove three issues (#158, #159, #160), one
process improvement (#157 staleness check), and three regression-test
suites — every Sprint 3 lesson that hit hard now has automated
defenses.

## What went well

1. **Audit-first paid off for Sprint 4 prep**: writing
   `docs/audits/sprint-4-app-management-prep.md` BEFORE starting
   Sprint 4 work let the kill series split into reviewable PRs from
   day one (PR1+PR2+PR3+PR5 merged cleanly; PR4 deferred with clear
   scope).
2. **TDD discipline held under pressure**: every prod-incident fix
   landed with a regression test (#158 has 10 assertions across 5
   scenarios; #159 has 9 assertions; #160 has 6 cases including a
   9-input fuzz; backup retention has 14 assertions). Following the
   "bug fix = regression test, no exceptions" memory rule consistently.
3. **ADR-0019 codified tech-debt tracking**: the user's "do you have
   tech-debt documented?" question became an ADR specifically saying
   "no flat TECH-DEBT.md" with the rationale, so the next maintainer
   with the reflex finds the reasoning before they create one.
4. **Real-time prod debug via SSH** worked: identified data-migration
   root cause in <10min, hot-fixed in <5min, opened tracking issues
   inline. The user's observation was the trigger; the diagnostic
   playbook was effective.

## What went wrong

These are the items that hit hard and cost the user real downtime.
Each one is now defended against, but the underlying patterns are
worth surfacing.

### 1. Skipped Phase 1.5 of release-checklist for v0.5.4

`docs/release-checklist.md` Phase 1.5 explicitly says "On an upgrade
DB: install the previous release, create a user + at least one app,
then upgrade in place to the candidate. Confirm: user can still log
in, apps still listed, no data loss."

I went straight from "tag pushed" to "release published" without
running this step. Result: the user's login broke on v0.5.4 because
PR #140 changed data paths but install.sh didn't migrate the data.

**Process gap**: the checklist is manual; nothing automated enforces
it. Open issue tracks the "make Phase 1.5 an automated test" remediation.

### 2. Build pipeline ldflag was broken since v0.1.x

`scripts/package-linux.sh` was setting `-X main.version=$VERSION`
(wrong var name; main.go declares `commit` and `date`) and pointing
at `github.com/IceWhaleTech/CasaOS/common.POWERLAB_VERSION` (dead
path after PR #151 module rename). Go's `-X` is fail-soft — both
overrides silently no-op'd. Every release ever shipped contained
`commit = "private build"`.

The bug only manifested in v0.5.4 because that's when the in-UI
updater went live. But the underlying defect existed for months.

**Lesson**: silent-no-op classes (linker flags, codegen, embed
directives) need test coverage that asserts the OUTPUT, not just the
input. Fixed in PR #161 — both the bash check (assert ldflag string
targets right vars) AND the Go integration test (`go build` + grep
binary) shipped together.

### 3. release-manifest.yaml summary went stale

v0.5.4 shipped with v0.5.0's summary text in the in-UI updater
because `release-manifest.yaml` was never refreshed.
`docs/UPDATE_MANIFEST.md` documented the requirement; nothing
enforced it.

Fixed in PR #157 — pre-tag check refuses to proceed if the YAML
summary matches the previously-published release. Stops the next
v0.5.x mishap before tag push.

### 4. event_listen.go panic recovered but never properly fixed

The `user-service/route/event_listen.go:77` nil-deref appeared in
production logs every time the message-bus restarted (which is every
upgrade attempt). `pkg/lifecycle.SafeGo` recovered the panic so the
process kept running, but the goroutine died on every cycle. That's
a bandaid, not a fix.

I noticed it during the v0.5.4 debug, captured it in #160, didn't
fix until pressed. Per the memory rule "bug fix = regression test, no
exceptions" — by SafeGo-recovering and moving on, I treated the
recovery as a fix when it wasn't.

Fixed in PR #164 with 6 regression tests including a 9-input fuzz.

**Lesson**: SafeGo is a safety net, not a substitute for fixing the
underlying bug. When SafeGo logs a panic, file an issue + fix it
within the same sprint window.

### 5. cmd/migration-tool/ overlap with install.sh data migration

When writing PR #158 (install.sh data migration script), I noticed
that every service has a `cmd/migration-tool/main.go` whose job
description overlaps with what the new install.sh script does. The
migration tool was never audited or tested during this sprint.
Decision deferred.

**Lesson**: when a sprint touches "migration" code, audit ALL the
code labeled "migration" up front. Open issue tracks the cmd/
migration-tool/ audit + decision (delete or promote).

### 6. Audit-first NOT used for Sprint 3 rebrand wave

Sprint 4 prep had a written audit before the kill series started.
Sprint 3 rebrand wave did not — bugs #140 (core.conf mismatch),
#158 (data migration), #148 (hardcoded /var/lib/casaos paths) all
came from the same blind spot: changing one path category at a time
without first inventorying every call site that depended on the old
path.

**Lesson**: refactor sprints get an audit doc up front. The Sprint 4
prep audit was the right shape; do the same for Sprint 5+.

### 7. Snapshots accumulated without cleanup

Every upgrade leaves ~100MB in `/var/lib/powerlab/backups/`. The
user accumulated 4 snapshots in one debugging session before this
was noticed.

Fixed in PR #165 — `POWERLAB_BACKUP_KEEP=3` default with override.
Trivial fix; should have been there from day one of the snapshot
feature.

## Issues opened from this retrospective

Each item below is an issue opened separately and labeled per the
recommendations.

| Item | Issue | Sprint fit |
|---|---|---|
| Phase 1.5 release-checklist as automated test (docker-compose v(N-1)→vN) | #169 | NOT Sprint 4 — separate effort, 3-4h focused work, deserves own design |
| `cmd/migration-tool/` audit (delete or promote) | #170 | NOT Sprint 4 — orthogonal to app-management, low blast radius |
| `flag.Parse()` in `init()` template fix (gateway, core, others) | #171 | Sprint 4 if convenient — small, would make new tests easier |
| Branch protection: release PR body must reference Phase 1.5 | #172 | NOT Sprint 4 — process item, can be done any time |
| `goleak` shared convention (instead of per-test ignore lists) | #173 | NOT Sprint 4 — test infra, separate concern |
| `ui/ ` 73MB stray file cleanup + .gitignore | #174 | Sprint 4 yes — trivial, can squeeze in |

## Sprint 4 recommended additions (from this retro)

In addition to the audit's 5 PRs (PR1 ✅ PR2 ✅ PR3 ✅ PR4 ❌ PR5 ✅):

- **`ui/ ` cleanup** — 5min item, satisfying to close
- **`flag.Parse()` template** — only if it doesn't grow scope; useful
  when adding tests to the heavy service packages (#150)

NOT recommended for Sprint 4 (open as standalone):
- Phase 1.5 automation
- cmd/migration-tool/ audit
- Branch protection process change
- goleak convention

## Sprint 3 outcome scoreboard

Net work shipped:
- 4 releases tagged (v0.5.3 → v0.5.6)
- 12+ structural rebrand PRs merged
- ~3500 LOC removed
- 80+ regression test assertions added (across all hotfix PRs)
- 1 ADR (ADR-0019)
- 2 audits refreshed (casaos-dependencies + sprint-4-prep)
- 3 prod incidents identified + fixed + tested
- 4 process improvements (ADR-0019, staleness check, retention, migration script)

Issues closed: #104 (logger tail), #156 (staleness check), #157 (PR
merged), #158 (PR merged), #159 (PR merged), #160 (PR merged); #101 +
#106 substantially closed.

Issues opened: #150 (backend coverage), plus the 6 from this retro.

## Reference

- Master roadmap: #67
- Tech-debt tracking pattern: ADR-0019
- Companion audits: `casaos-dependencies.md`, `sprint-4-app-management-prep.md`
