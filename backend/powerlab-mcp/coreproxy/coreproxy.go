// Package coreproxy is powerlab-mcp's thin HTTP client over the rest
// of the PowerLab stack. It discovers each upstream's URL the same
// way every other PowerLab service does (reading the .url file the
// service publishes at startup), forwards the calling agent's Bearer
// token, and surfaces failures as structured errors the agent can
// recognise — never a fake snapshot.
//
// Why this package exists: ADR-0044 amended ADR-0034's "isolated MCP"
// stance — sysadmin telemetry is already implemented in core and
// accessed by the panel, so MCP thin-proxies those endpoints instead
// of re-reading /proc. ADR-0045 extends the same pattern to a second
// upstream (app-management) for apps:// and docker://* resources,
// keeping MCP storage-agnostic so a future SQLite→PostgreSQL
// migration on app-management is a no-op for MCP. Audit and journal
// stay independent (they have no upstream).
package coreproxy

import (
	"bytes"
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
// already used elsewhere in MCP — a brief upstream restart heals
// within one TTL window without locking MCP into a stale URL.
const urlCacheTTL = 10 * time.Second

// defaultHTTPTimeout caps every proxied call. PowerLab's panel
// endpoints return in milliseconds in practice; 8 s leaves headroom
// for a busy box without letting a stuck handler tie up the MCP
// request.
const defaultHTTPTimeout = 8 * time.Second

// Service IDs for the upstreams this client knows how to resolve. The
// strings are stable wire constants used in error Codes
// (`<service>_unavailable`); the agent pattern-matches on them.
const (
	ServiceCore = "core"
	ServiceApps = "apps"
)

// CoreURLFile is the filename core writes its address to under
// RuntimePath. Kept as "casaos.url" because that's what core actually
// writes today (legacy from the fork — see backend/core/main.go:237);
// renaming the file is its own ADR.
const CoreURLFile = "casaos.url"

// AppsURLFile is the filename app-management writes its address to.
// Per ADR-0045 — same RuntimePath, distinct file.
const AppsURLFile = "app-management.url"

// defaultURLFiles maps each known service ID to its .url filename.
// The constructor copies this so per-Client overrides don't leak
// into the package-level default.
var defaultURLFiles = map[string]string{
	ServiceCore: CoreURLFile,
	ServiceApps: AppsURLFile,
}

// Client is the multi-upstream proxy: hand it a RuntimePath at
// construction, then call Get/GetFrom to round-trip a request to the
// upstream identified by service ID. Concurrent-safe (per-service URL
// caches are mutex-guarded).
type Client struct {
	runtimePath string

	// urlFiles maps the service IDs this Client knows about to their
	// .url filenames. Constructed from defaultURLFiles; future
	// extensions can add entries via a setter (not yet needed).
	urlFiles map[string]string

	mu    sync.Mutex
	cache map[string]urlEntry

	httpClient *http.Client
}

type urlEntry struct {
	url string
	at  time.Time
}

// NewClient returns a Client that resolves PowerLab service URLs via
// runtimePath and makes HTTP calls with the given httpClient; nil
// falls back to a client with a sensible total timeout. Production
// wiring passes nil; tests pass an httptest server's client so the
// resolved URLs go nowhere over the network.
func NewClient(runtimePath string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	// Copy the package default so per-Client tweaks don't mutate the
	// shared map.
	files := make(map[string]string, len(defaultURLFiles))
	for k, v := range defaultURLFiles {
		files[k] = v
	}
	return &Client{
		runtimePath: runtimePath,
		urlFiles:    files,
		cache:       map[string]urlEntry{},
		httpClient:  httpClient,
	}
}

// Resolve returns the upstream base URL for the named service (e.g.
// "http://127.0.0.1:8810" for service="core"), reading
// $RuntimePath/<.url-filename>. Cached for urlCacheTTL so a hot
// resource isn't re-reading the file on every read. A missing or
// empty file returns an *Error with Code "<service>_unavailable" so
// the caller can surface the canonical shape to the agent.
//
// An unknown service ID is treated as service_unavailable rather than
// a panic — callers passing a typo see the same structured error
// shape an operator sees for a real outage, which keeps the failure
// path predictable.
func (c *Client) Resolve(service string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.cache[service]; ok && entry.url != "" && time.Since(entry.at) < urlCacheTTL {
		return entry.url, nil
	}
	file, ok := c.urlFiles[service]
	if !ok {
		return "", &Error{
			Code:   unavailableCode(service),
			Detail: fmt.Sprintf("unknown service %q — known services: %s", service, knownServicesList(c.urlFiles)),
		}
	}
	path := filepath.Join(c.runtimePath, file)
	// #nosec G304 -- runtimePath is operator-configured (mcp.conf),
	// joined with a per-service fixed filename from urlFiles.
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", &Error{
				Code:   unavailableCode(service),
				Detail: fmt.Sprintf("%s missing (%s hasn't published its URL — service down or RuntimePath misconfigured)", path, service),
			}
		}
		return "", &Error{Code: unavailableCode(service), Detail: fmt.Sprintf("read %s: %v", path, err)}
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return "", &Error{Code: unavailableCode(service), Detail: fmt.Sprintf("%s is empty", path)}
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	c.cache[service] = urlEntry{url: raw, at: time.Now()}
	return raw, nil
}

