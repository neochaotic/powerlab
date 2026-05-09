# Troubleshooting

Common errors → root cause → fix matrix. Pulled from production
reports across v0.2.x, v0.3.x, and the Sprint 1 audit.

## Quick triage

When something is wrong, in this order:

1. **Check the correlation ID.** If a UI toast or log line shows
   `correlation_id=<32-hex>`, grep across all six service logs:
   `journalctl -u powerlab-* | grep <id>`. That reconstructs the
   full request path.
2. **Check service status.** `systemctl status powerlab-gateway`
   first; if it's down, nothing else matters. Then check the
   downstream services individually.
3. **Check recent logs.** `journalctl -u powerlab-gateway -n 100`
   — the gateway's structured logs include caller file:line and the
   correlation ID for every recent request.

## HTTPS / Trust

| Symptom | Likely cause | Fix |
|---|---|---|
| Browser shows "Your connection is not private" | CA cert not yet trusted on this device | Settings → Security → download `.crt` (or `.mobileconfig` on Apple, `.cer` on Windows). Install per the on-screen walkthrough |
| Chrome blocks `.crt` download with "Insecure download blocked" | Chrome's high-risk-file rule fires on HTTPS-with-untrusted-cert | Use the **"Open via HTTP"** button — opens the same download in an HTTP tab, bypassing the rule |
| `.mobileconfig` shows "Unverified" on iOS | CA isn't installed yet (catch-22 — install IS what user is doing) | Proceed; the profile installer is what makes the CA trusted, then "Verified" sticks for re-installs |
| Verify (Test) button does nothing | Already-on-HTTPS path: same URL, no redirect needed | A success toast should appear distinguishing "redirect" from "already-secure-noop" (fixed in v0.3.2 #7) |
| HTTPS works on one device but not another | Each device needs the CA installed independently | Trust is per-device. Repeat install on each machine |
| Reset trust then redo dance, still warns | `localStorage` flag stuck | Settings → Security → Reset trust button clears both server-side gate and client-side cached fingerprint |
| HTTPS warns even after correct install | Cert was rotated since install | Re-download the new cert. Settings → Security → check expiry; rotate if needed |

## Updater / Versioning

| Symptom | Likely cause | Fix |
|---|---|---|
| "Intermediate release first" error | Current binary is `vdev` (no `-X POWERLAB_VERSION=...` ldflag — dev/CI/source build) | Use the shell-install fallback (`curl ... \| bash`) — the updater no longer rejects vdev as of v0.3.1 (#55) |
| Update button does nothing | Manifest fetch failed; check connectivity | DevTools → Network for `manifest.json` request; verify GitHub releases reachable |
| After update, services in restart loop | Failed health-check during upgrade | install.sh auto-restored from the snapshot. Check `/var/lib/powerlab/last-upgrade.json` for `failed_at` and `snapshot_path` |
| Services keep using old binary | systemd didn't pick up new files | `systemctl daemon-reload` then `systemctl restart powerlab-*` (install.sh does this; manual fix only if you replaced binaries by hand) |

## Apps / Compose

| Symptom | Likely cause | Fix |
|---|---|---|
| Apps disappear after page refresh | JWT not yet rehydrated when fetch fires | Fixed in v0.3.2 — auth rehydrates synchronously at module init. If recurring, hard-refresh (Ctrl+Shift+R) to clear stale bundle |
| "All predefined address pools have been fully subnetted" on install | Docker default 15-subnet pool exhausted | Run `docker network prune` (UI surfaces this); long-term, expand `default-address-pools` in `/etc/docker/daemon.json` |
| Custom App: "there are ports in use" on edit + redeploy | Frontend hits Install endpoint instead of UpdateSettings (#65) | Workaround today: stop the app first (don't delete config), then redeploy. Fix scheduled for Sprint 4 (app-management kill) |
| Install progress bar invisible | UI was ignoring SSE "Phase N/M" markers | Fixed in v0.3.2 — bar now visible above live log |
| `/v2/app_management/compose` returns 500 with no message | Backend used `errors.New(...)` — generic 500 | Once Sprint 4 lands the app-management kill with `pkg/errors`, every error becomes structured JSON with code + i18n key |

## Files

| Symptom | Likely cause | Fix |
|---|---|---|
| Click on file opens new tab instead of editor | File-extension whitelist was too narrow | Fixed in v0.3.2 — handler is positive-by-default (directory/media/large/else routes correctly) |
| Save toast invisible | Toast container z-50 vs editor modal z-100 | Fixed in v0.3.2 — toast bumped to z-200 |
| No Delete button visible | Delete only shows when items are selected; selection requires Cmd-click or right-click | UX gap tracked in #66 (sprint-4 — checkbox column) |
| Editor opens but text area is inert | `initEditor()` ran while `loading=true` (CodeMirror failed to attach) | Fixed in v0.2.6; if recurring, F12 → Console for errors and report on #57 |
| Files: `?token=` in URL when downloading media | JWT exposed in URL — privacy leak | Tracked in #35; fix moves to JWT cookie. Sprint 2 |
| Logged-in user can read `/etc/shadow` via files | Per-user scope sandbox not enforced | Tracked in #36 — Sprint 2 (security-critical) |

## Networking / mDNS

| Symptom | Likely cause | Fix |
|---|---|---|
| `powerlab.local` doesn't resolve on Linux | mDNS announcement not hitting Avahi correctly | Tracked in #33 — Sprint 1, dies in gateway rewrite |
| HTTPS cert SAN missing Tailscale hostname | Cert generation didn't query Tailscale | Tracked in #44 — Sprint 1, dies in gateway rewrite |
| Gateway crashed with SIGSEGV during config reload | `checkURL` inverted condition + nil-deref (#64, CasaOS legacy) | Mitigated by panic recovery middleware once Sprint 1 part 3 lands; structurally closed in gateway rewrite |

## Process / OS

| Symptom | Likely cause | Fix |
|---|---|---|
| `systemctl status powerlab-*` shows `failed` | Check `journalctl -u powerlab-<svc> -n 100` for the actual error | Most common: dependency on `message-bus` not yet up — check boot order in topology.md |
| Install fails on Fedora/openSUSE/Arch | Install.sh suggests `apt-get` only (pre-v0.4.0) | install.sh distro-aware lands in v0.4.0 — `apt-get`/`dnf`/`zypper`/`pacman` per host |
| Two web panels (PowerLab + CasaOS) on same host | Coexistence is allowed but Docker labels overlap today (#85) | Sprint 4 isolates Docker labels and AppData paths |

## Debug levels

If the default INFO-level logs aren't enough to diagnose:

```bash
# Set on the gateway service unit (or any service)
sudo systemctl edit powerlab-gateway
# Add:
[Service]
Environment="POWERLAB_LOG_LEVEL=debug"
sudo systemctl restart powerlab-gateway
```

Levels: `debug` · `info` (default) · `warn` · `error`. Format:
`POWERLAB_LOG_FORMAT=json` (machine-readable, default for prod) or
`console` (human-readable, default for dev).

## Reporting bugs

When opening an issue, include:

1. **Correlation ID** if you saw one (toast / error response /
   `X-Request-Id` header on a failing request).
2. **Service status:**
   `systemctl status powerlab-gateway powerlab-app-management ... | head -50`
3. **Recent logs:**
   `journalctl -u powerlab-* -n 200 --no-pager`
4. **Version:** `cat /etc/powerlab/version`
5. **Distro family:** `cat /etc/os-release | grep ^ID=`

A correlation ID alone makes triage 10x faster.
