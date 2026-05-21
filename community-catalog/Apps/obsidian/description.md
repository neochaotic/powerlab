# Obsidian

Obsidian LiveSync Server provides a self-hosted sync backend for the Obsidian markdown note-taking app. It uses CouchDB under the hood to replicate your vault across all your devices without touching a third-party cloud.

**First steps:** Open `http://<host>:15323` (CouchDB Fauxton UI) to verify the server is up. Then configure Obsidian LiveSync plugin to point at `http://<host>:15323` with your credentials.

**Data:** `/DATA/PowerLabAppData/obsidian/` — CouchDB database with your Obsidian vault replicas.
