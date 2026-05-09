package v2

import (
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the package-level logger used by every method in this
// package. Same pattern as gateway/route/logger.go and
// user-service/service/logger.go. ADR-0011.
var _log pkglogging.Logger = mustDefaultLogger()

// SetLogger overrides the package-level logger. Call from main()
// after constructing the foundation logger so that this package's
// log lines flow through the same handler as everything else.
func SetLogger(l pkglogging.Logger) {
	if l != nil {
		_log = l
	}
}

func mustDefaultLogger() pkglogging.Logger {
	l, _ := pkglogging.New(pkglogging.Config{Level: "info", Format: "json"})
	return l
}
