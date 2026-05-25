package constants_test

// Sprint 18 P0 regression — locks the canonical UI path so the
// gateway's `-w` default and the systemd unit's `-w` argument never
// drift again.
//
// Background: v0.6.12 cut surfaced a divergence — gateway's main.go
// defaulted `-w` to /var/lib/powerlab/www, but the systemd unit
// emitted by package-linux.sh hard-coded /usr/share/powerlab/www.
// A developer running the binary by hand or doing a dev hot-swap
// would write the UI to the WRONG path; the running gateway kept
// serving the stale bundle silently. The AuditPane "disappeared"
// from the user's view during the v0.6.12 verify.
//
// Single source of truth: `constants.DefaultWWWPath`.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

func TestDefaultWWWPath_NotEmpty(t *testing.T) {
	if constants.DefaultWWWPath == "" {
		t.Fatal("DefaultWWWPath must be set by platform init or maybeApplyDevSandbox")
	}
}

func TestDefaultWWWPath_LinuxProductionValue(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux production path only")
	}
	// The dev sandbox overrides this when no /etc/powerlab marker
	// exists. Only assert the production value when the marker IS
	// present (i.e. in a real install / Linux CI runner that has it).
	if _, err := os.Stat("/etc/powerlab"); err != nil {
		t.Skip("/etc/powerlab not present — running under dev sandbox; production path test does not apply")
	}
	// DefaultWWWPath is still the gateway's `-w` flag DEFAULT (used by
	// the dev `-w` override and as the legacy on-disk lookup path). The
	// production systemd unit no longer passes `-w` (ADR-0043 phase 3 —
	// the UI is embedded), so this only pins the constant's value, not a
	// unit argument.
	const want = "/usr/share/powerlab/www"
	if constants.DefaultWWWPath != want {
		t.Errorf("Linux production DefaultWWWPath = %q, want %q", constants.DefaultWWWPath, want)
	}
}

// ADR-0043 phase 3: the gateway serves its UI embedded in the binary,
// so the systemd unit must NOT pass `-w <dir>` — passing it would pin
// the gateway to an on-disk bundle and revive the version-skew bug
// class (the gateway serving a stale UI from disk after a binary
// upgrade). This locks the unit on the embedded contract: a future
// edit that re-adds `-w` to the gateway ExecStart fails here.
func TestGatewaySystemdUnitHasNoWFlag(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only — the systemd unit is Linux-specific")
	}

	// Find the repo root by walking up from this file's dir.
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Dir(here)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("cannot find repo root from %s", here)
		}
		dir = parent
	}
	pkgScript := filepath.Join(dir, "scripts", "package-linux.sh")
	data, err := os.ReadFile(pkgScript)
	if err != nil {
		t.Skipf("package-linux.sh not readable at %s: %v", pkgScript, err)
	}

	// The gateway ExecStart line must be a bare invocation — no `-w`.
	gatewayExec := regexp.MustCompile(`ExecStart=/usr/bin/powerlab-gateway\b[^\n]*`)
	m := gatewayExec.FindString(string(data))
	if m == "" {
		t.Fatal("could not locate gateway ExecStart line in package-linux.sh")
	}
	if strings.Contains(m, "-w") {
		t.Errorf("gateway ExecStart still passes `-w` (%q) — ADR-0043 phase 3 serves the UI embedded; the `-w` flag must not appear in the production unit (it would pin the gateway to a stale on-disk bundle after a binary upgrade)", m)
	}
}
