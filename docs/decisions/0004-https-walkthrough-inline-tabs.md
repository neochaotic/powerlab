# 0004 — HTTPS walkthrough: inline 4-tab page (not wizard)

**Status:** accepted
**Date:** 2026-05-07
**Tags:** ux, https, v0.2.7

## Context

The Settings → Security page guides the user through installing the
PowerLab CA on each of their devices. There are two reasonable UI
patterns:

- **Inline 4-tab page** — all four OS walkthroughs (iOS, macOS,
  Android, Linux+Windows) are visible at once via tabs at the top.
  Each tab is a vertically-scrollable list of steps with screenshots.
  The default-active tab is the one matching the user's UA.

- **Step-by-step wizard** — Next/Back buttons. One step per screen.
  Forces the user to advance linearly.

## Decision

Inline 4-tab page. Tab default-selected by `detectOS()` from
`navigator.userAgent`. User can switch tabs.

## Rationale

- PowerLab's audience runs homelab boxes. They install CAs and edit
  config files for fun. They have baseline technical fluency that
  rewards "all the steps visible" UX.
- Letting the user **see** all the steps before starting reduces
  anxiety ("how long is this going to take?") and lets them mentally
  plan ("ok, I need to enter my passcode in step 5").
- Many users install on multiple devices in one sitting (their
  phone + their laptop). Inline tabs let them flip between two OS
  walkthroughs side-by-side without losing place.
- A wizard with Next/Back forces linearity that gets in the way of
  power users who already know what they're doing — they'd skip
  steps but the wizard wouldn't let them.

## Alternatives considered

- **Step-by-step wizard**. Rejected for the audience reasons above.
  Worth reconsidering if/when PowerLab targets a less-technical
  consumer audience (post-1.0).
- **A single "do everything for me" button** (e.g., generate a QR,
  user scans, magic). Rejected: there's no way to remotely complete
  the OS-level "trust the CA" step on iOS/macOS — Apple's profile
  install flow requires the user to physically open Settings and
  toggle a trust switch. The walkthrough has to exist.
- **Video tutorial embedded**. Deferred: nice-to-have but a screenshot
  walkthrough is faster to consume and easier to maintain (no
  re-record when iOS UI changes).

## Consequences

- Settings → Security page is denser than a wizard would be. We
  rely on visual hierarchy (collapsible sections, clear headings,
  screenshot thumbnails) to keep scannability.
- Each tab needs its own screenshots — the junior brief in #43
  itemises this as a separate deliverable (`ui/static/docs/security/`).
- If user testing post-launch shows that newcomers struggle, we can
  add a "Walk me through it" button that overlays a wizard-style
  step counter on top of the inline view, without scrapping the
  underlying structure.

## References

- Issue [#40](https://github.com/neochaotic/powerlab/issues/40) — UX
  spec, "Open question" #3.
- Issue [#43](https://github.com/neochaotic/powerlab/issues/43) — v0.2.7
  milestone.
