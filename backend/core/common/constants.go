package common

const (
	SERVICENAME = "casaos"
	VERSION     = "0.4.15"
	BODY        = " "
	RANW_NAME   = "IceWhale-RemoteAccess"
)

// POWERLAB_VERSION is overridden at link time by the build pipeline:
//
//   go build -ldflags='-X github.com/neochaotic/powerlab/backend/core/common.POWERLAB_VERSION=0.2.6'
//
// Dev builds (`go build` without ldflags, ./start.sh, etc.) keep the
// "dev" sentinel so the UI shows "dev" in the version handshake rather
// than misleadingly stamping a tag-aligned version on uncommitted code.
//
// The /v1/powerlab/version handler returns this string verbatim. The
// UI compares it to its compiled-in __APP_VERSION__; mismatch surfaces
// a "UI cached — please reload" banner.
var POWERLAB_VERSION = "dev"
