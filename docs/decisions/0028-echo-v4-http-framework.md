# 0028. Echo v4 as the HTTP Framework

- **Status:** accepted
- **Date:** 2026-05-14

## Context

All PowerLab backend services expose REST APIs over HTTP. To maintain consistency across the project, we need a single, standard HTTP framework used by every service, rather than mixing `net/http`, `gin`, `chi`, and others.

CasaOS originally selected Echo. During the PowerLab fork, we evaluated whether to migrate to the standard library's `net/http` (especially given the `net/http` routing improvements in Go 1.22) or switch to a lighter router like `chi`.

## Decision

We use [Echo v4](https://github.com/labstack/echo) as the universal HTTP framework for all backend services. 

- All API routes are built using `echo.Echo` or `echo.Group`.
- Middleware (CORS, logging, JWT auth) uses Echo's middleware signatures.
- Context is passed using `echo.Context`.

## Consequences

- **Positive:** High performance and zero-allocation routing.
- **Positive:** We maintain compatibility with the extensive existing route handler codebase inherited from CasaOS, avoiding a massive, low-value rewrite of every HTTP handler.
- **Positive:** Excellent integration with our OpenAPI codegen tooling (`oapi-codegen`), which provides an out-of-the-box Echo server generator and Echo middleware for request validation.
- **Negative:** We remain tied to a third-party framework for our primary transport layer, meaning we wrap standard `http.Handler` objects when interacting with tools that expect standard library signatures.
