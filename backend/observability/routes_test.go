package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHealthz_OK — liveness contract. Always 200, JSON body, no
// auth required. Operators + systemd + curl all rely on this.
func TestHealthz_OK(t *testing.T) {
	srv := httptest.NewServer(newMux())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
	if body["service"] != "powerlab-observability" {
		t.Errorf("service field = %q, want powerlab-observability", body["service"])
	}
}

// TestPlaceholderRoutes_501 — the audit/journal/system routes
// MUST return 501 with a JSON envelope that names the slice.
// This pins the contract that operators hitting these endpoints
// before the feature lands get an honest error, not a 404.
func TestPlaceholderRoutes_501(t *testing.T) {
	srv := httptest.NewServer(newMux())
	t.Cleanup(srv.Close)

	for _, path := range []string{
		"/v1/audit/recent",
		"/v1/audit/stats",
		"/v1/journal",
		"/v1/system/metrics",
	} {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusNotImplemented {
				t.Errorf("status = %d, want 501", res.StatusCode)
			}
			var body map[string]string
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body["error"] != "not implemented yet" {
				t.Errorf("error field = %q", body["error"])
			}
			if body["slice"] == "" {
				t.Errorf("slice field empty — must name the Sprint 17 slice")
			}
			if !strings.Contains(body["see"], "ADR-0034") {
				t.Errorf("see field should reference ADR-0034, got %q", body["see"])
			}
		})
	}
}

// TestUnknownRoute_404 — anything outside the explicit routes is
// 404, NOT 501. Distinguishes "not implemented yet (planned)"
// from "this route doesn't exist".
func TestUnknownRoute_404(t *testing.T) {
	srv := httptest.NewServer(newMux())
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}
