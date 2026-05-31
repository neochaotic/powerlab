# 0048. MCP docs surface — concepts, catalog, and the `compose_authoring` Prompt

- **Status:** proposed
- **Date:** 2026-05-31
- **Tracks:** Maintainer ask — *"o agente deve saber criar um yaml no padrão PowerLab"*
- **Builds on:** [ADR-0046](0046-mcp-tool-curation-strategy.md) (explicitly reserved flexibility for "meta-prompt / system context dumping" — this ADR cashes that in via the MCP Prompts primitive)

## Context

PowerLab's `community-catalog/Apps/` has 137 curated docker-compose YAMLs that embody the PowerLab pattern: `/DATA/PowerLabAppData/<id>/` volume paths, named user-facing networks, label conventions, healthcheck idioms. Agents asked to "design a compose for a new app" today have to discover this pattern by reading individual catalog files one-by-one — a discovery surface that exists (`apps://list` returns installed apps), but no curated "here's how PowerLab thinks about compose" surface exists.

The OpenAPI surface (`docs://api/<service>`) covers REST APIs but does not cover compose conventions. The mkdocs site at `docs/concepts/` covers them in prose (glossary, security-model, mcp-server) but is not exposed to MCP.

### Industry-best-practice survey (2026-05-31)

Surveyed the MCP community implementations + Anthropic / Block / Stripe / Cloudflare engineering posts for "how do servers expose documentation to agents":

| Pattern | Used by | Fit for PowerLab |
|---|---|---|
| **One resource per doc, raw markdown** | GitHub MCP server, mcp-server-filesystem (Anthropic reference) | ✅ Yes — `docs://concepts/{name}` template |
| **Index/schema resource** | All non-trivial servers | ✅ Yes — `docs://concepts/index` already part of `docs://api`'s pattern |
| **Search tool over docs** | Sentry MCP, search-server reference impl | ✅ Yes — `search_docs(query)` for cross-doc fuzzy match |
| **Parametric resource template for collections** | GitHub (issues, PRs), Stripe (objects) | ✅ Yes — `catalog://app/{id}` for 137 catalog YAMLs |
| **MCP Prompts primitive for curated bundles** | Stripe (`create_payment` flow), Block | ✅ Yes — `compose_authoring` Prompt bundles conventions + 3 catalog examples + validator rules |
| **Full-spec dumps into system prompt** | (anti-pattern) | ❌ Same rejection as ADR-0046 |

The consensus is **resources for stable docs + tool for search + Prompts primitive for curated context bundles**. The Prompts primitive is the killer feature for "create a compose YAML in PowerLab pattern": instead of N round-trips of discovery, the agent invokes one prompt and receives a curated bundle ready to ground the response.

## Decision

Four new MCP surfaces land in one feature bundle. All four use existing SDK primitives (no new transport, no new auth surface, no go.mod changes):

### 1. `docs://concepts/{name}` — resource template

Reads markdown files from a configured concepts directory (default: `/usr/share/powerlab/docs/concepts/`, staged by `scripts/package-linux.sh`). Path-traversal hardened: the `{name}` segment is canonicalized and rejected if it escapes the concepts directory or contains path separators (same pattern as `docs://api/{service}` in [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md)).

A companion `docs://concepts/index` resource lists the available concept files (one entry per `*.md` in the directory) so the agent's discovery flow is: read index → pick relevant concept → read it. Same `docs://api` discovery pattern that already works.

### 2. `catalog://app/{id}` + `catalog://index` — resource templates

Reads from `/var/lib/powerlab/community-catalog/Apps/<id>/docker-compose.yml`. The catalog directory path is configurable (`CatalogDir` in `mcp.conf`); default matches the install layout.

`catalog://index` lists app IDs available in the catalog (one entry per subdirectory with a `docker-compose.yml`). `catalog://app/{id}` returns the raw YAML — the agent reads it as a pattern example, not as something to install.

Path-traversal hardened identically to `docs://concepts/{name}`. Per memory `feedback_catalog_trust_policy`, the catalog ships with the install — these are curated trusted YAMLs, not user uploads.

