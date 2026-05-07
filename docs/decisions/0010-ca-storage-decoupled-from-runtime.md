# 0010 — CA storage decoupled from the runtime data dir

**Status:** accepted
**Date:** 2026-05-07
**Tags:** security, https, persistence, v0.3.0

## Context

The v0.2.7 implementation stored the local CA + leaf at:

- Production: `/etc/powerlab/tls/`
- Dev: `<runtimePath>/tls/` (typically `backend/runtime/tls/`)

The dev path is colocated with the runtime data dir. That made it
trivial for a `start.sh --build` cycle, a `rm -rf backend/runtime/`,
or any cleanup script to delete the CA along with logs and PIDs —
silently breaking every device that had previously installed it.

In production the path was already on `/etc/`, but the `tls/`
basename is ambiguous: a sysadmin grepping `/etc/powerlab/` for
"what is this dir?" would not immediately read it as "the
authoritative CA the panel signs with".

We had a real, observed regression today where the dev CA changed
between rebuilds because the runtime dir was wiped. The user had
the OLD CA in their macOS keychain, the panel was serving leaves
signed by a NEW CA, every browser hit `ERR_CERT_AUTHORITY_INVALID`.

## Decision

- **Production storage**: rename `/etc/powerlab/tls/` →
  `/etc/powerlab/security/`. The new name is unambiguous;
  `tls/` could plausibly be just runtime cache.
- **Dev storage**: change from `<runtimePath>/tls/` to
  `~/.config/powerlab/security/` (XDG convention). This path
  survives `start.sh --build`, `rm -rf backend/runtime/`, and
  pretty much every routine dev cycle that doesn't deliberately
  target it.
- **Migration**: `CertManager.Setup()` performs a one-shot
  migration on boot. If the legacy `/etc/powerlab/tls/` path
  exists and the new path is empty, the files are moved over
  with the same permissions. Idempotent: a no-op on subsequent
  boots.
- **`runtimePath` argument** to `NewCertManager` is preserved as
  a fallback when `os.UserHomeDir()` fails (rare, but possible
  in some sandboxed runtimes). Dev installations falling back
  to the legacy path still work.

## Rationale

- **Convention: `/etc/` survives data wipes, `/var/lib/` does
  not.** The CA is not "data" produced by the running app; it
  is closer to config — it's the cryptographic identity of the
  host. Storing it next to logs and runtime caches conflated
  two lifecycles that should be separate.
- **XDG `~/.config/` for dev** mirrors the prod location's role
  (config-tier, persistent) without polluting `/etc/` on a dev
  laptop.
- **Migration not required, but cheap.** The cost of getting it
  wrong (orphaned CA, every device re-trusts) is high; the cost
  of writing the migration is ~20 LOC. Easy decision.

## Alternatives considered

- **Just document "don't delete this dir".** Rejected: relying
  on documentation against the natural cleanup-cycle path is the
  pattern that produced the bug we're fixing.
- **Store the CA inside the user's home keychain instead of on
  disk.** Rejected: the CA private key needs to be readable by
  the gateway daemon (a different uid than the user); keychain
  integration is platform-specific and adds runtime dependencies.
- **Encrypt the CA private key with a passphrase the user types
  on first boot.** Rejected for v0.3.0: the operational cost
  (lost passphrase = lost CA) is higher than the protection
  buys for a LAN-only deployment. Tracked separately.

## Consequences

- Existing users upgrading from v0.2.7 will see a one-time
  migration log line. No user action required.
- Backups + DR docs need to reference the new path. Updated in
  `docs/HTTPS.md`.
- The dev-local path `~/.config/powerlab/security/` should be
  added to any "rm -rf to reset state" instructions so devs
  who *want* a clean reset can find it.

## References

- `backend/common/pkg/security/cert.go` — `NewCertManager`,
  `migrateLegacyStorage`
- ADR 0001 — cert validity (the CA is signed for 10 years; this
  storage decision is what makes that 10-year promise credible)
