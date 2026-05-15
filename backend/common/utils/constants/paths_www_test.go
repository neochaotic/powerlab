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
	"bufio"
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
	const want = "/usr/share/powerlab/www"
	if constants.DefaultWWWPath != want {
		t.Errorf("Linux production DefaultWWWPath = %q, want %q (must match systemd unit `-w` flag in scripts/package-linux.sh)",
			constants.DefaultWWWPath, want)
	}
}

// Lock the systemd unit template + the constant on the same path.
// If someone changes one without the other, this test catches it.
// The package-linux.sh script emits an inline systemd unit; we read
// it and extract the `-w` argument.
func TestDefaultWWWPath_MatchesSystemdUnitFlag(t *testing.T) {
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
	f, err := os.Open(pkgScript)
	if err != nil {
		t.Skipf("package-linux.sh not readable at %s: %v", pkgScript, err)
	}
	defer f.Close()

	// Look for `ExecStart=/usr/bin/powerlab-gateway -w <path>`.
	re := regexp.MustCompile(`ExecStart=/usr/bin/powerlab-gateway\s+-w\s+(\S+)`)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	var unitPath string
	for sc.Scan() {
		if m := re.FindStringSubmatch(sc.Text()); m != nil {
			unitPath = strings.TrimSpace(m[1])
			break
		}
	}
	if unitPath == "" {
		t.Skip("could not locate ExecStart line in package-linux.sh; test inert")
	}

	// On Linux production runners (with /etc/powerlab marker), assert
	// the constant equals the unit path. In dev sandbox, only assert
	// that BOTH point to the same conceptual location ending in /www.
	if _, err := os.Stat("/etc/powerlab"); err == nil {
		if constants.DefaultWWWPath != unitPath {
			t.Errorf("DefaultWWWPath %q diverges from systemd unit `-w %s` — this WILL bite production again (v0.6.12 cut bug class)",
				constants.DefaultWWWPath, unitPath)
		}
	} else {
		if !strings.HasSuffix(constants.DefaultWWWPath, "/www") {
			t.Errorf("DefaultWWWPath %q does not end in /www; convention is <constant-path>/www", constants.DefaultWWWPath)
		}
		if !strings.HasSuffix(unitPath, "/www") {
			t.Errorf("systemd unit path %q does not end in /www", unitPath)
		}
	}
}
