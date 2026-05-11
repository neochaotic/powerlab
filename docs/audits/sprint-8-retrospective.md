# Sprint 8 retrospective

**Date:** 2026-05-11
**Sprint:** 8 (Closure + Bug Bash + Carnage)
**Status:** complete

Per ADR-0019, retrospectives live in `docs/audits/`. This one covers
the Sprint 8 work: a v0.5.12 release cut, eight production-bug fixes
with TDD-strict regression coverage, nine kill-list PRs that removed
~11.6 k LOC of CasaOS-era dead weight, and three audit waves that
materially changed how we frame the v0.6 bump.

## Headline

Sprint 8 was supposed to be a closure + bug-bash sprint. It ended up
being **the largest single-sprint deletion event** in the project's
history. **18 PRs** landed (or are merged-pending): one release cut,
eight bug fixes (all with regression tests authored failing-first),
and **nine consecutive kill PRs that removed ~11 600 LOC** of code
the UI never consumed and no script ever invoked.

The carnage was not a re-scope — it grew organically from a single
question ("does CasaOS upstream stop today, what breaks?") into a
methodical sweep of every CasaOS-era pattern still squatting in the
repo. The user's pivot from "audit it" to "auditar não resolve, se
apaga" mid-sprint set the tone.

## What we shipped

### Bug-bash + regression coverage (PRs A–H)

