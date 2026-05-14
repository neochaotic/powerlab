package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// install subcommand tests. Reads /var/log/powerlab/install-*.log
// files; default behaviour is "print the most recent one". --list
// flag enumerates available files with timestamps.

func TestRunInstall_TailsNewestFile(t *testing.T) {
	dir := t.TempDir()
	// Older file
	old := filepath.Join(dir, "install-20260101T100000Z.log")
	if err := os.WriteFile(old, []byte("old install line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Newer file
	newest := filepath.Join(dir, "install-20260513T200000Z.log")
	if err := os.WriteFile(newest, []byte("new install line\nphase 1/3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure mtime ordering matches name ordering.
	pastTime := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(old, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runInstall(dir, out, InstallOpts{}); err != nil {
		t.Fatalf("runInstall: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "new install line") {
		t.Errorf("expected newest file content, got: %q", text)
	}
	if strings.Contains(text, "old install line") {
		t.Errorf("default mode should NOT print older files, got: %q", text)
	}
}

func TestRunInstall_ListMode_ShowsAllFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"install-20260101T100000Z.log", "install-20260513T200000Z.log"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := &bytes.Buffer{}
	if err := runInstall(dir, out, InstallOpts{List: true}); err != nil {
		t.Fatalf("runInstall list: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "20260101T100000Z") || !strings.Contains(text, "20260513T200000Z") {
		t.Errorf("list mode should mention both filenames, got: %q", text)
	}
}

func TestRunInstall_NoFiles_ListMode(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	if err := runInstall(dir, out, InstallOpts{List: true}); err != nil {
		t.Fatalf("runInstall: %v", err)
	}
	// Empty list — output may be empty or a friendly message; the
	// behaviour locked here is "no error, no panic". Empty out is
	// fine; the operator sees nothing because nothing exists.
}

func TestRunInstall_NoFiles_DefaultMode_PrintsHelpfulMessage(t *testing.T) {
	dir := t.TempDir()
	out := &bytes.Buffer{}
	if err := runInstall(dir, out, InstallOpts{}); err != nil {
		t.Fatalf("runInstall: %v", err)
	}
	// When no install log files exist, default mode prints a
	// short note so the operator knows there's nothing to show
	// (vs. silently exiting which looks like a bug).
	text := strings.ToLower(out.String())
	if !strings.Contains(text, "no install") && !strings.Contains(text, "not found") &&
		!strings.Contains(text, "no log") {
		t.Errorf("expected helpful 'no install logs' message, got: %q", out.String())
	}
}

func TestRunInstall_MissingDirectory_NotAnError(t *testing.T) {
	// On a fresh box the /var/log/powerlab dir might not exist yet
	// — pre-first-install. Treat as "no files".
	out := &bytes.Buffer{}
	if err := runInstall(filepath.Join(t.TempDir(), "does-not-exist"), out, InstallOpts{}); err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
}

func TestRunInstall_IgnoresNonInstallFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "random.log"), []byte("ignore me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "install-real.log"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	if err := runInstall(dir, out, InstallOpts{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "ignore me") {
		t.Errorf("should NOT include non-install-*.log files, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "keep me") {
		t.Errorf("should include install-*.log files, got: %q", out.String())
	}
}
