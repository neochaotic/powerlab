package constants

import (
	"os"
	"path/filepath"
)

// Production install paths. Actual values are set per-platform in
// paths_<goos>.go (build-tagged). The variables here exist so the
// package compiles on platforms we have not shipped yet — a binary
// that actually starts on such a platform would fail downstream
// because no service writes a config there. That is intentional:
// adding a platform without a matching install pipeline should be
// a noticeable failure, not a silent fallback.
var (
	DefaultConfigPath   = ""
	DefaultConstantPath = ""
	DefaultDataPath     = ""
	DefaultFilePath     = ""
	DefaultLogPath      = ""
	DefaultRuntimePath  = ""
	// DefaultWWWPath is the canonical path where the static UI bundle
	// is installed. On Linux production it lives under DefaultConstantPath
	// (/usr/share/powerlab/www) — matches the `-w` flag in the systemd
	// unit emitted by scripts/package-linux.sh. The gateway's main.go
	// uses this as the default for the `-w` flag so a developer running
	// the binary by hand picks up the same path the installer wrote to.
	DefaultWWWPath = ""
)

// devProductionMarkers are well-known directories created by a real
// production install. If any exists, the binary trusts the platform
// defaults set in paths_<goos>.go and skips the dev-sandbox rewrite.
//
//   /etc/powerlab          – Linux production install
//   /opt/powerlab/etc      – macOS production install
//   /etc/casaos            – legacy CasaOS install (co-resident hosts)
var devProductionMarkers = []string{
	"/etc/powerlab",
	"/opt/powerlab/etc",
	"/etc/casaos",
}

// maybeApplyDevSandbox is called by each platform's init() AFTER it has
// set the production defaults. If the binary is running outside an
// installed environment (no production marker on disk), the defaults
// are pivoted into the project tree so multiple services running under
// `./start.sh` share a writable sandbox without touching system
// directories. This split lets paths_<goos>.go run first (Go init order
// is alphabetical, so `paths_darwin.go` runs before `paths.go` — but
// either way, the platform must set defaults first) and the override
// runs deterministically after.
func maybeApplyDevSandbox() {
	for _, marker := range devProductionMarkers {
		if _, err := os.Stat(marker); err == nil {
			return // production install — keep platform defaults
		}
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return
	}
	baseDir := currentDir
	for {
		if _, err := os.Stat(filepath.Join(baseDir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(baseDir)
		if parent == baseDir {
			break
		}
		baseDir = parent
	}

	// Resolve the project's `backend/` root so all services share a
	// single sandbox tree. Three cases:
	//   1. baseDir IS backend/                — running from backend/
	//   2. baseDir is backend/<service>/      — step up one level
	//   3. anything else                      — assume backend/ is a child
	backendDir := baseDir
	if filepath.Base(baseDir) == "backend" {
		backendDir = baseDir
	} else if filepath.Base(filepath.Dir(baseDir)) == "backend" {
		backendDir = filepath.Dir(baseDir)
	} else {
		backendDir = filepath.Join(baseDir, "backend")
	}

	DefaultConfigPath = filepath.Join(backendDir, "conf")
	DefaultConstantPath = filepath.Join(backendDir, "share")
	DefaultDataPath = filepath.Join(backendDir, "data")
	DefaultFilePath = filepath.Join(backendDir, "data", "files")
	DefaultLogPath = filepath.Join(backendDir, "logs")
	DefaultRuntimePath = filepath.Join(backendDir, "runtime")
	DefaultWWWPath = filepath.Join(backendDir, "share", "www")
}
