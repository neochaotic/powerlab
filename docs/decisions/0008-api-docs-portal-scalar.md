# 0008 — API documentation portal: Scalar, embedded, no spec mutation

**Status:** accepted
**Date:** 2026-05-07
**Tags:** docs, gateway, openapi, developer-experience

## Context

PowerLab exposes a coherent REST surface across six backend services
(gateway, app-management, user-service, core, message-bus,
local-storage), each with its own `openapi.yaml` consumed by
`oapi-codegen`. External integrators (and the user's future MCP
server, see issue #46) need a way to read these specs interactively
— "Try it out" buttons, search, syntax-highlighted schemas — without
asking them to clone the repo and run a docs generator locally.

The first attempt at this fed the specs through a custom Go script
(`scratch/rebrand_specs.go`) that stripped every `description:` block
and inserted a hardcoded base64-encoded SVG logo in their place. The
result was structurally broken YAML (duplicate `description:` keys
at the same level, mis-indented entries) and lost human-written
documentation across every endpoint, parameter, schema, and tag.

We need a portal that:

- **Renders quality docs** without requiring spec mutation — the
  source `openapi.yaml` files are also consumed by codegen and must
  stay round-trippable.
- **Is self-contained at runtime** — no CDN, no internet calls. Per
  ADR 0007 the gateway runs on a LAN-only deployment.
- **Has zero ops cost** — embedded in the gateway binary, no extra
  daemon to install or secure.
- **Survives spec drift** — the embedded copy must be regenerated on
  every build so the portal never serves a stale spec.

## Decision

- **Renderer: Scalar** (`@scalar/api-reference`). Single-file JS
  bundle, dark theme, OpenAPI 3.x native, "Try it out" with a
  pre-fillable bearer token. Bundle is vendored under
  `backend/gateway/api/static/docs/scalar.js` and embedded with
  `//go:embed`.
- **Specs are immutable inputs.** The portal reads them via
  `embed.FS` from `backend/gateway/api/docs/openapi_*.yaml`, copies
  of the canonical per-service `openapi.yaml` refreshed at build
  time by `start.sh` (`sync_specs()`). Nothing mutates the source
  YAML after codegen has consumed it.
- **Branding lives in the host page**, not in the specs. The Scalar
  config in `portal.html` sets the title, theme, and metaData; the
  underlying YAML remains pristine and authored by service owners.
- **Endpoint shape**:
  - `GET /docs?service=<id>` — Scalar host HTML, service preselected
  - `GET /docs/spec?service=<id>` — embedded YAML for that service
  - `GET /docs/scalar.js` — the bundled runtime
- **Service registration is data-driven.** The canonical list lives
  in `docs.Services` and is rendered into both the dropdown and the
  service routing. Adding a new service is one entry + one spec
  copy, no code changes elsewhere.
- **Unknown service IDs fall back to the gateway service** rather
  than returning 404, so a stale bookmark or typo lands on a working
  page.
- **Token auth via URL hash.** A token passed as `#access_token=...`
  is read by the host page and injected into Scalar's auth config.
  The token never reaches the server (URL fragments are not sent in
  HTTP requests) so server access logs do not capture it.

## Rationale

- **Scalar** over Swagger UI / Redoc / Stoplight Elements: smallest
  single-file bundle, dark mode that doesn't fight our visual
  identity, MIT licensed, actively maintained, ships with the
  features we need (search, request playground, tags) without a
  build step.
- **`embed.FS` over disk reads**: deployments are immutable tarballs;
  we don't want a missing-file failure mode in prod.
- **Build-time sync** over watch-and-copy: simpler model, codegen
  already runs at build, this hooks in next to it.
- **No spec mutation, ever** is a hard rule. The previous attempt
  proved a one-off Go script that rewrites YAML can lose more value
  in five seconds than the portal adds in five releases.

## Alternatives considered

- **Swagger UI** with a separate hosted instance. Rejected: needs a
  CDN or local server, doesn't blend with PowerLab's chrome, more
  ops surface.
- **Redoc**. Rejected: read-only (no Try-it-out), heavier bundle.
- **Hand-curated docs in markdown**. Rejected: drifts from the spec
  every release; we already maintain OpenAPI specs for codegen, no
  reason to maintain a parallel doc set.
- **Pre-merge the per-service specs into one `openapi_unified.yaml`
  with a build-time tool**. Rejected for v1 — the dropdown is a
  better UX than a 7000-line spec, and merging schemas correctly is
  non-trivial. Defer to a future ADR if a unified consumer (e.g.
  Postman import) needs it.

## Consequences

- The portal is a developer-facing surface. If we add an endpoint or
  rename a parameter and forget to regenerate the embedded spec, the
  portal silently shows the old shape until next build. `start.sh`'s
  `sync_specs()` mitigates this for dev; CI must run the same step
  for production tarballs.
- The `data-url` on the Scalar host page is loaded via a
  same-origin GET. CORS is therefore moot for the portal-to-spec
  hop; if a future integration wants to consume the spec
  cross-origin, we'll need to widen `Access-Control-Allow-Origin`
  on `/docs/spec`.
- Token in URL hash is a small risk: hashes don't go to the server,
  but they DO show up in browser history and some screen-sharing
  scenarios. Acceptable for v1 because (a) PowerLab is LAN-only per
  ADR 0007 and (b) the user explicitly clicked "Open API Portal"
  with a freshly-generated token.

## References

- Scalar — <https://github.com/scalar/scalar>
- Issue #46 — MCP server (will reuse the same per-service split)
- ADR 0007 — Internal-only initial deployment
