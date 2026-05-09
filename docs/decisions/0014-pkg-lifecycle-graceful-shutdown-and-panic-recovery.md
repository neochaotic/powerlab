# 0014 — `pkg/lifecycle` provides graceful shutdown and panic recovery

**Status:** accepted
**Date:** 2026-05-08
**Tags:** reliability, foundation, observability, v0.4.0

## Context

Two failure modes from the v0.3.x line motivate this package:

- **Process death from a single bad request.** Bug #64 — gateway
  `checkURL` dereferenced a nil response → SIGSEGV → the whole
  gateway process exited → systemd restart loop → user-visible
  outage. Every Go server that hosts user-defined or vendor-shipped
  handler code is one nil-deref away from the same fate without a
  recovery middleware.
- **Abrupt shutdown losing in-flight requests.** When systemd sends
  SIGTERM during a deploy, the current services exit immediately,
  closing TCP sockets mid-response. Browsers see `ERR_EMPTY_RESPONSE`
  during upgrades. There is no per-service or system-wide convention
  for "stop accepting new traffic, drain in-flight, then exit."

For the CasaOS strip (umbrella #67), `pkg/lifecycle` is the third
foundation package after `pkg/logging` and `pkg/errors`. Every
service rewritten in the strip uses it; the gateway rewrite (#73)
specifically depends on it to render bug-#64-class panics as logged
500 responses instead of crashes.

## Decision

`pkg/lifecycle` provides three primitives:

### 1. `Manager` — graceful shutdown coordinator

```go
m := lifecycle.New(logger)
m.RegisterShutdown("http-server", func(ctx context.Context) error { ... })
m.RegisterShutdown("db-pool", func(ctx context.Context) error { ... })
err := m.Run(ctx, 30*time.Second) // blocks until SIGTERM/SIGINT
```

- Hooks run in **reverse-init order (LIFO)** on shutdown so newer
  components stop before older ones they depend on.
- Each hook receives a deadline-bounded context so a misbehaving
  hook cannot block shutdown indefinitely.
- The first non-nil error from any hook is returned; subsequent hook
  errors are logged but not returned (no error-only-from-last-hook
  surprise).
- A separate `Shutdown(ctx)` method runs the same hooks without
  waiting on signals — used by tests and by callers that want to
  trigger shutdown in response to a non-signal event.

### 2. `RecoverMiddleware` — HTTP panic recovery

```go
mux := http.NewServeMux()
// ... register handlers ...
handler := lifecycle.RecoverMiddleware(logger)(mux)
```

- Wraps any `http.Handler`. On panic in the handler chain it:
  - Logs the panic value, the stack trace, and the request method/path
    at error level (structured, with correlation ID from context).
  - Writes a 500 response via `errors.WriteHTTP` with `ErrInternal`
    so the body is the same JSON shape as any other error.
  - Process keeps running.
- Closes the bug-#64 class entirely: a nil-deref in a handler is now
  a logged 500, not a process death.

### 3. `SafeGo` — goroutine panic recovery

```go
lifecycle.SafeGo(logger, func() {
    // background work — a panic here does not crash the process
})
```

- Replaces the `go fn()` pattern in code that should not crash on
  panic (timer ticks, background workers, retry loops).
- Inside the goroutine, recovers from any panic, logs it with the
  stack trace, and lets the goroutine exit cleanly.

## Rationale

- **Recovery is non-negotiable for a Go server hosting third-party
  code paths.** PowerLab handles user-supplied compose YAMLs, runs
  generated code from CasaOS upstream, and exposes endpoints that
  touch syscalls (mount, fuse, network). Any of these can panic. A
  recovery middleware is the difference between "logged 500 for the
  affected request" and "outage for everyone."
- **LIFO shutdown order matches dependency direction.** If A
  registers first and B registers later, B was likely built on top of
  A. Shutting B down first lets A still serve B's drain. This is the
  same convention `defer` uses, deliberately.
- **Hook context with deadline.** Without bounding each hook's
  runtime, a single broken `Close` can deadlock the whole shutdown.
  The bound is a cooperative timeout: hooks should respect ctx, but
  a misbehaving hook only delays itself, not the rest.
- **Three primitives, not one God-object.** Keeping `Manager`,
  `RecoverMiddleware`, and `SafeGo` as separate exports means a
  CLI tool that needs `SafeGo` does not pull in HTTP machinery, and
  a handler-only test does not need to instantiate a `Manager`.

## Consequences

- All HTTP handlers in PowerLab-owned code are wrapped by
  `RecoverMiddleware`. The gateway rewrite (#73) wires it
  unconditionally at the chain's outermost layer.
- Background goroutines launched in PowerLab-owned code use
  `SafeGo` instead of `go`. CI lint will flag bare `go` after Sprint
  1 (separate issue). Test code is exempt.
- Services declare every long-lived resource (HTTP server, DB pool,
  message-bus subscription) via `RegisterShutdown` so they all drain
  on SIGTERM.
- The 500 body emitted by recovery is the same shape as any other
  error response — UI clients do not need a special panic path.

## Alternatives considered

- **Use `runtime.SetPanicOnFault` / process-level recover.**
  Rejected — only catches a narrow class of faults, brittle, hard
  to get right. Per-handler recovery is the conventional approach.
- **Use a third-party shutdown library (e.g. `tomb.v2`,
  `lifecycle/v2`).** Rejected — adding a dependency to ship 80 lines
  of glue is the wrong direction for a sprint explicitly removing
  vendor weight.
- **Don't wrap goroutines (only HTTP).** Rejected — `pkg/tracing`,
  `pkg/lifecycle` itself, and several anticipated services launch
  background goroutines (timers, watchers). Leaving them
  unrecovered keeps the process-death class only partially closed.
- **Single-shot Run with no separate Shutdown.** Rejected — testing
  Run requires sending real signals to the test process, which is
  brittle in CI. Splitting into Run (signal-driven) and Shutdown
  (deterministic) cleanly separates the OS coupling from the hook
  semantics.

## Reference

- Umbrella roadmap: #67
- Issue: #70 (`pkg/lifecycle`)
- Companion ADRs: 0011 (backend/pkg/ coexistence),
  0012 (pkg/logging), 0013 (pkg/errors)
- Bug closed by adoption: #64 (gateway SIGSEGV) — once the gateway
  rewrite (#73) wraps its handlers in `RecoverMiddleware`, the same
  inverted-condition + nil-deref combination produces a logged 500
  instead of a process crash.
