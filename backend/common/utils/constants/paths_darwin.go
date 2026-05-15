//go:build darwin

package constants

// macOS production paths.
//
// We do not follow the `~/Library/Application Support/...` convention
// because PowerLab on macOS runs as a system-wide LaunchDaemon (not
// per-user) and writing system-wide files into a user's Library mixes
// scopes badly. `/opt/powerlab/...` is widely accepted by macOS
// admins for self-contained third-party services and is the same place
// brew puts its formulae prefixes (`/opt/homebrew/...`).
//
// Logs land under `/Library/Logs/PowerLab/` so they show up in
// Console.app the same way Apple's own daemons do.
//
// local-storage is not shipped on macOS — it depends on Linux fuse +
// xattr that have no portable Darwin equivalent. The other five
// services (gateway, message-bus, user-service, core, app-management)
// install under `/opt/powerlab/bin/`.
func init() {
	DefaultConfigPath = "/opt/powerlab/etc"
	DefaultConstantPath = "/opt/powerlab/share"
	DefaultDataPath = "/opt/powerlab/lib"
	DefaultFilePath = "/opt/powerlab/lib/files"
	DefaultLogPath = "/Library/Logs/PowerLab"
	DefaultRuntimePath = "/opt/powerlab/run"
	DefaultWWWPath = "/opt/powerlab/share/www"
	maybeApplyDevSandbox()
}
