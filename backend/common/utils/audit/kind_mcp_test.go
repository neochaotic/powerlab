package audit

import "testing"

// KindMCPToolCall + KindMCPResourceRead are the audit Kind values
// powerlab-mcp uses to discriminate its own records in audit.jsonl
// (ADR-0047). Locking them as constants so the wire token never
// drifts between writer (MCP) and reader (audit UI, audit:// MCP
// resource).
func TestKindMCP_Constants(t *testing.T) {
	if KindMCPToolCall != "mcp.tool_call" {
		t.Errorf("KindMCPToolCall = %q; want %q (locked wire token)", KindMCPToolCall, "mcp.tool_call")
	}
	if KindMCPResourceRead != "mcp.resource_read" {
		t.Errorf("KindMCPResourceRead = %q; want %q (locked wire token)", KindMCPResourceRead, "mcp.resource_read")
	}
	if KindMCPToolCall == KindMCPResourceRead {
		t.Errorf("KindMCPToolCall and KindMCPResourceRead must differ")
	}
}
