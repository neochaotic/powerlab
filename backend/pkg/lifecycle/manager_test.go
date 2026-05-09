package lifecycle_test

import (
	"context"
	stderrors "errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/pkg/lifecycle"
	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

func newLogger(t *testing.T) logging.Logger {
	t.Helper()
	l, err := logging.New(logging.Config{Level: "info", Format: "json", Writer: io.Discard})
	if err != nil {
		t.Fatalf("logging.New: %v", err)
	}
	return l
}

// --------------------------------------------------------------------
// New / Manager basics
// --------------------------------------------------------------------

func TestNew_ReturnsManager(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
}

// --------------------------------------------------------------------
// Shutdown — ordering & error semantics
// --------------------------------------------------------------------

func TestShutdown_NoHooks_ReturnsNil(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	if err := m.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown with no hooks should be no-op; got: %v", err)
	}
}

func TestShutdown_RunsHooksInLIFOOrder(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	var order []string

	m.RegisterShutdown("first", func(ctx context.Context) error {
		order = append(order, "first")
		return nil
	})
	m.RegisterShutdown("second", func(ctx context.Context) error {
		order = append(order, "second")
		return nil
	})
	m.RegisterShutdown("third", func(ctx context.Context) error {
		order = append(order, "third")
		return nil
	})

	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	want := []string{"third", "second", "first"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order: want %v, got %v", want, order)
	}
}

func TestShutdown_ReturnsFirstError(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	first := stderrors.New("first failure")
	second := stderrors.New("second failure")

	// LIFO: third registered runs first.
	m.RegisterShutdown("a", func(ctx context.Context) error { return nil })
	m.RegisterShutdown("b", func(ctx context.Context) error { return second })
	m.RegisterShutdown("c", func(ctx context.Context) error { return first })

	got := m.Shutdown(context.Background())
	if got != first {
		t.Errorf("expected first error to bubble up; got %v", got)
	}
}

func TestShutdown_ContinuesAfterHookError(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	ranAfterError := false

	m.RegisterShutdown("first", func(ctx context.Context) error {
		ranAfterError = true
		return nil
	})
	m.RegisterShutdown("second-fails", func(ctx context.Context) error {
		return stderrors.New("oops")
	})

	_ = m.Shutdown(context.Background())
	if !ranAfterError {
		t.Error("first hook (registered before failing one) should still run")
	}
}

// --------------------------------------------------------------------
// Run — signal/context-driven entry point
// --------------------------------------------------------------------

func TestRun_TriggersShutdownOnContextCancel(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	var hookRan bool
	m.RegisterShutdown("hook", func(ctx context.Context) error {
		hookRan = true
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Run(ctx, time.Second)
	}()

	// Tiny sync window — let Run register its signal handler and block.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run: unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	if !hookRan {
		t.Error("hook did not run on context-cancel-driven shutdown")
	}
}

func TestRun_GracePeriodBoundsHookExecution(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	hookCompleted := false

	m.RegisterShutdown("slow", func(ctx context.Context) error {
		select {
		case <-time.After(500 * time.Millisecond):
			hookCompleted = true
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Run(ctx, 50*time.Millisecond) // short grace
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected non-nil error (hook should have hit grace deadline)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}

	if hookCompleted {
		t.Error("slow hook should not have completed within the grace period")
	}
}

func TestShutdown_RespectsContextDeadline(t *testing.T) {
	m := lifecycle.New(newLogger(t))
	ranSecond := false

	// LIFO: slow runs first, fast runs second. We expect slow to time
	// out, slow's error to bubble up, and fast (the second-LIFO hook)
	// NOT to run because the deadline already expired.
	m.RegisterShutdown("fast", func(ctx context.Context) error {
		ranSecond = true
		return nil
	})
	m.RegisterShutdown("slow", func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := m.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected deadline error, got nil")
	}
	if ranSecond {
		t.Error("second hook should not have run after deadline expired")
	}
}
