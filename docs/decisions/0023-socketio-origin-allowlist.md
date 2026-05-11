---
title: "0023 — SocketIO CheckOrigin allowlist (close #219 CORS bypass)"
status: accepted
date: 2026-05-11
tags: security, message-bus, sprint-8
---

# 0023 — SocketIO CheckOrigin allowlist

**Status:** accepted
**Date:** 2026-05-11
**Tags:** security, message-bus, sprint-8

## Context

The message-bus exposes a SocketIO endpoint that the UI subscribes to
for live event/action streaming. The underlying `engineio` transports
(websocket + polling) each carry a `CheckOrigin func(*http.Request) bool`
field that decides whether to honour the cross-origin handshake.

Up to this ADR both transports were initialised with `CheckOrigin =
func(_ *http.Request) bool { return true }`, an explicit `// TODO
remove this debug setting` comment in CasaOS-era code. That meant any
origin — including a hostile page on the LAN — could open the stream
in a victim's browser and read every event the user is allowed to read.

The bypass was mitigated by the gateway's JWT auth (caller must
present a valid token) and PowerLab's same-LAN trust model
(ADR-0007), but the bypass IS real. Issue #219 captured the finding
out of the 2026-05-10 quality audit.

## Decision

Replace the unconditional `return true` with an allowlist enforced by
a small pure function:

```
newOriginChecker(allowedOrigins []string) func(*http.Request) bool
```

Rules, evaluated in order:

1. **Empty `Origin` header → allow.** Non-browser clients (curl,
   server-to-server `socket.io-client` without an explicit origin)
   never expose the CORS bypass.
2. **Same-origin → allow.** Origin host equals the request's
   destination Host. Common case when message-bus is reached through
   the gateway with Host preserved, or directly without a reverse
   proxy.
3. **Configured allowlist → allow.** Operator-supplied origins from
   the new `[security] AllowedOrigins` (comma-separated) section of
   `message-bus.conf`. Comparison is case-insensitive on the full
   origin (`scheme://host[:port]`).
4. **Anything else → reject** and emit a WARN log line with the
   offending Origin + the destination Host so the operator can
   either tighten the allowlist further or wire in a legitimate
   cross-origin app.

Default config ships `AllowedOrigins=` (empty). Vanilla installs
that don't proxy from a different origin keep working through the
same-origin rule.

## Why an allowlist, not magic localhost-in-dev

We considered adding a `dev mode → allow localhost variants`
shortcut. Rejected: the dev environment already drives a known
config, the operator can put `http://localhost:5173,
http://127.0.0.1:5173` in the allowlist explicitly. Magic rules in
security code are how `return true` survived for years.

## Why same-origin is allowed without configuration

Most installs run UI + gateway + message-bus on the same host,
behind the gateway's reverse proxy. Browsers send `Origin:
http://<gateway-host>:<port>` and the message-bus sees `Host:
<gateway-host>:<port>` (Echo + the standard reverse-proxy chain
preserve Host by default). Enforcing same-origin in this path keeps
zero-config installs functional; cross-origin deployments must opt
in.

## Test discipline

Per the TDD memory, the regression suite was authored before the
implementation:

- `socketio_origin_test.go` lives in
  `backend/message-bus/service/`
- Six unit tests cover: empty-Origin allow, same-origin allow,
  configured allow, case-insensitive allow, unknown reject, and
  the "blank entry must not collapse to wildcard" guard
- One table-driven test on `parseAllowedOrigins` covers
  whitespace, blank-entry stripping, and the empty input
- Failing-first → minimum-code-to-pass → race-clean across the
  whole `backend/message-bus/...` tree

Closes #219.

## Related

- ADR-0007 — same-LAN trust model
- ADR-0019 — tech-debt tracking surface
- `docs/audits/quality-and-tech-debt-2026-05-10.md` — security findings section
