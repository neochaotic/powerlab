# 0035. Audit storage — migrate from SQLite to JSONL + in-memory ring buffer

- **Status:** accepted (amended 2026-05-31 — see "Amendment — multi-writer safety (#632)" below)
- **Date:** 2026-05-14
- **Implemented:** PR #370 (Sprint 17, v0.6.12)
- **Amended:** PR closing #632 (v0.7.6) — replaced lumberjack with direct `O_APPEND` writes to deliver the multi-writer contract this ADR implicitly promised
- **Supersedes (in part):** [ADR-0033](./0033-audit-middleware-design.md) — middleware shape stays; storage backend changes
- **Unblocks:** [ADR-0034](./0034-standalone-observability-mcp-service.md) implementation — observability service can now consume JSONL directly

## Context

Sprint 16 (#357) shipped audit logging with **SQLite** as the storage backend (`/var/lib/powerlab/gateway/audit.db`, WAL mode, single `audit` table indexed by `ts_unix_us` and `user_id`). The middleware writes asynchronously through a recorder goroutine; the UI reads via `GET /v1/audit/recent` and `GET /v1/audit/stats`; a daily retention job deletes rows older than the configured TTL.

This works, the tests pass, and it's already in PR #362. But on review the user flagged it as **non-standard for system audit logs**. Surveying the field:

| System | Audit storage |
|---|---|
| Kubernetes audit log | JSON Lines file |
| HashiCorp Vault | File-based audit device (JSONL) |
| HashiCorp Consul | File-based audit log (JSONL) |
| AWS CloudTrail | JSON Lines in S3 |
| Docker (`json-file` driver) | JSON Lines per container |
| systemd-journald | Binary, but exports JSON |

**Nobody serious uses an embedded DB for system audit.** Embedded DBs are for business events (transactions, GDPR records); JSONL is the convention for system-level audit because operators expect `tail -F`, `grep`, `jq`, and `journalctl` ergonomics on the SSH prompt.

Where the SQLite choice hurts:

- **Not greppable from SSH.** Operator on the box has to run `sqlite3 audit.db 'SELECT ...'` instead of `tail -F /var/log/powerlab/audit.jsonl | jq`.
- **Schema is locked.** Adding a field needs a migration; JSONL is write-anything-read-anything.
- **No native streaming.** Tailing in real time needs polling; JSONL gives `tail -F` for free.
- **ADR-0034 friction.** The standalone `powerlab-observability` service needs read-only access to the audit data. With SQLite, that's a second handle on the same DB file from another process — works but adds WAL-checkpoint and lock-contention complexity. With JSONL, the observability service just opens the file and tails it.
- **DB corruption is unrecoverable.** If WAL gets truncated mid-write on a power loss, history is gone. JSONL is incrementally recoverable (skip the bad line, keep going).

Where SQLite earned its keep:

- **UI pagination + filter + stats are first-class in SQL.** `SELECT ... LIMIT/OFFSET WHERE user_id = ?` is sub-ms.
- **Retention is a single `DELETE WHERE ts < ?`.**
- **`modernc.org/sqlite` is pure Go**, no CGO, no operational complexity.

Volume reality check: PowerLab is a homelab. We see **dozens of authenticated requests per day**, not millions per second. Whatever SQL "does fast" at scale, a couple-MB JSONL file with an in-memory ring buffer also does fast enough.

## Decision

**Migrate the audit storage from SQLite to JSONL + in-memory ring buffer.** The middleware shape, async recorder pattern, retention contract, and HTTP API surface (`/v1/audit/recent`, `/v1/audit/stats`) all stay. Only the persistence layer changes.

### Storage layout

```
/var/log/powerlab/
  audit.jsonl          # current file, tail-able
  audit.jsonl.1.gz     # rotated, gzipped
  audit.jsonl.2.gz
  ...
```

Each line is a single JSON object — same fields as today's `Record` struct, just serialised as JSON instead of SQL columns:

```json
{"ts":"2026-05-14T20:35:42.123456Z","method":"POST","path":"/v1/users/login","query":"","status":200,"latency_us":4123,"user_id":1,"username":"alisson","remote_ip":"192.168.18.42","request_id":"abc-123"}
```

`ts` is RFC 3339 with microsecond precision (machine-readable + human-readable; supersedes today's `ts_unix_us` int).

### Rotation + retention

Reuse the **`lumberjack.Logger`** that the operational logger already depends on (no new deps):

- `MaxSize`: configurable, default 10 MB
- `MaxBackups`: configurable, default 60
- `MaxAge`: configurable, default 30 days
- `Compress`: true (gzip rotated files)

Today's `audit.RetentionRunner` becomes a thin wrapper that lets lumberjack do the work and only logs the housekeeping outcome.

### UI query layer

The middleware keeps an **in-memory ring buffer of the last N records** (default `N=1000`, configurable). This serves `GET /v1/audit/recent` instantly without any disk IO.

For older history, `GET /v1/audit/recent?since=<ts>&limit=N` falls through to a **streaming tail of the current file + gzipped backups** in reverse chronological order, applying filters (user, status, method) line-by-line. This is acceptable because:

- 99% of UI traffic asks for "the last few records", which the ring serves.
- Historical queries are operator-initiated and tolerate a few-hundred-ms latency.
- File scan caps at the configured retention window — bounded work.

`GET /v1/audit/stats` aggregates over the ring buffer for the live numbers and over the JSONL files for the historical counts. Both can be computed in a single pass.

### Concurrency

- Recorder goroutine: single writer to lumberjack (line-buffered, atomic per-line).
- Ring buffer: protected by a `sync.RWMutex`; reads (UI) take the read lock, the recorder takes the write lock per-append.
- File reads from the UI: each handler opens its own `os.Open` + `bufio.Scanner` — no shared file handle, no contention with the writer (lumberjack manages the active file).

### What stays from ADR-0033

- Middleware lives **per-service** (audit context still needs the resolved user_id from the JWT layer above)
- Token stripped from `query` (PII rule still applies)
- Loopback sentinel for non-authenticated requests (NULL user_id rendered as em-dash in UI)
- Same `audit.Record` struct surface — only the serializer changes

## Rationale

- Aligns with industry convention (Kubernetes / Vault / Consul / CloudTrail all do this)
- SSH-first ergonomics: `tail -F /var/log/powerlab/audit.jsonl | jq 'select(.status>=400)'` works on day one
- ADR-0034 simplification: observability service consumes a file, not a DB
- Recoverable: corrupted line skipped, rest of history intact
- Reuses lumberjack (already a dep) — no new external library
- Removes `modernc.org/sqlite` from `backend/common`, freeing every other service's go.sum from its transitive weight (the same drift that broke CI on PR #362)

## Alternatives considered

### A. Hybrid dual-sink (JSONL + SQLite index)
Recorder writes to BOTH JSONL (canonical) and SQLite (UI query index). Best operator ergonomics + best UI performance.

**Rejected for now:** 2× write IO is fine, but it doubles the failure surface (two backends to keep in sync, two retention loops, two recovery paths). The in-memory ring + file tail covers the UI's needs in our actual volume range.

May revisit if measured UI latency on history queries crosses ~500 ms.

### B. Stay SQLite-only
Accept the trade-off, ship as-is.

**Rejected:** locks ADR-0034's design into SQLite-on-second-process complexity and offers no operator value. The migration cost is bounded (the recorder + retention + endpoints are <1000 LOC of Go that already has tests we can re-target).

### C. Pure JSONL, no ring buffer (file-backed everything)
Simpler in-memory model — every UI query scans the file.

**Rejected:** the active file is being written to constantly by the recorder. Concurrent reads while the writer is active are messy (line-tear, partial writes during fsync). The ring buffer isolates the hot read path from the hot write path.

### D. systemd-journald with structured fields
Use `sd_journal_send` from each service. Native to the platform.

**Rejected:** ties us to systemd (PowerLab's manager today, but ADR-0007 explicitly leaves room for non-systemd targets). Querying journald structured fields needs `journalctl --output=json`, which is its own ergonomics. Doesn't simplify the observability service either.

## Consequences

### What this commits us to

- A migration PR that swaps the storage layer **before** ADR-0034 implementation begins
- Existing tests in `backend/common/utils/audit/` get re-targeted (recorder, retention, endpoints)
- ADR-0033 marked **superseded by 0035** for the storage section; middleware-shape section stays authoritative
- The Settings → Audit pane keeps its current API contract — no UI changes needed
- A one-shot migration script for users on v0.6.12+ who already have an `audit.db`: read existing rows → emit as JSONL into the new file → delete the DB. Or accept "history starts from upgrade" if migration friction outweighs the value (homelab audit history is not regulatorily binding).

### What this makes harder

- Aggregation queries that span historical files (full-text search, complex multi-field filters) — need to be scanned line-by-line. Acceptable at our volume; would not be at 1M req/s.
- Cross-record joins (e.g. "all requests in the same session") — JSONL can't index. If we ever need this, A. (hybrid) becomes the answer.
- Schema evolution: JSONL is append-tolerant but not enforce-able. Old records with missing fields need defensive reads. Lock the schema in `audit.Record`'s JSON tags and document additions as additive-only.

### Performance budget

| Operation | Today (SQLite) | Target (JSONL) |
|---|---|---|
| Recorder write | ~50 µs | ~30 µs (no SQL parse, no index update) |
| `/v1/audit/recent` (last 100) | ~2 ms | <1 ms (ring buffer, no IO) |
| `/v1/audit/recent` (history page, ~1k rows) | ~5 ms | ~50 ms (file scan, decompress on .gz) |
| `/v1/audit/stats` (last 24h) | ~3 ms | ~5 ms (ring + tail) |
| Retention sweep (daily) | ~10 ms (1 SQL DELETE) | ~0 ms (lumberjack rotates on size, age sweep is one `os.Remove` per expired backup) |

The history page degradation (5 → 50 ms) is the only real regression. Acceptable for an operator-initiated query.

## Amendment — multi-writer safety (#632)

**Date:** 2026-05-31

### What was implicit, and what we found

The original "Concurrency" section above said "single writer to lumberjack (line-buffered, atomic per-line)". When ADR-0047 (powerlab-mcp agent-identity propagation) was being implemented, the surrounding mental model crystallised into "the JSONL store is multi-writer safe — gateway + powerlab-mcp can both write to `/var/log/powerlab/audit.jsonl` because that's what ADR-0033's per-service middleware mandate implies". A TDD test added in PR #633 (`TestAuditJSONL_MultiWriterAtomicity`) proved that mental model wrong:

- `lumberjack.Logger` opens the file with `O_WRONLY|O_CREATE` — **not** `O_APPEND`.
- It guards writes with an internal `sync.Mutex` that protects **only its own instance**.
- Two `Store` instances (one per service) each hold an independent file descriptor with an independent offset. They overwrite each other's writes silently.
- Reproduced empirically: 4 concurrent writers × 250 records each produced ~750 of 1000 expected lines.

### What changed in the implementation

`backend/common/utils/audit/store.go` no longer uses lumberjack. The Store opens the file directly with:

```go
os.OpenFile(opts.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
```

`O_APPEND` is the load-bearing flag: the kernel atomically positions every `O_APPEND` write at end-of-file across separate FDs. Below `PIPE_BUF` (4096 B on Linux; macOS BSD-derived stacks honour the same lower bound) the write itself is atomic against concurrent writers. Audit records marshal to a few hundred bytes — well under the ceiling — so `AppendBatch` ships a per-call buffer in one Write and the kernel guarantees the line lands intact.

For oversized batches (`> 4000 B`) the implementation falls back to one Write per record — slightly more syscalls but every individual line is still atomic against concurrent writers, and the JSONL contract (line-oriented) tolerates interleaving from different sources.

### Contract delivered

| Property | Before (lumberjack) | After (direct O_APPEND) |
|---|---|---|
| Single-writer safety | Yes (per-instance mutex) | Yes (per-instance mutex + kernel atomic append) |
| Multi-writer safety (cross-process) | **No** (silent overwrite) | **Yes** (kernel guarantee under `PIPE_BUF`) |
| Per-line atomicity within batch | Yes | Yes |
| Rotation | Size-based + age-based, in-process | **None — see follow-up below** |

### What was dropped, and the follow-up

Lumberjack's size-based and age-based rotation went with it. PowerLab's homelab volume reality (dozens of authenticated requests per day) makes rotation a non-issue in practice — a year of audit logs fits comfortably in a few hundred MB. For operators who want bounded files:

- **Today's answer:** `logrotate(8)` with a `/etc/logrotate.d/powerlab` snippet, or `systemd-tmpfiles` with an age-out rule. The JSONL file is a plain file — every standard Unix rotation tool works against it.
- **Tomorrow's answer:** if anyone reports rotation as a missing feature, **#634** is the tracking issue — reintroduce in-process rotation via a rename-then-O_APPEND-reopen dance that preserves multi-writer safety (lumberjack's `Logger.Rotate()` won't do — it doesn't coordinate across processes). The `MaxSizeMB / MaxBackups / MaxAgeDays` fields stay on `StoreOptions` for API stability; they're ignored at runtime until #634 lands.

### Why this didn't bite earlier

Through v0.7.5 the gateway was the sole audit writer in production. The lumberjack-bounded contract was "fine" because there was no second writer. The moment ADR-0047 shipped `powerlab-mcp` as the second writer, every overlapping write between gateway and MCP would have lost records. The bug was latent; the architecture change is what would have exposed it.

### Regression lock

`backend/common/utils/audit/store_test.go::TestStore_MultiWriter_AtomicAppend` — 4 writers × 250 records, asserts exactly 1000 lines with every line parseable and per-writer counts intact. Runs under `-race`. Loops 3× passed in CI; failed on the lumberjack baseline (753/1000 lines). The original `TestAuditJSONL_MultiWriterAtomicity` test in `backend/powerlab-mcp/server/identity_integration_test.go` is now un-skipped and exercises the same contract through the public `audit.Service` surface.

## References

- [ADR-0033](./0033-audit-middleware-design.md) — current audit middleware design (this ADR supersedes the storage section)
- [ADR-0034](./0034-standalone-observability-mcp-service.md) — standalone observability service that benefits from this migration
- [ADR-0047](./0047-mcp-agent-identity-propagation.md) — second writer that exposed the latent multi-writer gap
- [Kubernetes audit log format](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/)
- [HashiCorp Vault file audit device](https://developer.hashicorp.com/vault/docs/audit/file)
- [`gopkg.in/natefinch/lumberjack.v2`](https://pkg.go.dev/gopkg.in/natefinch/lumberjack.v2) — still used by `backend/common/utils/logger` for operational logs; removed only from `audit`
