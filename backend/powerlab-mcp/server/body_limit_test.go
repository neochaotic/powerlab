package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The MCP transport reads the whole request body into memory, so an
// unbounded POST is an OOM vector (unauthenticated from loopback).
// limitBody must reject an over-cap body with 413 Payload Too Large
// (#606) — not a generic 400 — and never hand it to the downstream
// handler. Covers both a declared Content-Length and a chunked body of
// unknown length.
func TestLimitBody_OversizedBodyIs413(t *testing.T) {
	for _, tc := range []struct {
		name          string
		contentLength int64
	}{
		{"declared length", 1000},
		{"chunked (unknown length)", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := limitBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = io.ReadAll(r.Body)
			}), 16) // tiny cap for the test

			req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, strings.NewReader(strings.Repeat("x", 1000)))
			req.ContentLength = tc.contentLength
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d; want 413 for a 1000-byte body under a 16-byte cap", rec.Code)
			}
			if called {
				t.Fatalf("oversized body reached the downstream handler — it must be rejected first")
			}
		})
	}
}

// End-to-end through the real handler chain: a >1 MiB POST to /mcp
// must come back 413, which is what the issue reproduced with curl.
func TestMCPEndpoint_OversizedPostIs413(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, MCPEndpointPath,
			strings.NewReader(strings.Repeat("x", maxMCPRequestBytes+300_000)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.RemoteAddr = "127.0.0.1:5000"
		if chunked {
			req.ContentLength = -1
		}
		rec := httptest.NewRecorder()
		newTestHandler(t).ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("chunked=%v: POST >1 MiB to /mcp = %d; want 413", chunked, rec.Code)
		}
	}
}

// A normal small body must pass through untouched — the cap protects
// against abuse without breaking legitimate MCP requests.
func TestLimitBody_AllowsSmallBody(t *testing.T) {
	const payload = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	var got string
	h := limitBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("small body under the cap errored: %v", err)
		}
		got = string(b)
	}), maxMCPRequestBytes)

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, strings.NewReader(payload))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != payload {
		t.Fatalf("small body = %q; want it delivered intact %q", got, payload)
	}
}

// A small chunked body (unknown length) is buffered by limitBody and
// must still reach the handler intact.
func TestLimitBody_AllowsSmallChunkedBody(t *testing.T) {
	const payload = `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	var got string
	h := limitBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("small chunked body errored: %v", err)
		}
		got = string(b)
	}), maxMCPRequestBytes)

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, strings.NewReader(payload))
	req.ContentLength = -1
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != payload {
		t.Fatalf("small chunked body = %q; want it delivered intact %q", got, payload)
	}
}
