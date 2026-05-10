# Go API reference — app-management

The app-management service owns the docker-compose-based application lifecycle on a PowerLab host: catalog management (multi-store, git-backed), install + uninstall (with port-conflict auto-remap, AppData path migration, and pull-progress streaming), in-place upgrade, and the homepage app-tile data layer. It also brokers app-related labels (ADR-0021) and emits lifecycle events to the message-bus so the UI can react in real time.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [common](common.md) — service-name + label namespace constants (ADR-0021), AppData path helpers, context-property helpers, message-bus event-type tables
- [model](model.md) — DB row + JSON shapes (`ServerAppList`, `MyAppList`, `Category`, `CustomizationPostData`, port/env/path arrays for the V1 install form)
- [service](service.md) — high-leverage app-lifecycle surface: `ComposeService`, `ComposeApp` (Update/PullAndApply/PullAndInstall/Uninstall/Apply), `AppStore`, `AppStoreManagement` (multi-store CRUD + upgrade-availability cache), task helpers
- [service/v1](service/v1.md) — V1 service-layer compatibility shims
- [route/v1](route/v1.md), [route/v2](route/v2.md) — HTTP handlers
- [pkg/docker](pkg/docker.md) — typed wrappers around the docker engine API (auth, container ops, image pull/digest, registry, volumes, content trust)
- [pkg/config](pkg/config.md) — ini-backed config loaders (`CommonInfo`, `AppInfo`, `ServerInfo`, `Global`)

## Where to start

If you want to understand the app-install flow end-to-end, follow:
[`ComposeService.Install`](service.md) → [`ComposeApp.PullAndInstall`](service.md) → [`ComposeApp.Pull`](service.md) → [`ComposeApp.UpWithCheckRequire`](service.md) → [`ComposeApp.Up`](service.md). The autoRemapPorts + rewriteAppDataPathsToCanonical + remapVolumePaths helpers run inline before YAML write — see ADR-0021 for the AppData migration rationale.

For the app-store side, [`AppStoreManagement`](service.md) is the entry point — `RegisterAppStore` + `Catalog` + `Recommend` + `IsUpdateAvailable` are the surface the V2 routes call into.

For the container-label namespace migration (legacy `casaos` → canonical `io.powerlab.v1.kind=app`), see [`common.IsPowerLabApp`](common.md) and [`common.BuildLabels`](common.md), plus [ADR-0021](../../decisions/0021-docker-label-namespace-and-appdata-path.md).

## Coverage

app-management is the seventh module surfaced (after `pkg/*`, `gateway`, `user-service`, `message-bus`, `common`, `local-storage`). This raise lifted godoc coverage from ~35% to ~75% by documenting every export in `model`, `common`, the high-leverage `service` types (`App`, `ComposeApp`, `ComposeService`, `AppStore`, `AppStoreManagement`), and the package-level helpers used by the route layer.

The `codegen` package is intentionally untouched (regenerated from the OpenAPI spec by `oapi-codegen` on every `go generate`). The bundled CasaOS appstore data tree under `backend/data/appstore/...` is upstream-managed assets, not source code.
