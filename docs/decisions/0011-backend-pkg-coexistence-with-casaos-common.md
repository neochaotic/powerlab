# 0011 — `backend/pkg/` coexists with `backend/common/` during CasaOS strip

**Status:** accepted
**Date:** 2026-05-08
**Tags:** architecture, casaos-strip, v0.4.0

## Context

`backend/common/` is the shared module imported by all 6 backend services
(gateway, app-management, core, user-service, message-bus, local-storage).
Its `go.mod` declares the module path as
`github.com/IceWhaleTech/CasaOS-Common` — i.e. the very identity of the
shared module is owned by the upstream we are forking from. Inside it
live `model/`, `middleware/`, `interfaces.go`, generated code, and
utilities that every service depends on.

The North Star (umbrella issue #67) is to strip every line of
CasaOS-derived code from PowerLab and reach v1.0 owning the foundation
end-to-end. Concretely we need a place to put PowerLab-owned shared
code, and a story for how we get from "everything imports
`IceWhaleTech/CasaOS-Common`" to "no service imports anything CasaOS".

We considered three approaches:

1. **Big-bang elimination of `backend/common/` in Sprint 1.** Estimated
   8–12 weeks of isolated work (re-implement the whole module, migrate
   every service, regenerate `codegen/` from PowerLab-owned OpenAPI
   specs) — on top of foundation packages and two service rewrites
   (gateway, message-bus). Schedule risk catastrophic; quality bar
   guaranteed to slip. Rejected.
2. **In-place rename / takeover of `backend/common/`.** Change the
   module path on `backend/common/go.mod` to a PowerLab-owned URL and
   immediately update every importer. Touches all 6 services in one
   PR, breaks every fence test simultaneously, and produces a single
   massive review. Rejected — high blast radius without proportional
   gain over the chosen path.
3. **Coexistence with strangler migration.** Create a parallel module
   under `backend/pkg/`, route new code there, and migrate services
   off `backend/common/` one at a time as each is rewritten. Chosen.

## Decision

- Create a new Go module rooted at `backend/pkg/` with module path
  `github.com/neochaotic/powerlab/backend/pkg`. The path matches the
  filesystem location relative to the repo root, satisfying Go module
  conventions without `replace` directives.
- `backend/common/` remains untouched and importable. Treat it as
  read-only: **no new code lands in `backend/common/`**, and no
  refactors there beyond the surgical "remove gateway-exclusive code"
  cleanups documented per kill.
- The Sprint 1 foundation packages — `pkg/logging`, `pkg/errors`,
  `pkg/lifecycle`, `pkg/tracing` — live under
  `backend/pkg/logging/`, `backend/pkg/errors/`, etc.
- Service rewrites (Sprint 1: gateway and message-bus; Sprint 2-4:
  the rest) import **only** from `backend/pkg/`, never from
  `backend/common/`.
- After the final kill (Sprint 4: app-management), `backend/common/`
  is expected to be unreferenced and is deleted in the same PR.

## Rationale

- **Risk-adjusted velocity.** Each sprint reduces CasaOS surface by
  one or two services. The migration is reviewable in isolated PRs;
  failure of one kill does not block the others.
- **Test-matrix containment.** When gateway is rewritten, the test
  matrix only needs to prove gateway parity. The other 5 services
  continue to import the unchanged `backend/common/` and their
  existing tests continue to apply unchanged.
- **Clear identity boundary.** `backend/pkg/` is unambiguously
  PowerLab-owned. Anything in `backend/common/` is "still CasaOS, do
  not touch beyond what the kill PR strictly requires." This is
  legible to anyone reading the tree without prior context.
- **Module path matches filesystem.** Go's module resolution is
  happiest when the import path equals the repo URL plus the path to
  `go.mod`. Picking `github.com/neochaotic/powerlab/backend/pkg`
  avoids the `replace`-directive rabbit hole that would come from
  trying to make imports look like `github.com/neochaotic/powerlab/pkg`.

## Consequences

- For ~4 sprints, two shared modules coexist in the tree. Reviewers
  must check that new imports point at `backend/pkg/...` and reject
  any new dependency on `backend/common/`.
- Some short-term duplication: if `backend/common/utils/` has a
  function we need in `backend/pkg/`, we re-implement (or copy with
  attribution) rather than depend across the boundary. This is
  intentional — taking a dependency on `common` from `pkg` would
  defeat the whole isolation.
- Coverage gates apply to `backend/pkg/...` only. `backend/common/`
  keeps whatever coverage it has today; we do not invest there.
- Risk of confusion if a contributor adds new code to
  `backend/common/`. Mitigation: a top-level
  `backend/common/CONTRIBUTING.md` documenting the freeze, plus a
  CI lint that fails the build if new files are added under
  `backend/common/` after a cut-off date (TBD per kill PR).

## Alternatives considered (recap)

- **`backend/internal/`** instead of `backend/pkg/`. Rejected because
  several services intentionally consume each other's helpers via
  HTTP/UDS rather than direct Go imports; the `internal/` semantic
  (compile-time enforcement of "only this subtree may import")
  would prevent legitimate test fixtures and tooling from reaching
  the package. The convention chosen — `pkg/` — is intentionally
  shareable.
- **Module path `github.com/neochaotic/powerlab/pkg`** (without
  `backend/`). Rejected: the actual `go.mod` lives at
  `backend/pkg/go.mod`, and pretending otherwise via `replace` makes
  the module unconsumable from outside the repo. Honest path beats
  cosmetic.
- **PowerLab GitHub org as the module path host.** Rejected for now:
  no such org exists today. If/when it is created, a `replace`
  directive plus a redirect at the GitHub level handles the rename
  with low pain.

## Reference

- Umbrella roadmap: #67
- Sprint 1 foundation packages: #68 (logging), #69 (errors),
  #70 (lifecycle), #71 (tracing)
- Sprint 1 kills: #72 (message-bus), #73 (gateway)
