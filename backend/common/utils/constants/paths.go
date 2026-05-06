package constants

import (
	"os"
	"path/filepath"
)

var (
	DefaultConfigPath   = "/etc/casaos"
	DefaultConstantPath = "/usr/share/casaos"
	DefaultDataPath     = "/var/lib/casaos"
	DefaultFilePath     = "/var/lib/casaos/files"
	DefaultLogPath      = "/var/log/casaos"
	DefaultRuntimePath  = "/var/run/casaos"
)

func init() {
	// If /etc/casaos doesn't exist, assume we are in dev mode and use local paths
	if _, err := os.Stat(DefaultConfigPath); os.IsNotExist(err) {
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
