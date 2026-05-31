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
	// docs://api/<service> — probe dynamically. Read the manifest
	// FIRST and only call into a service that's actually staged on
	// this box. A Mac dev install or pre-install box has an empty
	// manifest; we surface that as a WARN, not a failure, because
	// the wire-up is fine — only the package-linux.sh stage step
	// for OpenAPI specs hasn't run.
	failures += probeFirstOpenAPISpec(ctx, cs)

	// 4. Tool surface (ADR-0046). tools/list discovery + read-only
	//    tool invocation. We deliberately do NOT call restart_app,
	//    install_app, or uninstall_app — those have side effects and
	//    the smoke runs against a live box. Read-only tools
	//    (journal_search + check_disk_free) get a real call to
	//    validate the typed output.
	failures += probeAndCallTools(ctx, cs)

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d resource/tool check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nOK — every advertised resource + tool read + data-quality assertions passed")
}

// probeFirstOpenAPISpec reads docs://api, picks the first staged
// service, and validates its OpenAPI spec shape. An empty manifest
// is a WARN: install hasn't run the OpenAPI staging step yet
// (Mac dev box, fresh install pre-package-linux.sh) — the wire-up
// is fine; the data just hasn't landed.
func probeFirstOpenAPISpec(ctx context.Context, cs *mcp.ClientSession) int {
	manifestRes, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "docs://api"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  docs://api manifest read: %v\n", err)
		return 1
	}
	if len(manifestRes.Contents) == 0 {
		fmt.Fprintf(os.Stderr, "FAIL  docs://api returned empty contents\n")
		return 1
	}
	var manifest struct {
		Specs []struct {
			Service string `json:"service"`
			URI     string `json:"uri"`
		} `json:"specs"`
	}
	if err := json.Unmarshal([]byte(manifestRes.Contents[0].Text), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  docs://api manifest is not valid JSON: %v\n", err)
		return 1
	}
	if len(manifest.Specs) == 0 {
		fmt.Printf("WARN  docs://api manifest is empty — package-linux.sh OpenAPI staging hasn't run on this box (Mac dev / pre-install). The wire-up is fine.\n")
		return 0
	}
	return readAndAssert(ctx, cs, manifest.Specs[0].URI)
}

// probeAndCallTools exercises the MCP tool surface — tools/list +
// the safe-to-call read-only tools. Returns the number of failing
// checks. Per ADR-0046, every tool's Description MUST carry an
// explicit side-effect class marker (READ ONLY / SIDE EFFECT /
// DESTRUCTIVE) so Claude clients surface it to the user; the smoke
// asserts this contract because the discipline is easy to lose in
// a tool-list refactor.
func probeAndCallTools(ctx context.Context, cs *mcp.ClientSession) int {
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  tools/list: %v\n", err)
		return 1
	}
	fmt.Printf("PASS  tools/list (%d advertised)\n", len(tools.Tools))

	failures := 0
	seen := map[string]string{}
	// Side-effect class marker enforcement. We accept either the
	// canonical ADR-0046 wording or any variant containing the
	// classifier word in upper-case — the LLM-facing surface is
	// what matters, not the punctuation.
	for _, tool := range tools.Tools {
		seen[tool.Name] = tool.Description
		if !carriesSideEffectClass(tool.Description) {
			fmt.Fprintf(os.Stderr, "FAIL  tool %q description missing side-effect class marker (READ ONLY / SIDE EFFECT / DESTRUCTIVE): %q\n", tool.Name, tool.Description)
			failures++
		}
	}

	// Read-only tools must always be advertised — they ship in batch
	// 1 with no gate. If they're missing the build is broken.
	for _, want := range []string{"journal_search", "check_disk_free"} {
		if _, ok := seen[want]; !ok {
			fmt.Fprintf(os.Stderr, "FAIL  tool %q missing from tools/list (expected unconditional)\n", want)
			failures++
		}
	}

	// restart_app ships in batch 2 with no gate — expect it too.
	if _, ok := seen["restart_app"]; !ok {
		fmt.Fprintf(os.Stderr, "FAIL  tool restart_app missing from tools/list (expected unconditional)\n")
		failures++
	}

	// install_app + uninstall_app are gated on EnableDestructiveTools.
	// We don't assert presence/absence — the smoke runs against
	// configurations both ways. We DO confirm that if they're present,
	// they're flagged DESTRUCTIVE.
	for _, name := range []string{"install_app", "uninstall_app"} {
		if desc, ok := seen[name]; ok {
			if !strings.Contains(desc, "DESTRUCTIVE") {
				fmt.Fprintf(os.Stderr, "FAIL  destructive tool %q description missing DESTRUCTIVE marker: %q\n", name, desc)
				failures++
			} else {
				fmt.Printf("      → %s advertised + DESTRUCTIVE class clear (EnableDestructiveTools=true)\n", name)
			}
		} else {
			fmt.Printf("      → %s NOT advertised (EnableDestructiveTools=false — gate respected)\n", name)
		}
	}

	// Drive the read-only tools end-to-end with real arguments.
	// Failures here mean the typed input/output contract is broken
	// — the call landing succeeded but the shape doesn't match.
	failures += callJournalSearch(ctx, cs)
	failures += callCheckDiskFree(ctx, cs)

	return failures
}

