package service

import (
	"os"
	"path/filepath"
	"testing"
)

// isDirEmpty gates whether image-skeleton seeding runs for a bind source.
// false = "skip seeding". Locks the four observable cases (missing dir is
// treated as empty; a file or a populated dir is not).
func TestIsDirEmpty(t *testing.T) {
	root := t.TempDir()

	missing := filepath.Join(root, "does-not-exist")
	if !isDirEmpty(missing) {
		t.Errorf("missing path should be treated as empty")
	}

	empty := filepath.Join(root, "empty")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if !isDirEmpty(empty) {
		t.Errorf("empty dir should be empty")
	}

	populated := filepath.Join(root, "populated")
	if err := os.Mkdir(populated, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(populated, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isDirEmpty(populated) {
		t.Errorf("dir with content should not be empty")
	}

	file := filepath.Join(root, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isDirEmpty(file) {
		t.Errorf("a file (not a dir) should return false, not empty")
	}
}
