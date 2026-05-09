// Package lifecycle provides graceful shutdown coordination and panic
// recovery for PowerLab services.
//
// Three primitives:
//
//   - Manager — registers shutdown hooks and runs them in LIFO order
//     when SIGTERM/SIGINT arrive (or when Shutdown is called directly).
//   - RecoverMiddleware — HTTP middleware that catches panics in the
//     handler chain, logs them with stack trace, writes a 500 via
//     pkg/errors.WriteHTTP. Process keeps running.
//   - SafeGo — goroutine helper that recovers from panics inside the
//     goroutine, logs them, and lets the goroutine exit cleanly.
//
// Example:
//
//	logger := must(logging.New(logging.Config{Level: "info"}))
//	m := lifecycle.New(logger)
//	m.RegisterShutdown("http-server", server.Shutdown)
//	m.RegisterShutdown("db-pool", db.Close)
//
//	handler := lifecycle.RecoverMiddleware(logger)(mux)
//	go server.Serve(listener)
//
//	if err := m.Run(ctx, 30*time.Second); err != nil {
//	    logger.Error(ctx, "shutdown error", err)
//	}
//
// See ADR-0014 for the rationale behind the design.
package lifecycle
