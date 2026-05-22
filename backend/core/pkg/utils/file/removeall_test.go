package file

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRemoveAllRecursive is the regression lock for #533: RemoveAll must
// remove a directory tree that contains nested subdirectories (not just a
// flat directory of files). The previous implementation only os.Remove'd
// non-dir entries and then os.Remove'd the top dir, leaving empty subdirs
// behind and returning "directory not empty".
func TestRemoveAllRecursive(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")

	// root/a.txt, root/sub/b.txt, root/sub/deep/c.txt
	if err := os.MkdirAll(filepath.Join(root, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "sub", "b.txt"),
		filepath.Join(root, "sub", "deep", "c.txt"),
	} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := RemoveAll(root); err != nil {
		t.Fatalf("RemoveAll(nested tree) returned error: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("RemoveAll should have removed the whole tree; stat err = %v", err)
	}
}