| PR | Issue | Change |
|---|---|---|
| A | #56 | i18n key verified stale — closed without code change (parity confirmed across 3 locales) |
| B (#238) | audit §C | 3 `panic(err)` → logged error + `return false` in `disk.go::EnsureDefaultMergePoint` |
| C (#239) | #50 | `/settings` walkthrough cert links: 5 inline anchors → buttons calling existing `downloadCA(format)` helper (binary artifacts only, per memory `feedback_no_text_cert`) |
| D (#240) | #48 | Inline name validation on `/apps/new` — `nameError` prop into `ComposeForm`, red border + `aria-invalid` + helper text under input. Playwright regression locks the visual state |
| E (#241) | #219 | `engineio` websocket + polling `CheckOrigin` allowlist — same-origin always, plus operator-configured `[security] AllowedOrigins`. ADR-0023 documents threat model. 6 + 1 vitest cases |
| F (#242) | #57 | Production-fidelity Playwright for the editor inert-textarea bug (jsdom doesn't run CodeMirror's input pipeline; the `.cm-editor` mount + dirty-indicator flip is the only real proof) |
| G (#253) | #66 | Tri-state header select-all checkbox in FileTable. Toolbar Delete now reachable without Cmd-click chord shortcuts. 3 vitest + 1 Playwright |
| H (#254) | #65 | Edit-mode redeploy routes to PUT `applyComposeAppSettings` (which has skip-self port logic) instead of POST install. New `client.putYaml()` + 3 vitest cases (success, failure, percent-encode) + Playwright asserting PUT not POST |

Every fix landed with a failing-first regression test per memory
`feedback_bug_regression_discipline`. The Sprint 7 E2E baseline (#234)
was the safety net that made the inline-validation + select-all + edit-
mode-PUT changes shippable in hours instead of days.

### The carnage — 9 kill PRs (~11 600 LOC removed)

| PR | Item | Δ |
|---|---|---|
| I (#262) | `notify_old.go` (62) + `migration_0412_and_older.go` (77) | -139 |
| L (#263) | Samba feature entirely (9 full-file deletes + 11 surgical edits + drop `go-smb2` dep) | -813 |
| J (#264) | `appfile2compose` CasaOS conversion tool | -95 |
| N (#265) | Quick-win sweep — 40 orphan `.github/workflows/`, 5 sysroot files, `core/Makefile`, `model.DeviceInfo` + `GetDeviceInfo()`, 3 dead UI consts + `ZTInfo` type, swagger contact rebrand, `"Casa"` → `"PowerLab"` discovery fallback | -1709 (48 files) |
| O (#266) | ZeroTier full surface + `WsSsh` (NOT `WsShell`) + `file_websocket.go` peer-broadcast (closes #261) + `pkg/ddns/` orphan consts | -915 |
| R (#267) | `cmd/validator/` + `cmd/message-bus-docgen/` ×4 services | -505 |
| M (#268) | `cmd/migration-tool/` tree across all 6 services + orphan `MigrationTool` interface in 2 files | -1248 |
| K (#269) | `backend/cli/` subproject — never built, never distributed, CI explicitly skips | -4840 (61 files) |
| Q (#270) | App-management `/v1/*` API surface (route/v1/, route/v1.go, openapi_v1.yaml, index_v1.html, gateway routing) | -1365 |

Every kill PR builds + passes its service's existing test suite
before push. Each carries a "verified" line that names the proof.
Three independent audit agents validated the kill targets before
each batch ran (no audit became a kill without independent
confirmation that nothing wired into production).

## What went well

1. **Audit-then-act loop generalized cleanly.** The pattern that
   shipped 9 kill PRs in a single afternoon: `gh issue view`
   → spawn focused agent in background → wait for kill-safe report
   → execute deletes in worktree → `go generate && go build`
   → confirm tests green → commit + push + PR. Each iteration was
   ~10–20 minutes once the pattern stabilized. No PR got pulled
   for a missed reference because the agents listed every
   surgical-edit site upfront.

2. **TDD discipline held under volume.** Eight bug fixes, eight
   failing-first regression tests, zero "weakened to pass" cases
   (memory `feedback_no_apagar_test_para_passar`). The new
   `feedback_no_protobuf_yet` memory captures the Scalar+oapi-codegen
   verdict so the question doesn't get re-litigated next sprint.

3. **The `"Casa"` discovery fallback was the smoking gun.** The most
   visible "PowerLab pretends to be CasaOS on the LAN" residue —
   `route/init.go:51`, exactly one line, hidden behind a fallback
   that only fires when `/etc/os-release` has no `MODEL` field. The
   third-party voice flagged it as the ZimaBoard cordão umbilical;
   killing it in PR N was a 4-character change that materially
   changed what other PowerLab instances see when they discover us.

4. **HTTPS deprioritization unblocked v0.6 framing.** The user's
   2026-05-11 message ("https e trudets dance esta dispriorizado
   ate segunda ordem") superseded the v0.5.2 release-manifest
   promise that v0.6 ships with HTTPS re-enabled. Memory
   `project_sprint7_no_trust_dance` updated to reflect the new
   indefinite deferral. Without this pivot, the v0.6 bump would
   have been blocked on trust-dance work that's not happening.

5. **The "auditar não resolve, se apaga" pivot.** The user's
   pushback halfway through the carnage — "Código morto não se
   audita em bloco de notas, se apaga" — was the right correction.
   Before that, the `docs/audits/raw/` deadcode-* files were
   piling up and the kill PRs were bottlenecked on me finishing
   "the next audit doc" instead of just shipping the deletes. After
   the pivot, the agents wrote validation reports, I shipped PRs
   directly, and the deletes accelerated.

## What we got wrong

1. **Issue #261 was a self-inflicted error.** I opened a feature
   issue ("Other PowerLabs nearby — peer discovery") based on the
   backend-features audit, then the dead-code agent flagged the
   same feature as CasaOS Snapdrop legacy. Two competing audits,
   one issue, contradictory recommendations — the user had to
   adjudicate. Closed #261 as "decided not to ship" and folded the
   delete into PR O. Lesson: when the same feature surfaces from
   "what could we add?" AND "what should we kill?", pause before
   opening either issue and reconcile first.

2. **PR #241's package-linux.sh inline conf wasn't updated.** The
   AllowedOrigins section landed in `build/sysroot/etc/powerlab/
   message-bus.conf.sample` (which IS embedded into the binary as
   a fallback), but `scripts/package-linux.sh` writes its own
   inline `message-bus.conf.sample` to `/etc/powerlab/` that gets
   precedence at runtime. So fresh installs get the message-bus
   conf without the `[security]` section. Functional impact is
   zero (empty AllowedOrigins is the secure default — same-origin
   only — which is what we want), but the operator who tries to
   add cross-origin allowlist entries by editing the conf will
   find the section missing. Follow-up: tiny patch to package-
   linux.sh to mirror the sysroot conf.

3. **Build/sysroot conf.samples almost got swept.** The "more
   legacy" audit flagged ALL of `build/sysroot/` as orphan based
   on package-linux.sh's inline writes. False — five services
   embed their sysroot conf.sample into the binary via `//go:embed`
   so the sysroot copies ARE the production fallback when no
   conf file exists yet. Caught by checking `go:embed` directives
   before deleting; the truly-orphan files (casaos.service unit,
   rclone.service, mergerfs.ctl, env stub) went in PR N. Lesson:
   when an audit flags a directory wholesale, validate with
   `grep go:embed` before trusting it.

## Process / scope notes

- **9 issues opened** for CasaOS-residue items the audit surfaced
  (#243–#252). Six are critical-tier (DefaultPassword, User-Agent,
  health glob, JWT aud, test-linux-e2e.sh casaos.service, fstab
  backup names). Three are alto-tier rebrand work (CLI binary
  rename, UI catalog label, v2 type rename). The first round of the
  6 criticals is queued for Sprint 9.

- **7 features opened** from the unused-backend-features audit
  (#255–#260, with #261 closed-as-killed). Disk temperature + SMART
  status (#255) is the highest bang/buck — backend already populates
  `Disk.Temperature`, the change is pure Svelte. mergerfs Pool wizard
  (#256) is the most differentiating but biggest scope.

- **`feedback_no_protobuf_yet` memory** captures the audit verdict
  that protobuf for inter-service comms isn't worth it — `oapi-
  codegen` already covers ~80 % of the type-safety win. Saves a
  re-litigation cycle in Sprint 9.

- **`project_sprint7_no_trust_dance` memory** updated from "deferred
  Sprint 7+8 → Sprint 9" to "indefinitely deferred per 2026-05-11
  message". This supersedes the v0.5.2 release-manifest promise
  that "v0.6 ships with HTTPS re-enabled".

## Sprint 8 metrics

- **PRs:** 18 (1 release + 8 bug fixes + 9 kill)
- **LOC removed (kill PRs):** ~11 600 across 9 PRs
- **LOC added (bug fixes):** ~800 (mostly tests + new validation logic)
- **Net LOC delta:** ~−10 800 (an order of magnitude bigger than any
  prior sprint)
- **Issues closed:** ~12 (incl. #48, #50, #56, #57, #65, #66, #219,
  #261, plus the issues quietly closed-by-PR)
- **Issues opened:** 16 (9 CasaOS criticals + 7 hidden features)
- **ADRs:** 1 new (ADR-0023 — SocketIO Origin allowlist)
- **Memories saved:** 2 (`feedback_no_protobuf_yet`,
  `project_sprint7_no_trust_dance` updated)
- **Audits run:** 4 (linux-agnostic + WSL2 + macOS portability,
  v0.6 minor-bump readiness, CasaOS residual, dead-code kill-list,
  unused-backend-features, zima.go IceWhale coupling, Samba ↔ Files
  coupling — six in total counting the focused ones)

## Carry-forward to Sprint 9

1. **6 CasaOS criticals (#243–#248)** — DefaultPassword, User-Agent,
   health glob, JWT aud, test-linux-e2e.sh casaos.service, fstab
   backup names. ~6 h estimated, 4 PRs.

2. **3 CasaOS rebrand-altos (#249–#251)** — CLI rename (mostly moot
   now that `backend/cli/` is gone in PR K — close #249 as stale),
   UI catalog label, v2 type rename. ~3 h.

3. **#252** — refresh `casaos-residue-2026-05-10.md` audit doc to
   reflect everything closed since (this whole sprint's worth of
   closures).

4. **PR S follow-up** — user-service v1 dead handlers (~600 LOC,
   originally part of PR Q's scope, split out for surgical clarity).

5. **Highest-bang/buck feature for v0.6 headline material:** disk
   temperature + SMART (#255). One Svelte PR, no backend work.

6. **package-linux.sh AllowedOrigins follow-up** — tiny patch to
   mirror the sysroot conf addition from PR #241.

7. **Bake window** — v0.5.12 cut today (2026-05-11). Per the v0.6
   audit, bumping minor needs ≥7 days from the cut with no production
   regressions in the bug-bash class. v0.6 readiness: gated on at
   least one visible UI redesign + frontend coverage measurement,
   NOT on HTTPS (memory updated).

## Closing

Sprint 8 turned a closure sprint into the deletion event the repo
needed. The principle that emerged — "Código morto não se audita
em bloco de notas, se apaga" — should be the default disposition
on any future audit finding flagged as zero-caller dead weight.
PowerLab is now ~12 k LOC lighter, 9 issues clearer, and one step
closer to "core orchestrates containers; everything else is an App
Store app".
