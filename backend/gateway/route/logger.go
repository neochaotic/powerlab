package route

import (
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the package-level logger used by every handler in this
// package. It defaults to a permissive INFO/json logger so tests can
// import this package without ceremony; main() overrides via
// SetLogger after constructing the foundation logger so all log
// lines flow through the same instance.
//
// This is part of the Sprint 1 gateway kill series (ADR-0016 — modular
// kill scope). A future refactor can move this to constructor DI; for
// now package-level is enough to remove the CasaOS-Common dependency
// without invasive constructor signature changes across the gateway.
var _log pkglogging.Logger = mustDefaultLogger()

// SetLogger overrides the package-level logger. Call from main() after
// constructing the foundation logger.
func SetLogger(l pkglogging.Logger) {
	if l != nil {
		_log = l
	}
}

func mustDefaultLogger() pkglogging.Logger {
	l, _ := pkglogging.New(pkglogging.Config{Level: "info", Format: "json"})
	return l
}
