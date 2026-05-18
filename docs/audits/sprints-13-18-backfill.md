# Sprints 13-18 — backfill retrospective

**Reconstructed**: 2026-05-18 during Sprint 23 process-recovery sweep.
**Sources**: `git log`, ADR files, .changes/ entries, memory log.
**Method**: per-sprint git-grep on commit subjects + cross-referenced PR numbers + tag history.

The cadence between 2026-05-12 and 2026-05-15 was very tight (multiple sprint
labels touching the same days because work pivoted mid-day). Each entry below
captures intent + actual deliverables + carries — but if you need wire-level
detail, the cited commit ranges are the source of truth.

## Why this exists

Sprint 23 audit caught that 7 of 13 sprints had no retro doc in
`docs/audits/`. Sprint 3-4, 6-12, 19-22 had them; **5, 13, 14, 15, 16, 17, 18
did not**. The institutional memory chain was broken right through the
period that defined the audit + observability + catalog story.

Sprint 5 falls outside the "last 10 sprints" focus of the audit. This doc
backfills 13-18. Going forward every sprint cut ships its retro doc as a
companion PR (lesson encoded in Sprint 22's retro under "What didn't work").

---

## Sprint 13 — Install UX hardening + SSE protocol fixes

**Dates**: 2026-05-12 → 2026-05-16
**Cuts**: v0.6.5 (#250)
**Theme**: Hot-fix the install modal "stuck on Preparing" bug class +
ship sync-catalog as a /usr/bin binary so catalog freshness decouples
from tarball freshness.

### Headline deliverables

- **SSE multi-line bug fix** (#341, `24df9ea`) — `Task.Subscribe()` was
  emitting the entire log buffer as a single channel message; the SSE
  handler wrote it as `data: <multi-line>\n\n`. Browser `EventSource`
  spec drops any line inside `data:` that doesn't start with a known
  field name. UI never saw `Phase N/M:` markers. Latent since v0.1.0,
  user-visible only after Sprint 13.2 install modal rewrite.
- **sync-catalog ships at /usr/bin** (`backend/sync-catalog/main.go`,
  Sprint 13.3 #248) — install.sh runs it post-install to refresh the
  catalog against current upstream. Decoupled catalog freshness from
  tarball freshness. (Repealed in Sprint 23 PR #451 per ADR-0039.)
- **URL-embedded host placeholder substitution** (#426, `9eafca6`) —
  install-time substitution of `<host>` / `<lan-ip>` in compose URLs
  so dynamic hostnames flow into env vars correctly.

### Carries closed in later sprints

- `#259` per-service journald viewer — picked up Sprint 18, completed
  Sprint 21 PR 4
- `#150` fuse build-tag + testcontainers — done as Sprint 13.5

### Bug class memorialised

- `feedback_no_ship_before_tests` — v0.6.1 → v0.6.2 → v0.6.3 was a
  3-release "I fixed it" → "no you didn't" cycle that triggered the
  rule "failing test FIRST. Manual SSH+browser verify BEFORE tag."

---

## Sprint 14 — Logs CLI + install lifecycle parity

**Dates**: 2026-05-13 → 2026-05-14
**Cuts**: v0.6.8 (task #263)
**Theme**: Operator visibility (powerlab-logs CLI + install-log capture
+ Docker rotation) + UI-side install lifecycle parity for Custom App.

### Headline deliverables

- **powerlab-logs CLI** (#150, multiple commits ending `c87e96a`) —
  standalone binary at /usr/bin/powerlab-logs with `app` subcommand
  wrapping `docker logs`. Install-log capture into journald with
  rotation.
- **Custom App install lifecycle parity** (#247 #341, `38079bf`) —
  Custom App `/apps/new` modal now drives the SAME SSE-backed install
  flow as community apps. Closed the dichotomy where Custom App used
  a separate spinner-only path that didn't surface phase markers.
- **UI 401 handler** (`bb9503a`) — centralised auth-expiry handler that
  triggers logout + toast (originally labelled "Sprint 16 C3" in the
  commit; landed during Sprint 14 window).

### Bugs fixed inline

- `#332` Custom App fork volumes form `[object Object]` rendering
- `#334` Postgres bind-mount perms (blinko-db)
- `#345` Install-modal unification via `createSubscriber`
- `#344` Playwright install-flow E2E test (mandatory pre-tag from now)

### Memory entries created

(none explicitly tagged Sprint 14 — most carry from earlier rules)

---

## Sprint 15 — Defense-in-depth for upgrade-401

**Dates**: 2026-05-14 (single day, intensive)
**Cuts**: v0.6.11 (`023a748`)
**Theme**: Close the v0.6.7 → v0.6.10 upgrade-401 bug class on multiple
levels (L1 backend tolerance, L2 UI E2E coverage, L3 ESLint rule).

### Headline deliverables

- **L3 backend Bearer prefix tolerance** (#342, #356) — JWT middleware
  accepts RFC 6750 `Bearer <token>` and bare-token formats. Earlier
  versions silently rejected one of the two formats.
- **L2 Playwright in-UI upgrade test** (#355) — covers the entire
  in-UI upgrade button flow against a real backend.
- **L1 ESLint flat config banning raw `fetch()` in stores/routes**
  (#353, #354) — automated catcher for the bug class that caused the
  3-release loop. The api client injects auth; raw `fetch()` bypasses
  it → silent 401.

### Memory entries created

- `feedback_raw_fetch_in_stores_is_bug_class` — root cause of v0.6.7 →
  v0.6.10. Tests must include a contract assertion on `Authorization`
  header, not just state-machine assertions.

---

## Sprint 16 — Audit pipeline JSONL migration + Settings → Audit pane

**Dates**: 2026-05-14
**Cuts**: v0.6.12 (`48e0dd8`)
**Theme**: Migrate audit storage from SQLite to JSONL (ADR-0035) +
deliver the Settings → Audit pane that consumes it.

### Headline deliverables

- **ADR-0035 audit JSONL** (`172021e`) — accepted + implemented same day
  (`d808fea`, #363). Backend audit pipeline moved from a per-row SQLite
  insert per request to append-only JSONL with an in-memory ring buffer
  for query. Performance + simplicity win + unblocks future ADR-0034
  observability service (audit was the original blocker per memory
  `project_audit_sqlite_revisit_before_adr_0034`).
- **Settings → Audit pane** (#357 B1f, `5902c0d`) — operator-facing UI
  for the audit log. Filters by user_id + since + kind.
- **ADR-0034 standalone observability + MCP service** (`2d3db6f`) —
  proposed (not implemented). Designs the future split where audit +
  metrics + traces ship in a separate process. **As of Sprint 23 still
  in `proposed` status — flagged in the Sprint 23 missing-work audit
  as a major MCP/enterprise-pivot deliverable that lost track.**

### Bugs fixed inline

- `#359 Sprint 16: godoc 100% — clean redo` opened but NOT closed (Phase
  2 docs per `feedback_airflow_level_docs` memory).

### Memory entries created

- `project_audit_sqlite_revisit_before_adr_0034` — ADR-0035 must land
  before ADR-0034 observability impl starts. (ADR-0035 landed; ADR-0034
  still hasn't.)

### Caught in Sprint 23 audit

- `feedback_playwright_mocks_are_not_e2e` was created in Sprint 16 after
  audit endpoints shipped with `19/19 Playwright + 524 vitest green` but
  were unreachable from the public port — mocks don't test the route
  exists, only the render given fake data. Required real-server smoke
  tests for new endpoints + a curl-the-real-port pre-tag step.

---

## Sprint 17 — Observability skeleton + orphan cleanup hot-fix

**Dates**: 2026-05-14
**Cuts**: (none — work folded into v0.6.12 / Sprint 16's cut)
**Theme**: First impl step of ADR-0034 observability service + tactical
hot-fix for Docker auto-renamed orphans on uninstall.

### Headline deliverables

- **Observability standalone-service skeleton** (`d7c5e0a` —
  `feat(observability): standalone service skeleton (Sprint 17 ADR-0034
  step 1)`). Per ADR-0034 the eventual MCP-served metrics/audit/traces
  process. **Step 1 only — never completed. Sprint 23 audit flagged
  this as the missing enterprise-pivot headline.**
- **Docker auto-renamed orphan cleanup** (`6a5c55a`, #367 Sprint 17 C1)
  — uninstall flow now detects Docker's auto-rename pattern
  (`<container>-1`, `<container>-2`...) and removes them. Earlier
  uninstalls left behind orphan containers that polluted `docker ps`.

### Carries open as of Sprint 23

- **ADR-0034 standalone observability service impl** — only the
  skeleton landed. Service runs but doesn't yet collect/serve metrics
  or MCP-format audit. **Critical gap for enterprise pivot**.
- `feedback_clean_up_planted_test_data` memory created during this
  sprint after the v0.6.12 cut wasted a round-trip due to leftover
  `aaaaaaaaaaaa_2fauth` test plant ambushing the user's install click.

---

## Sprint 18 — Journald viewer Phase 4 + golden-path E2E + Custom App YAML-first epic

**Dates**: 2026-05-15
**Cuts**: (none — Sprint 18 epic #376 never closed; deliverables
folded into Sprint 19 v0.6.14 + Sprint 21 v0.6.16)
**Theme**: Open-ended hardening epic that was supposed to land
v0.6.13 (hardening) → v0.7.0 (with Custom App YAML-first + observability).
Most P0 items shipped, the headline (Custom App YAML-first + observability
completion) didn't.

### Headline deliverables

- **Journald SSE stream endpoint for /logs Phase 4** (#259, #420 →
  `d390df7`) — backend `/v1/logs/services/{service}/stream` SSE
  endpoint exposing `journalctl -fu` follow output. Frontend integration
  landed in Sprint 21 PR #283/#284 (per-service tabs + live follow).
- **Real-backend Playwright coverage for golden-path journeys**
  (#409, `bb48fcf`) — Playwright spec that hits a real backend (not
  mocked) for login + install + uninstall. Defence against the
  Sprint 16 mocks-aren't-E2E lesson.
- **Sprint 18 epic #376** opened as the tracking issue. **Still open
  in Sprint 23.** Charter items that DIDN'T land:
  - Custom App YAML-first migration
  - Observability completion (ADR-0034 impl Step 2+)
  - Several P0 hardening items inferred from the epic checklist

### Carries open as of Sprint 23

- `#376` Sprint 18 epic — has never been formally closed or split.
  Recommend closing in Sprint 24 sweep with explicit "these items
  landed in Sprints 19/21/23, these items DEFERRED indefinitely" map.
- `#386` Adventure Log install 400 BadRequest — open since this sprint;
  likely fixed by Sprint 22 image-skeleton-seed (PR #444) but never
  confirmed/closed.
- `#402` catalog hostname underscore — opened during this sprint,
  partial work in Sprint 21 PR 1, likely killed by Sprint 22 ADR-0039
  wipe but not formally closed.

### Memory entries created

- `feedback_playwright_mocks_are_not_e2e` (already noted under Sprint 16
  but reinforced by the Sprint 18 audit-endpoints-unreachable incident
  that prompted the golden-path real-backend coverage push)

---

## Cross-sprint patterns these retros surface

- **Three "missing impl" items lost between Sprint 17 and Sprint 23**:
  ADR-0034 observability completion, Sprint 18 epic close-out, godoc
  100% pass. Sprint 24 should adopt the explicit "closing carries"
  ritual.
- **Mid-day sprint relabelling** (Sprint 14 commits tagged "Sprint 16",
  Sprint 17 work folded into Sprint 16's cut) means sprint boundaries
  are fuzzier than the retro structure assumes. Going forward, retro
  doc cites COMMIT RANGES not just sprint numbers.
- **Bug classes ALWAYS produce memory entries**: SSE multi-line,
  raw-fetch-in-stores, Playwright-mocks-not-E2E, planted-test-data
  cleanup — these are the lasting institutional value, not the
  per-PR deliverable list.

## Sprint 23 follow-up actions opened by this backfill

- [ ] Close or split `#376` Sprint 18 epic
- [ ] Verify + close `#386` Adventure Log + `#402` catalog underscore
- [ ] Decide on ADR-0034 observability completion (defer formally or
      schedule as Sprint 24 headline)
- [ ] Decide on `#359` godoc 100% (Phase 2 docs commitment per memory
      `feedback_airflow_level_docs`)
