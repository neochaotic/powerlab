package service

import (
	"strings"
	"testing"
)

// validateGatewayPort guards every gateway port write — the route
// handler, the startup config load, and the future in-UI port
// editor (issue #18) all flow through it. Pin every boundary.
func TestValidateGatewayPort(t *testing.T) {
	cases := []struct {
		name    string
		port    string
		wantErr string // empty == accept
	}{
		// Happy path
		{"common 8765", "8765", ""},
		{"low end 1", "1", ""},
		{"privileged 80", "80", ""},
		{"high end 65535", "65535", ""},

		// Boundaries
		{"zero rejected", "0", "out of range"},
		{"negative rejected", "-1", "out of range"},
		{"above max rejected", "65536", "out of range"},
		{"way above max rejected", "100000", "out of range"},

		// Non-integer
		{"empty string", "", "not a valid integer"},
		{"alpha", "abcd", "not a valid integer"},
		{"trailing junk", "8765x", "not a valid integer"},
		{"hex notation", "0x80", "not a valid integer"},
		{"float", "8080.5", "not a valid integer"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateGatewayPort(c.port)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("validateGatewayPort(%q) = %v, want nil", c.port, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateGatewayPort(%q) = nil, want error containing %q", c.port, c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("validateGatewayPort(%q) error %q does not contain %q", c.port, err.Error(), c.wantErr)
			}
		})
	}
}
