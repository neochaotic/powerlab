package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// parseAptList is the parser the apt shell-out feeds into; covering
// it directly keeps the contract one assertion per row.
func TestParseAptList_HandlesAllShapes(t *testing.T) {
	out := `Listing...
WARNING: apt does not have a stable CLI interface. Use with caution in scripts.
libssl3/jammy-security 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.17]
curl/jammy-updates 7.81.0-1ubuntu1.20 amd64 [upgradable from: 7.81.0-1ubuntu1.19]
openssl/jammy-security 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.17]

ubuntu-advantage-tools/jammy-updates 32.3.2~22.04 amd64 [upgradable from: 32.3]
`
	pl := parseAptList(out)
	if pl.Detected != "apt" {
		t.Errorf("Detected=%q want apt", pl.Detected)
	}
	if pl.Count != 4 {
		t.Errorf("Count=%d want 4 (listing/warning/blank lines ignored)", pl.Count)
	}
	if pl.SecurityCount != 2 {
		t.Errorf("SecurityCount=%d want 2 (libssl3 + openssl on -security pocket)", pl.SecurityCount)
	}
	// Spot-check libssl3 — it's the security canary.
	var libssl updateEntry
	for _, e := range pl.Packages {
		if e.Package == "libssl3" {
			libssl = e
			break
		}
	}
	if libssl.Package == "" {
		t.Fatalf("libssl3 missing from parsed list")
	}
	if libssl.Installed != "3.0.2-0ubuntu1.17" || libssl.Candidate != "3.0.2-0ubuntu1.18" || !libssl.Security {
		t.Errorf("libssl3 parsed wrong: %+v", libssl)
	}
}

// On a non-apt host (Mac dev box, RPM-land, container without apt)
// the resource must NOT error — it returns detected="none" + a note
// so the agent pattern-matches on `detected` before reading
// `packages`. Same defensive shape as proxy errors.
func TestUpdates_NoAptHostReturnsStructuredNone(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerSystemUpdatesWith(srv, func(_ context.Context) (string, error) {
		return "", errors.New(`exec: "apt": executable file not found in $PATH`)
	})
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemUpdatesURI})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	var pl updatesPayload
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &pl); uerr != nil {
		t.Fatalf("payload not JSON: %v", uerr)
	}
	if pl.Detected != "none" {
		t.Errorf("Detected=%q want none", pl.Detected)
	}
	if pl.Count != 0 || len(pl.Packages) != 0 {
		t.Errorf("Count=%d Packages=%d — both should be 0 on a no-apt host", pl.Count, len(pl.Packages))
	}
	if pl.Note == "" {
		t.Errorf("Note empty — agent needs a hint about WHY detected=none")
	}
}

// Happy path through the canned-runner seam — proves the registration
// → runner → parser → JSON path round-trips with real apt-list lines.
func TestUpdates_HappyPathHasSecurityFlag(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerSystemUpdatesWith(srv, func(_ context.Context) (string, error) {
		return `Listing...
libssl3/jammy-security 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.17]
curl/jammy-updates 7.81.0-1ubuntu1.20 amd64 [upgradable from: 7.81.0-1ubuntu1.19]
`, nil
	})
	cs := connectInProcess(t, srv)
	defer func() { _ = cs.Close() }()

	res, err := cs.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: systemUpdatesURI})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	var pl updatesPayload
	if uerr := json.Unmarshal([]byte(res.Contents[0].Text), &pl); uerr != nil {
		t.Fatalf("payload not JSON: %v", uerr)
	}
	if pl.Count != 2 || pl.SecurityCount != 1 {
		t.Errorf("Count=%d SecurityCount=%d want 2/1", pl.Count, pl.SecurityCount)
	}
}
