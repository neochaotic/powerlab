# Sonarr

Sonarr is an automated TV show collection manager. It monitors episode air dates, grabs downloads from configured indexers when episodes are available, and renames/organizes your library using custom naming templates.

**First steps:** Open `http://<host>:8989` → Settings → Media Management to set your TV root folder, then add indexers and download clients.

**Data:** `/DATA/PowerLabAppData/sonarr/` — SQLite database with show/episode metadata, download history, and library index.
