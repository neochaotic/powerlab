// Package common holds service-name + version constants and the
// message-bus event-type catalog the core service registers at
// startup. Lives here (vs. inline) so external code linking against
// the SDK can reference stable names.
package common

// Service identity. SERVICENAME is the message-bus SourceID core
// publishes events with. Renamed from "casaos" → "powerlab" in
// Sprint 9 PR F (#251). UI consumers filter by event Name, not
// SourceID, so the rename is invisible to clients.
//
// VERSION is the legacy CasaOS API version returned by /v1/sys.
// Kept frozen for back-compat with any external client still
// reading the version probe.
//
// RANW_NAME ("IceWhale-RemoteAccess") was removed in Sprint 9 PR F —
// it had zero callers and was a CasaOS-era remote-access tunnel
// identifier we never adopted.
const (
	SERVICENAME = "powerlab"
	VERSION     = "0.4.15"
	BODY        = " "
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
