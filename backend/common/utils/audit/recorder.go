package audit

import (
	"context"
	"sync/atomic"
	"time"
)

// RecorderOptions configures the async writer. Defaults are tuned
// for the audit use case per ADR-0033:
//   - Capacity 1024 — channel slots; large enough that a bursty
//     install (many requests in 100ms) doesn't drop, small enough
//     that a stuck writer doesn't eat unbounded memory.
//   - BatchSize 50 — rows per transaction; amortises commit cost
//     without holding the tx open long enough to block readers.
//   - MaxLatency 200ms — max time to wait for the channel to fill
//     before flushing a partial batch; bounds tail latency for
//     "single request, then quiet" scenarios.
//
// Zero values get filled in by NewRecorder so callers can pass
// RecorderOptions{} for defaults.
type RecorderOptions struct {
	Capacity   int
	BatchSize  int
	MaxLatency time.Duration
}

// Recorder is the async write path. Middleware calls Submit() on the
// hot path; a background goroutine drains the channel into batched
// transactions. Channel-full → drop oldest with a counter (never
// block the request).
type Recorder struct {
	db   *DB
	ch   chan Record
	stop chan struct{}
	done chan struct{}

	// dropped is incremented atomically when Submit can't enqueue.
	dropped atomic.Int64

	opts RecorderOptions
}

// NewRecorder starts the writer goroutine. The returned Recorder
// must be Close()d on shutdown; pending records flush before the
// goroutine exits.
func NewRecorder(db *DB, opts RecorderOptions) *Recorder {
	if opts.Capacity <= 0 {
		opts.Capacity = 1024
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 50
	}
	if opts.MaxLatency <= 0 {
		opts.MaxLatency = 200 * time.Millisecond
	}
	r := &Recorder{
		db:   db,
		ch:   make(chan Record, opts.Capacity),
		stop: make(chan struct{}),
		done: make(chan struct{}),
		opts: opts,
	}
	go r.run()
	return r
}

// Submit enqueues a record for asynchronous write. Non-blocking:
// if the channel is full, the oldest queued record is dropped to
// make room and dropped counter is incremented. The hot path NEVER
// blocks — recording an audit row must not stall the response.
func (r *Recorder) Submit(rec Record) {
	select {
	case r.ch <- rec:
		// fast path — slot available
	default:
		// channel full. Drop the oldest by draining one slot, then
		// retry the send. The drain may race with the writer
		// goroutine pulling its own value; either way exactly one
		// record is lost and the new one is in.
		select {
		case <-r.ch:
			r.dropped.Add(1)
		default:
			// writer drained between the full-detect and our drain;
			// retry the original send (now has a slot).
		}
		select {
		case r.ch <- rec:
		default:
			// still full (rare — writer is way behind). Drop the
			// new record; we already counted one drop above.
			r.dropped.Add(1)
		}
	}
}

// Dropped returns the cumulative count of records dropped due to
// backpressure since the Recorder was created. Surfaced in the
// /v1/audit/stats endpoint so operators see when the audit pipe is
// falling behind.
func (r *Recorder) Dropped() int64 {
	return r.dropped.Load()
}

// Close stops the writer goroutine after flushing pending records.
// Idempotent: safe to call multiple times.
func (r *Recorder) Close() {
	select {
	case <-r.stop:
		// already closing
		return
	default:
		close(r.stop)
	}
	<-r.done
}

// run is the writer goroutine. Pulls from the channel, batches up
// to opts.BatchSize records or opts.MaxLatency, then flushes via
// db.InsertBatch. Exits when stop is closed and the channel has
// drained.
func (r *Recorder) run() {
	defer close(r.done)

	batch := make([]Record, 0, r.opts.BatchSize)
	timer := time.NewTimer(r.opts.MaxLatency)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		// Use a fresh context per flush so a long-running shutdown
		// doesn't block forever on a stuck DB call.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.db.InsertBatch(ctx, batch)
		cancel()
		batch = batch[:0]
		// Reset timer for the next batch window.
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(r.opts.MaxLatency)
	}

	for {
		select {
		case rec, ok := <-r.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, rec)
			if len(batch) >= r.opts.BatchSize {
				flush()
			}
		case <-timer.C:
			flush()
		case <-r.stop:
			// Drain whatever's still queued, then exit.
			for {
				select {
				case rec := <-r.ch:
					batch = append(batch, rec)
					if len(batch) >= r.opts.BatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}
