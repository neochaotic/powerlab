# 0040 — Proportional engineering hygiene baseline (SAST + lint + metrics + tests)

- **Status:** proposed
- **Date:** 2026-05-18
- **Trigger:** External audit (2026-05-18) compared PowerLab to Kubernetes-grade hygiene and surfaced 6 concrete gaps. The same week the [`feedback_security_is_priority`](../audits/sprint-22-retrospective.md) memory hardened the catalog security floor against bash + warning models — but PowerLab's own Go code shipped with zero SAST, no linter beyond `go vet`, zero metrics endpoints, and 33% test coverage. The irony is the trigger: we vetted apps better than ourselves.

## Context

The audit's findings (paraphrased):

1. Test coverage 0.33 ratio test:code (K8s 0.7-1.0+, our own internal audits admit UI ~28%)
2. Zero SAST in CI — no `govulncheck` / `gosec` / `trivy` / CodeQL / Dependabot
3. No linter — no `.golangci.yml`, CI runs `go vet` + tests only
4. Observability almost nonexistent — 1 file with any metrics/tracing/health in 400 Go files. No `/metrics`, no `/readyz`+`/healthz` standardized, no distributed tracing
5. Structural debt — god files persist (`compose_app.go` 827 LOC, `disk.go` 923, `apps/+page.svelte` 1561)
6. Process maturity — no KEP-style design proposals, no public release cadence (still v0.x), no API compat guarantees, no security disclosure program

