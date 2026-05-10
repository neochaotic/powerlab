# Go API reference — user-service

The user-service owns authentication + user account state for PowerLab. CRUD on the `o_users` table, JWT signing keypair (persisted per ADR-0020 so sessions survive restarts), per-user custom config blobs, avatar/background image storage, the local audit-event log.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [model](model.md) — DB row types + JSON envelope (`UserDBModel`, `EventModel`, `Result`)
- [service](service.md) — `Repository`, `UserService` (CRUD + JWT keypair), `EventService`, `OSService`
- [route](route.md) — V1 + V2 router constructors (`InitRouter`, `InitV2Router`)
- [route/v1](route/v1.md) — V1 HTTP handlers (register, login, profile CRUD, image upload, refresh-token)
- [route/v2](route/v2.md) — V2 server-interface implementation (oapi-codegen)

## Where to start

If you want to understand the auth flow, read [`UserService`](service.md) (interface contract) → [`PostUserLogin`](route/v1.md) (login handler) → [`PostUserRefreshToken`](route/v1.md) (token rotation).

For the JWT lifecycle specifically, see [ADR-0020](../../decisions/0020-jwt-keypair-persisted-by-default.md).

## Coverage

User-service is the third module surfaced (after `pkg/*` and `gateway`). Coverage rose from 40% (pre-Sprint 5) to 75% via two PRs that added godoc to types/services first, then handler-level descriptions for the V1 user routes.
