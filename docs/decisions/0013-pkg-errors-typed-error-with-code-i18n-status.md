# 0013 — `pkg/errors` is a typed error with code, i18n key, and HTTP status

**Status:** accepted
**Date:** 2026-05-08
**Tags:** errors, foundation, observability, v0.4.0

## Context

The PowerLab codebase inherited from CasaOS a pattern of returning bare
errors with stringly-typed messages:

```go
return errors.New("there are ports in use")
```

The handler then renders that string in a generic 500 (or sometimes a
400) JSON body. The frontend cannot translate it. Logs cannot
correlate "ports in use" to a specific port. Code reviewers cannot
search for every site that returns "ports in use" because string
literals drift over time. Three concrete failure modes from this
pattern surfaced in the v0.3.2 stabilization sprint:

- **#50** — Settings → Security CA download lands on a plain-text
  error page because the handler returned `http.Error(w, msg, status)`
  without enough metadata for the UI to know what to do with it.
- **#65** — Custom App redeploy returns "there are ports in use",
  the UI surfaces the literal English string in a Portuguese locale,
  the user has no actionable information.
- **#64** — Gateway crashed with SIGSEGV; the panic emitted an
  unstructured trace, no error code, no correlation, no way to
  programmatically retry-with-backoff or surface a user-friendly UI
  state.

For the CasaOS strip (umbrella #67), `pkg/errors` is the second
foundation package after `pkg/logging`. Every service rewritten in
the strip emits errors of this typed shape; the gateway middleware
serializes them to HTTP and the UI client deserializes them back.

## Decision

Define a single concrete `Error` type carrying:

- **`Code`** — a stable, dot-separated, machine-readable identifier
  (e.g. `ports.conflict`, `auth.invalid_token`, `common.not_found`).
  Code is the search key when grepping logs and the dispatch key in
  the UI.
- **`I18nKey`** — the translation key the UI uses to render the
  user-facing message (e.g. `errors.ports_in_use`,
  `errors.invalid_token`). Decoupled from `Code` so a single Code
  can map to multiple messages over time without breaking log
  searches.
- **`HTTPStatus`** — the HTTP status the handler should emit when
  the error reaches the response. Catalog errors set this once;
  callers do not pass it.
- **`Cause`** — the underlying error, preserved through the chain.
  Standard `errors.Is` and `errors.As` work against `*Error` via the
  `Unwrap()` method.
- **`Fields`** — a structured `map[string]any` of incident details
  (e.g. `{"port": 8080}`, `{"path": "/foo"}`). The handler emits
  these as a `details` object on the response; logs include them
  as structured attributes.

Provide:

- **`New(code, i18nKey string, status int) *Error`** — constructs
  a fresh error.
- **`Wrap(err error, code, i18nKey string, status int) *Error`** —
  wraps an existing error, preserving the chain.
- **`WithField(key string, value any) *Error`** —
  **`WithFields(map[string]any) *Error`** — both **immutable**:
  return a new `*Error` with the field added; the original is not
  mutated. Catalog errors therefore stay pristine.
- **A catalog** of common HTTP-shaped errors:
  `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`,
  `ErrConflict`, `ErrTooManyRequests`, `ErrInternal`,
  `ErrServiceUnavailable`. Domain-specific catalogs (e.g.
  `ErrPortsConflict`) live in the service or domain package, not in
  `pkg/errors` — `pkg/errors` carries only the universal HTTP-tier
  errors.
- **`WriteHTTP(ctx, w, err) (int, error)`** — handler-side helper.
  Inspects `err`, extracts the `*Error` (via `errors.As`, so it
  works through any wrapping), sets `Content-Type: application/json`
  and the appropriate status, writes the body in a stable shape:

  ```json
  {
    "code": "ports.conflict",
    "i18n_key": "errors.ports_in_use",
    "correlation_id": "req-abc-123",
    "details": { "port": 8080 }
  }
  ```

  When `err` is not an `*Error`, the helper falls back to
  `ErrInternal` and logs the original at error level so it is not
  silently lost.

## Rationale

- **Code separation from i18n key.** The Code is a permanent identifier
  tied to *what happened*; the i18n key is tied to *how we phrase it*.
  Decoupling means the UI team can refine wording without breaking
  every grep alert that relies on a stable Code.
- **Immutable `WithField`.** A catalog like
  `errors.ErrPortsConflict.WithField("port", 8080)` must not mutate
  the package-level singleton — concurrent callers would race on it.
  Returning a fresh `*Error` is the obvious-correct choice.
- **`errors.Is` / `errors.As` compatibility.** The standard library
  semantics are well understood. Reusing them via `Unwrap()` means
  callers do not need to learn a PowerLab-specific predicate.
- **`Fields` as `map[string]any`.** Avoids inventing a typed
  field-by-field DSL. The handler serializes into JSON; the logger
  serializes into `slog.Attr`s. The map is the lingua franca.
- **`WriteHTTP` over manual response writing.** Centralizes the
  response shape (and Content-Type, and status) so every service
  emits identical JSON. Closes #50 by ensuring no handler can ever
  again accidentally emit a plain-text error.
- **Universal-only catalog.** Putting `ErrPortsConflict` in `pkg/errors`
  would couple a foundation package to a domain (compose
  orchestration). Domain-specific catalogs belong with the domain;
  foundation only ships the HTTP-tier ones every service needs.

## Consequences

- New backend code in `backend/pkg/` and rewritten services emit
  `*Error`. Reviewers reject any new `errors.New("...")` from
  PowerLab-owned code.
- The gateway rewrite (#73) wires `WriteHTTP` into the handler chain
  for all error paths — closes #50.
- The UI client must learn the new error shape: parse `code`,
  translate `i18n_key`, surface `details` in toasts. Frontend work
  is tracked separately.
- `errors.New` from the standard library is still acceptable in tests
  and inside helpers where no caller needs to inspect — internal
  details only. CI lint can later enforce "no `errors.New` outside
  test files" if drift becomes a problem.

## Alternatives considered

- **`pkg/errors` (the popular Davec package, deprecated in favor of
  Go 1.13+ wrapping).** Rejected — its main feature (stack traces)
  duplicates panic recovery in `pkg/lifecycle`, and we do not want
  the dep.
- **`google.golang.org/grpc/codes`-style enum for `Code`.** Rejected
  — string-based codes are more readable in logs and easier to
  evolve. The cost (no exhaustiveness check) is acceptable for our
  scale.
- **Just use `fmt.Errorf("…: %w", err)` everywhere.** Rejected — the
  whole point is to add metadata that wrapping alone cannot carry
  (HTTP status, i18n key, structured fields). `%w` chains are still
  used inside `Wrap` for the `Unwrap()` semantics.
- **Sentinel errors only (`var ErrPortsConflict = errors.New(...)`)**
  with no struct. Rejected — sentinels carry no incident data;
  callers cannot attach `port: 8080`. The struct-with-immutable-`With`
  pattern subsumes sentinels.

## Reference

- Umbrella roadmap: #67
- Issue: #69 (`pkg/errors`)
- Companion ADRs: 0011 (backend/pkg/ coexistence), 0012 (pkg/logging
  on slog)
- Bugs that this design closes once services adopt it: #50, #65; the
  panic-recovery helper in `pkg/lifecycle` (#70) will use this to
  emit consistent 500 bodies.
