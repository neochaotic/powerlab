package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSecretsFile_MkdirAllFails(t *testing.T) {
	// The parent "directory" is a regular file, so MkdirAll fails with
	// ENOTDIR regardless of the caller's privileges.
	base := t.TempDir()
	blocker := filepath.Join(base, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := writeSecretsFile(filepath.Join(blocker, "app", "app.env"), map[string]string{"K": "v"})
	if err == nil {
		t.Fatal("expected an error when the secrets dir cannot be created")
	}
}

func TestWriteSecretsFile_CreateTempFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	path := filepath.Join(dir, "app.env")
	if err := writeSecretsFile(path, map[string]string{"K": "v"}); err == nil {
		t.Fatal("expected an error writing into a read-only dir")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("secrets file should not exist after a failed write, stat err = %v", err)
	}
}

func TestWriteSecretsFile_RenameFailsAndCleansUpTemp(t *testing.T) {
	// The target path is a non-empty directory, so the final rename
	// fails (for root too). The temp file must not be left behind.
	dir := t.TempDir()
	path := filepath.Join(dir, "app.env")
	if err := os.MkdirAll(filepath.Join(path, "occupied"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeSecretsFile(path, map[string]string{"K": "v"}); err == nil {
		t.Fatal("expected an error when the target path is a directory")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".secrets-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestWriteSecretsFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "app.env")
	in := map[string]string{"APP_SEED": "abc", "APP_PASSWORD": "xyz"}
	if err := writeSecretsFile(path, in); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("secrets file perm = %o, want 600", perm)
	}
	got, err := loadSecretsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(in) || got["APP_SEED"] != "abc" || got["APP_PASSWORD"] != "xyz" {
		t.Errorf("round trip = %v, want %v", got, in)
	}
}
