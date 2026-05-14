package audit

import (
	"os"
	"path/filepath"
	"time"
)

// ServiceOptions bundles every audit option a host service needs.
// Pass an empty struct for defaults. Path is required.
type ServiceOptions struct {
	// Path to the audit SQLite file (e.g. /var/lib/powerlab/gateway/audit.db).
	// The parent directory is created with mode 0o750 if missing.
	Path string

	// Recorder tuning. Zero values get ADR-0033 defaults.
	Recorder RecorderOptions

	// Retention policy. Zero values get ADR-0033 defaults.
	Retention RetentionOptions
}

// Service bundles the DB, Recorder, and RetentionRunner so the
// host service can manage all three with a single Close().
// Created via NewService.
type Service struct {
	DB        *DB
	Recorder  *Recorder
	Retention *RetentionRunner
}

// NewService opens the DB, starts the Recorder writer goroutine,
// and starts the RetentionRunner. Caller MUST Close() to release
// the two goroutines + the DB handle.
//
// The parent directory of opts.Path is created with 0o750 if it
// doesn't exist — host services typically run as root and the
// audit DB shouldn't be world-readable (PII per ADR-0033).
func NewService(opts ServiceOptions) (*Service, error) {
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o750); err != nil {
		return nil, err
	}
	db, err := OpenDB(opts.Path)
	if err != nil {
		return nil, err
	}
	rec := NewRecorder(db, opts.Recorder)
	ret := NewRetentionRunner(db, opts.Retention)
	return &Service{DB: db, Recorder: rec, Retention: ret}, nil
}

// Close stops both goroutines (Recorder writer + Retention loop)
// and then closes the DB. Order matters: stop producers before
// closing the sink. Idempotent.
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	if s.Retention != nil {
		s.Retention.Close()
	}
	if s.Recorder != nil {
		// Give the writer a moment to flush before we drop the DB.
		s.Recorder.Close()
		// Drain any straggler — defensive; Close() already waits.
		time.Sleep(10 * time.Millisecond)
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}
