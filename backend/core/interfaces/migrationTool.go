// Package interfaces holds cross-package contracts whose
// implementations live in cmd/migration-tool. Same shape as
// common.MigrationTool — duplicated so the core service stays
// independently buildable.
package interfaces

// MigrationTool is the per-version data-migration contract.
// Each migration tool in the chain implements all four methods;
// the install/setup driver runs them in order.
type MigrationTool interface {
	// IsMigrationNeeded returns true when this tool's data
	// transformation is still pending. Cheap to call.
	IsMigrationNeeded() (bool, error)
	// PostMigrate verifies + cleans up after Migrate. Errors
	// here block the chain.
	PostMigrate() error
	// Migrate performs the actual data rewrite. Idempotent.
	Migrate() error
	// PreMigrate prepares the data before Migrate (snapshots,
	// schema-version reads). Idempotent.
	PreMigrate() error
}
