# 0033. Audit middleware — design decisions

- **Status:** proposed
- **Date:** 2026-05-14
- **Tracks:** [#357](https://github.com/neochaotic/powerlab/issues/357) (Sprint 16)
- **Feeds:** [#23](https://github.com/neochaotic/powerlab/issues/23) UI /logs page consumes the records this middleware produces; [#358](https://github.com/neochaotic/powerlab/issues/358) frontend errors land in the same store

## Context

Today, PowerLab logs come in three disconnected places:
1. **systemd journal** per service unit — captured via `journalctl -u powerlab-*` (the `powerlab-logs` CLI shipped in Sprint 14 #150 wraps this)
2. **Docker logs** per app container — `docker logs <container>` (also wrapped)
3. **install.sh logs** dumped to `/var/log/powerlab/install-*.log` (Sprint 14 #150 again)

What is missing is the **HTTP request audit trail** — every authenticated API call to the panel itself. Operators need it to answer "who restarted my apps?", "when did the gateway port change?", "did anyone try to log in last night?". Today the answer is "rummage through `journalctl` and hope the request happened to be info-logged."

This ADR locks the design before the code (#357 implementation) lands.

## Decision

Add an Echo middleware that records every authenticated HTTP request as a row in a **per-service SQLite database**, served read-only over a small `/v1/audit/` API. Settings → Audit pane shows recent records + retention controls.

### Where the middleware lives

**Per service** (not centralised at the gateway). Each service runs its own `audit.Middleware()` from `backend/common/utils/audit/`. Reasons:

- Gateway sees the URL but not the **resolved user** for some routes (some endpoints are loopback-skipped). Per-service runs *after* JWT parsing so the user_id is already on the request.
- Centralising at the gateway would require it to know the response status of downstream services after forwarding — adds an HTTP-tap that gateway doesn't have today.
- Per-service write to its own SQLite file keeps recovery simple (lose one service's DB → only that service's history goes; rest of the box is fine).

### Schema

Single table per service DB:

```sql
CREATE TABLE audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_unix_us  INTEGER NOT NULL,         -- µs since epoch — sortable, compact
  method      TEXT    NOT NULL,         -- GET, POST, PUT, DELETE
  path        TEXT    NOT NULL,         -- /v2/app_management/compose
  query       TEXT    NULL,             -- raw query string sans token=
  status      INTEGER NOT NULL,         -- HTTP status code
  latency_us  INTEGER NOT NULL,         -- request duration in µs
  user_id     INTEGER NULL,             -- from JWT claims, NULL for loopback
  username    TEXT    NULL,             -- from JWT claims (denorm for UI)
  remote_ip   TEXT    NOT NULL,         -- c.RealIP() — see PII section
  request_id  TEXT    NULL              -- correlation id from header X-Request-Id
);
CREATE INDEX audit_ts ON audit(ts_unix_us DESC);
CREATE INDEX audit_user ON audit(user_id, ts_unix_us DESC);
```

**Not stored** (intentional):
- Request body — too easy to leak passwords, secrets, compose YAML with embedded credentials
- Response body — same reason + size explosion
- Headers — except the request-id passthrough
- The JWT itself — never log credentials

### Retention

Two limits, whichever hits first:

- **Age**: rows older than N days are pruned. Default N=30.
- **Size**: SQLite file capped at M MB total. Default M=50. When exceeded, the oldest 10% of rows is purged and `PRAGMA wal_checkpoint(TRUNCATE)` runs.

Both configurable via Settings → Audit pane. Pruning runs every hour via a background goroutine; size check on every 100th write (cheaper than every write, still catches before disk-full).

### Performance

- **Async write**: middleware buffers the row in a 1024-slot channel and a single writer goroutine drains it via batched transactions (50 rows per tx, 200ms max delay). Hot path adds ~10µs per request — the channel send.
- **Channel full → drop oldest with a counter**: if the DB writer falls behind (disk pressure), we DROP rows rather than block the HTTP path. A `dropped_records` counter is logged at WARN every minute. Surfaced in the /logs page as a banner.

### Read API

Two endpoints under `/v1/audit/` (per service, behind same JWT auth):

- `GET /v1/audit/recent?limit=100&user_id=X&since=<unix_us>` — paginated, descending by ts. Default limit 100, max 1000.
- `GET /v1/audit/stats` — row count, oldest/newest timestamps, file size on disk. For the Settings pane to display "47,329 records (12 MB) since 2026-04-13".

Gateway aggregates across services for the unified /logs page (#23): calls each service's `/v1/audit/recent` in parallel, merges by ts, returns. Live SSE optional follow-up.

### PII

`remote_ip` is the most sensitive field. Options:

| Option | What | When acceptable |
|---|---|---|
| **Default: store full IP** | `192.168.18.86` for LAN, `203.0.113.42` for WAN | Single-operator homelab (PowerLab's target use case) — operator IS the data subject |
| Truncate to /24 | `192.168.18.0` | Multi-user box where some users have privacy expectations vs root admin |
| Hash | SHA-256 first 16 bytes | Pure compliance posture |

**Decision:** store full IP by default. Add a Settings toggle "Truncate IPs in audit log" that flips to /24 going forward (existing rows kept). Don't ship hashing in v1; revisit if commercial multi-tenant ever becomes a thing.

### Why SQLite (not pgsql / mongodb / parquet)

- **Zero-dep**: ships embedded in the Go binary via `modernc.org/sqlite` (pure Go, no CGO). PowerLab is already using SQLite for user-service (`pkg/sqlite/` exists) — operationally consistent.
- **Per-file isolation**: rm to nuke. backup = cp.
- **WAL mode** handles concurrent reader + writer cleanly; the audit middleware writes from one goroutine, the read API queries from echo handler goroutines.
- **GORM**: optional. The audit table is small + write-heavy; direct `database/sql` is fine and avoids ORM overhead. See ADR-0029 (GORM accepted) — this is an explicit per-feature exception.

Rejected:
- **Postgres**: requires running an extra container; PowerLab's value prop is "homelab without infra burden". No.
- **MongoDB**: same overhead + no query language fit.
- **JSON lines on disk**: tempting (rsync-friendly) but no indexed reads → /logs page would scan-all on every filter.

## Consequences

**Positive:**
- Single source of truth for "who did what when" in PowerLab.
- /logs page (#23) becomes a thin reader over a clean schema rather than a journalctl grep.
- Frontend errors (#358) and request errors share the same store, same retention policy — one mental model for operators.

**Neutral:**
- Each service grows ~50 MB disk allocation by default. Negligible on a homelab; controllable via Settings.

**Negative (controlled):**
- One more SQLite file per service to back up. The `powerlab-logs` CLI gains a `--audit` subcommand to dump records as text/JSON for offline analysis.
- Async write means the audit log can lag the actual request by up to 200ms — fine for forensic use, not OK for hard-realtime billing (not our use case anyway).
- Per-service DBs mean the /logs UI has to merge — adds complexity in the gateway aggregator. Worth it for the isolation benefit (a corrupt audit DB in one service doesn't take down the panel).

## Open questions

1. **Loopback requests** (`c.RealIP() == "127.0.0.1"`): JWT middleware skips auth for these (so on-host admin tools work). Should the audit middleware still record them with `user_id NULL`? **Lean yes** — they represent real actions (e.g. `powerlab-logs` CLI). Tag with `remote_ip = "loopback"` for searchability.
2. **WebSocket / SSE connections**: log the connect event, or every message? **Lean connect-only** — message-level audit defeats async streaming. Document in the schema comment.
3. **Bulk pruning during a heavy install**: if the prune goroutine and the install run concurrently, SQLite's WAL contention may add latency. Mitigation: run prune at fixed local time (3 AM) rather than every hour. Revisit if a user reports the issue.

## Acceptance for implementation (#357)

- [ ] `backend/common/utils/audit/` package with: `Middleware(db)`, `Recorder` struct (channel + writer goroutine), `OpenDB(path)` helper
- [ ] Schema migration via existing `pkg/migrations` (per ADR-0021 — applied to the audit DB on first open)
- [ ] 5+ services mount it: gateway, core, app-management, user-service, message-bus, local-storage
- [ ] Settings → Audit pane: recent records + retention controls
- [ ] `/v1/audit/recent` + `/v1/audit/stats` per service
- [ ] Test coverage: unit (recorder backpressure + retention) + integration (echo test server hitting the middleware end-to-end)
- [ ] Performance: < 20µs middleware overhead on the request path under load (benchmarked)
- [ ] Live SSH+browser verify on test box: Settings pane shows real records after exercising the UI

## Related

- ADR-0021 (storage paths) — audit DB lives under `/var/lib/powerlab/<service>/audit.db`
- ADR-0026 (logging via slog → journald) — audit is **structured request metadata**, not free-text log; the two are complementary
- ADR-0029 (GORM) — audit table opts out for hot-path performance
- Sprint 15 PRs #354, #355, #356 — defense-in-depth for the in-UI upgrade button; audit middleware here completes the "who triggered the upgrade?" story
