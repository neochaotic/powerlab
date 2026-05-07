# 0001 — HTTPS cert validity: 1-year leaf, 10-year CA

**Status:** accepted
**Date:** 2026-05-07
**Tags:** security, https, tls, v0.2.7

## Context

PowerLab generates a local Certificate Authority on first boot and signs
a leaf certificate that the gateway presents over HTTPS. We have to
pick a validity window for both certificates.

Constraints:

- LAN-only deployment (no public DNS, no Let's Encrypt — see ADR 0007).
- Browser policy: Chrome and Safari refuse certificates with validity
  longer than 825 days, even when issued by a custom CA.
- The server is a homelab box that frequently stays up for months
  between reboots.
- The certificate has to renew without manual intervention — users
  shouldn't have to know the CA exists.

## Decision

- **CA root validity: 10 years** (`/etc/powerlab/tls/ca.{crt,key}`).
- **Leaf cert validity: 1 year** (`/etc/powerlab/tls/server.{crt,key}`).
- A daily ticker re-issues the leaf when its remaining validity drops
  below 60 days.
- An IP-change watcher re-issues the leaf immediately when the host's
  IP set changes (DHCP renewal, network swap, multi-NIC toggle,
  mesh-VPN tunnel coming up or going down).

### SAN list (revised 2026-05-07 for mesh-VPN coexistence)

DNS:
- `powerlab.local`
- `<system-hostname>.local`
- `localhost`

IP:
- `127.0.0.1`, `::1`
- All RFC1918 addresses bound to host interfaces
  (`10/8`, `172.16/12`, `192.168/16`)
- IPv6 ULA `fc00::/7`
- **CGNAT `100.64.0.0/10`** — included only when a tunnel-style
  interface (`tailscale*` on Linux, `utun*` on macOS) is present on
  the host. Without this, a user accessing the panel through a
  mesh-VPN tunnel after confirming trust on LAN gets
  cert-not-trusted; combined with HSTS already armed from the LAN
  visit, that's a lock-out.
- Excluded: Docker bridge ranges (typically `172.17/16` — filtered by
  interface name `docker0` / `br-*`), WireGuard (`wg*`),
  link-local (`fe80::/10`).

## Rationale

- **1 year leaf** is the sweet spot:
  - Browser policy ceiling is 825 days (~27 months); 1 year is well
    below that, so no surprises.
  - The daily renew ticker has a 60-day margin to catch failures —
    even if the ticker silently dies for two months, the next boot
    still has time to recover before HTTPS breaks.
  - If the leaf's private key is somehow exposed, the exploitation
    window is bounded at one year.
- **10 year CA** is the mkcert convention. The CA private key is
  protected at file system level (`0600`, root-only) and is never
  served by any handler. Rotating the CA more frequently would cost
  more than it'd help, because every rotation forces every device
  the user installed the CA on to re-trust.
- **Daily ticker** is `~30 LOC` of `time.Ticker(24*time.Hour)`. Cheap
  insurance against the bomb-relógio of "everything was fine until the
  cert silently expired in production".
- **IP-change watcher** is critical: a leaf signed with SAN
  `192.168.1.42` becomes useless the moment the box's IP changes to
  `192.168.1.43`. Detect from the mDNS service that already watches
  interface state.

## Alternatives considered

- **90-day leaf** (Let's Encrypt convention). Rejected: a homelab box
  that stays up for months will silently break HTTPS on day 91 if the
  ticker fails. Combined with HSTS already armed, the user has no
  fallback path and is locked out of their own server.
- **10-year leaf**. Rejected: 10-year exposure if the leaf private key
  leaks (e.g., backup misconfigured, drive not encrypted). Plus
  browsers reject anything > 825 days.
- **No automatic renewal — manual button in Settings**. Rejected:
  defeats the goal of "users shouldn't have to know the CA exists".

## Consequences

- We commit to maintaining the daily-ticker code in production. A bug
  there silently degrades to "user finds out 11 months later that
  HTTPS is broken". The Settings → Security page must show the timestamp
  of the last successful renew, with an amber warning if it's > 30 days
  ago.
- The IP-change watcher must be debounced (DHCP renewal can flap an IP
  multiple times in a few seconds during a network swap, and a
  mesh-VPN tunnel bringing its interface up post-boot triggers
  another change).
- ADR 0006 (HSTS gate) is what prevents the lock-out scenario; this
  ADR assumes that gate exists.
- CGNAT in SAN means the cert lists at least one IP a remote
  attacker on the same VPN mesh could see if they capture handshake
  traffic. Since the mesh is the user's own, and CGNAT IPs are not
  publicly routable, this is a non-issue in practice. The cert is
  public information by definition.
- Custom DNS hostnames advertised by the mesh-VPN provider (e.g.
  `<host>.<tailnet>.ts.net`-style) are NOT in the SAN for v0.2.7 —
  detecting them requires querying the VPN client's local API and we
  don't want to add that runtime coupling yet. Users accessing via
  such a hostname see cert-not-trusted; deferred to a follow-up.

## References

- Issue [#19](https://github.com/neochaotic/powerlab/issues/19) — Backend
  HTTPS spec.
- Issue [#43](https://github.com/neochaotic/powerlab/issues/43) — v0.2.7
  HTTPS milestone.
- [Apple's 825-day cap](https://support.apple.com/en-us/HT211025).
- [mkcert](https://github.com/FiloSottile/mkcert) — pattern reference.
