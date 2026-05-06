package constants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPaths(t *testing.T) {
	if _, err := os.Stat("/etc/casaos"); os.IsNotExist(err) {
		assert.NotEqual(t, "/etc/casaos", DefaultConfigPath)
		assert.Contains(t, DefaultConfigPath, "backend/conf")
		assert.Contains(t, DefaultDataPath, "backend/data")
		assert.Contains(t, DefaultLogPath, "backend/logs")
		assert.Contains(t, DefaultRuntimePath, "backend/runtime")
	} else {
		assert.Equal(t, "/etc/casaos", DefaultConfigPath)
	}
}

// TestBackendDirResolvesFromServiceSubdir verifies that the backendDir
// computation correctly handles services running from backend/<service>/
// (the common case), not just from backend/ directly.
//
// The bug: if the service runs from backend/app-management/, the old code
// produced backend/app-management/backend/ instead of backend/.
func TestBackendDirResolvesFromServiceSubdir(t *testing.T) {
	if _, err := os.Stat("/etc/casaos"); !os.IsNotExist(err) {
		t.Skip("only relevant in dev mode (no /etc/casaos)")
	}

	// AppsPath must be under backend/data/, not under backend/<service>/backend/data/
	assert.NotContains(t, DefaultDataPath, filepath.Join("app-management", "backend"),
		"AppsPath must resolve to backend/data, not backend/app-management/backend/data")
	assert.NotContains(t, DefaultDataPath, filepath.Join("common", "backend"),
		"AppsPath must resolve to backend/data, not backend/common/backend/data")
	assert.NotContains(t, DefaultRuntimePath, filepath.Join("app-management", "backend"),
		"RuntimePath must resolve to backend/runtime, not backend/app-management/backend/runtime")
}
