// Package paths centralises every persistent-file path PowerLab
// services use. Services MUST NOT construct DB paths by hand — call
// the canonical/legacy helpers here so a future convention change
// happens in one place.
//
// Why this exists: see docs/audits/db-paths.md and issue #179. Two
// real prod failures during the v0.5.4 → v0.5.7 cycle traced back to
// services constructing paths from `dbPath + "/something.db"` strings
// scattered across packages, where each service made a different
// (silent) choice about the layout. The helpers here expose the
// canonical going-forward path AND the legacy path each service may
// still be reading from on existing installs, so:
//
//   1. New code uses Canonical*() helpers and never thinks about the
//      legacy path.
//   2. Migration code (install.sh, cmd/migration-tool/) uses the
//      Legacy*() helpers so it knows what to migrate FROM.
//   3. Boot-time AssertNoSplitBrain refuses to start when both
//      canonical and legacy exist — preventing silent data drift.
package paths

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/neochaotic/powerlab/backend/common/utils/constants"
)

// Logger is the minimal subset of pkg/logging.Logger this package
// needs. Defined locally as a duck-typed interface to avoid making
// the common module depend on pkg/logging — services pass their
// real logger and Go's structural typing handles the rest. nil is
// a valid value (skips the log line; the returned error already
// carries full context).
type Logger interface {
	Error(ctx context.Context, msg string, err error, attrs ...slog.Attr)
}

// dataRoot is constants.DefaultDataPath, indirected through a function
// so tests can override via a temp dir without poking package globals.
// Production callers always use the constants value.
var dataRoot = func() string { return constants.DefaultDataPath }

// Filename constants. The canonical filename each service uses for
// its persistent SQLite DB. Kept as constants so the migration
// script + integration tests can spell them once.
const (
	FilenameUserServiceDB  = "user.db"
	FilenameCoreDB         = "core.db"
	FilenameLocalStorageDB = "local-storage.db"
	FilenameMessageBusDB   = "message-bus.db"
)

// CanonicalUserServiceDB returns the going-forward path for
// user-service's SQLite database. Format: <DataPath>/user.db.
//
// On a fresh PowerLab install this is the only path that exists and
// gets used. On an upgraded install, see LegacyUserServiceDB.
func CanonicalUserServiceDB() string {
	return UserServiceDBIn(dataRoot())
}

// LegacyUserServiceDB returns the path the v0.5.4 install.sh hot-fix
// migration accidentally targeted (`/var/lib/powerlab/db/user.db`).
// User-service NEVER read from this path; the file existed only as a
// stale copy. Returned here so split-brain detection can flag it.
func LegacyUserServiceDB() string {
	return LegacyUserServiceDBIn(dataRoot())
}

// UserServiceDBIn is the canonical path with a caller-supplied base.
// Use this when the user-service `-db` flag or config has overridden
// the default DataPath.
func UserServiceDBIn(base string) string {
	return filepath.Join(base, FilenameUserServiceDB)
}

// LegacyUserServiceDBIn is the legacy path with a caller-supplied base.
func LegacyUserServiceDBIn(base string) string {
	return filepath.Join(base, "db", FilenameUserServiceDB)
}

// CanonicalCoreDB returns the going-forward path for core's SQLite
// database. Format: <DataPath>/core.db.
//
// Legacy code (as of v0.5.7) still uses
// `LegacyCoreDB() = <DataPath>/db/casaOS.db` AND
// `LegacyCasaOSCoreDB() = /var/lib/casaos/db/casaOS.db`. Both are
// migrated separately (see docs/audits/db-paths.md).
func CanonicalCoreDB() string {
	return filepath.Join(dataRoot(), "core.db")
}

// LegacyCoreDB is the path core has been writing to since the rebrand
// — uses /db/ subdir + the inherited casaOS.db filename. Migration
// to CanonicalCoreDB is tracked separately.
func LegacyCoreDB() string {
	return filepath.Join(dataRoot(), "db", "casaOS.db")
}

// LegacyCasaOSCoreDB is the pre-rebrand location that core may STILL
// be reading from on hosts where install.sh's skip-if-exists logic
// preserved the old DBPath conf value (the v0.5.7 / #179 finding).
func LegacyCasaOSCoreDB() string {
	return "/var/lib/casaos/db/casaOS.db"
}

// CanonicalLocalStorageDB returns the going-forward path for
// local-storage's SQLite database. Format: <DataPath>/local-storage.db.
//
// Already matches what local-storage's GetGlobalDB constructs in
// production code, so no migration needed for this service.
func CanonicalLocalStorageDB() string {
	return filepath.Join(dataRoot(), "local-storage.db")
}

// LegacyLocalStorageDB is the v0.5.4 hot-fix copy destination — never
// actually read by the service.
func LegacyLocalStorageDB() string {
	return filepath.Join(dataRoot(), "db", "local-storage.db")
}

// CanonicalMessageBusDB returns the going-forward path for
// message-bus's persistent SQLite database. Format:
// <DataPath>/message-bus.db.
//
// As of v0.5.7 the production code uses <DataPath>/db/message-bus.db
// (with /db/ subdir); migration to canonical is tracked separately.
func CanonicalMessageBusDB() string {
	return filepath.Join(dataRoot(), "message-bus.db")
}

// LegacyMessageBusDB is the path message-bus currently writes to.
func LegacyMessageBusDB() string {
	return filepath.Join(dataRoot(), "db", "message-bus.db")
}

// ErrSplitBrain signals that two paths the caller asked about both
// exist on disk with non-zero size — i.e. the system has the same
// logical data in two physical places. Services that hit this MUST
// refuse to start; silently picking one would risk persistent data
// drift.
var ErrSplitBrain = errors.New("database split-brain detected")

// AssertNoSplitBrain returns ErrSplitBrain (and logs full context) if
// MORE THAN ONE of the supplied paths exists on disk with non-zero
// size. Returns nil if zero or one paths exist.
//
// Typical usage at service startup:
//
//	if err := paths.AssertNoSplitBrain(ctx, _log,
//	    "user-service",
//	    paths.CanonicalUserServiceDB(),
//	    paths.LegacyUserServiceDB(),
//	); err != nil {
//	    panic(err)  // or log.Fatal — refuse to start
//	}
//
// The serviceName is included in the error message + log line so the
// operator knows which service is misconfigured.
//
// Empty paths are treated as "not configured" and skipped silently —
// callers can pass conditional alternates without first checking
// non-empty.
func AssertNoSplitBrain(ctx context.Context, log Logger, serviceName string, paths ...string) error {
	var present []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			continue // missing or unreadable → not present
		}
		if st.Size() == 0 {
			continue // empty file → SQLite hasn't initialised it yet → safe
		}
		present = append(present, p)
	}

	if len(present) <= 1 {
		return nil
	}

	msg := fmt.Sprintf(
		"split-brain: %s has the same data in %d places (%v) — refusing to start. "+
			"Pick the authoritative copy (usually the most recently modified), "+
			"move the others to <file>.bak.$(date +%%s), then restart. "+
			"See docs/audits/db-paths.md for the full recovery playbook.",
		serviceName, len(present), present)

	if log != nil {
		log.Error(ctx, msg, ErrSplitBrain)
	}
	return fmt.Errorf("%w: %s has %d copies: %v", ErrSplitBrain, serviceName, len(present), present)
}
