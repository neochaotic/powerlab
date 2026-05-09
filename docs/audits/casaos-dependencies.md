# CasaOS dependency map

**Date:** 2026-05-08
**Sprint:** 1 (CasaOS strip — issue #62)
**Status:** complete

## Headline

**All seven existing backend modules declare CasaOS module paths**.
The dependency on the upstream `IceWhaleTech` org is not a leaf-level
detail — it is the **identity** of every shared module. Stripping
CasaOS therefore means renaming every `go.mod`, regenerating every
generated file that references the old path, and updating every
import across the tree. The new `backend/pkg/` module (PowerLab-owned)
is the only one that is not CasaOS-shaped.

## Module paths today

| Service | `go.mod` declares | LOC | Files |
|---|---|---:|---:|
| `gateway` | `github.com/IceWhaleTech/CasaOS-Gateway` | 4,450 | 25 |
| `message-bus` | `github.com/IceWhaleTech/CasaOS-MessageBus` | 3,376 | 47 |
| `core` | `github.com/IceWhaleTech/CasaOS` | 14,554 | 159 |
| `user-service` | `github.com/IceWhaleTech/CasaOS-UserService` | 2,762 | 29 |
| `local-storage` | `github.com/IceWhaleTech/CasaOS-LocalStorage` | 10,596 | 98 |
| `app-management` | `github.com/IceWhaleTech/CasaOS-AppManagement` | 13,214 | 87 |
| `common` | `github.com/IceWhaleTech/CasaOS-Common` (shared by all 6) | 5,207 | 45 |
| **`pkg`** (new) | `github.com/neochaotic/powerlab/backend/pkg` | ~600 | 12 |

Total backend (excluding `pkg/` and codegen/external): ~54,000 LOC.

## What "kill" means per service

For each service, the kill PR (Sprint 1-4) does three things:

1. **Rename `go.mod`** → `github.com/neochaotic/powerlab/backend/<svc>`.
2. **Replace internal imports** of the old path with the new one
   inside that service.
3. **Migrate one shared module dependency at a time** away from
   `backend/common/` (CasaOS-Common) onto `backend/pkg/`
   (PowerLab-owned).

When the last service is killed, `backend/common/` is unreferenced
and is deleted in the same PR (per ADR-0011 strangler pattern).

## Sprint order — recap from #67

| Sprint | Service(s) killed | Notes |
|---|---|---|
| 1 | `gateway`, `message-bus` | Smallest, foundation-dependent |
| 2 | `local-storage`, `user-service` | Filesystem + auth |
| 3 | `core`, appstore | System info, drivers, hardware |
| 4 | `app-management` | The largest; compose orchestrator |

## Risk surfaces beyond Go imports

- **Generated code (`codegen/`)** — most services have OpenAPI specs
  that refer to CasaOS schemas. The kill PR for a service must
  regenerate from a PowerLab-owned spec or strip the upstream
  schema reference.
- **Hardcoded URLs** — `casaos.oss-cn-shanghai.aliyuncs.com` appears
  in `appstore` data fetched by `app-management`. Migrating off this
  requires running our own appstore mirror (Sprint 3).
- **Database migration tags** — `app-management` and others embed
  `casaos_*` table names in migrations. Renaming requires a data
  migration step on existing installs (planned for Sprint 4 with
  the app-management kill).
- **License posture** — every CasaOS-derived file carries an Apache
  2.0 license header. The kill PRs preserve attribution where the
  code is preserved-but-renamed, and remove headers when the file is
  rewritten from scratch. Tracked per kill PR.

## Reference

- Strangler pattern rationale: ADR-0011
- Issue: #62 (this audit)
- Roadmap: #67
- Dead-code findings: see `dead-code.md` (companion document)
