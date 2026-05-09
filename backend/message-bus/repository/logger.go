package repository

import (
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the package-level logger. Defaults to a permissive
// INFO/json instance so tests work without setup; the service's
// main() overrides via SetLogger after foundation logger init.
var _log pkglogging.Logger = mustDefaultLogger()

// SetLogger overrides the package-level logger.
func SetLogger(l pkglogging.Logger) {
	if l != nil {
		_log = l
	}
}

func mustDefaultLogger() pkglogging.Logger {
	l, _ := pkglogging.New(pkglogging.Config{Level: "info", Format: "json"})
	return l
}
