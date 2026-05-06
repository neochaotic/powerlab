// Package devmode answers "is this binary running from a developer
// checkout, or is it installed on a production host?".
//
// Production hosts always have one of /etc/powerlab or /etc/casaos
// (created by scripts/package-linux.sh::install.sh). When neither
// exists, we assume the binary is being run from a `go run` /
// `./start.sh` developer flow and we redirect runtime/log paths into
// the project tree so multiple services can share a sandbox without
// touching system directories.
//
// Every service that wants to fall back to local sandbox paths in
// development must gate that override behind devmode.IsDev(). Doing
// it unconditionally — as the original CasaOS fork did — silently
// breaks production installs because os.Getwd() returns "/" under
// systemd, and the binaries end up writing routes.json,
// PID files and address files to /runtime instead of the configured
// /var/run/<svc> path.
package devmode

import "os"

// productionMarkers are paths that, if any of them exist, mean the
// binary was started by the production install (systemd unit pointing
// at /etc/<svc>/<svc>.conf). Order matters only for short-circuit
// speed; both are equally authoritative.
var productionMarkers = []string{
	"/etc/powerlab",
	"/etc/casaos",
}

// IsDev reports whether the current process is running in a developer
// sandbox (no production install detected).
func IsDev() bool {
	for _, marker := range productionMarkers {
		if _, err := os.Stat(marker); err == nil {
			return false
		}
	}
	return true
}
