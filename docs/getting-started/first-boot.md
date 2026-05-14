# First boot

After install, browse to the URL the script printed (`http://<host>:8765` by default). You'll land on the SetupWizard.

## SetupWizard

Three short steps:

1. **Pick a username.** Used for login + as the display name in the panel.
2. **Pick a password.** Stored as a bcrypt hash in `user.db`. Don't lose this — there's no email-recovery flow.
3. **Confirm time + locale.** Auto-detected from the OS; you can override.

The wizard creates the admin account, persists it, and redirects to the dashboard.

## Optional: PAM-backed login (Linux/amd64 only)

If your Linux user account already has a password and you'd prefer not to maintain a separate one, PowerLab can authenticate against PAM (the same source that gates `sudo` and SSH). This requires the production install on Linux/amd64 (CGO + libpam linkage; not available on arm64 or macOS).

Set in `/etc/powerlab/user-service.conf`:

```ini
[auth]
PAMService = login    # or "su", "sshd" — anything in /etc/pam.d/
```

Restart `powerlab-user-service`. The SetupWizard becomes a "PAM detected" notice instead of a password form. You log in with your OS user/password from then on.

## Optional: HTTPS

By default PowerLab serves HTTP on the chosen port. To enable HTTPS with a host-trusted cert (no browser warnings, even for `https://<lan-ip>:port`):

See **[HTTPS setup](../HTTPS.md)** — a multi-step "trust dance" that installs PowerLab's local CA into your devices' trust stores.

> HTTPS is **disabled by default in v0.5.x** per ADR-0007 + a v0.5.2 polish that simplified onboarding for first-time users. Re-enable it explicitly in `/etc/powerlab/gateway.ini`.

## What's running after boot

Six systemd units handle different concerns. The [Architecture → Service topology](../architecture/topology.md) page maps each one in detail. Quickly:

- **gateway** — single HTTP entry point; reverse-proxies to the others.
- **app-management** — Docker compose orchestration.
- **core** — port discovery, system settings, samba shares.
- **user-service** — login, JWT signing, sessions.
- **message-bus** — event pub/sub between services.
- **local-storage** — disk inventory, merge points, mount metadata.

Logs land in `/var/log/powerlab/<service>.log`. Use `journalctl -u powerlab-<service>` for a tail.

## What to do next

- **Install your first app** — Apps tab → browse the store → click install. Labeled with `io.powerlab.v1.kind=app` so it never collides with a CasaOS install.
- **Mount external storage** — Storage tab → Disks. PowerLab discovers attached drives + offers a one-click mount.
- **Set up backups** — See [Backup and restore](../operations/backup-restore.md) for the snapshot strategy and recovery flows.
- **Try the AI chat** *(coming soon)* — bottom-right widget. Brings up a chat backed by your configured LLM (defaults to local Ollama if available).
