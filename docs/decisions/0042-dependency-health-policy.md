# 0042 — Dependency health policy: CVE response + abandoned-module criteria

- **Status:** proposed
- **Date:** 2026-05-20
- **Trigger:** v0.7.2 govuln sweep (PR #492) found 4 fixable CVEs (cleared) and 3 no-fix CVEs in abandoned libraries (`archiver/v3`, `rardecode`, `jwt/v3+incompatible`). No policy existed to decide whether to accept, replace, or block such dependencies. ADR-0040 introduced govulncheck but deferred the acceptance criteria to a follow-up. This is that follow-up.

## Context

The govuln sweep revealed two distinct problem classes:

**Class A — fixable CVEs in maintained libs.** Fixed by `go get <module>@<patched>`. Low friction, covered by routine dep bumps. govulncheck catches these; the CI matrix flags them per service.

**Class B — no-fix CVEs in abandoned libs.** The library has a known CVE and either has no patch available or is EOL. The three instances found in v0.7.2:

| Library | CVE(s) | Situation |
|---|---|---|
| `github.com/mholt/archiver/v3` | GO-2025-3605, GO-2024-2698 | Project replaced by v4 (breaking API), v3 branch has no security fixes. Used directly by `common/utils/file/file.go` for archive creation. |
| `github.com/nwaples/rardecode` | GO-2025-4020 | Transitive dep of archiver/v3 (RAR extraction). No patch exists; RAR support is the vulnerable surface. |
| `github.com/golang-jwt/jwt` v3 | GO-2025-3553 | jwt v3 is EOL. Entry point is indirect (a transitive dep of another lib). jwt/v4 and jwt/v5 already ship in the codebase. |

None of these are reachable via the attack surfaces PowerLab exposes (HTTP API, systemd, Docker socket). The archive functions are used for log-export download endpoints. The RAR surface is never triggered — PowerLab doesn't process user-uploaded RAR files. The jwt/v3 path is purely transitive.

## Decision

### 1. Abandonment criteria (hard)

A dependency is **abandoned** if ANY of the following is true:

- Last commit on the default branch is older than **24 months** AND no security advisories have been addressed since then.
- The project explicitly states it is EOL / unmaintained (README, archived GitHub repo, etc.).
- A CVE exists with `Fixed in: N/A` in the govulncheck output AND the upstream project has no open PR/issue tracking a fix.

An abandoned dep triggers a **replacement window of one release cycle** (the next v0.7.x cut). Exception: if the vulnerable code path is demonstrably unreachable AND the dep is indirect (transitive only), the window extends to two release cycles with a written acceptance comment in go.mod.

### 2. CVE response SLA

| Severity | Fix deadline |
|---|---|
| Critical (CVSS ≥ 9.0) | Before next release cut, no exceptions |
| High (CVSS 7.0–8.9) | Within 2 release cycles |
| Medium/Low with reachable path | Within 3 release cycles |
| Medium/Low with unreachable path | Document + defer; re-evaluate each release |

"Release cycle" = one v0.7.x cut, roughly 2–4 weeks at current cadence.

### 3. New dependency intake criteria

Before adding a new `go get` dependency (direct), the author must verify:

1. **Activity** — at least one commit in the last 12 months OR an explicit LTS/maintenance statement.
2. **CVE clean** — `govulncheck` shows no open vulnerabilities at the version being pinned.
3. **License** — MIT, Apache-2.0, BSD-2/3, ISC, or MPL-2.0. GPL-licensed libs require explicit approval (comment in PR).
4. **Scope** — prefer stdlib equivalents when they exist and the ergonomic cost is low. Introduce a lib only when the stdlib path would require >100 LOC of re-implementation.
5. **Transitive footprint** — `go mod graph | wc -l` before and after; a new direct dep that adds >20 transitive deps requires a comment justifying the tradeoff.

These criteria apply to direct deps only. Transitive deps are audited by govulncheck and the abandonment scan (see §5).

### 4. Acceptance-risk exceptions

A dep that fails the abandonment or CVE criteria may be retained with an explicit acceptance comment in `go.mod` using the following format:

```
// ACCEPTED-RISK: <CVE-ID or reason> — <one-line justification> — revisit <release>
github.com/example/abandoned v1.2.3 // indirect
```

Acceptance comments must be renewed each release. A dep with a stale acceptance comment (older than 2 releases) is treated as unapproved and must be resolved before the next cut.

### 5. Periodic abandonment scan

A lightweight CI step (`scripts/check-dep-health.sh`, to be created) will:
- Parse all direct deps from each `go.mod`
- Query the GitHub API for last-commit date on each repo
- Emit a warning (not a failure) for any dep older than 24 months

This runs weekly via a scheduled workflow, not on every PR (to avoid rate-limiting and network flakiness in the hot path).

### 6. Immediate application to current no-fix CVEs

| Dep | Action | Target release |
|---|---|---|
| `archiver/v3` + `rardecode` | Replace `GetCompressionAlgorithm`/`AddFile` in `common/utils/file` with stdlib `archive/tar` + `compress/gzip` + `archive/zip`. Drop exotic formats (lz4, sz) unless used. | v0.7.3 |
| `jwt/v3+incompatible` | Identify the transitive dep pulling it; bump that dep or add `go mod edit -droprequire`. | v0.7.3 |

Until replaced, these deps carry an explicit `// ACCEPTED-RISK` comment in each relevant `go.mod`.

## Consequences

**Good:**
- Clear, actionable SLAs for CVE response — no ambiguity about "when do we fix this?"
- New dep intake criteria prevent accumulating the same class of technical debt again.
- `ACCEPTED-RISK` comments make the risk register visible in the source, not hidden in Jira/issues.
- Govulncheck strict flip (remove `continue-on-error: true`) becomes achievable once the three current no-fix deps are replaced.

**Bad / tradeoffs:**
- Replacing `archiver/v3` requires ~2-3h of work and testing. The stdlib path is more verbose but has zero transitive CVE risk.
- The 24-month activity threshold may flag libraries that are "done" and intentionally not changing (e.g., small utilities). The written acceptance path handles these; the threshold itself is not changed.
- The weekly dep-health scan adds a GitHub Actions job. Low cost, but not zero.

## Alternatives considered

**Accept all no-fix CVEs permanently as "unreachable."** Rejected. CVE reachability analysis is correct today but fragile — a new feature or refactor can make a previously unreachable path reachable without anyone noticing. Policy-based removal is safer than perpetual reachability analysis.

**Migrate to `archiver/v4`.** The v4 API is significantly different (streaming-oriented) and would require rewriting the same ~80 LOC. Since stdlib covers all formats actually used, there is no advantage to pulling in v4.

**Ignore jwt/v3 because it is indirect.** The indirect status means we don't directly call vulnerable code, but govulncheck marks it reachable. Removing it via the transitive vector is the correct fix.

## References

- PR #492: govuln dep-bump sweep (v0.7.2)
- ADR-0040: proportional engineering hygiene baseline
- `common/utils/file/file.go`: archiver/v3 usage (11 call sites)
- `backend/core/route/v2.go`, `file_download.go`, `health.go`: callers of `GetCompressionAlgorithm`/`AddFile`
- Issue #493: audit all modules against this policy
- Issue #494: replace archiver/v3 + rardecode with stdlib
- Issue #495: remove jwt/v3 transitive dependency
