# 0050. MCP-served docs are canonical, code is implementation

- **Status:** accepted
- **Date:** 2026-06-01
- **Trigger:** 2026-05-31 MCP-only chat-mode test (see `memory/project_mcp_ux_gaps_chat_test`) — an agent asked to author a PowerLab compose YAML using only MCP signals produced **legacy CasaOS-era output** (`x-casaos`, `/DATA/AppData/$AppID`, hardcoded port `10380`) because it fell back to reading the source tree directly. The canonical PowerLab idioms (`x-powerlab`, `/DATA/PowerLabAppData/<id>/<purpose>`, ephemeral ports) live in the `compose_authoring` Prompt and `docs/concepts/compose-conventions.md` — the agent never reached them.
- **Builds on:** [ADR-0034](0034-standalone-observability-mcp-service.md) (MCP architecture), [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) (docs:// surface), [ADR-0048](0048-mcp-docs-surface-compose-authoring.md) (compose_authoring + concepts), [ADR-0049](0049-mcp-sensitive-sysadmin-tier-threat-model.md)
- **Related:** PR #656 (P0.1 — discovery Tools that expose Prompts as Tools), PR #655 (P0.3 — search_docs covers OpenAPI + catalog)

## Context

PowerLab grew out of a CasaOS fork. The codebase carries multiple legacy idioms (`x-casaos` compose metadata, `/DATA/AppData/` volume paths, hardcoded container port numbers from upstream catalogs) that are **still parsed for backward compatibility** with imported apps. New PowerLab apps use **different canonical idioms**:

| Legacy (still parsed) | Canonical (current) |
|---|---|
| `x-casaos: { ... }` | `x-powerlab: { ... }` |
| `/DATA/AppData/$AppID/data` | `/DATA/PowerLabAppData/<id>/<purpose>` |
| Hardcoded `10380:80` ports | Ephemeral allocation via PowerLab port manager |
| `casaOS.db` filename | (cosmetic carry-over; the schema is PowerLab's) |

The parsers in `backend/app-management/codegen/extension.go` prioritize `x-powerlab` and fall back to `x-casaos` — both work; only one is **idiomatic for new code**. The same is true for volume paths and port handling. There is no single doc in the source tree that draws the line cleanly; the canonical idiom lives in the MCP-served documentation:

- `docs://concepts/compose-conventions` (the canonical conventions document)
- The `compose_authoring` MCP Prompt (which bundles conventions + 3 real catalog examples + the validator deny-list)
- The 137-app catalog at `catalog://index` (every app uses the canonical idioms — they're a working pattern set)

### What broke

An agent searching the source tree first will find the legacy patterns (more numerous, well-commented, in the parsers — they look authoritative). The MCP-served docs are the LATER opinion and the canonical answer, but they are not reachable by code search. The agent then **drifts off canonical guidance** without warning.

This is the same risk pattern as: "the README says X, the wiki says Y." When two source-of-truth claims exist, the reader picks the one they hit first.

### What we already did about the surface

Three PRs landed before this ADR:

1. **PR #655 (P0.3)** — `search_docs` now indexes concepts + OpenAPI + the catalog, so agents searching for `vaultwarden` or `install_app` actually find canonical hits instead of `{matches: []}`.
2. **PR #656 (P0.1)** — `browse_catalog`, `get_compose_conventions`, and `start_compose_authoring` are now Tools (not just Resources / Prompts), so chat-mode agents that only think in `tools/call` reach the canonical content.
3. **PR #657 (P1.5)** — `get_system_health` aggregates 4 surfaces so the agent doesn't fall back to ad-hoc shell commands.

Those fix the **surface**. This ADR fixes the **doctrine** so future code changes don't reopen the gap.

## Decision

**MCP-served documentation is the canonical source of truth for any "what should the agent / operator / new app do?" question. Code is implementation. Code MAY support legacy aliases for backward compatibility but MUST NOT be treated as the canonical answer.**

Three specific consequences:

1. **When code and MCP docs disagree, the MCP docs win.** If a parser reads `x-casaos`, fine — that's backward compat. If a code comment says "use `x-casaos` for new apps", it's WRONG and should be updated. The conventions document at `docs/concepts/compose-conventions.md` is the answer.
2. **Every new feature that affects agent-visible behavior MUST update the relevant MCP-served doc as part of the same PR.** New compose conventions → `compose-conventions.md` updated. New API endpoint → `<service>.yaml` OpenAPI spec updated. New ADR-level decision → ADR file + cross-referenced from concepts where relevant.
3. **CI gate (proposed, not yet implemented):** a script that diffs the canonical schemas (extracted from the MCP-served docs) against what the code actually accepts and flags drift. Out of scope for this ADR — see "Follow-up" below.

### Why MCP docs and not the source tree

Three reasons:

- **Reachability for agents.** Source code is only reachable through Read/Glob/Grep — tools that chat-mode MCP clients don't surface by default. MCP-served docs are reachable through the MCP server itself, which is what the agent is talking to.
- **Versioning.** MCP-served docs ship as part of the install (`/usr/share/powerlab/docs/concepts/*.md` and `/usr/share/powerlab/openapi/*.yaml`), so they match the running binary version. Source-tree docs are pinned to the developer's checkout, which may be ahead of or behind production.
- **Reviewability.** "Did this PR update the docs?" is a binary check on PR reviewers. "Did this PR update every comment in every file that might reference this convention?" is not — and the chat-mode test proved it.

### What this is NOT

- **Not a deprecation of `x-casaos`, /DATA/AppData/, etc.** Legacy parsers stay until the catalog is fully migrated (separate roadmap). Backward-compat is a separate decision from canonicality.
- **Not a mandate to rewrite existing code comments.** Comments that document legacy parsing logic are fine — they describe what the code does. Comments that recommend legacy idioms for new code are stale and should be fixed as encountered.
- **Not a freeze on code-level changes.** Code can ship before the MCP doc is rewritten when there's a clear hierarchy (e.g., an internal refactor that doesn't change behavior). The rule applies to **agent-visible behavior**.

## Consequences

### Positive

- **Agents that only see MCP get the canonical story.** This is the user-visible win: the bug from 2026-05-31 cannot reproduce as long as the doctrine holds.
- **Doc currency improves.** Tying agent-visible behavior to the MCP-served doc forces PRs to keep docs in sync. The cost is mostly visible at PR review time, which is the right place.
- **Reviewers get a clear question to ask.** "Does this PR change anything an agent sees? If yes, where's the doc update?" Replaces the open-ended "are the docs up to date?" with a binary check.

### Negative / risks

- **PR overhead.** Every behavior change now potentially carries a doc update. For trivial fixes this is friction; for substantive changes it's the right amount of friction.
- **Stale legacy comments stay around.** This ADR doesn't trigger a sweep of existing legacy idioms in code comments. They will accumulate until a contributor encounters them and fixes them organically. The CI gate (below) is the only mechanical mitigation.
- **The doctrine requires MCP docs to actually exist and be accurate.** If `compose-conventions.md` falls out of date, the agent gets canonical-but-wrong guidance. This is worse than today's "no clear canonical." Doc maintenance burden is the trade.

### Operator impact

None at install time. The doctrine affects developer + contributor workflow only.

## Follow-up

### Drift-detection CI gate (P1.4-followup)

A separate PR will add `scripts/check-mcp-docs-canonical.sh` that:

- Extracts the canonical compose-extension keys from `compose-conventions.md` (parses out the `x-powerlab.*` field names listed there).
- Greps `backend/app-management/codegen/extension.go` for the keys the parser actually reads.
- Fails on any key the parser reads that isn't listed in the canonical document, OR vice versa — except for explicitly allowlisted legacy keys (e.g. `x-casaos`) marked as "backward compat only" in the doc.

Same shape can be applied to:
- OpenAPI endpoint paths vs route registrations
- App catalog metadata fields vs `validate.py` (in the separate `powerlab-store` repo)

Out of scope for this ADR — captured here so it doesn't get lost. Tracking issue to be filed.

### Documentation sweep (best-effort)

Code comments encountered during normal work that recommend legacy idioms for new code should be updated to point at the canonical document. Not a sweep PR — an organic clean-up as files are touched.

## Revisit triggers

- The MCP docs become inaccurate or fall behind code reality — doctrine is unenforceable, revisit before formalizing further.
- A real-world enterprise customer challenges "MCP docs canonical" because their compliance team wants source-controlled docs only — revisit with a compromise (e.g., generate MCP docs from versioned source).
- Code-derived schemas and MCP-served schemas become fully generated from a single source — this ADR is then moot (both ARE canonical because they're the same).
