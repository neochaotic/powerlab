# HTTP endpoint usage matrix

**Date:** 2026-05-08
**Sprint:** 1 (CasaOS strip — issue #62)
**Status:** complete for Sprint 1 kill targets (gateway, message-bus); methodology established for Sprints 2-4

## Headline

The frontend (`ui/`) is the only consumer for most backend routes.
Routes registered by `gateway` and `message-bus` (Sprint 1 kill
targets) have a small, well-defined surface — making both kills
practical without breaking external consumers.

## Aggregate counts

| Service | Routes registered | Frontend caller count | Internal caller count |
|---|---:|---:|---:|
| `gateway` | 6 | 1 (proxy + SPA) | n/a |
| `message-bus` | 0 (HTTP) | 0 | n/a — pub/sub via WebSocket / UDS |
| `core` | 75 | many (system, file, sys, hardware) | several |
| `user-service` | 21 | a few (auth) | indirect via gateway |
| `local-storage` | 14 | files, disk, mount | none direct |
| `app-management` | 15 | apps, compose | none direct |

(Exact per-route mapping deferred until each service's kill PR.
This matrix is the gating tool for **what to delete vs port** at
that point.)

## Sprint 1 kill targets — full mapping

### `gateway` (6 routes)

| Method | Path | Purpose | Caller |
|---|---|---|---|
| `GET` | `/ping` | Liveness probe | UI version handshake; CI smoke; install scripts |
| `GET` | `/*` | SPA static fallthrough | Browser nav (every UI route) |
| `GET` | `/v1/gateway/routes` | List registered downstream routes (debug) | Internal only — `route_management_test.go` |
| `POST` | `/v1/gateway/routes` | Register a downstream route | Each service on boot |
| `GET` | `/v1/gateway/port` | Get current gateway port | UI updater + version handshake |
| `PUT` | `/v1/gateway/port` | Change gateway port | UI Settings → Network |

**Action for kill PR (#73):**
- Port all 6 to the rewrite. They are all reachable.
- Replace the static fallthrough with `embed.FS` (planned anyway for
  the rewrite) plus delete the `CustomFS` helper flagged in
  `dead-code.md`.
- Verify the route-registration protocol is intact — every other
  service depends on `POST /v1/gateway/routes`.

### `message-bus`

**Zero classic HTTP routes.** The service exposes its API via a
WebSocket endpoint and a Unix Domain Socket. The `pkg/ysk/`
package (flagged in `dead-code.md` for deletion) was the only HTTP
shim and is unused.

**Action for kill PR (#72):**
- Replicate the WebSocket and UDS interface in the new in-memory
  eventbus (per ADR-0011 strangler).
- Delete `pkg/ysk/` outright — see `dead-code.md`.
- API parity test: every existing topic produces and consumes
  correctly through the new implementation.

## Methodology (for Sprints 2-4)

The full per-route × per-caller matrix for the larger services
(`core`, `app-management`) is **deferred to each service's kill
PR**. Doing the full mapping today, three sprints in advance, would
be wasted detail by the time it is consumed: routes get added,
callers shift, and re-running the analysis at kill time is faster
than maintaining a stale document.

Each kill PR will run, in this order:

1. **Extract route registrations** — grep
   `e\.(GET|POST|...)`, `Group(...)`, etc. inside the service's
   `route/` directory.
2. **Extract frontend callers** — grep
   `'/v[12]/<route-prefix>` in `ui/src/lib/api/`.
3. **Cross-reference** — produce a table per the format above.
4. **Drop unused routes** — any route with zero callers
   (frontend or internal) is deleted with the kill, not ported.
5. **Document the dropped routes** in the kill PR's commit message
   so reviewers can verify the deletion.

### Greppable patterns

```bash
# Backend route registrations (echo / gin)
rg -n '\.(GET|POST|PUT|DELETE|PATCH)\("' backend/<svc>/route/

# Frontend callers
rg -n "'/v[12]/" ui/src/lib/api/

# Internal service-to-service callers
rg -rn "<service-base-url>" backend/ ui/
```

## Why no full mapping for `core` / `app-management` today

- `core`'s 75 routes plus 288 dead functions need understanding
  *together*, not separately. The dead-code findings (especially
  the Dropbox + Google Drive drivers and the `route/v1/` legacy)
  already eliminate large chunks of the route surface before any
  matrix pass.
- `app-management`'s 15 routes are tightly coupled to the compose
  orchestrator that is being rewritten in Sprint 4. Mapping today
  freezes a snapshot that will not match the kill PR's reality.

When each kill is in flight, the matrix is built **fresh and small**
for that service alone. It informs the kill, then ships in the kill
PR.

## Out of scope

- WebSocket / UDS endpoints — flagged where they exist but matrix
  is HTTP-focused.
- Internal-only debug routes — usually fine to drop or hide behind
  a build tag, decided per kill.
