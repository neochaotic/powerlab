package model

import (
	"encoding/json"
	"testing"
)

// TestFileUpdate_JSONBinding pins the contract between the editor UI
// (ui/src/lib/api/files.ts → updateFileContent) and the PUT /v1/file
// handler. The frontend sends keys `file_path` and `file_content`. If
// the struct tags ever drift back to `path` / `content`, the binding
// silently zeroes both fields and PUT /v1/file returns "File already
// exists" on every save (because file.Exists("") is false at line 738
// of file.go).
func TestFileUpdate_JSONBinding(t *testing.T) {
	body := []byte(`{"file_path":"/etc/powerlab/app-management.conf","file_content":"hello"}`)

	var fu FileUpdate
	if err := json.Unmarshal(body, &fu); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if fu.FilePath != "/etc/powerlab/app-management.conf" {
		t.Errorf("FilePath: want %q, got %q", "/etc/powerlab/app-management.conf", fu.FilePath)
	}
	if fu.FileContent != "hello" {
		t.Errorf("FileContent: want %q, got %q", "hello", fu.FileContent)
	}
}

// TestFileUpdate_RejectsLegacyKeys ensures we don't accidentally
// support both old (`path`/`content`) and new (`file_path`/`file_content`)
// shapes — that would mask future drift. Old keys must NOT bind.
func TestFileUpdate_RejectsLegacyKeys(t *testing.T) {
	body := []byte(`{"path":"/tmp/x","content":"y"}`)

	var fu FileUpdate
	if err := json.Unmarshal(body, &fu); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fu.FilePath != "" || fu.FileContent != "" {
		t.Errorf("expected zero values for legacy keys, got FilePath=%q FileContent=%q", fu.FilePath, fu.FileContent)
	}
}
