# 0049. MCP sensitive sysadmin tier — `journal://system/*` threat model

- **Status:** proposed
- **Date:** 2026-05-31
- **Tracks:** Original deferred item ("PR 3.5 — sysadmin tier (sensitive, opt-in): system/auth journal + threat model ADR") from the [v0.7.5 MCP foundation roadmap](0034-standalone-observability-mcp-service.md)
- **Builds on:** [ADR-0034](0034-standalone-observability-mcp-service.md) (MCP foundation), [ADR-0044](0044-mcp-hybrid-architecture-thin-proxy-to-core.md) (hybrid pattern), [ADR-0046](0046-mcp-tool-curation-strategy.md) (tool tier + opt-in gates), [ADR-0047](0047-mcp-agent-identity-propagation.md) (audit dogfood — will record sensitive-tier reads when implemented)

## Context

PowerLab's MCP server today exposes `journal://{unit}` strictly scoped to PowerLab units — `canonicalUnit("anything")` rewrites the argument to `powerlab-<x>.service` so an agent cannot read `/var/log/auth.log`, `journalctl -u ssh.service`, or any other host journal that doesn't carry the `powerlab-` prefix. That was the right default for the MVP: the LAN-trust model assumed any caller with a valid JWT might be hostile-curious-enough to enumerate SSH attempts or `sudo` invocations.

But the enterprise pivot (memory `project_enterprise_pivot`) named a real use case: **"answer 'did anyone try to log in last night?' or 'what privileged actions ran during the maintenance window?' through the MCP surface, so the operator's AI is a first-class observability client across the WHOLE box, not just the PowerLab services."**

Doing that means exposing host-level authentication journals. That data is sensitive in ways PowerLab service journals aren't:

- **SSH attempt logs** (`journalctl _SYSTEMD_UNIT=ssh.service`) name the usernames being probed by attackers + the source IPs. An agent reading them sees the attack surface; an attacker WITH the agent's JWT sees the same view.
- **`sudo` invocations** (`journalctl _COMM=sudo`) log the user + the command argv. Operators routinely pass file paths, hostnames, sometimes accidentally a credential as a flag, in those argvs.
- **Login session events** (`journalctl -t login`) reveal which TTYs are active + who logged in from where.

Reading any of this is **legitimate enterprise observability**. Exposing it default-on would be reckless.

This ADR locks the threat model + the gate + the resource design **before** the code (similar to how [ADR-0046](0046-mcp-tool-curation-strategy.md) locked the destructive-tools gate before `install_app` shipped).

## Decision

A new tier of MCP resources — `journal://system/*` — ships **off by default** and only registers when the operator flips `EnableSensitiveTier = true` in `mcp.conf`. Same shape as the existing `EnableDestructiveTools` gate ([ADR-0046](0046-mcp-tool-curation-strategy.md) batch 3): when false, the resources are NOT registered — they don't appear in `resources/list`, the agent has no URI to address, the surface effectively doesn't exist.

### Resources shipped behind the gate

| URI | What it reads | Why an operator + agent want it |
|---|---|---|
| `journal://system/auth?{lines,since}` | `journalctl _SYSTEMD_UNIT=ssh.service _SYSTEMD_UNIT=sshd.service _COMM=sudo _COMM=su -o json` — the auth-relevant subset of the host journal | "Did anyone try to log in?" / "Who ran sudo today?" / "Are we under an SSH brute force?" |
| `journal://system/failures?{lines,since}` | Same source as above, filtered to PRIORITY=warning..error | Faster path to "what went wrong with auth in the last hour" without paging through every success line |

Each entry is shaped:

```json
{
  "ts": "2026-05-31T12:34:56Z",
  "unit": "ssh.service",
  "hostname": "powerlab-host",
  "message": "Failed password for invalid user root from 198.51.100.42 port 51234 ssh2"
}
```

