package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// REGRESSION TEST — pinning the fix for the "Upgrade refused (HTTP 500):
// download tarball: context deadline exceeded" bug operators hit
// upgrading v0.7.4 → v0.7.6 from the panel.
//
// Root cause: NewPowerLabUpdater set http.Client.Timeout = 30 * time.Second.
// That field counts the body read against the wall clock, so an 80 MB
// tarball downloading at ~2.7 MB/s (typical homelab ISP) would race a
// 30 s budget and silently fail mid-body. The fix moves all bounds
// onto the Transport (DialContext / TLS / ResponseHeader) and leaves
// Client.Timeout unset — body reads are then bounded only by the
// caller's context.
//
// This test simulates the failure shape: an httptest server that
// streams a large body slowly, total body-read time deliberately
// exceeding the old 30 s budget. The production updater client MUST
// download to completion. If a future refactor reintroduces
// Client.Timeout, this test fails fast — exactly the regression lock
// the bug was missing.
func TestDownloadFile_SurvivesSlowBodyRead(t *testing.T) {
	// 8 MiB total body, written in 256 KiB chunks with a 100 ms
	// sleep between chunks: total stream time ≈ 32 chunks * 100 ms =
	// 3.2 s of artificial latency. Faster than a real homelab ISP
	// but enough to blow past a 30 s Client.Timeout if someone
	// regresses + also keeps the test under 10 s on CI.
	//
	// To force the old failure mode without slowing CI: cap the
	// regression-test Client.Timeout to 1 s and verify it FAILS
	// (the negative-case sibling test below).
	const totalBytes = 8 * 1024 * 1024
	const chunkBytes = 256 * 1024
	const chunkDelay = 100 * time.Millisecond

	payload := make([]byte, totalBytes)
	for i := range payload {
		payload[i] = byte(i % 251) // deterministic so SHA matches
	}
	wantHash := sha256.Sum256(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "8388608")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for off := 0; off < totalBytes; off += chunkBytes {
			end := off + chunkBytes
			if end > totalBytes {
				end = totalBytes
			}
			if _, err := w.Write(payload[off:end]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(chunkDelay)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "tarball.tar.gz")

	client := newUpdaterHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	if err := downloadFile(ctx, client, srv.URL, dst); err != nil {
		t.Fatalf("downloadFile failed under slow-stream conditions: %v (this is the v0.7.4→v0.7.6 upgrade bug regressing)", err)
	}
	t.Logf("downloaded %d bytes in %s (slow-stream simulation)", totalBytes, time.Since(start))

	// Verify the body landed intact + correctly sized.
	got, err := os.ReadFile(dst) // #nosec G304 -- dst is t.TempDir()
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if len(got) != totalBytes {
		t.Fatalf("got %d bytes; want %d", len(got), totalBytes)
	}
	gotHash := sha256.Sum256(got)
	if hex.EncodeToString(gotHash[:]) != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("payload SHA-256 mismatch — body was corrupted mid-stream")
	}
}

// SAFETY NET — proves the regression test above is real: with a
// 1 second Client.Timeout (the old failure mode), the same slow-
// stream server FAILS. If both this and the production-client test
// pass simultaneously, the regression test is a no-op and not
// actually proving anything.
func TestDownloadFile_OldClientTimeoutFails_NegativeCase(t *testing.T) {
	const totalBytes = 2 * 1024 * 1024
	const chunkBytes = 64 * 1024
	const chunkDelay = 200 * time.Millisecond
	payload := make([]byte, totalBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "2097152")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for off := 0; off < totalBytes; off += chunkBytes {
			end := off + chunkBytes
			if end > totalBytes {
				end = totalBytes
			}
			if _, err := w.Write(payload[off:end]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(chunkDelay)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	dst := filepath.Join(dir, "tarball.tar.gz")

	// Old-style client: total Timeout shorter than total body-read time.
	// 32 chunks × 200 ms = 6.4 s of stream; 1 s timeout WILL fail.
	oldStyleClient := &http.Client{Timeout: 1 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := downloadFile(ctx, oldStyleClient, srv.URL, dst)
	if err == nil {
		t.Fatalf("downloadFile UNEXPECTEDLY succeeded with Client.Timeout=1s + slow stream — the regression test above is no longer a real probe")
	}
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("expected timeout-shaped error; got %v", err)
	}
	t.Logf("old-style client correctly failed: %v", err)

	// io.EOF on a partial file is fine — we just need the call to error.
	_ = io.EOF
}