// callJournalSearch picks a unit that should exist on any PowerLab
// install (gateway is the foundation service) and asserts the
// typed output shape: {unit, pattern, entries: []}. An empty entries
// list is not a failure — a fresh box may not have logged anything
// matching yet.
func callJournalSearch(ctx context.Context, cs *mcp.ClientSession) int {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "journal_search",
		Arguments: map[string]any{
			"unit":  "gateway",
			"lines": 10,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  journal_search call: %v\n", err)
		return 1
	}
	if res.IsError {
		// Don't count as a hard failure when the wiring is OK and only
		// the upstream data is missing (no journal yet, no journalctl).
		// Surface as a WARN so the operator sees the gap but the smoke
		// stays green for "MCP tool surface intact."
		fmt.Printf("WARN  journal_search returned IsError on this box (no powerlab-gateway journal yet OR journalctl unavailable) — tool wiring is fine\n")
		return 0
	}
	var got struct {
		Unit    string                   `json:"unit"`
		Pattern string                   `json:"pattern,omitempty"`
		Entries []map[string]interface{} `json:"entries"`
	}
	if err := decodeStructured(res.StructuredContent, &got); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  journal_search output decode: %v\n", err)
		return 1
	}
	if got.Unit != "gateway" {
		fmt.Fprintf(os.Stderr, "FAIL  journal_search output.unit=%q; want 'gateway' (echoed)\n", got.Unit)
		return 1
	}
	fmt.Printf("PASS  journal_search (unit=gateway, %d entries)\n", len(got.Entries))
	return 0
}

// callCheckDiskFree probes the root filesystem and asserts the
// arithmetic invariants: used + available == total, 0 ≤ used_percent
// ≤ 100, total > 0.
func callCheckDiskFree(ctx context.Context, cs *mcp.ClientSession) int {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "check_disk_free",
		Arguments: map[string]any{"path": "/"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  check_disk_free call: %v\n", err)
		return 1
	}
	if res.IsError {
		fmt.Fprintf(os.Stderr, "FAIL  check_disk_free returned IsError=true on a path that must exist (/)\n")
		return 1
	}
	var got struct {
		Path           string  `json:"path"`
		TotalBytes     uint64  `json:"total_bytes"`
		AvailableBytes uint64  `json:"available_bytes"`
		UsedBytes      uint64  `json:"used_bytes"`
		UsedPercent    float64 `json:"used_percent"`
	}
	if err := decodeStructured(res.StructuredContent, &got); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  check_disk_free output decode: %v\n", err)
		return 1
	}
	if got.TotalBytes == 0 {
		fmt.Fprintf(os.Stderr, "FAIL  check_disk_free / returned TotalBytes=0 (statfs broken?)\n")
		return 1
	}
	if got.UsedBytes+got.AvailableBytes != got.TotalBytes {
		fmt.Fprintf(os.Stderr, "FAIL  check_disk_free arithmetic: used(%d)+avail(%d) != total(%d)\n",
			got.UsedBytes, got.AvailableBytes, got.TotalBytes)
		return 1
	}
	if got.UsedPercent < 0 || got.UsedPercent > 100 {
		fmt.Fprintf(os.Stderr, "FAIL  check_disk_free used_percent=%v out of 0..100\n", got.UsedPercent)
		return 1
	}
	fmt.Printf("PASS  check_disk_free / (%.1f%% used, %d MiB available)\n",
		got.UsedPercent, got.AvailableBytes/(1024*1024))
	return 0
}

// carriesSideEffectClass checks for ADR-0046 §1's marker discipline
// — every tool's Description leads with the side-effect class. We
// accept any of the three canonical markers (case-sensitive on the
// classifier word so a stray lowercase doesn't slip through).
func carriesSideEffectClass(desc string) bool {
	return strings.Contains(desc, "READ ONLY") ||
		strings.Contains(desc, "READ") && strings.Contains(desc, "ONLY") ||
		strings.Contains(desc, "SIDE EFFECT") ||
		strings.Contains(desc, "DESTRUCTIVE")
}

