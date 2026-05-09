package route

import (
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the package-level logger used by every handler in this
// package. Defaults to a permissive INFO/json logger so tests can
// import this package without ceremony; main() overrides via
// SetLogger after constructing the foundation logger so all log
// lines flow through the same instance.
//
// Same pattern as gateway/route/logger.go. ADR-0011 (strangler).
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
