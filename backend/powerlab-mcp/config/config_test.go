package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A fresh PowerLab box has no /etc/powerlab/mcp.conf until the first
// install writes one — and the service still has to boot. Load on a
// missing path must therefore return the defaults, NOT an error.
func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "does-not-exist.conf"))
	if err != nil {
		t.Fatalf("Load(missing) returned error %v; want nil (boot must not depend on the conf existing)", err)
	}
	if got != Default() {
		t.Fatalf("Load(missing) = %+v; want defaults %+v", got, Default())
	}
}

// The whole point of the conf is to move the listener off :9090 when an
// operator needs to. A written ListenAddr must win over the default.
func TestLoad_OverridesListenAddr(t *testing.T) {
	path := writeConf(t, "ListenAddr = 127.0.0.1:9595\n")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ListenAddr != "127.0.0.1:9595" {
		t.Fatalf("ListenAddr = %q; want %q", got.ListenAddr, "127.0.0.1:9595")
	}
	// Keys the conf didn't mention must keep their defaults — a partial
	// conf must not blank out the rest of the config.
	if got.AuditDir != Default().AuditDir {
		t.Fatalf("AuditDir = %q; a conf that only set ListenAddr must leave AuditDir at the default %q", got.AuditDir, Default().AuditDir)
	}
	if got.RuntimePath != Default().RuntimePath {
		t.Fatalf("RuntimePath = %q; a conf that only set ListenAddr must leave RuntimePath at the default %q", got.RuntimePath, Default().RuntimePath)
	}
}

// The auth gate resolves the JWT public key from RuntimePath, so an
// operator who relocates the PowerLab runtime dir must be able to point
// the gate at it via the conf.
func TestLoad_OverridesRuntimePath(t *testing.T) {
	path := writeConf(t, "RuntimePath = /run/custom-powerlab\n")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RuntimePath != "/run/custom-powerlab" {
		t.Fatalf("RuntimePath = %q; want %q", got.RuntimePath, "/run/custom-powerlab")
	}
}

// Operators comment configs; future PowerLab versions add keys this
// binary predates. Neither may break Load — comments and blank lines are
// skipped, and an unknown key is ignored (forward-compatible), not fatal.
func TestLoad_IgnoresCommentsBlanksAndUnknownKeys(t *testing.T) {
	path := writeConf(t, "# managed by powerlab installer\n\n  \nListenAddr = :7000\nFutureKnob = whatever\n")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ListenAddr != ":7000" {
		t.Fatalf("ListenAddr = %q; want %q (comments/blanks must not derail parsing)", got.ListenAddr, ":7000")
	}
}

// The Disabled key is the operator kill-switch. Default is false (the
// service runs). When flipped to a truthy value the daemon exits 0
// before binding — see main.go. The conf parser must recognise the
// standard truthy spellings so `Disabled = 1` / `Disabled = true` /
// `Disabled = on` all work, anything else stays "service runs".
func TestLoad_DisabledKey(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"true wins", "Disabled = true\n", true},
		{"yes wins", "Disabled = yes\n", true},
		{"on wins", "Disabled = on\n", true},
		{"1 wins", "Disabled = 1\n", true},
		{"uppercase TRUE wins (case-insensitive)", "Disabled = TRUE\n", true},
		{"false stays false", "Disabled = false\n", false},
		{"empty value stays false (no accidental shutdown)", "Disabled =\n", false},
		{"garbage stays false (no accidental shutdown)", "Disabled = blargh\n", false},
		{"missing key stays at default false", "ListenAddr = :7000\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Load(writeConf(t, tc.body))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.Disabled != tc.want {
				t.Fatalf("Disabled = %v (body %q); want %v", got.Disabled, tc.body, tc.want)
			}
		})
	}
}

// A syntactically broken line (no '=') must be skipped, not abort the
// load — a single fat-fingered line should never take the service down.
func TestLoad_SkipsMalformedLines(t *testing.T) {
	path := writeConf(t, "this line has no equals sign\nListenAddr = :8181\n")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error on a malformed line %v; want it skipped", err)
	}
	if got.ListenAddr != ":8181" {
		t.Fatalf("ListenAddr = %q; want %q (a bad line must not stop later valid lines)", got.ListenAddr, ":8181")
	}
}

func writeConf(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.conf")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp conf: %v", err)
	}
	return path
}

// EnableDestructiveTools is the operator opt-in for the install_app +
// uninstall_app tools (ADR-0046 batch 3). Default must be false —
// agents cannot mutate app state until the operator explicitly flips
// the knob. Default-on would be a security-violation by surprise.
func TestLoad_EnableDestructiveToolsDefaultFalse(t *testing.T) {
	got, err := Load(writeConf(t, ""))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.EnableDestructiveTools {
		t.Fatalf("EnableDestructiveTools=true on empty conf; want false (operators must opt in)")
	}
}

// Truthy spellings flip the knob; garbage stays false. Mirrors the
// Disabled key contract so an operator who knows one knows both.
func TestLoad_EnableDestructiveToolsAcceptsTruthyOnly(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"EnableDestructiveTools = true\n", true},
		{"EnableDestructiveTools = yes\n", true},
		{"EnableDestructiveTools = on\n", true},
		{"EnableDestructiveTools = 1\n", true},
		{"EnableDestructiveTools = TRUE\n", true},
		{"EnableDestructiveTools = false\n", false},
		{"EnableDestructiveTools = blargh\n", false},
		{"EnableDestructiveTools =\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			got, err := Load(writeConf(t, tc.body))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.EnableDestructiveTools != tc.want {
				t.Fatalf("EnableDestructiveTools=%v (body %q); want %v", got.EnableDestructiveTools, tc.body, tc.want)
			}
		})
	}
}
