# Go API reference — gateway

The gateway is PowerLab's single HTTP entry point. It owns the public listener (port 8765 by default), the in-memory route table that backend services register into, the embedded SvelteKit SPA, the certificate manager (CA + leaf cert lifecycle for HTTPS), the API docs portal (Scalar), and the mDNS responder so the host shows up as `powerlab.local`.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [common](common.md) — config loader (`gateway.ini` reader)
- [route](route.md) — the route bundles: `GatewayRoute` (public), `ManagementRoute` (internal), `SecurityRoute`, `DocsRoute`, `StaticRoute`
- [service](service.md) — runtime services: `Management` (route table), `MDNSService`, `State` (gateway port + paths)

## Where to start

If you want to understand the gateway's responsibilities, read [`Management`](service.md#Management) first — it's the in-memory route table backing the reverse-proxy. Then [`GatewayRoute`](route.md#GatewayRoute) for how the public listener is composed.

## Coverage

Gateway is the second module surfaced in the docs site (after `pkg/*`). Coverage rose from 21% (pre-Sprint 5) to 85% in PR following the [godoc raise plan](https://github.com/neochaotic/powerlab/issues/196).