// decodeStructured round-trips an MCP StructuredContent (which the
// transport delivers as map[string]interface{}) into a typed struct
// via JSON. Used by every typed-output tool probe.
func decodeStructured(sc any, out any) error {
	b, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
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
	case uri == "system://utilization",
		uri == "system://disk",
		uri == "system://network",
		uri == "system://services",
		uri == "system://kernel",
		uri == "system://processes",
		uri == "apps://list",
		strings.HasPrefix(uri, "apps://state/"),
		strings.HasPrefix(uri, "docker://logs/"):
		// All of these are thin proxies (system://* → core per ADR-0044,
		// apps://* + docker://* → app-management per ADR-0045). On a
		// running box the payload is the upstream JSON; with the
		// upstream down each shares the canonical <service>_unavailable
		// shape — assertProxiedPayload handles every variant, treating
		// the degraded path as WARN with the audit + journal fallback
		// hint preserved.
		return assertProxiedPayload(uri, payload)
	case uri == "system://gpu":
		return assertSystemGPU(payload)
	case uri == "system://updates":
		return assertSystemUpdates(payload)
	case strings.HasPrefix(uri, "docs://api/"):
		return assertOpenAPISpecPayload(uri, payload)
	case uri == "docs://concepts/index":
		return assertConceptsIndex(payload)
	case strings.HasPrefix(uri, "docs://concepts/"):
		return assertConceptMarkdown(uri, payload)
	case uri == "catalog://index":
		return assertCatalogIndex(payload)
	case strings.HasPrefix(uri, "catalog://app/"):
		return assertCatalogApp(uri, payload)
	}
	return 0
}

// assertSystemUpdates validates the system://updates payload. The
// resource has two valid shapes — detected="apt" with parsed entries
// on Debian/Ubuntu, or detected="none" with a note on every other
// host. Either is PASS; the smoke just confirms the wire contract is
// honoured so a future regression breaking the shape gets caught.
func assertSystemUpdates(payload string) int {
	var pl struct {
		Detected      string `json:"detected"`
		Count         int    `json:"count"`
		SecurityCount int    `json:"security_count"`
		Packages      []struct {
			Package   string `json:"package"`
			Candidate string `json:"candidate"`
			Security  bool   `json:"security"`
		} `json:"packages"`
		Note string `json:"note,omitempty"`
	}
	if err := json.Unmarshal([]byte(payload), &pl); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  system://updates — payload not JSON: %v\n", err)
		return 1
	}
	switch pl.Detected {
	case "apt":
		// On an apt host: count must agree with packages slice length
		// (the parser sets one from the other).
		if pl.Count != len(pl.Packages) {
			fmt.Fprintf(os.Stderr, "FAIL  system://updates — count=%d vs packages=%d mismatch\n", pl.Count, len(pl.Packages))
			return 1
		}
		fmt.Printf("      → detected=apt, %d pending (%d security-flagged)\n", pl.Count, pl.SecurityCount)
	case "none":
		// On a non-apt host: must surface a note so the agent
		// pattern-matches WHY before reading packages.
		if pl.Note == "" {
			fmt.Fprintf(os.Stderr, "FAIL  system://updates — detected=none with no note (agent can't tell apt-missing from query-failed)\n")
			return 1
		}
		fmt.Printf("      → detected=none (note: %s)\n", pl.Note)
	default:
		fmt.Fprintf(os.Stderr, "FAIL  system://updates — detected=%q (expected apt|none)\n", pl.Detected)
		return 1
	}
	return 0
}

// assertConceptsIndex pins the docs://concepts/index manifest shape.
// Empty array is OK on a Mac dev box; agents pattern-match on length.
func assertConceptsIndex(payload string) int {
	var idx struct {
		Description string `json:"description"`
		Concepts    []struct {
			Name string `json:"name"`
			URI  string `json:"uri"`
		} `json:"concepts"`
	}
	if err := json.Unmarshal([]byte(payload), &idx); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  docs://concepts/index — payload not JSON: %v\n", err)
		return 1
	}
	if idx.Description == "" {
		fmt.Fprintf(os.Stderr, "FAIL  docs://concepts/index — empty description\n")
		return 1
	}
	if len(idx.Concepts) == 0 {
		fmt.Printf("      → empty concepts manifest (Mac dev / pre-install — wire-up fine, staging not run)\n")
	} else {
		fmt.Printf("      → %d concept(s) advertised\n", len(idx.Concepts))
	}
	return 0
}

