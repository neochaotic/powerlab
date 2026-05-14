# 0027. Uber fx for Gateway Dependency Injection

- **Status:** accepted
- **Date:** 2026-05-14

## Context

The gateway is the front-door service for the entire PowerLab architecture. It manages reverse-proxying, HTTPS certificate lifecycle, mDNS broadcasting, and configuration reloading. Unlike the other microservices (which have a more linear initialization pattern), the gateway has complex, interdependent lifecycle hooks:
- The HTTP server needs the router.
- The router needs the reverse proxy handlers.
- The reverse proxy needs dynamic configuration.
- The mDNS broadcaster needs to know the HTTP/HTTPS ports.
- The TLS cert manager (ACME) needs to hook into the server listener.

Manually wiring these dependencies in `main.go` using a struct-based or global-variable approach leads to brittle initialization order and makes unit testing individual components difficult. 

## Decision

We use [Uber fx](https://github.com/uber-go/fx) as the dependency injection (DI) framework **for the gateway service only**. 

- `fx.Provide` is used to register constructors for services, routers, and configuration.
- `fx.Invoke` is used to trigger the application lifecycle (e.g., starting the HTTP server).
- `fx.Lifecycle` is used to register `OnStart` and `OnStop` hooks for graceful shutdown.

## Consequences

- **Positive:** Clean, declarative initialization in the gateway. Components only declare what they need, and `fx` builds the dependency graph and guarantees the correct initialization order.
- **Positive:** Easier unit testing for gateway components, as dependencies can be easily mocked and injected.
- **Negative (Controlled):** We introduce a heavy, reflection-based DI framework.
- **Constraint:** To prevent `fx` from spreading throughout the codebase and adding unnecessary complexity to simpler services, its use is strictly limited to `backend/gateway`. Other services (like `core` or `message-bus`) will continue to use manual dependency wiring.
