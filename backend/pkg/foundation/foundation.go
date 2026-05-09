// Package foundation composes PowerLab's per-process HTTP middleware
// stack into a single Wrap helper, so every service applies an
// identical chain to its http.Server.Handler.
//
// Before this package, each service's main.go inlined the same
// composition:
//
//	pkgtracing.Middleware(pkglifecycle.RecoverMiddleware(log)(h))
//
// Four duplicates were a typo class waiting to happen: a service
// could silently lose panic recovery if a developer rewrote the
// chain incorrectly. With foundation.Wrap, there is a single source
// of truth, tested against the bug-#64 SIGSEGV behavior contract.
//
// See ADR-0011 (strangler — pkg/* foundation) and ADR-0016 (modular
// kill scope).
package foundation

import (
	"net/http"

	pkglifecycle "github.com/neochaotic/powerlab/backend/pkg/lifecycle"
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
	pkgtracing "github.com/neochaotic/powerlab/backend/pkg/tracing"
)

// Wrap returns h wrapped with the canonical PowerLab middleware chain:
//
//  1. tracing.Middleware (outermost) — reads or mints an X-Request-Id,
//     stores it in the request context for downstream handlers, and
//     echoes it back on the response.
//
//  2. lifecycle.RecoverMiddleware — catches panics in the handler
//     chain, logs the panic value + stack trace + correlation ID at
//     error level, and writes a 500 response via pkg/errors.WriteHTTP
//     so the body shape matches every other PowerLab error envelope.
//
// Apply Wrap to every http.Server.Handler in every PowerLab service.
// Composition order is fixed by this function — callers pass the inner
// handler and a logger, never the middleware constructors.
//
// If logger is nil, Wrap substitutes a permissive default (info/json
// to stdout) rather than constructing the chain with a nil reference
// that would nil-deref on the first request. This guards against the
// exact bug class the chain is supposed to fix: a misconfigured
// service refusing to start (good) or starting silently broken (bad)
// would both be regressions; quietly substituting a default and
// continuing is the lesser evil.
func Wrap(h http.Handler, logger pkglogging.Logger) http.Handler {
	if logger == nil {
		logger, _ = pkglogging.New(pkglogging.Config{Level: "info", Format: "json"})
	}
	return pkgtracing.Middleware(
		pkglifecycle.RecoverMiddleware(logger)(h),
	)
}
