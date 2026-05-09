# 0015 — `pkg/tracing` propagates correlation IDs via `X-Request-Id`

**Status:** accepted
**Date:** 2026-05-08
**Tags:** observability, foundation, v0.4.0

## Context

Bug #64 (gateway SIGSEGV) was hard to triage because a panic in one
service could not be tied back to the user request that triggered it,
nor to log lines emitted by other services along the same request
path. PowerLab's architecture has six backend services that
collaborate over UDS/HTTP: a single user action (install an app,
mount a USB drive, save a file) typically touches three or four of
them. Without a propagated correlation ID, every grep starts blind.

`pkg/logging` (#68) already auto-injects a `correlation_id` attribute
into every log record when a value is found at
`logging.CorrelationIDKey{}` in the request context. What is missing
is the machinery that **places** the value there at request entry and
**propagates** it on every outbound call.

## Decision

`pkg/tracing` provides four primitives:

```go
// Generation
func NewID() string

// Context helpers
func WithID(ctx context.Context, id string) context.Context
func FromContext(ctx context.Context) string

// HTTP middleware — generates on entry, accepts upstream X-Request-Id
func Middleware(next http.Handler) http.Handler

// HTTP client helper — injects ID from context onto outbound requests
func InjectHeader(req *http.Request, ctx context.Context)
```

### ID format

**32 hex characters** (16 random bytes from `crypto/rand`, hex-encoded).

- Wide enough to make collisions practically impossible (2^128 space).
- Compact enough to fit comfortably in log lines and HTTP headers.
- All-hex stays URL/header-safe with zero escaping.
- No external dependency — `crypto/rand` and `encoding/hex` are
  stdlib.

Sortable IDs (ULID, KSUID, Snowflake) were considered but rejected:
correlation IDs are grep keys, not sort keys, and the logger already
emits a timestamp on every record.

### Header

**`X-Request-Id`**.

The most widely used unofficial standard: HAProxy, Traefik, nginx
(via `ngx_http_v2_module`), Heroku, and most Go middleware libraries
use this name. Setting the same header makes PowerLab's correlation
IDs visible to upstream proxies and downstream observability tools
without translation.

`traceparent` (W3C Trace Context) was considered. Rejected for now —
it carries more semantics than we need (sampling, parent span,
flags), and adopting it half-way is worse than not adopting it.
A future sprint can layer W3C tracing on top once the basic
propagation is bedded in.

### Storage key

Same key as `pkg/logging`: `logging.CorrelationIDKey{}`. `pkg/tracing`
imports the type from `pkg/logging`. We do not introduce a parallel
key — there is exactly one correlation ID per context.

(The other direction — moving the key from `pkg/logging` to
`pkg/tracing` — was considered. Rejected because `pkg/logging` was
written first and other services already import the key from there;
moving it now would break in-flight PRs for no real benefit. The
key's home is a one-line refactor whenever convenient.)

## Rationale

- **Generation at entry, propagation everywhere.** The gateway's
  `Middleware` is the only place IDs are minted; every internal
  service either inherits an ID via `Middleware` (when it accepts
  HTTP from the gateway) or via `InjectHeader` (when it makes
  outbound calls to peers). One source of truth, no
  re-generation-mid-flow surprises.
- **Header on the wire matches key in the context.** A request that
  carries `X-Request-Id: abc123` produces `correlation_id=abc123` in
  every log line, every error response (`pkg/errors.WriteHTTP`
  already wires this), and every outbound call's
  `X-Request-Id`. Symmetric and predictable.
- **No external dep.** Continues the Sprint-1 discipline of building
  on the stdlib. Hex from `crypto/rand` is boring; boring is the
  goal for foundation packages.
- **Accepting upstream `X-Request-Id`.** When PowerLab is fronted by
  an external proxy (Cloudflare, Tailscale Funnel, a future
  enterprise reverse proxy), that proxy's correlation ID is what the
  user / oncall already has in their tooling. Replacing it with a
  fresh PowerLab ID would scatter the trace; reusing it stitches the
  two halves together.

## Consequences

- The gateway rewrite (#73) wraps its handler chain in
  `tracing.Middleware` as the **outermost** layer, before
  `lifecycle.RecoverMiddleware` so even panics carry the correlation
  ID into the logged stack trace.
- Every PowerLab-owned `http.Client` call site uses
  `tracing.InjectHeader(req, ctx)` before `client.Do(req)`. CI lint
  can later flag bare `client.Do` without injection (separate
  issue).
- Logs across the six services for one user action are now joinable
  by `correlation_id=…`. Bug reports include a 32-character hex
  string; that becomes the search key during triage.
- `pkg/errors.WriteHTTP` (#69) already serializes the correlation
  ID into every error body — **the user can quote the ID from the
  toast they saw**, and we grep right to it.

## Alternatives considered

- **UUID v4 from a library.** Rejected — extra dep for a marginal
  improvement (UUIDs are slightly more recognizable but offer no
  functional advantage over plain hex).
- **ULID / KSUID for sortable IDs.** Rejected — not worth the
  complexity for a foundation package; sortability is unused
  in our triage workflow.
- **W3C `traceparent` with full Trace Context spec.** Rejected for
  Sprint 1 (over-scope). Re-evaluate after the four foundation
  packages and the first two service kills are bedded in.
- **Generate one ID per service.** Rejected — defeats the entire
  purpose. The whole point is one ID per user action.

## Reference

- Umbrella roadmap: #67
- Issue: #71 (`pkg/tracing`)
- Companion ADRs: 0011 (backend/pkg/ coexistence),
  0012 (pkg/logging), 0013 (pkg/errors), 0014 (pkg/lifecycle)
- Header convention reference (de facto):
  https://http.dev/x-request-id
