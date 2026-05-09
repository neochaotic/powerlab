package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/neochaotic/powerlab/backend/pkg/logging"
)

// newCapture returns a Logger writing into the returned buffer.
// Test helper — keeps each test focused on behavior, not setup.
func newCapture(t *testing.T, level, format string) (logging.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l, err := logging.New(logging.Config{
		Level:  level,
		Format: format,
		Writer: &buf,
	})
	if err != nil {
		t.Fatalf("logging.New: unexpected error: %v", err)
	}
	if l == nil {
		t.Fatal("logging.New: returned nil Logger")
	}
	return l, &buf
}

// --------------------------------------------------------------------
// New — construction & validation
// --------------------------------------------------------------------

func TestNew_DefaultConfig_ReturnsLogger(t *testing.T) {
	l, err := logging.New(logging.Config{Writer: io.Discard})
	if err != nil {
		t.Fatalf("expected no error for zero Config, got: %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil Logger for zero Config")
	}
}

func TestNew_AllValidLevels_Construct(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error", "DEBUG", "Info"} {
		t.Run(lvl, func(t *testing.T) {
			_, err := logging.New(logging.Config{Level: lvl, Writer: io.Discard})
			if err != nil {
				t.Fatalf("level %q should be valid, got error: %v", lvl, err)
			}
		})
	}
}

func TestNew_InvalidLevel_ReturnsError(t *testing.T) {
	_, err := logging.New(logging.Config{Level: "verbose", Writer: io.Discard})
	if err == nil {
		t.Fatal("expected error for unknown level, got nil")
	}
}

func TestNew_AllValidFormats_Construct(t *testing.T) {
	for _, f := range []string{"console", "json", "JSON", "Console"} {
		t.Run(f, func(t *testing.T) {
			_, err := logging.New(logging.Config{Format: f, Writer: io.Discard})
			if err != nil {
				t.Fatalf("format %q should be valid, got error: %v", f, err)
			}
		})
	}
}

func TestNew_InvalidFormat_ReturnsError(t *testing.T) {
	_, err := logging.New(logging.Config{Format: "logfmt", Writer: io.Discard})
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}

// --------------------------------------------------------------------
// Emission — message and attribute serialization
// --------------------------------------------------------------------

func TestInfo_EmitsMessage(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	l.Info(context.Background(), "service started")
	if !strings.Contains(buf.String(), "service started") {
		t.Errorf("expected output to contain message; got: %s", buf.String())
	}
}

func TestInfo_EmitsAttributes(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	l.Info(context.Background(), "served request",
		slog.String("method", "GET"),
		slog.Int("status", 200),
	)

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got: %s\nerr: %v", buf.String(), err)
	}
	if parsed["method"] != "GET" {
		t.Errorf("expected method=GET, got: %v", parsed["method"])
	}
	if parsed["status"] != float64(200) { // JSON numbers are float64 in Go
		t.Errorf("expected status=200, got: %v", parsed["status"])
	}
}

func TestError_IncludesErrorMessage(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	l.Error(context.Background(), "operation failed", errors.New("disk full"))

	out := buf.String()
	if !strings.Contains(out, "operation failed") {
		t.Errorf("expected output to contain message; got: %s", out)
	}
	if !strings.Contains(out, "disk full") {
		t.Errorf("expected output to contain wrapped error message; got: %s", out)
	}
}

// --------------------------------------------------------------------
// Levels — filtering behavior
// --------------------------------------------------------------------

func TestDebug_FilteredAtInfoLevel(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	l.Debug(context.Background(), "verbose detail")
	if buf.Len() != 0 {
		t.Errorf("expected no output at info level for debug call; got: %s", buf.String())
	}
}

func TestDebug_EmittedAtDebugLevel(t *testing.T) {
	l, buf := newCapture(t, "debug", "json")
	l.Debug(context.Background(), "verbose detail")
	if !strings.Contains(buf.String(), "verbose detail") {
		t.Errorf("expected debug output at debug level; got: %s", buf.String())
	}
}

func TestInfo_FilteredAtErrorLevel(t *testing.T) {
	l, buf := newCapture(t, "error", "json")
	l.Info(context.Background(), "operational")
	if buf.Len() != 0 {
		t.Errorf("expected no output at error level for info call; got: %s", buf.String())
	}
}

// --------------------------------------------------------------------
// With — persistent attributes
// --------------------------------------------------------------------

func TestWith_AddsPersistentAttrs(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	scoped := l.With(slog.String("component", "gateway"))
	scoped.Info(context.Background(), "started")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", buf.String(), err)
	}
	if parsed["component"] != "gateway" {
		t.Errorf("expected component=gateway, got: %v", parsed["component"])
	}
}

func TestWith_DoesNotAffectParent(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	_ = l.With(slog.String("component", "scoped"))
	l.Info(context.Background(), "from parent")

	if strings.Contains(buf.String(), "scoped") {
		t.Errorf("parent logger leaked scoped attribute; got: %s", buf.String())
	}
}

// --------------------------------------------------------------------
// Correlation ID — auto-injection from context
// --------------------------------------------------------------------

func TestInfo_InjectsCorrelationIDFromContext(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	ctx := context.WithValue(context.Background(), logging.CorrelationIDKey{}, "req-abc-123")
	l.Info(ctx, "handled")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", buf.String(), err)
	}
	if parsed["correlation_id"] != "req-abc-123" {
		t.Errorf("expected correlation_id=req-abc-123, got: %v", parsed["correlation_id"])
	}
}

func TestInfo_NoCorrelationIDWhenAbsent(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	l.Info(context.Background(), "handled")

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %s\nerr: %v", buf.String(), err)
	}
	// Either absent, or present-but-empty. Both acceptable; presence with
	// a non-empty value would be the bug.
	if v, ok := parsed["correlation_id"]; ok && v != "" {
		t.Errorf("expected no correlation_id (or empty), got: %v", v)
	}
}

// --------------------------------------------------------------------
// Format — console vs json
// --------------------------------------------------------------------

func TestFormat_JSON_ProducesValidJSON(t *testing.T) {
	l, buf := newCapture(t, "info", "json")
	l.Info(context.Background(), "structured", slog.String("k", "v"))

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("json format should produce valid JSON, got: %s\nerr: %v", buf.String(), err)
	}
	if parsed["msg"] != "structured" {
		t.Errorf("expected msg=structured, got: %v", parsed["msg"])
	}
}

func TestFormat_Console_NotJSON(t *testing.T) {
	l, buf := newCapture(t, "info", "console")
	l.Info(context.Background(), "human readable")

	out := strings.TrimSpace(buf.String())
	// console format starts with a timestamp / level prefix, not '{'.
	if strings.HasPrefix(out, "{") {
		t.Errorf("console format should not start with '{', got: %s", out)
	}
	if !strings.Contains(out, "human readable") {
		t.Errorf("expected output to contain message; got: %s", out)
	}
}
