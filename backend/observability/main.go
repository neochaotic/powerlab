// Command powerlab-observability is the standalone observability +
// MCP service. Runs INDEPENDENTLY of every other PowerLab service
// so operators can debug the box even when the gateway / app-
// management / etc. are down.
//
// Skeleton only — actual handlers fill in across Sprint 17 slices
// per ADR-0034. See routes.go for the not-implemented stubs.
//
// Usage:
//
//	powerlab-observability \
//	  --addr :9090                 (default — Prometheus convention)
//	  --shutdown-grace 5s          (default)
//
// Exit codes:
//
//	0  — clean shutdown after SIGTERM/SIGINT
//	1  — listener bind failed (port already in use, perms, etc.)
//	2  — invalid args
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Build-time stamps. Set via `-ldflags "-X main.commit=… -X main.date=…"`
// in scripts/package-linux.sh — same pattern as the other services.
var (
	commit = "private build"
	date   = "private build"
)

func main() {
	addr := flag.String("addr", ":9090", "HTTP listen address (Prometheus convention)")
	grace := flag.Duration("shutdown-grace", 5*time.Second, "graceful shutdown timeout")
	versionFlag := flag.Bool("v", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("powerlab-observability %s (%s)\n", commit, date)
		os.Exit(0)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	log.Info("powerlab-observability starting", "addr", *addr, "commit", commit, "build_date", date)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second, // mitigate Slowloris (gosec G112)
	}

	// Run the listener in a goroutine so the main thread can wait
	// for SIGTERM and orchestrate graceful shutdown.
	listenErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	// Wait for either a listen error or a termination signal.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-listenErr:
		log.Error("listener failed", "err", err)
		os.Exit(1)
	case sig := <-stop:
		log.Info("shutdown requested", "signal", sig.String())
	}

	// Graceful shutdown: stop accepting new connections, drain
	// in-flight requests up to the grace window. Anything still
	// running gets killed by the deferred cancel().
	ctx, cancel := context.WithTimeout(context.Background(), *grace)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	log.Info("powerlab-observability stopped")
}