The audit framed these as "to reach Google/K8s grade." That framing is wrong for PowerLab. Two ADRs already constrain the answer: [`feedback_no_v1_without_alignment`](../../) defers v1.0 indefinitely + [`project_enterprise_pivot`](../audits/sprint-22-retrospective.md#project-enterprise-pivot) pushes the bar up but not to hyperscaler scale.

This ADR defines the **proportional** baseline. What we adopt now, what we ratchet, what we explicitly refuse for this product stage.

## Decision

Four hygiene families MANDATORY going forward. Three explicitly DEFERRED. The split is the lens "would enterprise IT accept this in production" (the enterprise-pivot lens) tempered by single-maintainer cadence reality.

### Adopted now

| Family | Tooling | Mode | Sprint |
|---|---|---|---|
| **SAST — vuln scan** | `govulncheck` per service in CI matrix | Strict from day 1 (vulnerable transitive deps fail the build) | Sprint 23 (PR 15) |
| **SAST — code patterns** | `gosec` via golangci-lint | Warn-only initially, ratchet to strict per ADR-0037 pattern | Sprint 23 (PR 14) → continuous |
| **Lint** | `golangci-lint` with 8 linters: govet, staticcheck, errcheck, gosec, revive, ineffassign, unused, gocyclo | Warn-only initially, ratchet to strict per family over N sprints | Sprint 23 (PR 14) → continuous |
| **Dependency hygiene** | `dependabot.yml` for Go + npm + GitHub Actions | Weekly PRs, auto-merge for patch only | Sprint 23 (PR 14) |
| **Metrics + health** | `/healthz` + `/readyz` + `/metrics` (Prometheus exposition) on every service | Mandatory on every new HTTP service; backfill existing 6 in Sprint 24 | Sprint 24 |
| **Test coverage push** | `vitest --coverage` UI threshold ratchet; `go test -coverprofile` per service | Ratchet ceiling: UI 28% → 40% by v0.7.10, → 50% by v0.7.30. Backend per service: case-by-case. | continuous |

### Explicitly deferred

| Family | Why deferred | Revisit trigger |
|---|---|---|
| **API versioning guarantees** | v1.0 deferred indefinitely (memory `project_v1_deferred`). No external consumers of the API yet — the UI is the only consumer and ships in the same tarball. Committing to API compat now would be performative. | When external consumer (MCP integration, third-party UI, etc.) lands |
| **KEP-style design proposals** | Single-maintainer + sprint cadence. KEP overhead pays off only when N stakeholders need to align before code. ADRs already cover "why we decided X". | When ≥2 maintainers + community contribs land |
| **Public security disclosure program** | No external traffic to support disclosure inbox triage. A SECURITY.md with email + GPG already covers responsible disclosure. | When public installs > N (TBD) or first external report |
| **CodeQL** | Heavyweight, GitHub-only, marginal value vs gosec for Go. `staticcheck` + `gosec` + `govulncheck` cover the same surface. | If gosec misses a class CodeQL would catch |
| **Distributed tracing** | Premature without ADR-0034 observability service to consume traces. Mark as Step 3 of ADR-0034. | After ADR-0034 Step 2 (`/metrics` aggregation) lands |
| **Trivy container scanning** | We don't build Docker images — we ship binaries. Apps' images are operator/upstream responsibility. | If/when PowerLab ships container images |

### god-file refactoring (audit point #5)

god-file splits stay **deferred until coverage clears 50%** (UI) / case-by-case (backend). Splitting at low coverage risks silent regressions (memory `feedback_tdd_strict` + `feedback_no_apagar_test_para_passar`). The Sprint 7 #123 carry remains carry until then.

When coverage is sufficient, splits happen as mechanical extractions (memory `feedback_critique_before_executing` — no clever refactor unless test surface supports it).

## Why "proportional" and not "K8s-grade"

K8s is a framework that hyperscalers run at planet scale + a multi-vendor governance body funds. Their hygiene reflects:

- 200+ active contributors → KEP needed for alignment
- SLA contracts with cloud vendors → API compat sacred
- Security inbox triaging dozens of reports/week → disclosure program critical
- Hyperscale ops debugging incidents → distributed tracing mandatory

PowerLab today:

- 1 maintainer + occasional contributor
- No external SLA, ships open-source for self-hosted operators
- Security inbox = email
- Single-host ops, log files + journalctl suffice for now

The 4 adopted families address PowerLab's actual risk surface (homelab+SMB → enterprise lab). The 3 deferred families address risk surfaces PowerLab doesn't have yet. Adopting them now would be cargo-cult.

## Memory alignment

| Memory | Alignment |
|---|---|
| `feedback_security_is_priority` | SAST adoption is the direct retort to "we vetted apps better than ourselves" |
| `project_enterprise_pivot` | "Would enterprise IT accept this?" lens drives the 4-adopted vs 3-deferred split |
| `feedback_tdd_strict` | Coverage ratchet + god-file defer-until-coverage are both grounded here |
| `feedback_no_v1_without_alignment` | API versioning deferral cites this directly |
| `project_v1_deferred` | Same |
| `feedback_airflow_level_docs` | Phase 3 mkdocs-material pre-v1.0 reference docs aligned with KEP-deferral (docs don't need KEP overhead) |
| `feedback_critique_before_executing` | The "tempted to copy K8s 1:1" anti-pattern this ADR rejects |

## Consequences

### Positive

- Closes the security-priority irony (Sprint 22 catalog work hardened the floor; this ADR hardens the ceiling of our own code)
- Establishes a reference for future "should we adopt X from $project" decisions
- Coverage ratchet creates measurable progress without a feature freeze
- Linter ratchet (warn → strict) follows the deadcode pattern from ADR-0037, which worked

### Negative / accepted trade-offs

- Linter strict-ratchet bloqueia features novas até cada family hit zero violations. Mitigated by warn-only start + per-family ratchet (one linter at a time).
- Coverage push consumes cycles. Mitigated by tracking ratio, not absolute; small features can ship without coverage backlog hit.
- Metrics endpoints add HTTP surface to maintain. Mitigated by shared middleware (one impl, 7 services consume).
- ADR-0034 observability bumped up in priority. This is **good** — alignment with enterprise pivot — but is multi-sprint.

### Risks

- **Lint ratchet stalls** if developers can't make progress against the baseline. Mitigated by per-linter activation; one bad family doesn't block the rest.
- **Coverage gaming** — devs writing low-value tests just to hit ratio. Mitigated by code review + memory `feedback_no_apagar_test_para_passar` (don't weaken tests).
- **Dependabot fatigue** — N PRs per week. Mitigated by auto-merge for patch versions + grouping.

## Rejected alternatives

- **"Copy K8s 1:1"** — overengineering. Treated as a strawman by this ADR.
- **"Defer all hygiene to v1.0 work"** — concedes the security-priority memory for ~6+ months until v1.0 is even on the calendar. Unacceptable trade-off given the catalog-security ironies.
- **"Adopt all 6 audit findings now"** — see "monster release / cramming bug class" analysis (Sprint 23 maintainer discussion 2026-05-18). Realistic impl is 3-4 weeks of focused work; v0.7.1 cut would slip indefinitely.
- **"Skip ADR, just ship the lint"** — future "why did we adopt X but not Y?" loses its anchor. ADR-0040 IS the anchor.

## Acceptance criteria

This ADR is accepted when:

1. PR 14 (golangci-lint + dependabot warn-only) merges
2. PR 15 (govulncheck CI matrix) merges
3. Sprint 24 PR (metrics endpoints) merges
4. Sprint 24 retro confirms the linter ratchet path is sustainable

If any of those reveal the proportional baseline is too aggressive (e.g. linter rules a non-trivial fraction of merges fail), this ADR is amended, not the implementation rolled back.

## References

- Sprint 23 maintainer discussion (this conversation, 2026-05-18) — original audit + risk analysis
- ADR-0037 deadcode delta-strict — pattern for warn-only ratchet
- Memory: feedback_security_is_priority, project_enterprise_pivot, feedback_tdd_strict, project_v1_deferred
- Sprint 22 retrospective — catalog security floor work this ADR mirrors for own-code
- Sprint 23 retrospective (forthcoming) — Layer 2/3 lockout defences this ADR's metrics endpoints complement
