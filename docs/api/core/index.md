# Go API reference — core

The core service is the historical CasaOS daemon — system info aggregation (CPU/memory/disk/network/sensors stats for the homepage dashboard), file management (browse, upload, download with HMAC-signed URLs), SMB share configuration, peer-discovery for the multi-device pairing flow, system reboot/shutdown, the legacy WebSocket broadcaster, and the powerlab-updater that drives in-place upgrades. The vendored `internal/op` storage-driver runtime that the historical CasaOS shipped was removed in Sprint 19 (#394) — PowerLab uses Docker volumes + bind mounts via `app-management` instead.

Largest module in the repo. This raise focused on the high-leverage public surface — package docs, Service interfaces, model types, common constants — rather than aiming for 100% coverage. Per-method docs on internal helpers + auto-generated codegen + the FUSE-style file handlers were intentionally skipped.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [common](common.md) — service-name + version constants, message-bus event-type catalog (`EventSystemUtilization`, `EventFileOperate`)
- [model](model.md) — DB row + JSON shapes (`UserInfo`, `SystemUser`, `Shares`, `SettingItem`, `StorageA`, `Sort`, `Proxy`, `FileStream`, `Object`/`ObjThumb`/`ObjectURL`/`ObjThumbURL`, `Path`, `DeviceInfo`, sys/server/app config, the `Result` envelope)
- [service](service.md) — `Repository` container + every service interface: `HealthService`, `NotifyServer` (event broker), `PeerService` (LAN device pairing), `RelyService` (per-app deps), `SystemService` (the big one), `OtherService`, plus per-request `Name`/IP helpers
- [service/model](service/model.md) — gorm row types specific to the service layer (PeerDriveDBModel, RelyDBModel, etc.)
- [route/v2](route/v2.md) — HTTP handlers (v1 was removed in Sprint 8 PR Q kill-list)
- [pkg/cache](pkg/cache.md) — go-cache wrapper with core defaults
- [pkg/sign](pkg/sign.md) — time-bound HMAC signature primitive used by file-share URLs
- [pkg/sqlite](pkg/sqlite.md) — gorm + sqlite + ADR-0018 versioned migrations (`GetDb`)
- [pkg/config](pkg/config.md) — ini-backed config loader (`InitSetup`, `CoreConfigFilePath`, package-level `*Info` singletons)
- [pkg/utils](pkg/utils.md) — long-tail helper grab-bag (path, slice, time, balance, network detection)

## Where to start

If you want to understand the system-stats flow end-to-end, follow [`Repository.System`](service.md) → [`SystemService.GetCpuPercent`](service.md) / `GetMemInfo` / `GetDiskInfo` — these wrap gopsutil and are read by both the homepage dashboard and the legacy WebSocket broadcaster.

For SMB share management: [`Repository.Shares`](service.md) → [`SharesService.UpdateConfigFile`](service.md) (which rewrites smb.conf from the gorm rows) is the entry point.

For the file-share signed-URL flow: [`pkg/sign.HMACSign.Sign`](pkg/sign.md). Note the load-bearing "token" placeholder secret — see the package doc.

## Coverage

core is the eighth and final module surfaced (after `pkg/*`, `gateway`, `user-service`, `message-bus`, `common`, `local-storage`, `app-management`). This raise lifted godoc coverage from ~39% to ~75% by documenting the high-leverage exports across `model`, `common`, `interfaces`, `service`, `pkg/{cache,sign,sqlite,config}`. Per-method docs on the `SystemService` 35-method gopsutil-wrapper interface, low-value getters, and the codegen package were intentionally skipped — the names self-document.

The `codegen` package is regenerated from the OpenAPI spec by `oapi-codegen` on every `go generate`.
