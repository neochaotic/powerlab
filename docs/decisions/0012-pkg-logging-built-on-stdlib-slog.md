# 0012 — `pkg/logging` is built on `log/slog` (stdlib)

**Status:** accepted
**Date:** 2026-05-08
**Tags:** observability, foundation, v0.4.0

## Context

Every backend service currently logs via `go.uber.org/zap`, transitively
inherited from CasaOS. Logger construction is ad hoc per service: there
is no shared package, no shared level configuration, no shared format
selection, and no correlation between log lines emitted by different
services for the same user request. Bug #64 (gateway `checkURL`
SIGSEGV) was hard to triage because the panic logged in one service
could not be correlated to the originating request in another.

For the CasaOS strip (umbrella #67), we are creating PowerLab-owned
foundation packages under `backend/pkg/` (per ADR-0011). Logging is
the first foundation package because `pkg/errors`, `pkg/lifecycle`,
and `pkg/tracing` all depend on it.

The choice is which logging library underpins `pkg/logging`.

## Decision

`pkg/logging` exposes a small interface and is implemented on top of
**`log/slog`** from the Go standard library. No external logging
dependency.

The package provides:

- `Logger` interface with `Debug / Info / Warn / Error` methods, each
  taking `context.Context` first, a message string, and structured
  `slog.Attr` attributes.
- A `Config` struct (level, format) populated from environment
  variables (`POWERLAB_LOG_LEVEL`, `POWERLAB_LOG_FORMAT`).
- A custom `slog.Handler` that, on every record, pulls the correlation
  ID from `context.Context` (via `pkg/tracing`, when present) and
  injects it as a `correlation_id` attribute. Services therefore log
  correlation IDs for free without remembering to pass them.
- `Format` choices: `console` (human-readable, dev default) and
  `json` (machine-readable, prod default). `console` runs through
  `slog.NewTextHandler` configured for readability; `json` runs
  through `slog.NewJSONHandler`.
- Levels:
  - `Debug` — verbose internal state; off by default in prod.
  - `Info` — operational events worth logging on a healthy system.
  - `Warn` — recoverable anomalies worth attention.
  - `Error` — failures that affected a request or the service.
  - There is **no** `Fatal` method — services do not own the
    decision to terminate the process; that is `pkg/lifecycle`'s
    responsibility, which logs via `Error` and triggers shutdown
    through the lifecycle manager.

## Rationale

- **Standard library means zero supply-chain surface.** No version
  bumps to chase, no abandonment risk, no compatibility breaks. This
  matters disproportionately for a project explicitly stripping a
  vendor dependency (CasaOS) — adding a new third-party logger would
  be a step in the wrong direction.
- **`log/slog` is mature.** Stable since Go 1.21 (mid-2023); the
  Go ecosystem (Kubernetes, the standard library itself,
  fx, gRPC, et al.) has converged on it as the structured-logging
  contract. By picking `slog` we align with where everything else
  is going.
- **Performance is sufficient.** `slog` is not the fastest structured
  logger in Go — `zap` and `zerolog` retain measurable advantages on
  serialization throughput. But PowerLab's logging volume is not in
  the hot path that benchmarks capture: a self-hosted home server
  emits hundreds to thousands of log lines per second under heavy
  use, not millions. Within that envelope, `slog`'s overhead is
  invisible.
- **Custom Handler covers the gap.** Anything `zap` offers that
  `slog` does not (e.g. sampling, advanced field types) can be
  composed via a custom handler. We start with the stdlib handlers
  and extend only when a concrete need appears.
- **Migration path stays open.** If we ever need `zap` performance
  for a specific subsystem, we can wrap a `zap.Logger` behind a
  `slog.Handler` adapter (the official `go.uber.org/zap/exp/zapslog`
  package does exactly this) without changing call sites in PowerLab
  code. The interface stays `slog`-shaped; the engine swaps.

## Consequences

- All new code in `backend/pkg/` and in services rewritten under
  the CasaOS strip imports `backend/pkg/logging` only. `zap` does
  not appear in PowerLab-owned modules.
- Existing services still using `zap` (via `backend/common/`) continue
  to do so until they are killed. We do not retrofit them.
- The `zap` dependency is removed from a service's `go.mod` as part
  of that service's kill PR (e.g., #73 gateway rewrite).
- The custom Handler that injects correlation IDs depends on
  `pkg/tracing`. Until `pkg/tracing` lands (issue #71), the Handler
  treats a missing correlation ID as `correlation_id=""`. This
  enables logging to ship before tracing without coupling.

## Alternatives considered

- **`go.uber.org/zap` directly.** Rejected — see Rationale above.
  The performance argument does not hold for our envelope, the
  supply-chain argument cuts the other way, and `slog` is the
  ecosystem direction.
- **`github.com/rs/zerolog`.** Rejected for the same reasons as
  `zap`, plus its API ergonomics are further from the rest of Go.
- **No abstraction — services use `slog` directly.** Rejected
  because we would lose the centralized correlation-ID injection,
  the centralized config, and the centralized format/level
  selection. The package is a thin layer (likely <200 LOC) but
  buys real consistency.

## Reference

- Umbrella roadmap: #67
- Issue: #68 (`pkg/logging`)
- Companion ADR: 0011 (backend/pkg/ coexistence with backend/common/)
- Go `log/slog` documentation:
  https://pkg.go.dev/log/slog
- `zapslog` adapter, in case we ever swap engines:
  https://pkg.go.dev/go.uber.org/zap/exp/zapslog
