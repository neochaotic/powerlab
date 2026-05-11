package service

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// newOriginChecker returns a CheckOrigin function suitable for the
// engineio websocket / polling transports. The returned function
// applies the #219 allowlist rules described on the type-level
// comments at TestNewOriginChecker_*.
//
// Pre-conditions:
//   - allowedOrigins entries are full origins like
//     "http://my-other-app.local:3000". Entries are trimmed and
//     lowercased before comparison; blank entries are ignored.
//
// Logging: rejected origins are emitted at WARN with the offending
// Origin + the destination Host so the operator can either tighten
// the allowlist further or add a legitimate cross-origin app.
func newOriginChecker(allowedOrigins []string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(strings.ToLower(o))
		if o != "" {
			allowed[o] = struct{}{}
		}
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// Non-browser clients (curl, socket.io-client without an
		// explicit Origin) don't send the header. The CORS bypass
		// risk is browser-specific (an attacker page can't strip
		// the Origin header), so missing-Origin requests are safe
		// to allow.
		if origin == "" {
			return true
		}

		// Same-origin: Origin host equals the request's destination
		// Host. Common case when the message-bus is reached directly
		// (no reverse proxy rewrite) or when the gateway preserves
		// the Host header.
		if u, err := url.Parse(origin); err == nil && u.Host != "" && strings.EqualFold(u.Host, r.Host) {
			return true
		}

		if _, ok := allowed[strings.ToLower(origin)]; ok {
			return true
		}

		_log.Warn(context.Background(), "rejected socketio origin (#219)", slog.String("origin", origin), slog.String("host", r.Host))
		return false
	}
}
