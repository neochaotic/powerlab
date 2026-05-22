package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetExt(t *testing.T) {
	cases := map[string]string{
		"a.txt":           ".txt",
		"archive.tar.gz":  ".gz",
		"noext":           "",
		"/path/to/x.JSON": ".JSON",
		".hidden":         ".hidden",
	}
	for in, want := range cases {
		if got := GetExt(in); got != want {
			t.Errorf("GetExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommonPrefix(t *testing.T) {
	cases := []struct {
		paths []string
		want  string
	}{
		{[]string{}, ""},
		{[]string{"/a/b/c"}, "/a/b/c"},
		{[]string{"/a/b/c", "/a/b/d"}, "/a/b"},
		{[]string{"/a/x", "/a/y", "/a/z"}, "/a"},
	}
	for _, c := range cases {
		if got := CommonPrefix(filepath.Separator, c.paths...); got != c.want {
			t.Errorf("CommonPrefix(%v) = %q, want %q", c.paths, got, c.want)
		}
	}
}

func TestExistsDirFileAndMkdir(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "nope")
	if !CheckNotExist(missing) {
		t.Error("CheckNotExist(missing) should be true")
	}
	if Exists(missing) {
		t.Error("Exists(missing) should be false")
	}

	sub := filepath.Join(dir, "sub", "deep")
	if err := MkDir(sub); err != nil {
		t.Fatalf("MkDir: %v", err)
	}
	if !Exists(sub) || !IsDir(sub) {
		t.Error("MkDir should create an existing directory")
	}
	if IsFile(sub) {
		t.Error("IsFile on a dir should be false")
	}
	// IsNotExistMkDir is a no-op when the dir already exists.
	if err := IsNotExistMkDir(sub); err != nil {
		t.Errorf("IsNotExistMkDir on existing dir: %v", err)
	}

	f := filepath.Join(dir, "file.txt")
	if err := CreateFileAndWriteContent(f, "hello"); err != nil {
		t.Fatalf("CreateFileAndWriteContent: %v", err)
	}
	if !IsFile(f) || IsDir(f) {
		t.Error("created file should be IsFile and not IsDir")
	}
	if got := ReadFullFile(f); string(got) != "hello" {
		t.Errorf("ReadFullFile = %q, want hello", got)
	}

	// CreateFile makes an empty file.
	empty := filepath.Join(dir, "empty")
	if err := CreateFile(empty); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if !IsFile(empty) {
		t.Error("CreateFile should produce a file")
	}

	// RMDir removes a whole tree (os.RemoveAll under the hood).
	if err := RMDir(filepath.Join(dir, "sub")); err != nil {
		t.Errorf("RMDir: %v", err)
	}
	if Exists(filepath.Join(dir, "sub")) {
		t.Error("RMDir should have removed the tree")
	}
}

func TestGetFileOrDirSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b"), []byte("678"), 0o644); err != nil {
		t.Fatal(err)
	}
	size, err := GetFileOrDirSize(dir)
	if err != nil {
		t.Fatalf("GetFileOrDirSize: %v", err)
	}
	if size < 8 {
		t.Errorf("GetFileOrDirSize = %d, want >=8", size)
	}
}
