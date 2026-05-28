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

const (
	journalSchemaURI = "journal://schema"
	// RFC 6570 form-style query expansion ({?...}) so the published
	// template both documents the params and lets the SDK's template
	// regex match concrete URIs that carry a query string.
	journalURITemplate = "journal://{unit}{?lines,since,priority}"
	// journalUnitsURI is the discovery resource: returns the names of
	// every powerlab-*.service unit the installer dropped into
	// /etc/systemd/system. The agent reads this once to know what
	// values it can pass as {unit} into journalURITemplate.
	journalUnitsURI = "journal://units"
)

// journalSchemaDoc is the self-description an agent reads to learn the
// journal:// field shape and parameters before querying — so it can
// filter intelligently instead of dumping logs.
const journalSchemaDoc = `{
  "description": "PowerLab service logs from the systemd journal, scoped to powerlab-*.service units.",
  "uri_template": "journal://{unit}?lines=N&since=T&priority=P",
  "fields": {
    "time": "RFC3339Nano timestamp, UTC",
    "unit": "systemd unit (always powerlab-*.service)",
    "priority": "syslog priority 0-7 (0=emerg, 3=err, 4=warning, 6=info, 7=debug)",
    "message": "log line"
  },
  "params": {
    "unit": "PowerLab service, e.g. 'core' or 'gateway' (canonicalised; access is scoped to powerlab-* units)",
    "lines": "max records returned (journalctl -n)",
    "since": "journalctl --since value, e.g. '1h' or '2026-05-28 00:00:00'",
    "priority": "journalctl -p filter, e.g. 'err'"
  }
}`

// registerJournal exposes journal://schema (self-describing) and the
// journal://{unit} template backed by run.
func registerJournal(s *mcp.Server, run journal.Runner) {
	schema := &mcp.Resource{
		URI:         journalSchemaURI,
		Name:        "Journal schema",
		Description: "Field and parameter reference for the journal:// resource.",
		MIMEType:    "application/json",
	}
	s.AddResource(schema, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(journalSchemaURI, journalSchemaDoc)}}, nil
	})

	tmpl := &mcp.ResourceTemplate{
		URITemplate: journalURITemplate,
		Name:        "PowerLab journal",
		Description: "Systemd journal entries for a PowerLab unit (scoped to powerlab-*.service). Query params: lines, since, priority. See journal://schema.",
		MIMEType:    "application/json",
	}
	s.AddResourceTemplate(tmpl, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		entries, err := journal.Read(ctx, run, parseJournalURI(req.Params.URI))
		if err != nil {
			return nil, fmt.Errorf("read journal: %w", err)
		}
		b, err := json.Marshal(entries)
		if err != nil {
			return nil, fmt.Errorf("marshal journal entries: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(req.Params.URI, string(b))}}, nil
	})
}

// registerJournalUnits exposes journal://units — the discovery resource
// listing PowerLab unit stems an agent can pass back as {unit}. systemdDir
// is normally /etc/systemd/system (the FHS path package-linux.sh's
// installer drops powerlab-*.service files to); tests pass a fixture.
//
// Missing or empty dir returns an empty list, NOT an error — a fresh
// box without the PowerLab installation in place must not fail the
// resource; it just reports zero discoverable units.
func registerJournalUnits(s *mcp.Server, systemdDir string) {
	s.AddResource(
		&mcp.Resource{
			URI:         journalUnitsURI,
			Name:        "PowerLab journal units",
			Description: "Discoverable PowerLab service unit names. Pass any entry as {unit} in journal://{unit} to read that service's logs.",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			units, err := journal.ListUnits(systemdDir)
			if err != nil {
				return nil, fmt.Errorf("list journal units: %w", err)
			}
			// nil → [] in the JSON, not the literal "null" — matches
			// the audit:// recent shape so the agent's parser doesn't
			// need a special case for "no units".
			if units == nil {
				units = []string{}
			}
			payload := struct {
				Description string   `json:"description"`
				Units       []string `json:"units"`
			}{
				Description: "PowerLab service units installed on this box (stems — pass back as {unit} in journal://{unit}).",
				Units:       units,
			}
			b, err := json.Marshal(payload)
			if err != nil {
				return nil, fmt.Errorf("marshal journal units: %w", err)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(journalUnitsURI, string(b))}}, nil
		},
	)
}

// parseJournalURI turns "journal://core?lines=50&since=1h&priority=err"
// into a journal.Query. The unit is the URI host; unparseable values
// fall back to their zero value (the journal package re-validates and
// scopes the unit).
func parseJournalURI(raw string) journal.Query {
	u, err := url.Parse(raw)
	if err != nil {
		return journal.Query{}
	}
	vals := u.Query()
	q := journal.Query{
		Unit:     u.Host,
		Since:    vals.Get("since"),
		Priority: vals.Get("priority"),
	}
	if n, err := strconv.Atoi(vals.Get("lines")); err == nil {
		q.Lines = n
	}
	return q
}

// textJSON builds JSON resource contents for uri.
func textJSON(uri, body string) *mcp.ResourceContents {
	return &mcp.ResourceContents{URI: uri, MIMEType: "application/json", Text: body}
}
