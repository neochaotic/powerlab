# 0032. mDNS Strategy (Avahi with Fallback)

- **Status:** accepted
- **Date:** 2026-05-14

## Context

PowerLab is designed to be easily discoverable on a local network (LAN) without requiring users to memorize IP addresses. To achieve this, the gateway broadcasts the host's presence using Multicast DNS (mDNS), allowing users to reach the UI at `http://powerlab.local` (or a custom hostname).

However, Linux distributions vary wildly in how they handle mDNS. Many standard distributions (like Debian/Ubuntu) run Avahi. Conflicts arise if multiple processes try to bind to the standard mDNS port (5353) or if Avahi is running and blocking direct UDP multicast.

## Decision

We adopt a **tiered mDNS broadcasting strategy** in the gateway:

1. **Avahi Preferred:** The gateway first checks if the `avahi-daemon` is available via D-Bus. If it is, the gateway registers its services directly with Avahi, acting as a well-behaved Linux citizen and letting the OS manage the multicast traffic.
2. **Direct Multicast Fallback:** If Avahi is not present or unreachable, the gateway falls back to a direct, embedded mDNS implementation (using standard UDP multicast on port 5353).

## Consequences

- **Positive:** High reliability for LAN discovery across a wide range of Linux environments (from full-fat Ubuntu to barebones Alpine).
- **Positive:** Avoids port collision errors on systems where the OS already claims port 5353.
- **Negative:** Increased complexity in the gateway's network discovery code, as it must maintain and coordinate two separate broadcasting mechanisms.
