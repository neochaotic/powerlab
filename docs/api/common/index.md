# Go API reference — common

`backend/common` is the shared module that every other PowerLab service imports. It provides:

- the **cross-service SDK** (`external/*`) — typed clients each service uses to call its peers (gateway, message-bus, user-service, app-management) without re-writing URL resolution, retry, ping, or auth boilerplate
- the **JWT verifier** (`utils/jwt`) — ES256 sign + verify + JWKS, the auth surface every echo middleware in the codebase wraps
- the **HTTPS / cert manager** (`pkg/security`) — local CA + leaf lifecycle, IP-change rotation, HSTS gate (see ADR-0001 / ADR-0006 / ADR-0010 / ADR-0012)
- **shared response shapes** (`model.Result`, `model.DeviceInfo`, `model.ComposeAppWithStoreInfo`, …) — kept stable so V1 endpoints across services produce identical envelopes
- a long tail of OS-glue utilities (`utils/file`, `utils/exec`, `utils/systemctl`, `utils/paths`, …)

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [external](external.md) — cross-service SDK: `ManagementService` (gateway), `AppManageService`, `NotifyService`, `ShareService`, `GetPublicKey` / `ParseToken` (user-service), `GetMessageBusAddress` / `PublishEventInSocket`, `GPUInfoList` / `GetGPUUtilization`
- [model](model.md) — `Result` envelope, `DeviceInfo`, `Route`, `ChangePortRequest`, `ComposeAppWithStoreInfo`
- [utils/jwt](utils/jwt.md) — `Claims`, `GenerateToken` / `ParseToken` / `Validate`, `JWT` echo middleware, JWKS helpers (`JWK`, `JWKS`, `GenerateJwksJSON`, `PublicKeyFromJwksJSON`, `JWKSHandler`), `GenerateKeyPair`
- [pkg/security](pkg/security.md) — `CertManager` (CA + leaf lifecycle), `RotateCA`, `CheckAndRotate`, HSTS gate (`ArmHSTS` / `DisarmHSTS` / `IsHSTSArmed` / `IsHSTSDisarming`), `ShouldIncludeIP`, `HTTPSEnabled`
- [utils/file](utils/file.md), [utils/exec](utils/exec.md), [utils/http](utils/http.md), [utils/systemctl](utils/systemctl.md), [utils/devmode](utils/devmode.md), [utils/logger](utils/logger.md), [utils/constants](utils/constants.md), [utils/paths](utils/paths.md), [utils/port](utils/port.md), [utils/random](utils/random.md), [utils/time](utils/time.md), [utils/command](utils/command.md), [utils/common_err](utils/common_err.md) — OS-glue helpers
- [utils](utils.md) — package-level `Ptr` / time helpers

## Where to start

If you want to understand the auth flow end-to-end, follow:
[`utils/jwt.GenerateToken`](utils/jwt.md) (signing in user-service) → [`external.GetPublicKey`](external.md) (verifier-side cache) → [`utils/jwt.JWT`](utils/jwt.md) (echo middleware) → [`utils/jwt.Validate`](utils/jwt.md). For JWKS specifically, [ADR-0020](../../decisions/0020-jwt-keypair-persisted-by-default.md) covers why the keypair is persisted.

For the HTTPS / trust-dance flow, [`pkg/security.CertManager.Setup`](pkg/security.md) is the entry point, and [ADR-0001 / 0006 / 0010 / 0012](../../decisions/) document the design choices.

## Coverage

`common` is the fifth module surfaced (after `pkg/*`, `gateway`, `user-service`, `message-bus`). This raise lifted godoc coverage from ~49% to ~75% by documenting the high-traffic surface: every export in `external`, `model`, `utils/jwt`, plus the `pkg/security.CertManager` API. The long tail of `utils/*` helper packages is intentionally left at lower coverage — most are 1-2 line wrappers whose names already document them.
