// Package observability is PowerLab's standalone observability +
// MCP service skeleton. It runs INDEPENDENTLY of every other
// PowerLab service so operators can debug the system even when
// the gateway / app-management / etc. are down.
//
// This file is the HTTP route surface — minimal in this skeleton,
// expanded in subsequent Sprint 17 slices to match ADR-0034:
//
//   - /healthz                 — liveness probe (this file)
//   - /v1/audit/recent         — read across per-service audit DBs
//   - /v1/audit/stats          — aggregated stats
//   - /v1/journal              — journalctl proxy
//   - /v1/system/metrics       — direct /proc reads
//
// The MCP transports (stdio + HTTP) live in separate files and
// share the same resource handlers — see ADR-0034 §3.
//
// All handlers in THIS skeleton return 501 NotImplemented with a
// JSON envelope describing the slice that will fill them in.
// Operators hitting the endpoint before the feature lands get an
// honest "not yet, see issue #N" rather than a 404.
package main

import (
	"encoding/json"
	"net/http"
)

// notImplemented writes a 501 with the slice tracker so callers
// know which Sprint 17 issue closes the placeholder.
func notImplemented(w http.ResponseWriter, slice string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
		"slice": slice,
		"see":   "ADR-0034 — standalone observability + MCP service",
	})
}

// healthz is the liveness probe. Returns 200 with a tiny JSON
// blob. systemd / kube / curl can poll this without auth — it
// reveals nothing sensitive.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service": "powerlab-observability",
		"status":  "ok",
	})
}

// newMux returns the HTTP handler chain. Kept separate from main
// so the test suite can mount it against httptest.NewServer
// without spinning up a real listener.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz)

	// Placeholder routes — each subsequent Sprint 17 slice replaces
	// one of these with a real handler that hits the audit DBs /
	// journalctl / /proc.
	mux.HandleFunc("GET /v1/audit/recent", func(w http.ResponseWriter, _ *http.Request) {
		notImplemented(w, "Sprint 17: audit aggregator across per-service DBs")
	})
	mux.HandleFunc("GET /v1/audit/stats", func(w http.ResponseWriter, _ *http.Request) {
		notImplemented(w, "Sprint 17: audit aggregator across per-service DBs")
	})
	mux.HandleFunc("GET /v1/journal", func(w http.ResponseWriter, _ *http.Request) {
		notImplemented(w, "Sprint 17: journalctl proxy with filter + tail")
	})
	mux.HandleFunc("GET /v1/system/metrics", func(w http.ResponseWriter, _ *http.Request) {
		notImplemented(w, "Sprint 17: direct /proc reads (CPU, mem, disk, net)")
	})

	return mux
}
