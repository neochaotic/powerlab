package audit

import (
	"context"
	"sync/atomic"
	"time"
)

// RecorderOptions configures the async writer.
//
//   - Capacity 1024 — channel slots; large enough that a bursty
//     install (many requests in 100ms) doesn't drop, small enough
//     that a stuck writer doesn't eat unbounded memory.
//   - BatchSize 50 — records per flush; amortises file write cost.
//   - MaxLatency 200ms — max time to wait for the channel to fill
//     before flushing a partial batch; bounds tail latency.
//
// Zero values get filled in by NewRecorder so callers can pass
// RecorderOptions{} for defaults.
type RecorderOptions struct {
	Capacity   int
	BatchSize  int
	MaxLatency time.Duration
}

// Recorder is the async write path. Middleware calls Submit() on the
// hot path; a background goroutine drains the channel into batches
// and writes them to the Store (JSONL file + ring buffer). Channel-
// full → drop oldest with a counter (never block the request).
type Recorder struct {
	store *Store
	ch    chan Record
	stop  chan struct{}
	done  chan struct{}

	dropped atomic.Int64

	opts RecorderOptions
}

// NewRecorder starts the writer goroutine. The returned Recorder
// must be Close()d on shutdown; pending records flush before the
// goroutine exits.
func NewRecorder(store *Store, opts RecorderOptions) *Recorder {
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
		store: store,
		ch:    make(chan Record, opts.Capacity),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		opts:  opts,
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
		// retry the send.
		select {
		case <-r.ch:
			r.dropped.Add(1)
		default:
			// writer drained between full-detect and our drain
		}
		select {
		case r.ch <- rec:
		default:
			r.dropped.Add(1)
		}
	}
}

// Dropped returns the cumulative count of records dropped due to
// backpressure since the Recorder was created.
func (r *Recorder) Dropped() int64 {
	return r.dropped.Load()
}

// Close stops the writer goroutine after flushing pending records.
// Idempotent: safe to call multiple times.
func (r *Recorder) Close() {
	select {
	case <-r.stop:
		return
	default:
		close(r.stop)
	}
	<-r.done
}

func (r *Recorder) run() {
	defer close(r.done)

	batch := make([]Record, 0, r.opts.BatchSize)
	timer := time.NewTimer(r.opts.MaxLatency)
	defer timer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.store.AppendBatch(ctx, batch)
		cancel()
		batch = batch[:0]
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
