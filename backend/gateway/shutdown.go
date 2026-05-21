package main

import (
	"context"
	"net/http"
	"time"
)

// gatewayShutdownTimeout bounds graceful shutdown of a gateway HTTP
// server. The gateway proxies long-lived connections (SSE: audit feed,
// journald log streams, telemetry) that never close on their own.
// http.Server.Shutdown blocks until every active connection drains, so
// shutting down with an unbounded context hangs until systemd SIGKILLs
// the process (~TimeoutStopSec) whenever a browser holds an open stream
// — which also stalls the Layer 4 self-restart path. A deadline lets
// Shutdown drain briefly, then return so the listener closes and the
// process exits (or the new gateway takes over) promptly.
const gatewayShutdownTimeout = 10 * time.Second

// shutdownGateway gracefully stops srv, bounded by timeout. On deadline
// expiry Shutdown returns context.DeadlineExceeded and any still-open
// connections are abandoned (the process is exiting / handing off anyway).
func shutdownGateway(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}
