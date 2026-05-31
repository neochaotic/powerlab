package audit

import (
	"os"
	"path/filepath"
	"time"
)

// ServiceOptions bundles every audit option a host service needs.
// Pass an empty struct for defaults. Path is required.
type ServiceOptions struct {
	// Path to the JSONL file (e.g. /var/log/powerlab/audit.jsonl).
	// The parent directory is created with mode 0o750 if missing.
	Path string

	// Store tuning. Zero values get ADR-0035 defaults.
	Store StoreOptions

	// Recorder tuning. Zero values get sensible defaults.
	Recorder RecorderOptions
}

// Service bundles the Store and Recorder so the host service can
// manage both with a single Close().
type Service struct {
	Store    *Store
	Recorder *Recorder
}

// NewService opens the JSONL file (O_APPEND for multi-writer safety
// — see ADR-0035 amendment in #632), starts the Recorder writer
// goroutine, and returns the bundle. Caller MUST Close() to release
// the goroutine + flush the file.
//
// The parent directory of opts.Path is created with 0o750 if it
// doesn't exist — host services typically run as root and the
// audit log shouldn't be world-readable (PII per ADR-0033).
func NewService(opts ServiceOptions) (*Service, error) {
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0o750); err != nil {
		return nil, err
	}
	// Make sure store options carry the path.
	if opts.Store.Path == "" {
		opts.Store.Path = opts.Path
	}
	store, err := NewStore(opts.Store)
	if err != nil {
		return nil, err
	}
	rec := NewRecorder(store, opts.Recorder)
	return &Service{Store: store, Recorder: rec}, nil
}

// Close stops the writer goroutine and flushes the JSONL file.
// Idempotent.
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	if s.Recorder != nil {
		s.Recorder.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if s.Store != nil {
		return s.Store.Close()
	}
	return nil
}
