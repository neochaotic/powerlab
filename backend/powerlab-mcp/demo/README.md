# powerlab-mcp demo seed

Static fixtures the upcoming demo image (`ghcr.io/neochaotic/powerlab-mcp-demo`) bakes in so an isolated MCP sandbox like the Glama Inspector sees substantial responses — not a one-app catalog and three log lines.

## Layout

```
demo/
├── README.md              ← this file
├── catalog-pick.txt       ← 44-app subset of community-catalog/Apps/ to bake in
├── audit.jsonl            ← 50 realistic HTTP-audit entries spanning ~8h
└── journal/
    ├── gateway.log        ← request log + health ticks
    ├── app-management.log ← install/restart/uninstall lifecycle
    └── mcp.log            ← Tool-call + Resource-read traces
```

## Why this exists

`browse_catalog` on a fresh demo container with no catalog returns an empty list. `journal_search` on a container with no logs returns nothing. The Glama scorer (and any external eval) calling Tools sees thin responses and drops the score accordingly.

Baking in a curated 44-app catalog + 50-line audit + 3 PowerLab service logs lifts every read-side Tool from "structurally correct, empty result" to "shows real data — agent can reason from it".

## How the Dockerfile.demo consumes this

```dockerfile
# Catalog: copy only the picked subset
RUN xargs -I{} cp -r /workspace/community-catalog/Apps/{} \
    /var/lib/powerlab/catalog/Apps/{} < /workspace/backend/powerlab-mcp/demo/catalog-pick.txt

# Audit
COPY backend/powerlab-mcp/demo/audit.jsonl /var/log/powerlab/audit.jsonl

# Journals
COPY backend/powerlab-mcp/demo/journal/*.log /var/log/powerlab/
```

## What the fixtures demonstrate

The fixtures are deliberately shaped to surface Tool chains the prompts encode:

- **Failed install (audit row c-005 → app-management.log ERROR at 08:34:55Z)** — exercises `troubleshoot_install_failure` end-to-end: `audit_query` finds the failure, `journal_search` confirms the port collision, `get_system_health` shows the host was healthy → diagnosis = port conflict, not capacity.
- **Validator rejection (audit row c-015 → app-management.log ERROR at 09:41:18Z)** — `install_app` deny-list working: `privileged: true` rejected pre-flight.
- **Health-warn from updates** — `get_system_health` overall=warn with `updates_pending=7, security_updates=3` (audit row c-026). The disk + services + memory legs all=ok; agent should surface that the warn is operator-actionable (apt upgrade), not a service bug.

## Updating the fixtures

These fixtures are static and committed. When the Tool/Prompt/Resource surface materially changes, regenerate the audit/journal lines to keep the demo realistic. The `catalog-pick.txt` list should track community-catalog growth at a slow cadence (re-pick once per significant release, not per PR).

## Not staged here

- Docker socket — the demo image runs read-only Tools only; `EnableDestructiveTools=false`. The fixtures show install/restart/uninstall in their log + audit form, but the Tools themselves are NOT exposed in the demo's `tools/list`.
- Real user data — every actor is `demo-operator` or `agent:claude` or `system`; no real names per the `feedback_no_real_names_in_fixtures` convention.
