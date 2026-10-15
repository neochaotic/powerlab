# Enterprise maturity assessment — 2026-05-21

**Scope:** advisory assessment of PowerLab against a 3-tier enterprise-maturity
framework, grounded in `main` at v0.7.2. Verified against the repo (file paths,
ADRs, CI config), not assumed.

**Target profile:** enterprise-*trustworthy* for demanding technical individuals
and small technical companies — **not** regulated-enterprise (SOC2/HIPAA) scale.
This calibration drives the weights and the "do not do" list.

**Team constraint:** 1 human owner + 1 AI agent. No QA/SRE. 2-week sprints,
sustainable pace. Decisions via ADRs.

Related: extends [ADR-0040](../decisions/0040-hygiene-baseline.md) (hygiene
baseline / proportional coverage). Execution tracked in the companion issue.

---

## Section 1 — Current-state diagnosis

### Tier 1 — Trustworthy baseline

| Item | Status | Evidence |
|---|---|---|
| T1.1 unit cov ≥60% backend | 🟡 PARTIAL | ~33% aggregate (ADR-0040); `common/utils/jwt` ~90.8%, `app-management/service` ~35% after the v0.7.2 adversarial push. Coverage-cadence rule exists (+10pp/surface/cycle). Below 60%. |
| T1.2 blocking SAST | 🔴 ABSENT (as a gate) | `govulncheck` runs `continue-on-error: true` (`ci.yml`) — warn-only. `.golangci.yml` enables `gosec` but **no `golangci-lint` job exists in any workflow**. Config exists; enforcement does not. |
| T1.3 /metrics + /healthz + /readyz | 🔴 ABSENT | Zero matches across `backend/`. No liveness/readiness/Prometheus surface. |
| T1.4 automated E2E (install→…→uninstall) | 🟡 PARTIAL | `frontend-e2e` (Playwright mock) + `backend-integration` (real `docker compose`, #338) + `browser-e2e-real` (install→SSE-phases→uninstall vs real stack, **release gate**, `ci.yml`). start/restart/stop not yet covered. |
| T1.5 threat models | 🟡 PARTIAL | `docs/concepts/security-model.md` + security ADRs (0033/0035/0039). No per-surface (gateway/MCP/audit) formal threat model. |
| T1.6 Dependabot + process | ✅ DONE | `.github/dependabot.yml`, 9 ecosystems. Caveat: warn-only, no boot gate (viper-1.21 boot panic entered this way; tracked). |
| T1.7 SECURITY.md + SLA | ✅ DONE | Disclosure email + 5-business-day ack SLA. |
| T1.8 logger migration complete | 🟡 PARTIAL | **33 files** still import `common/utils/logger`; **24** import `pkg/logging` (~42% migrated). Active dual-pattern debt. |
| T1.9 operational runbooks (10) | 🟡 PARTIAL | `docs/operations/` + recovery/lockout docs. Likely 4–6 of the 10 critical scenarios. |

### Tier 2 — B2B differentiators

| Item | Status | Evidence |
|---|---|---|
| T2.1 upgrade tests N→N+1 | 🟡 PARTIAL | `scripts/test-upgrade-resolves-stale-legacy_test.sh`, `check-install-upgrade-clean_test.sh`, #303. Shell-level; not a versioned state-preservation matrix. |
| T2.2 chaos tests | 🔴 ABSENT | — |
| T2.3 soak tests | 🔴 ABSENT | `pkg/lifecycle` would make this tractable. |
| T2.4 MCP fuzzing | 🔴 ABSENT | MCP doesn't exist (ADR-0034 proposed). N/A. |
| T2.5 perf benchmarks + gate | 🔴 ABSENT | — |
| T2.6 cosign + SBOM | 🔴 ABSENT | No cosign/syft/sbom. Tarballs ship a SHA-256 manifest only. |
| T2.7 fixed release cadence | 🟡 PARTIAL | Just shifted sprints→releases (v0.7.x). Forming, not published. |
| T2.8 API compat documented+versioned | 🟡 PARTIAL | oapi-codegen + Scalar; `/v1` + `/v2`. No written compat/deprecation policy. |
| T2.9 reproducible build | 🟡 PARTIAL | `-trimpath`; no `SOURCE_DATE_EPOCH`/attestation. |

### Tier 3 — Full maturity

| Item | Status | Evidence |
|---|---|---|
| T3.1 bus factor >1 | 🔴 ABSENT | Single maintainer. Structural. |
| T3.2 governance docs | 🔴 ABSENT | `CONTRIBUTING.md` exists; no `GOVERNANCE.md`/`MAINTAINERS.md`. |
| T3.3 conformance tests | 🔴 N/A | No forks/variants. |
| T3.4 public roadmap | 🔴 ABSENT | No `ROADMAP.md`; ADRs+epics internal. |
| T3.5 bug bounty | 🔴 ABSENT | No traffic to justify. |
| T3.6 distributed tracing (OTel) | 🔴 ABSENT | `pkg/tracing` exists but **no OpenTelemetry import** — stub seam. |
| T3.7 multi-arch + per-arch tests | 🟡 PARTIAL | `Package smoke (amd64)` and `(arm64)` build+smoke both; integration/browser-e2e amd64-only. |
| T3.8 compliance docs | 🔴 ABSENT | No regulated client in pipeline. |

## Section 2 — Weights & dependencies

Impact = how much it closes the *enterprise-trust* gap for the stated target.
Effort = 1 (days) … 5 (quarters) for 1 human + 1 agent.

| Item | Impact | Effort | Depends on | Unblocks |
|---|---|---|---|---|
| T1.2 blocking SAST | 5 | 1 | — | trust review; T1.5 |
| T1.3 /healthz+/readyz | 4 | 2 | — | T2.2/T2.3; updater preflight |
| T1.3 /metrics | 2 | 2 | healthz | T3.6 |
| T1.1 cov ≥60% | 5 | 4 | — | safe refactor; T1.8; T2.x |
| T1.4 E2E lifecycle | 4 | 3 (½ done) | T1.3 | T2.1, T2.2 |
| T1.5 threat models | 3 | 2 | — | trust review; MCP |
| T1.8 logger migration | 3 | 3 | — | T3.6; debt removal |
| T1.9 runbooks | 3 | 2 | — | self-service users |
| T2.1 upgrade tests | 5 | 3 (½ done) | T1.4 | predictable upgrades |
| T2.6 cosign+SBOM | 3 | 2 | release pipeline (have it) | vendor review |
| T2.7 cadence | 3 | 1 | — | T2.1 planning |
| T2.2 chaos | 3 | 4 | T1.3 | resilience claims |
| T2.3 soak | 2 | 3 | T1.3, lifecycle | leak detection |
| T2.5 perf gate | 2 | 3 | — | perf claims |
| T2.9 reproducible | 2 | 3 | — | T2.6 attestation |
| T3.1 bus factor | 5 (for sale) | 5 | external (people) | T3.2/T3.8 |
| T3.6 OTel | 2 | 4 | T1.8 | deep debug |
| T3.7 per-arch tests | 2 | 3 | T1.4 | ARM claim |

Highest-leverage early moves are asymmetric: **T1.2** (impact 5 / effort 1 — flip
existing config) and **T1.1** (impact 5 / effort 4 — long pole). Resilience items
(T2.2/T2.3/T1.4-lifecycle) depend on T1.3 health → health is a small unlock with
large downstream.

## Section 3 — Phased roadmap

Sequential (1+1 can't parallelize). Sprints = 2 weeks.

### Phase 1 — Enforce what already exists (1 sprint)
Stop the bleeding cheaply.
- T1.2: add a blocking `golangci-lint` job (`.golangci.yml`/gosec is written); flip `govulncheck` `continue-on-error:false` after clearing findings. Gate Dependabot group bumps on a boot check (prevents the viper-class break).
- **Exit:** a vuln/SAST finding fails CI (release already gated on tests since v0.7.2).
- **Risk:** existing gosec findings flood the first run — triage/allowlist with comments, don't suppress wholesale.

### Phase 2 — Operability floor (1–2 sprints)
The box can say if it's alive.
- T1.3 `/healthz` + `/readyz` in every module (wire via `pkg/foundation`/`pkg/lifecycle`). Defer `/metrics` to Phase 5.
- T1.9: the 6–8 runbooks that actually recur (gateway won't start, app stuck installing, disk full, lockout, restore, upgrade rollback).
- **Exit:** updater preflight probes readiness; user recovers top scenarios solo.
- **Risk:** scope-creep into full metrics — resist; healthz only.

### Phase 3 — Testability backbone (3–5 sprints, long pole)
Refactor stops being roulette.
- T1.1 toward 60% on the mutation-heavy modules (app-management, gateway, core) via the +10pp cadence (agent generates table/adversarial tests).
- T1.4: extend `browser-e2e-real` to start/restart/stop.
- T1.8: finish logger migration (33→0 on `common/utils/logger`) — mechanical, agent-suited.
- **Exit:** ≥60% on the three hot modules; full lifecycle E2E green; one logger.
- **Risk:** coverage theater — tie tests to adversarial/regression discipline, not the %.

### Phase 4 — Upgrade & supply-chain trust (2–3 sprints)
A buyer can apply updates without fear.
- T2.1: real N→N+1 matrix (install v(N) → upgrade → assert apps/data/config survive), built on Phase-3 E2E.
- T2.6: cosign-sign tarballs (keyless/OIDC) + `syft` SBOM as release assets.
- T2.7: publish the cadence already implicitly run.
- **Exit:** signed artifacts + SBOM on the release; upgrade test gates the tag.
- **Risk:** cosign key management → use OIDC keyless.

### Phase 5 — Resilience & depth (4–6 sprints, ongoing)
Claims you can defend.
- T2.2 chaos (daemon dies / disk full), T2.3 soak (lifecycle manager), `/metrics`, T3.7 per-arch test (arm64 runner), T3.6 OTel (only after T1.8).
- **Exit:** weekly soak + chaos suite in CI/nightly.
- **Risk:** open-ended — timebox.

**Horizon:** Tier 1 substantially closed ~4–5 months (Phases 1–3). Tier 2 partial
~9–12 months. "Enterprise in 6 months" is **not realistic** for 1+1 — but
"trustworthy for demanding technical individuals/small teams in ~6 months" is.
Calibrate the claim to that, not SOC2.

### Cross-cutting objective — external trust badges

Two third-party, externally-verifiable signals are among the cheapest high-trust
wins for the target profile (technical evaluators read README badges before code).
Tracked by #505 (OpenSSF) / #506 (Go Report). They mostly *package and certify*
the Tier-1 work rather than add new engineering — exactly the leverage a 1+1 team
wants.

- **Go Report Card → A+** (#506): gofmt / go vet / ineffassign / gocyclo /
  misspell / license hygiene. Largely free once the Phase-1 `golangci-lint`
  enforcement (T1.2) lands, plus a gofmt + misspell sweep. **Target: Phase 1.**
- **OpenSSF Best Practices → passing** (#505): a checklist badge bundling
  SECURITY.md (✅ T1.7), disclosure process (✅), automated tests + CI (✅),
  SAST (Phase 1 T1.2), no-known-vulns (Phase 1 — govulncheck blocking), and
  basic project docs. Most criteria are already met or land in Phases 1–3.
  **Target: "passing" by end of Phase 3; "silver" later** once coverage (T1.1)
  and signed releases (T2.6) are in.

Both badges go in the README once earned — they convert the internal Tier-1
work into a claim an outside evaluator can verify in five seconds.

## Section 4 — Consciously NOT doing (12-month horizon)

- **T3.1 bus factor** — solo owner can't fix without a co-maintainer; it's a people problem. Genuine **veto for regulated B2B sale** → don't chase that buyer. Document the risk honestly.
- **T3.8 compliance docs** — waste with no regulated client; write when a deal needs it.
- **T3.5 bug bounty** — needs traffic + triage budget. SECURITY.md channel is the right level now.
- **T2.4 MCP fuzzing** — MCP doesn't exist (ADR-0034 proposed). Build it first.
- **T3.3 conformance tests** — no fork ecosystem.
- **T3.6 OTel** — defer behind T1.8; tracing on a half-migrated logger is rework. Low value for single-node.

## Section 5 — Disagreements with the framework

1. **T1.3 conflates two things.** `/healthz`+`/readyz` are Tier-1 (updater + user need liveness). Prometheus `/metrics` is **Tier 2** for a single-node homelab OS — nobody federates Prometheus against a NAS. Split them.
2. **T1.2 over-bundles.** govulncheck (deps) / gosec (SAST) / golangci (style) have different signal/noise. Make govulncheck + gosec blocking; keep noisier linters advisory. Bundling invites suppression.
3. **Flat 60% is the wrong shape.** Docker-coupled code gives low unit-coverage ROI. ADR-0040's proportional/per-surface ceiling is *more correct*. Restate T1.1 as "≥60% on mutation-heavy pure-logic surfaces".
4. **Missing item: fresh-install/boot smoke gate.** The highest-ROI gate for *this* project's recurring "compiles green, breaks on real install" class (viper, SSE v0.6.13, audit Sprint 16). Already started (`browser-e2e-real` + boot-smoke issue) — deserves first-class Tier-1 status.
5. **T2.7 "monthly" too prescriptive.** For 1+1, *predictable* > *frequent*: "release when the gate is green, ≥ every 6 weeks" beats a forced monthly that ships half-baked.

## Section 6 — What changes at 1 human + 1 agent

**Scales well:**
- **Gates over process.** With no QA team, CI is the reviewer. Every lesson must become a *mechanical* gate (the release gate + `browser-e2e-real` are exactly right). Tribal knowledge doesn't survive a solo team's bad week.
- **Agent on mechanical breadth:** test generation (table/adversarial), the logger migration (33 files, pure mechanical), runbook drafts, SBOM/cosign wiring, CI-failure triage.
- **ADRs as bus-factor mitigation.** 43 ADRs is the real asset — how a future co-maintainer (or a cold agent) reconstructs intent. Partially offsets T3.1.

**Scales badly:**
- Sustained human vigilance (soak triage, bug-bounty intake, perf babysitting). Automate detection; accept slow response.
- **The agent is weakest where this project gets bitten:** environment-fidelity judgment — it greens a mock and misses the real-install break. Point it at *real-backend* validation; the human owns "does it actually boot/install".
- **Agent reviewing agent's PRs** is a blind spot. Concentrate the human's scarce attention on irreversible/blast-radius changes (release cuts, auth, gateway, migrations), not green mechanical PRs.

**Net:** the 1+1 shape makes **Phase 1 + the boot-smoke gate disproportionately
valuable** — they convert "owner must remember" into "CI refuses". Spend the
human's judgment on blast-radius; spend the agent on breadth-with-a-real-backend-check.
