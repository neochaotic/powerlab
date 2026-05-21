# Prowlarr

Prowlarr is the indexer manager for the *arr stack. It centralizes indexer configuration and syncs to Sonarr, Radarr, Lidarr, and Readarr automatically — so you configure each indexer once instead of once per *arr app.

**First steps:** Open `http://<host>:9696` → Settings → Indexers to add sources. Then sync to your *arr apps via Settings → Apps.

**Data:** `/DATA/PowerLabAppData/prowlarr/` — SQLite database with indexer configuration and sync state.
