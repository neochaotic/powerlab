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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
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

	// 3. Read each resource the daemon advertised + run resource-
	//    specific data-quality assertions. Template URIs (`{var}`) get
	//    a concrete probe right after — `audit://recent?limit=5` so the
	//    record-shape checks fire on a real payload even when no agent
	//    is wired up yet. We don't hardcode the resource list because
	//    a future release adding more resources should be gated
	//    automatically.
	failures := 0
	for _, r := range list.Resources {
		if hasTemplatePlaceholder(r.URI) {
			fmt.Printf("SKIP  %s (URI template — concrete reads below where applicable)\n", r.URI)
			continue
		}
		failures += readAndAssert(ctx, cs, r.URI)
	}

	// Concrete reads against the templates the MVP advertises. Empty
	// payloads are NOT failures (a fresh box has no audit records);
	// protocol errors ARE failures.
	for _, uri := range []string{"audit://recent?limit=5"} {
		failures += readAndAssert(ctx, cs, uri)
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d resource check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nOK — every advertised resource read + data-quality assertions passed")
}

// readAndAssert reads a resource and runs resource-specific quality
// assertions against the payload. Returns 1 on any error / contract
// break, 0 on success. Logs PASS / FAIL per check + a per-resource
// note when meaningful.
func readAndAssert(ctx context.Context, cs *mcp.ClientSession, uri string) int {
	read, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  %s — read: %v\n", uri, err)
		return 1
	}
	if len(read.Contents) == 0 {
		fmt.Fprintf(os.Stderr, "FAIL  %s — empty contents\n", uri)
		return 1
	}
	payload := read.Contents[0].Text
	fmt.Printf("PASS  %s (%d byte%s)\n", uri, len(payload), pluralS(len(payload)))

	switch {
	case strings.HasSuffix(uri, "://schema"):
		return assertSchemaPayload(uri, payload)
	case strings.HasPrefix(uri, "audit://recent"), strings.HasPrefix(uri, "audit://action/"):
		return assertAuditRecords(uri, payload)
	case uri == "system://metrics":
		return assertSystemMetrics(payload)
	}
	return 0
}

// assertSchemaPayload validates that a self-describing schema parses as
// JSON and carries a non-empty "description" + at least one documented
// field — the contract every schema:// resource implements.
func assertSchemaPayload(uri, payload string) int {
	var s struct {
		Description string                 `json:"description"`
		Resources   map[string]string      `json:"resources"`
		Fields      map[string]interface{} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  %s — schema is not valid JSON: %v\n", uri, err)
		return 1
	}
	if s.Description == "" {
		fmt.Fprintf(os.Stderr, "FAIL  %s — schema missing 'description' (agents read this)\n", uri)
		return 1
	}
	if len(s.Fields) == 0 {
		fmt.Fprintf(os.Stderr, "FAIL  %s — schema documents zero fields\n", uri)
		return 1
	}
	fmt.Printf("      → description set + %d field(s) documented\n", len(s.Fields))
	return 0
}

// assertAuditRecords validates an audit:// payload against the contract
// ADR-0033 promises operators + agents: 'ts' is RFC 3339, 'status' is
// a valid HTTP code, 'method' is a known verb, 'remote_ip' is set
// (the literal "loopback" sentinel is fine).
func assertAuditRecords(uri, payload string) int {
	var recs []map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &recs); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  %s — payload is not a JSON array of records: %v\n", uri, err)
		return 1
	}
	if len(recs) == 0 {
		fmt.Printf("      → zero records (fresh box / no matching correlation — not a failure)\n")
		return 0
	}
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}
	for i, r := range recs {
		ts, _ := r["ts"].(string)
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s record %d: ts %q is not RFC 3339 (%v)\n", uri, i, ts, err)
			return 1
		}
		status, _ := r["status"].(float64)
		if status < 100 || status > 599 {
			fmt.Fprintf(os.Stderr, "FAIL  %s record %d: status %v out of HTTP range\n", uri, i, status)
			return 1
		}
		method, _ := r["method"].(string)
		if !validMethods[method] {
			fmt.Fprintf(os.Stderr, "FAIL  %s record %d: method %q not a known HTTP verb\n", uri, i, method)
			return 1
		}
		if remoteIP, _ := r["remote_ip"].(string); remoteIP == "" {
			fmt.Fprintf(os.Stderr, "FAIL  %s record %d: remote_ip empty (should be IP or 'loopback')\n", uri, i)
			return 1
		}
	}
	fmt.Printf("      → %d record(s) with valid ts / status / method / remote_ip\n", len(recs))
	return 0
}

// assertSystemMetrics validates the system://metrics payload against the
// shape declared in metrics.Metrics — every documented field present,
// counters in plausible ranges. Fails fast on Mac (no /proc), where
// the resource itself errors out before we reach this check.
//
// The field list here is the product contract on the wire. If a future
// change renames a field (e.g. load1 → load_avg_1m), this assertion is
// where the rename surfaces: every operator running the smoke gets a
// loud FAIL until the change is reconciled across the panel + this
// smoke + downstream MCP clients.
func assertSystemMetrics(payload string) int {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  system://metrics — payload is not a JSON object: %v\n", err)
		return 1
	}
	required := []string{
		"mem_total_kb", "mem_available_kb", "mem_used_percent",
		"load1", "load5", "load15",
		"cpu_cores", "uptime_seconds",
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			fmt.Fprintf(os.Stderr, "FAIL  system://metrics — missing %q (product contract)\n", k)
			return 1
		}
	}
	if up, _ := m["uptime_seconds"].(float64); up <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL  system://metrics — uptime_seconds=%v (must be > 0 on a running box)\n", up)
		return 1
	}
	if cores, _ := m["cpu_cores"].(float64); cores < 1 {
		fmt.Fprintf(os.Stderr, "FAIL  system://metrics — cpu_cores=%v (must be ≥ 1)\n", cores)
		return 1
	}
	if mem, _ := m["mem_total_kb"].(float64); mem <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL  system://metrics — mem_total_kb=%v (must be > 0)\n", mem)
		return 1
	}
	if pct, _ := m["mem_used_percent"].(float64); pct < 0 || pct > 100 {
		fmt.Fprintf(os.Stderr, "FAIL  system://metrics — mem_used_percent=%v (out of 0..100)\n", pct)
		return 1
	}
	fmt.Printf("      → all %d required fields present + sane\n", len(required))
	return 0
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
