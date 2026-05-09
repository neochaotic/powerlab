# UI feature → backend service map

**Date:** 2026-05-08
**Sprint:** 1 (CasaOS strip — issue #62)
**Status:** static analysis complete; runtime E2E coverage deferred to Playwright sprint

## Headline

Six UI feature areas, each backed by one or two backend services.
Knowing this map up front means each kill PR can be **scoped to a
known set of UI flows** that need regression-testing — no surprises
from "this page also calls that service."

## UI feature areas

```
ui/src/routes/
├── +page.svelte         # Launchpad (entry, links to all areas)
├── apps/                # Installed apps + custom-app builder
├── dashboard/           # System health & resource graphs
├── files/               # File manager (browse, edit, upload)
├── models/              # AI / model management
├── product/             # About / marketing
└── settings/            # Network, security, updates, apps
```

## Feature → API module → likely backend service

| UI area | `$lib/api/*` modules used | Primary backend service | Secondary |
|---|---|---|---|
| `/apps` | `apps` | app-management | gateway (proxy) |
| `/apps/new` (Custom App) | `apps`, `client` | app-management | — |
| `/dashboard` | `system` | core | — |
| `/files` | `files` | local-storage | gateway (file upload streaming) |
| `/models` | (none yet) | app-management (deploys via compose) | — |
| `/product` | (none) | static — no backend touch | — |
| `/settings` | `apps`, `gateway`, `updater` | gateway, app-management, core (updater backend) | user-service (auth) |

## Component-level breakdown

API consumer counts per module:

| Module | Importers (files) |
|---|---:|
| `$lib/api/apps` | 9 |
| `$lib/api/files` | 8 |
| `$lib/api/client` | 6 |
| `$lib/api/endpoints` | 3 |
| `$lib/api/updater` | 2 |
| `$lib/api/system` | 2 |
| `$lib/api/gateway` | 1 |

### `$lib/api/apps` consumers

- `lib/stores/apps.svelte.ts` (state store)
- `lib/components/apps/ContainerLogs.svelte` (log streaming)
- `lib/components/apps/AppMetrics.svelte` (CPU/RAM stats)
- `lib/components/apps/AppCard.svelte` (per-app card)
- `lib/components/orchestrator/ComposeForm.svelte` (custom-app form)
- `routes/apps/+page.svelte` (apps list)
- `routes/apps/new/+page.svelte` (custom-app builder)
- `routes/settings/+page.svelte` (apps in settings panel)

### `$lib/api/files` consumers

- `lib/stores/files.svelte.ts`
- `lib/components/files/FileTable.svelte`
- `lib/components/files/FilePreview.svelte`
- `lib/components/files/Uploader.svelte`
- `lib/components/files/TextEditor.svelte`
- `routes/files/+page.svelte`

### `$lib/api/system` consumers

- `lib/stores/system.svelte.ts`
- `routes/dashboard/+page.svelte`

## Implication per kill

| Sprint | Service killed | UI areas to regression-test |
|---|---|---|
| 1 | gateway | **Every UI route** (gateway proxies all of them) — but only the routing/handshake side, not feature logic |
| 1 | message-bus | Implicit only — no direct UI consumer (event delivery is internal) |
| 2 | local-storage | `/files` page in full (FileTable, FilePreview, Uploader, TextEditor); large-file upload path |
| 2 | user-service | Login flow; every page that requires auth (essentially all but `/product`) |
| 3 | core | `/dashboard` (system metrics); `/settings` (updater fetches version info) |
| 4 | app-management | `/apps`, `/apps/new`, `/models`; `/settings` apps panel |

## Why this is enough — for now

This static map identifies **where to look** when each kill's
regression checks are written. Actual click-through verification
is deferred to a dedicated Playwright sprint (separate issue,
post-Sprint-1).

The static map answers: "if I delete app-management, what UI breaks?"
Playwright would answer: "after I rewrite app-management, does
clicking 'Install Syncthing' still produce the right toast at the
right time?" Both are useful; the latter is heavier infrastructure.

## What this audit does NOT cover

- **Realtime channels** (WebSocket subscriptions in `$lib/api/`)
  — these connect to message-bus or service-specific endpoints and
  need their own mapping.
- **Server-Sent Events (SSE)** for install logs and similar streams
  — already known from prior bug fixes, but not formally documented
  here.
- **Auth boundary** — when user-service is killed (Sprint 2), the
  JWT issuance and validation paths need separate analysis. Out of
  scope for this static audit.
- **Playwright E2E coverage** — separate issue, dedicated sprint.

## Methodology

```bash
# Find every API module imported in ui/
rg -E "from ['\"]\\\$lib/api" ui/src/

# Find every consumer of a specific module
rg -l "from ['\"]\\\$lib/api/<module>" ui/src/

# Find every backend route called from frontend
rg -E "'/v[12]/[a-z]+/" ui/src/lib/api/
```

Re-run the second command per kill PR to produce the
service-specific UI-impact list.
