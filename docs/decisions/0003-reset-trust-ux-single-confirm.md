# 0003 — Reset-trust UX: single confirm with device list

**Status:** accepted
**Date:** 2026-05-07
**Tags:** ux, security, https, v0.2.7

## Context

The user can revoke the local CA and regenerate it from
Settings → Security ("Reset trust"). This is a destructive action:

- Every device that previously installed the CA loses trust.
- Every device has to re-install the new CA.
- Existing browser sessions stay logged in until the next reload.

We need to choose how forceful the confirmation flow is. Two main
options on the table for v0.2.7:

| Option | Friction | Use case |
|---|---|---|
| Single confirm dialog | Low | "Internal users, single admin, advanced setting" |
| Typed confirmation ("DESTROY", or the device's hostname) | High | "Public deployments, multi-user, prevent shared-screen accidents" |

## Decision

Single confirm dialog, but the dialog's body **lists the devices that
will need to re-install the CA** (read from the audit log of CA
downloads).

```
┌──────────────────────────────────────────────────────────────┐
│  Regenerate Local CA?                                        │
│                                                              │
│  This will invalidate trust on every device that previously │
│  installed the PowerLab CA. You'll need to re-install on:    │
│                                                              │
│   • This browser                                             │
│   • 3 other devices we know about (last seen 2h ago)         │
│                                                              │
│  Existing sessions stay logged in until they reload.         │
│                                                              │
│            [ Cancel ]    [ Regenerate Now ]                  │
└──────────────────────────────────────────────────────────────┘
```

## Rationale

- For initial deployment (ADR 0007: internal LAN only), the user is
  a single admin running a homelab. The downside of an accidental
  click is "I have to re-install the CA on my phone and my laptop" —
  measured in minutes, not hours.
- A typed-DESTROY pattern is theatrical for this audience. It signals
  "this is for shared production environments where deletes are
  fatal" — which doesn't match the deployment context.
- Showing the device list does the real work of confirmation: the
  user reads "5 devices including my parents' iPhones" and naturally
  pauses, vs. a typed confirmation that gets muscle-memoried into
  habit.
- Cleaner Settings → Security flow: the destructive path is a single
  modal away, but it tells you exactly what you're destroying.

## Alternatives considered

- **Typed "DESTROY"**. Rejected for v0.2.7: theatrical for internal
  deployment. Reconsidered if/when PowerLab grows to multi-user
  shared deployments (ADR will be superseded).
- **Typed device hostname** ("type 'm900' to confirm"). Rejected:
  the same friction as DESTROY without the recognisable pattern.
  Users would just copy/paste the hostname from the same modal.
- **Two-step confirm** (first dialog: "Are you sure?", second dialog:
  "Really sure?"). Rejected: classic "click-Through-syndrome" — the
  second dialog gets habituated on the first day.
- **Email/SMS confirmation code**. Rejected: PowerLab doesn't have a
  configured outbound channel; we'd have to introduce SMTP config
  just for this one flow.

## Consequences

- The destructive path stays one click away — admin needs to apply
  judgment.
- We commit to maintaining an audit log of CA-cert downloads (UA + IP
  + timestamp) so the device-list shown in the dialog is real, not
  guessed.
- If PowerLab's audience shifts to multi-user / shared deployments
  (e.g., post-multi-tenant work in v0.3+), this ADR is superseded by
  a stricter UX — typed confirmation makes more sense at that point.

## References

- Issue [#43](https://github.com/neochaotic/powerlab/issues/43) — v0.2.7
  milestone, "Open question" #2.
- ADR [0007](./0007-internal-network-only-initial-deployment.md) — the
  scope assumption this ADR rests on.
- GitHub's repository-delete flow uses typed-confirmation; that's the
  reference for the pattern we're rejecting at this stage.
