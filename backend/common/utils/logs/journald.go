package logs

// journald.go implements the live-streaming endpoint for systemd journal
// reads (one PowerLab unit per request). Spawns `journalctl -u <unit>
// -o json -f` as a subprocess and re-emits each parsed line as an SSE
// `data:` event. Subprocess lifecycle is bound to the request's
// context: when the client disconnects, the subprocess is killed via
// the same context cancellation path.
//
// Why a subprocess instead of go-systemd / sd_journal cgo:
//   - The pure-Go journald-reader libraries are unmaintained.
//   - The cgo sd_journal binding pulls a build-time dependency on
//     libsystemd-dev that we don't otherwise need.
//   - `journalctl -o json -f` is the canonical operator interface;
//     using it directly means our behavior matches what an SSH-ed
//     operator sees, which is the truth source.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// serviceNameAllowlist is the strict regex every requested service
// name must match BEFORE we let it near `exec.Command`. Lowercase +
// digit + hyphen only, must start with a lowercase letter, max 64
// chars. The constraint set defeats shell metacharacters
// (`; & | $ \` ' " < > * ? ! ~`), spaces, path traversal (`/ ..`),
// and null bytes. The 64-char cap matches systemd's unit name limit
// (255 bytes) with headroom.
var serviceNameAllowlist = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// StreamJournaldHTTPHandler returns a handler for
// `GET /v1/logs/services/{service}/stream`. The handler:
//   - Reads `{service}` from the LAST-BUT-ONE path segment
//     (final segment is the literal `stream`)
//   - Validates it against serviceNameAllowlist
//   - Sets SSE headers
//   - Spawns `journalctl -u <unitPrefix><service>.service -o json -f`
//   - Streams each JSON line as `data: {…}\n\n`
//
// unitPrefix is prepended to the validated service name (so the caller
// passes `powerlab-` to scope reads to PowerLab's own units). Empty
// prefix means raw unit names — only safe in tests.
//
// Errors:
//   - 400 if the service name fails the allowlist
//   - 405 if the method is not GET
//   - 500 if journalctl exits with a non-zero status before any data
//
// Auth is the caller's responsibility (the public mux wraps this
// with JWT middleware).
func StreamJournaldHTTPHandler(unitPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		service := extractServiceFromStreamPath(r.URL.Path)
		if !serviceNameAllowlist.MatchString(service) {
			http.Error(w, "invalid service name", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		// Defeat nginx / Apache buffering on operator proxies.
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Connection", "keep-alive")

		flusher, hasFlusher := w.(http.Flusher)
		// SSE headers are committed BEFORE we touch the subprocess so
		// that an exec failure can be surfaced over the already-
		// established SSE channel (as a typed error event) rather
		// than via http.Error — which would overwrite Content-Type
		// and confuse a browser that has already started parsing the
		// stream.
		fmt.Fprint(w, ": stream-open\n\n")
		if hasFlusher {
			flusher.Flush()
		}

		unit := unitPrefix + service + ".service"
		// CommandContext kills the subprocess when the request ctx
		// is cancelled (client disconnect, timeout, server shutdown).
		// This is the leak-prevention path — without it, a long-lived
		// operator session would accumulate dead journalctl PIDs.
		cmd := exec.CommandContext(r.Context(), "journalctl",
			"-u", unit, "-o", "json", "-f", "-n", "50")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %q\n\n", "stdout pipe: "+err.Error())
			if hasFlusher {
				flusher.Flush()
			}
			return
		}
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(w, "event: error\ndata: %q\n\n", "start journalctl: "+err.Error())
			if hasFlusher {
				flusher.Flush()
			}
			return
		}

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			sse, ok := journalLineToSSE(line)
			if !ok {
				continue
			}
			if _, err := w.Write(sse); err != nil {
				return
			}
			if hasFlusher {
				flusher.Flush()
			}
		}
		// Best-effort wait so the subprocess gets reaped on the
		// normal path. CommandContext handles the cancel case.
		_ = cmd.Wait()
	}
}

// extractServiceFromStreamPath pulls `<service>` from the route
// `/v1/logs/services/<service>/stream`. Returns empty string if the
// shape doesn't match — the allowlist will then reject it as expected.
func extractServiceFromStreamPath(p string) string {
	// Trim trailing slash so split is stable.
	p = strings.TrimSuffix(p, "/")
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return ""
	}
	if parts[len(parts)-1] != "stream" {
		return ""
	}
	return parts[len(parts)-2]
}

// journalLineToSSE parses one journalctl JSON line and returns the
// SSE-formatted bytes to write to the response stream. Returns
// ok=false when the line isn't valid JSON (journalctl occasionally
// emits status markers like `-- Reboot --` in -f mode).
func journalLineToSSE(line []byte) ([]byte, bool) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, false
	}
	msg, _ := raw["MESSAGE"].(string)
	priority, _ := raw["PRIORITY"].(string)
	tsMicro, _ := raw["__REALTIME_TIMESTAMP"].(string)

	payload := struct {
		Severity string `json:"severity"`
		Message  string `json:"message"`
		TsMicro  string `json:"ts_micro,omitempty"`
	}{
		Severity: mapJournalPriority(priority),
		Message:  msg,
		TsMicro:  tsMicro,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	out := append([]byte("data: "), body...)
	out = append(out, '\n', '\n')
	return out, true
}

// mapJournalPriority buckets RFC 5424 severity codes into the four
// colors the UI renders: error / warn / info / debug. Unknown / empty
// values default to info so a malformed line doesn't disappear into
// a debug-filtered view.
func mapJournalPriority(p string) string {
	n, err := strconv.Atoi(p)
	if err != nil {
		return "info"
	}
	switch {
	case n <= 3:
		return "error"
	case n == 4:
		return "warn"
	case n <= 6:
		return "info"
	default:
		return "debug"
	}
}

