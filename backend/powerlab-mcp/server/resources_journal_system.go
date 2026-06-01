package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/journal"
)

// Sensitive sysadmin tier (ADR-0049). These two URIs read the HOST
// auth journals — ssh.service, sshd.service, sudo, su — and are
// registered ONLY when EnableSensitiveTier=true in mcp.conf.
//
// Wire shape locked in journal/system.go (SystemEntry: ts, unit,
// hostname, message — NO _pid, NO _cmdline, NO _audit_session). The
// reflect+JSON test in journal/system_test.go is the load-bearing
// guard; the resource layer here just plumbs the runner output to
// the agent.
//
// Selectors (ssh.service / sshd.service / sudo / su) are fixed in
// journal/system.go::authSelectors — never agent-supplied. The agent
// supplies only the bounded `lines` + `since` query params and chooses
// between the two URIs (auth vs failures).
const (
	// Concrete URIs for the no-query case + the resources/list
	// advertisement. The SDK lists each AddResource entry; the
	// AddResourceTemplate entries supply the matching pattern for
	// concrete URIs that carry a query string.
	journalSystemAuthURI     = "journal://system/auth"
	journalSystemFailuresURI = "journal://system/failures"

	// RFC 6570 form-style query expansion ({?...}) so an agent that
	// passes ?lines=50&since=1h matches the template registered with
	// the SDK. Mirrors the journal://{unit}{?...} pattern in
	// resources_journal.go.
	journalSystemAuthTemplate     = "journal://system/auth{?lines,since}"
	journalSystemFailuresTemplate = "journal://system/failures{?lines,since}"
)

// registerJournalSystem registers the sensitive-tier resources. The
// caller (newMCPServer) MUST guard this with the enableSensitiveTier
// flag — calling this function unconditionally would defeat the gate.
//
// Each variant registers both a concrete AddResource (so the no-query
// URI is discoverable in resources/list) AND a URI template
// (AddResourceTemplate) so the agent's queried form
// (?lines=50&since=1h) matches and parses. Same dual registration as
// journal:// in resources_journal.go (the SDK's resources/list shows
// concrete URIs; templates dispatch concrete reads).
func registerJournalSystem(s *mcp.Server, run journal.Runner) {
	addSystemJournalResource(
		s, run,
		journalSystemAuthURI, journalSystemAuthTemplate,
		"Host auth journal",
		"Auth-relevant subset of the host systemd journal — ssh.service, sshd.service, sudo, su (ADR-0049). Query params: lines (default 100, max 500), since (journalctl --since value, e.g. '1 hour ago'). Wire shape per entry: {ts, unit, hostname, message} — _PID, _CMDLINE, _AUDIT_SESSION deliberately omitted (argv leaks secrets — same security promise as system://processes).",
		false,
	)

	addSystemJournalResource(
		s, run,
		journalSystemFailuresURI, journalSystemFailuresTemplate,
		"Host auth journal — failures only",
		"Same source as journal://system/auth but filtered to PRIORITY err..warning (the err and warning syslog levels — what an operator + agent want when triaging 'what went wrong with auth recently' without paging through every success line, ADR-0049). Same query params + wire shape as journal://system/auth.",
		true,
	)
}

// addSystemJournalResource wires one of the two sensitive URIs. Factored
// out so the auth + failures registrations share the parse/run/marshal
// path — differing only in the Failures flag set on the SystemQuery.
func addSystemJournalResource(s *mcp.Server, run journal.Runner, uri, template, name, description string, failures bool) {
	handler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		q := parseSystemJournalURI(req.Params.URI)
		q.Failures = failures
		entries, err := journal.ReadSystem(ctx, run, q)
		if err != nil {
			return nil, fmt.Errorf("read system journal: %w", err)
		}
		// nil → [] so a fresh box (no matching records) returns an
		// empty array on the wire, not "null" — matches the audit://
		// recent shape; the agent's parser doesn't need a special
		// case for "no entries".
		if entries == nil {
			entries = []journal.SystemEntry{}
		}
		b, err := json.Marshal(entries)
		if err != nil {
			return nil, fmt.Errorf("marshal system journal entries: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(req.Params.URI, string(b))}}, nil
	}

	// Concrete URI — shows up in resources/list, dispatches reads
	// that carry no query string.
	s.AddResource(
		&mcp.Resource{URI: uri, Name: name, Description: description, MIMEType: "application/json"},
		handler,
	)
	// URI template — dispatches reads with ?lines=...&since=... .
	// resources/list does NOT re-advertise this (templates are listed
	// separately under resources/templates/list), so this addition
	// doesn't double the surface.
	s.AddResourceTemplate(
		&mcp.ResourceTemplate{URITemplate: template, Name: name, Description: description, MIMEType: "application/json"},
		handler,
	)
}

// parseSystemJournalURI turns "journal://system/auth?lines=50&since=1h"
// into a journal.SystemQuery. There is NO unit param — the unit set is
// fixed in journal/system.go. Unparseable values fall back to the zero
// value (the journal package re-validates + clamps the lines bound).
func parseSystemJournalURI(raw string) journal.SystemQuery {
	u, err := url.Parse(raw)
	if err != nil {
		return journal.SystemQuery{}
	}
	vals := u.Query()
	q := journal.SystemQuery{
		Since: vals.Get("since"),
	}
	if n, err := strconv.Atoi(vals.Get("lines")); err == nil {
		q.Lines = n
	}
	return q
}
