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

// ADR-0048 — catalog://app/{id} exposes the 137-app community catalog
// (PowerLab-curated trusted compose YAMLs shipped with the install) so
// an agent designing a new compose can read existing patterns by
// example. catalog://index lists app IDs available.
//
// This surface is READ-ONLY. It does NOT install anything — the agent
// reads catalog YAMLs as patterns to learn from. Per memory
// `feedback_catalog_trust_policy`, the catalog is "inspiration only";
// exposing it as an MCP resource family is consistent with that
// policy because we're showing the agent what good looks like.

const (
	catalogIndexURI    = "catalog://index"
	catalogAppPrefix   = "catalog://app/"
	catalogAppTemplate = "catalog://app/{id}"
)

// catalogManifest is the wire shape of catalog://index.
type catalogManifest struct {
	Description string                 `json:"description"`
	Apps        []catalogManifestEntry `json:"apps"`
}

type catalogManifestEntry struct {
	ID  string `json:"id"`  // subdirectory name under <CatalogDir>/Apps/
	URI string `json:"uri"` // catalog://app/<id>
}

func registerCatalog(s *mcp.Server, catalogDir string) {
	s.AddResource(
		&mcp.Resource{
			URI:         catalogIndexURI,
			Name:        "PowerLab community catalog index",
			Description: "List of app IDs available in the PowerLab community catalog. Read catalog://app/<id> to inspect any app's docker-compose.yml as a pattern example for designing your own. 137+ curated apps spanning every category (media, dev, dashboards, finance, networking, AI, …).",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			b, err := buildCatalogManifest(catalogDir)
			if err != nil {
				return nil, fmt.Errorf("build catalog manifest: %w", err)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{textJSON(catalogIndexURI, string(b))}}, nil
		},
	)

	s.AddResourceTemplate(
		&mcp.ResourceTemplate{
			URITemplate: catalogAppTemplate,
			Name:        "PowerLab catalog app compose",
			Description: "Raw docker-compose.yml for one PowerLab catalog app. Read catalog://index first to discover available IDs. The compose_authoring MCP prompt bundles a representative trio automatically; this template is the agent-driven 'show me this specific app' path.",
			MIMEType:    "application/yaml",
		},
		func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			id := catalogIDFromURI(req.Params.URI)
			if id == "" {
				return nil, fmt.Errorf("catalog://app/{id} requires an app id (see catalog://index for the list)")
			}
			body, err := readCatalogApp(catalogDir, id)
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/yaml", Text: body}}}, nil
		},
	)
}

// buildCatalogManifest enumerates <catalogDir>/Apps/<id>/ subdirectories
// that contain a docker-compose.yml. Missing catalog → empty list
// (graceful — fresh box or dev environment).
func buildCatalogManifest(catalogDir string) ([]byte, error) {
	manifest := catalogManifest{
		Description: "Available apps in the PowerLab community catalog. Each entry is a curated docker-compose.yml the operator can install or that an agent can read as a pattern example. Use catalog://app/<id> for the raw YAML.",
		Apps:        []catalogManifestEntry{},
	}

	appsRoot := filepath.Join(catalogDir, "Apps")
	entries, err := os.ReadDir(appsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return json.Marshal(manifest)
		}
		return nil, fmt.Errorf("read %s: %w", appsRoot, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		// Only list apps that actually have a docker-compose.yml —
		// some catalog directories may hold icons or metadata for
		// in-progress apps; skip them so the agent never reads a 404.
		composePath := filepath.Join(appsRoot, id, "docker-compose.yml")
		if _, statErr := os.Stat(composePath); statErr != nil {
			continue
		}
		manifest.Apps = append(manifest.Apps, catalogManifestEntry{
			ID:  id,
			URI: catalogAppPrefix + id,
		})
	}
	sort.Slice(manifest.Apps, func(i, j int) bool { return manifest.Apps[i].ID < manifest.Apps[j].ID })
	return json.Marshal(manifest)
}

// readCatalogApp reads <catalogDir>/Apps/<id>/docker-compose.yml.
// Path-traversal hardened: id cannot contain '/', '\\', or '.'.
func readCatalogApp(catalogDir, id string) (string, error) {
	if strings.ContainsAny(id, "/\\.") {
		return "", fmt.Errorf("invalid app id %q (must match catalog://index entries; no path separators)", id)
	}
	path := filepath.Join(catalogDir, "Apps", id, "docker-compose.yml")
	// #nosec G304 -- catalogDir is operator-configured + id is
	// validated above (no path separators, no dots).
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("no catalog app named %q (looked at %s) — read catalog://index for the available apps", id, path)
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(body), nil
}

// catalogIDFromURI extracts "<id>" from "catalog://app/<id>".
func catalogIDFromURI(raw string) string {
	if !strings.HasPrefix(raw, catalogAppPrefix) {
		return ""
	}
	tail := strings.TrimPrefix(raw, catalogAppPrefix)
	if tail == "" {
		return ""
	}
	return tail
}
