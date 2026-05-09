package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// Manager coordinates ordered shutdown of registered components.
//
// Hooks are run in reverse-init order (LIFO). The first non-nil error
// from any hook is returned from Shutdown and Run; subsequent errors
// are logged but not chained.
type Manager struct {
	logger logging.Logger
	mu     sync.Mutex
	hooks  []hook
}

type hook struct {
	name string
	fn   func(ctx context.Context) error
}

// New constructs a Manager that emits lifecycle events through the given
// Logger.
func New(logger logging.Logger) *Manager {
	return &Manager{logger: logger}
}

// RegisterShutdown adds a hook to be invoked during shutdown.
//
// Hooks are invoked in reverse-init order on shutdown — the most
// recently registered runs first. The name is included in log lines
// and shutdown error reporting.
//
// Safe for concurrent use during the registration phase. Registering
// during shutdown itself is allowed but the new hook will not run; it
// is queued for the next (never-arriving) shutdown.
func (m *Manager) RegisterShutdown(name string, fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook{name: name, fn: fn})
}

// Run blocks until SIGTERM, SIGINT, or ctx is cancelled, then invokes
// all registered shutdown hooks honoring gracePeriod as a hard timeout.
//
// Returns the first non-nil error from any hook, or context.DeadlineExceeded
// if gracePeriod expires while hooks are still running.
func (m *Manager) Run(ctx context.Context, gracePeriod time.Duration) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		m.logger.Info(ctx, "shutdown triggered by context cancellation")
	case sig := <-sigCh:
		m.logger.Info(ctx, "shutdown signal received", slog.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()
	return m.Shutdown(shutdownCtx)
}

// Shutdown invokes registered hooks in LIFO order, bounded by ctx.
//
// Each hook receives the same ctx — they share the deadline. A hook
// that ignores its context can still hold up shutdown; a well-behaved
// hook respects ctx.Done() and returns promptly.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	hooks := make([]hook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.Unlock()

	var firstErr error
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		m.logger.Info(ctx, "running shutdown hook", slog.String("hook", h.name))

		if ctx.Err() != nil {
			m.logger.Warn(ctx, "shutdown deadline exceeded; abandoning remaining hooks",
				slog.String("hook", h.name),
				slog.Int("remaining", i+1),
			)
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			return firstErr
		}

		if err := h.fn(ctx); err != nil {
			m.logger.Error(ctx, "shutdown hook failed", err, slog.String("hook", h.name))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
