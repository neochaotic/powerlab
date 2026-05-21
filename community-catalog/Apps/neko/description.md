# Neko

Neko runs a full browser desktop (Firefox, Chromium, etc.) inside Docker and serves it via WebRTC — enabling low-latency, synchronized viewing for multiple users. Perfect for watch parties or shared browsing sessions.

**First steps:** Open `http://<host>:8080` — enter the room password to join (set via `NEKO_PASSWORD` env var).

**Data:** `/DATA/PowerLabAppData/neko/` — Browser profile and session state.
