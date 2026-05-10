// Package common holds service-name + event-type identifiers shared
// across the local-storage routes, services, and message-bus
// publishers. Lives here (vs. inline constants) so external code
// linking against the SDK can reference stable names.
package common

// Service-wide identity. Version is read by the /v1/sys/version
// endpoint; ServiceName is the message-bus SourceID; DefaultMountPoint
// is the merge-pool root path on a fresh install.
const (
	Version           = "0.4.4"
	ServiceName       = "local-storage"
	DefaultMountPoint = "/DATA"
)
