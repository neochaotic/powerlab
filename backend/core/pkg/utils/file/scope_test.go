package file

import (
	"errors"
	"testing"
)

func TestResolveWithinScope(t *testing.T) {
	cases := []struct {
		name    string
		scope   string
		req     string
		want    string
		wantErr bool
	}{
		// Acceptance criteria (#36).
		{"in scope", "/DATA", "/DATA/foo", "/DATA/foo", false},
		{"absolute escape", "/DATA", "/etc/passwd", "", true},
		{"dotdot escape", "/DATA", "/DATA/../etc/passwd", "", true},

		// Adversarial traversals.
		{"deep dotdot", "/DATA", "/DATA/a/b/../../../etc/shadow", "", true},
		{"trailing slash in scope", "/DATA/", "/DATA/foo", "/DATA/foo", false},
		{"scope root itself", "/DATA", "/DATA", "/DATA", false},
		{"scope root with slash", "/DATA", "/DATA/", "/DATA", false},
		{"sibling prefix not in scope", "/DATA", "/DATArogue/x", "", true},
		{"nested ok", "/DATA", "/DATA/AppData/blinko/db", "/DATA/AppData/blinko/db", false},
		{"redundant slashes cleaned", "/DATA", "/DATA//sub///f", "/DATA/sub/f", false},
		{"relative under scope", "/DATA", "foo/bar", "/DATA/foo/bar", false},
		{"relative dotdot escape", "/DATA", "../etc/passwd", "", true},

		// Legacy whole-fs (empty scope).
		{"legacy passes etc", "", "/etc/passwd", "/etc/passwd", false},
		{"legacy cleans dotdot", "", "/DATA/../etc/passwd", "/etc/passwd", false},
		{"legacy whitespace scope", "   ", "/etc/shadow", "/etc/shadow", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveWithinScope(c.scope, c.req)
			if c.wantErr {
				if !errors.Is(err, ErrPathOutsideScope) {
					t.Fatalf("expected ErrPathOutsideScope for %q in scope %q, got (%q, %v)", c.req, c.scope, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q in scope %q: %v", c.req, c.scope, err)
			}
			if got != c.want {
				t.Errorf("ResolveWithinScope(%q, %q) = %q, want %q", c.scope, c.req, got, c.want)
			}
		})
	}
}
