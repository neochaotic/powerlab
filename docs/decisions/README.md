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
