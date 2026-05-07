# 0007 — Initial deployment scope: internal LAN only

**Status:** accepted
**Date:** 2026-05-07
**Tags:** scope, deployment, security, v0.2.x

## Context

PowerLab is a server panel for home / homelab use. Several design
choices in v0.2.x — local-CA HTTPS, single-admin model, no
rate-limited login, no audit log retention policy, no encryption at
rest for the user database — make sense **only if the panel is
reachable solely from a trusted local network**.

We need to set this scope explicitly so:

- Future contributors understand which threat model the code defends
  against (and which it doesn't).
- Users who try to expose the panel to the public internet know
  what they're signing up for.
- Reviewers can challenge a PR that adds public-facing assumptions
  to a v0.2.x codebase.

## Decision

PowerLab v0.2.x targets **internal network deployment only**. The
threat model assumes:

- The user is on the same LAN (or VPN/Tailscale-mesh that they
  control end-to-end) as the server.
- There is a single admin user. Multi-user is not supported.
- A compromised browser session has root-equivalent access on the
  host (this is what ADR-future #36 will harden).
- Outbound DNS resolution works (for the App Store catalogue
  fetches) but no inbound internet exposure is assumed.

If the user wants to expose the panel publicly, they must:

- Front it with a reverse proxy (NGINX / Caddy / Traefik) that
  handles TLS via Let's Encrypt or similar.
- Set `TLSEnabled=false` in `gateway.ini` so PowerLab doesn't
  duplicate TLS handling.
- Be aware that the rest of the v0.2.x security model wasn't
  designed for that exposure.

This will be documented prominently in `docs/HTTPS.md` and in
`docs/SECURITY.md`.

## Rationale

- **Mismatch is the bigger risk.** Shipping with a "supports
  internet exposure" promise we can't deliver is worse than shipping
  with a "LAN only" promise we can.
- **Real users today.** PowerLab is positioned for homelab and
  small-team-internal use. The premium README, the App Store, the
  in-UI updater, the editor — all built around "this is your
  server you visit on your network".
- **Future is open.** Public-internet support is a real product
  direction (Tailscale Funnel pattern, Cloudflare Tunnel pattern,
  managed certs via ACME). It just isn't v0.2.x. ADR for that
  pivot will land as `0NNN-public-internet-deployment.md` when
  the work starts.

## Alternatives considered

- **"Hardened from day one" mode** — design every feature for the
  worst-case threat model. Rejected: massively slower to ship, and
  we'd over-build security for users who don't need it. mkcert and
  Tailscale both ship LAN-first and have years of credibility.
- **Feature flags per deployment mode** — `--internal` vs
  `--public` toggle. Rejected: doubles the test matrix, and it's
  way too easy for a flag to default-flip back to "public" after a
  config edit. Better to scope explicitly.

## Consequences

- Several v0.2.x features assume LAN trust:
  - HTTP fallback exists (until HSTS gate flips per ADR 0006).
  - The CA download endpoint is unauthenticated (catch-22:
    user needs the CA to authenticate). Acceptable on LAN; would
    not be acceptable public.
  - The Files page has no per-user scope sandbox yet (#36).
  - The terminal websocket runs unprivileged shell as the daemon's
    OS user — full host access for any logged-in admin.
- Public-internet deployment is supported via reverse proxy, with
  the user taking responsibility for the layered security.
- The threat model section of `docs/SECURITY.md` MUST stay current
  as features land. If a v0.3 feature requires hardening this
  scope, the ADR for that feature has to either (a) extend the
  threat model or (b) trigger this ADR's supersession.

## References

- Issue [#43](https://github.com/neochaotic/powerlab/issues/43) — v0.2.7
  milestone, references this ADR for the deployment context.
- ADR 0001 — cert validity (depends on LAN-only assumption).
- ADR 0003 — reset-trust UX (single confirm because of single-admin
  model).
- Issue #36 — per-user FS scope sandbox (one of the hardenings
  needed before public-internet mode).
