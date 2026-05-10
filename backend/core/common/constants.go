// Package common holds service-name + version constants and the
// message-bus event-type catalog the core service registers at
// startup. Lives here (vs. inline) so external code linking against
// the SDK can reference stable names.
package common

// Service identity. SERVICENAME is the message-bus SourceID — kept as
// "casaos" for backwards-compat with subscribers that already filter
// on it. VERSION is the legacy CasaOS API version returned by /v1/sys.
// RANW_NAME is the remote-access tunnel name used by the bundled
// IceWhale tunnel client.
const (
	SERVICENAME = "casaos"
	VERSION     = "0.4.15"
	BODY        = " "
	RANW_NAME   = "IceWhale-RemoteAccess"
)

// POWERLAB_VERSION is overridden at link time by the build pipeline:
//
//	go build -ldflags='-X github.com/neochaotic/powerlab/backend/core/common.POWERLAB_VERSION=0.2.6'
//
// Dev builds (`go build` without ldflags, ./start.sh, etc.) keep the
// "dev" sentinel so the UI shows "dev" in the version handshake rather
// than misleadingly stamping a tag-aligned version on uncommitted code.
//
// The /v1/powerlab/version handler returns this string verbatim. The
// UI compares it to its compiled-in __APP_VERSION__; mismatch surfaces
// a "UI cached — please reload" banner.
var POWERLAB_VERSION = "dev"
