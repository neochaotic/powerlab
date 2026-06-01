package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// REGRESSION (2026-06-01): same IPv6 bracket-parsing bug fixed in
// backend/common/utils/jwt/http_middleware.go on 2026-05-31 (PR #653)
// lived in audit's realIP helper too. Mac→VM (limactl, Docker
// Desktop) connections arrive with RemoteAddr `[::1]:PORT`. The
// previous LastIndexByte(:) strip left brackets in the result —
// downstream `remote == "::1"` comparison in HTTPMiddleware then
// missed every IPv6 loopback request and recorded the raw `[::1]`
// instead of the LoopbackSentinel.
//
// This test locks the canonical IPv4 + IPv6 + bracket + XFF paths.
func TestRealIP_HandlesIPv6Brackets(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{name: "ipv4_loopback_port", remoteAddr: "127.0.0.1:54321", want: "127.0.0.1"},
		{name: "ipv6_loopback_brackets_port", remoteAddr: "[::1]:54321", want: "::1"},
		{name: "ipv6_addr_brackets_port", remoteAddr: "[fe80::1]:8080", want: "fe80::1"},
		{name: "xff_overrides_remoteaddr", remoteAddr: "10.0.0.1:1", xff: "203.0.113.5", want: "203.0.113.5"},
		{name: "xff_first_hop_wins", remoteAddr: "10.0.0.1:1", xff: "203.0.113.5, 10.0.0.99", want: "203.0.113.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := realIP(r)
			if got != tc.want {
				t.Fatalf("realIP(%q xff=%q) = %q; want %q", tc.remoteAddr, tc.xff, got, tc.want)
			}
		})
	}
}
