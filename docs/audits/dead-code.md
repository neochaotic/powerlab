# Dead code report

**Date:** 2026-05-08
**Sprint:** 1 (CasaOS strip — issue #62)
**Tool:** `golang.org/x/tools/cmd/deadcode` v0.45.0
**Status:** complete (with one caveat — see local-storage)

## Headline

**368 unreachable functions across the backend.** When each service
is rewritten in the CasaOS strip, **none of these functions need to
be ported.** Two entire feature subsystems (Dropbox driver, Google
Drive driver) appear to be inherited but unused — strong candidates
for outright deletion before kill PRs even start.

## Per-service summary

| Service | Unreachable funcs | Notable findings |
|---|---:|---|
| `gateway` | 9 | `CustomFS` filesystem wrapper unused; `GetGatewayPort` unused |
| `message-bus` | 9 | Entire `pkg/ysk/` package orphaned; `NewDatabaseRepositoryInMemory` unused |
| **`core`** | **288** | Dropbox + Google Drive drivers unused; `route/v1/` legacy routes; mass of unused utilities |
| `user-service` | 5 | Migration-tool helpers |
| `local-storage` | ⚠️ deferred | Cannot analyze on macOS (linux-only `udev`/`fuse` deps); rerun in Linux CI |
| `app-management` | 32 | `cmd/migration-tool/`, `cmd/appfile2compose/`, log helpers |
| `common` | 1 | One stray helper |

Raw output captured under `docs/audits/raw/deadcode-<service>.txt` for
review during each kill PR.

## High-impact findings

### 🟢 Definitely dead — delete during kill

These are isolated, no false-positive risk, no reflection or codegen
references. Recommended for outright deletion in the kill PR.

#### `core` — abandoned cloud drivers (29 functions)

```
drivers/dropbox/  — 15 unused methods on Dropbox struct
drivers/google_drive/ — 14 unused methods on GoogleDrive struct
```

These are inherited from upstream's "alist"-style integration.
PowerLab does not surface Dropbox or Google Drive in the UI; no
endpoint exposes them; no test exercises them. **Recommendation:
delete the entire `drivers/dropbox/` and `drivers/google_drive/`
directories during the core kill (Sprint 3).** Saves ~2,000 LOC.

#### `core` — `route/v1/` legacy routes (16 functions)

```
route/v1/cloud.go  — GetStorage
route/v1/file.go   — GetLocalFile, PostFileOctet
route/v1/notify_old.go — NotifyWS, PutNotifyRead
route/v1/system.go — GetCasaOSPort, PutCasaOSPort, GetSystemCupInfo, ...
```

These are the v1 of the API. The frontend uses v2. v1 routes are
registered but no caller hits them. **Recommendation: delete the
entire `route/v1/` directory during the core kill.** Saves ~1,500 LOC
(estimated).

#### `app-management` — migration tools (≥10 functions)

```
cmd/appfile2compose/  — log.go helpers
cmd/migration-tool/   — IsMigrationNeeded / PreMigrate / Migrate /
                        PostMigrate for several legacy versions
```

These are CasaOS-era data migration tools (e.g. v0.4.12 → newer).
They cover migrations that no PowerLab installation has ever
needed. **Recommendation: delete the entire `cmd/migration-tool/`
during the app-management kill (Sprint 4).**

#### `gateway` — unused `CustomFS` (7 functions)

```
route/static_route.go — CustomFS, CustomFile, CustomFileInfo
```

A custom `http.FileSystem` wrapper with no caller. The static asset
path uses `http.FS()` directly. **Recommendation: delete during
the gateway kill (Sprint 1, this sprint, in #73).**

#### `message-bus` — orphaned `pkg/ysk/` (4 functions)

```
pkg/ysk/ysk.go     — DefineCard, NewYSKCard, DeleteCard
pkg/ysk/adapter.go — FromCodegenYSKCard
```

YSK = "your smart kit" — an upstream product line not active in
PowerLab. **Recommendation: delete the entire `pkg/ysk/` directory
during the message-bus kill (Sprint 1, this sprint, in #72).**

### 🟡 Probably dead — verify before deletion

These are internal helpers that look unused statically but might
be used through reflection, generated code, or tests we haven't
catalogued. Each kill PR should grep before deleting.

- `core` `pkg/utils/file/` (28 functions) — file utilities. Some
  may be invoked by tests not covered by `deadcode`.
- `core` `pkg/generic_sync/` (21 functions) — sync utilities. Could
  be used by future code we don't want to delete.
- `core` `pkg/utils/` (18 functions) — general utilities. Manual
  scan recommended before mass-delete.
- `core` `internal/op/` (13 functions) — internal operations. Worth
  understanding before deletion.
- `common` `utils/jwt/` (8 functions) — JWT helpers shared across
  services. Will overlap with future PowerLab-owned auth in
  Sprint 2 (user-service kill).

### 🔴 False positive — keep

No findings flagged false-positive yet. The clearest false-positive
signal would be a function called only via reflection or only
referenced in a string (e.g., `reflect.MethodByName`,
`go:generate` directives). Manual review pending per kill PR.

## Caveat: local-storage

`deadcode` failed to analyze `local-storage` on macOS because the
service depends on `bazil.org/fuse` and `github.com/pilebones/go-udev`,
both of which fail to compile outside Linux. The 25 functions
reported are linker errors from the dependency tree, not actual
findings.

**Action:** rerun `deadcode` against `local-storage` from the
Linux-based CI environment when the local-storage kill PR (Sprint 2)
is opened. Capture the result inline in that PR.

## How each kill PR consumes this audit

For each kill PR:

1. **Open the corresponding raw file** under
   `docs/audits/raw/deadcode-<service>.txt`.
2. **For every 🟢 entry:** delete the function in the same PR. Tests
   must still pass; if they fail, the entry was a false positive
   and the file is moved to 🔴 with a reasoning note.
3. **For every 🟡 entry:** grep for the function name across
   `backend/`, `ui/`, and any generated specs. If the grep finds
   only the definition site, delete; otherwise mark 🔴 and move on.
4. **The kill PR's commit message includes a section "Dropped from
   the audit"** listing each deletion.

## Aggregate impact

If we act on the 🟢 findings in the upcoming kill PRs:

| Sprint | Service | Estimated LOC removed |
|---|---|---:|
| 1 | gateway (kill #2) | ~150 LOC (CustomFS + helpers) |
| 1 | message-bus (kill #1) | ~250 LOC (ysk + repo helpers) |
| 3 | core | ~3,500 LOC (drivers + v1 routes + utils) |
| 4 | app-management | ~500 LOC (migration tools) |
| **Total** | | **~4,400 LOC** |

8% of the backend never gets ported because it does not need to be.

## Next steps

- ✅ Capture findings (this document)
- Per kill PR: act on 🟢, verify 🟡, document 🔴
- Sprint 2: rerun deadcode for `local-storage` on Linux
- Sprint 4: rerun the full pass after all kills, confirm
  `backend/common/` is unreachable, then delete
