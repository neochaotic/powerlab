# Request lifecycle

How a single HTTP request travels through PowerLab — what handles it
at each layer, where the correlation ID is generated, and how
errors are surfaced.

## Happy path: app install

```mermaid
sequenceDiagram
    autonumber
    participant Browser
    participant Gateway as Gateway<br/>(:8443 HTTPS)
    participant Tracing as pkg/tracing<br/>middleware
    participant Recover as pkg/lifecycle<br/>RecoverMiddleware
    participant App as app-management
    participant Docker as docker daemon
    participant MsgBus as message-bus

    Browser->>Gateway: POST /v2/app_management/compose<br/>(YAML body, JWT cookie)
    Gateway->>Tracing: enter chain
    Tracing->>Tracing: read X-Request-Id<br/>(or NewID() = 32 hex chars)
    Tracing->>Recover: ctx now has correlation_id
    Recover->>App: forward (UDS)<br/>InjectHeader X-Request-Id

    App->>App: parse YAML, generate compose project
    App->>Docker: docker compose up
    Docker-->>App: stream container events
    App->>MsgBus: publish "install.progress" events
    MsgBus-->>Browser: SSE forward

    Docker-->>App: install complete
    App->>App: log Info(ctx, "install ok",<br/>slog.String("app", name))
    App-->>Recover: 200 OK
    Recover-->>Tracing: 200 OK
    Tracing-->>Gateway: header X-Request-Id echoed
    Gateway-->>Browser: 200 OK + correlation_id in header
```

Every log line in every service the request touches carries the same
`correlation_id`. Greppping a single 32-char hex string reconstructs
the full path.

## Error path: panic recovered

When a handler panics — any nil-deref, any out-of-bounds, any
contract violation — `pkg/lifecycle.RecoverMiddleware` catches it and
turns it into a logged 500 response without taking down the process.

```mermaid
sequenceDiagram
    autonumber
    participant Browser
    participant Gateway
    participant Tracing as pkg/tracing
    participant Recover as pkg/lifecycle<br/>RecoverMiddleware
    participant Logger as pkg/logging
    participant Errors as pkg/errors
    participant Handler

    Browser->>Gateway: GET /v1/sys/something
    Gateway->>Tracing: ctx with correlation_id
    Tracing->>Recover: forward
    Recover->>Handler: ServeHTTP

    Handler-->>Recover: PANIC (e.g. nil-deref)

    Note over Recover: defer recover() catches the panic
    Recover->>Logger: Error(ctx, "panic recovered",<br/>err, slog.String("stack", debug.Stack()),<br/>slog.String("path", "/v1/sys/something"))
    Logger->>Logger: emit JSON line with correlation_id

    Recover->>Errors: WriteHTTP(ctx, w, ErrInternal)
    Errors-->>Browser: 500 + body { code: "common.internal",<br/>i18n_key: "errors.internal",<br/>correlation_id: "abc123" }

    Note over Browser,Recover: Process keeps running.<br/>Other concurrent requests unaffected.
```

This is the structural fix for **bug #64** (gateway `checkURL`
SIGSEGV). Once the gateway rewrite (#73 part 3) wires
`RecoverMiddleware` at the chain's outermost layer, the inverted
condition + nil-deref produces a logged 500 instead of a process
crash.

## Error path: typed error from handler

When a handler explicitly returns an `*errors.Error`, `WriteHTTP`
serializes it with the correlation ID, the i18n key, and structured
details — the UI can translate the message and surface meaningful
context.

```mermaid
sequenceDiagram
    autonumber
    participant Handler
    participant Errors as pkg/errors.WriteHTTP
    participant Browser

    Note over Handler: User tries to install on a port already in use

    Handler->>Errors: WriteHTTP(ctx, w,<br/>errors.ErrConflict.<br/>WithField("port", 8080).<br/>WithField("service", "nginx"))

    Errors->>Errors: extract typed error via errors.As
    Errors->>Errors: pull correlation_id from ctx

    Errors-->>Browser: 409 + body { code: "common.conflict",<br/>i18n_key: "errors.conflict",<br/>correlation_id: "abc123",<br/>details: { port: 8080,<br/>service: "nginx" } }

    Note over Browser: UI translates i18n_key,<br/>shows toast with details,<br/>user reports correlation_id<br/>on bug report
```

## Why the layers matter

| Layer | Without it | With it |
|---|---|---|
| `pkg/tracing` | Logs in different services unrelatable | One ID joins everything |
| `pkg/lifecycle.RecoverMiddleware` | Single panic crashes whole service | Panic → logged 500, service stays up |
| `pkg/errors.WriteHTTP` | Plain-text 500 page (#50 class) | Structured JSON, UI can translate |
| `pkg/logging` (`Logger.With(...)`) | `fmt.Println` debug spam | Structured search keys, correlation auto |

## Reference

- `pkg/tracing/tracing.go` — `Middleware` and `InjectHeader`
- `pkg/lifecycle/middleware.go` — `RecoverMiddleware` and `SafeGo`
- `pkg/errors/errors.go` — `WriteHTTP` and the catalog
- ADRs 0012-0015 cover the design of each foundation package
