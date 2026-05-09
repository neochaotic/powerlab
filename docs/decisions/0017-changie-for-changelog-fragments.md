# 0017 — `changie` for changelog fragment workflow

**Status:** accepted
**Date:** 2026-05-09
**Tags:** developer-experience, ci, sprint-2, v0.5.0

## Context

Sprint 1 produced 9 stacked PRs in a final merge cascade. Every PR
edited the same `[Unreleased]` section of `CHANGELOG.md`. Every merge
after the first triggered a CHANGELOG-only conflict — git couldn't
auto-merge because two diffs targeted the same insertion point.

The conflicts were mechanical (always "keep both entries, in time
order"), so each one cost ~30 seconds of `<<<<<<< HEAD` /
`>>>>>>> branch` resolution. Across the 9-PR cascade, that's ~5 minutes
of manual toil per merge wave plus context-switching overhead. None
of it added value — every conflict resolution kept both sides
verbatim.

Sprint 2 is forecast at ~15-18 PRs (more code, more bugs, two kills).
At Sprint 1's per-PR conflict rate, we're looking at ~10-15 minutes
of pointless toil compounded across the merge cascades. Beyond the
time, the conflict friction biases reviewers toward squash-merging
to "just get it in" — which loses commit granularity.

This is a structural problem, not a discipline problem. The fix is
structural: each PR writes to its own file, never to a shared one.

## Decision

Adopt **`changie`** (https://changie.dev/) for changelog fragment
workflow.

Workflow:

1. Each PR adds a single tiny YAML file under
   `.changes/unreleased/<id>.yaml` with shape:
   ```yaml
   kind: Fixed
   body: |
     Files: save toast invisible because z-index conflict between
     toast container and editor modal.
   custom:
     Issue: "3"
   ```
2. Two PRs never edit the same file. **Zero CHANGELOG conflicts.**
3. At release time, `changie batch <version>` consumes all
   fragments and generates a new section in `CHANGELOG.md`.
   Fragments archive to `.changes/<version>/` for posterity.
4. CI gates that any PR touching `backend/`, `ui/src/`, or
   `scripts/` includes at least one fragment.

`CHANGELOG.md` is no longer hand-edited day-to-day — it's
**generated**. The header (`# Changelog ...`) stays static via
`.changes/header.tpl.md`; sections below it are managed by changie.

## Rationale

- **Structural elimination of conflict class.** No clever process
  fix; the file simply isn't shared anymore.
- **Single Go binary.** No runtime cost in the PowerLab product —
  changie runs in CI and at release time, not in any service.
- **YAML fragments are portable.** If we ever switch tools
  (towncrier, changesets, custom), the fragments are mechanically
  convertible. We're not locking into changie's internals, only
  its file convention.
- **Keep-a-Changelog format preserved.** The generated `CHANGELOG.md`
  matches the format we already use; readers see no break.
- **Per-kind grouping.** Fragments declare `kind: Added/Changed/...`
  and changie groups them per release section automatically.

## Consequences

- **`CHANGELOG.md` becomes generated.** Direct edits are now
  reserved for typo fixes in already-released sections. Trying to
  edit `[Unreleased]` is a smell.
- **CI gates fragment presence** on PRs that touch watched paths.
  This catches the "I forgot to changelog" case at PR time, not at
  release time.
- **Existing CHANGELOG.md content is preserved as-is.** No migration
  of v0.3.x / v0.4.0 entries to fragments — those sections continue
  to render exactly as they were. The header replacement is
  cosmetic.
- **Release script gets a step.** `changie batch <version>` runs
  before `git tag` to regenerate the file. Easy to wrap in a
  release script later.
- **Contributor onboarding gains one step.** `go install` changie,
  then `changie new` per PR. Documented in `CONTRIBUTING.md`.

## Alternatives considered

- **`towncrier` (Python)** — well-maintained, used by Pip / Pytest /
  Twisted. Rejected: adds Python tooling to a Go-first repo just to
  manage YAML files. Net dependency complexity not justified.
- **`changesets` (JS/TS)** — used by Vite, Astro, Tailwind. Rejected:
  the model is npm-package-centric (versions per package), awkward
  for our hybrid Go-services + TS-frontend layout.
- **Custom shell script** — possible (`cat .changes/unreleased/*.yaml
  | yq ...` in a release script). Rejected: reinventing wheel
  poorly; changie's CI gating, kind groups, and edit-friendliness
  are non-trivial to replicate.
- **Squash-merge everything** — sidesteps the conflict by
  collapsing each PR to a single commit before merge. Rejected:
  loses commit granularity in `git log` (each part of a kill series
  becomes one commit instead of 4), which hurts `git bisect` and
  retrospective code archaeology.

## Implementation

This PR:

- `.changie.yaml` config — kinds, format, paths
- `.changes/header.tpl.md` — static prelude for `CHANGELOG.md`
- `.changes/unreleased/.gitkeep` + `98-changie-adoption.yaml`
  — first fragment, demonstrates the pattern
- `CONTRIBUTING.md` updated with the new workflow
- `.github/workflows/ci.yml` — new `changelog-fragment` job that
  gates PRs touching watched paths

After merge:

- Add `Changelog fragment present` to branch protection's required
  status checks.
- Sprint 2 PRs use the new workflow as the reference. No backfill
  of pre-changie entries.

## Reference

- `changie` repo: https://github.com/miniscruff/changie
- Issue: #98
- Sprint 1 cascade that motivated this: PRs #80-#96 (16 PRs, 9 in
  the final batch, every merge-after-first had CHANGELOG conflict)
