package constants

import (
	"os"
	"path/filepath"
)

// Production install paths. The package script (scripts/package-linux.sh)
// writes the powerlab-specific equivalents (/etc/powerlab, /var/lib/powerlab,
// etc.) into /etc/<svc>/<svc>.conf — those values then come back through
// each service's config loader and replace what's defined here. The
// `casaos` defaults below are only used as last-resort fallbacks for
// services that haven't loaded their config yet.
var (
	DefaultConfigPath   = "/etc/powerlab"
	DefaultConstantPath = "/usr/share/powerlab"
	DefaultDataPath     = "/var/lib/powerlab"
	DefaultFilePath     = "/var/lib/powerlab/files"
	DefaultLogPath      = "/var/log/powerlab"
	DefaultRuntimePath  = "/var/run/powerlab"
)

func init() {
	// Dev mode: when there is no production install (neither /etc/powerlab
	// nor /etc/casaos exists), pivot all defaults into the project tree
	// so multiple services running under `./start.sh` can share a writable
	// sandbox without touching system directories.
	_, prodPowerLab := os.Stat("/etc/powerlab")
	_, prodCasaOS := os.Stat("/etc/casaos")
	if prodPowerLab != nil && prodCasaOS != nil {
		if currentDir, err := os.Getwd(); err == nil {
			// Find the project root (walk up from backend/xxx to backend/)
			// For simplicity, let's just use a "casaos" folder in the current working directory or parent
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
			
			// We want to use backend/ as the base for all data.
			// Handle three cases:
			//   1. Running from backend/         → baseDir IS backend/
			//   2. Running from backend/<svc>/   → parent of baseDir is backend/
			//   3. Running from anywhere else    → assume backend/ is a subdirectory
			backendDir := baseDir
			if filepath.Base(baseDir) == "backend" {
				// case 1: already in backend/
				backendDir = baseDir
			} else if filepath.Base(filepath.Dir(baseDir)) == "backend" {
				// case 2: in backend/<service>/ — step up one level
				backendDir = filepath.Dir(baseDir)
			} else {
				// case 3: assume backend/ lives under the current directory
				backendDir = filepath.Join(baseDir, "backend")
			}

			DefaultConfigPath = filepath.Join(backendDir, "conf")
			DefaultConstantPath = filepath.Join(backendDir, "share")
			DefaultDataPath = filepath.Join(backendDir, "data")
			DefaultFilePath = filepath.Join(backendDir, "data", "files")
			DefaultLogPath = filepath.Join(backendDir, "logs")
			DefaultRuntimePath = filepath.Join(backendDir, "runtime")
		}
	}
}
