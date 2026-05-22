package file_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/file"
	"gotest.tools/v3/assert"
)

// TestArchiveFilesRoundTrip locks the observable output of the archives.v4
// migration: a directory tree archived by ArchiveFiles must read back with the
// same entry names (basename(commonPath)/relative-path) and contents. This is
// the behavior characterization for the archiver/v3 → archives/v4 swap.
func TestArchiveFilesRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	common := filepath.Join(tmp, "data")
	assert.NilError(t, os.MkdirAll(filepath.Join(common, "sub"), 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(common, "b.txt"), []byte("bbb"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(common, "sub", "a.txt"), []byte("aaa"), 0o644))

	ext, format, err := file.GetCompressionAlgorithm("zip")
	assert.NilError(t, err)
	assert.Equal(t, ext, ".zip")

	var buf bytes.Buffer
	err = file.ArchiveFiles(context.Background(), &buf, format, []string{common}, common)
	assert.NilError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	assert.NilError(t, err)

	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		assert.NilError(t, err)
		data, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(data)
	}

	// common/file.go names entries under basename(commonPath) preserving the
	// relative path: data/b.txt and data/sub/a.txt.
	assert.Equal(t, got["data/b.txt"], "bbb")
	assert.Equal(t, got["data/sub/a.txt"], "aaa")
}
