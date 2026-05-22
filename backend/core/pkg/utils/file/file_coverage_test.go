package file

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestGetExt(t *testing.T) {
	cases := map[string]string{
		"a.txt":        ".txt",
		"archive.tar.gz": ".gz",
		"noext":        "",
		"/path/to/x.JSON": ".JSON",
		".hidden":      ".hidden",
	}
	for in, want := range cases {
		if got := GetExt(in); got != want {
			t.Errorf("GetExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetBlockInfo(t *testing.T) {
	// fileSize → expected blockSize; length = ceil(size/blockSize).
	cases := []struct {
		size      int64
		blockSize int
	}{
		{1 << 10, 1 << 17},        // tiny → 128kb
		{1 << 28, 1 << 17},        // 256M boundary
		{(1 << 28) + 1, 1 << 18},  // >256M → 256kb
		{1 << 33, 1 << 22},        // 8G → 4mb
		{1 << 40, 1 << 24},        // huge → 16mb
	}
	for _, c := range cases {
		bs, length := GetBlockInfo(c.size)
		if bs != c.blockSize {
			t.Errorf("GetBlockInfo(%d) blockSize = %d, want %d", c.size, bs, c.blockSize)
		}
		if length < 1 {
			t.Errorf("GetBlockInfo(%d) length = %d, want >=1", c.size, length)
		}
	}
}

func TestHashHelpers(t *testing.T) {
	data := []byte("powerlab")
	sum := md5.Sum(data)
	want := hex.EncodeToString(sum[:])

	if got := GetHashByContent(data); got != want {
		t.Errorf("GetHashByContent = %q, want %q", got, want)
	}
	if !ComparisonHash(data, want) {
		t.Error("ComparisonHash should match the correct hash")
	}
	if ComparisonHash(data, "deadbeef") {
		t.Error("ComparisonHash should reject a wrong hash")
	}

	// GetHashByPath on a temp file must equal the content hash.
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := GetHashByPath(p); got != want {
		t.Errorf("GetHashByPath = %q, want %q", got, want)
	}
	if got := GetHashByPath(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("GetHashByPath(missing) = %q, want empty", got)
	}
}

func TestPrefixAndDataLength(t *testing.T) {
	// Zero-padded fixed-width encodings: PrefixLength → 6 bytes, DataLength → 8.
	if got := string(PrefixLength(42)); got != "000042" {
		t.Errorf("PrefixLength(42) = %q, want 000042", got)
	}
	if got := string(PrefixLength(0)); got != "000000" {
		t.Errorf("PrefixLength(0) = %q, want 000000", got)
	}
	if got := string(DataLength(12345)); got != "00012345" {
		t.Errorf("DataLength(12345) = %q, want 00012345", got)
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

	// CheckNotExist / Exists on a missing path.
	missing := filepath.Join(dir, "nope")
	if !CheckNotExist(missing) {
		t.Error("CheckNotExist(missing) should be true")
	}
	if Exists(missing) {
		t.Error("Exists(missing) should be false")
	}

	// MkDir creates it.
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

	// A regular file.
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

	// RMDir removes a whole tree (os.RemoveAll under the hood).
	if err := RMDir(filepath.Join(dir, "sub")); err != nil {
		t.Errorf("RMDir: %v", err)
	}
	if Exists(filepath.Join(dir, "sub")) {
		t.Error("RMDir should have removed the tree")
	}

	// RemoveAll only handles a flat directory of files (it does not recurse
	// into subdirectories — see #533). Exercise its working path here.
	flat := filepath.Join(dir, "flat")
	if err := MkDir(flat); err != nil {
		t.Fatal(err)
	}
	if err := CreateFileAndWriteContent(filepath.Join(flat, "x"), "x"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAll(flat); err != nil {
		t.Errorf("RemoveAll(flat dir of files): %v", err)
	}
	if Exists(flat) {
		t.Error("RemoveAll should have removed the flat dir")
	}
}

func TestDirSize(t *testing.T) {
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
		t.Errorf("GetFileOrDirSize = %d, want >=8 (sum of file bytes)", size)
	}
}
