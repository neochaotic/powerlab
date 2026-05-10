---
title: "0020 — JWT signing keypair persists by default; ephemeral mode is opt-in"
status: accepted
date: 2026-05-10
tags: security, ux, user-service, casaos-strip
---

# 0020 — JWT signing keypair persists by default; ephemeral mode is opt-in

**Status:** accepted
**Date:** 2026-05-10
**Tags:** security, ux, user-service, casaos-strip

## Context

The user-service code that PowerLab inherited from CasaOS generates a
fresh ECDSA keypair on every `NewUserService(db)` call and never
persists it. The keypair lives in process memory until the next
restart, when a new one is generated. Result: every restart of the
service — including every in-app upgrade — invalidates every
outstanding JWT cookie. Users get silently logged out on every
upgrade.

A user-reported observation in v0.5.5 → v0.5.6 ("upgrade worked but
refresh logged me out") triggered the investigation that surfaced
this. See issue #176.

The pre-v0.5.7 godoc on `NewUserService` described this as a
"deliberate trade-off: session continuity across restarts is
sacrificed for a stronger guarantee that a stolen disk image cannot
forge tokens." Two problems with that framing:

1. **It was not actually deliberate.** Git blame shows the code was
   inherited verbatim in PowerLab's first public commit (`eb0d48c`,
   v0.1.0); the godoc comment claiming "deliberate trade-off" was
   added much later (`ce7993ad`, godoc Sprint 2 Phase 6) by the same
   author writing this ADR. The comment was a *post-hoc rationalization*
   of inherited behavior, not a record of a real architectural choice.

2. **The threat model doesn't survive scrutiny for PowerLab.**
   "Stolen disk image" attackers who can extract the user-service
   binary and database also have access to:
   - Bcrypt password hashes in `o_users.password` (offline crack)
   - Every config file under `/etc/powerlab/` (DB paths, secrets, env)
   - Container app data under `/var/lib/powerlab/apps/` (often raw
     credentials in compose env vars)
   - The ability to install backdoors in the binary or systemd unit
     and wait for the operator to power it back on

   JWT forge is a microscopic incremental capability against an
   attacker who already has all of the above. The cost — every user
   logged out on every upgrade — is significant and certain, while
   the benefit is contingent on an attack scenario where it adds
   little value.

PowerLab's primary use case is a self-hosted home server. The
home-server operator's threat model is:

- Network-borne attackers (bad actor exploits a misconfig from the LAN)
- Casual physical access (a guest plugging a USB stick)
- NOT: a sophisticated adversary with full disk-image custody

For that user, "every upgrade logs me out" is a real friction. The
cost is concrete and recurring; the benefit is theoretical and
non-cumulative.

## Decision

**JWT signing keypair persists in `user.db` by default.** A new
`jwt_keypair` table holds a single PEM-encoded ECDSA private key.
On startup, `NewUserService` loads the row if present; if absent
(first boot, or row manually wiped to force rotation) it generates
a fresh keypair and persists it. The keypair survives restarts,
upgrades, and crash recovery — existing JWT cookies stay valid
across all of these.

**Operators can opt back into the inherited behavior** via the
`POWERLAB_EPHEMERAL_JWT_KEY=true` environment variable. When set,
the service generates a fresh keypair on every startup and never
writes it to disk — same as pre-v0.5.7. No code path changes;
this is a single env-var check at the top of the
`loadOrGenerateKeypair` helper.

This change is **user-visible behavior** and lands in v0.5.7+
release notes. The first-boot path on a new install is unchanged
(generate fresh) — only the restart-after-first-boot path is
different (now reuses persisted key instead of generating a new one).

## Rationale

### Why default-persist (not default-ephemeral)

The PowerLab default should match the PowerLab user. A home-server
operator who upgrades their NAS at 11pm doesn't want every family
member's tablet kicked back to a login screen tomorrow morning. The
inherited behavior is hostile to that workflow.

A higher-threat operator (small business, multi-tenant) is the
*minority* and is also more sophisticated — they can opt out via
env var, document the choice, and accept the re-login cost. Forcing
the cost on every user to defend against a threat model most users
don't have is the wrong default.

### Why an env var, not a config file knob

`POWERLAB_EPHEMERAL_JWT_KEY=true` is:
- **Discoverable in process listings** (`systemctl show`, `ps`),
  so a future operator inheriting the host can SEE the choice
  rather than having to grep config files
- **Easy to set in a systemd unit override** without editing
  shipped configs
- **Greppable in support requests** ("what env vars do you have set?")

A config-file knob would require schema changes, parser updates,
and adds a place where the wrong default could quietly take over.
An env var is a thin contract.

### Why a CHECK constraint on the table id

`CREATE TABLE jwt_keypair (id INTEGER PRIMARY KEY CHECK (id = 1), ...)`.
The single-row constraint prevents someone from manually adding a
second keypair row that would silently shadow the first. INSERT OR
REPLACE always targets the same row by primary key. Multi-row
support is YAGNI.

## Consequences

**Positive:**
- Users no longer kicked out on upgrade (the user-reported issue #176)
- Disk-stored keypair survives crash recovery + scheduled restarts
- Operator-controlled trade-off via env var
- The "deliberate trade-off" godoc claim no longer lies

**Negative / accepted:**
- A stolen `user.db` now contains the JWT signing private key. Anyone
  with that file can forge tokens. We accept this cost; it's small
  relative to the bcrypt password hashes and config credentials
  already in the same backup-window.
- One additional schema migration (`0002_jwt_keypair.sql`) to
  maintain forever.

**Mitigations operators can layer on top:**
- Set `POWERLAB_EPHEMERAL_JWT_KEY=true` to revert (per-host opt-in).
- Encrypt the disk at rest (`/var/lib/powerlab` on a LUKS volume).
- Use a remote backup destination with a different trust boundary
  than the source disk.

## Alternatives considered

1. **Persist encrypted with a key derived from machine-id.** Best of
   both worlds (disk theft → can't forge tokens unless attacker also
   gets machine-id, which requires running the host). Rejected for
   v0.5.7 as overengineering — the marginal security gain is small,
   the implementation surface is large (key derivation, rotation
   semantics, error paths). Possible future ADR if the threat model
   warrants.

2. **Persist by default, no env-var opt-out.** Simpler. Rejected
   because the inherited behavior was technically correct for some
   users; an env var preserves their choice without code changes.

3. **Bump JWT TTL from 3h to 7d so restart re-login is rare.**
   Doesn't fix the root cause (every upgrade still logs people out;
   long-lived tokens just delay the eventual mass logout). Also
   widens the blast radius of a token leak.

4. **Refresh-token flow that survives restarts.** Real fix, but
   requires UI changes and is a much bigger surface. Worth doing
   eventually for unrelated reasons (mobile clients, app sessions),
   but not as the v0.5.7 hotfix for #176.

## Refresh discipline (per ADR-0019)

- Status `accepted` until threat model changes (e.g. multi-tenant
  PowerLab deploys become a target use case).
- Supersession path: a future ADR proposing encrypted-at-rest
  storage would supersede this one.

## Reference

- Issue #176 (the watchlist issue that triggered this)
- Pre-v0.5.7 inherited code: `backend/user-service/service/user.go`
  (commit `eb0d48c` initial public release)
- Misleading godoc that prompted the "is this our decision?"
  question: same file, comment block at lines 104-113 (commit
  `ce7993ad` godoc Sprint 2 Phase 6)
- Implementation: `backend/user-service/service/keypair_store.go` +
  `pkg/sqlite/migrations/0002_jwt_keypair.sql`
