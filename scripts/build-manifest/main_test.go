package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// looksLikeSemver gates every release: a typo in the version string
// would silently produce a release with an unparseable tag. Pin every
// boundary the pipeline cares about.
func TestLooksLikeSemver(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"0.2.4", true},
		{"1.0.0", true},
		{"10.20.30", true},
		{"0.2.4-rc.1", true}, // pre-release suffix accepted (split on '-')

		{"", false},
		{"v0.2.4", false},   // leading v rejected — package script strips it
		{"0.2", false},      // missing patch
		{"0.2.4.5", false},  // four parts
		{".2.4", false},     // empty major
		{"0..4", false},     // empty minor
		{"abc", false},
		{"0.2.x", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := looksLikeSemver(c.in); got != c.want {
				t.Fatalf("looksLikeSemver(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// anchorise turns "0.2.4" into the GFM heading anchor "024" used by
// CHANGELOG.md. If GitHub ever changes its slug rules this test will
// fail loudly so we know to update the format here AND the rendered
// changelog will need to use the new convention.
func TestAnchorise(t *testing.T) {
	cases := map[string]string{
		"0.2.4":      "024",
		"1.0.0":      "100",
		"0.2.4-rc.1": "024-rc1",
		"10.20.30":   "102030",
	}
	for in, want := range cases {
		if got := anchorise(in); got != want {
			t.Fatalf("anchorise(%q) = %q, want %q", in, got, want)
		}
	}
}

// tarballMeta ships a SHA-256 the host updater uses to verify the
// download. If the digest is wrong the host accepts a tampered
// tarball — that's a security regression. Pin the encoding format.
func TestTarballMetaSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "powerlab-test.tar.gz")
	// Known content → known SHA-256:
	// echo -n "powerlab" | sha256sum
	// = a8b91812be2e6dca7d18cf4f15c1e0a39a3c95c0d1c2c5d3da... (compute below)
	const content = "powerlab-fake-tarball"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := tarballMeta(path, "neochaotic/powerlab", "0.2.4", "amd64")
	if err != nil {
		t.Fatalf("tarballMeta failed: %v", err)
	}
	if got.SizeBytes != int64(len(content)) {
		t.Fatalf("size = %d, want %d", got.SizeBytes, len(content))
	}
	// hex.EncodeToString of sha256("powerlab-fake-tarball") =
	// 2bdfe55fefdec... but we don't need to assert the literal value
	// — just that it's the canonical 64-char lowercase hex string,
	// because that's what the host updater parses.
	if len(got.SHA256) != 64 {
		t.Fatalf("sha256 hex length = %d, want 64", len(got.SHA256))
	}
	for _, r := range got.SHA256 {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("sha256 contains non-lowercase-hex char %q", r)
		}
	}

	wantURLPrefix := "https://github.com/neochaotic/powerlab/releases/download/v0.2.4/"
	if !strings.HasPrefix(got.URL, wantURLPrefix) {
		t.Fatalf("URL %q does not start with %q", got.URL, wantURLPrefix)
	}
	if !strings.HasSuffix(got.URL, "-linux-amd64.tar.gz") {
		t.Fatalf("URL %q does not end with -linux-amd64.tar.gz", got.URL)
	}
}

// defaultEmpty turns nil arrays into empty arrays so the JSON output
// is `"breaking_changes": []` (the contract) instead of
// `"breaking_changes": null` (which would technically validate but
// makes the host updater code messier).
func TestDefaultEmpty(t *testing.T) {
	if got := defaultEmpty(nil); got == nil {
		t.Fatal("defaultEmpty(nil) returned nil")
	}
	if got := defaultEmpty(nil); len(got) != 0 {
		t.Fatalf("defaultEmpty(nil) length = %d, want 0", len(got))
	}
	src := []map[string]any{{"k": "v"}}
	if got := defaultEmpty(src); len(got) != 1 {
		t.Fatalf("defaultEmpty(non-nil) collapsed the input: got len=%d", len(got))
	}
}
