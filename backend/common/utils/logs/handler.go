// Package logs implements the read-side HTTP handlers for the
// PowerLab on-disk log directory (/var/log/powerlab/). Each service
// writes its stdout to its own .log file there; the audit JSONL +
// upgrade.log + rotated archives also live here. The UI's
// Settings → Logs pane is the read-only consumer.
//
// Why expose these via gateway + UI:
//   Without this package, operators have to SSH into the host to
//   read past install errors or any non-install diagnostic (gateway
//   routing, user auth, upgrade history). The live-install SSE
//   stream shows the current install but PAST runs are invisible
//   from the panel. This closes that operator-experience gap.
//
// Scope:
//   - List `.log` files (rotated `.log.gz` archives are out of MVP
//     scope; tracked separately for a future viewer enhancement).
//   - Read content with a tail cap (default 200 KB) so a 50 MB
//     log doesn't fill the panel.
//   - Path-traversal hardening — filename is validated against a
//     strict allowlist BEFORE filepath.Join.
package logs

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultTailBytes caps the per-file response size when the caller
// doesn't specify a `tail=` query. 200 KB is roughly 4000 log lines
// — enough to debug "the last hour" without freezing the browser.
const DefaultTailBytes int64 = 200 * 1024

// MaxTailBytes caps the explicit `tail=` query too, so a caller
// can't request a 1 GB read and DoS the gateway. Operators with a
// genuine need for the full file can SSH.
const MaxTailBytes int64 = 5 * 1024 * 1024 // 5 MB

// filenameAllowlist is the strict regex every requested filename
// must match. Lowercase + digit + `_-.` only, max 64 chars. Defeats
// path traversal at the input gate (never relies on filepath.Clean
// alone — defense in depth).
var filenameAllowlist = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}\.log$`)

// FileEntry is the shape returned by the list endpoint.
type FileEntry struct {
	Name        string `json:"name"`
	SizeBytes   int64  `json:"size_bytes"`
	ModifiedTs  string `json:"modified_ts"`
	ModifiedUs  int64  `json:"modified_us"`
}

// envelope mirrors the rest of the gateway's JSON shape.
type envelope[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message,omitempty"`
}

// ListFilesHTTPHandler returns a handler for `GET /v1/logs/files`.
// Lists `.log` files in logDir, newest-first by mtime. Rotated
// archives (`.log.gz`) are intentionally excluded — out of MVP
// scope; a future enhancement can add transparent decompression.
//
// Errors:
//   - 405 if method != GET
//   - 500 if logDir is missing or unreadable
//
// Auth: this handler trusts the caller is authenticated. The
// gateway's public mux wraps it with JWT middleware.
func ListFilesHTTPHandler(logDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		entries, err := listLogFiles(logDir)
		if err != nil {
			http.Error(w, "read log dir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(envelope[[]FileEntry]{Data: entries})
	}
}

// ReadFileHTTPHandler returns a handler for `GET /v1/logs/files/{name}`.
// `name` is read from the LAST path segment. Returns the last N bytes
// of the file as plain text (`text/plain; charset=utf-8`).
//
// Query params:
//   - tail (optional, default DefaultTailBytes, max MaxTailBytes)
//
// Errors:
//   - 400 if name fails the allowlist (path traversal attempt or
//     non-`.log` file)
//   - 404 if the file doesn't exist in logDir
//   - 405 if method != GET
//   - 500 on read failure
func ReadFileHTTPHandler(logDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := lastPathSegment(r.URL.Path)
		if !filenameAllowlist.MatchString(name) {
			http.Error(w, "invalid filename", http.StatusBadRequest)
			return
		}

		tail := DefaultTailBytes
		if q := r.URL.Query().Get("tail"); q != "" {
			if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 {
				if n > MaxTailBytes {
					n = MaxTailBytes
				}
				tail = n
			}
		}

		full := filepath.Join(logDir, name)
		// filepath.Join + the allowlist together prevent escape from
		// logDir. The allowlist forbids "/", "..", and null bytes.
		// Belt-and-braces: verify the resolved path is still a child
		// of logDir.
		if rel, err := filepath.Rel(logDir, full); err != nil || strings.HasPrefix(rel, "..") {
			http.Error(w, "invalid filename", http.StatusBadRequest)
			return
		}

		f, err := os.Open(full)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "open: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			http.Error(w, "stat: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Seek to (size - tail) clamped at 0. Reading from the end is
		// the cheapest way to satisfy "show me the recent activity"
		// without loading the entire file into memory.
		offset := stat.Size() - tail
		if offset < 0 {
			offset = 0
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			http.Error(w, "seek: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Log-Size-Bytes", strconv.FormatInt(stat.Size(), 10))
		w.Header().Set("X-Log-Tail-Offset", strconv.FormatInt(offset, 10))
		_, _ = io.Copy(w, f)
	}
}

func listLogFiles(logDir string) ([]FileEntry, error) {
	dirEntries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		// Only surface .log files — .log.gz archives are out of MVP.
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		if !filenameAllowlist.MatchString(name) {
			// Defensive: skip names that wouldn't survive the read-
			// handler's allowlist anyway.
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, FileEntry{
			Name:       name,
			SizeBytes:  info.Size(),
			ModifiedTs: info.ModTime().UTC().Format(time.RFC3339),
			ModifiedUs: info.ModTime().UnixMicro(),
		})
	}
	// Newest first — operators almost always want recent activity.
	sort.Slice(out, func(i, j int) bool {
		return out[i].ModifiedUs > out[j].ModifiedUs
	})
	return out, nil
}

func lastPathSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

