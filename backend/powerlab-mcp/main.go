// Command powerlab-mcp is PowerLab's standalone observability + MCP
// service (ADR-0034). It runs independently of the rest of the stack —
// reading system metrics, journal logs, and the JSONL audit store
// directly — and exposes them to operators (browser) and AI agents
// (MCP) over HTTP+SSE on :9090.
//
// This is the Foundation skeleton: it boots, loads config, serves the
// control endpoints (/healthz, /version) and the MCP transport, and
// shuts down cleanly. Resources and tools are registered in follow-up
// changes.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/daemon"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/config"
	"github.com/neochaotic/powerlab/backend/powerlab-mcp/server"
)

// Build identity, injected at link time via -ldflags -X. Defaults make
// a `go run` build identifiable as un-stamped.
var (
	version = "private build"
	commit  = "private build"
	date    = "private build"
)

func main() {
	confPath := flag.String("conf", "/etc/powerlab/mcp.conf", "path to mcp.conf")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load(*confPath)
	if err != nil {
		// A malformed conf must not be fatal — Load already fell back to
		// defaults for the unreadable parts; log loudly and carry on so
		// the observability surface stays up exactly when something is
		// misconfigured.
		log.Warn("config load fell back to defaults", "path", *confPath, "err", err)
	}

	srv, err := server.New(cfg, server.BuildInfo{Version: version, Commit: commit, Date: date})
	if err != nil {
		log.Error("build server", "err", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("powerlab-mcp listening", "addr", cfg.ListenAddr, "version", version)
		// Tell systemd we're ready (no-op outside a notify unit).
		_, _ = daemon.SdNotify(false, daemon.SdNotifyReady)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
}
