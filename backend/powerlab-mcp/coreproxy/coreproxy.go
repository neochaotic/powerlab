// Package coreproxy is powerlab-mcp's thin HTTP client over the rest
// of the PowerLab stack. It discovers the core service URL the same
// way every other PowerLab service does (reading the .url file core
// publishes at startup), forwards the calling agent's Bearer token,
// and surfaces failures as structured errors the agent can recognise
// — never a fake snapshot.
//
// Why this package exists: ADR-0044 amends ADR-0034's "isolated MCP"
// stance — sysadmin telemetry (CPU, RAM, disk, network, GPU, SMART)
// is already implemented in core and accessed by the panel, so MCP
// thin-proxies those endpoints instead of re-reading /proc. Audit
// and journal stay independent (they have no upstream).
package coreproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// urlCacheTTL is how long a resolved service URL is cached before we
// re-read the .url file. 10 seconds matches the JWKS cache lifetime
// already used elsewhere in MCP — a brief core restart heals within
// one TTL window without locking MCP into a stale URL.
const urlCacheTTL = 10 * time.Second

// defaultHTTPTimeout caps every proxied call. core's panel endpoints
// return in milliseconds in practice; 8 s leaves headroom for a busy
// box without letting a stuck handler tie up the MCP request.
const defaultHTTPTimeout = 8 * time.Second

// CoreURLFile is the filename core writes its address to under
// RuntimePath. Kept as "casaos.url" because that's what core actually
// writes today (legacy from the fork — see backend/core/main.go:237)
// and renaming the file is its own ADR.
const CoreURLFile = "casaos.url"

// Client is the typed proxy: hand it a RuntimePath at construction,
// then call Get*(ctx, path, token) to round-trip a request to core.
// Concurrent-safe (the URL cache is mutex-guarded).
type Client struct {
	runtimePath string

	mu        sync.Mutex
	cachedURL string
	cachedAt  time.Time

	httpClient *http.Client
}

// NewClient returns a Client that resolves core via runtimePath and
// makes calls with the given HTTP client; nil falls back to a client
// with a sensible total timeout. Production wiring passes nil; tests
// pass an httptest server's client so the cache lookups go nowhere
// over the network.
func NewClient(runtimePath string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{runtimePath: runtimePath, httpClient: httpClient}
}

// ResolveCore returns core's base URL (e.g. "http://127.0.0.1:8810")
// reading $RuntimePath/casaos.url. Cached for urlCacheTTL so a hot
// resource isn't re-reading the file on every read. A missing file
// returns ErrCoreUnreachable so the caller can surface the canonical
// shape to the agent.
func (c *Client) ResolveCore() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedURL != "" && time.Since(c.cachedAt) < urlCacheTTL {
		return c.cachedURL, nil
	}
	path := filepath.Join(c.runtimePath, CoreURLFile)
	// #nosec G304 -- runtimePath is operator-configured (mcp.conf),
	// joined with a fixed filename.
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &Error{Code: "core_unavailable", Detail: fmt.Sprintf("%s missing (core hasn't published its URL — service down or RuntimePath misconfigured)", path)}
		}
		return "", &Error{Code: "core_unavailable", Detail: fmt.Sprintf("read %s: %v", path, err)}
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return "", &Error{Code: "core_unavailable", Detail: fmt.Sprintf("%s is empty", path)}
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	c.cachedURL = raw
	c.cachedAt = time.Now()
	return raw, nil
}

// Get proxies an HTTP GET to core at the given path (e.g.
// "/v1/sys/utilization") and returns the response body verbatim. If
// token is non-empty it's forwarded via Authorization: Bearer; for
// loopback callers (no JWT) MCP makes the call from 127.0.0.1, which
// core's own loopback skip honours.
//
// Failures (core down, non-2xx, transport error, bad body) all
// surface as *Error so the agent gets a predictable shape — never a
// fake snapshot, never a leaked Go error string.
func (c *Client) Get(ctx context.Context, path, token string) ([]byte, error) {
	base, err := c.ResolveCore()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := base + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &Error{Code: "core_unavailable", Detail: fmt.Sprintf("build request: %v", err)}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// Politeness header so a core log line says who called.
	req.Header.Set("User-Agent", "powerlab-mcp/proxy")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Invalidate the cached URL so the next retry re-reads the
		// file — handles the case where core restarted on a new port.
		c.invalidate()
		return nil, &Error{Code: "core_unavailable", Detail: fmt.Sprintf("GET %s: %v", url, err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Surface the status so the agent can distinguish "core said
		// no" from "core is down" — both are operator-meaningful but
		// the fix is different.
		return nil, &Error{
			Code:   fmt.Sprintf("core_status_%d", resp.StatusCode),
			Detail: fmt.Sprintf("GET %s returned HTTP %d", url, resp.StatusCode),
			Body:   string(body),
		}
	}
	if readErr != nil {
		return nil, &Error{Code: "core_unavailable", Detail: fmt.Sprintf("read response: %v", readErr)}
	}
	return body, nil
}

// invalidate drops the cached URL so the next ResolveCore re-reads
// the file from disk. Called when a request fails — the cause might
// be a stale URL after a core restart on a different port.
func (c *Client) invalidate() {
	c.mu.Lock()
	c.cachedURL = ""
	c.cachedAt = time.Time{}
	c.mu.Unlock()
}

// Error is the structured failure the agent receives when a proxied
// call fails. The Code field is short + machine-readable; Detail is
// human-readable; Body (when set) is the upstream response body so
// the operator can see what core actually said.
//
// As an MCP resource payload this serialises to a JSON object with
// "error", "detail", and optionally "body" — the agent can pattern-
// match on `error == "core_unavailable"` to fall back to journal +
// audit reads (which never need core to be up).
type Error struct {
	Code   string `json:"error"`
	Detail string `json:"detail"`
	Body   string `json:"body,omitempty"`
}

// Error implements the error interface so the proxy errors compose
// with the standard library's error machinery.
func (e *Error) Error() string {
	if e == nil {
		return "<nil coreproxy error>"
	}
	return e.Code + ": " + e.Detail
}

// AsErrorPayload returns the canonical JSON payload an MCP resource
// should serve when a proxied call fails — including the agent-
// friendly "fallback" hint pointing at the resources that survive
// core being down. Used by every system://, apps:// resource so the
// shape is consistent across the proxy surface.
func AsErrorPayload(err error) []byte {
	e := &Error{Code: "core_unavailable", Detail: err.Error()}
	var pe *Error
	if errors.As(err, &pe) {
		e = pe
	}
	out := struct {
		Error    string `json:"error"`
		Detail   string `json:"detail"`
		Body     string `json:"body,omitempty"`
		Fallback string `json:"fallback"`
	}{
		Error:    e.Code,
		Detail:   e.Detail,
		Body:     e.Body,
		Fallback: "audit:// and journal:// (for powerlab- units) remain readable — they don't need core to be up",
	}
	b, _ := json.Marshal(out)
	return b
}