**Fields deliberately omitted from the wire**:
- `_PID` — adds noise, no operator value, leaks process IDs to the agent.
- `_CMDLINE` — sudo's full argv lives here; argv routinely carries secrets (passwords passed as flags, signed URLs, JWT tokens via env expansion). Same reasoning as `system://processes`' "name only, no cmdline" promise from the v0.7.6 sysadmin tier.
- `_AUDIT_SESSION` / `_SELINUX_CONTEXT` — kernel internals an agent reasoning about auth events does not need.

`MESSAGE` is kept intact — the operator + agent need the actual log line to reason ("Failed password for `<user>` from `<IP>`"). The cost: a `sudo` invocation that logs the full command line via the `LOG_INFO` path (rare; `pam_unix` style, not the default `sudoers.log` style) would surface that command in `MESSAGE`. **Documented as a known limit** — operators who enable this tier accept they're reading raw auth journals.

### Gate semantics

- **Default**: `EnableSensitiveTier = false`. Resources NOT registered.
- **Enable**: operator edits `/etc/powerlab/mcp.conf`, sets `EnableSensitiveTier = true`, runs `sudo systemctl restart powerlab-mcp`. Resources appear in `resources/list`.
- **Disable**: flip back to `false`, restart. Resources disappear from `resources/list`. Operator does NOT need to revoke any agent tokens.

The gate is a **single switch for the whole tier**. Per-resource gates would compound the operator confusion ("which combination of flags exposes what?"). One flag, documented threat model, all-or-nothing.

### Auth + audit interaction

- Same JWT gate as the rest of MCP. Loopback exempt remains (trusted local agent on the box).
- When [ADR-0047](0047-mcp-agent-identity-propagation.md) (audit dogfood) lands, every read of `journal://system/*` will write one `audit.jsonl` record with `kind = "mcp.resource_read"` + the validated user. The compliance trail will answer "which agent read which sensitive log when".

### Read limits

Same bounds the existing `journal://{unit}` enforces, slightly tighter:

