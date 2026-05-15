# Backend Go Coverage Gap Audit

**Generated:** 2026-05-15 (Sprint 20 PR 8 — analysis-only)

**Scope:** 7 service modules under `backend/`.

**Methodology:** ran `go test -coverprofile -covermode=atomic ./...` per module, aggregated per source file, sorted by absolute uncovered statements (the prioritisation that maps to "biggest payoff per test added").


## Per-service totals

| Service | Statements | Covered | Coverage |
|---|---:|---:|---:|
| **common** | 1,857 | 532 | 28.6% |
| **app-management** | 3,602 | 938 | 26.0% |
| **gateway** | 878 | 353 | 40.2% |
| **core** | 2,784 | 255 | 9.2% |
| **user-service** | 631 | 69 | 10.9% |
| **message-bus** | 846 | 141 | 16.7% |
| **sync-catalog** | 510 | 314 | 61.6% |

## Top 10 coverage gaps per service

Sorted by **uncovered statement count** (the bigger the gap, the more leverage one test PR has). Files with <20 statements are filtered — those are usually constants or one-liners where coverage% is misleading.

### common

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `utils/file/file.go` | 349 | 0 | 0% |
| `pkg/security/cert.go` | 260 | 9 | 3% |
| `utils/systemctl/systemctl.go` | 165 | 0 | 0% |
| `common/external/gpu.go` | 59 | 0 | 0% |
| `utils/audit/store.go` | 120 | 62 | 52% |
| `common/utils/utils.go` | 51 | 0 | 0% |
| `utils/http/methods.go` | 48 | 0 | 0% |
| `common/external/user_service.go` | 88 | 40 | 45% |
| `common/external/gateway.go` | 45 | 0 | 0% |
| `common/external/app_manage.go` | 37 | 0 | 0% |

### app-management

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `route/v2/compose_app.go` | 362 | 0 | 0% |
| `app-management/service/container.go` | 284 | 0 | 0% |
| `app-management/service/compose_app_lifecycle.go` | 220 | 0 | 0% |
| `app-management/service/appstore_management.go` | 297 | 79 | 27% |
| `route/v2/appstore.go` | 208 | 12 | 6% |
| `app-management/service/compose_app_runtime.go` | 124 | 0 | 0% |
| `pkg/docker/container.go` | 118 | 0 | 0% |
| `app-management/service/compose_service.go` | 196 | 86 | 44% |
| `app-management/service/image.go` | 93 | 0 | 0% |
| `app-management/service/appstore.go` | 215 | 128 | 60% |

### gateway

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `gateway/main.go` | 251 | 18 | 7% |
| `gateway/route/security_route.go` | 186 | 110 | 59% |
| `gateway/service/mdns.go` | 78 | 11 | 14% |
| `gateway/route/static_route.go` | 66 | 33 | 50% |
| `gateway/route/management_route.go` | 58 | 35 | 60% |
| `gateway/route/gateway_route.go` | 64 | 42 | 66% |
| `gateway/route/docs_route.go` | 67 | 47 | 70% |
| `gateway/service/management.go` | 62 | 49 | 79% |

### core

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `utils/file/file.go` | 357 | 3 | 1% |
| `core/service/system.go` | 308 | 0 | 0% |
| `route/v1/system.go` | 141 | 0 | 0% |
| `route/v1/file_router_upload.go` | 122 | 0 | 0% |
| `core/service/notify.go` | 111 | 0 | 0% |
| `route/v1/file_mutate.go` | 101 | 0 | 0% |
| `core/route/v2.go` | 101 | 8 | 8% |
| `route/v1/file_browse.go` | 92 | 0 | 0% |
| `utils/file/reader.go` | 89 | 0 | 0% |
| `core/main.go` | 82 | 2 | 2% |

### user-service

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `route/v1/user.go` | 127 | 0 | 0% |
| `user-service/main.go` | 104 | 0 | 0% |
| `utils/file/file.go` | 75 | 0 | 0% |
| `pkg/config/init.go` | 33 | 0 | 0% |
| `user-service/route/event_listen.go` | 37 | 8 | 22% |
| `user-service/route/v2.go` | 35 | 7 | 20% |
| `user-service/route/v1.go` | 24 | 0 | 0% |
| `utils/file/image.go` | 21 | 0 | 0% |
| `user-service/service/user.go` | 43 | 23 | 53% |
| `user-service/service/keypair_store.go` | 22 | 18 | 82% |

### message-bus

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `message-bus/main.go` | 101 | 0 | 0% |
| `message-bus/route/api_route_action.go` | 98 | 0 | 0% |
| `message-bus/service/action_service_websocket.go` | 94 | 0 | 0% |
| `message-bus/route/api_route_event.go` | 100 | 16 | 16% |
| `message-bus/repository/repository_db.go` | 72 | 0 | 0% |
| `message-bus/service/event_service_websocket.go` | 101 | 55 | 54% |
| `message-bus/route/routers.go` | 44 | 0 | 0% |
| `message-bus/service/ysk.go` | 42 | 8 | 19% |
| `message-bus/service/socketio_service.go` | 37 | 8 | 22% |
| `message-bus/config/init.go` | 22 | 0 | 0% |

### sync-catalog

| File | Stmts | Cov | % |
|---|---:|---:|---:|
| `sync-catalog/main.go` | 120 | 0 | 0% |
| `sync-catalog/transform.go` | 159 | 136 | 86% |
| `sync-catalog/description.go` | 61 | 42 | 69% |
| `sync-catalog/validate.go` | 69 | 57 | 83% |
| `sync-catalog/emit.go` | 41 | 34 | 83% |
| `sync-catalog/filter.go` | 44 | 37 | 84% |

## How to read this audit

The right Sprint 21+ workflow is **not** "chase total coverage %" but "pick 2-3 files from this list per sprint, write tests, ship". The top entries are where uncovered statement count is highest — every test landed there moves the needle most.


**Bug-class targets (write tests if you touch these files for ANY reason):**

- `common/external/*` — cross-service SDK. Untested is exactly the v0.6.x bug class (gateway routing breaks silently).

- `common/pkg/security` — CA / leaf cert lifecycle. Untested edges lurk in HTTPS re-enable.

- `core/route/v[12]/*` — HTTP handlers; integration-test surface, not unit.


**Skip-list** (don't chase coverage here):

- `codegen/*` — regenerated from OpenAPI on every `go generate`; tests get wiped.

- `model/*` — pure data shapes; coverage is misleading.

- `cmd/*` main packages — exercised end-to-end by package-smoke CI.


## Not in scope for this audit

- Frontend coverage (see vitest --coverage artifacts, separate cadence).

- Integration vs unit ratio (testcontainer specs cover specific seams; this audit treats them all the same).

- `feedback_release_coverage_gate` numeric target — Sprint 11 retro already set the ≥25% baseline; this audit is about prioritisation, not enforcement.