### 3. `search_docs(query, top_k)` — tool

Substring search across the concepts directory. Returns up to `top_k` matches (default 5, ceiling 20) with `{file, line_number, snippet}` per hit. No regex, no fuzzy distance — simple case-insensitive substring keeps the implementation tight and predictable, and the agent can chain reads of the matching files for full context.

Side-effect class: **READ ONLY** (per ADR-0046's tier system).

### 4. `compose_authoring` — MCP Prompt primitive

The killer feature. Server-declared prompt the agent (or user, via clients that surface prompts) invokes with one argument:

- `app_type` (optional, free-text hint): e.g., `"stateful database"`, `"static web service"`, `"background worker"`. Empty string is valid — the prompt returns the conventions overview.

When invoked, the server builds a `GetPromptResult` whose messages bundle:

1. **System message**: the canonical PowerLab compose conventions (sourced from `docs/concepts/compose-conventions.md`, which this PR also adds).
2. **User message**: 3 representative catalog YAMLs picked deterministically from `community-catalog/Apps/` based on `app_type` (or a stable default trio when `app_type` is empty). Each shown with its filename header so the agent can refer back.
3. **User message**: the composevalidator deny-list (the rules `install_app` enforces before forwarding to app-management — agent learns what NOT to write).
4. **User message**: the user's actual question template, parameterized on `app_type`.

The agent receives one round-trip of curated context. Token cost ~ a few KB; reasoning quality dramatically higher than agents discovering catalog files via `apps://list` one-by-one.

Per ADR-0046 §6 (escape hatches), this is the "meta-prompt / system context dumping" pattern explicitly reserved. Industry consensus matured; PowerLab adopts it.

## Consequences

### Wins

- **Killer use case unlocked.** Agent designing a PowerLab compose is no longer a discovery scavenger hunt. One prompt invocation → ready-to-reason context bundle.
- **Reuses what exists.** The concepts directory is the mkdocs source. The catalog is already on disk via the install. Zero net-new infrastructure on the operational side.
- **Industry-pattern alignment.** Four new surfaces, all matching established MCP community patterns. No PowerLab invention; agent libraries / IDE integrations that "know how to talk to MCP servers" know how to talk to ours.
- **Threat model unchanged.** All four surfaces are READ ONLY. No new auth surface. No new write path. Concepts + catalog files are PowerLab-controlled (shipped with install), not user-controlled.

### Costs / risks

- **Concepts directory must be staged at install time.** `scripts/package-linux.sh` already stages OpenAPI specs (ADR-0044 packaging) — adding concepts is the same mechanism. The risk of the directory being missing on a real install is real and proven (the OpenAPI staging gap PR #609 fixed). **Mitigation:** integration test in `scripts/check-package-linux-powerlab-mcp_test.sh` asserts the concepts directory ships AND `docs://concepts/index` is non-empty on a fresh install.
- **Prompts primitive is first-use in this codebase.** No prior PowerLab MCP work has registered a prompt. SDK supports it (`server.AddPrompt(prompt, handler)`) and there is a conformance test in the SDK that exercises the round-trip; the impl risk is concentrated in one file. **Mitigation:** unit test covers `prompts/list` + `prompts/get` happy path + the `app_type=""` default path.
- **Catalog as inspiration vs source-of-truth.** Per memory `feedback_catalog_trust_policy`, catalog is "inspiration only, never ingestion" for IMAGES — but ships as PowerLab-curated files. Exposing catalog YAMLs as **read-only patterns the agent learns from** is consistent with that policy (it's not auto-installing anything; it's showing the agent what good looks like).
- **Token cost of `compose_authoring` bundle.** A 3-example bundle + conventions + validator rules is ~3-5KB of context. Acceptable — that's smaller than `apps://list` on a host with 20 installed apps.

### Operational changes

- `mcp.conf` gains two new optional keys: `ConceptsDir` (default `/usr/share/powerlab/docs/concepts`) and `CatalogDir` (default `/var/lib/powerlab/community-catalog`). Sample updated.
- `scripts/package-linux.sh` stages `docs/concepts/*.md` to `/usr/share/powerlab/docs/concepts/` alongside the OpenAPI specs already staged.
- `docs/concepts/compose-conventions.md` is the new canonical document the `compose_authoring` Prompt sources from. Authored as part of this ADR's implementation.

### Out of scope (separate work)

- **Catalog metadata** (description.md, icon.svg) — `catalog://app/{id}` returns the compose YAML only. Metadata may land later if agents want a structured manifest of the catalog.
- **Search index optimisation** — `search_docs` is a brute-force substring scan. For ~10-20 concept files this is sub-millisecond; for a future docs explosion we can add an index. Premature now.
- **Concept authoring tooling** — adding concepts is still `git commit` + `scripts/package-linux.sh` rebuild. A live "edit concepts via MCP" surface (write tool) is explicitly NOT in this scope per the READ-ONLY classification.

## Alternatives considered

- **Tool-only surface** (`get_concept(name)` + `list_concepts()` as tools): rejected. Stable docs are RESOURCES per the MCP spec; using tools forces agents to call them imperatively when they could discover via the resource list. Same reason `docs://api` is a resource family, not a tool.
- **Single mega-resource** (`docs://everything` returns one giant blob): rejected. Anti-pattern per ADR-0046's "no full-spec dumps in system prompt" rule. Forces the agent to chunk; index + per-doc resources are the established alternative.
- **Defer the Prompt primitive** (only ship resources + search tool now, Prompt later): rejected. The Prompts primitive is the killer feature for the stated use case; deferring it ships a half-design that the next PR has to immediately revisit. Either commit to the bundle or wait — half-shipping is the worst option.

## Acceptance criteria

- [ ] `backend/powerlab-mcp/server/resources_docs_concepts.go` registers `docs://concepts/{name}` resource template + `docs://concepts/index` resource; path-traversal hardened.
- [ ] `backend/powerlab-mcp/server/resources_catalog.go` registers `catalog://app/{id}` resource template + `catalog://index` resource; path-traversal hardened.
- [ ] `backend/powerlab-mcp/server/tools_search_docs.go` registers `search_docs(query, top_k)` tool with the READ ONLY marker.
- [ ] `backend/powerlab-mcp/server/prompts_compose_authoring.go` registers the `compose_authoring` MCP Prompt with the `app_type` argument; handler bundles conventions + 3 catalog examples + validator rules.
- [ ] `docs/concepts/compose-conventions.md` authored — the canonical document the prompt sources from.
- [ ] `mcp.conf.sample` declares `ConceptsDir` + `CatalogDir`; `config.Load` parses them with safe defaults.
- [ ] `scripts/package-linux.sh` stages `docs/concepts/*.md` to `/usr/share/powerlab/docs/concepts/`.
- [ ] `scripts/check-package-linux-powerlab-mcp_test.sh` asserts concepts staging + index non-empty post-install.
- [ ] Unit tests for each surface: resources (happy path + path-traversal reject + missing-file shape), tool (substring matching + top_k cap), prompt (`prompts/list` + `prompts/get` round-trip + empty-arg default).
- [ ] `cmd/smoke` extended with probes for all four surfaces.
- [ ] `docs/concepts/mcp-server.md` documents the new surfaces with the same depth as existing sections.

## References

- [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) — `docs://api` discovery pattern this extends
- [ADR-0046](0046-mcp-tool-curation-strategy.md) — explicitly reserved "meta-prompt / system context dumping" flexibility; this ADR cashes it in
- [memory: feedback_catalog_trust_policy](../../../memory/feedback_catalog_trust_policy.md) — catalog as inspiration; this ADR exposes it as a pattern source consistently
- [MCP spec — Prompts](https://spec.modelcontextprotocol.io/specification/server/prompts/) — primitive contract
- modelcontextprotocol/servers — community reference implementations the survey drew from
