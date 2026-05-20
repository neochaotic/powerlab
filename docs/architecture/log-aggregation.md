# Log aggregation

PowerLab exposes operator-facing logs through **two surfaces** with
different survivability guarantees:

```
┌─────────────────────────────────────────────────────────────────┐
│  Surface A — CLI                                                 │
│  /usr/bin/powerlab-logs                                          │
│  No daemon dependency. Survives total PowerLab failure.          │
└──────────┬───────────────────────────┬──────────────────────────┘
           │                           │
           │ exec journalctl           │ exec docker logs
           │ (read systemd journal)    │ (read Docker socket)
           │                           │
           ▼                           ▼
    ┌─────────────┐             ┌─────────────┐
    │ systemd-    │             │ Docker      │
    │ journald    │             │ engine      │
    └─────────────┘             └─────────────┘
           ▲                           ▲
           │ slog → stdout             │ json-file driver
           │                           │   max-size=10m
           │                           │   max-file=3
    ┌──────┴────────┐           ┌─────┴───────┐
    │ powerlab-*    │           │ app         │
    │ services      │           │ containers  │
    └───────────────┘           └─────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Surface B — UI (Sprint 14 follow-up: ADR-0027 §UI page)         │
│  /logs page in SvelteKit SPA                                     │
│  SSE-streams via gateway → core → same upstream tools.           │
│  Requires the gateway + core to be alive.                        │
└─────────────────────────────────────────────────────────────────┘
```

## Why two surfaces

Surface A exists because the most common diagnostic workflow is
"PowerLab is broken, what happened?" — and if PowerLab is broken,
the UI is unreachable. The CLI deliberately reads from the same
sources Surface B reads from (journald, Docker socket, install
log files) **directly**, without any PowerLab daemon in the path.
Total PowerLab daemon failure does not impair Surface A.

Surface B exists because the most common ROUTINE workflow is
"I'm in the UI, I want to glance at logs without leaving" — and
SSH+CLI is friction. Surface B requires the gateway (which serves
the SPA) + core (which proxies the SSE) but offers a tab-based,
filtered, severity-coloured view.

## Data sources

| Source | Where it lives | Producer | Retention |
|---|---|---|---|
| PowerLab service logs | systemd journal | each `powerlab-*` service via `pkg/logging` (ADR-0026, slog → stdout → journal) | journald default (10 % disk) |
| Docker container logs | `/var/lib/docker/containers/<id>/<id>-json.log` | each app container's stdout/stderr | 30 MB rolling per container (max-size=10m, max-file=3) — set by `install.sh` patching `/etc/docker/daemon.json` |
| Install/upgrade logs | `/var/log/powerlab/install-<ISO8601>.log` | `install.sh` tee's its own stdout there | 10 newest files |
| Audit trail (Phase 2) | `/var/lib/powerlab/db/audit.db` (SQLite) | `backend/core/middleware/audit.go` | configurable, default 7 days |
| Frontend JS errors (Phase 3) | in-memory ring buffer in core | `window.onerror` → `/v1/logs/frontend` | 1 000 entries OR 24 h, ephemeral |

## Wire format — HTTP + SSE + JSON

Surface B uses HTTP+SSE+JSON. Not protobuf. Reasoning in ADR-0027 §
"Wire format — HTTP+SSE+JSON, not protobuf":

- `oapi-codegen` captures 80 % of the type-safety win without
  breaking ad-hoc debugging (`curl http://host:8765/v1/logs/...`)
- Browser's `EventSource` consumes SSE natively; gRPC-Web would
  add a shim and tooling
- No non-Go consumer is on the roadmap; protobuf doesn't unlock
  anything we need
- If multi-box aggregation becomes a real requirement, the right
  move is OTLP/HTTP (industry standard) as an additional export
  mode — not a roll-our-own protobuf schema

## Subcommand contract (Surface A)

The CLI exposes three subcommands; each shells out to the most
appropriate upstream tool:

| Subcommand | Upstream tool | Why exec rather than embed |
|---|---|---|
| `journal` | `journalctl -o json` | journald's read-path is the canonical journal API; reimplementing it inside the binary would require the journal protocol + permissions handling. The wrapper parses the JSON-line output and reformats it. |
| `app` | `docker logs <name>` | Docker SDK is ~30 MB of transitive deps; the survival-binary goal is to stay small + standalone. Docker CLI is already on every host that runs containers. |
| `install` | direct file read | install logs are plain files; no upstream tool needed. |

Severity colouring is applied to **journal output only** — Docker
logs and install logs are passed through unchanged because their
formats are operator-specific.

## Severity model

PowerLab's `pkg/logging` (ADR-0026) emits slog records with a
`level` field — DEBUG, INFO, WARN, ERROR, FATAL. The CLI's parser
prefers this inner level when the journal MESSAGE field parses as
JSON; otherwise it falls back to the outer syslog PRIORITY:

```
PRIORITY 0–2  → FATAL  (emerg / alert / crit)
PRIORITY 3    → ERROR  (err)
PRIORITY 4    → WARN   (warning)
PRIORITY 5–6  → INFO   (notice / info)
PRIORITY 7    → DEBUG  (debug)
```

Both surfaces (CLI + UI) use the same severity model so a row in
the UI matches a row in the CLI.

## Survivability boundaries

The CLI explicitly does NOT support:

- Querying logs from a **remote** PowerLab host (would require
  a wire protocol — out of scope for the survival use case)
- **Centralised log shipping** (Loki, ELK) — the `--json` output
  is the export hook; operators can wire OTLP / vector / fluentbit
  externally
- **Log enrichment / parsing rules** — passes through what the
  upstream emitted

These trade-offs keep the binary lean and the failure modes
predictable: if `powerlab-logs journal` doesn't work, the operator
has a clear next step (`apt install systemd`, `sudo`, etc.) rather
than chasing a custom log-aggregation pipeline.

## References

- [ADR-0027 — Log aggregation service](../decisions/0027-log-aggregation-service.md)
- [ADR-0026 — `pkg/logging` on stdlib slog](../decisions/0026-pkg-logging-built-on-stdlib-slog.md)
- [Operations — `powerlab-logs` CLI](../operations/powerlab-logs.md)
- Issue [#23](https://github.com/neochaotic/powerlab/issues/23) — original logs viewer request
- Issue [#150](https://github.com/neochaotic/powerlab/issues/150) — Phase 1 backend coverage tracker
