# Backend coverage baseline — Sprint 11

**Date:** 2026-05-11
**Tooling:** `go test -race -coverprofile=coverage.out -covermode=atomic ./...`
**Source revision:** `main` post-#150 Phase 1 wiring
**How to reproduce locally:** `cd backend/<service> && go generate ./... && go test -race -coverprofile=coverage.out -covermode=atomic ./... && go tool cover -func=coverage.out | tail -1`

## Why this exists

Issue #150 (filed Sprint 3) flagged that the **heavy service packages** — `core/service/`, `local-storage/service/`, `app-management/service/` — sit at single-digit coverage because they hit Docker, fuse, mergerfs, mount syscalls, and external network. Sprint 3 closeout deliberately deferred lifting them since the upcoming kill PRs would rewire half the code anyway. Sprint 11 lands **Phase 1 (measurement) + Phase 2 (404 regression locks)**. Phases 3 + 4 (testcontainers + fuse build-tag tests) are queued for Sprint 12+.

## Baseline numbers (Phase 1)

Captured locally on darwin/arm64 + Go 1.26.1 (CI uses 1.25; small delta possible). `local-storage` cannot be measured on darwin because it transitively depends on fuse + mergerfs + udev (`syscall.AF_NETLINK`, `syscall.Setxattr`, …) — first CI run with the new artifact upload surfaces those numbers.

### core

| Surface | Statements |
|---|---:|
| **`backend/core` (total)** | **6.1 %** |
| `core/route` | 42.0 % (boosted by Phase 2 404 locks) |
| `core/service` | 17.0 % (the #150 target package) |
| `core/route/v1` | 0 % |
| `core/route/v2` | 0 % |
| `core/pkg/utils/ip_helper` | 28.6 % |
| `core/pkg/utils/encryption` | 0 % |
| `core/pkg/utils/httper` | 0 % |

### app-management

| Surface | Statements |
|---|---:|
| **`backend/app-management` (total)** | **15.5 %** |
| `app-management/model` | 87.2 % |
| `app-management/common` | 77.1 % |
| `pkg/utils/downloadHelper` | 100 % |
| `pkg/utils/envHelper` | 66.7 % |
| `pkg/docker` | 37.1 % |
| **`app-management/service`** | **24.7 %** (the #150 target package) |
| `app-management/route/v2` | 4.6 % |
| `app-management/service/v1` | 0 % (dead surface — Sprint 9 kill) |

### local-storage

Not measurable on darwin. CI Linux runner produces the artifact on the first run after this PR merges; the number will be appended here.

## Phase 2 regression locks landed in this PR

| Issue | What we locked |
|---|---|
| #101 / #143 (cloud-drive removal at core) | `/v1/recover/:type`, `/v1/cloud[/*]`, `/v1/driver[/*]` return 404 on `core` |
| #101 / #143 (cloud-drive removal at local-storage) | `/v1/recover/:type`, `/v1/cloud[/*]`, `/v1/driver[/*]` return 404 on `local-storage` (linux build-tag) |
| Sprint 3 self-update kill | `/v1/sys/version/check`, `/v1/sys/update` return 404 on `core` |
| Survivors (defensive) | `/ping`, `/v1/sys/version/current`, `/v1/powerlab/version`, `/v1/disks`, `/v1/storage`, `/v1/usb/usb-auto-mount` still register |

These tests instantiate the real `InitV1Router()` handler and assert via `httptest.NewRequest` + `req.RemoteAddr = "127.0.0.1:1234"` (forces the JWT skipper so the 404 surfaces instead of a middleware 401).

## Phase 3 — testcontainers (deferred to Sprint 12+)

The Docker-touching code paths (`app-management/service` install/uninstall flow, `core/service` Docker calls) need real Docker for honest coverage. `testcontainers-go` is the right tool but adds 1-2 days of harness work + a Docker-in-Docker requirement on CI runners. Defer.

## Phase 4 — fuse/mergerfs build-tag (deferred to Sprint 12+)

`local-storage/service` has `fuse.Listxattr`, `mergerfs.*`, `lsblk` plumbing that needs Linux + privileges. Tests must be build-tagged `//go:build linux + privileged` (or sudo in CI). Defer.

## Threshold gate (NOT in this PR)

Per the #150 issue charter, no hard threshold gate yet — Sprint 11 establishes the baseline + adds measurement. Like the frontend coverage gate (#297), a backend gate would land **after we have a 2-data-point trend**, with floors ~5 pp below the measurement. Track in a follow-up issue if and when the trend stabilizes.

## CI artifact

Each service's `coverage.out` is uploaded as `backend-coverage-<service>-<run-id>` with 14-day retention. To drill in:

```bash
gh run download <run-id> --name backend-coverage-core-<run-id>
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

## Not yet measured

- `gateway`, `user-service`, `message-bus` — out of #150 scope (these aren't the "heavy service packages"). CI still uploads their coverage artifacts now that Phase 1 is wired, but the audit numbers above only cover the three target services.

## Reproduce

```bash
cd backend/core         # or backend/app-management
go generate ./...        # regen codegen from openapi.yaml
go test -race -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out | tail -1
go tool cover -html=coverage.out -o coverage.html
open coverage.html       # macOS
```
