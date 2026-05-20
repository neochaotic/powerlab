# `powerlab-logs` — diagnostic CLI reference

`powerlab-logs` is the survival-grade diagnostic surface for a
PowerLab host. It exposes systemd-journal, Docker-container and
install/upgrade logs **without depending on any running PowerLab
daemon**. When something is on fire — gateway crashing, core in a
loop, install.sh half-done — this is the binary the operator
SSHs in and runs.

The companion design ADR is
[`docs/decisions/0027-log-aggregation-service.md`](../decisions/0027-log-aggregation-service.md).
The architecture overview is
[`docs/architecture/log-aggregation.md`](../architecture/log-aggregation.md).

## Where it lives

`/usr/bin/powerlab-logs` — installed by `install.sh` alongside the
six PowerLab services. Cross-compiled from `backend/logs-cli/`
during `scripts/package-linux.sh`.

## Subcommands

```
powerlab-logs journal [--service NAME] [--follow] [--lines N] [--json] [--no-color]
powerlab-logs app <name>   [--follow] [--tail N] [--timestamps]
powerlab-logs install      [--list]
powerlab-logs help
```

### `journal` — systemd journal for PowerLab services

Wraps `journalctl -o json -u powerlab-*.service`. Without `--service`
it matches every `powerlab-*` unit. The PowerLab services emit
structured slog records (see ADR-0026); `powerlab-logs journal`
parses them so the **level** column reflects the inner slog level,
not the outer syslog PRIORITY.

| Flag | Description |
|---|---|
| `--service NAME` | Filter to one service (`core`, `gateway`, etc.). Translated to `-u powerlab-NAME.service`. |
| `--follow`, `-f` | Stream new entries continuously (`journalctl -f`). |
| `--lines N`, `-n N` | Limit to the last N entries. `0` (default) means no limit. |
| `--json` | Emit one JSON object per line (NDJSON). Bypasses ANSI coloring. |
| `--no-color` | Force plain text even on a TTY. |

Examples:

```
# Last 200 lines from core, with colored levels on the terminal
powerlab-logs journal --service core --lines 200

# Live-tail every powerlab-* service, no colors (pipe-safe)
powerlab-logs journal --follow --no-color

# Pipe through jq for offline analysis
powerlab-logs journal --json | jq 'select(.level == "ERROR")'
```

### `app` — Docker container logs for an installed app

Wraps `docker logs <name>`. `<name>` is the compose project name
or the bare container name. Output is passed through unchanged —
Docker's own log driver already timestamps + formats per-container,
so we don't double-parse.

| Flag | Description |
|---|---|
| `--follow`, `-f` | Stream new entries (`docker logs -f`). |
| `--tail N` | Last N lines only. `0` (default) means no limit. |
| `--timestamps`, `-t` | Prefix each line with the timestamp Docker captured. |

Examples:

```
powerlab-logs app blinko --tail 200 --follow
powerlab-logs app gitingest --timestamps
```

### `install` — install/upgrade transcripts

Reads `/var/log/powerlab/install-<UTC-ISO8601>.log` files. Each
`install.sh` run is captured here automatically. Default mode dumps
the newest file; `--list` enumerates available files with their
mtimes.

| Flag | Description |
|---|---|
| `--list`, `-l` | List all install logs (newest first) instead of dumping. |

Examples:

```
# Read what just happened during the last upgrade
powerlab-logs install

# Find which install runs are on disk
powerlab-logs install --list
```

## Severity colouring

When `stdout` is a TTY and `--no-color` is NOT set, the journal
subcommand emits ANSI-coloured level columns:

| Level | Style |
|---|---|
| `FATAL` | bold red |
| `ERROR` | red |
| `WARN` | yellow |
| `INFO` | default |
| `DEBUG` | dim |

`--json` output is **never** coloured (pipe-safe for jq + log
shippers).

## Retention

| Source | Default retention | Tunable via |
|---|---|---|
| systemd journal | OS default (10 % disk, capped 4 GB) | `/etc/systemd/journald.conf` |
| Docker container logs | 30 MB rolling per container (`max-size=10m`, `max-file=3`) | per-app compose override (post-MVP) |
| Install logs | 10 newest files | `POWERLAB_NO_INSTALL_LOG=1` env to skip capture entirely |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success (or "nothing to print, no error", e.g. empty install-log dir). |
| `1` | Runtime error (`journalctl`/`docker` not found, permission denied, etc.). |
| `2` | Usage error (unknown subcommand, missing arg, invalid flag value). |

## Survivability matrix

| What's broken | `powerlab-logs` works? |
|---|---|
| PowerLab `core` crashed | ✅ yes — no daemon involvement |
| `gateway` down | ✅ yes |
| ALL PowerLab services down | ✅ yes |
| `journalctl` missing | ❌ no — `journal` subcommand will error |
| Docker daemon down | ✅ `journal`/`install` still work; `app` errors |
| `/var/log/powerlab` missing | ✅ `install` returns a friendly "no install logs" message |
| Disk full | ⚠️ may fail to write new install-log; existing reads work |
| Permission denied on the journal | ❌ run with sudo |

## Common workflows

**"The UI says 500 Internal Error — what's happening?"**

```
powerlab-logs journal --service core --lines 50
```

**"My app won't start"**

```
powerlab-logs app <app-name> --tail 100
```

**"Was the last upgrade clean?"**

```
powerlab-logs install
```

**"Stream everything live during a debug session"**

```
powerlab-logs journal --follow
```

## See also

- [ADR-0027 — Log aggregation service](../decisions/0027-log-aggregation-service.md)
- [Architecture — log aggregation](../architecture/log-aggregation.md)
- [ADR-0026 — `pkg/logging` on stdlib slog](../decisions/0026-pkg-logging-built-on-stdlib-slog.md)
