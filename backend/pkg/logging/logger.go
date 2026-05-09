package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger is the structured logging contract every PowerLab service uses.
//
// Methods take a context.Context first so that the underlying handler can
// extract the correlation ID (and any future request-scoped attributes)
// without callers having to remember to pass it explicitly.
//
// There is intentionally no Fatal method. Process termination is the
// responsibility of pkg/lifecycle, which logs through Error and then
// triggers a graceful shutdown.
type Logger interface {
	Debug(ctx context.Context, msg string, attrs ...slog.Attr)
	Info(ctx context.Context, msg string, attrs ...slog.Attr)
	Warn(ctx context.Context, msg string, attrs ...slog.Attr)
	Error(ctx context.Context, msg string, err error, attrs ...slog.Attr)

	// With returns a new Logger that adds the given attributes to every
	// subsequent log line. Useful for service-scoped or component-scoped
	// loggers (e.g. logger.With(slog.String("component", "gateway"))).
	With(attrs ...slog.Attr) Logger
}

// CorrelationIDKey is the context.Context key under which a correlation
// ID may be stored. The Logger's Handler reads from this key on every
// emission and injects the value as a "correlation_id" attribute when
// non-empty.
//
// pkg/tracing will use this same key when it lands; defining it here
// keeps pkg/logging free of dependencies for now and lets services use
// correlation IDs even before tracing is wired up.
type CorrelationIDKey struct{}

// Config controls how a Logger is constructed.
//
// Zero values are sensible: Level defaults to "info", Format to "console",
// Writer to os.Stdout.
type Config struct {
	// Level is one of "debug", "info", "warn", "error" (case-insensitive).
	// Empty string defaults to "info".
	Level string

	// Format is "console" (human-readable) or "json" (machine-readable),
	// case-insensitive. Empty string defaults to "console".
	Format string

	// Writer is where log lines are emitted. Defaults to os.Stdout.
	Writer io.Writer
}

// New constructs a Logger from the given Config.
//
// Returns an error if Level or Format is set to an unrecognized value.
// Empty strings are treated as defaults.
func New(cfg Config) (Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	format, err := parseFormat(cfg.Format)
	if err != nil {
		return nil, err
	}

	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	var base slog.Handler
	switch format {
	case "json":
		base = slog.NewJSONHandler(writer, opts)
	default: // "console"
		base = slog.NewTextHandler(writer, opts)
	}

	return &slogLogger{slog: slog.New(&correlationHandler{base: base})}, nil
}

func parseLevel(s string) (slog.Level, error) {
	if s == "" {
		return slog.LevelInfo, nil
	}
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("logging.New: unknown level %q (want one of: debug, info, warn, error)", s)
}

func parseFormat(s string) (string, error) {
	if s == "" {
		return "console", nil
	}
	switch strings.ToLower(s) {
	case "console":
		return "console", nil
	case "json":
		return "json", nil
	}
	return "", fmt.Errorf("logging.New: unknown format %q (want: console or json)", s)
}

// correlationHandler wraps an underlying slog.Handler and injects a
// "correlation_id" attribute on every record when one is present in the
// context. Other handler methods delegate transparently.
type correlationHandler struct {
	base slog.Handler
}

func (h *correlationHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *correlationHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(CorrelationIDKey{}).(string); ok && id != "" {
		r.AddAttrs(slog.String("correlation_id", id))
	}
	return h.base.Handle(ctx, r)
}

func (h *correlationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &correlationHandler{base: h.base.WithAttrs(attrs)}
}

func (h *correlationHandler) WithGroup(name string) slog.Handler {
	return &correlationHandler{base: h.base.WithGroup(name)}
}

// slogLogger implements Logger over a *slog.Logger.
type slogLogger struct {
	slog *slog.Logger
}

func (l *slogLogger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.slog.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

func (l *slogLogger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.slog.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *slogLogger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.slog.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

func (l *slogLogger) Error(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	l.slog.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

func (l *slogLogger) With(attrs ...slog.Attr) Logger {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return &slogLogger{slog: l.slog.With(args...)}
}
