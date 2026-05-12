package main

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseUmbrelManifest decodes `umbrel-app.yml` into UmbrelManifest.
// The struct's yaml tags are the field allowlist: anything not on
// the struct is silently dropped by gopkg.in/yaml.v3. To add a new
// upstream field you have to edit the struct — a deliberate review
// point. ADR-0024 captures the legal rationale.
func ParseUmbrelManifest(data []byte) (UmbrelManifest, error) {
	var m UmbrelManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return UmbrelManifest{}, fmt.Errorf("parse umbrel-app.yml: %w", err)
	}
	if m.ID == "" {
		return UmbrelManifest{}, fmt.Errorf("umbrel-app.yml missing required `id` field")
	}
	return m, nil
}

// ParseComposeFile decodes `docker-compose.yml` into the minimal
// shape the filter inspects. Strict yaml.Unmarshal would reject
// unknown fields; we want lenient + allowlist-on-struct semantics
// for the same legal-posture reason as the manifest.
func ParseComposeFile(data []byte) (ComposeFile, error) {
	var c ComposeFile
	if err := yaml.Unmarshal(data, &c); err != nil {
		return ComposeFile{}, fmt.Errorf("parse docker-compose.yml: %w", err)
	}
	return c, nil
}
