# Lockout recovery

PowerLab UI unreachable, services flaky, host unresponsive — this page is the
operator's escape hatch. Print it, save it offline, send it to whoever inherits
the box.

## TL;DR triage

| Symptom | Most likely | Skip to |
|---|---|---|
| UI loads but spinning forever | Backend service crashed | [Service down — UI still reachable](#service-down--ui-still-reachable) |
| UI doesn't load, browser says "connection refused" | Gateway down | [Gateway down — UI unreachable](#gateway-down--ui-unreachable) |
| UI doesn't load AND SSH works | Probably gateway or routing | [Gateway down — UI unreachable](#gateway-down--ui-unreachable) |
| Box pings but SSH refused | Gateway+SSH both down | [Box pings, SSH dead](#box-pings-ssh-dead) |
| Box doesn't ping | Host off | [Host unreachable](#host-unreachable) |
| Just upgraded and now nothing works | Bad release | [Rollback after bad upgrade](#rollback-after-bad-upgrade) |

## Service down — UI still reachable

If the Settings → Power pane loads but a service shows **failed**:

```bash
# Via SSH
ssh root@<host>
journalctl -u powerlab-<service> -n 200 --no-pager
systemctl status powerlab-<service>
systemctl restart powerlab-<service>
```

Common service names: `powerlab-gateway`, `powerlab-app-management`,
`powerlab-core`, `powerlab-user-service`, `powerlab-local-storage`,
`powerlab-message-bus`.

If `systemctl restart` brings it back, you're done. If it crashes again:

```bash
journalctl -u powerlab-<service> -n 500 --no-pager | grep -iE 'panic|error|fatal'
```

Capture the output before destroying state. File an issue with this excerpt.

## Gateway down — UI unreachable

The gateway is what serves the UI (embedded in the gateway binary —
ADR-0043) on `http://<host>:80` (or `:8765`). If it's down, no UI. Try
SSH first:

```bash
ssh root@<host>
systemctl status powerlab-gateway
journalctl -u powerlab-gateway -n 200 --no-pager
```

If status shows `inactive (dead)` or `failed`, restart it:

```bash
systemctl restart powerlab-gateway
sleep 3
systemctl status powerlab-gateway
```

If it restarts but immediately crashes (`Restart=always` then enters
`StartLimitBurst` failure):

```bash
# Reset failure state and try once more
systemctl reset-failed powerlab-gateway
systemctl start powerlab-gateway
journalctl -u powerlab-gateway -n 100 --no-pager
```

If the binary itself is broken (post-upgrade), see
[Rollback after bad upgrade](#rollback-after-bad-upgrade).

## Box pings, SSH dead

If `ping <host>` succeeds but `ssh root@<host>` refuses:

- **SSH service crashed**: needs physical/IPMI access. No way to recover from a
  remote-only position. Lesson for next time: enable IPMI.
- **Firewall blocking SSH**: check from local console. If you can get into the
  console, `iptables -L INPUT` and look for DROP/REJECT rules.

PowerLab does NOT modify the host's SSH config; if SSH is broken it's a host
admin issue, not a PowerLab issue.

## Host unreachable

`ping <host>` fails. The box is off, network is down, or DNS is broken.

| Recovery path | Requires |
|---|---|
| Physical power button | Walk to the box |
| IPMI / iDRAC / iLO | Enterprise hardware + IPMI configured before incident |
| Wake-on-LAN | NIC + BIOS support + WoL enabled before incident + magic packet sender on same LAN |
| Smart plug / PDU | Smart power strip wired before incident |
| Network outage | Wait + check router |
| DNS | Try IP directly |

If the host was **shut down** (not just rebooted) and you have none of the
above, you are locked out. The Settings → Power pane hides Shutdown by default
specifically because of this trap. If you enabled it and clicked it
remotely without IPMI/WoL/physical access — sorry.

## Rollback after bad upgrade

A new release binary loops on startup. As of v0.7.x PowerLab does NOT keep
previous binaries on disk for automatic rollback. Manual procedure:

```bash
# Identify the previous version you were on
ls /var/log/powerlab/powerlab-install.log 2>/dev/null
journalctl -u powerlab-gateway --since "1 hour ago" | grep -i version

# Download the previous release tarball
PREV_VERSION="v0.6.16"  # adjust to the last known good
cd /tmp
curl -fsSL "https://github.com/neochaotic/powerlab/releases/download/$PREV_VERSION/powerlab-linux-amd64.tar.gz" -o powerlab-prev.tar.gz
tar -xzf powerlab-prev.tar.gz -C /tmp/powerlab-prev

# Stop services, swap binaries, restart
systemctl stop 'powerlab-*'
cp /tmp/powerlab-prev/bin/powerlab-* /usr/bin/
systemctl daemon-reload
systemctl start powerlab-gateway powerlab-message-bus
sleep 5
systemctl start powerlab-app-management powerlab-core powerlab-user-service powerlab-local-storage
systemctl status 'powerlab-*'
```

Open an issue with the failing release version + journalctl output before
trying the upgrade again.

## What you should set up BEFORE an incident

1. **SSH access verified** to the PowerLab host from at least one device that
   isn't the one running PowerLab. Test it now.
2. **One out-of-band power method** (any of):
   - Physical button + presence
   - IPMI / iDRAC / iLO if enterprise hardware
   - Wake-on-LAN enabled in BIOS + tested via `wakeonlan <mac>` from another
     LAN device (NOT from the PowerLab host itself)
   - Smart plug / managed PDU
3. **PowerLab Settings → Power → Shutdown** stays disabled UNLESS the above is
   true. Re-tick the opt-in only on a per-device basis.
4. **One offline copy** of this doc.
5. **Recovery contact** — if PowerLab runs unattended, someone with physical
   access who knows enough to follow the rollback steps.

## What the platform does to help

| Defence | Behaviour |
|---|---|
| `Restart=always` on all 6 systemd units | Crashed service auto-restarts |
| `RestartSec=5` | 5s delay before restart, avoids tight loops |
| `StartLimitBurst=10 / StartLimitIntervalSec=60` | After 10 crashes in 60s systemd stops trying — needs manual `systemctl reset-failed` + `start` |
| `Wants=` / `After=` chain | Services start in dependency order |
| `ExecStartPre` URL-file wait | Dependent service waits up to 30s for its upstream's signal file |
| Settings → Power shutdown opt-in (#260) | Hidden by default per browser |
| Gateway restart delayed-exec | Backend returns 200 before triggering its own restart (avoids stuck UI) |

## Reporting

When filing an issue about a lockout, include:

- Output of `systemctl status 'powerlab-*' --no-pager`
- Last 200 lines of `journalctl -u powerlab-gateway -n 200 --no-pager`
- Tarball version you upgraded FROM and TO (if applicable)
- Whether the box was physically reachable when the incident started
