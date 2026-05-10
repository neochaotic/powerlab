package common

import (
	"path/filepath"
)

// Per-app data directory naming. Per ADR-0021 PowerLab moves from
// the shared <StoragePath>/AppData/<app> tree (which collides with
// CasaOS) to its own <StoragePath>/PowerLabAppData/<app> tree. New
// containers' compose volume binds use the canonical name; existing
// installs migrate via PowerLabAppDataMigrationPlan on first boot.
const (
	// AppDataDirName is the canonical per-product directory under
	// <StoragePath>. The "PowerLab" prefix prevents collision with
	// CasaOS's "AppData" tree on the same host.
	AppDataDirName = "PowerLabAppData"

	// LegacyAppDataDirName is what PowerLab used to write to. Read
	// for migration discovery; never written by new code.
	LegacyAppDataDirName = "AppData"
)

// PowerLabAppDataPath returns the canonical per-app data directory
// for a given app name. storagePath is the configured StoragePath
// (typically /DATA on Linux, a Docker-Desktop-accessible path on
// macOS dev installs).
//
// Example: PowerLabAppDataPath("/DATA", "nextcloud")
//   → "/DATA/PowerLabAppData/nextcloud"
func PowerLabAppDataPath(storagePath, appName string) string {
	return filepath.Join(storagePath, AppDataDirName, appName)
}

// LegacyAppDataPath returns the pre-ADR-0021 directory for a given
// app name. Used by the migration logic to know what to move FROM.
//
// Example: LegacyAppDataPath("/DATA", "nextcloud")
//   → "/DATA/AppData/nextcloud"
func LegacyAppDataPath(storagePath, appName string) string {
	return filepath.Join(storagePath, LegacyAppDataDirName, appName)
}
