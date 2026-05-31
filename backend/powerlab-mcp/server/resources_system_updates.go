package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// updateEntry is one pending package update — package name, the
// version that's installed, the upgrade target. We deliberately do
// NOT include the changelog/description: keeping the wire shape
// minimal means agents can scan a 200-package box without exhausting
// context, and the "what does this update contain" question is what
// downstream tools / vendor pages are for.
type updateEntry struct {
	Package   string `json:"package"`
	Installed string `json:"installed,omitempty"`
	Candidate string `json:"candidate"`
	Security  bool   `json:"security,omitempty"` // apt marks security pocket as <distro>-security
}

// updatesPayload is the system://updates wire shape. The `detected`
// field is the *package manager* we ran against, not the OS — so an
// agent on RPM-land sees `detected:"none"` rather than an apt result
// it should not act on.
type updatesPayload struct {
	Detected      string        `json:"detected"`       // "apt" | "none"
	Count         int           `json:"count"`
	SecurityCount int           `json:"security_count"`
	Packages      []updateEntry `json:"packages"`
	Note          string        `json:"note,omitempty"` // human-readable hint on why detected="none" or partial data
}

// aptRunner is the testable seam — production wires execAptList; unit
// tests substitute a canned-output fn.
type aptRunner func(ctx context.Context) (stdout string, err error)

func execAptList(ctx context.Context) (string, error) {
	// `apt list --upgradable` is the stable parseable surface — `apt-get`
	// does not have an equivalent (`apt-get -s upgrade` is verbose +
	// machine-unfriendly). We run with -q to suppress the progress
	// noise apt emits to stderr.
	cmd := exec.CommandContext(ctx, "apt", "list", "--upgradable", "-q")
	out, err := cmd.Output()
	return string(out), err
}

// parseAptList consumes the line-based `apt list --upgradable` output:
//
//	Listing...
//	package/distro-pocket version arch [upgradable from: oldversion]
//	package/distro-security version arch [upgradable from: oldversion]
//
// The security flag is set when the source pocket name contains
// "-security" (Debian + Ubuntu convention).
func parseAptList(stdout string) updatesPayload {
	pl := updatesPayload{Detected: "apt", Packages: []updateEntry{}}

	sc := bufio.NewScanner(strings.NewReader(stdout))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Listing") || strings.HasPrefix(line, "WARNING") {
			continue
		}
		// "name/pocket version arch [upgradable from: oldver]"
		// Split first on space to separate the "name/pocket" head.
		head, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		name, pocket, _ := strings.Cut(head, "/")
		// rest = "version arch [upgradable from: oldver]"
		parts := strings.SplitN(rest, " ", 3)
		if len(parts) < 1 {
			continue
		}
		candidate := parts[0]

		installed := ""
		if upgradable := strings.Index(line, "[upgradable from: "); upgradable != -1 {
			tail := line[upgradable+len("[upgradable from: "):]
			if end := strings.IndexByte(tail, ']'); end != -1 {
				installed = tail[:end]
			}
		}

		entry := updateEntry{
			Package:   name,
			Installed: installed,
			Candidate: candidate,
			Security:  strings.Contains(pocket, "-security"),
		}
		pl.Packages = append(pl.Packages, entry)
		if entry.Security {
			pl.SecurityCount++
		}
	}
	pl.Count = len(pl.Packages)
	return pl
}

func collectUpdates(ctx context.Context, run aptRunner) updatesPayload {
	if run == nil {
		return updatesPayload{Detected: "none", Packages: []updateEntry{}, Note: "no package manager runner configured"}
	}
	out, err := run(ctx)
	if err != nil {
		// apt missing → exec.LookPath error wrapped in *exec.Error;
		// we surface "none" + note so the agent's parse logic stays
		// uniform across hosts.
		return updatesPayload{
			Detected: "none",
			Packages: []updateEntry{},
			Note:     fmt.Sprintf("apt not available on this host: %v", err),
		}
	}
	return parseAptList(out)
}

func registerSystemUpdates(s *mcp.Server) {
	registerSystemUpdatesWith(s, execAptList)
}

// registerSystemUpdatesWith is the testable seam.
func registerSystemUpdatesWith(s *mcp.Server, run aptRunner) {
	s.AddResource(&mcp.Resource{
		URI:         systemUpdatesURI,
		Name:        "System updates (pending OS package upgrades)",
		Description: "Pending OS package updates via 'apt list --upgradable' on Debian/Ubuntu. Returns {detected, count, security_count, packages[]} with each entry's installed + candidate version and a security boolean (true when the source pocket name contains '-security'). On non-apt hosts returns detected='none' + a note — agent should pattern-match on `detected` before reading `packages`.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		pl := collectUpdates(ctx, run)
		b, err := json.Marshal(pl)
		if err != nil {
			return nil, fmt.Errorf("marshal updates payload: %w", err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(systemUpdatesURI, string(b))}}, nil
	})
}
