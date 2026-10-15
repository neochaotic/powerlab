# Test-coverage assessment through the Kubernetes testing taxonomy — 2026-05-21

**Scope:** PowerLab's test landscape mapped against K8s's 12-layer testing model,
grounded in `main` at v0.7.2 (verified counts, not assumed). Companion to
[enterprise-maturity-2026-05-21.md](./enterprise-maturity-2026-05-21.md).

**Framing (this changes everything):** K8s's 12 layers exist for a *distributed
control plane* certified by CNCF at 5000-node scale. PowerLab is a *single-node
app orchestrator* on a homelab box. Three layers (scale, conformance, node-
conformance) **do not map** — applying them literally is cargo-culting. The
honest translation keeps the layers that catch PowerLab's actual recurring
failure class ("compiles green, breaks on real install/runtime" — viper boot
panic, SSE v0.6.13, audit Sprint 16) and reframes the rest.

## Layer-by-layer diagnosis

| # | Layer | Status | Evidence | Fit |
|---|---|---|---|---|
| 1 | Unit | 🟡 PARTIAL | 132 backend `_test.go`; **~33% aggregate** (ADR-0040). Pockets: `common/utils/jwt` ~90.8%, `app-management/service` ~35%. Frontend: 68 vitest files. K8s targets 75–85%. | Maps. |
| 2 | Integration (in-process) | 🟡 PARTIAL | **2** `//go:build integration` files (`app-management/service/docker_integration_test.go` + catalog). Real `docker compose up` (no testcontainers — deliberate, #150). CI `backend-integration` covers **app-management only**. | Maps; under-built. |
| 3 | E2E (real stack) | 🟡 PARTIAL | 18 Playwright specs: mock UI + real-backend `@smoke` (env-guarded) + **`browser-e2e-real`** (install→SSE phases→uninstall vs real stack) now a **release gate**. | Maps. |
| 3a | Conformance (distro cert) | 🔴 N/A | Nearest analogue: store↔core "amarração" contract (validate.py + bundle-compat + catalog gates) — a *compatibility* contract, not distro conformance. | **Doesn't map** (single product, no forks). |
| 3b | NodeConformance | 🔴 N/A | — | **Doesn't map.** |
| 3c | Disruptive | 🟡 PARTIAL | Gateway bounded-shutdown test (SSE-doesn't-hang-restart) + restart `@smoke`. No systematic kill→recover. | Maps; barely started. High value (systemd `Restart=always` masks bugs — cf. the message-bus boot-order issue). |
| 4 | Upgrade (N→N+1) | 🟡 PARTIAL | `test-upgrade-resolves-stale-legacy_test.sh`, `check-install-upgrade-clean_test.sh`, #303. Shell-level; not a versioned install→upgrade→assert-state matrix. | Maps; **critical** for enterprise. |
| 5 | Scale (5000 nodes) | 🔴 N/A (reframe) | Node-scale meaningless single-node. Real analogue: **N apps installed** (50+) — does launchpad/list/SSE degrade? No test. | **Reframe**, don't drop. |
| 6 | Performance (bench + gate) | 🔴 ABSENT | **0** `func Benchmark`. | Maps; low priority (install latency = Docker pull, not our code). |
| 7 | Soak (days under load) | 🔴 ABSENT | None. `pkg/lifecycle` makes leak detection tractable but unused. | Maps; **high value** (homelab uptime = months → leaks bite). |
| 8 | Chaos (kill/partition/fill) | 🔴 ABSENT | None. Real scenarios: docker daemon dies, disk full, gateway killed→recovers. | Maps; high value, cheap to start. |
| 9 | Security / fuzzing | 🟡 PARTIAL | **0** `func Fuzz`, no go-fuzz/OSS-Fuzz. But hand-written **adversarial unit tests** exist (bind-mount containment, `stripPort` IPv6, secret-regex, `isSystemPath`). `gosec` configured, **not run in CI**. | Maps; fuzzing premature until MCP (ADR-0034) — that's the real fuzz surface. |
| 10 | SAST (static, blocking) | 🔴 ABSENT (as a gate) | `govulncheck` `continue-on-error:true` (warn-only); `.golangci.yml` has **23 linters** incl. gosec/staticcheck/govet but **no `golangci-lint` CI job**. `backend-deadcode` *is* enforced. | Maps; **cheapest high-impact fix**. |
| 11 | Dependency scanning | 🟡 PARTIAL | Dependabot (9 ecosystems) + govulncheck matrix. Warn-only, **no boot gate** (viper-1.21 entered here). | Maps; gap is *enforcement + boot gate* (#515). |
| 12 | Lint / style | 🟡 PARTIAL | gofmt/govet/staticcheck/revive in `.golangci.yml` **not CI-wired**; deadcode enforced; frontend ESLint flat config (bans raw `fetch()`). | Maps. |

## The real shape

PowerLab has an **hourglass, not a pyramid**: a thin-but-real top (the
`browser-e2e-real` release gate + real-backend `@smoke` — genuinely good and rare
at this size), a *narrow* middle (2 integration files, 1 module), a *thin* base
(~33% unit). The recurring "green-but-broken-on-real-install" failures come from
the thin base + narrow middle: unit/mocks pass, the real-wiring layer isn't
exercised.

The correct target is **not "be K8s."** It's: thicken the base (unit→60% on
mutation-heavy pure logic), widen the middle (integration into gateway/core/
user-service, not just app-management), *keep* the strong top. Scale/conformance/
node-conformance stay out.

## Priorities (folded into the maturity roadmap phases)

1. **Wire + flip SAST/lint (Layers 10, 12) — Phase 1, effort ≈1.** The 23 linters + gosec + govulncheck exist; add a blocking `golangci-lint` job, flip `govulncheck` to fail. Highest ROI in the whole taxonomy, nearly free.
2. **Dependency boot-gate (Layer 11) — Phase 1** (#515): a bump must boot, not just compile.
3. **Unit base → 60% on hot modules (Layer 1) — Phase 3** (agent-generated table/adversarial tests).
4. **Widen integration (Layer 2) — Phase 3:** replicate `docker_integration_test.go` into gateway/core/user-service.
5. **Upgrade matrix (Layer 4) — Phase 4:** install-v(N)→upgrade→assert-state on the E2E harness.
6. **Chaos-lite (Layer 8) + Soak (Layer 7) — Phase 5:** 3 chaos scenarios (daemon dies, disk full, service killed→recovers); soak via `pkg/lifecycle`.
7. **Reframe "scale" (Layer 5):** an E2E installing N=50 apps, assert launchpad/SSE don't degrade.

**Deliberately not:** Layers 3a/3b (no distro to certify), Layer 5 literal node-
scale, Layer 9 fuzzing *until MCP exists* (then first-class — LLM input is the
fuzz surface ADR-0034 will need).
