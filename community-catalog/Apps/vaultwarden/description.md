# Vaultwarden

Vaultwarden is an unofficial, lightweight Bitwarden server written in Rust. Compatible with all official Bitwarden clients (browser extension, mobile, desktop). Full-featured password manager with vault sharing, TOTP, and attachments.

**First steps:** Open `http://<host>:10380` and register the first user — that account is not automatically an admin. Access the admin panel at `/admin` using the `ADMIN_TOKEN` environment variable.

**Data:** `/DATA/PowerLabAppData/vaultwarden/` — SQLite database with encrypted vault entries, attachments, and user accounts.
