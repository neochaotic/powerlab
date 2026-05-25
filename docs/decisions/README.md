# Architecture Decision Records (ADRs)

This directory contains the project's Architecture Decision Records — short
documents that capture significant architectural choices, the constraints
behind them, and the alternatives that were rejected.

## Why ADRs

- A six-month-old "why did we pick X?" question has a real answer in
  the repo, not in someone's memory.
- New contributors learn the project's reasoning without sitting through
  hour-long calls.
- The reviewer of a PR can challenge a decision with the original
  rationale on the table.

## Format

One file per decision, numbered sequentially:

```
0001-name-of-the-decision.md
0002-other-decision.md
```

Each file follows the same structure:

```markdown
# NNNN — Short title

**Status:** proposed | accepted | superseded by NNNN
**Date:** YYYY-MM-DD
**Tags:** area, area, area

## Context
What forced the choice? What constraints apply?

## Decision
The single sentence verdict.

## Rationale
Why this choice. Bullet points are fine.

## Alternatives considered
What we rejected and why.

## Consequences
What this commits us to. What it makes harder.

## References
Links to issues, PRs, external docs.
```

## When to write one

- Anything that's load-bearing for a public release
- Anything that touches security, performance, or compatibility
- Anything that someone is going to want to revisit in a year

If a decision can be undone in an afternoon, skip the ADR and just
comment the code.

## Index

| # | Title | Status |
|---|---|---|
| [0001](./0001-https-cert-validity-1y-leaf-10y-ca.md) | HTTPS cert validity — 1y leaf, 10y CA | accepted |
| [0002](./0002-pkcs7-library-digitorus.md) | PKCS#7 signing library — `github.com/digitorus/pkcs7` | accepted |
| [0003](./0003-reset-trust-ux-single-confirm.md) | Reset-trust UX — single confirm with device list | accepted |
| [0004](./0004-https-walkthrough-inline-tabs.md) | HTTPS walkthrough — inline 4 tabs (not wizard) | accepted |
| [0005](./0005-pwa-scaffolding-no-cache-yet.md) | PWA scaffolding in v0.2.7 — no-op service worker | accepted |
| [0006](./0006-hsts-after-trust-gate.md) | HSTS gated on first verified non-localhost client | accepted |
| [0007](./0007-internal-network-only-initial-deployment.md) | Initial deployment scope — internal LAN only | accepted |
| [0008](./0008-api-docs-portal-scalar.md) | API docs portal — Scalar, embedded, no spec mutation | accepted |
| [0009](./0009-https-trust-onboarding-pattern.md) | Name the v0.2.7 trust-dance: "HTTPS Trust Onboarding Pattern" | accepted |
| [0010](./0010-ca-storage-decoupled-from-runtime.md) | CA storage decoupled from the runtime data dir | accepted |
| [0011](./0011-ca-mismatch-detection-and-recovery.md) | CA-mismatch detection + browser-side HSTS recovery | accepted |
| [0012](./0012-ca-rotation-flow.md) | CA rotation: separate destructive action from "Reset trust" | accepted |
| [0013](./0013-pkg-errors-typed-error-with-code-i18n-status.md) | `pkg/errors` — typed error with code, i18n key, HTTP status | accepted |
| [0014](./0014-pkg-lifecycle-graceful-shutdown-and-panic-recovery.md) | `pkg/lifecycle` — graceful shutdown + panic recovery | accepted |
| [0015](./0015-pkg-tracing-correlation-id-via-x-request-id-header.md) | `pkg/tracing` — correlation ID via `X-Request-Id` header | accepted |
| [0016](./0016-modular-kill-scope-vs-full-extraction.md) | Modular kill scope vs. full extraction — kill PRs target one service at a time | accepted |
| [0017](./0017-changie-for-changelog-fragments.md) | Changie for changelog fragments | accepted |
| [0018](./0018-goose-for-versioned-migrations.md) | Goose for versioned SQLite migrations | accepted |
| [0019](./0019-tech-debt-tracked-in-audits-adrs-and-issues.md) | Tech debt tracked in audits + ADRs + issues | accepted |
| [0020](./0020-jwt-keypair-persisted-by-default.md) | JWT keypair persisted by default (sessions survive restart) | accepted |
| [0021](./0021-docker-label-namespace-and-appdata-path.md) | Docker label namespace `io.powerlab.v1.*` + AppData path `PowerLabAppData/` | accepted |
| [0022](./0022-casaos-upstream-is-abandoned-no-new-dependencies.md) | CasaOS upstream is abandoned; PowerLab takes no new dependencies | accepted |
| [0023](./0023-socketio-origin-allowlist.md) | SocketIO CheckOrigin allowlist (close #219 CORS bypass) | accepted |
| [0025](./0025-backend-pkg-coexistence-with-casaos-common.md) | `backend/pkg` coexists with `backend/common` during the strangler kill series (renumbered from 0011) | accepted |
| [0026](./0026-pkg-logging-built-on-stdlib-slog.md) | `pkg/logging` built on `log/slog` (not zap, not zerolog; renumbered from 0012) | accepted |
| [0027](./0027-uber-fx-for-gateway-di.md) | Uber `fx` is the DI framework for the gateway only | accepted |
| [0028](./0028-echo-v4-http-framework.md) | Echo v4 as the universal HTTP framework | accepted |
| [0029](./0029-gorm-as-orm.md) | GORM as the ORM; AutoMigrate is explicitly forbidden | accepted |
| [0030](./0030-svelte-5-runes-lock-in.md) | Svelte 5 Runes lock-in; no Svelte 4 stores permitted | accepted |
| [0031](./0031-oapi-codegen-for-openapi.md) | `oapi-codegen` for OpenAPI to Go; codegen output is gitignored | accepted |
| [0032](./0032-mdns-strategy.md) | mDNS Strategy: Avahi preferred, direct multicast fallback | accepted |
| [0033](./0033-audit-middleware-design.md) | Audit middleware: per-service SQLite, async writer, 30d/50MB retention | proposed |
| [0034](./0034-standalone-observability-mcp-service.md) | Standalone observability + MCP service (independent runtime, port :9090, MCP-first) | proposed |
| [0035](./0035-audit-storage-jsonl.md) | Audit storage — JSONL + in-memory ring buffer (supersedes ADR-0033 storage section) | accepted |
| [0043](./0043-embed-frontend-into-gateway-binary.md) | Embed the static frontend into the gateway binary via `go:embed`; `-w` retained as dev override | proposed |

> **Renumber resolved 2026-05-11.** ADR-0011 and ADR-0012 originally each had **two** files because the CA series (0010–0012) and the foundation `backend/pkg/` series (originally 0011–0015) were filed in parallel branches on 2026-05-07 and 2026-05-08. The foundation pair was renumbered to **0025** (`backend-pkg-coexistence`) and **0026** (`pkg-logging`) to break the ambiguity. Historical refs to "ADR-0011 (strangler)" / "ADR-0011 (pkg coexistence)" now point at 0025; refs to "ADR-0012 (logging)" / "ADR-0012 (slog)" point at 0026. CA-context refs to 0011/0012 are unchanged. Each renumbered file carries a `Renumber history` note at the top.
