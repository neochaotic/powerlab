# Frontend coverage baseline — Sprint 9 → Sprint 11

**Date:** 2026-05-11
**Tooling:** `vitest --coverage` with the v8 provider
**Source revision:** `main` post-Sprint-9-PRs A–H (baseline) + Sprint 11 PR #296 (second data point)
**How to reproduce locally:** `cd ui && npm run test:coverage`

## Why this exists

The 2026-05-11 v0.6 audit listed "frontend test coverage measured + reported" as a must-have before bumping the minor version. Until now coverage was an unknown — the Sprint 8 retro flagged this as soft. Sprint 9 PR I lands the measurement infrastructure so v0.6 readiness gets a number instead of a guess.

## Trend

| Date       | Sprint | Statements | Branches | Functions | Lines | PR |
|---|---|---:|---:|---:|---:|---|
| 2026-05-11 | S9  | 16.77 % | 16.57 % | 16.44 % | 17.78 % | #281 |
| 2026-05-11 | S11 | **28.75 %** | **24.21 %** | **26.41 %** | **29.60 %** | #296 |

Delta from S9 baseline to S11: **+11.98 pp statements, +7.64 pp branches, +9.97 pp functions, +11.82 pp lines.** All four Sprint 11 targets met (≥ 25 / ≥ 24 / ≥ 23 / ≥ 26).

## Baseline numbers (frozen)

| Metric | % | Covered / Total |
|---|---:|---:|
| Statements | 16.77 % | 1261 / 7517 |
| Branches | 16.57 % | 401 / 2419 |
| Functions | 16.44 % | 315 / 1915 |
| Lines | 17.78 % | 841 / 4729 |

## Sprint 11 numbers (current)

| Metric | % | Covered / Total |
|---|---:|---:|
| Statements | **28.75 %** | 2155 / 7495 |
| Branches | 24.21 % | 586 / 2420 |
| Functions | 26.41 % | 513 / 1942 |
| Lines | 29.60 % | 1397 / 4719 |

Reported by the same `vitest --coverage` invocation CI now runs on every push to `main` and every PR. CI uploads the full HTML report as a build artifact (`frontend-coverage-<run-id>`, 14-day retention).

## Where the Sprint 11 lift came from

| Surface | New test files | Notes |
|---|---|---|
| Stores (`lib/stores/*.svelte.ts`) | `theme`, `ui`, `system`, `settings`, `versionHandshake` | 37 tests, raised lib/stores from 20.21 % → ~50 % statements |
| Settings panes | `AppsPane`, `GeneralPane`, `NetworkPane`, `SecurityPane`, `AboutPane` | 25 tests covering prop wiring, callback fires, and conditional render branches |
| Apps modals | `ForkAppModal`, `UninstallAppModal`, `UpdateAppModal` | 17 tests for open/close + onConfirm contract |
| Apps surface | `AppCard` | 21 tests covering installed/store/running/stopped + every isPowerLabApp branch |
| Dashboard widgets | `MiniProgress`, `RadialGauge`, `Sparkline` | 20 tests; covered status-color branches + value clamping |
| Utility regression locks | `compose-name`, `compose-extension`, `format`, `os` (extended) | Locks #240 inline name validation + ADR-0021 extension priority chain |

Regression locks wired in this PR (≥ 3 from the Sprint 11 charter):

1. **#240** — Custom App empty-name silent fallback to `'web'`. New util `lib/utils/compose-name.ts` + `compose-name.test.ts`.
2. **ADR-0021 / #201** — `x-powerlab → x-web → x-casaos` extension priority chain. `compose-extension.test.ts` pins the read priority + the round-trip write that preserves the author's chosen key.
3. **#242** — TextEditor inert/disabled state during file open. Already locked in `TextEditor.test.ts` from Sprint 9; re-verified to still pass after settings/apps splits.

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

**Sprint 10 goal (achieved Sprint 11 — same week):** ≥ 25 % statements. Hit at 28.75 %. Reached via stores + settings panes + apps modals + AppCard + dashboard widgets + utility regression locks.

**Next step (Sprint 11):** vitest threshold gate (issue #297) — once this PR lands, gate the four metrics at the Sprint 11 floor minus a 1-pp safety margin (≥ 27 / ≥ 23 / ≥ 25 / ≥ 28) so a regression breaks CI.

**v0.6 cut gate (audit-aligned):** number stamped in CHANGELOG; threshold gates land via #297 after this PR. With two data points trending strongly up (+12 pp statements in one sprint), v0.6 readiness on the coverage axis is satisfied.

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
