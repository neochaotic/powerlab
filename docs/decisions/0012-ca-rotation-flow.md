# 0012 — CA rotation: separate destructive action from "Reset trust"

**Status:** accepted
**Date:** 2026-05-07
**Tags:** security, https, ux, v0.3.0

## Context

v0.2.7 had a single "Reset trust" action that cleared the HSTS
gate file. That covers the lighter use case ("the trust dance got
into a weird state, let me redo it") but conflates it with the
much heavier "I need to rotate the CA itself" use case (key leak,
operator handover, scheduled hygiene).

Without separation, the only way to rotate the CA was to drop to
a shell on the host, delete `ca.{crt,key}`, and let the server
regenerate. That's:

- not surfaceable in the UI;
- not auditable;
- racy with the running gateway (it might serve stale leaves);
- inaccessible to users without sudo on the panel host.

We need a UI-surfaced rotation flow that is intentionally HARD to
trigger by accident, since a wrong click voids trust on every
device the user has installed the CA on.

## Decision

Two distinct actions, two distinct endpoints, two distinct
buttons:

### "Reset trust" — light, idempotent

- Endpoint: `DELETE /v1/sys/trust-confirmed`
- Action: removes the HSTS gate file, drops the disarming marker
  for 15 minutes (so browsers also clear their cached pin).
  CA + leaf untouched.
- UI: a small button inside an "Advanced / Recovery" `<details>`
  fold in Settings → Security. Single confirm dialog.
- When to use: the trust dance got tangled, re-run it.

### "Rotate CA" — destructive, two-step confirm

- Endpoint: `POST /v1/sys/rotate-ca?confirm=ROTATE_CA`
- Guards (defense in depth):
  - Must be HTTPS (`r.TLS != nil`) — proves the caller has the
    OLD CA installed and trusted.
  - Must come from a non-localhost peer (same logic as
    `/trust-confirmed`).
  - Must include the literal `confirm=ROTATE_CA` query param.
- Action: writes current `ca.{crt,key}` aside as `ca.{crt,key}.previous`
  (audit trail), generates a fresh CA + leaf, refreshes the
  public-backup file, drops the disarming marker so HSTS clears
  on next visit. Does NOT delete the previous material — admin
  with shell access can roll back.
- UI: a separate, rose-tinted button in the same Recovery fold.
  Opens a full modal with:
  - Big amber AlertTriangle icon
  - Bullet list of consequences ("Every device must re-install")
  - "When should you rotate?" — short list of legit reasons + a
    bold reminder that routine maintenance does NOT need this
  - Type-to-confirm input ("ROTATE", uppercase)
  - Disabled "Rotate now" button until the input matches exactly
- When to use: CA private key leaked, panel handed off to a new
  operator, scheduled hygiene rotation.

## Rationale

- **Two actions, two prompts** stops users from doing the
  destructive thing when they meant the recovery thing. Confusing
  the two voids trust on every device — very high cost for a
  near-zero benefit (no one means "rotate" when they say
  "reset").
- **Type-to-confirm** is dramatic but appropriate: the action
  has household-wide blast radius. A muscle-memory "click yes"
  pattern would be too easy.
- **Preserve `.previous`** files lets a panicked admin recover
  ("I clicked the wrong button"). 1-line shell command:
  `mv ca.crt.previous ca.crt && mv ca.key.previous ca.key &&
  systemctl restart powerlab-gateway`.
- **Same HTTPS + non-localhost guards as `/trust-confirmed`**
  keeps the security posture consistent. Anyone who can rotate
  the CA must already trust the current CA — preventing a
  drive-by from a hostile LAN peer who hasn't completed the
  trust dance.

## Alternatives considered

- **Single "Reset" button with a checkbox: "Also rotate the CA"**.
  Rejected: hides the destructive action behind a single click +
  a checkbox most users won't read. UI gravity should match
  blast radius.

- **Rotate-CA only via shell**. Rejected: that's the status quo
  and fails users who don't have sudo or don't want to ssh in.

- **Auto-rotate every N years**. Rejected for v0.3.0: forced
  rotation generates exactly the lockout this doc tries to
  prevent. If we ever auto-rotate, it should be opt-in + give
  the user N weeks of dual-CA serving (cross-signed leaves) so
  devices migrate gracefully. Tracked separately.

## Consequences

- The Settings → Security page grows a "Recovery" expandable
  section. Users who never open it never see either button.
- The Rotate modal is intentionally jarring; we accept that it
  feels heavy, because the alternative is a click that voids
  trust silently.
- We own the responsibility of preserving the `.previous` files
  forever (or until manual cleanup); they take ~3KB each.
- Future cross-signing extension can layer onto this without
  changing the rotate semantics: instead of replacing the CA
  outright, the new CA is signed by the old one for N weeks of
  dual trust.

## References

- `backend/common/pkg/security/cert.go` — `RotateCA`,
  `DisarmHSTS`
- `backend/gateway/route/security_route.go` — `handleRotateCA`,
  `handleTrustConfirmed` (DELETE branch)
- `ui/src/routes/settings/+page.svelte` — Recovery fold + rotate
  modal
- ADR 0011 — disarming window that handles browser-side HSTS
  cleanup post-rotation
