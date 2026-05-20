# FlareSolverr

FlareSolverr is a proxy server that bypasses Cloudflare's anti-bot protection using a headless browser. Used as a backend by Jackett, Prowlarr, and other indexer tools when the indexer sits behind Cloudflare.

**API endpoint:** FlareSolverr is a backend service — configure Jackett or Prowlarr to use `http://<host>:8191` as the FlareSolverr URL. No browser UI.

**Data:** `/DATA/PowerLabAppData/flaresolverr/` — Stateless — no persistent data.
