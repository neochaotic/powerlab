# Sprint 8 proposal — Closure + Bug Bash

**Status:** authorized 2026-05-10
**Source:** Sprint 7 retrospective + open-issues triage
**Theme:** close the refactor track from #227, finish the audit-flagged panic fixes, attack the user-visible bug backlog from past sprints.
**Scope NOT in this sprint:** trust-dance redo (#118 — explicitly out per user, both Sprint 7 + 8). HTTPS re-enable, backend integration coverage (#150), real-upgrade test (#169).

The previous two sprints shipped ~50 PRs almost entirely internal (godoc raises + structural refactors). Sprint 8 swings the mix back toward user-visible quality — every item below either (a) closes a leftover from the refactor track or (b) fixes a bug that's been open ≥ 2 sprints.

---

## PR ordering — easy wins first, UI splits last

| # | PR | Class | Risk | Est. effort |
|---|---|---|---|---|
| 1 | #56 — fix i18n missing key `launchpad.uninstall` | trivial | none | 5 min |
| 2 | #50 — Settings → Security: CA download buttons error page | bug | low | 30 min |
| 3 | #48 — Custom App name field validation inconsistency | bug | low | 30 min |
| 4 | Audit #216 §C — convert 3 remaining panics in `local-storage/service/disk.go` (lines 90, 114, 147) to error returns. Same pattern as PR #230. | refactor | low | 30 min |
| 5 | #56 follow-up — implement-or-delete the skipped UI test in `TextEditor.test.ts:229` (closes audit #216 §F) | test | low | 30-60 min |
| 6 | #57 — Files editor textarea inert (regression) | bug | medium | 1-2 hours |
| 7 | #66 — Files: Delete button unreachable | bug | low-medium | 30-60 min |
| 8 | #65 — Custom App edit + redeploy fails with "ports in use" (own running container) | bug | medium | 1-2 hours |
| 9 | #219 — SocketIO CheckOrigin allowlist (security finding from Sprint 6 audit) | security | low | 1 hour |
| 10 | #227 PR 3 — Split `apps/+page.svelte` (1561 LOC) into 4 components + 4 stores | refactor | **medium-high** | 3-4 hours |
| 11 | #227 PR 4 — Split `settings/+page.svelte` (1469 LOC) into 8 panes | refactor | **medium-high** | 3-4 hours |

**Estimated total: ~2-3 focused days YOLO.**

---

## Per-PR risk + benefit

### Trivial / low-risk closures

**#56 (i18n key):** Add `launchpad.uninstall` key to all locales. **Risk: none.** **Benefit: visible bug eliminated** — UI was showing the literal key string instead of translated text.

**#50 (CA download error):** Investigate why the CA download buttons in Settings → Security land on an error page. Likely endpoint missing or path wrong. **Risk: low.** **Benefit: trust-dance prep** (clean state for Sprint 9).

**#48 (Custom App name validation):** Form rejects empty value but text editor accepts it. Add same validation to both paths. **Risk: low.** **Benefit: consistent UX, kills foot-gun.**

**3 remaining panics in `disk.go`:** Same pattern as the audit-flagged GetDownloadSingleFile fix (PR #230). Convert `panic(err)` to a proper error envelope. The pkg/lifecycle recover middleware catches them today, but users see 500 instead of expected 4xx. **Risk: low** — mechanical pattern application. **Benefit: closes audit #216 §C entirely.**

**`TextEditor.test.ts:229` skipped test:** Audit #216 §F flagged it. Either implement (the PUT-save toast verification) or delete (and accept manual verification). **Risk: low.** **Benefit: closes audit §F + removes "skipped test that hides missing coverage" debt.**

### Medium-effort bug fixes

**#57 (Files editor inert textarea):** Old regression. Editor opens in modal but text area doesn't accept input + save reports no toast. **Risk: medium** — could indicate a deeper modal-focus or store-binding bug. **Benefit: high** — file editor is a primary feature; broken since v0.3.x.

**#66 (Files Delete unreachable):** UX bug — no way to select a file without opening it, so Delete button never enables. Likely keyboard nav or click handler issue. **Risk: low-medium.** **Benefit: basic file-management operation works again.**

**#65 (Custom App ports-in-use false positive):** When editing + redeploying a custom app, the port-conflict check counts the app's OWN running container as a conflict. Fix: filter the check to exclude containers being recreated. **Risk: medium** — touches the install pipeline; need to verify it doesn't break first-install (where the port really IS in use). **Benefit: kills a workflow blocker for compose-app users.**

### Security

**#219 (SocketIO CheckOrigin allowlist):** Both WebSocket + polling transports currently `return true` for any origin. JWT auth on the gateway path mitigates, but the bypass IS real. Was tracked in PR #220's TODO/FIXME burndown with a `// see #219` reference. **Risk: low** — replace `func(*http.Request) bool { return true }` with an allowlist of known origins (loopback + LAN ranges + the configured device hostname). **Benefit: closes the only known security finding from the Sprint 6 audit + tightens the surface.**

### Refactor closure (UI splits — biggest items)

**#227 PR 3 — apps/+page.svelte split (1561 LOC):**
- Extract: `AppGrid.svelte`, `AppFilter.svelte`, `AppInstallDialog.svelte`, `AppDetailDrawer.svelte` (4 components)
- Extract: `useAppCatalog`, `useAppInstall`, `useAppFilter`, `useAppDialog` (4 Svelte 5 rune-backed stores)
- Result: `+page.svelte` → ~100-line composition file
- **Risk: medium-high.** UI behaviour is sensitive: drag-and-drop ordering, long-press context menu, install-progress streaming, container-logs popover. The E2E suite from #234 catches "page no longer renders" but won't catch animation glitches or stale prop bindings.
- **Mitigation:** run E2E locally (`cd ui && npm run test:e2e`) before push (per memory `feedback_e2e_run_local_first`). Per-component tests follow per extracted file. User does manual smoke test before merge.
- **Benefit: high** — most-edited UI file in Sprint 6, refactor pays back on every future feature touch.

**#227 PR 4 — settings/+page.svelte split (1469 LOC):**
- Already structured per-pane; each section becomes its own component under `ui/src/lib/components/settings/`
- Panes: `SystemPane`, `NetworkPane`, `SecurityPane`, `StoragePane`, `AppsPane`, `BackupPane`, `AdvancedPane`, `AboutPane` (8 components)
- Result: `+page.svelte` → ~80-line tab-router shell
- **Risk: medium-high** — same as PR 3. Settings page has fewer interactive elements than apps but more conditional rendering paths.
- **Mitigation:** same as PR 3.
- **Benefit: high** — sets up surface for per-pane unit tests + makes future pane additions trivial.

---

## Why this combination

1. **Sprint 6 + 7 were 50+ PRs of internal work.** User deserves visible quality next; bug bash delivers that.
2. **Refactor track must close** — leaving 2 of 7 PRs pending creates "is the refactor done?" ambiguity. Sprint 8 closes #227 entirely.
3. **UI splits have safety net now** (E2E #234) — they were unsafe to attempt in Sprint 7. Now they're attemptable.
4. **Easy wins first** sets momentum + clears small debt. The 5 trivial/low items at the top can ship in a single morning.
5. **Sets up Sprint 9** with a clean board for the trust-dance + HTTPS path.

## Position in the multi-sprint plan

| Sprint | Theme | Status |
|---|---|---|
| 5 | CasaOS strangler — kill the inherited deps | done (audit #203, ADR-0022) |
| 6 | Quality consolidation — godoc raise + obliterate wave | done |
| 7 | Refactor track + E2E baseline | done (5/7 PRs from #227, #108 baseline) |
| **8** | **Closure + Bug Bash (this proposal)** | **authorized** |
| 9 | Trust-dance redo + HTTPS re-enable + #40 one-tap CA UX | queued (gates v1.0) |
| 10+ | Test infra (#150 integration, #122 frontend DX, #169 real upgrade) | queued |

## Out of scope (explicit)

- **Trust-dance redo (#118)** — user removed twice (Sprint 7 + Sprint 8). Lands ≥ Sprint 9.
- **HTTPS re-enable (#130)** — gated by trust-dance.
- **Backend integration coverage (#150)** — needs Docker; deferred while user runs tests outside containerised environments.
- **Real upgrade test (#169)** — partial work landed Sprint 4; needs 2-step host setup.

## Authorization

Sprint plan authorized by user 2026-05-10 ("podemos seguir com a sugestao..    sem truted dance"). Execution may begin immediately; v0.5.12 is live in CI for tag-driven release.