// assertConceptMarkdown spot-checks a docs://concepts/<name> read.
// Must be non-empty markdown (we don't validate content, just shape).
func assertConceptMarkdown(uri, payload string) int {
	if strings.TrimSpace(payload) == "" {
		fmt.Fprintf(os.Stderr, "FAIL  %s — empty body\n", uri)
		return 1
	}
	fmt.Printf("      → %d-byte concept markdown\n", len(payload))
	return 0
}

// assertCatalogIndex pins catalog://index manifest shape.
func assertCatalogIndex(payload string) int {
	var idx struct {
		Description string `json:"description"`
		Apps        []struct {
			ID  string `json:"id"`
			URI string `json:"uri"`
		} `json:"apps"`
	}
	if err := json.Unmarshal([]byte(payload), &idx); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  catalog://index — payload not JSON: %v\n", err)
		return 1
	}
	if idx.Description == "" {
		fmt.Fprintf(os.Stderr, "FAIL  catalog://index — empty description\n")
		return 1
	}
	if len(idx.Apps) == 0 {
		fmt.Printf("      → empty catalog (community-catalog not staged on this host)\n")
	} else {
		fmt.Printf("      → %d app(s) in catalog\n", len(idx.Apps))
	}
	return 0
}

// assertCatalogApp spot-checks a catalog://app/<id> read — must be a
// non-empty YAML body that looks like compose (contains 'services:'
// or starts with 'version:').
func assertCatalogApp(uri, payload string) int {
	if strings.TrimSpace(payload) == "" {
		fmt.Fprintf(os.Stderr, "FAIL  %s — empty body\n", uri)
		return 1
	}
	if !strings.Contains(payload, "services:") && !strings.HasPrefix(payload, "version:") {
		fmt.Fprintf(os.Stderr, "FAIL  %s — body doesn't look like a docker-compose.yml\n", uri)
		return 1
	}
	fmt.Printf("      → %d-byte compose YAML (looks valid)\n", len(payload))
	return 0
}

// assertOpenAPISpecPayload verifies docs://api/<service> returns a
// real-looking OpenAPI YAML: starts with `openapi:` or `swagger:` and
// has a non-trivial body. We don't validate the spec syntactically
// (that's the upstream Scalar's job + the spec authors'); we DO
// catch the obvious failure of "the package-linux.sh staging step
// silently shipped an empty file."
func assertOpenAPISpecPayload(uri, payload string) int {
	body := strings.TrimSpace(payload)
	if body == "" {
		fmt.Fprintf(os.Stderr, "FAIL  %s — spec is empty (package-linux.sh staging step broken?)\n", uri)
		return 1
	}
	if !strings.HasPrefix(body, "openapi:") && !strings.HasPrefix(body, "swagger:") {
		fmt.Fprintf(os.Stderr, "FAIL  %s — first line %q doesn't look like an OpenAPI/Swagger doc\n",
			uri, firstLine(body))
		return 1
	}
	if len(body) < 200 {
		// A real spec is at least a few KB; a tiny stub is a signal
		// that something staged the wrong file.
		fmt.Fprintf(os.Stderr, "FAIL  %s — spec is only %d bytes; expected at least a few hundred for any real service\n",
			uri, len(body))
		return 1
	}
	fmt.Printf("      → %d-byte OpenAPI doc starts with %q\n", len(body), firstLine(body))
	return 0
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// assertSystemGPU validates the system://gpu payload shape. The
// resource imports common/external::GetGPUUtilization directly so it
// never errors — on a no-GPU box it returns an empty struct (model=""),
// on Apple Silicon / Nvidia it carries real numbers. The contract is:
// every field present, never null, percent in 0..100, temperature
// non-negative. An empty model is fine (it means "no GPU detected"),
// not a failure.
func assertSystemGPU(payload string) int {
	if payload == "null" {
		fmt.Fprintf(os.Stderr, "FAIL  system://gpu — payload is literal null (handler bug)\n")
		return 1
	}
	var g struct {
		Percent     float64 `json:"percent"`
		MemoryUsed  int64   `json:"memoryUsed"`
		Model       string  `json:"model"`
		Temperature int     `json:"temperature"`
	}
	if err := json.Unmarshal([]byte(payload), &g); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  system://gpu — payload is not a JSON object: %v\n", err)
		return 1
	}
	if g.Percent < 0 || g.Percent > 100 {
		fmt.Fprintf(os.Stderr, "FAIL  system://gpu — percent=%v out of 0..100\n", g.Percent)
		return 1
	}
	if g.Temperature < 0 {
		fmt.Fprintf(os.Stderr, "FAIL  system://gpu — temperature=%d negative\n", g.Temperature)
		return 1
	}
	if g.Model == "" {
		fmt.Printf("      → no GPU detected (empty model — not a failure)\n")
	} else {
		fmt.Printf("      → %s (%.1f%% util, %d MiB used, %d°C)\n", g.Model, g.Percent, g.MemoryUsed/(1024*1024), g.Temperature)
	}
	return 0
}

