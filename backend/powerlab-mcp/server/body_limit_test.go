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
// limitBody must make an over-cap body fail the downstream read rather
// than buffer it all.
func TestLimitBody_CapsOversizedBody(t *testing.T) {
	var readErr error
	var readN int
	h := limitBody(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		readN, readErr = len(b), err
	}), 16) // tiny cap for the test

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, strings.NewReader(strings.Repeat("x", 1000)))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if readErr == nil {
		t.Fatalf("reading a 1000-byte body under a 16-byte cap returned nil error (read %d bytes) — MaxBytesReader not applied", readN)
	}
	if int64(readN) > 16 {
		t.Fatalf("read %d bytes past the 16-byte cap — the body was not bounded", readN)
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
