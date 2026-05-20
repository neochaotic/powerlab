# SQLite Browser

DB Browser for SQLite (SQLiteBrowser) is a high-quality visual tool for creating, editing, and browsing SQLite databases. This containerized version exposes the full desktop app via a browser-based VNC interface.

**First steps:** Open `http://<host>:3000` — the SQLiteBrowser desktop loads in your browser. Mount your `.db` files as volumes to open them.

**Data:** `/DATA/PowerLabAppData/sqlitebrowser/` — SQLite database files you bind-mount into the container.
