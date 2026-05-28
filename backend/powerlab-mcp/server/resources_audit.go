package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/audittail"
)

const (
	auditSchemaURI       = "audit://schema"
	auditRecentTemplate  = "audit://recent{?limit}"
	auditActionTemplate  = "audit://action/{correlation_id}"
	auditRecentURI       = "audit://recent"
	auditActionURIPrefix = "audit://action/"
)

const auditSchemaDoc = `{
  "description": "PowerLab request audit trail (who did what, when), read from the gateway's JSONL audit log.",
  "resources": {
    "audit://recent{?limit}": "the most recent audit records (default 100, max 1000)",
    "audit://action/{correlation_id}": "all audit records tagged with one correlation id (X-Request-Id) — e.g. everything a single request or tool-call triggered"
  },
  "fields": {
    "ts": "RFC3339 timestamp, UTC",
    "method": "HTTP method",
    "path": "request path (carries the target service)",
    "status": "HTTP status code",
    "latency_us": "handler latency in microseconds",
    "user_id": "authenticated user id (null for loopback / unauthenticated)",
    "username": "denormalised username (null if no user)",
    "remote_ip": "client IP, or 'loopback'",
    "request_id": "correlation id (X-Request-Id) — the join key for audit://action"
  }
}`

// registerAudit exposes the audit trail read from auditPath: a
// self-describing schema, the recent tail, and a per-correlation-id view.
func registerAudit(s *mcp.Server, auditPath string) {
	s.AddResource(
		&mcp.Resource{URI: auditSchemaURI, Name: "Audit schema", Description: "Field and resource reference for audit://.", MIMEType: "application/json"},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(auditSchemaURI, auditSchemaDoc)}}, nil
		},
	)

	s.AddResourceTemplate(
		&mcp.ResourceTemplate{URITemplate: auditRecentTemplate, Name: "Recent audit", Description: "Most recent PowerLab audit records (default 100, max 1000). See audit://schema.", MIMEType: "application/json"},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			recs, err := audittail.Recent(auditPath, auditLimitFromURI(req.Params.URI))
			if err != nil {
				return nil, fmt.Errorf("read audit recent: %w", err)
			}
			return auditResult(req.Params.URI, recs)
		},
	)

	s.AddResourceTemplate(
		&mcp.ResourceTemplate{URITemplate: auditActionTemplate, Name: "Audit by correlation id", Description: "All audit records for one correlation id (X-Request-Id) — what a single request or tool-call triggered.", MIMEType: "application/json"},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			id := correlationIDFromURI(req.Params.URI)
			if id == "" {
				return nil, fmt.Errorf("audit://action requires a correlation id")
			}
			recs, err := audittail.ByCorrelation(auditPath, id, 0)
			if err != nil {
				return nil, fmt.Errorf("read audit by correlation: %w", err)
			}
			return auditResult(req.Params.URI, recs)
		},
	)
}

// auditResult marshals records into a JSON resource result.
func auditResult(uri string, recs []audittail.Record) (*mcp.ReadResourceResult, error) {
	b, err := json.Marshal(recs)
	if err != nil {
		return nil, fmt.Errorf("marshal audit records: %w", err)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(uri, string(b))}}, nil
}

// auditLimitFromURI pulls ?limit=N out of an audit://recent URI; 0 (the
// audittail default) when absent or unparseable.
func auditLimitFromURI(raw string) int {
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(u.Query().Get("limit"))
	if err != nil {
		return 0
	}
	return n
}

// correlationIDFromURI extracts the id from "audit://action/<id>".
func correlationIDFromURI(raw string) string {
	return strings.TrimPrefix(raw, auditActionURIPrefix)
}
