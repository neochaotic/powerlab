package audit

import (
	"encoding/json"
	"net/http"
)

// RecentHTTPHandler serves GET /v1/audit/recent on a stdlib mux.
// Used by the gateway's public route bundle (ADR-0035) — the Echo
// variant in RecentHandler is kept for management-port consumers.
//
// JWT validation happens BEFORE this handler — the gateway wraps it
// with the same JWT middleware that protects every other public API.
// This handler trusts that the caller is authenticated.
func RecentHTTPHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		opts := parseRecentOptions(r.URL.Query().Get)
		rows := store.Recent(opts)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(envelope[[]Record]{Data: rows})
	}
}

// StatsHTTPHandler serves GET /v1/audit/stats on a stdlib mux.
func StatsHTTPHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s, err := store.Stats(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(envelope[Stats]{Data: Stats{}, Message: err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(envelope[Stats]{Data: s})
	}
}
