package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxFrontendErrorBodyBytes caps the inbound payload at 16 KiB. A real
// browser stack trace fits in 4–8 KiB; anything larger is either a
// runaway loop emitting the same handler over and over (the dedupe
// happens client-side) or an exfiltration attempt.
const maxFrontendErrorBodyBytes = 16 * 1024

// frontendErrorBody is the wire shape the SvelteKit shell POSTs.
//
//   - message: required, non-empty after trim. Empty/whitespace-only
//     payloads are rejected — they carry no signal.
//   - stack:   optional. Captured from Error.stack / event.reason.
//   - url:     the page URL where the error fired (location.pathname
//     plus search; never the full origin to avoid embedding the
//     deployment host).
//   - ua:      optional User-Agent fingerprint.
//   - viewport: optional {w, h} pair — useful when reproducing
//     layout-dependent crashes.
type frontendErrorBody struct {
	Message  string      `json:"message"`
	Stack    string      `json:"stack,omitempty"`
	URL      string      `json:"url,omitempty"`
	UA       string      `json:"ua,omitempty"`
	Viewport interface{} `json:"viewport,omitempty"`
}

// FrontendErrorHTTPHandler returns a stdlib handler for
// POST /v1/audit/frontend-error.
//
// JWT auth is wrapped at the gateway level (HTTPJWT). When that
// middleware accepts a request it populates `user_id` and `user_name`
// request headers; this handler reads them to denormalise the
// audit record so the UI can render "alice triggered a UI error on
// /apps" without joining against user-service.
//
// Returns 202 Accepted on success — the recorder is async (the
// browser's error fired and was already shown to the user; we don't
// hold its tab waiting for a JSONL fsync).
func FrontendErrorHTTPHandler(rec *Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxFrontendErrorBodyBytes+1))
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(body) > maxFrontendErrorBodyBytes {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var payload frontendErrorBody
		if err := json.Unmarshal(body, &payload); err != nil {
			// Surface a generic parse error — the actual cause
			// (truncated, syntax) doesn't matter to the client.
			http.Error(w, "malformed json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.Message) == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		// Build the audit record. Method/Path/Status mirror the
		// incoming HTTP request so existing UI filters (group by
		// path, filter by status) keep working uniformly across
		// record kinds.
		record := Record{
			Kind:     "ui_error",
			Method:   http.MethodPost,
			Path:     "/v1/audit/frontend-error",
			Status:   http.StatusAccepted,
			RemoteIP: realIP(r),
			Payload: map[string]any{
				"message": payload.Message,
			},
		}
		if payload.Stack != "" {
			record.Payload["stack"] = payload.Stack
		}
		if payload.URL != "" {
			record.Payload["url"] = payload.URL
		}
		if payload.UA != "" {
			record.Payload["ua"] = payload.UA
		}
		if payload.Viewport != nil {
			record.Payload["viewport"] = payload.Viewport
		}
		// User identity from the gateway's JWT decode-only layer.
		if uid := r.Header.Get("user_id"); uid != "" {
			if n, err := strconv.ParseInt(uid, 10, 64); err == nil {
				record.UserID = &n
			}
		}
		if uname := r.Header.Get("user_name"); uname != "" {
			record.Username = &uname
		}
		record.FillTimestamps(time.Now())

		rec.Submit(record)

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}


