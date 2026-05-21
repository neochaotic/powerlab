# Jackett

Jackett is a proxy server that translates the Torznab/Newznab API used by Sonarr/Radarr into searches on hundreds of private and public torrent sites. It handles login cookies, CAPTCHA solving (via FlareSolverr), and result normalization.

**First steps:** Open `http://<host>:9117` and add your torrent indexers. Copy the Torznab feed URL into Sonarr/Radarr.

**Data:** `/DATA/PowerLabAppData/jackett/` — Indexer configuration, cookies, and API keys.
