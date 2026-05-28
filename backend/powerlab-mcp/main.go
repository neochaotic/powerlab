// Command powerlab-mcp is PowerLab's standalone observability + MCP
// service (ADR-0034). It runs independently of the rest of the stack —
// reading system metrics, journal logs, and the JSONL audit store
// directly — and exposes them to operators (browser) and AI agents
// (MCP) over HTTP+SSE on :9090.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	info := server.BuildInfo{Version: version, Commit: commit, Date: date}
	if err := run(ctx, cfg, info, log); err != nil {
		log.Error("powerlab-mcp exited with error", "err", err)
		os.Exit(1)
	}
}

// run binds the listener, serves until ctx is cancelled, then shuts down
// gracefully. It returns an error if the listener can't bind or the
// server fails mid-flight — so a bind failure (e.g. the port is already
// in use) surfaces as a non-zero exit, not the silent clean stop systemd
// would otherwise misread as a successful shutdown. Extracted from main
// so this lifecycle is testable.
//
// The cfg.Disabled kill-switch is checked here (not in main) so the
// existing run-tests cover it too. When set, run returns nil before
// binding — systemd records the start as successful and does NOT
// restart-loop (Restart=always retries non-zero only). The operator
// re-enables by flipping `Disabled` back in mcp.conf + restarting.
func run(ctx context.Context, cfg config.Config, info server.BuildInfo, log *slog.Logger) error {
	if cfg.Disabled {
		log.Info("powerlab-mcp is Disabled in mcp.conf — exiting without binding")
		return nil
	}

	srv, err := server.New(cfg, info)
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	// Bind synchronously so "address already in use" is caught here —
	// before we announce readiness to systemd or background the serve
	// loop.
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("powerlab-mcp listening", "addr", cfg.ListenAddr, "version", info.Version)
		// Ready only now that the listener is bound.
		_, _ = daemon.SdNotify(false, daemon.SdNotifyReady)
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down")
		_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serveErr:
		return err
	}
}
