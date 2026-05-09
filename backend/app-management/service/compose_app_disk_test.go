package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neochaotic/powerlab/backend/app-management/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestComposeApp_DiskUsage(t *testing.T) {
	// Setup temp storage path
	tmpDir := t.TempDir()
	config.AppInfo.StoragePath = tmpDir

	appName := "test-app"
	appDataPath := filepath.Join(tmpDir, "AppData", appName)
	err := os.MkdirAll(appDataPath, 0755)
	assert.NoError(t, err)

	// Create a dummy file
	dummyFile := filepath.Join(appDataPath, "dummy.txt")
	content := []byte("hello world") // 11 bytes
	err = os.WriteFile(dummyFile, content, 0644)
	assert.NoError(t, err)

	app := &ComposeApp{
		Name: appName,
	}

	bytes, err := app.DiskUsage()
	assert.NoError(t, err)
	
	// du -sb might return slightly more due to block sizes or directory overhead
	// but on most Linux it should be exactly 11 for the file + some for dir.
	// We'll check if it's at least the file size.
	assert.GreaterOrEqual(t, bytes, int64(11))
}
