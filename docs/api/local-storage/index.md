# Go API reference — local-storage

The local-storage service owns disk + USB management on a PowerLab host: block-device inspection (lsblk + smartctl), partitioning + formatting, mount-on-boot persistence (via /etc/fstab and an internal volume table), and the mergerfs union pool that exposes /DATA as a single filesystem across multiple physical disks. It also brokers SMB share lifecycle and emits hot-plug events to the message-bus so the UI can react to disk insertions in real time.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [common](common.md) — service identity (`Version`, `ServiceName`, `DefaultMountPoint`) and the message-bus event-type tables generated from the udev property-name lookup
- [model](model.md) — DB row shapes (`SettingItem`, `StorageA`, `Sort`, `Proxy`, `FileStream`) + system-config types (`CommonModel`, `APPModel`, `ServerModel`)
- [service](service.md) — `Services` container, `DiskService` interface, `USBService`, `NotifyServer`, plus the package-level `MyService` + `Cache` singletons
- [service/v2](service/v2.md) — driver-agnostic file API (`LocalStorageService`) used by the v2 routes
- [route/v1](route/v1.md), [route/v2](route/v2.md) — HTTP handlers
- [pkg/mount](pkg/mount.md) — wrappers around mount/umount + the FUSE filesystem (rclone-vfs backed)
- [pkg/fstab](pkg/fstab.md) — typed reader/writer for /etc/fstab (`Entry`, `FStab`, `Add`/`Remove`/`GetEntries`)
- [pkg/partition](pkg/partition.md) — wrappers around lsblk/partx/parted/partprobe/blkid (`Partition`, `GetPartitions`, `AddPartition`, `CreatePartitionTable`)
- [pkg/mergerfs](pkg/mergerfs.md) — typed wrapper around the mergerfs control file (`SetSource`/`AddSource`/`RemoveSource`)
- [pkg/sign](pkg/sign.md) — time-bound HMAC signature scheme used to authenticate file-download URLs (`Sign` interface + `HMACSign`)
- [pkg/sqlite](pkg/sqlite.md) — gorm + sqlite + ADR-0018 versioned migrations (`GetGlobalDB`, `GetDBByFile`, GORM hook constants)
- [pkg/cache](pkg/cache.md) — go-cache wrapper with local-storage default TTLs

## Where to start

If you want to understand the disk-management flow end-to-end, follow [`DiskService`](service.md) → [`pkg/partition.AddPartition`](pkg/partition.md) → [`pkg/mount.Mount`](pkg/mount.md) → [`pkg/fstab.FStab.Add`](pkg/fstab.md) for the mount-on-boot persistence path.

For the merge pool that exposes /DATA, [`DiskService.EnsureDefaultMergePoint`](service.md) is the entry point and the [`pkg/mergerfs`](pkg/mergerfs.md) helpers handle branch add/remove as disks come in + out.

## Coverage

local-storage is the sixth module surfaced (after `pkg/*`, `gateway`, `user-service`, `message-bus`, `common`). This raise lifted godoc coverage from ~27% to ~75% by documenting every export in `common`, `model`, `service` (DiskService + USBService + NotifyServer + Services), and the high-traffic `pkg/*` helpers (`mount`, `fstab`, `partition`, `mergerfs`, `sign`, `sqlite`, `cache`).

The FUSE handler methods on `pkg/mount.File` + `pkg/mount.Dir` already had brief comments and follow the bazil.org/fuse interface contract — left as-is. The `codegen` package is intentionally untouched (regenerated from OpenAPI on every `go generate`).