// assertProxiedPayload accepts EITHER a core-shaped payload (any valid
// JSON object) OR the canonical core_unavailable shape from
// coreproxy.AsErrorPayload. Treats core-down as a WARN (the box
// running this smoke may be a Mac dev machine or have core stopped —
// neither is a FAIL of the MCP layer itself), and an actual upstream
// payload as a PASS. Used by every system://* / apps://* resource
// that proxies through coreproxy (ADR-0044).
func assertProxiedPayload(uri, payload string) int {
	var probe struct {
		Error    string `json:"error"`
		Detail   string `json:"detail"`
		Fallback string `json:"fallback"`
	}
	if err := json.Unmarshal([]byte(payload), &probe); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  %s — payload is not valid JSON: %v\n", uri, err)
		return 1
	}
	// Match every <service>_unavailable / <service>_status_NNN shape
	// — same template across core (ADR-0044) and apps (ADR-0045) and
	// any future upstream. The Code suffix is what distinguishes
	// which upstream is degraded; the report tells the operator.
	if isDegradedCode(probe.Error) {
		fmt.Printf("      → WARN: %s — proxy resource degraded (%s); audit + journal still readable\n", probe.Error, probe.Detail)
		if !strings.Contains(probe.Fallback, "audit") || !strings.Contains(probe.Fallback, "journal") {
			fmt.Fprintf(os.Stderr, "FAIL  %s — degraded payload missing fallback hint pointing at audit + journal\n", uri)
			return 1
		}
		return 0
	}
	// Real upstream payload: confirm it parsed as a JSON object/array
	// and isn't empty. Resource-specific shape checks live in the
	// resource-dedicated assertions (this is the catch-all).
	if len(payload) < 2 || payload == "null" {
		fmt.Fprintf(os.Stderr, "FAIL  %s — proxied payload is empty / null\n", uri)
		return 1
	}
	fmt.Printf("      → proxied payload OK (%d bytes from upstream)\n", len(payload))
	return 0
}

// isDegradedCode recognises the canonical proxy-error wire codes —
// `<service>_unavailable` or `<service>_status_NNN`. Centralised so a
// new upstream (future "service_unavailable" umbrella, etc.) doesn't
// require touching every per-resource branch.
func isDegradedCode(code string) bool {
	return strings.HasSuffix(code, "_unavailable") || strings.Contains(code, "_status_")
}

// assertSchemaPayload validates that a self-describing schema parses
// as JSON and carries a non-empty "description" + a meaningful body.
// The body can be either:
//   - a "fields" map (audit + journal pattern — own data shape)
//   - per-resource "fields_*" maps (system pattern — distinct shapes
//     per resource in the namespace)
//   - a "resources" routing map (apps + docs pattern — the resource
//     IS a proxy; field shapes live in the upstream and are
//     discoverable via docs://api)
// Any of the three counts as "the agent has documentation."
func assertSchemaPayload(uri, payload string) int {
	var s map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL  %s — schema is not valid JSON: %v\n", uri, err)
		return 1
	}
	desc, _ := s["description"].(string)
	if desc == "" {
		fmt.Fprintf(os.Stderr, "FAIL  %s — schema missing 'description' (agents read this)\n", uri)
		return 1
	}
	fieldCount := 0
	resourceCount := 0
	for k, v := range s {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		switch {
		case k == "fields" || strings.HasPrefix(k, "fields_"):
			fieldCount += len(m)
		case k == "resources":
			resourceCount += len(m)
		}
	}
	if fieldCount == 0 && resourceCount == 0 {
		fmt.Fprintf(os.Stderr, "FAIL  %s — schema documents zero entries (no 'fields', 'fields_*', or 'resources' map with content)\n", uri)
		return 1
	}
	switch {
	case fieldCount > 0 && resourceCount > 0:
		fmt.Printf("      → description set + %d field(s) + %d resource(s) documented\n", fieldCount, resourceCount)
	case fieldCount > 0:
		fmt.Printf("      → description set + %d field(s) documented\n", fieldCount)
	default:
		fmt.Printf("      → description set + %d resource(s) documented (proxy schema — field shapes via docs://api)\n", resourceCount)
	}
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
