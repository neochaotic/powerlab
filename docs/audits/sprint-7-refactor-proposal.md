# Sprint 7 refactor proposal — split the four >1000 LOC files

**Status:** draft, awaiting authorization
**Source:** quality audit `quality-and-tech-debt-2026-05-10.md` §D + §E
**Drafted:** 2026-05-10 (Sprint 6 closeout, while PR #226 was in CI)
**Scope:** four flagged "god files" + four flagged "god functions"

The Sprint 5.5 audit flagged 4 files >1000 LOC and 25 functions >100 LOC as "split when you touch them." Sprint 6 is closing on the godoc raise initiative; this doc enumerates concrete extract targets so the next sprint can pick them up without re-reading the audit cold.

**Risk level:** low — every split is a lift-and-shift inside a single package. No interface changes, no behaviour changes. Each file becomes its own PR; tests stay green throughout.

---

## File 1 — `backend/app-management/service/compose_app.go` (1,276 LOC)

The compose-lifecycle "god file" — owns every method on `ComposeApp`. Already carries 3 of its own TODOs.

### Extract targets

| New file | What moves | Why |
|---|---|---|
| `compose_app_metadata.go` | `StoreInfo`, `getExtension`, `getExtensionMap`, `AuthorType`, `SetStoreAppID`, `SetTitle` | Pure read/write of the x-extension block. No docker calls. Easy to unit-test in isolation. |
| `compose_app_lifecycle.go` | `Update`, `PullAndApply`, `PullAndInstall`, `Apply`, `Uninstall` | The mutation surface — pulls images, runs `compose up/down`. Group together because they share helpers (`autoRemapPorts`, `rewriteAppDataPathsToCanonical`, `remapVolumePaths`). |
| `compose_app_runtime.go` | `Up`, `UpWithCheckRequire`, `Create`, `Pull`, `Containers` | The "talk to docker engine" layer. Distinct from lifecycle in that these don't touch the compose-file YAML on disk. |
| `compose_app_query.go` | `App`, `Apps`, `MainService`, `MainTag`, `DiskUsage` | Read-only helpers. |

`compose_app.go` keeps the type declaration + the package doc.

### Counter-recommendation

If the package grows much beyond what's listed, consider extracting `ComposeApp` into its own subpackage (`service/composeapp/`) with one file per concern. That's a bigger lift; flag-only for now.

### Test plan

Existing tests in `compose_app_test.go`, `compose_app_disk_test.go`, `extension_test.go`, `autoremap_test.go` cover the public surface. Move the file split → run the suite → no test edits needed if the split is pure.

---

## File 2 — `backend/core/route/v1/file.go` (1,166 LOC)

File-manager V1 router. Mixed responsibilities: REST handlers, WebSocket upgrade + read/write pumps, file-monitoring goroutine, panic-on-encode at line 243 (audit §C).

### Extract targets

| New file | What moves | Why |
|---|---|---|
| `file_browse.go` | `DirPath`, `GetSize`, `GetFileCount`, `GetLocalFile`, `GetFileImage`, `GetFilerContent` | The read-side: browse + stat. |
| `file_mutate.go` | `RenamePath`, `MkdirAll`, `DeleteFile`, `PostOperateFileOrDir`, `DeleteOperateFileOrDir`, `PutFileContent`, `PostFileContent` | The write-side: mutations + content edits. |
| `file_upload.go` *(already exists at backend/core/service/file_upload.go — pick a different name)* `file_router_upload.go` | `GetFileUpload`, `PostFileUpload`, `PostFileOctet` | Multipart + tus upload paths. Distinct flow worth its own file. |
| `file_download.go` | `GetDownloadFile`, `GetDownloadSingleFile` | The signed-URL download flow. |
| `file_websocket.go` | `ConnectWebSocket`, `init` (upgrader), `Client`, `writePump`, `readPump`, `CenterHandler.monitoring`, `GetPeers` | The legacy WebSocket peer-broadcast subsystem. The audit-flagged panic at line 243 lives in `GetDownloadFile` — fix it as part of the `file_download.go` split (audit §C item 2 — convert to error return). |

`file.go` keeps the package import block + any package-level globals.

### Test plan

Currently no per-handler tests in `route/v1/file*`. The Playwright E2E suite covers the file-explorer happy path — that's the safety net during the split. Add a minimum test covering each of the 5 new files in a follow-up PR if the audit asks for it.

---

## File 3 — `ui/src/routes/apps/+page.svelte` (1,561 LOC)

App-store page with install pipeline + filter + grid all inline. 23 functions in the script block (1-618), 941 lines of markup (620-1561).

### Extract targets

#### Components (`ui/src/lib/components/apps/`)

| New file | What moves | Why |
|---|---|---|
| `AppGrid.svelte` | The category grid + per-app card rendering loop | The visual surface. Sole consumer of the catalog list. |
| `AppFilter.svelte` | The category-chip filter row + search input | Stateless display + a couple of bindings; safe to extract first. |
| `AppInstallDialog.svelte` | The install confirmation dialog + progress streaming | Modal lifecycle is self-contained. |
| `AppDetailDrawer.svelte` | The slide-in detail panel | Same pattern. |

#### Logic (`ui/src/lib/state/apps.svelte.ts`)

The 23 script functions currently mix 4 concerns: catalog fetch, install pipeline, filter state, dialog open/close. Extract each into a Svelte 5 rune-backed store:

- `useAppCatalog()` — fetch + cache + recommend list
- `useAppInstall()` — install + uninstall + progress event subscription
- `useAppFilter()` — search + category state, derived filtered list
- `useAppDialog()` — modal open/close + selected app

After the split, `+page.svelte` becomes a ~100-line composition file.

### Test plan

Existing Playwright `tests/apps.spec.ts` covers app-list render + install button. Use that as the smoke gate. Component-level vitest stubs follow per extracted file.

---

## File 4 — `ui/src/routes/settings/+page.svelte` (1,469 LOC)

Settings monolith — already fits the "split per-pane" pattern hinted in the audit.

### Extract targets

The settings page already renders distinct panels. Each becomes its own component under `ui/src/lib/components/settings/`:

| New component | Pane |
|---|---|
| `SystemPane.svelte` | System info, hostname, timezone |
| `NetworkPane.svelte` | Port settings, network interfaces |
| `SecurityPane.svelte` | HTTPS toggle, trust dance, cert reset (post-#101/#106/#104) |
| `StoragePane.svelte` | USB auto-mount, mergerfs toggle |
| `AppsPane.svelte` | App store URLs, OpenAI key |
| `BackupPane.svelte` | Backup/restore controls |
| `AdvancedPane.svelte` | Reboot, shutdown, factory reset |
| `AboutPane.svelte` | Version, links, license |

`+page.svelte` becomes the tab-router shell (~80 lines).

### Test plan

The skip'd `TextEditor.test.ts:229` (audit §F) is in a different file — separate cleanup. Settings-page Playwright coverage is thin today; the split surfaces obvious unit-test boundaries for future work.

---

## Functions >130 lines (audit §E — extract while you're in there)

The four service/route ones flagged as "real candidates for extraction":

| Function | File | Suggested extraction |
|---|---|---|
| `CreateContainer` (193 LOC) | `backend/app-management/service/container.go:382` | Split prep (label resolution, port-conflict scan) from execution (compose call). |
| `RecreateContainer` (192 LOC) | `backend/app-management/service/container.go:576` | Same split. The two functions duplicate ~60% of their bodies; extract a shared `prepareContainerSpec` helper. |
| `SendFileOperateNotify` (157 LOC) | `backend/core/service/notify.go:73` | Extract the per-event-type switch into a map of handlers; the function shrinks to a dispatcher. |
| `PostAddStorage` (146 LOC) | `backend/local-storage/route/v1/storage.go:138` | Validation (current ~60 LOC inline) becomes a `validateAddStorageRequest()` helper; storage-creation becomes the handler body. |

The four `main()` bodies (>130 LOC each) are wiring code — less of a smell. Skip unless touched.

---

## Suggested PR ordering (Sprint 7)

Each row is one PR; rows are independent so they can ship in parallel.

| # | PR | Risk | Estimated LOC moved |
|---|---|---|---|
| 1 | Split `compose_app.go` into 4 files | Low | 1,276 redistributed |
| 2 | Split `file.go` into 5 files + fix the flagged panic | Low–medium (the panic fix is behaviour-changing) | 1,166 redistributed + ~10 fix |
| 3 | Extract apps page components + state files | Medium (UI behaviour-sensitive) | 1,561 → ~100 + 4 components + 4 stores |
| 4 | Extract settings page panes | Medium | 1,469 → ~80 + 8 panes |
| 5 | Container.go: split `CreateContainer` + `RecreateContainer` (+ shared helper) | Low | ~400 → ~3 helpers + 2 thinned functions |
| 6 | `SendFileOperateNotify`: extract dispatcher map | Low | ~157 → ~30 + handler map |
| 7 | `PostAddStorage`: extract `validateAddStorageRequest` | Low | ~146 → ~50 + ~80 helper |

**Total file-count delta:** +20 files, no public-API changes, no test changes (existing suites cover the public surface).

---

## Why now (vs. opportunistic)

Audit §D explicitly recommends "split when you touch this file." The case for doing it as a deliberate sprint instead:

- **Three of the four files are about to be touched again.** `compose_app.go` is in the install path the v0.5.x reliability work keeps revisiting; `file.go` is involved in every file-manager bug fix; `apps/+page.svelte` is the most-edited UI file in Sprint 6.
- **Each split is independently revertable.** No cross-file dependencies.
- **Reduces the audit's "things we said we'd fix" backlog by 8 items in 7 PRs** without writing new behaviour — pure structural work.

---

## Out of scope for this proposal

- `backend/core/main.go`, `backend/app-management/main.go`, `backend/message-bus/main.go` — wiring code, audit-flagged but not worth splitting per audit §E.
- `internal/driver/*` — vendored upstream framework; do not refactor.
- The four `main()` bodies — wiring code; less of a smell.
- ADR splits — separate proposal track.

---

## Authorization needed

This doc is plan-only. None of the splits have been started. Approve the ordering (or rearrange) and I'll execute the PRs in sequence — each is a single mechanical change with the test suite as the safety net.
