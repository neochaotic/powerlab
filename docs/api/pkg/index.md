# Go API reference — pkg/*

These are the foundation packages every PowerLab service consumes. They are PowerLab-owned (`github.com/neochaotic/powerlab/backend/pkg/*`), have 100% godoc coverage, and are stable across releases.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) — every release rebuilds them so they never drift from the code.

## Packages

- [logging](logging.md) — structured logger built on log/slog
- [errors](errors.md) — typed errors with code, i18n key, HTTP status
- [lifecycle](lifecycle.md) — graceful shutdown + panic recovery
- [tracing](tracing.md) — correlation IDs via X-Request-Id
- [foundation](foundation.md) — composes the above into one Wrap call
- [migrations](migrations.md) — versioned migration runner over goose

## Service packages

Per-service Go packages (`backend/<svc>/`) are NOT in this site yet — godoc coverage there is below the 70% threshold for inclusion (tracked in [issue #196](https://github.com/neochaotic/powerlab/issues/196) with a per-module raise plan). They'll be surfaced once each service hits the bar.

For now, browse them on GitHub:

- [backend/gateway](https://github.com/neochaotic/powerlab/tree/main/backend/gateway)
- [backend/core](https://github.com/neochaotic/powerlab/tree/main/backend/core)
- [backend/user-service](https://github.com/neochaotic/powerlab/tree/main/backend/user-service)
- [backend/message-bus](https://github.com/neochaotic/powerlab/tree/main/backend/message-bus)
- [backend/app-management](https://github.com/neochaotic/powerlab/tree/main/backend/app-management)
- [backend/local-storage](https://github.com/neochaotic/powerlab/tree/main/backend/local-storage)
- [backend/common](https://github.com/neochaotic/powerlab/tree/main/backend/common)