// GetFrom proxies an HTTP GET to the named upstream service at the
// given path (e.g. service="apps", path="/v2/app_management/compose")
// and returns the response body verbatim. If token is non-empty it's
// forwarded via Authorization: Bearer; for loopback callers (no JWT)
// MCP makes the call without an Authorization header at all, which
// the upstream's own loopback skip honours.
//
// Failures (upstream down, non-2xx, transport error, bad body) all
// surface as *Error with Code "<service>_unavailable" or
// "<service>_status_NNN" so the agent gets a predictable shape —
// never a fake snapshot, never a leaked Go error string.
func (c *Client) GetFrom(ctx context.Context, service, path, token string) ([]byte, error) {
	return c.RequestFrom(ctx, http.MethodGet, service, path, token, nil, "")
}

// RequestFrom is the verb-and-body capable form used by write-class
// MCP tools (ADR-0046 — restart_app, prune_orphans, install_app).
// body is an optional payload sent as request body; contentType is
// the Content-Type header (e.g. "application/json"). For GETs / DELETEs
// without a body, pass body=nil and contentType="".
//
// The error contract is identical to GetFrom — failures surface as
// *Error with Code "<service>_unavailable" or "<service>_status_NNN".
// Writes that succeed with a 2xx return the upstream's response body
// verbatim; the caller (a tool handler) is responsible for marshalling
// it back to the agent.
func (c *Client) RequestFrom(ctx context.Context, method, service, path, token string, body []byte, contentType string) ([]byte, error) {
	base, err := c.Resolve(service)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := base + path

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, &Error{Code: unavailableCode(service), Detail: fmt.Sprintf("build request: %v", err)}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	// Politeness header so an upstream log line says who called.
	req.Header.Set("User-Agent", "powerlab-mcp/proxy")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Invalidate the cached URL so the next retry re-reads the
		// file — handles the case where the upstream restarted on a
		// new port.
		c.invalidate(service)
		return nil, &Error{Code: unavailableCode(service), Detail: fmt.Sprintf("%s %s: %v", method, url, err)}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{
			Code:   fmt.Sprintf("%s_status_%d", service, resp.StatusCode),
			Detail: fmt.Sprintf("%s %s returned HTTP %d", method, url, resp.StatusCode),
			Body:   string(respBody),
		}
	}
	if readErr != nil {
		return nil, &Error{Code: unavailableCode(service), Detail: fmt.Sprintf("read response: %v", readErr)}
	}
	return respBody, nil
}

// invalidate drops the cached URL for one service so the next
// Resolve(service) re-reads the file from disk. Called when a
// request fails — the cause might be a stale URL after the upstream
// restarted on a different port.
func (c *Client) invalidate(service string) {
	c.mu.Lock()
	delete(c.cache, service)
	c.mu.Unlock()
}

// unavailableCode returns the canonical "<service>_unavailable" wire
// constant. Kept in one place so the agent's pattern matchers and
// MCP-side assertions agree on the spelling.
func unavailableCode(service string) string {
	return service + "_unavailable"
}

// knownServicesList renders the registered service IDs for an
// operator-facing error detail, sorted for stable output.
func knownServicesList(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Sort manually so we don't pull in sort for one tiny helper.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return strings.Join(keys, ", ")
}

// Error is the structured failure the agent receives when a proxied
// call fails. The Code field is short + machine-readable
// (`<service>_unavailable` or `<service>_status_NNN`); Detail is
// human-readable; Body (when set) is the upstream response body so
// the operator can see what the upstream actually said.
//
// As an MCP resource payload this serialises to a JSON object with
// "error", "detail", and optionally "body" — the agent can pattern-
// match on `error == "core_unavailable"` or
// `error == "apps_unavailable"` to fall back to journal + audit
// reads (which never need an upstream to be up).
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
// friendly "fallback" hint pointing at the resources that survive an
// upstream being down. Used by every system://, apps://, docker://
// resource so the shape is consistent across the proxy surface.
//
// The fallback hint is uniform regardless of which service failed —
// audit:// + journal:// never need any upstream to be up, so they're
// the agent's pivot target in every degraded scenario.
func AsErrorPayload(err error) []byte {
	e := &Error{Code: "service_unavailable", Detail: err.Error()}
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
		Fallback: "audit:// and journal:// (for powerlab- units) remain readable — they don't need any upstream to be up",
	}
	b, _ := json.Marshal(out)
	return b
}
