package lifecycle

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/neochaotic/powerlab/backend/pkg/errors"
	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// RecoverMiddleware returns an HTTP middleware that catches panics in
// the wrapped handler chain.
//
// On panic it:
//   - Logs the panic value, stack trace, request method, and request
//     path at error level, with the correlation ID auto-attached by
//     pkg/logging from request context.
//   - Writes a 500 response via errors.WriteHTTP using ErrInternal so
//     the body shape matches every other error response.
//   - The process keeps running. The class of bug behind #64
//     (nil-deref → SIGSEGV → process death) cannot escape past this
//     middleware once handlers are wrapped.
//
// Wrap as the outermost middleware on the chain so it sees panics from
// every nested handler.
func RecoverMiddleware(logger logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error(r.Context(), "panic recovered in handler",
						panicError(rec),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("stack", string(debug.Stack())),
					)
					errors.WriteHTTP(r.Context(), w, errors.ErrInternal)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SafeGo runs fn in a goroutine, recovering from any panic and logging
// the recovered value with a stack trace.
//
// Use anywhere a bare `go fn()` would crash the process if fn panics.
// Background workers, periodic tasks, async cleanup — these all want
// SafeGo. The panic is logged and the goroutine exits cleanly; the
// process and other goroutines keep running.
//
// ctx is used only for the panic log — the goroutine itself does not
// observe ctx.Done(). If callers want cancellation, fn should accept
// the ctx in its closure and respect it directly.
func SafeGo(ctx context.Context, logger logging.Logger, fn func()) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error(ctx, "panic recovered in goroutine",
					panicError(rec),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()
		fn()
	}()
}

// panicError converts a recover() result into an error so it can be
// passed to logger.Error. Most panics are strings or errors; the
// fallback covers the nil and weird-typed cases.
func panicError(rec any) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return &recoveredPanic{value: rec}
}

type recoveredPanic struct{ value any }

func (r *recoveredPanic) Error() string {
	switch v := r.value.(type) {
	case string:
		return v
	case nil:
		return "nil panic"
	default:
		return "panic: non-string, non-error value"
	}
}
