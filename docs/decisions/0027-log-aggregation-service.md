# ADR-0027 — Log aggregation service (`powerlab-logs`)

- **Status**: Proposed
- **Target**: Sprint 14
- **Tracks**: Issue [#23](https://github.com/neochaotic/powerlab/issues/23)
- **Related**: Issue [#257](https://github.com/neochaotic/powerlab/issues/257) (per-service health panel, overlaps "restart buttons")

## Context

When something on a PowerLab host misbehaves, the operator's first
move is "show me the logs". Today there is no first-class surface
for that:

- The six PowerLab services write structured slog output to stdout
  (see ADR-0026), which systemd captures into the journal. The
  operator has to SSH in and run `journalctl -u powerlab-*` by hand.
- Docker container logs for installed apps live behind
  `docker logs <container>`, also SSH-only.
- The bundled `install.sh` script writes to whatever stdout the
  invoker has — there is no persistent file. Diagnosing a failed
  upgrade after the fact is impossible.
- There is no audit trail of authenticated POST/DELETE operations
  (who installed which app, who changed the listen port).
- Frontend JS errors are silent unless the user happens to have
  DevTools open at the moment of failure.

The v0.6.5 → v0.6.6 upgrade cycle (where the user saw "500 Internal
Error" during the gateway-restart window, see PR #339) is a clean
illustration: even with PowerLab healthy, transient symptoms appear
in the UI with no immediate way to verify what happened on the host.
Worse, when the gateway IS broken, the UI is the only channel — and
it is gone too.

## Decision

Ship a dedicated **log service** as a separate concern, in two
delivery surfaces with different survivability properties.

### Surface A — `powerlab-logs` CLI (Sprint 14 MVP)

A standalone Go binary in `/usr/bin/powerlab-logs`, cross-compiled
alongside the other six services in `scripts/package-linux.sh`.

It depends on **nothing PowerLab-managed at runtime**:

- Reads the systemd journal directly via the journalctl binary
  (already on the host as a soft-dep of systemd) — no daemon
  involvement.
- Reads Docker container logs via the Docker socket
  (`/var/run/docker.sock`) — same path Docker CLI uses, daemon
  only.
- Reads install/upgrade logs from `/var/log/powerlab/install-*.log`
  (new path, see "Install log capture" below) — plain files.
- Reads the audit SQLite file from `/var/lib/powerlab/db/audit.db`
  (new file, see "Audit trail" below) — file access only.

This guarantees the survivability the operator asked for: even if
all six PowerLab daemons are crashing or the binaries are corrupted,
`powerlab-logs` can still answer "what happened?". The operator
SSHes in and runs:

```
powerlab-logs journal                # systemd journal for powerlab-*
powerlab-logs journal --service core # filter to one unit
powerlab-logs app blinko             # docker logs <container>
powerlab-logs install                # tail last install.sh run
powerlab-logs install --list         # list rotated install logs
powerlab-logs audit                  # tail audit trail
powerlab-logs audit --since 1h       # last hour only
powerlab-logs audit --user neochaotic
```

Subcommands intentionally mirror the systemd / docker mental model
the operator already has. Output is plain text by default, with
`--json` available for piping into jq.

### Surface B — UI `/logs` page (Sprint 14 follow-up if time, else Sprint 15)

A new route in the SvelteKit SPA, gated behind the normal auth flow.
Lives in `routes/logs/+page.svelte`. Layout:

- Top tabs: **System** (journal) · **Apps** (per-container) ·
  **Install/Upgrade** · **Audit** · **Frontend errors**.
- Each tab streams via SSE from a backend endpoint
  (`/v1/logs/journal`, `/v1/logs/app/:name`, `/v1/logs/install`,
  `/v1/logs/audit`, `/v1/logs/frontend`).
- Live tail toggle (pause/resume), grep box, follow-tail behaviour
  matching `LogStreamer.svelte` (the install-log component
  introduced in #335).
- **Per-service controls**: a small row of badges at the top of the
  System tab shows the systemd state of each `powerlab-*` unit
  (active/failed) with a Restart button. Backend calls
  `systemctl restart powerlab-<svc>` via a privileged exec helper
  in `powerlab-core` (with a sudoers rule installed by `install.sh`
  granting `core` user permission to restart only the
  `powerlab-*.service` unit pattern — least privilege).

Surface B requires the gateway and at least one backend service
alive. When that is not the case, the operator falls back to
Surface A.

The UI surface ALSO embeds a top-right banner with `Tip: cannot
reach this page? Run \`powerlab-logs\` over SSH — see <docs link>`
so operators in mid-incident discover Surface A.

## Log sources and retention

| Source | Mechanism | Default retention | Configurable? |
|---|---|---|---|
| systemd journal (`powerlab-*`) | journalctl, structured slog from each service | `SystemMaxUse=10% disk` (systemd default — unchanged) | Operator-side via `/etc/systemd/journald.conf` |
| Docker container logs (installed apps) | Docker `json-file` driver via socket | **NEW**: `max-size=10m, max-file=3` per container (~30 MB rolling per app) | Per-app override in compose `x-powerlab.log.*` (post-MVP) |
| Install/upgrade logs | `install.sh` tees stdout into `/var/log/powerlab/install-<ISO8601-ts>.log` | **NEW**: **7 days** **OR** last 10 files, whichever happens first | settings input `logs.install.retention_days`, default 7 |
| Audit trail | New SQLite db `/var/lib/powerlab/db/audit.db`, one row per authenticated POST/PUT/PATCH/DELETE | **NEW**: **7 days** **OR** 100 000 entries, whichever happens first | settings input `logs.audit.retention_days`, default 7 |
| Frontend JS errors | `window.onerror` + `window.onunhandledrejection` POST to `/v1/logs/frontend` | **NEW**: in-memory ring buffer in `powerlab-core` — 1 000 entries OR 24 h, lost on service restart | Not configurable (intentionally ephemeral) |

The 7-day default for audit + install matches a "last week of
activity" mental model — long enough to investigate "what happened
this morning?" or "what did I do last weekend?" but short enough
that disk usage stays in the kilobytes for a typical home server.
Operators who need compliance-grade retention (90 / 365 days) can
bump the value in Settings → Logs without touching config files.

### Settings UI surface

A new section in **Settings → Logs** exposes:

```
[ Audit trail retention ]   [ 7 ] days   (1–365)
[ Install log retention ]   [ 7 ] days   (1–365)
```

Two number inputs, validated 1–365. Default 7. Changes write to
`/var/lib/powerlab/conf/logs.json` and take effect on the next
hourly prune cycle (no service restart needed). The form lives in
`ui/src/lib/components/settings/LogsPane.svelte` (new pane).

Disk math with the 7-day default:

- Install logs: 7 days × at most 1 cut = ~50 KB typical
- Audit: 50 ops/day operator × 7 days × 256 B = **~90 KB**
- Docker rotation: enforced ceiling = ~30 MB × N apps; previous
  state was unbounded — net improvement
- Frontend ring buffer: 0 disk

Total NEW persistent footprint at default 7 days: **well under 1 MB**
(plus Docker rotation which IS a net win versus today's no-rotation).
Bumping to 90 days lifts audit to ~1 MB; bumping to 365 days lifts
it to ~5 MB. Still trivial.

## Why a separate service / why not extend core

`powerlab-core` already does HTTP routing + state. Two reasons to
keep logs out of it:

1. **Survivability against core crashes.** If logs lived in core,
   a core panic would also kill the diagnostic surface. The CLI
   path (Surface A) explicitly avoids this — even with all daemons
   down, `powerlab-logs` works.
2. **Layering against process-restart loss.** Frontend-error
   ingestion endpoints (`/v1/logs/frontend`) DO live in core
   because they need HTTP + auth context; that ring buffer is
   intentionally ephemeral. The persistent sources (journal,
   docker, install, audit) bypass core entirely and read from the
   OS / Docker / files directly. Even a runaway core restart loop
   leaves the diagnostic data intact.

There is no long-running `powerlab-logs` daemon in this design.
The CLI is request/response and exits when the operator does. The
ingestion endpoints for frontend errors live in core because they
are HTTP-bound; they are not a separate process.

## Audit trail schema

```sql
CREATE TABLE audit_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp    INTEGER NOT NULL,    -- unix ms
  user_id      TEXT,                -- nullable for unauthenticated paths
  method       TEXT NOT NULL,       -- POST/PUT/PATCH/DELETE
  path         TEXT NOT NULL,       -- e.g., /v1/app-management/install
  status       INTEGER NOT NULL,    -- HTTP response code
  remote_ip    TEXT NOT NULL,
  request_id   TEXT,                -- echo X-Request-ID if present
  payload_hash TEXT                 -- sha256 of body for privacy-aware
                                    -- diff (not the body itself)
);
CREATE INDEX idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX idx_audit_user      ON audit_log(user_id);
```

The middleware that writes to this table lives in `backend/core/
middleware/audit.go`, wraps every authenticated POST/PUT/PATCH/
DELETE handler, and writes async (small-batch) to avoid blocking
the request path.

A separate cleanup goroutine in core runs hourly and prunes rows
older than `logs.audit.retention_days` (default 7, configurable
1–365 via Settings → Logs) AND beyond 100k entries.

**We do not store request bodies.** That would balloon the table
and leak secrets (passwords, tokens, app env). The `payload_hash`
is sha256(body) — enough for a postmortem to verify two requests
were the same payload without revealing it.

## Frontend error ingestion

```ts
// ui/src/app.html or a new module
window.addEventListener('error', (e) => {
  fetch('/v1/logs/frontend', {
    method: 'POST',
    body: JSON.stringify({
      kind: 'error',
      message: e.message,
      stack: e.error?.stack,
      url: location.href,
      ua: navigator.userAgent
    })
  });
});
window.addEventListener('unhandledrejection', (e) => { /* similar */ });
```

The ingest endpoint lives in `backend/core/route/v1/logs.go`,
writes to a ring buffer (1k entries OR 24h sliding window). When
the buffer fills, oldest entry is evicted. **Intentionally NOT
persisted** — the buffer is debugging-grade, not audit-grade.

## Install log capture

Update `scripts/package-linux.sh` so the generated `install.sh`
tees its output:

```bash
INSTALL_LOG="/var/log/powerlab/install-$(date -u +%Y%m%dT%H%M%SZ).log"
mkdir -p "$(dirname "$INSTALL_LOG")"
exec > >(tee "$INSTALL_LOG") 2>&1
```

Plus a rotation step at the start: keep newest 10 files, delete the
rest.

## Restart buttons (Surface B)

The UI's per-service Restart button calls
`POST /v1/services/<svc>/restart` in core. Core executes
`systemctl restart powerlab-<svc>.service` via a small helper that:

- Requires the caller to be authenticated (existing JWT middleware).
- Refuses any service name not matching `^powerlab-[a-z-]+$`
  (allowlist via regex, no shell escape risk).
- Writes the operation to the audit trail (`logs.audit`).

`install.sh` installs a sudoers fragment at
`/etc/sudoers.d/powerlab-restart`:

```
%powerlab ALL=(root) NOPASSWD: /bin/systemctl restart powerlab-*.service
```

assuming `core` runs as a member of the `powerlab` group. This is
already true today for systemd's normal service-management need.

Restart of `powerlab-core` itself is special-cased: the request is
acknowledged + the response written + a deferred `time.AfterFunc`
issues the restart 250 ms later so the response can drain. The UI
falls back to the upgrade-progress-overlay (added in #339) to
absorb the gateway-down window — they share the polling code.

## Sprint 14 scope and phasing

| Phase | Deliverable | Sprint |
|---|---|---|
| 1 | `powerlab-logs` CLI: `journal`, `app`, `install` subcommands | 14 MVP |
| 1 | Install-log capture in `install.sh` + rotation | 14 MVP |
| 1 | Docker log rotation config in `install.sh` (one-time, daemon.json patch) | 14 MVP |
| 2 | Audit middleware + SQLite store + `powerlab-logs audit` subcommand + retention pruning + Settings → Logs pane (retention number inputs) | 14 |
| 3 | Frontend error capture (UI side + core ingest endpoint + ring buffer) | 14 |
| 4 | UI `/logs` page with SSE streaming + tabs + grep | 14 if time, else 15 |
| 5 | Per-service Restart buttons + sudoers fragment | 14 if time, else 15 |

Phases 1–3 are the survivability + diagnostic baseline. Phases 4–5
are the polished UI surface; absence of them does not block the
"diagnose a broken host" use case (Surface A covers it).

## Trade-offs and what we are not doing

- **No centralized log shipping** (Loki, ELK, journald-remote).
  This is a single-box panel; ops can layer external aggregation
  on top of the journal independently. The CLI's `--json` output
  is the export hook.
- **No log enrichment / parsing rules.** Logs go through as the
  daemon wrote them. Structured queries are scoped to the audit
  table only (where the schema is ours).
- **No log encryption-at-rest beyond the existing
  `/var/lib/powerlab` directory permissions.** Audit SQLite is
  chmod 0600 in chmod 0700 dir.
- **No backpressure on frontend error ingest beyond rate limit
  in core.** A misbehaving browser tab could spam the ring buffer
  but cannot fill disk. Rate-limit middleware (100 req/min per
  authenticated user) is sufficient.

## Open questions to resolve during Sprint 14

1. Should the `powerlab-logs` CLI accept a `--follow` flag and
   stream live (a thin wrapper over `journalctl -f` + `docker
   logs -f` + `tail -f`)? Lean yes; cheap to add.
2. Should the audit table also capture GET requests for sensitive
   paths (`/v1/users`, `/v1/conf`)? Lean no for MVP — adds noise.
3. Where exactly does the "Restart all powerlab services" button
   live? Probably NOT on the logs page (too dangerous as one
   click). Probably Settings → Power → Restart services subsection.

## References

- ADR-0026 — `pkg/logging` built on stdlib slog (the structured
  output we will be reading)
- Issue #23 — original logs viewer request
- Issue #257 — per-service health panel (consumed by restart
  buttons)
- PR #339 — upgrade-progress overlay (reused for core-restart UX)
- PR #335 — `LogStreamer.svelte` (reused for live tail UI)
