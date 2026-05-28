// Command smoke is a focused end-to-end check of a running
// powerlab-mcp instance. It connects via the official MCP SDK over
// the same Streamable-HTTP transport real agents use, then lists +
// reads every resource the daemon ships and prints PASS/FAIL per
// resource.
//
// It exists so an operator can answer "is MCP actually working on
// my box?" with one command, without writing a Claude Desktop
// config or installing a separate MCP CLI:
//
//	# loopback (no token needed — the read-tier gate skips loopback)
//	go run ./backend/powerlab-mcp/cmd/smoke
//	# LAN (need a JWT from /v1/users/login or the pairing flow)
//	go run ./backend/powerlab-mcp/cmd/smoke \
//	    -endpoint http://192.168.18.142:9090 -token "$JWT"
//
// Exits 0 on full pass, 1 if any resource read fails — so it slots
// into a release-cut pre-flight or a periodic systemd-timer check.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:9090", "powerlab-mcp base URL")
	token := flag.String("token", "", "Bearer JWT for LAN endpoints (loopback doesn't need one)")
	timeout := flag.Duration("timeout", 15*time.Second, "per-resource timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// 1. Control endpoints first — surface obvious failures (binary not
	//    running, wrong port, gateway-in-front-of-us, etc.) before we
	//    spin up the MCP transport.
	if err := pingControl(ctx, *endpoint, *token); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL control endpoints: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS  /healthz + /version")

	// 2. MCP transport: connect, list, read every advertised resource.
	cli := mcp.NewClient(&mcp.Implementation{Name: "powerlab-mcp-smoke", Version: "1"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:   *endpoint + "/mcp",
		HTTPClient: bearerClient(*token),
	}
	cs, err := cli.Connect(ctx, transport, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL mcp connect: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = cs.Close() }()
	fmt.Println("PASS  mcp connect + initialize")

	list, err := cs.ListResources(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL resources/list: %v\n", err)
		os.Exit(1)
	}
	if len(list.Resources) == 0 {
		fmt.Fprintln(os.Stderr, "FAIL resources/list returned an empty set — daemon registered no resources")
		os.Exit(1)
	}
	fmt.Printf("PASS  resources/list (%d advertised)\n", len(list.Resources))

	// 3. Read each resource the daemon advertised. We don't hardcode a
	//    URI list — if a future release adds resources, this gates
	//    them automatically. URI templates (with `{var}`) can't be
	//    read directly; skip them and report.
	failures := 0
	for _, r := range list.Resources {
		if hasTemplatePlaceholder(r.URI) {
			fmt.Printf("SKIP  %s (URI template — provide a concrete read separately)\n", r.URI)
			continue
		}
		read, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: r.URI})
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s — read: %v\n", r.URI, err)
			failures++
			continue
		}
		if len(read.Contents) == 0 {
			fmt.Fprintf(os.Stderr, "FAIL  %s — empty contents\n", r.URI)
			failures++
			continue
		}
		fmt.Printf("PASS  %s (%d byte%s)\n", r.URI, len(read.Contents[0].Text), pluralS(len(read.Contents[0].Text)))
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d resource read(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nOK — every advertised resource read successfully")
}

// pingControl reaches /healthz then /version. /version returns a JSON
// body containing the build-time version stamp; we don't assert its
// shape (the SDK validates content-type later), just that the server
// is alive and the version isn't "private build" (indicating a
// dev/local binary the operator should know about).
func pingControl(ctx context.Context, base, token string) error {
	for _, path := range []string{"/healthz", "/version"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			return fmt.Errorf("new request %s: %w", path, err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
		}
	}
	return nil
}

// bearerClient returns an *http.Client that adds Authorization: Bearer
// to every request. The Streamable transport uses the client for all
// MCP traffic; loopback callers pass an empty token and get the stock
// client (the read-tier gate skips loopback).
func bearerClient(token string) *http.Client {
	if token == "" {
		return http.DefaultClient
	}
	base := http.DefaultTransport
	return &http.Client{Transport: bearerTransport{token: token, base: base}}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if b.token == "" {
		return nil, errors.New("bearerTransport called without a token")
	}
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(clone)
}

func hasTemplatePlaceholder(uri string) bool {
	for _, ch := range uri {
		if ch == '{' {
			return true
		}
	}
	return false
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
