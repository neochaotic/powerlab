// Service Worker for PowerLab.
//
// CURRENT STRATEGY — registers but does NOT intercept fetches.
//
// PWA install criteria require a registered service worker (so the
// browser knows the page can run as an app). Beyond that, this SW
// does nothing — there is no `fetch` handler, so every request goes
// straight to the network with no caching, no offline support, no
// interception.
//
// We deliberately removed the previous passthrough fetch handler in
// v0.2.5 because it broke vite dev (intercepted SPA navigations and
// surfaced "Failed to fetch" errors). A real cache strategy lives
// in the v0.2.8+ roadmap with proper cross-version invalidation
// (see docs/decisions/0005-pwa-scaffolding-no-cache-yet.md).
//
// If you're tempted to add a fetch handler here: stop and read
// the ADR first.

self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(clients.claim());
});

// NO fetch handler. Intentional — see header comment above.
