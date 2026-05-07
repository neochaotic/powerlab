# 0011 — CA-mismatch detection + browser-side HSTS recovery

**Status:** accepted
**Date:** 2026-05-07
**Tags:** security, https, ux, v0.3.0

## Context

After v0.2.7 we had three real failure modes on the trust dance
that were not handled gracefully:

1. **Server-side CA changed, client doesn't know.** The user
   installed a CA, the panel rotated or regenerated, every
   subsequent visit hits `ERR_CERT_AUTHORITY_INVALID`. The user
   sees the red Chrome wall and thinks the panel is broken.

2. **Browser HSTS pin survives a server-side reset.** The HSTS
   gate file is removed on the server, but the browser already
   cached `Strict-Transport-Security: max-age=31536000`. For up
   to 1 year the browser refuses to talk HTTP to the host. If
   the cert chain is also broken in the same period, the user
   has no path back without `chrome://net-internals/#hsts`.

3. **No "trust state" inspection.** Until now there was no way
   for the SPA to know the server's current CA fingerprint
   without parsing the actual cert via the (browser-restricted)
   security info API.

These three together can lock a user out of their own panel even
though the server is healthy.

## Decision

Three additions:

### 1. `GET /v1/sys/trust-state` endpoint

Unauthenticated, returns:

```json
{ "armed": true, "ca_fingerprint": "01:05:8D:..." }
```

The CA fingerprint is SHA-256 of the cert DER bytes, formatted
as colon-separated uppercase hex (the `openssl x509 -fingerprint
-sha256` shape so external tools can compare directly).

**Unauthenticated by design**: the CA cert is public information
already, the HSTS armed state is observable by anyone who hits
the host. Adding auth would just create a chicken-and-egg with
the trust dance.

### 2. UI `TrustStateChecker` component

Mounts globally (in `+layout.svelte`). On every app load:

- Reads `localStorage.powerlab_trusted_ca_fp` (set by a successful
  trust dance).
- Fetches `/v1/sys/trust-state`.
- If the local value is non-empty AND differs from the server
  fingerprint → surfaces a small amber pill: "Trust changed —
  re-install CA". Click → `/settings#security`.

The component is silent on the happy paths (no local fingerprint,
match, network error). It only fires when the user has previously
trusted SOMETHING and the server has moved on — which is
exactly when they would otherwise hit the cert wall.

### 3. HSTS disarming window

After a `Reset trust` (`DELETE /v1/sys/trust-confirmed`) or a
`Rotate CA`, the server emits `Strict-Transport-Security:
max-age=0` for 15 minutes (`HSTSDisarmingTTL`). RFC 6797 §6.1.1
mandates that browsers evict their cached HSTS pin on
`max-age=0`. This is the only mechanism that recovers a browser
that already pinned without forcing the user to clear cache
manually.

State machine of HSTS header emission:

| Server state | HTTPS request | HTTP request |
|---|---|---|
| Unarmed (default) | no header | passthrough |
| Armed | `max-age=31536000; includeSubDomains` | 301 → HTTPS |
| **Disarming** (post-reset, < TTL) | **`max-age=0`** | **passthrough** |

The disarming marker file (`.hsts-disarming`) is checked by
mtime; once older than `HSTSDisarmingTTL`, the middleware
ignores it and behavior reverts to "unarmed".

## Rationale

- **Mismatch detection has to be explicit; browsers do not surface
  it.** A SPA cannot inspect the actual cert chain it ran over.
  Comparing fingerprints via a `/trust-state` endpoint is the
  cleanest available mechanism.

- **`max-age=0` is the standards-track HSTS clear** (RFC 6797
  §6.1.1). Every major browser implements it. We're not using a
  workaround; we're using the spec's prescribed recovery
  mechanism.

- **15 minutes for the disarming window** is short enough that
  CDN / proxy caches don't latch onto `max-age=0` for hours, and
  long enough for the user to load the page on each device they
  want to recover.

## Alternatives considered

- **Just tell users to clear `chrome://net-internals/#hsts`.**
  Rejected: requires the user to type a chrome:// URL, then a
  hostname, then click a button. Gambiarra.

- **Force a hostname rotation** (`powerlab.local` →
  `powerlab2.local`) on rotation. Rejected: HSTS cache is
  per-host so it works, but renaming the host is a much bigger
  change than the disarm-window fix.

- **Silent re-prompt instead of explicit banner.** Rejected:
  on a CA mismatch the user genuinely needs to take action
  (install the new CA on their device). Hiding it would just
  delay the failure to the next HTTPS visit.

## Consequences

- The `/v1/sys/trust-state` endpoint exposes the CA fingerprint
  to anyone who can reach the panel. This is fine: the CA
  fingerprint is published in every TLS handshake anyway. It's
  not a secret.

- Browsers respect `max-age=0` per the RFC, but corporate
  proxies and CDNs may strip the header. If the disarming
  doesn't propagate the first time, the user can still recover
  via the `/v1/sys/trust-state` mismatch banner + manual
  re-install.

- `localStorage.powerlab_trusted_ca_fp` is now part of our
  client-state contract. A future i18n / persistence refactor
  must preserve it (see `feedback_redirect_validation.md` for
  the related "never invalidate trust state without surfacing
  recovery" memory).

## References

- RFC 6797 §6.1.1 — "If the value of `max-age` is 0, the UA
  MUST remove its cached HSTS Policy information"
- `backend/common/pkg/security/cert.go` — `CAFingerprint`,
  `DisarmHSTS`, `IsHSTSDisarming`, `HSTSDisarmingTTL`
- `backend/gateway/route/security_route.go` — `handleTrustState`
- `backend/gateway/route/gateway_route.go` — `WrapHSTS`
  middleware (now with disarming branch)
- `ui/src/lib/components/security/TrustStateChecker.svelte`
- ADR 0006 — HSTS gate after first verified non-localhost client
