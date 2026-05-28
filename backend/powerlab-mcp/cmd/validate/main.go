// Command validate is the operator-facing CLI wrapper around the
// composevalidator package. It reads a Docker Compose YAML from a
// file path (or stdin with `-`) and reports any ADR-0046 deny-list
// hits as either pretty text or JSON.
//
// Usage:
//
//	powerlab-mcp-validate ./my-app.yml
//	cat my-app.yml | powerlab-mcp-validate -
//	powerlab-mcp-validate -json ./my-app.yml > result.json
//
// Exit code: 0 if the YAML passes; 1 if any violation; 2 on a
// missing file or other operator error. The JSON form is the
// machine-readable output (composevalidator.Result) MCP's
// install_app tool will use; the text form is for humans.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/composevalidator"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit JSON (composevalidator.Result) instead of a human-readable report")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: %s [-json] <path | ->\n\n", os.Args[0])
		_, _ = fmt.Fprintln(os.Stderr, "Validates a Docker Compose YAML against the ADR-0046 deny-list (no privileged: true,")
		_, _ = fmt.Fprintln(os.Stderr, "no Docker socket bind, no host namespace sharing, no dangerous cap_add, no raw")
		_, _ = fmt.Fprintln(os.Stderr, "device passthrough, no sensitive host path binds). Use '-' to read stdin.")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Exit code: 0 = clean, 1 = violations, 2 = I/O / argument error.")
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	body, err := readSource(flag.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "powerlab-mcp-validate: %v\n", err)
		os.Exit(2)
	}

	result := composevalidator.Validate(body)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "powerlab-mcp-validate: marshal: %v\n", err)
			os.Exit(2)
		}
	} else {
		if result.OK {
			_, _ = fmt.Fprintln(os.Stdout, "OK — no ADR-0046 deny-list violations.")
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "REJECTED — %d violation(s):\n", len(result.Violations))
			for _, v := range result.Violations {
				svc := v.Service
				if svc == "" {
					svc = "<doc>"
				}
				_, _ = fmt.Fprintf(os.Stderr, "  [%s] %s — %s\n", v.Code, svc, v.Detail)
			}
		}
	}

	if !result.OK {
		os.Exit(1)
	}
}

// readSource reads from disk or stdin depending on path. "-" means
// stdin; any other value is treated as a path.
func readSource(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	// #nosec G304 -- operator-supplied path on a local CLI.
	return os.ReadFile(path)
}
