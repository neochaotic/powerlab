# Go API reference — message-bus

The message-bus is PowerLab's in-process pub/sub: publishers register schemas (EventType / ActionType) up front, then publish/trigger live events; subscribers connect by WebSocket or socketio and receive matching messages with a 10s heartbeat. The bus also owns YSK ("Your Smart Knowledge") pinned-card persistence — the home-screen card list arrives over the bus and is stored in a separate sqlite DB so cards survive restarts.

The pages below are auto-generated from the Go source via [gomarkdoc](https://github.com/princjef/gomarkdoc) and rebuilt on every release.

## Packages

- [model](model.md) — wire-shape + DB-row types (`EventType`, `Event`, `ActionType`, `Action`, `PropertyType`, `Settings`)
- [repository](repository.md) — `Repository` interface + sqlite-backed `DatabaseRepository`; two physical DBs (events vs persist)
- [service](service.md) — `Services` container, `EventServiceWS` / `ActionServiceWS` dispatchers, `SocketIOService`, `YSKService`
- [route](route.md) — `APIRoute` (codegen ServerInterface), `NewAPIRouter` (echo + JWT + OAPI validator), `NewDocRouter`
- [route/adapter/in](route/adapter/in.md) — codegen → model adapters
- [route/adapter/out](route/adapter/out.md) — model → codegen adapters
- [config](config.md) — `InitSetup` and the package-level config singletons
- [common](common.md) — service-name constants and reserved event/action/property type identifiers
- [utils](utils.md) — bundled YSK onboarding card payloads
- [codegen](codegen.md) — oapi-codegen output (regenerate via `go generate`); not hand-edited
- [pkg/ysk](pkg/ysk.md) — vendored YSK card domain types shared with publishers

## Where to start

If you want to understand the publish path end-to-end, follow the call chain:
[`PublishEvent`](route.md) (HTTP handler) → [`EventServiceWS.Publish`](service.md) → [`SocketIOService.Publish`](service.md). For subscribers it's [`SubscribeEventWS`](route.md) → [`EventServiceWS.Subscribe`](service.md).

For YSK cards specifically, the [`YSKService.Start`](service.md) docstring covers the seed-on-first-boot + event-driven upsert flow.

## Coverage

Message-bus is the fourth module surfaced (after `pkg/*`, `gateway`, and `user-service`). This raise lifted godoc coverage from ~5% to ~75% by documenting every exported type, constructor, and route handler in `model`, `repository`, `service`, `route`, `route/adapter/in`, `route/adapter/out`, and `config`. The `codegen` package is intentionally left bare — it is regenerated from the OpenAPI spec by `oapi-codegen` and doc edits would be wiped on the next `go generate`.
