package audit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/audit"
)

// TestService_NewService_CreatesParentDir — the constructor
// creates the parent dir if missing; otherwise OpenDB fails with
// "no such file or directory". Real install paths (e.g.
// /var/lib/powerlab/gateway/audit.db) typically don't exist
// before first run, so this is on the happy path.
func TestService_NewService_CreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "subdir")
	path := filepath.Join(dir, "audit.db")

	svc, err := audit.NewService(audit.ServiceOptions{Path: path})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	// The parent dir should now exist.
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("parent dir not created: %v", err)
	}
}

// TestService_RecorderDrainsThroughBundle — confirms Submit() →
// writer goroutine → DB still works when called via the bundle.
func TestService_RecorderDrainsThroughBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	svc, err := audit.NewService(audit.ServiceOptions{
		Path: path,
		Recorder: audit.RecorderOptions{
			Capacity:   64,
			BatchSize:  1,
			MaxLatency: 10 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	svc.Recorder.Submit(sampleRecord(time.Now(), 1))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		n, _ := svc.DB.Count(context.Background())
		if n == 1 {
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	n, _ := svc.DB.Count(context.Background())
	t.Errorf("Submit did not drain via bundle: count = %d", n)
}

// TestService_Close_Idempotent — multi-Close must not panic /
// hang. Mirrors the contract of Recorder/RetentionRunner.
func TestService_Close_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	svc, err := audit.NewService(audit.ServiceOptions{Path: path})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = svc.Close()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second Close hung")
	}
}

// TestService_NilSafe — Close() on a nil pointer must not panic.
// Defensive: lets host services that fail mid-init in lifecycle
// hooks safely call Close in a defer.
func TestService_NilSafe(t *testing.T) {
	var svc *audit.Service
	if err := svc.Close(); err != nil {
		t.Errorf("nil Close should be no-op, got %v", err)
	}
}