- `lines` default: 100, ceiling: 500 (vs PowerLab journal's 200 default / 2000 ceiling). Each sensitive entry is heavier per-record from a leakage perspective; bounded reads reduce per-call exfil if a token is compromised.
- `since` is an opaque string forwarded to journalctl's `--since` flag (e.g., `"1 hour ago"`, `"2026-05-31"`). Operator-set bounds via journalctl's own parser.

## Alternatives considered

| Option | Why considered | Why rejected |
|---|---|---|
| **A** — Always-on, no opt-in | Removes operator-action friction | Reckless default. Sensitive logs leak too easily under any compromised-token scenario. |
| **B** — Per-resource opt-ins (one knob per surface) | More granular | Confusing operator surface ("which combination shows what?"); the threat model is the same across the tier. |
| **C** — Filter `MESSAGE` to redact IPs / usernames | Mitigates worst-case data exposure | Defeats the purpose. "Did anyone try to log in?" requires seeing the username and IP. Redaction makes the surface useless for the legitimate observability case. |
| **D** — Tool instead of resource (`search_auth_log(query)`) | Tools are more rate-limit-able + don't auto-discover | Resources match the existing `journal://` shape; an agent that already knows how to read `journal://gateway` knows how to read `journal://system/auth`. Tool path adds learning curve for no security gain (the same data still flows). |
| **E — Adopt (chosen)** | Operator-controlled opt-in, single switch, mirrors the destructive-tools gate operators already know | Selected. |

## Consequences

### Wins

- **Enterprise observability completed** for the read side. Combined with the existing `journal://gateway` + `audit://recent`, an MCP agent can answer essentially every "what happened on this box" question an operator could answer themselves with `journalctl + tail`.
- **Threat model is explicit**. Operator who flips the gate accepts the documented exposure ("agent will see SSH attempts + sudo invocations"); operator who leaves it off accepts the documented limit ("MCP cannot answer host-level auth questions").
- **Consistent with existing tier gating** (`EnableDestructiveTools`). One mental model; not a new gating mechanism.

### Costs / risks

- **The MESSAGE-carries-command-args edge case.** If an operator runs `sudo command --password=hunter2`, and their PAM stack logs the full argv, MCP would surface that in `journal://system/auth`. **Mitigation:** documented as a known limit; the threat model ADR is explicit; future redaction layer is out of scope but tracked as a follow-up if a real incident hits.
- **Token-compromise blast radius widens.** A leaked JWT now grants read on auth journals when the gate is on. **Mitigation:** same JWT controls already exist; the audit dogfood ([ADR-0047](0047-mcp-agent-identity-propagation.md)) records every read so post-incident forensics are possible.
- **Operator support burden.** "I enabled it but my agent says the resource isn't there" / "I disabled it but I want to re-enable" — flip + restart cycle. **Mitigation:** the operator quickstart at `docs/operations/mcp-quickstart.md` gets a section.
- **Wire shape future-proofing.** Adding fields to the entry shape is forward-compatible (agents ignore unknown JSON keys); REMOVING fields would break consumers. Lock the field set conservatively for v0.7.6, expand only with operator-visible reasoning.

### Operational changes

- `mcp.conf` gains one new key: `EnableSensitiveTier` (default false). `mcp.conf.sample` documents the threat model inline.
- `journal://system/auth` + `journal://system/failures` appear in `resources/list` only when the gate is on.
- `docs/operations/mcp-quickstart.md` gains a "Step 5.5 — opt in to sensitive journals" section between the existing Step 5 (destructive tools) and Step 6 (troubleshooting). Same opt-in narrative pattern.
- `docs/concepts/mcp-server.md` gets a new row in the system:// / journal:// resource table.

### Out of scope

- **`journal://system/btmp`** + **`journal://system/wtmp`** — these need `last(1)` / `lastb(1)` not `journalctl`. The format is different (binary) + requires different argv. If operators ask, follow-up PR. Not in this scope to keep the design crisp.
- **Redaction of MESSAGE** — see "Costs" above. Documented limit; future work if a real incident demands it.
- **`tools_security` family** (active threat-response tools: ban IPs, revoke tokens, kill SSH session) — explicitly NOT in this tier. This is read-only observability. Active mutation would need its own threat model ADR.

## Acceptance criteria

- [ ] `config.Config` gains `EnableSensitiveTier bool` (default false).
- [ ] `mcp.conf.sample` declares the key inline with the threat-model comment.
- [ ] `backend/powerlab-mcp/journal/system.go` (new file) declares `SystemQuery` + `BuildSystemArgs(SystemQuery) []string` — pure, testable, builds the journalctl argv for the sensitive surface.
- [ ] `backend/powerlab-mcp/server/resources_journal_system.go` (new file) registers `journal://system/auth` + `journal://system/failures` **only when** `rc.enableSensitiveTier` is true.
- [ ] Wire shape: `{ts, unit, hostname, message}` — NO `_pid`, NO `_cmdline`, NO `_audit_session`. Wire-key stability test (mirrors `processes_test.go::TestProcessSummary_NeverLeaksCmdline`).
- [ ] Gate test: when flag is false, `resources/list` does NOT advertise `journal://system/*` and `ReadResource` on those URIs returns "not found"; when flag is true, both appear AND read.
- [ ] Bounds: `lines` default 100, ceiling 500. Edge cases tested (negative, zero, > ceiling).
- [ ] `docs/operations/mcp-quickstart.md` Step 5.5 added.
- [ ] `docs/concepts/mcp-server.md` updated.

## References

- [ADR-0034](0034-standalone-observability-mcp-service.md) — MCP foundation; this ADR closes the deferred sysadmin-sensitive-tier item.
- [ADR-0046](0046-mcp-tool-curation-strategy.md) — established the `EnableDestructiveTools` opt-in pattern this ADR mirrors.
- [ADR-0047](0047-mcp-agent-identity-propagation.md) — audit dogfood; will record sensitive-tier reads when implemented.
- [memory: project_enterprise_pivot](../../../memory/project_enterprise_pivot.md) — design lens that promoted this work.
- [memory: feedback_security_is_priority](../../../memory/feedback_security_is_priority.md) — "smaller-and-safer beats larger-and-warned"; honoured here via the opt-in gate.
