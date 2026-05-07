# 0006 — HSTS gated on first verified non-localhost client

**Status:** accepted
**Date:** 2026-05-07
**Tags:** security, https, v0.2.7

## Context

`Strict-Transport-Security` is the header that tells browsers
"this origin is HTTPS only — refuse fallback to HTTP forever".
It's the right thing to send on a properly-trusted HTTPS deployment.

It's also a foot-gun on a deployment where the user is **still
finishing the trust dance**. Sequence that bites:

1. User installs PowerLab, accesses over HTTP.
2. User clicks "Enable Secure Connection", starts the CA install
   walkthrough.
3. They flip over to HTTPS to test, but they haven't actually
   completed the OS-level "trust this CA" step yet.
4. Browser sees HTTPS response with HSTS header → caches "this
   origin is HTTPS only".
5. User goes back to HTTP to figure out what went wrong.
6. Browser refuses the HTTP request (HSTS).
7. User can't see the panel. They have no idea why. They need
   to dig through `chrome://net-internals/#hsts` to fix it.

Lock-out. Worst-case UX failure mode.

## Decision

PowerLab does NOT emit the `Strict-Transport-Security` header until
**at least one non-localhost client** has confirmed trust by
successfully completing an HTTPS handshake AND posting to
`/v1/sys/trust-confirmed` from that HTTPS connection.

The state is persisted in `/etc/powerlab/tls/.hsts-armed` (presence
file, no content needed). Once the file exists:

- HSTS header is emitted on every HTTPS response.
- HTTP requests are 301-redirected to HTTPS.

Before the file exists:

- HTTP and HTTPS both serve normally, no redirect.
- HTTPS responses have no HSTS header — a browser that fails the
  trust check can fall back to HTTP without ceremony.

## Rationale

- The flag's existence is proof, not assertion: it's only created
  after a real HTTPS handshake from a real device. We can't fake
  ourselves into thinking we're trusted.
- Restricting to non-localhost ensures the dev environment doesn't
  arm the gate prematurely (curl from the host hits `127.0.0.1`,
  which doesn't count).
- Single-file state is restartable: gateway crashes mid-config,
  the flag survives, no re-doing the trust dance after every reboot.
- Recoverable: if the flag is misset and a real user is locked out,
  `sudo rm /etc/powerlab/tls/.hsts-armed` puts everything back to
  HTTP-permitted. Documented in `docs/HTTPS.md` troubleshooting.

## Alternatives considered

- **Always emit HSTS once HTTPS lands.** Rejected: user-hostile
  during the trust install flow. Inevitable production lock-outs.
- **Emit HSTS based on a Settings toggle.** Rejected: an admin
  could mis-toggle and trigger the same lock-out. The point of the
  gate is that it's automatic and self-correcting.
- **Emit HSTS with a short `max-age` initially, lengthen later.**
  Rejected: short max-age means the browser keeps re-checking
  HSTS state, defeating the persistence guarantee. We want long
  max-age (1 year), so we have to be confident before arming.

## Consequences

- The `POST /v1/sys/trust-confirmed` endpoint is JWT-protected,
  HTTPS-only, non-localhost — the polling Test Connection in the UI
  is what actually fires it. No way for a malicious LAN scanner to
  arm the gate without authenticating.
- The flag file (`/etc/powerlab/tls/.hsts-armed`) is `0644` because
  it's presence-only — the value doesn't matter. We log every
  creation in the audit log so admins know when the gate flipped.
- If the user uses "Reset trust" (ADR 0003), the flag is also
  removed — fresh CA means fresh trust dance.
- We commit to monitoring the gate's state in the Settings → Security
  page. A panel that says "HSTS: armed since 2026-05-08, on 2 of 3
  known devices" tells admins exactly where they are.

## References

- Issue [#19](https://github.com/neochaotic/powerlab/issues/19) — Backend
  HTTPS spec (HSTS gate is the load-bearing safety here).
- Issue [#43](https://github.com/neochaotic/powerlab/issues/43) — v0.2.7
  milestone, R1 in the risk register is "HSTS lock-out", mitigated by
  this ADR.
- ADR 0003 — Reset-trust UX, which depends on this flag for
  recoverability.
