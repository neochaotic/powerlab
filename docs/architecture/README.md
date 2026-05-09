# PowerLab architecture

This directory hosts the high-level architecture documentation. Every
diagram is **Mermaid** so it renders directly on GitHub and stays
versioned with the code.

## Documents

- **[Service topology](./topology.md)** — what runs where, how services
  find each other, who boots first
- **[Dependency graph](./dependency-graph.md)** — Go-module-level
  dependencies, plus the strangler progress against `backend/common/`
- **[Request lifecycle](./request-lifecycle.md)** — how an HTTP request
  travels through the gateway → service → response, and how the
  correlation ID propagates
- **[Foundation interfaces](./foundation-interfaces.md)** — the
  `pkg/logging`, `pkg/errors`, `pkg/lifecycle`, `pkg/tracing` contracts
  and how they compose
- **[Data persistence](./data-persistence.md)** — every path PowerLab
  writes to on disk, what owns it, how upgrades preserve it
- **[CasaOS strangler progress](./casaos-strangler.md)** — live status
  of the migration off `backend/common/` (CasaOS-Common); updated each
  sprint

## Related references

- **API reference** — Scalar portal at `/docs` (running gateway).
  Aggregates all 6 services' OpenAPI specs interactively. See
  ADR-0008.
- **ADRs** — `docs/decisions/` for the reasoning behind any
  architectural choice. Each diagram here cross-links to the relevant
  ADR.
- **Audits** — `docs/audits/` for the snapshot of dependencies and
  dead code at the start of the CasaOS strip.

## When to update these docs

Each kill PR (Sprint 1-4) updates the relevant diagrams:

- **Service rewrite** → update `topology.md` (boot order, deps changed?)
- **Module rebrand** → update `dependency-graph.md` and
  `casaos-strangler.md`
- **New foundation pkg** → update `foundation-interfaces.md`
- **New on-disk path** → update `data-persistence.md`

## Why Mermaid (not PlantUML / Excalidraw / etc.)

- Renders natively on GitHub — no external service, no broken links
- Lives in markdown — same review flow as code, same diff
- ASCII source — `git blame`, `git log`, code review all work
- Compatible with `mkdocs-material` if/when we ship a docs site
- Zero install for contributors
