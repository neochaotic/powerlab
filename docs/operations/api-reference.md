# API reference

PowerLab ships an interactive API documentation portal at `/docs` on every running instance, rendered with [Scalar](https://github.com/scalar/scalar). The portal is embedded in the gateway binary — no internet, no separate daemon, no CDN.

For the design rationale (why Scalar over Swagger UI, why embedded, why no spec mutation), see [ADR-0008](../decisions/0008-api-docs-portal-scalar.md).

## Live portal

On a running PowerLab host, point a browser at:

```
http://<your-host>:8765/docs
```

Or, with HTTPS enabled and trust completed:

```
https://powerlab.local/docs
```

The portal exposes a service dropdown covering all six backend services:

| Service | Purpose |
|---|---|
| **gateway** | Front door, proxies to other services, owns mDNS + TLS cert manager |
| **app-management** | Docker Compose lifecycle (install / uninstall / disk-usage / SSE task logs) |
| **core** | System metrics (CPU, RAM, GPU, disks, network) |
| **user-service** | Auth (PAM on Linux, dscl on macOS, bcrypt SetupWizard fallback) |
| **message-bus** | Pub/sub between services |
| **local-storage** | Disk / USB / RAID operations |

Each service's panel includes search, schema browser, and a "Try it out" request playground. Bearer tokens can be pre-filled via the URL fragment (`#access_token=...`) — fragments stay client-side and never reach server access logs.

> **Screenshot stub.** The portal is visually identical to upstream Scalar with PowerLab branding in the host page. A canonical screenshot will be added here once the brand polish (this PR's `brand.css`) propagates to a fresh deploy. Track under the docs site polish issue series.

## Source-of-truth specs

The portal serves spec files copied at build time from each service's canonical OpenAPI definition. Editing the canonical files is the only supported way to evolve the API — the portal is a presentation layer, not an editor.

Canonical paths in the repo:

| Service | OpenAPI file |
|---|---|
| gateway | `backend/gateway/api/gateway/openapi.yaml` |
| app-management | `backend/app-management/api/app_management/openapi.yaml` (+ `openapi_v1.yaml` for the v1 surface) |
| core | `backend/core/api/casaos/openapi.yaml` (rebrand pending) |
| user-service | `backend/user-service/api/user-service/openapi.yaml` |
| message-bus | `backend/message-bus/api/message_bus/openapi.yaml` |
| local-storage | `backend/local-storage/api/local_storage/openapi.yaml` |

These files are the input to `oapi-codegen`, which generates the Echo route bindings under each service's `codegen/` directory. The generated code is `.gitignored` and re-emitted on every build.

The build-time copies served by the portal live under `backend/gateway/api/docs/openapi_*.yaml`, refreshed by `start.sh`'s `sync_specs()` helper. ADR-0008 covers the round-trip discipline that keeps the two in sync.

## Endpoint shape

| Path | What |
|---|---|
| `GET /docs?service=<id>` | Scalar host HTML, service preselected |
| `GET /docs/spec?service=<id>` | The embedded OpenAPI YAML for that service |
| `GET /docs/scalar.js` | The bundled Scalar runtime |

Unknown service IDs fall back to the gateway service rather than 404, so a stale bookmark or typo lands on a working page.

## To expand

- Authenticated example flows (login, install an app, stream task logs) once the v0.6 examples-as-tests work lands.
- A "what changed since v0.5" delta page generated from the spec history.
- A canonical Postman/Insomnia collection export.

Track gaps under the docs site polish issue series.
