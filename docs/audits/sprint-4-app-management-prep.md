# Sprint 4 prep — `app-management` CasaOS surface

**Date:** 2026-05-09
**Sprint:** 4 prep (input to issue #85)
**Status:** complete (read-only audit)
**Refresh trigger** (per ADR-0019): re-run when Sprint 4 lands or when
the underlying compose-extension contract changes.

## Purpose

`app-management` is the largest service in the tree (~13,300 LOC across
88 `.go` files, excluding codegen) and the **only one without a
dedicated kill series**. Sprint 4 (#85) is the planned attack. This
doc maps every CasaOS-surface item still inside `app-management` so
the kill series can be scoped before it starts.

Earlier sprint findings on smaller services (#101 / #106) showed that
auditing first prevents two failure modes:

1. **Underscoping** — discovering a hidden coupling mid-PR (e.g. the
   `core.conf` / systemd `-c` mismatch in #140 only surfaced because
   the PR touched the surrounding code).
2. **Overscoping** — bundling unrelated concerns into one PR because
   they share a search hit. The audit lets the kill series split work
   into reviewable PRs from day one.

## Scope summary

App-management's CasaOS surface falls into **5 categories** with
very different risk profiles. In rough priority order:

| Category | Risk | Scope size |
|---|---|---|
| **1. Compose extension `x-casaos`** | High (ecosystem) | ~10 files |
| **2. Docker label `casaos`** | High (in-place container migration) | 4 sites |
| **3. CasaOS-team URLs** | Low (already kept intentional) | 4 URLs |
| **4. Code-internal CasaOS literals** | Medium (cosmetic + minor wire) | ~10 files |
| **5. License headers / `@FilePath` markers** | None (intentional attribution) | many files |

## Category 1 — Compose extension `x-casaos`

Every app shipped via the CasaOS App Store includes a
`docker-compose.yml` with an `x-casaos:` block carrying PowerLab/CasaOS-
specific metadata (icon, name, port mapping hints, before-install
tips, etc.). This is the **most ecosystem-coupled** item in
app-management.

### Status: dual-read already in place

The `service/extension.go::extensionPriority` chain already handles
all three keys with priority `x-powerlab → x-web → x-casaos`:

```go
var extensionPriority = []string{
    common.ComposeExtensionNameXPowerLab,
    common.ComposeExtensionNameWeb,
    common.ComposeExtensionNameXCasaOS,
}
```

The SvelteKit UI mirrors this chain — see
`ui/src/lib/utils/compose-extension.ts:27`:

```ts
export const EXTENSION_KEYS = ['x-powerlab', 'x-web', 'x-casaos'] as const;
```

### What's left

- The const name `ComposeExtensionNameXCasaOS` is fine — it documents
  the legacy key it represents.
- `service/errs.go::ErrComposeExtensionNameXCasaOSNotFound` could be
  renamed to `ErrComposeExtensionNotFound` (the "X-CasaOS" specificity
  is wrong now that the chain accepts three aliases). Mechanical
  rename, ~8 call sites.
- App store catalog migration (writing `x-powerlab:` instead of
  `x-casaos:` when we host our own catalog) — separate from the
  app-management code, that's appstore-hosting work.

### Held back forever

The `x-casaos:` key itself stays in the priority chain. Apps in the
wild ship with this key today and will for years. Removing it would
break every existing CasaOS-era install upgrading to PowerLab. This
is the single largest backward-compat carry in the tree.

## Category 2 — Docker label `casaos`

Used as the discriminator: "is this Docker container managed by us?"

### Sites (4 total)

```
service/container.go:291   if m.Labels["casaos"] == "casaos" { ... }   # GetContainerAppList
service/container.go:412   showENV := []string{"casaos"}                # env-var allowlist (different concept, named "casaos" only by accident)
service/container.go:510   if config.Labels["casaos"] == "casaos" { ... }  # mutation guard during update
service/container.go:526   config.Labels["casaos"] = "casaos"          # apply on every container we create
```

### Migration risk

Every PowerLab container running today has `Labels["casaos"] = "casaos"`.
Renaming the label means:

- Existing containers no longer match the discriminator → "where did
  my apps go?" UX disaster on upgrade.
- Migration must be **dual-read** (accept both `casaos` and `powerlab`
  labels as managed) for at least one release window before the
  legacy label is dropped.

The UI does not grep for the literal label — confirmed by
`grep -rn "labels.casaos\|Labels\.casaos" ui/src` (zero hits). So the
rename is contained to backend.

### Proposed Sprint 4 PR

1. Add a `powerlab` label written alongside `casaos` on every
   container we create (write both, keep the old).
2. Update the discriminator to accept either label.
3. After 1-2 release windows in the wild, drop the `casaos` label
   write but keep the read.

### `showENV := []string{"casaos"}`

This is **not a label** — it's an environment-variable allowlist
seed. The literal "casaos" here means "the env var named casaos".
Independent from the label work; can be renamed cosmetically or left.
Low priority.

## Category 3 — CasaOS-team URLs

| URL | Where | What | Action |
|---|---|---|---|
| `cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@gh-pages/store/main.zip` | `app-management.conf.sample` (default `appstore` setting) + `migration_0415_and_older.go` | Upstream CasaOS app catalog | **Keep until we host our own mirror.** External data source, intentional. |
| `cdn.jsdelivr.net/gh/IceWhaleTech/CasaOS-AppStore@main/Apps/...` | `validator_test.go` test fixtures | Image URLs in test docker-compose.yml fixtures | **Keep.** Test data, never fetched at test time. |
| `casaos.oss-cn-shanghai.aliyuncs.com/IceWhaleTech/_appstore/...` | `cmd/migration-tool/migration_0415_and_older.go` | Old CasaOS app store URL (China mirror) | **Keep.** Migration tool runs once during legacy install upgrade. |
| `github.com/bigbeartechworld/big-bear-casaos` | `app-management.conf.sample` + `migration_0412_and_older.go` | Community-maintained app store | **Keep forever.** Name predates PowerLab; it's the data source. |

All four are intentional per #147's reasoning. No Sprint 4 work needed
here unless we run our own appstore mirror (separate post-v1.0 effort).

## Category 4 — Code-internal CasaOS literals

### Type / constant renames (mechanical, ~10 files)

| Symbol | File | Notes |
|---|---|---|
| `model.CasaOSGlobalVariables` (struct) | `model/sys_common.go:24` | 1-field struct (`AppChange bool`). Used in 2 sites in `route/v1/docker.go`. Rename to `AppChangeFlag` or wrap in a more sensible name. |
| `config.CasaOSGlobalVariables.AppChange` | `route/v1/docker.go:693, 764` | Call sites for the above. |
| `data["casaos_apps"]` JSON key | `route/v1/docker.go:497` | Wire-format. **Endpoint `/v1/app/my/list` is legacy v1**, not consumed by the SvelteKit UI (verified by grep). Pre-v1.0 wire-format change is allowed. Rename to `apps` or `managed_apps`. |
| `common.ComposeAppAuthorCasaOSTeam = "CasaOS Team"` | `common/constants.go:12` | Author string for legacy v0.4.3 apps converted to compose. Used in `compose_app.go:170` for catalog filtering. Pre-v1.0 cosmetic. |
| `DefaultPassword = "casaos"` | `common/constants.go:37` | Default password for app installs (env var `$DefaultPassword`). Apps that rely on this string will break if we change it. **Held back.** Treat as compat surface. |
| `placeholderFile := ".casaos-appstore"` | `service/appstore.go:186` | Marker file written into app store dir. Internal. Rename to `.powerlab-appstore` is safe — the file is regenerated on every store sync. |
| `os.MkdirTemp("", "casaos-compose-app-*")` | `service/compose_app.go:1079` | Tmpdir prefix. Internal, no impact, cosmetic. |
| `ErrComposeExtensionNameXCasaOSNotFound` | `service/errs.go:12` | Renamed under Category 1. |

### Documentation strings

| String | File | Action |
|---|---|---|
| `"This is a compose app converted from a legacy app (CasaOS v0.4.3 or earlier)"` | `model/manifest_adapter.go:121` | **Keep.** Accurately describes legacy CasaOS data being read. |
| `"CasaOS User"` (default Author) | `model/manifest_adapter.go:125` | Default author for converted legacy apps. Could rename to `"PowerLab User"` but for legacy data it's accurate as-is. **Keep.** |
| `"PowerLab/CasaOS extension not found (tried x-powerlab, x-web, x-casaos)"` | `service/app.go:17` | Already PowerLab-aware. Good. |

## Category 5 — License headers / `@FilePath` / `@Website` markers

Every CasaOS-derived `.go` file carries an Apache-2.0 license header
with `@FilePath: /CasaOS/...` and `@Website: https://www.casaos.io`.
These are **intentional attribution** per the original audit's
`License posture` guidance and AGPL practice.

Per ADR-0019 + the Sprint 1 audit:
- Files preserved-but-renamed: keep the original header.
- Files rewritten from scratch in a kill PR: remove the header.

No bulk action. Sprint 4 PRs touch them as side effect of rewrites.

## Module path rename

`backend/app-management/go.mod` declares
`github.com/IceWhaleTech/CasaOS-AppManagement` as of this audit. The
sweeping module rename PR (#151, in flight at audit time) renames it
to `github.com/neochaotic/powerlab/backend/app-management` — once
that PR merges this category becomes done.

## Test surface

22 `*_test.go` files in app-management today:

- `model/manifest_adapter_test.go`
- `service/{local_store,service,compose_app,appstore,extension,powerlab,autoremap,list_filtering,compose_app_disk}_test.go`
- `cmd/validator/pkg/validator_test.go`
- (others in `service/v1/`, `route/v2/`)

Coverage % was reported in PR #149 audit: low single-digit on the
`service/` package. Sprint 4 work should not regress this and can use
the existing `extension_test.go` + `service_test.go` patterns as a
template for new tests.

## Suggested Sprint 4 PR breakdown

In rough order, smallest to largest:

1. **Cosmetic literals** — placeholder file (`.casaos-appstore` →
   `.powerlab-appstore`), tmpdir prefix, doc strings. Single small
   PR, low risk.
2. **Type rename** — `CasaOSGlobalVariables` → `AppChangeFlag` (or
   similar). Touches 3 sites. Mechanical.
3. **Wire-format `casaos_apps` JSON key** — pre-v1.0 rename.
   Touches the legacy `/v1/app/my/list` endpoint only (not consumed
   by SvelteKit UI). One PR.
4. **Docker label dual-write** — the big one. Adds the `powerlab`
   label alongside `casaos` on every new container. Updates the
   discriminator to accept either. Keeps the legacy label-write for
   one release window (drop in a follow-up PR after 1-2 versions in
   the wild).
5. **Extension error rename** — `ErrComposeExtensionNameXCasaOSNotFound`
   → `ErrComposeExtensionNotFound`. ~8 call sites.

These can interleave with Sprint 4's larger goals (#85: Docker labels +
AppData isolation) — items 1-3 are pure cosmetic and can land first
to clear noise before the riskier dual-write work.

## What's NOT in this audit's scope

- **App store hosting** — running our own app catalog mirror is post-v1.0
  work and lives outside `app-management`.
- **AppData isolation** — partially in #85; the AppData filesystem
  layout is a separate cross-cutting concern.
- **Migration-tool surface** — `cmd/migration-tool/main.go` etc. is
  intentionally CasaOS-pointed (legacy install upgrade). Held back
  by every Sprint 3 PR; same applies for Sprint 4.

## Reference

- Master roadmap: #67
- Sprint 4 issue: #85
- Tech-debt tracking pattern: ADR-0019
- Cross-project structural map: `casaos-dependencies.md` (this file
  is the deep-dive companion for app-management)
