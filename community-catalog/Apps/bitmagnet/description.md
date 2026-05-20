# Bitmagnet

Bitmagnet is a self-hosted BitTorrent indexer and search engine. It crawls the DHT network, classifies torrents using built-in metadata enrichment (TMDB, TVDB), and exposes a full-text search UI and GraphQL API. Think of it as your private torrent search engine.

**First steps:** Open `http://<host>:3333` after the initial DHT crawl populates data (allow a few minutes).

**Data:** `/DATA/PowerLabAppData/bitmagnet/` — PostgreSQL database with torrent metadata, classifications, and crawl state.
