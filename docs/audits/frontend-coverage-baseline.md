# Frontend coverage baseline — Sprint 9

**Date:** 2026-05-11
**Tooling:** `vitest --coverage` with the v8 provider
**Source revision:** `main` post-Sprint-9-PRs A–H
**How to reproduce locally:** `cd ui && npm run test:coverage`

## Why this exists

The 2026-05-11 v0.6 audit listed "frontend test coverage measured + reported" as a must-have before bumping the minor version. Until now coverage was an unknown — the Sprint 8 retro flagged this as soft. Sprint 9 PR I lands the measurement infrastructure so v0.6 readiness gets a number instead of a guess.

## Baseline numbers

| Metric | % | Covered / Total |
|---|---:|---:|
| Statements | **16.77 %** | 1261 / 7517 |
| Branches | 16.57 % | 401 / 2419 |
| Functions | 16.44 % | 315 / 1915 |
| Lines | 17.78 % | 841 / 4729 |

Reported by the same `vitest --coverage` invocation CI now runs on every push to `main` and every PR. CI uploads the full HTML report as a build artifact (`frontend-coverage-<run-id>`, 14-day retention).

## What's covered well

| Surface | Statements | Notes |
|---|---:|---|
| `lib/i18n/index.svelte.ts` | **97.14 %** | tight unit-test surface; locale loaders + format helpers |
| `lib/utils/install-phase.ts` | 100 % | pure parser, table-tested |
| `lib/utils/probe.ts` | 100 % | likewise pure |
| `lib/api/*` | 60-100 % range | unit-tested per endpoint (Sprint 7 + 8 wave) |

## What's covered poorly

The big zeros are the Svelte page roots:

| Surface | Statements | Why |
|---|---:|---|
| `routes/apps/+page.svelte` | 0 % | 1561 LOC god file (Sprint 7 PR 3 carry-forward) |
| `routes/settings/+page.svelte` | 0 % | 1469 LOC god file (Sprint 7 PR 4 carry-forward) |
| `routes/dashboard/+page.svelte` | 0 % | only ever exercised by Playwright |
| `routes/files/+page.svelte` | 0 % | likewise |
| `routes/+page.svelte` (Launchpad) | 0 % | likewise |
| `lib/stores/files.svelte.ts` | 0 % | side-effect-heavy, no unit harness yet |
| `lib/stores/system.svelte.ts` | 0 % | poller — unit-testable but not exercised |
| `lib/stores/theme.svelte.ts` | 0 % | trivial; cheap to test |
| `lib/stores/ui.svelte.ts` | 0 % | trivial; cheap to test |
| `lib/components/files/{SshModal,Terminal}.svelte` | 0 % | jsdom-hostile (xterm + websocket) |

Playwright covers many of these at the page level (E2E suite expanded #234) but those tests don't contribute to the vitest coverage report. The two numbers measure different things on purpose.

## Targets

**Sprint 10 goal:** ≥ **25 %** statements. Reachable by:
1. Splitting `apps/+page.svelte` (Sprint 7 PR 3 carry) + adding component tests on the splits.
2. Splitting `settings/+page.svelte` (Sprint 7 PR 4 carry) + ditto.
3. Three trivial store tests (`theme`, `ui`, `system`) — each one is < 20 lines of test code.

**v0.6 cut gate (audit-aligned):** number stamped in CHANGELOG; no hard threshold yet. Threshold gates land Sprint 10 retro when we have a 2-data-point trend (16.77 → Sprint 10 number).

## CI artifact

CI run uploads `ui/coverage/` as `frontend-coverage-<run-id>` (14-day retention). Drill into uncovered lines via the HTML report: download the artifact, open `index.html`.

## Not yet measured

- E2E coverage (Playwright doesn't contribute to vitest reports). The Sprint 7 #234 baseline of 10 specs across 6 areas + Sprint 8 + Sprint 9 additions are a separate stream — when paired with the vitest number they describe the actual test surface.
- Backend coverage. Tracked separately in `docs/audits/godoc-coverage-2026-*` and friends — `go test -cover` per-service, raised in Sprint 6's godoc wave to ≥70 % per module.

## Reproduce

```bash
cd ui
npm install
npm run test:coverage
open coverage/index.html   # macOS
xdg-open coverage/index.html  # Linux
```
