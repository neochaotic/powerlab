package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestShutdownGateway_BoundedByTimeoutWithOpenStream locks the fix for
// the gateway shutdown hang: a long-lived connection (SSE — what the
// gateway proxies for the audit feed, journald logs, telemetry) must
// NOT make shutdown block indefinitely. With the pre-fix
// Shutdown(context.Background()) this hangs until the process is killed
// (~TimeoutStopSec) and also stalls the Layer 4 self-restart. The
// bounded context makes Shutdown return promptly even with the stream
// still open.
func TestShutdownGateway_BoundedByTimeoutWithOpenStream(t *testing.T) {
	handlerStarted := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		// Simulate an SSE/long-lived handler: flush headers, then hold
		// the connection open until the test releases it.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(handlerStarted)
		<-release
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	// Open a client connection that keeps the stream open.
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/stream", ln.Addr().String()))
		if err == nil {
			defer resp.Body.Close()
			_, _ = resp.Body.Read(make([]byte, 1)) // park reading the open stream
		}
	}()

	select {
	case <-handlerStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never started — test setup failed")
	}

	// The invariant: shutdown returns within ~the timeout even though a
	// connection is still open. A short timeout keeps the test fast.
	const shutdownTimeout = 300 * time.Millisecond
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- shutdownGateway(srv, shutdownTimeout) }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		// Shutdown should hit the deadline (open conn never drains) and
		// return DeadlineExceeded — promptly, not hang.
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded (open stream can't drain), got: %v", err)
		}
		if elapsed > 2*time.Second {
			t.Fatalf("shutdown took %v — bound not respected (hang regression)", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdownGateway HANG: did not return within 3s with an open stream — the bug regressed")
	}
}
