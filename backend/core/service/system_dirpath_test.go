package service

import (
	"os"
	"path/filepath"
	"testing"
)

// #38: GetDirPath flags symlinks so the listing can expose is_symlink.
func TestGetDirPath_FlagsSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "linkdir")); err != nil {
		t.Fatal(err)
	}

	svc := &systemService{}
	got, err := svc.GetDirPath(dir)
	if err != nil {
		t.Fatalf("GetDirPath: %v", err)
	}
	want := map[string]struct{ symlink, dir bool }{
		"real.txt": {false, false},
		"sub":      {false, true},
		"link.txt": {true, false},
		"linkdir":  {true, true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for _, p := range got {
		w, ok := want[p.Name]
		if !ok {
			t.Errorf("unexpected entry %q", p.Name)
			continue
		}
		if p.IsSymlink != w.symlink || p.IsDir != w.dir {
			t.Errorf("%s: is_symlink=%v is_dir=%v, want %v/%v", p.Name, p.IsSymlink, p.IsDir, w.symlink, w.dir)
		}
	}
}
