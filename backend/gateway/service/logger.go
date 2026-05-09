package service

import (
	pkglogging "github.com/neochaotic/powerlab/backend/pkg/logging"
)

// _log is the package-level logger. Defaults to a permissive
// INFO/json instance so tests work without setup; main() overrides
// via SetLogger after constructing the foundation logger so all log
// lines flow through one instance.
//
// Sprint 1 gateway kill series, ADR-0016. A future refactor can move
// to constructor DI; package-level is enough to remove the
// CasaOS-Common dependency now.
var _log pkglogging.Logger = mustDefaultLogger()

// SetLogger overrides the package-level logger. Call from main()
// after constructing the foundation logger.
func SetLogger(l pkglogging.Logger) {
	if l != nil {
		_log = l
	}
}

func mustDefaultLogger() pkglogging.Logger {
	l, _ := pkglogging.New(pkglogging.Config{Level: "info", Format: "json"})
	return l
}
