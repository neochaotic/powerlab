package audit

import (
	"context"
	"time"
)

// RetentionOptions configures the periodic prune job that keeps the
// audit DB bounded per ADR-0033:
//
//   - MaxAge      — rows older than now-MaxAge are dropped. Default 30d.
//   - MaxRows     — when total exceeds this, oldest dropped until at
//                   or below MaxRows. Default 0 (disabled). Row count
//                   is the coarse approximation; byte-cap is a
//                   separate follow-up.
//   - Interval    — how often the prune runs. Default 1 hour.
//   - SkipWALCheckpoint — when true, skips the
//                   `PRAGMA wal_checkpoint(TRUNCATE)` after each
//                   prune. Default false (checkpoint runs). Tests
//                   that inspect WAL state set this true.
type RetentionOptions struct {
	MaxAge            time.Duration
	MaxRows           int64
	Interval          time.Duration
	SkipWALCheckpoint bool
}

// RetentionRunner owns the periodic prune goroutine. Created via
// NewRetentionRunner, stopped via Close. RunOnce is exposed for
// tests and for an eventual operator action.
type RetentionRunner struct {
	db   *DB
	opts RetentionOptions
	stop chan struct{}
	done chan struct{}
}

// defaults wires zero-valued options to their ADR-0033 defaults.
func (o *RetentionOptions) defaults() {
	if o.MaxAge <= 0 {
		o.MaxAge = 30 * 24 * time.Hour
	}
	if o.Interval <= 0 {
		o.Interval = time.Hour
	}
}

// NewRetentionRunner constructs the runner and starts the goroutine.
// Caller MUST eventually Close() to release the goroutine.
func NewRetentionRunner(db *DB, opts RetentionOptions) *RetentionRunner {
	opts.defaults()
	r := &RetentionRunner{
		db:   db,
		opts: opts,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go r.loop()
	return r
}

// RunOnce executes one prune cycle synchronously: age-based prune,
// then row-cap prune (if MaxRows > 0), then WAL checkpoint (unless
// SkipWALCheckpoint). Returns counts pruned per phase. Errors from
// any stage are returned but do not stop subsequent stages — a
// failing checkpoint must not mask a successful prune.
func (r *RetentionRunner) RunOnce(ctx context.Context) (agePruned, rowsPruned int64, err error) {
	cutoff := time.Now().Add(-r.opts.MaxAge)
	agePruned, err = r.db.PruneByAge(ctx, cutoff)

	if r.opts.MaxRows > 0 {
		var rowsErr error
		rowsPruned, rowsErr = r.db.PruneToMaxRows(ctx, r.opts.MaxRows)
		if rowsErr != nil && err == nil {
			err = rowsErr
		}
	}

	if !r.opts.SkipWALCheckpoint && (agePruned > 0 || rowsPruned > 0) {
		// Best-effort: if the checkpoint can't grab the write lock
		// right now, the next cycle will try again. The data is
		// already pruned at this point.
		_, _ = r.db.sql.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	}

	return agePruned, rowsPruned, err
}

// Close stops the loop and waits for the current cycle (if any) to
// finish. Idempotent.
func (r *RetentionRunner) Close() {
	select {
	case <-r.stop:
		return
	default:
		close(r.stop)
	}
	<-r.done
}

// loop is the periodic worker. Sleeps Interval between cycles; runs
// RunOnce per tick. Exits on stop. A failed RunOnce is swallowed so
// transient SQLite contention doesn't kill the goroutine.
func (r *RetentionRunner) loop() {
	defer close(r.done)
	t := time.NewTicker(r.opts.Interval)
	defer t.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			_, _, _ = r.RunOnce(ctx)
			cancel()
		}
	}
}
