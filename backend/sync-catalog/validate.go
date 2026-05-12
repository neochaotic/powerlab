package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationError is one shape-violation found in a single
// community-catalog entry. Path is relative to the catalog root
// for readability in the CI log.
type ValidationError struct {
	Path    string // <root>/Apps/<id>/docker-compose.yml
	AppID   string // "<id>"
	Rule    string // short rule name for grep-ability
	Message string
}

// String formats one error as "<path>: <rule>: <message>".
func (e ValidationError) String() string {
	return fmt.Sprintf("%s: %s: %s", e.Path, e.Rule, e.Message)
}

// ValidateCatalogTree walks <root>/Apps/* and returns the list
// of shape violations found. Empty result = catalog is well-
// formed and safe to merge.
//
// Walked invariants (each is a separate Rule for grep-ability):
//
//	yaml.parse      — file decodes as YAML
//	yaml.services   — top-level `services:` block present
//	yaml.x-powerlab — top-level `x-powerlab:` extension present
//	field.store_app_id — `x-powerlab.store_app_id` non-empty string
//	field.title     — `x-powerlab.title.en_us` non-empty string
//	field.source    — `x-powerlab.source.catalog` non-empty string
//	field.icon      — `x-powerlab.icon`, if present, parses as URL
//
// The catalog root must contain an `Apps/` directory; an
// empty `Apps/` is valid (means "no apps yet" — the weekly
// sync hasn't produced anything for this source).
func ValidateCatalogTree(root string) ([]ValidationError, error) {
	appsDir := filepath.Join(root, appsDirectoryName)
	stat, err := os.Stat(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Empty catalog — not an error; just nothing to validate.
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", appsDir, err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", appsDir)
	}

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", appsDir, err)
	}

	var errs []ValidationError
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		appErrs := validateAppDir(root, e.Name())
		errs = append(errs, appErrs...)
	}

	// Stable ordering — deterministic CI logs make per-file diffs
	// readable across runs.
	sort.Slice(errs, func(i, j int) bool {
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		return errs[i].Rule < errs[j].Rule
	})

	return errs, nil
}

func validateAppDir(root, appID string) []ValidationError {
	composePath := filepath.Join(root, appsDirectoryName, appID, composeYAMLFileName)
	relPath, _ := filepath.Rel(root, composePath)

	mk := func(rule, msg string) ValidationError {
		return ValidationError{Path: relPath, AppID: appID, Rule: rule, Message: msg}
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		return []ValidationError{mk("file.missing", err.Error())}
	}

	// Decode as a generic map so we can walk arbitrary keys.
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return []ValidationError{mk("yaml.parse", err.Error())}
	}

	var out []ValidationError

	// Rule: yaml.services
	if _, ok := doc["services"]; !ok {
		out = append(out, mk("yaml.services", "missing top-level `services:` block"))
	}

	// Rule: yaml.x-powerlab
	xRaw, ok := doc["x-powerlab"]
	if !ok {
		out = append(out, mk("yaml.x-powerlab", "missing top-level `x-powerlab:` extension"))
		return out
	}
	x, ok := xRaw.(map[string]any)
	if !ok {
		out = append(out, mk("yaml.x-powerlab", fmt.Sprintf("expected map, got %T", xRaw)))
		return out
	}

	// Rule: field.store_app_id
	if s, ok := x["store_app_id"].(string); !ok || strings.TrimSpace(s) == "" {
		out = append(out, mk("field.store_app_id", "missing or empty"))
	}

	// Rule: field.title
	titleRaw := x["title"]
	titleMap, ok := titleRaw.(map[string]any)
	if !ok {
		out = append(out, mk("field.title", "missing or not a map"))
	} else {
		en, _ := titleMap["en_us"].(string)
		if strings.TrimSpace(en) == "" {
			out = append(out, mk("field.title", "title.en_us is missing or empty"))
		}
	}

	// Rule: field.source
	sourceRaw := x["source"]
	sourceMap, ok := sourceRaw.(map[string]any)
	if !ok {
		out = append(out, mk("field.source", "missing — provenance required for community-catalog entries"))
	} else {
		cat, _ := sourceMap["catalog"].(string)
		if strings.TrimSpace(cat) == "" {
			out = append(out, mk("field.source", "source.catalog is missing or empty"))
		}
	}

	// Rule: field.icon — only validated if present; an absent icon
	// is fine (the UI falls through to a Lucide Package fallback).
	if iconRaw, present := x["icon"]; present {
		iconStr, _ := iconRaw.(string)
		if strings.TrimSpace(iconStr) == "" {
			out = append(out, mk("field.icon", "icon is present but empty"))
		} else if _, err := url.Parse(iconStr); err != nil {
			out = append(out, mk("field.icon", fmt.Sprintf("icon URL malformed: %v", err)))
		}
	}

	return out
}
