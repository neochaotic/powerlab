# 0005 — PWA scaffolding in v0.2.7 (no caching strategy yet)

**Status:** accepted
**Date:** 2026-05-07
**Tags:** ux, pwa, v0.2.7

## Context

Once HTTPS lands (v0.2.7), the panel becomes eligible to be installed
as a PWA — "Add to Home Screen" on iOS / Android / Desktop, app icon,
fullscreen window without browser chrome.

PWA install requires three things:

1. HTTPS (covered by the rest of v0.2.7).
2. A `manifest.webmanifest` describing the app.
3. A registered Service Worker.

We deliberately removed our previous Service Worker in v0.2.5 because
it was a passthrough (`fetch(event.request)` and nothing else) that
intercepted SPA navigations under vite dev and surfaced spurious
"Failed to fetch" errors. We do NOT yet have a real offline-cache
strategy, and we don't want to ship one half-baked.

## Decision

Ship PWA scaffolding in v0.2.7:

- `ui/static/manifest.webmanifest` with `start_url`,
  `display: standalone`, `theme_color`, icons.
- 192px and 512px PNG icons (we already have the source SVG from the
  v0.1.5 favicon work).
- A Service Worker that registers but **has no `fetch` handler** —
  it satisfies the PWA install criteria without intercepting
  anything. Zero caching, zero offline support, zero failure modes.
- iOS-specific `<meta>` tags in `app.html`
  (`apple-mobile-web-app-capable`, `apple-touch-icon`, etc).

A real caching strategy (offline shell, background sync, etc) lands
in a separate issue tracked as v0.2.8+.

## Rationale

- **Half a day of extra work** in v0.2.7 (manifest + icons + 10-line
  SW + meta tags) unlocks a major UX delta: app icon on the user's
  home screen, fullscreen window, badge support eventually.
- Internal positioning: "PowerLab is your home server" vs "PowerLab
  is a website you visit". The home-screen icon is the difference.
- Since we already broke the SW in v0.2.5, users see the registration
  toggle on/off across releases. Re-enabling alongside HTTPS is the
  natural moment.
- Skipping caching avoids the v0.2.5 regression coming back. A SW
  that doesn't intercept fetch can't break fetch.

## Alternatives considered

- **Defer entirely to v0.2.8**. Rejected: the half-day cost is
  trivial; the UX win is large; HTTPS without PWA is "we have HTTPS"
  while HTTPS with PWA is "we have a real app".
- **Ship a real caching strategy now**. Rejected: caching is the
  most error-prone part of PWAs (stale assets, race with deploy,
  service-worker update flow). We need the version-handshake banner
  (already shipped in v0.2.6) to mature first, then layer caching
  on top in v0.2.8 with proper invalidation.
- **Use a library** (Workbox, etc). Rejected for the no-op SW —
  overkill. Will reconsider when we land real caching.

## Consequences

- The PWA install prompt shows up on supported browsers. We have to
  make sure the install experience matches the rest of the panel
  (matching theme color, correct app name).
- If we ever ship a real `fetch` handler in the SW, it must respect
  the version handshake — i.e., serve cached assets only when the
  cached `__APP_VERSION__` matches what the backend reports. Otherwise
  we get cross-version stale-bundle issues that v0.2.6 specifically
  fixed.
- The SW registers from v0.2.7 onward; users on v0.2.6 won't have
  one. The `+layout.svelte` registration code stays simple — guard
  with `'serviceWorker' in navigator` before calling `register`.

## References

- Issue [#43](https://github.com/neochaotic/powerlab/issues/43) — v0.2.7
  milestone, "Open question" #4.
- v0.2.5 SW removal commit: `fix(app-management): register
  /compose/:id/disk-usage + drop dead SW` — context for why the
  previous SW was bad.
- ADR 0001 (cert validity) and ADR 0006 (HSTS gate) — the rest of
  v0.2.7 that has to land before PWA is meaningful.
