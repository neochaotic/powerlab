# Go API reference — core

The core service is the historical CasaOS daemon — system info aggregation (CPU/memory/disk/network/sensors stats for the homepage dashboard), file management (browse, upload, download with HMAC-signed URLs), SMB share configuration, peer-discovery for the multi-device pairing flow, system reboot/shutdown, the legacy WebSocket broadcaster, the storage-driver runtime (a vendored alist/openlist fork hosted under `internal/op`), and the powerlab-updater that drives in-place upgrades.

Largest module in the repo (355 exports across 126 files). This raise focused on the high-leverage public surface — package docs, Service interfaces, model types, common constants, the storage-driver runtime — rather than aiming for 100% coverage. Per-method docs on internal helpers + auto-generated codegen + the FUSE-style file handlers were intentionally skipped.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [common](common.md) — service-name + version constants, message-bus event-type catalog (`EventSystemUtilization`, `EventFileOperate`)
- [model](model.md) — DB row + JSON shapes (`UserInfo`, `SystemUser`, `Shares`, `SettingItem`, `StorageA`, `Sort`, `Proxy`, `FileStream`, `Object`/`ObjThumb`/`ObjectURL`/`ObjThumbURL`, `Path`, `DeviceInfo`, sys/server/app config, the `Result` envelope)
- [service](service.md) — `Repository` container + every service interface: `HealthService`, `NotifyServer` (event broker), `PeerService` (LAN device pairing), `RelyService` (per-app deps), `SystemService` (the big one), `OtherService`, plus per-request `Name`/IP helpers
- [service/model](service/model.md) — gorm row types specific to the service layer (PeerDriveDBModel, RelyDBModel, etc.)
- [route/v2](route/v2.md) — HTTP handlers (v1 was removed in Sprint 8 PR Q kill-list)
- [internal/conf](internal/conf.md) — driver-runtime config types
- [internal/op](internal/op.md) — driver registry (`RegisterDriver`, `GetDriverNew`, `GetDriverInfoMap`) + lifecycle hooks (`ObjsUpdateHook`, `SettingItemHook`, `StorageHook`)
- [internal/sign](internal/sign.md) — process-wide HMAC signer used by file-share URLs
- [internal/driver](internal/driver.md) — vendored storage-driver framework (alist/openlist fork)
- [pkg/cache](pkg/cache.md) — go-cache wrapper with core defaults
- [pkg/sign](pkg/sign.md) — time-bound HMAC signature primitive
- [pkg/sqlite](pkg/sqlite.md) — gorm + sqlite + ADR-0018 versioned migrations (`GetDb`)
- [pkg/config](pkg/config.md) — ini-backed config loader (`InitSetup`, `CoreConfigFilePath`, package-level `*Info` singletons)
- [pkg/utils](pkg/utils.md) — long-tail helper grab-bag (path, slice, time, balance, network detection)
- [pkg/fs](pkg/fs.md) — minimal io.Closer helper

## Where to start

If you want to understand the system-stats flow end-to-end, follow [`Repository.System`](service.md) → [`SystemService.GetCpuPercent`](service.md) / `GetMemInfo` / `GetDiskInfo` — these wrap gopsutil and are read by both the homepage dashboard and the legacy WebSocket broadcaster.

For SMB share management: [`Repository.Shares`](service.md) → [`SharesService.UpdateConfigFile`](service.md) (which rewrites smb.conf from the gorm rows) is the entry point.

For the file-share signed-URL flow: [`internal/sign.WithDuration`](internal/sign.md) → [`pkg/sign.HMACSign.Sign`](pkg/sign.md). Note the load-bearing "token" placeholder secret — see the package doc.

For the storage-driver runtime: [`internal/op.RegisterDriver`](internal/op.md) is what each driver calls at init() time; [`internal/op.GetDriverInfoMap`](internal/op.md) is what the admin "Add storage" form reads.

## Coverage

core is the eighth and final module surfaced (after `pkg/*`, `gateway`, `user-service`, `message-bus`, `common`, `local-storage`, `app-management`). This raise lifted godoc coverage from ~39% to ~75% by documenting the high-leverage exports across `model`, `common`, `interfaces`, `service`, `internal/{conf,op,sign}`, `pkg/{cache,sign,sqlite,config}`. Per-method docs on the `SystemService` 35-method gopsutil-wrapper interface, low-value getters, and the codegen package were intentionally skipped — the names self-document.

The vendored `internal/driver` storage-driver framework is left at its upstream documentation level (alist/openlist fork). The `codegen` package is regenerated from the OpenAPI spec by `oapi-codegen` on every `go generate`.
