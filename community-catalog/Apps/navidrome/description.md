# Navidrome

Navidrome is a fast, lightweight music streaming server compatible with Subsonic and Airsonic clients (DSub, Symfonium, Sonixd, etc.). Scans your music library, reads tags, and serves album art with a built-in web player.

**First steps:** Open `http://<host>:4533` — default login is `admin` / `admin`. Change in User Settings.

**Default login:** `admin` / `admin` — change immediately after first login.

**Data:** `/DATA/PowerLabAppData/navidrome/` — Music library (bind-mounted), SQLite database with scan cache and user data.
