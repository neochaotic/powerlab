# 0016 — Kill scope: modular kill in Sprints 1-4, full extraction in Sprint 4.5

**Status:** accepted
**Date:** 2026-05-09
**Tags:** casaos-strip, scope, sprint-planning, v0.4.0, v1.0

## Context

Each service "kill" in the CasaOS-strip roadmap (#67) is a multi-PR
series. The default reading is "kill = remove every CasaOS dependency
from this service". The reality is more nuanced: each service imports
**multiple** packages from `backend/common/` (which is
`github.com/IceWhaleTech/CasaOS-Common`), and replacing all of them at
once balloons the per-kill effort to multiple sprints each — defeating
the per-sprint cadence the umbrella roadmap promises.

After implementing kill #2 (gateway) parts 1-2 and surveying what
remains, the gateway still imports **9 distinct CasaOS-Common
packages**: `pkg/security`, `utils/jwt`, `utils/file`, `utils/systemctl`,
`utils/http`, `utils/constants`, `utils/devmode`, `utils/common_err`,
`model`, and `external`. Some are 1-hour swaps (constants, devmode);
others are sprint-sized projects (security with CA + cert + HSTS +
mobileconfig signing).

We need to decide: when is a service "killed"?

## Decision

Adopt **two stages** of kill, scheduled differently:

### Stage A — "Modular kill" (Sprints 1-4, per service)

A service is **modular-killed** when:

1. Its `go.mod` module path is PowerLab-owned
   (`github.com/neochaotic/powerlab/backend/<svc>`).
2. It uses `pkg/lifecycle.RecoverMiddleware` and
   `pkg/tracing.Middleware` on every `http.Server.Handler`.
3. Its logger is `pkg/logging` (no more
   `CasaOS-Common/utils/logger` imports).
4. Dead code from the audit (#62) has been per-function reviewed
   and either deleted or reclassified.

The service may still import other CasaOS-Common packages
(`pkg/security`, `utils/jwt`, etc.). These are **documented debt**,
tracked per service, and addressed in Stage B.

Each service's modular kill is N PRs in series — typically 4 (rebrand,
middleware, logger swap, dead-code review). After all 6 services are
modular-killed (end of Sprint 4), the foundation packages are in use
everywhere PowerLab-owned code emits errors and logs.

### Stage B — "Full extraction" (Sprint 4.5)

A dedicated stabilization-and-extraction sprint after Sprint 4 finishes
its modular kill of `app-management`. Stage B:

1. **Extracts** `backend/common/pkg/security` into PowerLab-owned
   `backend/pkg/security` (CA + cert + HSTS + mobileconfig). The 6
   services that need certs migrate to the new package together.
2. **Extracts** `backend/common/utils/jwt` into `backend/pkg/auth`,
   coupled with the user-service kill (which already touched JWT in
   Sprint 2 — Stage B finishes the move).
3. **Migrates** the 7 small remaining packages
   (`utils/file`, `utils/http`, `utils/systemctl`, `utils/constants`,
   `utils/devmode`, `utils/common_err`, `model`, `external`) — most
   are 1-hour stdlib or local-package swaps.
4. **Deletes** `backend/common/` once it is unreferenced.
5. Plus the **stabilization** work the user has reaffirmed multiple
   times: extensive E2E coverage, every reported bug closed or
   explicitly deferred, alignment with the user before tagging.
6. Tag **v1.0** when alignment is reached.

## Rationale

- **Per-sprint cadence is real.** The roadmap promises a tag per
  sprint (v0.4 / v0.5 / v0.6 / v1.0). Forcing each kill to be "full"
  pushes timelines out and breaks the cadence. Modular-killing buys
  sprint-grained progress without sacrificing eventual end-state.
- **Some extractions are batch-sized, not per-service.**
  `pkg/security` is used by **all** services that touch HTTPS
  (currently the gateway via UDS, but in principle any service that
  emits TLS). Extracting it once for everyone in Stage B is cheaper
  than each kill re-doing the analysis.
- **`utils/jwt` couples with user-service.** Moving JWT during the
  gateway kill creates rework when the user-service kill rewrites the
  auth boundary. Doing it as part of user-service kill (Sprint 2) and
  finishing in Stage B is one path of work, not two.
- **The 7 small packages batch better at the end.** When the surface
  is locked (no service is mid-rewrite), we know exactly which
  functions are still called and can replace just those. Migrating
  speculatively now risks porting code we don't actually use.
- **Quality discipline.** Smaller atomic PRs ship faster and review
  better. A modular kill is 4 PRs of ~5 files each. A full kill in
  one go would be 30+ files in a single review — exactly the shape
  we explicitly want to avoid.

## Consequences

- After Sprint 4 (end of modular kills): every service is
  PowerLab-owned at the module level, but `backend/common/` still
  exists and is still imported by some services for the deferred
  packages. **`backend/common/` is NOT deleted at end of Sprint 4.**
- Sprint 4.5 is a real, named sprint — not a "we'll get to it
  someday." It produces v1.0.
- Documentation must explicitly call out which CasaOS-Common
  packages remain after each modular kill (per service). Tracked in
  `docs/architecture/casaos-strangler.md` and per kill PR.
- `v0.4.0` (end of Sprint 1) ships a system where gateway and
  message-bus are modular-killed but still depend on common for
  security / jwt / file / systemctl / etc. **This is fine** — the
  product works; the foundation is solid; the strip continues.

## Alternatives considered

- **Full kill per service in its sprint.** Rejected — pushes timeline
  out 4-8 weeks, defeats per-sprint cadence, kills review quality.
- **Big-bang Sprint 1: replace `backend/common/` entirely up front.**
  Rejected in ADR-0011 (strangler pattern) for risk reasons; this ADR
  reaffirms.
- **No "kill" boundary at all — just incrementally remove imports
  PR by PR.** Rejected — sprints need a tag-able milestone, and
  "service is modular-killed" is the natural unit of progress to
  track per sprint.

## Reference

- Umbrella roadmap: #67
- Strangler pattern: ADR-0011
- Foundation packages: ADRs 0012-0015
- Strip progress tracker: `docs/architecture/casaos-strangler.md`
  (updated each kill PR with which CasaOS-Common imports remain)
