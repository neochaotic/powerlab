# 0037 — Deadcode CI gate: delta-strict instead of zero-strict

- **Status:** accepted
- **Date:** 2026-05-15
- **Trigger:** Sprint 21 PR 7 sweep stopped at 37 remaining dead funcs in `backend/core/`; flipping `POWERLAB_DEADCODE_STRICT=1` (the Sprint 19 PR 5 / v0.7.0 commitment) is no longer the right move. We need a CI gate that catches NEW regressions without forcing the historical baseline to zero.

## Context

Sprint 19 PR 5 (`scripts/check-deadcode.sh` + matrix CI job) shipped warn-only, with a stated intent to "flip strict at v0.7.0 after the existing dead code is gone." Two sprints of cleanup (Sprint 19 + Sprint 21 PR 7) reduced the count from 200+ to ~40, but the remaining dead falls into three uncomfortable categories:

1. **Mixed-live files** — `pkg/utils/file/file.go` has 16 dead funcs interleaved with 8 live ones. Each removal requires per-func reachability analysis (does any current call site reach this function? Are the callers themselves dead? Will deleting cascade into pkg-private helpers that were keeping the live ones happy?). Estimated effort: a full sprint for `pkg/utils/file` alone.

2. **Test seams** — `service.NewHealthServiceWithLister` is reported dead by `golang.org/x/tools/cmd/deadcode` because it's not reachable from `main`. But `service/health_test.go` consumes it as the dependency-injection point for unit tests. Deleting it broke the test build. `deadcode` has no concept of test-only reachability, and Go has no `//go:build testseam` idiom. We'd need either:
   - A `//deadcode:ignore` pragma (doesn't exist upstream)
   - A maintained allowlist file checked by the CI script
   - Rewriting tests to not need the seam (kills unit testability)

3. **Future-feature helpers** — `pkg/docker/digest.go::GetManifest`, `pkg/docker/helpers.go::GetArchitectures` are unused TODAY but are the obvious building blocks for "show the upgrade-available indicator" (#258 backlog). Deleting them now means re-inventing the same code when the feature lands.

**Forcing zero is therefore the wrong goal.** It produces three bad outcomes: hours of surgical deletes for marginal value; deleted utilities re-implemented two sprints later; or `_ = funcName` style hacks scattered through code just to keep the linter quiet.

What the gate is REALLY for — operationally — is preventing **new** dead code from entering the tree. A function added today that's never called from any reachable path is almost always a mistake (forgotten wire-up, refactor leftover, copy-paste). That's the regression class worth blocking.

## Decision

Replace the `POWERLAB_DEADCODE_STRICT=0/1` toggle with a **delta-strict** gate.

### Mechanics

Each service has a baseline file at `scripts/deadcode-baseline/<service>.txt` storing the current count. CI runs `deadcode` and compares to the baseline:

- `count == baseline`  → exit 0 (no change)
- `count <  baseline`  → exit 0 + emit "you've reduced dead code by N; update the baseline file in this PR"
- `count >  baseline`  → exit 1 (FAIL — new dead code introduced)

The baseline file is **lower-bound only** — it represents the current ceiling, not a target. Reducing it (genuine cleanup PRs) is encouraged; raising it is forbidden. The baseline gets edited in the same commit as a dead-code-reducing change, keeping the audit trail in git history.

This gives us:
- ✅ Real protection against new dead code (the regression class we care about)
- ✅ No forced cleanup of inherited debt
- ✅ Incremental improvement is automatic (counters tick down when someone refactors)
- ✅ Test seams that *exist today* are grandfathered (in the baseline count); new ones still get flagged

### Env vars

```bash
POWERLAB_DEADCODE_MODE=delta   # the new default (was: warn-only)
POWERLAB_DEADCODE_MODE=warn    # legacy warn-only; rarely needed
POWERLAB_DEADCODE_MODE=strict  # original zero-strict; never enabled in CI
```

`POWERLAB_DEADCODE_STRICT=0/1` keeps working as an alias for `warn`/`strict` for backwards compatibility, but new code paths use `MODE`.

### CI changes

`.github/workflows/ci.yml` `backend-deadcode` job sets `POWERLAB_DEADCODE_MODE=delta`. The job continues to run per-service (matrix), so per-service baselines diverge cleanly without one service's regression blocking another.

### Baseline drift over time

Baselines are checked into git. A reduction PR's diff shows both the code deletion AND the baseline decrement; review is a single coherent thing. Increases are blocked at CI time before they ever land — there's no path for a baseline to grow without explicit commit.

If a service's baseline can be driven to 0 in a targeted future sprint (e.g., when `pkg/utils/file` finally gets the surgical pass), the gate seamlessly transitions to effective-strict for that service — no flag flip, no ADR amendment.

## Consequences

**Positive:**
- Stops the multi-sprint "are we ready to flip strict?" question. Answer: never matters; the gate works at every count.
- Removes the test-seam blocker; existing seams ride in the baseline, new ones get caught.
- Reductions are visible in PR diffs (baseline line changes), creating positive feedback for cleanup work.
- No flag-flip ceremony at v0.7.0 or v1.0.

**Negative:**
- Adds 7 small text files to git (`deadcode-baseline/<service>.txt` × 7 backend services).
- Baseline-update friction: a developer who reduces dead code MUST update the baseline file in the same commit, or CI fails (count below baseline). Mitigated by emitting an exact "update this file to N" instruction in the failure message.
- Loses the "single global goal" of "0 dead funcs" — replaces with "no NEW dead funcs." Some teams find the former motivating; this team prefers the latter pragmatism.

**Neutral:**
- The 37 remaining funcs in `backend/core/` stay as-is. They're tracked by baseline, not by a backlog ticket. A future contributor who wants to clean them up just deletes + decrements the baseline; nothing else needed.

## Out of scope

- Allowlist mechanism for individual symbols (not needed — baseline count covers it).
- Per-package baselines (file-level granularity isn't worth the bookkeeping; per-service is enough).
- Cross-service deadcode (e.g., common package functions only used by one consumer) — `deadcode` already handles this by walking per-main; no change needed.

## Supersedes

- The Sprint 19 PR 5 statement that `POWERLAB_DEADCODE_STRICT=1` would flip at v0.7.0. That commitment is retired; v0.7.0 will not include a flip.

## Tracking

The implementation lives in the same PR as this ADR:
- `scripts/check-deadcode.sh` gets a `MODE=delta` branch.
- `scripts/deadcode-baseline/{app-management,core,gateway,message-bus,sync-catalog,user-service,local-storage}.txt` files seeded with current counts (`local-storage` Linux-only, may seed at 0 from a CI run rather than locally).
- `.github/workflows/ci.yml` `backend-deadcode` step uses `POWERLAB_DEADCODE_MODE=delta`.
