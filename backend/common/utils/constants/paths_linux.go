//go:build linux

package constants

// Linux production paths — Filesystem Hierarchy Standard locations a
// daemon should use, matching what scripts/package-linux.sh::install.sh
// writes to disk. After setting them we call maybeApplyDevSandbox()
// which pivots into the project tree if no production marker is found.
func init() {
	DefaultConfigPath = "/etc/powerlab"
	DefaultConstantPath = "/usr/share/powerlab"
	DefaultDataPath = "/var/lib/powerlab"
	DefaultFilePath = "/var/lib/powerlab/files"
	DefaultLogPath = "/var/log/powerlab"
	DefaultRuntimePath = "/var/run/powerlab"
	DefaultWWWPath = "/usr/share/powerlab/www"
	maybeApplyDevSandbox()
}
