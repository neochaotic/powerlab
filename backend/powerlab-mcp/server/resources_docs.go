package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// docsAPIURI is the manifest resource — agents start here to know
	// which PowerLab services have OpenAPI specs they can read.
	docsAPIURI = "docs://api"

	// docsAPITemplate is the per-service template. {service} is the
	// short name (core, gateway, app-management, …) matching the
	// .yaml filename in OpenAPIDir.
	docsAPITemplate = "docs://api/{service}"

	// docsAPIPrefix is the literal URI prefix used to extract the
	// service name from a concrete docs://api/<service> read.
	docsAPIPrefix = "docs://api/"
)

// docsAPIDescription explains the purpose to operators reading
// resources/list. Kept aligned with ADR-0008 (the Scalar API docs
// portal) and ADR-0044 (the hybrid proxy architecture) — same
// underlying OpenAPI specs, MCP-shaped.
const docsAPIDescription = "OpenAPI specs for every PowerLab service. The agent reads docs://api to discover what's available, then docs://api/{service} for the raw YAML — same source the Scalar docs portal serves."

// docsManifest is the JSON the docs://api resource returns. The agent
// reads this once at the start of a session, then drives per-service
// reads from it. Each entry includes a small description so the agent
// doesn't have to download a multi-KB YAML just to learn what's there.
type docsManifest struct {
	Description string             `json:"description"`
	Specs       []docsManifestSpec `json:"specs"`
}

type docsManifestSpec struct {
	Service     string `json:"service"`              // short name, matches the URI template
	URI         string `json:"uri"`                  // concrete docs://api/<service> the agent reads
	Description string `json:"description,omitempty"`
	SizeBytes   int64  `json:"size_bytes"`
}

// serviceDescriptions are short one-liners surfaced in the manifest so
// the agent doesn't need to parse YAML to know what each service does.
// Kept in lockstep with the curated service set — adding a new service
// to PowerLab? Add a line here.
var serviceDescriptions = map[string]string{
	"core":           "system observability + power actions + updater (CPU, mem, disk, network, hardware, users, version, reboot)",
	"app-management": "install / update / remove Docker Compose apps; container lifecycle + logs",
	"gateway":        "auth, request routing, audit middleware; mounts the embedded UI bundle",
	"user-service":   "user accounts + JWT signing; PAM-backed OS auth; account recovery",
	"message-bus":    "internal pub/sub between services + the SSE bridge to the panel",
	"local-storage":  "physical disk listing (lsblk + SMART), USB / SD auto-mount lifecycle",
}

// registerDocs exposes the OpenAPI surface as MCP resources rooted at
// docs://. openAPIDir is the directory where the operator-facing specs
// land (production: /usr/share/powerlab/openapi/, populated by
// package-linux.sh's stage step; tests point it at a fixture).
//
// A missing or empty directory is NOT an error — the resource degrades
// gracefully to an empty manifest. Same pattern as audittail.Recent
// for a fresh box that hasn't audited anything yet.
func registerDocs(s *mcp.Server, openAPIDir string) {
	s.AddResource(
		&mcp.Resource{
			URI:         docsAPIURI,
			Name:        "PowerLab API manifest",
			Description: docsAPIDescription,
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			b, err := buildDocsManifest(openAPIDir)
			if err != nil {
				return nil, fmt.Errorf("build docs manifest: %w", err)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(docsAPIURI, string(b))}}, nil
		},
	)

	s.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: docsAPITemplate,
			Name:        "PowerLab service OpenAPI spec",
			Description: "Raw OpenAPI YAML for one PowerLab service. Use docs://api to discover the available services first.",
			MIMEType:    "application/yaml",
		},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			svc := serviceFromDocsURI(req.Params.URI)
			if svc == "" {
				return nil, fmt.Errorf("docs://api/{service} requires a service name (see docs://api for the list)")
			}
			body, err := readSpec(openAPIDir, svc)
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/yaml", Text: body}}}, nil
		},
	)
}

// buildDocsManifest walks openAPIDir and returns the manifest payload.
// Missing dir → empty manifest (not an error — Mac dev box doesn't
// have /usr/share/powerlab/openapi/ unless we wire a fixture).
func buildDocsManifest(openAPIDir string) ([]byte, error) {
	manifest := docsManifest{
		Description: "Available PowerLab OpenAPI specs. Read docs://api/{service} for the raw YAML of any entry below.",
		Specs:       []docsManifestSpec{},
	}

	entries, err := os.ReadDir(openAPIDir)
	if err != nil {
		if os.IsNotExist(err) {
			return json.Marshal(manifest)
		}
		return nil, fmt.Errorf("read %s: %w", openAPIDir, err)
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		svc := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
		// Stat without resolving so we don't follow a symlink loop.
		info, statErr := e.Info()
		var size int64
		if statErr == nil {
			size = info.Size()
		}
		manifest.Specs = append(manifest.Specs, docsManifestSpec{
			Service:     svc,
			URI:         docsAPIPrefix + svc,
			Description: serviceDescriptions[svc], // empty if unknown — harmless, agent still sees URI
			SizeBytes:   size,
		})
	}
	sort.Slice(manifest.Specs, func(i, j int) bool { return manifest.Specs[i].Service < manifest.Specs[j].Service })
	return json.Marshal(manifest)
}

// readSpec returns the raw YAML for a service, with a friendly
// "spec not found" error pointing at docs://api for discovery.
func readSpec(openAPIDir, svc string) (string, error) {
	// Defensive: reject anything that could escape the dir. The MCP
	// SDK routes by URI but a malicious template substitution
	// (`../../etc/passwd`) is the kind of thing an adversarial review
	// expects us to handle — match the literal filename pattern.
	if strings.ContainsAny(svc, "/\\.") {
		return "", fmt.Errorf("invalid service name %q (must match the docs://api manifest)", svc)
	}
	path := filepath.Join(openAPIDir, svc+".yaml")
	// #nosec G304 -- openAPIDir is operator-configured + svc is
	// validated above (no path separators, no dots).
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("no spec for service %q at %s — read docs://api for the available services", svc, path)
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(body), nil
}

// serviceFromDocsURI extracts "<svc>" from "docs://api/<svc>". The MCP
// SDK doesn't expand the template for us when the agent reads a
// concrete URI; we parse it ourselves the same way the audit/journal
// templated resources do.
func serviceFromDocsURI(raw string) string {
	if !strings.HasPrefix(raw, docsAPIPrefix) {
		return ""
	}
	return strings.TrimPrefix(raw, docsAPIPrefix)
}
