package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/config"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/server"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// A listener that can't bind (port already in use) must make run return
// an error so main exits non-zero — otherwise systemd reads the failed
// start as a clean stop and never reports or restarts it.
func TestRun_BindFailureReturnsError(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy a port: %v", err)
	}
	defer func() { _ = busy.Close() }()

	cfg := config.Default()
	cfg.ListenAddr = busy.Addr().String() // already taken

	err = run(context.Background(), cfg, server.BuildInfo{Version: "test"}, quietLogger())
	if err == nil {
		t.Fatal("run on an occupied port returned nil; want a bind error so the process exits non-zero")
	}
}

// A normal lifecycle — bind, serve, then the context is cancelled (the
// SIGTERM path) — must shut down cleanly and return nil.
func TestRun_GracefulShutdownReturnsNil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := config.Default()
	cfg.ListenAddr = "127.0.0.1:0" // ephemeral free port

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, cfg, server.BuildInfo{Version: "test"}, quietLogger())
	}()

	cancel() // simulate SIGINT/SIGTERM

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown returned %v; want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after the context was cancelled")
	}
}
