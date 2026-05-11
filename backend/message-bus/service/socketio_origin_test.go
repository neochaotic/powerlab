package service

import (
	"net/http"
	"testing"
)

// Regression suite for #219 — SocketIO CheckOrigin must reject
// origins that are not allowlisted. Prior code returned `true`
// unconditionally, allowing any origin to open the WebSocket /
// polling stream against a victim's browser.
//
// Rules under test:
//   - Empty Origin header → allow (non-browser client like curl,
//     socket.io-client without Origin set)
//   - Origin host equals request Host → allow (same-origin call)
//   - Origin matches an entry in the configured allowlist
//     (case-insensitive) → allow
//   - Anything else → reject
func TestNewOriginChecker_EmptyOrigin_Allow(t *testing.T) {
	check := newOriginChecker(nil)
	req := &http.Request{
		Host:   "localhost:8080",
		Header: http.Header{},
	}
	if !check(req) {
		t.Fatalf("empty Origin must be allowed (non-browser client)")
	}
}

func TestNewOriginChecker_SameOrigin_Allow(t *testing.T) {
	check := newOriginChecker(nil)
	req := &http.Request{
		Host: "powerlab.local:8080",
		Header: http.Header{
			"Origin": []string{"http://powerlab.local:8080"},
		},
	}
	if !check(req) {
		t.Fatalf("same-host Origin must be allowed")
	}
}

func TestNewOriginChecker_ConfiguredAllowlist_Allow(t *testing.T) {
	check := newOriginChecker([]string{"http://my-other-app.local:3000"})
	req := &http.Request{
		Host: "powerlab.local:8080",
		Header: http.Header{
			"Origin": []string{"http://my-other-app.local:3000"},
		},
	}
	if !check(req) {
		t.Fatalf("origin in configured allowlist must be allowed")
	}
}

func TestNewOriginChecker_AllowlistCaseInsensitive_Allow(t *testing.T) {
	check := newOriginChecker([]string{"HTTP://My-Other-App.Local:3000"})
	req := &http.Request{
		Host: "powerlab.local:8080",
		Header: http.Header{
			"Origin": []string{"http://my-other-app.local:3000"},
		},
	}
	if !check(req) {
		t.Fatalf("allowlist comparison must be case-insensitive")
	}
}

func TestNewOriginChecker_UnknownOrigin_Reject(t *testing.T) {
	check := newOriginChecker([]string{"http://trusted.local:3000"})
	req := &http.Request{
		Host: "powerlab.local:8080",
		Header: http.Header{
			"Origin": []string{"http://attacker.example.com"},
		},
	}
	if check(req) {
		t.Fatalf("unknown Origin must be rejected — this is the #219 bypass")
	}
}

func TestParseAllowedOrigins(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty input", "", nil},
		{"single", "http://a:1", []string{"http://a:1"}},
		{"two with whitespace", " http://a:1 , http://b:2 ", []string{"http://a:1", "http://b:2"}},
		{"blank entries dropped", " , http://a:1 ,, ", []string{"http://a:1"}},
		{"only blanks", " , , ", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAllowedOrigins(tc.in)
			if !equalStrings(got, tc.want) {
				t.Fatalf("parseAllowedOrigins(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNewOriginChecker_BlankConfigEntry_Ignored(t *testing.T) {
	// Operator configures `AllowedOrigins = " "` — a single blank
	// entry must NOT collapse into a wildcard "allow empty origin
	// header" rule. Any Origin still has to match either same-host
	// or a non-blank allowlist entry.
	check := newOriginChecker([]string{"  ", ""})
	req := &http.Request{
		Host: "powerlab.local:8080",
		Header: http.Header{
			"Origin": []string{"http://attacker.example.com"},
		},
	}
	if check(req) {
		t.Fatalf("blank allowlist entries must be ignored, not allow everything")
	}
}
