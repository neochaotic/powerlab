package audit

// MCP-side Kind values (ADR-0047 — powerlab-mcp agent-identity
// propagation). The audit middleware's default empty Kind is the
// HTTP-request audit; these tokens discriminate records produced by
// powerlab-mcp so audit:// readers + the audit UI can render
// agent-driven actions differently from regular panel-driven HTTP
// requests.
//
// Locked as typed constants — adding a new MCP kind value is a
// one-line change here, but the existing tokens NEVER drift (the
// reader contract in the audit UI + audit:// MCP resource depends
// on stability).
const (
	// KindMCPToolCall — agent invoked a tool via /mcp tools/call.
	// Carries the resolved user from the JWT subject in the audit
	// record's UserID/Username fields (or the loopback sentinel
	// for trusted local calls).
	KindMCPToolCall = "mcp.tool_call"

	// KindMCPResourceRead — agent read a resource via /mcp
	// resources/read. Same identity propagation as the tool kind.
	KindMCPResourceRead = "mcp.resource_read"
)
