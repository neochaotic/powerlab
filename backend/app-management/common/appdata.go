package common

// Per-app data directory naming. Per ADR-0021 PowerLab moves from
// the shared <StoragePath>/AppData/<app> tree (which collides with
// CasaOS) to its own <StoragePath>/PowerLabAppData/<app> tree. New
// containers' compose volume binds use the canonical name; existing
// installs migrate via PowerLabAppDataMigrationPlan on first boot.
//
// Only the constants are referenced (compose_service.go's rewrite
// loop assembles paths inline via string concatenation). The helper
// functions that wrapped filepath.Join were never adopted.
const (
	// AppDataDirName is the canonical per-product directory under
	// <StoragePath>. The "PowerLab" prefix prevents collision with
	// CasaOS's "AppData" tree on the same host.
	AppDataDirName = "PowerLabAppData"

	// LegacyAppDataDirName is what PowerLab used to write to. Read
	// for migration discovery; never written by new code.
	LegacyAppDataDirName = "AppData"
)
