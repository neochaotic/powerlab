# powerlab-mcp — agent-visible error sanity

Audit performed 2026-06-04 covering every Tool registered in `server/tools_*.go`. Goal: verify no Tool surfaces language-internal noise (subprocess wrap, Go runtime panic strings, absolute install paths) to the agent.

## Forbidden substrings (the contract)

Codified in `server/tools_error_sanity_test.go::agentForbiddenLeakStrings()`. Any agent-visible Tool output (errors, `Note` fields, `Summary` fields) must NOT contain:

- `exit status` — subprocess wrap (the original P0)
- `panic:` — Go runtime panic
- `goroutine` — runtime stack frame language
- `/usr/local/go/` — Go install path from a wrapped chain
- `runtime.gopanic`, `runtime/debug.Stack` — panic-recovery internals

New Tools must extend the regression test rather than adding ad-hoc per-tool checks.

## Tool-by-tool finding

| Tool | Error path | Carrier | Sanitiser | Status |
| --- | --- | --- | --- | --- |
| `journal_search` | journalctl missing / ACL-denied / timeout | wrapped error | `classifyJournalReadError` + `sanitizeJournalErr` | ✅ Locked (5 tests in `tools_readonly_test.go`) |
| `browse_catalog` | catalog dir missing / unreadable / malformed JSON | `Note` field on success result | None — relies on `err.Error()` shape | ✅ Locked (new test in `tools_error_sanity_test.go`) |
| `get_compose_conventions` | conventions file missing | stub `Note`, no err chain | (n/a — stub) | ✅ Safe by construction |
| `start_compose_authoring` | sub-call to `buildComposeAuthoringResult` always returns; missing files become stubs | stub `Note` | (n/a — pure function over dirs) | ✅ Safe by construction |
| `get_system_health` | metrics/disk/services/updates upstream errors | `Summary` field with `err.Error()` | None | ⚠️ Carries `err.Error()` raw; analysed below |
| `check_disk_free` | path doesn't exist / statfs syscall error | `fmt.Errorf("%w", err)` chain | None | ✅ Errno is informative + non-leaky |
| `install_app` | composevalidator rejection / coreproxy HTTP error | structured `InstallAppOutput` / `coreproxy.AsErrorPayload` | Yes (validator returns typed Violations; coreproxy returns structured payload) | ✅ Locked |
| `uninstall_app` | invalid id / coreproxy HTTP error | structured error message / `coreproxy.AsErrorPayload` | Yes | ✅ Locked |
| `restart_app` | invalid id / coreproxy HTTP error | structured error message / `coreproxy.AsErrorPayload` | Yes | ✅ Locked |
| `search_docs` | substring search over preloaded corpus; returns empty result, no error path | (n/a) | (n/a) | ✅ Safe by construction |
| `list_capabilities` | pure config read; no error path | (n/a) | (n/a) | ✅ Safe by construction |
| `generate_artifact` | validator failure on `compose-yaml` kind | structured Violations | Yes | ✅ Locked |

## get_system_health — accepted residual risk

`tools_get_system_health.go` embeds raw `err.Error()` strings into `SystemHealthArea.Summary` for the metrics/disk/services/updates legs. Analysis:

- The errors originate from local `/proc` reads (memory) or `coreproxy` HTTP calls (disk/services/updates).
- `/proc` read errors only fire on non-Linux dev boxes — content is filesystem prose ("open /proc/meminfo: no such file or directory"), no secrets.
- `coreproxy` errors come from `coreproxy.AsErrorPayload`-shaped JSON OR a structured `dial tcp 127.0.0.1:<port>: connection refused` message. No Authorization headers, no Authorization tokens. The internal port is operator-known.
- The Summary field is operator-facing prose; informative-but-leaky internal paths are acceptable.

Conclusion: not a security issue, not an agent-quality issue. Tracked here so a future change that introduces a richer wrapped-error chain knows to sanitise before merging.

## How to extend

When a new Tool gains an error path that materialises into an agent-visible carrier (`Note`, `Summary`, or an explicit error return), add a row to the test in `tools_error_sanity_test.go` exercising that path. The shared `agentForbiddenLeakStrings()` list catches the language-internal noise classes automatically.
